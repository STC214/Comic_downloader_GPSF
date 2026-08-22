package assets

import (
	"context"
	"fmt"
	"image"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	_ "github.com/gen2brain/avif"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

const (
	maxWorkers             = 7
	maxImageBytes          = int64(128 << 20)
	defaultDownloadRetries = 4
	hitomiDownloadRetries  = 20
)

var outputCommitLocks sync.Map

type CollectionSummary struct {
	Site      string
	BaseURL   string
	ReaderURL string
	Title     string
}

type DownloadProgress struct {
	Current  int
	Total    int
	Phase    string
	Message  string
	Fraction float64
}

type DownloadProgressFunc func(DownloadProgress)

type DownloadResult struct {
	OutputDir string
	Files     []string
	Bytes     int64
}

func DownloadImagesContext(ctx context.Context, summary CollectionSummary, imageURLs []string, outputRoot string, progress DownloadProgressFunc) (DownloadResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return DownloadResult{}, err
	}
	outputRoot = strings.TrimSpace(outputRoot)
	if outputRoot == "" {
		return DownloadResult{}, fmt.Errorf("output root is empty")
	}
	imageURLs = uniqueStrings(imageURLs)
	if len(imageURLs) == 0 {
		return DownloadResult{}, fmt.Errorf("image urls are empty")
	}
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return DownloadResult{}, fmt.Errorf("create output root %q: %w", outputRoot, err)
	}
	outputDir := filepath.Join(outputRoot, sanitizePathPart(summary.Title))
	if err := recoverInterruptedCommit(outputDir); err != nil {
		return DownloadResult{}, err
	}
	stagingDir, err := os.MkdirTemp(outputRoot, "."+sanitizePathPart(summary.Title)+".staging-*")
	if err != nil {
		return DownloadResult{}, fmt.Errorf("create staging dir for %q: %w", outputDir, err)
	}
	defer os.RemoveAll(stagingDir)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Minute}
	files := make([]string, len(imageURLs))
	var totalBytes int64
	var resultMu sync.Mutex
	completed := 0
	var firstErr error
	var errMu sync.Mutex
	type job struct {
		index int
		url   string
	}
	jobs := make(chan job)
	workers := maxWorkers
	if len(imageURLs) < workers {
		workers = len(imageURLs)
	}
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				referer := summary.ReaderURL
				if strings.TrimSpace(referer) == "" {
					referer = summary.BaseURL
				}
				if strings.TrimSpace(referer) == "" && strings.EqualFold(strings.TrimSpace(summary.Site), "hitomi") {
					referer = "https://hitomi.la/"
				}
				path, size, err := downloadOne(ctx, client, item.url, referer, stagingDir, item.index+1)
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
						cancel()
					}
					errMu.Unlock()
					return
				}
				resultMu.Lock()
				files[item.index] = path
				totalBytes += size
				completed++
				if progress != nil {
					progress(DownloadProgress{
						Current: completed, Total: len(imageURLs), Phase: "下载",
						Message:  fmt.Sprintf("%d/%d (%d bytes)", completed, len(imageURLs), totalBytes),
						Fraction: float64(completed) / float64(len(imageURLs)),
					})
				}
				resultMu.Unlock()
			}
		}()
	}
sendLoop:
	for i, raw := range imageURLs {
		select {
		case jobs <- job{index: i, url: raw}:
		case <-ctx.Done():
			break sendLoop
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return DownloadResult{}, firstErr
	}
	if err := ctx.Err(); err != nil {
		return DownloadResult{}, err
	}
	if err := commitStagedDirectory(stagingDir, outputDir); err != nil {
		return DownloadResult{}, err
	}
	for i, file := range files {
		files[i] = filepath.Join(outputDir, filepath.Base(file))
	}
	return DownloadResult{OutputDir: outputDir, Files: files, Bytes: totalBytes}, nil
}

func commitStagedDirectory(stagingDir, outputDir string) error {
	lock := outputCommitLock(outputDir)
	lock.Lock()
	defer lock.Unlock()

	if _, err := os.Stat(outputDir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat existing output dir %q: %w", outputDir, err)
		}
		if err := os.Rename(stagingDir, outputDir); err != nil {
			return fmt.Errorf("commit staging dir %q: %w", outputDir, err)
		}
		return nil
	}

	backupDir, err := os.MkdirTemp(filepath.Dir(outputDir), "."+filepath.Base(outputDir)+".previous-*")
	if err != nil {
		return fmt.Errorf("reserve output backup for %q: %w", outputDir, err)
	}
	if err := os.Remove(backupDir); err != nil {
		return fmt.Errorf("prepare output backup %q: %w", backupDir, err)
	}
	if err := os.Rename(outputDir, backupDir); err != nil {
		return fmt.Errorf("backup existing output dir %q: %w", outputDir, err)
	}
	if err := os.Rename(stagingDir, outputDir); err != nil {
		restoreErr := os.Rename(backupDir, outputDir)
		if restoreErr != nil {
			return fmt.Errorf("commit output dir %q: %v; restore failed: %w", outputDir, err, restoreErr)
		}
		return fmt.Errorf("commit output dir %q: %w", outputDir, err)
	}
	_ = os.RemoveAll(backupDir)
	return nil
}

func recoverInterruptedCommit(outputDir string) error {
	lock := outputCommitLock(outputDir)
	lock.Lock()
	defer lock.Unlock()

	parentDir := filepath.Dir(outputDir)
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return fmt.Errorf("find interrupted output backups for %q: %w", outputDir, err)
	}
	backupPrefix := "." + filepath.Base(outputDir) + ".previous-"
	backups := make([]string, 0)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), backupPrefix) {
			backups = append(backups, filepath.Join(parentDir, entry.Name()))
		}
	}
	if len(backups) == 0 {
		return nil
	}
	if _, err := os.Stat(outputDir); err == nil {
		for _, backup := range backups {
			if err := os.RemoveAll(backup); err != nil {
				return fmt.Errorf("remove stale output backup %q: %w", backup, err)
			}
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat output dir during recovery %q: %w", outputDir, err)
	}

	newest := backups[0]
	newestTime := time.Time{}
	for _, backup := range backups {
		info, statErr := os.Stat(backup)
		if statErr != nil {
			return fmt.Errorf("stat interrupted output backup %q: %w", backup, statErr)
		}
		if info.ModTime().After(newestTime) {
			newest = backup
			newestTime = info.ModTime()
		}
	}
	if err := os.Rename(newest, outputDir); err != nil {
		return fmt.Errorf("restore interrupted output backup %q: %w", newest, err)
	}
	for _, backup := range backups {
		if backup == newest {
			continue
		}
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove superseded output backup %q: %w", backup, err)
		}
	}
	return nil
}

func outputCommitLock(outputDir string) *sync.Mutex {
	key := strings.ToLower(filepath.Clean(outputDir))
	value, _ := outputCommitLocks.LoadOrStore(key, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func downloadOne(ctx context.Context, client *http.Client, rawURL, referer, outputDir string, index int) (string, int64, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", 0, fmt.Errorf("parse image url %q: %w", rawURL, err)
	}
	var resp *http.Response
	attempts := defaultDownloadRetries
	if isHitomiHost(parsed.Hostname()) {
		attempts = hitomiDownloadRetries
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if reqErr != nil {
			return "", 0, reqErr
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		if strings.TrimSpace(referer) != "" {
			req.Header.Set("Referer", referer)
		}
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		if err == nil && resp != nil && resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			return "", 0, fmt.Errorf("download image %q: unexpected status %s", rawURL, resp.Status)
		}
		if attempt >= attempts {
			break
		}
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		case <-time.After(time.Duration(attempt*250) * time.Millisecond):
		}
	}
	if err != nil {
		return "", 0, fmt.Errorf("download image %q: %w", rawURL, err)
	}
	if resp == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("download image %q failed after retries", rawURL)
	}
	defer resp.Body.Close()
	if resp.ContentLength > maxImageBytes {
		return "", 0, fmt.Errorf("download image %q exceeds size limit", rawURL)
	}
	ext := strings.ToLower(filepath.Ext(parsed.Path))
	if len(ext) > 8 || strings.ContainsAny(ext, `\/:*?"<>|`) {
		ext = ""
	}
	if ext == "" {
		if extensions, _ := mime.ExtensionsByType(resp.Header.Get("Content-Type")); len(extensions) > 0 {
			ext = extensions[0]
		}
	}
	if ext == "" {
		ext = ".jpg"
	}
	target := filepath.Join(outputDir, fmt.Sprintf("%04d%s", index, ext))
	file, err := os.CreateTemp(outputDir, fmt.Sprintf(".%04d-*.part", index))
	if err != nil {
		return "", 0, err
	}
	tempPath := file.Name()
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	limited := &io.LimitedReader{R: resp.Body, N: maxImageBytes + 1}
	written, err := io.Copy(file, limited)
	if err != nil {
		return "", written, err
	}
	if written > maxImageBytes {
		return "", written, fmt.Errorf("download image %q exceeds size limit", rawURL)
	}
	if err := file.Close(); err != nil {
		return "", written, fmt.Errorf("close temporary image %q: %w", tempPath, err)
	}
	probe, err := os.Open(tempPath)
	if err != nil {
		return "", written, err
	}
	config, _, decodeErr := image.DecodeConfig(probe)
	_ = probe.Close()
	if decodeErr != nil || config.Width <= 0 || config.Height <= 0 {
		return "", written, fmt.Errorf("download image %q returned invalid image data", rawURL)
	}
	if err := os.Rename(tempPath, target); err != nil {
		if removeErr := os.Remove(target); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", written, fmt.Errorf("replace image %q: %w", target, removeErr)
		}
		if err := os.Rename(tempPath, target); err != nil {
			return "", written, fmt.Errorf("commit image %q: %w", target, err)
		}
	}
	committed = true
	return target, written, nil
}

func isHitomiHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "hitomi.la" || strings.HasSuffix(host, ".hitomi.la") ||
		host == "gold-usergeneratedcontent.net" || strings.HasSuffix(host, ".gold-usergeneratedcontent.net")
}

func sanitizePathPart(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "download"
	}
	var b strings.Builder
	for _, r := range []rune(text)[:min(len([]rune(text)), 120)] {
		switch {
		case r < 32 || strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteRune('_')
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Trim(strings.TrimSpace(b.String()), ".")
	if out == "" {
		return "download"
	}
	switch strings.ToUpper(out) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		out += "_"
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
