package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"comic_downloader_go_playwright_stealth/browser"
	projectruntime "comic_downloader_go_playwright_stealth/runtime"
	"comic_downloader_go_playwright_stealth/siteflow/zeri"
)

const defaultFirefoxUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:149.0) Gecko/20100101 Firefox/149.0"

// BrowserLaunchRequest is the task-level browser input that flows into the middleware.
type BrowserLaunchRequest struct {
	URL                  string                      `json:"url"`
	BrowserType          string                      `json:"browserType"`
	Headless             bool                        `json:"headless"`
	LaunchTimeoutMS      int                         `json:"launchTimeoutMs"`
	RuntimeRoot          string                      `json:"runtimeRoot"`
	BrowserPath          string                      `json:"browserPath"`
	BrowserInstallDir    string                      `json:"browserInstallDir"`
	DriverDir            string                      `json:"driverDir"`
	ProfileDir           string                      `json:"profileDir"`
	UserDataDir          string                      `json:"userDataDir"`
	UserAgent            string                      `json:"userAgent"`
	Locale               string                      `json:"locale"`
	TimezoneID           string                      `json:"timezoneId"`
	ViewportWidth        int                         `json:"viewportWidth"`
	ViewportHeight       int                         `json:"viewportHeight"`
	FirefoxUserPrefsJSON string                      `json:"firefoxUserPrefsJson"`
	FirefoxUserPrefs     map[string]any              `json:"firefoxUserPrefs"`
	DownloadRoot         string                      `json:"downloadRoot"`
	OutputDir            string                      `json:"outputDir"`
	Adblock              bool                        `json:"adblock"`
	KeepOpen             bool                        `json:"keepOpen"`
	WorkerID             string                      `json:"workerId"`
	TaskID               string                      `json:"taskId"`
	Progress             func(zeri.DownloadProgress) `json:"-"`
}

// Normalize returns a cleaned request with defaults applied.
func (r BrowserLaunchRequest) Normalize() BrowserLaunchRequest {
	if strings.TrimSpace(r.RuntimeRoot) == "" {
		r.RuntimeRoot = "runtime"
	}
	if r.LaunchTimeoutMS <= 0 {
		r.LaunchTimeoutMS = 120000
	}
	r.URL = strings.TrimSpace(r.URL)
	r.BrowserType = normalizeBrowserType(r.BrowserType)
	if zeri.IsZeriURL(r.URL) {
		r.BrowserType = string(projectruntime.BrowserTypeFirefox)
	}
	r.BrowserInstallDir = strings.TrimSpace(r.BrowserInstallDir)
	if r.BrowserInstallDir == "" && r.BrowserType == string(projectruntime.BrowserTypeChromium) {
		r.BrowserInstallDir = strings.TrimSpace(os.Getenv("PLAYWRIGHT_BROWSERS_PATH"))
	}
	if strings.TrimSpace(r.DriverDir) == "" {
		if r.BrowserType == string(projectruntime.BrowserTypeChromium) && strings.TrimSpace(r.BrowserInstallDir) != "" {
			r.DriverDir = filepath.Join(r.BrowserInstallDir, "driver")
		} else {
			r.DriverDir = projectruntime.DefaultPlaywrightDriverDir(r.RuntimeRoot)
		}
	}
	selectedProfileDir := strings.TrimSpace(r.ProfileDir)
	userDataDir := strings.TrimSpace(r.UserDataDir)
	r.WorkerID = strings.TrimSpace(r.WorkerID)
	r.TaskID = strings.TrimSpace(r.TaskID)
	r.UserAgent = strings.TrimSpace(r.UserAgent)
	if r.UserAgent == "" {
		r.UserAgent = defaultUserAgentForBrowserType(r.BrowserType)
	}
	if strings.TrimSpace(r.BrowserPath) == "" {
		if r.BrowserType != string(projectruntime.BrowserTypeChromium) {
			r.BrowserPath = defaultExecutablePathForBrowserType(r.RuntimeRoot, r.BrowserType)
		}
	}
	if selectedProfileDir != "" {
		r.ProfileDir = filepath.Clean(selectedProfileDir)
	} else if userDataDir != "" {
		r.ProfileDir = filepath.Clean(userDataDir)
	} else {
		r.ProfileDir = defaultProfileDirForBrowserType(r.BrowserType)
	}
	if userDataDir != "" {
		r.UserDataDir = filepath.Clean(userDataDir)
	} else {
		r.UserDataDir = r.ProfileDir
	}
	if trimmed := strings.TrimSpace(r.DownloadRoot); trimmed != "" {
		r.DownloadRoot = filepath.Clean(trimmed)
	} else {
		r.DownloadRoot = ""
	}
	if trimmed := strings.TrimSpace(r.OutputDir); trimmed != "" {
		r.OutputDir = filepath.Clean(trimmed)
	} else {
		r.OutputDir = ""
	}
	if trimmed := strings.TrimSpace(r.FirefoxUserPrefsJSON); trimmed != "" {
		var prefs map[string]any
		if err := json.Unmarshal([]byte(trimmed), &prefs); err == nil {
			r.FirefoxUserPrefs = prefs
		}
	}
	r.Locale = strings.TrimSpace(r.Locale)
	r.TimezoneID = strings.TrimSpace(r.TimezoneID)
	return r
}

// UsesTaskProfile reports whether the request should clone the current mother profile into a task temp directory.
func (r BrowserLaunchRequest) UsesTaskProfile() bool {
	return strings.TrimSpace(r.WorkerID) != "" && strings.TrimSpace(r.TaskID) != ""
}

// PrepareTaskProfile copies the current mother profile into a task-scoped temporary profile tree.
func (r BrowserLaunchRequest) PrepareTaskProfile() (projectruntime.BrowserTaskProfile, error) {
	r = r.Normalize()
	manager := projectruntime.NewBrowserProfileManager(workspaceRootFromRuntimeRoot(r.RuntimeRoot))
	return manager.PrepareTaskProfileFromSource(projectruntime.BrowserType(r.BrowserType), r.ProfileDir, r.WorkerID, r.TaskID)
}

// CleanupTaskProfile removes the task-scoped temporary profile tree.
func (r BrowserLaunchRequest) CleanupTaskProfile() error {
	r = r.Normalize()
	manager := projectruntime.NewBrowserProfileManager(workspaceRootFromRuntimeRoot(r.RuntimeRoot))
	return manager.CleanupTaskProfile(projectruntime.BrowserType(r.BrowserType), r.WorkerID, r.TaskID)
}

// BrowserOptions converts the request into browser-layer launch options.
func (r BrowserLaunchRequest) BrowserOptions() browser.BrowserSessionOptions {
	return browser.BrowserSessionOptions{
		URL:               r.URL,
		Headless:          browser.HeadlessPtr(r.Headless),
		LaunchTimeoutMS:   r.LaunchTimeoutMS,
		DriverDir:         r.DriverDir,
		ProfileDir:        r.ProfileDir,
		BrowserInstallDir: r.BrowserInstallDir,
		UserAgent:         r.UserAgent,
		Locale:            r.Locale,
		TimezoneID:        r.TimezoneID,
		ViewportWidth:     r.ViewportWidth,
		ViewportHeight:    r.ViewportHeight,
		FirefoxUserPrefs:  r.FirefoxUserPrefs,
	}
}

// FirefoxMiddleware builds the browser middleware from the task-level request.
func (r BrowserLaunchRequest) FirefoxMiddleware() browser.FirefoxMiddleware {
	r = r.Normalize()
	middleware := browser.NewFirefoxMiddleware(r.URL).
		WithRuntimeRoot(r.RuntimeRoot).
		WithBrowserPath(r.BrowserPath).
		WithBrowserInstallDir(r.BrowserInstallDir).
		WithDriverDir(r.DriverDir).
		WithLaunchTimeoutMS(r.LaunchTimeoutMS).
		WithProfileDir(r.ProfileDir).
		WithUserDataDir(r.UserDataDir).
		WithUserAgent(r.UserAgent).
		WithLocale(r.Locale).
		WithTimezoneID(r.TimezoneID).
		WithViewport(r.ViewportWidth, r.ViewportHeight).
		WithFirefoxUserPrefs(r.FirefoxUserPrefs).
		WithDownloadRoot(r.DownloadRoot).
		WithOutputDir(r.OutputDir).
		WithHeadless(r.Headless).
		WithAdblock(r.Adblock)
	return middleware
}

// ChromiumMiddleware builds the chromium browser middleware from the task-level request.
func (r BrowserLaunchRequest) ChromiumMiddleware() browser.ChromiumMiddleware {
	r = r.Normalize()
	middleware := browser.NewChromiumMiddleware(r.URL).
		WithRuntimeRoot(r.RuntimeRoot).
		WithBrowserPath(r.BrowserPath).
		WithBrowserInstallDir(r.BrowserInstallDir).
		WithDriverDir(r.DriverDir).
		WithLaunchTimeoutMS(r.LaunchTimeoutMS).
		WithProfileDir(r.ProfileDir).
		WithUserDataDir(r.UserDataDir).
		WithUserAgent(r.UserAgent).
		WithLocale(r.Locale).
		WithTimezoneID(r.TimezoneID).
		WithViewport(r.ViewportWidth, r.ViewportHeight).
		WithDownloadRoot(r.DownloadRoot).
		WithOutputDir(r.OutputDir).
		WithHeadless(r.Headless).
		WithAdblock(r.Adblock)
	return middleware
}

func normalizeBrowserType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(projectruntime.BrowserTypeFirefox):
		return string(projectruntime.BrowserTypeFirefox)
	case string(projectruntime.BrowserTypeChromium):
		return string(projectruntime.BrowserTypeChromium)
	default:
		return string(projectruntime.BrowserTypeFirefox)
	}
}

func defaultProfileDirForBrowserType(browserType string) string {
	switch browserType {
	case string(projectruntime.BrowserTypeChromium):
		return projectruntime.DefaultChromiumProfileSourceDir()
	default:
		return projectruntime.DefaultFirefoxProfileDir()
	}
}

func defaultExecutablePathForBrowserType(runtimeRoot, browserType string) string {
	switch browserType {
	case string(projectruntime.BrowserTypeChromium):
		return ""
	default:
		return projectruntime.DefaultFirefoxExecutablePath(runtimeRoot)
	}
}

func defaultUserAgentForBrowserType(browserType string) string {
	switch browserType {
	case string(projectruntime.BrowserTypeChromium):
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:149.0) Gecko/20100101 Chrome/149.0"
	default:
		return defaultFirefoxUserAgent
	}
}

func workspaceRootFromRuntimeRoot(runtimeRoot string) string {
	runtimeRoot = filepath.Clean(strings.TrimSpace(runtimeRoot))
	if runtimeRoot == "" || runtimeRoot == "." {
		return "."
	}
	return filepath.Dir(runtimeRoot)
}
