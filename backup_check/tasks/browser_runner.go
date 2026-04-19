package tasks

import (
	"fmt"
	"path/filepath"
	"strings"

	projectruntime "comic_downloader_go_playwright_stealth/runtime"
)

// BrowserRunResult is the task-layer outcome of opening a browser page.
type BrowserRunResult struct {
	URL                  string `json:"url"`
	Title                string `json:"title"`
	Headless             bool   `json:"headless"`
	KeepOpen             bool   `json:"keepOpen"`
	PlaywrightProfileDir string `json:"playwrightProfileDir,omitempty"`
}

// RunBrowserRequest opens the page described by the request and returns a normalized result.
func RunBrowserRequest(req BrowserLaunchRequest) (BrowserRunResult, error) {
	req = req.Normalize()
	if strings.TrimSpace(req.URL) == "" {
		return BrowserRunResult{}, fmt.Errorf("browser url is empty")
	}
	manager := projectruntime.NewBrowserProfileManager(req.RuntimeRoot)
	var cleanupProfile func()
	if req.UsesTaskProfile() {
		profile, err := req.PrepareTaskProfile()
		if err != nil {
			return BrowserRunResult{}, err
		}
		req.UserDataDir = absolutePathOrClean(profile.ContentUserData)
		cleanupProfile = func() {
			_ = req.CleanupTaskProfile()
		}
	} else {
		profile, err := manager.PreparePlaywrightProfileFromSource(projectruntime.BrowserTypeFirefox, req.ProfileDir)
		if err != nil {
			return BrowserRunResult{}, err
		}
		req.UserDataDir = absolutePathOrClean(profile.RootDir)
		cleanupProfile = func() {
			_ = manager.CleanupPlaywrightProfile(profile)
		}
	}
	if req.UserDataDir != "" {
		fmt.Printf("playwright profile dir: %s\n", req.UserDataDir)
	}
	middleware := req.FirefoxMiddleware()
	session, err := middleware.Open(req.BrowserOptions())
	if err != nil {
		if cleanupProfile != nil {
			cleanupProfile()
		}
		return BrowserRunResult{}, err
	}
	defer func() {
		_ = session.Close()
		if cleanupProfile != nil {
			cleanupProfile()
		}
	}()

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
		Title:                title,
		Headless:             req.Headless,
		KeepOpen:             req.KeepOpen,
		PlaywrightProfileDir: req.UserDataDir,
	}, nil
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
