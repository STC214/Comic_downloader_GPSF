package zeri

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// DownloadProgress reports the status of reader image downloads.
type DownloadProgress struct {
	Current  int     `json:"current"`
	Total    int     `json:"total"`
	Phase    string  `json:"phase,omitempty"`
	Message  string  `json:"message,omitempty"`
	Fraction float64 `json:"fraction"`
}

// DownloadProgressFunc receives download progress updates.
type DownloadProgressFunc func(DownloadProgress)

// DownloadResult summarizes a batch of downloaded reader images.
type DownloadResult struct {
	OutputDir string   `json:"outputDir"`
	Files     []string `json:"files,omitempty"`
	Bytes     int64    `json:"bytes"`
}

// DownloadImages downloads the collected image URLs into a chapter-scoped output directory.
func DownloadImages(summary SummaryPage, imageURLs []string, outputRoot string, progress DownloadProgressFunc) (DownloadResult, error) {
	outputRoot = strings.TrimSpace(outputRoot)
	if outputRoot == "" {
		return DownloadResult{}, fmt.Errorf("output root is empty")
	}
	imageURLs = normalizeUniqueStrings(imageURLs)
	if len(imageURLs) == 0 {
		return DownloadResult{}, fmt.Errorf("image urls are empty")
	}

	chapterDir := filepath.Join(outputRoot, "zeri")
	if title := sanitizePathPart(summary.Title); title != "" {
		chapterDir = filepath.Join(chapterDir, title)
	}
	if err := os.MkdirAll(chapterDir, 0o755); err != nil {
		return DownloadResult{}, fmt.Errorf("create output dir %q: %w", chapterDir, err)
	}

	files := make([]string, 0, len(imageURLs))
	var totalBytes int64
	usedNames := make(map[string]int, len(imageURLs))
	report := func(current int, phase, message string) {
		if progress == nil {
			return
		}
		total := len(imageURLs)
		fraction := 0.0
		if total > 0 {
			fraction = float64(current) / float64(total)
		}
		progress(DownloadProgress{
			Current:  current,
			Total:    total,
			Phase:    phase,
			Message:  message,
			Fraction: fraction,
		})
	}

	report(0, "准备", "开始下载")
	for i, raw := range imageURLs {
		report(i, "下载中", fmt.Sprintf("%d/%d", i, len(imageURLs)))
		saved, written, err := downloadOneImage(raw, chapterDir, i+1, usedNames)
		if err != nil {
			return DownloadResult{}, err
		}
		files = append(files, saved)
		totalBytes += written
		report(i+1, "下载中", fmt.Sprintf("%d/%d", i+1, len(imageURLs)))
	}
	report(len(imageURLs), "完成", "下载完成")

	return DownloadResult{
		OutputDir: chapterDir,
		Files:     files,
		Bytes:     totalBytes,
	}, nil
}

func downloadOneImage(rawURL, outputDir string, index int, usedNames map[string]int) (string, int64, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", 0, fmt.Errorf("image url is empty")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", 0, fmt.Errorf("parse image url %q: %w", rawURL, err)
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("create image request %q: %w", rawURL, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("download image %q: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("download image %q: unexpected status %s", rawURL, resp.Status)
	}

	baseName := strings.TrimSuffix(filepath.Base(parsed.Path), filepath.Ext(parsed.Path))
	ext := sanitizeExt(filepath.Ext(parsed.Path))
	if ext == "" {
		ext = extFromContentType(resp.Header.Get("Content-Type"))
	}
	if ext == "" {
		ext = ".jpg"
	}

	filename := uniqueDownloadFilename(baseName, ext, index, usedNames)
	targetPath := filepath.Join(outputDir, filename)

	file, err := os.Create(targetPath)
	if err != nil {
		return "", 0, fmt.Errorf("create image file %q: %w", targetPath, err)
	}
	defer file.Close()

	written, err := io.Copy(file, resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("write image file %q: %w", targetPath, err)
	}
	return targetPath, written, nil
}

func uniqueDownloadFilename(baseName, ext string, index int, usedNames map[string]int) string {
	baseName = sanitizePathPart(baseName)
	if baseName == "" {
		baseName = fmt.Sprintf("%03d", index)
	}
	ext = sanitizeExt(ext)
	if ext == "" {
		ext = ".jpg"
	}
	key := strings.ToLower(baseName + ext)
	usedNames[key]++
	if usedNames[key] == 1 {
		return baseName + ext
	}
	return fmt.Sprintf("%s-%d%s", baseName, usedNames[key], ext)
}

func extFromContentType(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return ""
	}
	if ext, _ := mime.ExtensionsByType(contentType); len(ext) > 0 {
		for _, candidate := range ext {
			if candidate != "" {
				return candidate
			}
		}
	}
	return ""
}

func sanitizeExt(ext string) string {
	ext = strings.TrimSpace(ext)
	if ext == "" {
		return ""
	}
	if len(ext) > 8 {
		return ""
	}
	if strings.ContainsAny(ext, `\/:*?"<>|`) {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		return ""
	}
	return strings.ToLower(ext)
}

func sanitizePathPart(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(text))
	lastDash := false
	for _, r := range text {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r):
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	return out
}
