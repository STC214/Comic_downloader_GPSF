package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	projectruntime "comic_downloader_go_playwright_stealth/runtime"
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
}

// RunBrowserRequest opens the page described by the request and returns a normalized result.
func RunBrowserRequest(req BrowserLaunchRequest) (BrowserRunResult, error) {
	req = req.Normalize()
	if strings.TrimSpace(req.URL) == "" {
		return BrowserRunResult{}, fmt.Errorf("browser url is empty")
	}

	manager := projectruntime.NewBrowserProfileManager(workspaceRootFromRuntimeRoot(req.RuntimeRoot))
	var cleanupProfile func()
	sourceProfileDir := ""
	activeProfileDir := ""

	if req.BrowserType == string(projectruntime.BrowserTypeFirefox) {
		profile, err := manager.PrepareFreshPlaywrightProfile(projectruntime.BrowserType(req.BrowserType))
		if err != nil {
			return BrowserRunResult{}, err
		}
		req.UserDataDir = absolutePathOrClean(profile.RootDir)
		activeProfileDir = req.UserDataDir
		cleanupProfile = func() {
			_ = manager.CleanupFreshPlaywrightProfile(profile)
		}
	} else if req.UsesTaskProfile() {
		profile, err := req.PrepareTaskProfile()
		if err != nil {
			return BrowserRunResult{}, err
		}
		sourceProfileDir = profile.MotherProfileDir
		req.UserDataDir = absolutePathOrClean(profile.ContentUserData)
		activeProfileDir = req.UserDataDir
		cleanupProfile = func() {
			_ = req.CleanupTaskProfile()
		}
	} else {
		profile, err := manager.PreparePlaywrightProfileFromSource(projectruntime.BrowserType(req.BrowserType), req.ProfileDir)
		if err != nil {
			return BrowserRunResult{}, err
		}
		sourceProfileDir = profile.SourceProfileDir
		req.UserDataDir = absolutePathOrClean(profile.RootDir)
		activeProfileDir = req.UserDataDir
		cleanupProfile = func() {
			_ = manager.CleanupPlaywrightProfile(profile)
		}
	}

	if sourceProfileDir != "" || activeProfileDir != "" {
		fmt.Printf("profile flow: source=%s temp=%s output=%s\n", sourceProfileDir, activeProfileDir, req.OutputDir)
		logBrowserProfileAudit(req.BrowserType, sourceProfileDir, activeProfileDir)
	}

	if req.Progress != nil {
		req.Progress(zeri.DownloadProgress{Fraction: 0.02, Phase: "启动", Message: "准备"})
	}

	session, err := openTaskBrowserSession(req)
	if err != nil {
		if cleanupProfile != nil {
			cleanupProfile()
		}
		return BrowserRunResult{}, err
	}
	if req.Progress != nil {
		req.Progress(zeri.DownloadProgress{Fraction: 0.08, Phase: "启动", Message: "完成"})
	}
	defer func() {
		_ = session.Close()
		if cleanupProfile != nil {
			cleanupProfile()
		}
	}()

	var zeriResult zeri.ExecutionResult
	var downloadResult zeri.DownloadResult
	site := ""
	if zeri.IsZeriURL(req.URL) {
		site = "zeri"
		if req.Progress != nil {
			req.Progress(zeri.DownloadProgress{Fraction: 0.10, Phase: "解析", Message: "摘要"})
		}
		zeriResult, err = zeri.ExecuteWithProgress(session, req.URL, progressSpan(req.Progress, 0.10, 0.90))
		if err != nil {
			return BrowserRunResult{}, err
		}
		if strings.TrimSpace(req.OutputDir) != "" && len(zeriResult.CollectedImages) > 0 {
			downloadWeight := zeri.DownloadWeightForCount(zeriResult.Summary.PageCount)
			parseWeight := 1 - downloadWeight
			if parseWeight < 0 {
				parseWeight = 0
			}
			downloadStart := 0.10 + 0.90*parseWeight
			downloadSpan := 0.90 * downloadWeight
			downloadResult, err = zeri.DownloadImages(
				zeriResult.Summary,
				zeriResult.CollectedImages,
				req.OutputDir,
				progressSpan(req.Progress, downloadStart, downloadSpan),
			)
			if err != nil {
				return BrowserRunResult{}, err
			}
		}
	}

	title, err := session.Title()
	if err != nil {
		return BrowserRunResult{}, err
	}
	if req.KeepOpen {
		if err := session.WaitClosed(); err != nil {
			return BrowserRunResult{}, err
		}
	}

	return BrowserRunResult{
		URL:                  req.URL,
		ResolvedURL:          session.PageURL(),
		Title:                title,
		BrowserType:          req.BrowserType,
		BrowserPath:          req.BrowserPath,
		BrowserMode:          "playwright-persistent",
		Headless:             req.Headless,
		KeepOpen:             req.KeepOpen,
		PlaywrightProfileDir: req.UserDataDir,
		Site:                 site,
		PageType:             "content",
		ReaderURL:            zeriResult.Reader.URL,
		SummaryPageCount:     zeriResult.Summary.PageCount,
		ReaderPageCount:      zeriResult.Reader.PageCount,
		ReaderImageCount:     len(zeriResult.Reader.ImageURLs),
		ReaderFilteredCount:  len(zeriResult.CollectedImages),
		ReaderActivation:     zeriResult.ActivationClicks,
		Verified:             true,
		VerificationNeeded:   false,
		Blocked:              false,
		MatchedMarker:        "",
		Note:                 "",
		DownloadedCount:      len(downloadResult.Files),
		DownloadedBytes:      downloadResult.Bytes,
		DownloadedDir:        downloadResult.OutputDir,
	}, nil
}

func openTaskBrowserSession(req BrowserLaunchRequest) (taskBrowserSession, error) {
	switch projectruntime.BrowserType(req.BrowserType) {
	case projectruntime.BrowserTypeChromium:
		session, err := req.ChromiumMiddleware().Open(req.BrowserOptions())
		if err != nil {
			return nil, err
		}
		return session, nil
	default:
		session, err := req.FirefoxMiddleware().Open(req.BrowserOptions())
		if err != nil {
			return nil, err
		}
		return session, nil
	}
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

func logBrowserProfileAudit(browserType, sourceRoot, tempRoot string) {
	sourceRoot = filepath.Clean(strings.TrimSpace(sourceRoot))
	tempRoot = filepath.Clean(strings.TrimSpace(tempRoot))
	if sourceRoot == "" || tempRoot == "" {
		return
	}
	fmt.Printf("%s profile source: %s\n", browserType, sourceRoot)
	fmt.Printf("%s profile temp:   %s\n", browserType, tempRoot)
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
			fmt.Printf("%s dir: %s (read error: %v)\n", label, path, readErr)
			return
		}
		fmt.Printf("%s dir: %s (entries=%d)\n", label, path, len(entries))
	case err == nil:
		fmt.Printf("%s file: %s (size=%d)\n", label, path, info.Size())
	default:
		fmt.Printf("%s missing: %s (%v)\n", label, path, err)
	}
}
