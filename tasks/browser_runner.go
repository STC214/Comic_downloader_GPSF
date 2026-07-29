package tasks

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	projectruntime "comic_downloader_go_playwright_stealth/runtime"
	"comic_downloader_go_playwright_stealth/siteflow/assets"
	"comic_downloader_go_playwright_stealth/siteflow/hentai2"
	"comic_downloader_go_playwright_stealth/siteflow/hentaiaz"
	"comic_downloader_go_playwright_stealth/siteflow/hitomi"
	"comic_downloader_go_playwright_stealth/siteflow/nyahentai"
	"comic_downloader_go_playwright_stealth/siteflow/zeri"
)

// BrowserRunResult is the task-layer outcome of opening a browser page.
type BrowserRunResult struct {
	URL                  string `json:"url"`
	ResolvedURL          string `json:"resolvedURL,omitempty"`
	Title                string `json:"title"`
	BrowserType          string `json:"browserType,omitempty"`
	BrowserPath          string `json:"browserPath,omitempty"`
	BrowserMode          string `json:"browserMode,omitempty"`
	Headless             bool   `json:"headless"`
	KeepOpen             bool   `json:"keepOpen"`
	PlaywrightProfileDir string `json:"playwrightProfileDir,omitempty"`
	Site                 string `json:"site,omitempty"`
	PageType             string `json:"pageType,omitempty"`
	ReaderURL            string `json:"readerURL,omitempty"`
	SummaryPageCount     int    `json:"summaryPageCount,omitempty"`
	ReaderPageCount      int    `json:"readerPageCount,omitempty"`
	ReaderImageCount     int    `json:"readerImageCount,omitempty"`
	ReaderFilteredCount  int    `json:"readerFilteredCount,omitempty"`
	ReaderActivation     int    `json:"readerActivationClicks,omitempty"`
	Verified             bool   `json:"verified,omitempty"`
	VerificationNeeded   bool   `json:"verificationNeeded,omitempty"`
	Blocked              bool   `json:"blocked,omitempty"`
	MatchedMarker        string `json:"matchedMarker,omitempty"`
	Note                 string `json:"note,omitempty"`
	DownloadedCount      int    `json:"downloadedCount,omitempty"`
	DownloadedBytes      int64  `json:"downloadedBytes,omitempty"`
	DownloadedDir        string `json:"downloadedDir,omitempty"`
	ThumbnailPath        string `json:"thumbnailPath,omitempty"`
}

// RunBrowserRequest opens the page described by the request and returns a normalized result.
func RunBrowserRequest(req BrowserLaunchRequest) (BrowserRunResult, error) {
	req = req.Normalize()
	ctx := req.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return BrowserRunResult{}, err
	}
	if strings.TrimSpace(req.URL) == "" {
		return BrowserRunResult{}, fmt.Errorf("browser url is empty")
	}
	runtimePaths := projectruntime.NewPathsFromRuntimeRoot(req.RuntimeRoot)
	if hitomi.IsHitomiURL(req.URL) {
		return runHitomiRequest(ctx, req, runtimePaths)
	}
	if !zeri.IsZeriURL(req.URL) && !hentai2.IsHentai2URL(req.URL) && !hentaiaz.IsHentaiazURL(req.URL) && !nyahentai.IsNyahentaiURL(req.URL) {
		return BrowserRunResult{}, fmt.Errorf("unsupported download site: %s", req.URL)
	}
	log.Printf("browser request start: type=%s headless=%t keepOpen=%t url=%s output=%s profile=%s driver=%s",
		req.BrowserType, req.Headless, req.KeepOpen, req.URL, req.OutputDir, req.ProfileDir, req.DriverDir)

	manager := projectruntime.NewBrowserProfileManager(workspaceRootFromRuntimeRoot(req.RuntimeRoot))
	var cleanupProfile func()
	activeProfileDir := ""

	profile, err := manager.PrepareFreshPlaywrightProfile(projectruntime.BrowserType(req.BrowserType))
	if err != nil {
		return BrowserRunResult{}, err
	}
	if err := ctx.Err(); err != nil {
		_ = manager.CleanupFreshPlaywrightProfile(profile)
		return BrowserRunResult{}, err
	}
	req.UserDataDir = absolutePathOrClean(profile.RootDir)
	req.ProfileDir = req.UserDataDir
	activeProfileDir = req.UserDataDir
	log.Printf("browser task profile ready: %s", activeProfileDir)
	cleanupProfile = func() {
		_ = manager.CleanupFreshPlaywrightProfile(profile)
	}

	if activeProfileDir != "" {
		log.Printf("profile flow: source=%s temp=%s output=%s", "(fresh)", activeProfileDir, req.OutputDir)
		logBrowserProfileAudit(req.BrowserType, "", activeProfileDir)
	}

	if req.Progress != nil {
		req.Progress(zeri.DownloadProgress{Fraction: 0.02, Phase: "启动", Message: "准备"})
	}

	session, err := openTaskBrowserSession(req)
	if err != nil {
		if cleanupProfile != nil {
			cleanupProfile()
		}
		return BrowserRunResult{}, preferContextError(ctx, err)
	}
	if req.Progress != nil {
		req.Progress(zeri.DownloadProgress{Fraction: 0.08, Phase: "启动", Message: "完成"})
	}
	var closeOnce sync.Once
	closeSession := func() {
		closeOnce.Do(func() {
			_ = session.Close()
		})
	}
	stopCancelWatcher := make(chan struct{})
	cancelWatcherDone := make(chan struct{})
	go func() {
		defer close(cancelWatcherDone)
		select {
		case <-ctx.Done():
			log.Printf("browser request canceled: task=%s url=%s", req.TaskID, req.URL)
			closeSession()
		case <-stopCancelWatcher:
		}
	}()
	defer func() {
		close(stopCancelWatcher)
		closeSession()
		<-cancelWatcherDone
		if cleanupProfile != nil {
			cleanupProfile()
		}
	}()

	var zeriResult zeri.ExecutionResult
	var downloadResult assets.DownloadResult
	site := ""
	var imageURLs []string
	var siteTitle, resolvedURL, readerURL string
	var pageCount, readerPageCount, readerImageCount, filteredImageCount int
	downloadStart, downloadSpan := 0.35, 0.65
	if zeri.IsZeriURL(req.URL) {
		site = "zeri"
		if req.Progress != nil {
			req.Progress(zeri.DownloadProgress{Fraction: 0.10, Phase: "解析", Message: "摘要"})
		}
		zeriResult, err = zeri.ExecuteWithProgress(session, req.URL, progressSpan(req.Progress, 0.10, 0.90))
		if err != nil {
			return BrowserRunResult{}, preferContextError(ctx, err)
		}
		siteTitle = zeriResult.FinalTitle
		resolvedURL = zeriResult.Reader.URL
		readerURL = zeriResult.Reader.URL
		pageCount = zeriResult.Summary.PageCount
		readerPageCount = zeriResult.Reader.PageCount
		readerImageCount = len(zeriResult.Reader.ImageURLs)
		filteredImageCount = len(zeriResult.CollectedImages)
		imageURLs = zeriResult.CollectedImages
		downloadWeight := zeri.DownloadWeightForCount(zeriResult.Summary.PageCount)
		parseWeight := 1 - downloadWeight
		if parseWeight < 0 {
			parseWeight = 0
		}
		downloadStart = 0.10 + 0.90*parseWeight
		downloadSpan = 0.90 * downloadWeight
	} else {
		switch {
		case hentai2.IsHentai2URL(req.URL):
			site = "hentai2"
			result, runErr := hentai2.ExecuteWithProgress(session, req.URL, progressSpan(req.Progress, 0.10, 0.25))
			if runErr != nil {
				return BrowserRunResult{}, preferContextError(ctx, runErr)
			}
			siteTitle, resolvedURL, readerURL = result.FinalTitle, result.FinalURL, result.Reader.URL
			pageCount, readerPageCount = result.Summary.PageCount, len(result.CollectedImages)
			readerImageCount, filteredImageCount = len(result.Reader.ImageURLs), len(result.CollectedImages)
			imageURLs = result.CollectedImages
		case hentaiaz.IsHentaiazURL(req.URL):
			site = "hentaiaz"
			result, runErr := hentaiaz.ExecuteWithProgress(session, req.URL, progressSpan(req.Progress, 0.10, 0.25))
			if runErr != nil {
				return BrowserRunResult{}, preferContextError(ctx, runErr)
			}
			siteTitle, resolvedURL, readerURL = result.FinalTitle, result.FinalURL, result.Reader.URL
			pageCount, readerPageCount = result.Summary.PageCount, len(result.CollectedImages)
			readerImageCount, filteredImageCount = len(result.Reader.ImageURLs), len(result.CollectedImages)
			imageURLs = result.CollectedImages
		case nyahentai.IsNyahentaiURL(req.URL):
			site = "nyahentai"
			result, runErr := nyahentai.ExecuteWithProgress(session, req.URL, progressSpan(req.Progress, 0.10, 0.25))
			if runErr != nil {
				return BrowserRunResult{}, preferContextError(ctx, runErr)
			}
			siteTitle, resolvedURL, readerURL = result.FinalTitle, result.FinalURL, result.Reader.URL
			pageCount, readerPageCount = result.PageCount, result.PageCount
			readerImageCount, filteredImageCount = len(result.Reader.ImageURLs), len(result.CollectedImages)
			imageURLs = result.CollectedImages
		}
		if len(imageURLs) == 0 {
			return BrowserRunResult{}, fmt.Errorf("%s target images not found", site)
		}
	}
	if len(imageURLs) == 0 {
		return BrowserRunResult{}, fmt.Errorf("%s target images not found", site)
	}
	downloadResult, err = assets.DownloadImagesContext(
		ctx,
		assets.CollectionSummary{Site: site, BaseURL: req.URL, ReaderURL: readerURL, Title: siteTitle},
		imageURLs,
		req.OutputDir,
		assetProgressSpan(req.Progress, downloadStart, downloadSpan),
	)
	if err != nil {
		return BrowserRunResult{}, err
	}
	if len(downloadResult.Files) != len(imageURLs) {
		return BrowserRunResult{}, fmt.Errorf("%s download incomplete: %d/%d files", site, len(downloadResult.Files), len(imageURLs))
	}
	thumbnailPath, err := createTaskThumbnail(runtimePaths, req.TaskID, downloadResult.OutputDir, downloadResult.Files)
	if err != nil {
		return BrowserRunResult{}, fmt.Errorf("%s thumbnail: %w", site, err)
	}

	title, err := session.Title()
	if err != nil {
		log.Printf("session title lookup failed: %v", err)
		title = req.URL
	}
	if strings.TrimSpace(siteTitle) != "" {
		title = siteTitle
	}
	if strings.TrimSpace(resolvedURL) == "" {
		resolvedURL = session.PageURL()
	}
	if req.KeepOpen {
		if err := waitForBrowserCloseOrSignal(ctx, session); err != nil {
			return BrowserRunResult{}, err
		}
	}

	return BrowserRunResult{
		URL:                  req.URL,
		ResolvedURL:          resolvedURL,
		Title:                title,
		BrowserType:          req.BrowserType,
		BrowserPath:          req.BrowserPath,
		BrowserMode:          "playwright-persistent",
		Headless:             req.Headless,
		KeepOpen:             req.KeepOpen,
		PlaywrightProfileDir: req.UserDataDir,
		Site:                 site,
		PageType:             "content",
		ReaderURL:            readerURL,
		SummaryPageCount:     pageCount,
		ReaderPageCount:      readerPageCount,
		ReaderImageCount:     readerImageCount,
		ReaderFilteredCount:  filteredImageCount,
		ReaderActivation:     zeriResult.ActivationClicks,
		Verified:             true,
		VerificationNeeded:   false,
		Blocked:              false,
		MatchedMarker:        "",
		Note:                 "",
		DownloadedCount:      len(downloadResult.Files),
		DownloadedBytes:      downloadResult.Bytes,
		DownloadedDir:        downloadResult.OutputDir,
		ThumbnailPath:        thumbnailPath,
	}, nil
}

func runHitomiRequest(ctx context.Context, req BrowserLaunchRequest, runtimePaths projectruntime.Paths) (BrowserRunResult, error) {
	log.Printf("hitomi request start: url=%s output=%s", req.URL, req.OutputDir)
	result, err := hitomi.ExecuteWithProgress(ctx, req.URL, progressSpan(req.Progress, 0.02, 0.08))
	if err != nil {
		return BrowserRunResult{}, err
	}
	if len(result.CollectedImages) == 0 {
		return BrowserRunResult{}, fmt.Errorf("hitomi target images not found")
	}
	downloadResult, err := assets.DownloadImagesContext(
		ctx,
		assets.CollectionSummary{
			Site:    "hitomi",
			BaseURL: result.FinalURL,
			Title:   result.FinalTitle,
		},
		result.CollectedImages,
		req.OutputDir,
		assetProgressSpan(req.Progress, 0.10, 0.90),
	)
	if err != nil {
		return BrowserRunResult{}, err
	}
	if len(downloadResult.Files) == 0 {
		return BrowserRunResult{}, fmt.Errorf("hitomi download completed without files")
	}
	thumbnailPath, err := createTaskThumbnail(runtimePaths, req.TaskID, downloadResult.OutputDir, downloadResult.Files)
	if err != nil {
		return BrowserRunResult{}, fmt.Errorf("hitomi thumbnail: %w", err)
	}
	return BrowserRunResult{
		URL:                 req.URL,
		ResolvedURL:         result.FinalURL,
		Title:               result.FinalTitle,
		BrowserType:         req.BrowserType,
		BrowserMode:         "http",
		Headless:            true,
		Site:                "hitomi",
		PageType:            "content",
		ReaderURL:           result.FinalURL,
		SummaryPageCount:    result.PageCount,
		ReaderPageCount:     result.PageCount,
		ReaderImageCount:    len(result.CollectedImages),
		ReaderFilteredCount: len(result.CollectedImages),
		Verified:            true,
		DownloadedCount:     len(downloadResult.Files),
		DownloadedBytes:     downloadResult.Bytes,
		DownloadedDir:       downloadResult.OutputDir,
		ThumbnailPath:       thumbnailPath,
	}, nil
}

func assetProgressSpan(cb func(zeri.DownloadProgress), start, span float64) assets.DownloadProgressFunc {
	if cb == nil {
		return nil
	}
	return func(update assets.DownloadProgress) {
		if update.Fraction < 0 {
			update.Fraction = 0
		}
		if update.Fraction > 1 {
			update.Fraction = 1
		}
		cb(zeri.DownloadProgress{
			Current:  update.Current,
			Total:    update.Total,
			Phase:    update.Phase,
			Message:  update.Message,
			Fraction: start + span*update.Fraction,
		})
	}
}

func createTaskThumbnail(runtimePaths projectruntime.Paths, taskID, outputDir string, files []string) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no downloaded files")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		taskID = strings.TrimSpace(filepath.Base(outputDir))
	}
	if taskID == "" || taskID == "." {
		taskID = "task"
	}
	thumbPath := runtimePaths.TaskThumbnailPath(taskID)
	source := zeri.SelectThumbnailSource(files)
	if source == "" {
		source = files[0]
	}
	log.Printf("task thumbnail source: task=%s source=%s", taskID, source)
	if err := zeri.CreateJPGThumbnail(source, thumbPath, 256); err != nil {
		return "", err
	}
	return thumbPath, nil
}

func openTaskBrowserSession(req BrowserLaunchRequest) (taskBrowserSession, error) {
	session, err := req.FirefoxMiddleware().Open(req.BrowserOptions())
	if err != nil {
		return nil, err
	}
	return session, nil
}

type taskBrowserSession interface {
	Close() error
	Title() (string, error)
	WaitClosed() error
	PageURL() string
	Content() (string, error)
	Goto(string) error
	ClickText(string) error
	LoadLazyContent() error
	LoadLazyContentForCount(expectedImageCount int) error
}

func absolutePathOrClean(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func progressSpan(cb func(zeri.DownloadProgress), start, span float64) func(zeri.DownloadProgress) {
	if cb == nil {
		return nil
	}
	return func(update zeri.DownloadProgress) {
		if update.Fraction < 0 {
			update.Fraction = 0
		}
		if update.Fraction > 1 {
			update.Fraction = 1
		}
		update.Fraction = start + span*update.Fraction
		cb(update)
	}
}

func waitForBrowserCloseOrSignal(ctx context.Context, session taskBrowserSession) error {
	return waitForBrowserCloseOrSignalWithTimeout(ctx, session, 5*time.Second)
}

func waitForBrowserCloseOrSignalWithTimeout(ctx context.Context, session taskBrowserSession, closeTimeout time.Duration) error {
	if closeTimeout <= 0 {
		closeTimeout = 5 * time.Second
	}
	waitErr := make(chan error, 1)
	go func() {
		waitErr <- session.WaitClosed()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-waitErr:
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	case <-ctx.Done():
		_ = session.Close()
		select {
		case <-waitErr:
		case <-time.After(closeTimeout):
			log.Printf("browser close wait timed out after cancellation")
		}
		return ctx.Err()
	case sig := <-sigCh:
		log.Printf("browser session interrupted by %s; closing browser and cleaning task temp files", sig)
		_ = session.Close()
		select {
		case err := <-waitErr:
			if err != nil {
				return err
			}
		case <-time.After(closeTimeout):
			return fmt.Errorf("browser session interrupted by %s; close timed out", sig)
		}
		return fmt.Errorf("browser session interrupted by %s", sig)
	}
}

func preferContextError(ctx context.Context, err error) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func logBrowserProfileAudit(browserType, sourceRoot, tempRoot string) {
	sourceRoot = filepath.Clean(strings.TrimSpace(sourceRoot))
	tempRoot = filepath.Clean(strings.TrimSpace(tempRoot))
	if sourceRoot == "" || tempRoot == "" {
		return
	}
	log.Printf("%s profile source: %s", browserType, sourceRoot)
	log.Printf("%s profile temp:   %s", browserType, tempRoot)
	paths := []string{
		"prefs.js",
		"extensions.json",
		"addons.json",
		"addonStartup.json.lz4",
		"parent.lock",
		filepath.Join("Default", "Preferences"),
		filepath.Join("Default", "Secure Preferences"),
		filepath.Join("Default", "Extensions"),
		filepath.Join("Default", "Local Extension Settings"),
		filepath.Join("Default", "Extension Rules"),
		filepath.Join("Default", "Extension Scripts"),
		filepath.Join("Default", "Extension State"),
		filepath.Join("extensions"),
		filepath.Join("browser-extension-data"),
		filepath.Join("storage"),
		filepath.Join("sessionstore-backups"),
	}
	for _, rel := range paths {
		logProfilePathAudit(browserType+" source", filepath.Join(sourceRoot, rel))
		logProfilePathAudit(browserType+" temp", filepath.Join(tempRoot, rel))
	}
}

func logProfilePathAudit(label, path string) {
	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			log.Printf("%s dir: %s (read error: %v)", label, path, readErr)
			return
		}
		log.Printf("%s dir: %s (entries=%d)", label, path, len(entries))
	case err == nil:
		log.Printf("%s file: %s (size=%d)", label, path, info.Size())
	default:
		log.Printf("%s missing: %s (%v)", label, path, err)
	}
}
