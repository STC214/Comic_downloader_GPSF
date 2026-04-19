package ui

import (
	"path/filepath"
	"strings"
	"time"

	"comic_downloader_go_playwright_stealth/runtime"
)

// BrowserInstallMiddleware coordinates Playwright browser installation actions for the frontend.
type BrowserInstallMiddleware struct {
	WorkspaceRoot string
}

// NewBrowserInstallMiddleware builds a browser installation middleware for one workspace.
func NewBrowserInstallMiddleware(workspaceRoot string) BrowserInstallMiddleware {
	return BrowserInstallMiddleware{WorkspaceRoot: workspaceRoot}
}

// InstallChromium installs the Playwright-managed Chromium runtime into targetRoot.
func (m BrowserInstallMiddleware) InstallChromium(targetRoot string) (runtime.BrowserInstallResult, error) {
	return runtime.NewBrowserInstallManager(m.WorkspaceRoot).InstallPlaywrightBrowser(runtime.BrowserTypeChromium, targetRoot)
}

// InstallFirefox installs the Playwright-managed Firefox runtime into targetRoot.
func (m BrowserInstallMiddleware) InstallFirefox(targetRoot string) (runtime.BrowserInstallResult, error) {
	return runtime.NewBrowserInstallManager(m.WorkspaceRoot).InstallPlaywrightBrowser(runtime.BrowserTypeFirefox, targetRoot)
}

// InstallBrowser installs the selected Playwright-managed browser into targetRoot.
func (m BrowserInstallMiddleware) InstallBrowser(browserType runtime.BrowserType, targetRoot string) (runtime.BrowserInstallResult, error) {
	return runtime.NewBrowserInstallManager(m.WorkspaceRoot).InstallPlaywrightBrowser(browserType, targetRoot)
}

// InstallChromiumWithProgress installs Chromium and reports progress updates.
func (m BrowserInstallMiddleware) InstallChromiumWithProgress(targetRoot string, progress func(runtime.BrowserInstallProgress)) (runtime.BrowserInstallResult, error) {
	return runtime.NewBrowserInstallManager(m.WorkspaceRoot).InstallPlaywrightBrowserWithProgress(runtime.BrowserTypeChromium, targetRoot, progress)
}

// InstallFirefoxWithProgress installs Firefox and reports progress updates.
func (m BrowserInstallMiddleware) InstallFirefoxWithProgress(targetRoot string, progress func(runtime.BrowserInstallProgress)) (runtime.BrowserInstallResult, error) {
	return runtime.NewBrowserInstallManager(m.WorkspaceRoot).InstallPlaywrightBrowserWithProgress(runtime.BrowserTypeFirefox, targetRoot, progress)
}

// InstallAllBrowsers installs both Chromium and Firefox into the same target root.
func (m BrowserInstallMiddleware) InstallAllBrowsers(targetRoot string) (runtime.BrowserInstallBatchResult, error) {
	return runtime.NewBrowserInstallManager(m.WorkspaceRoot).InstallPlaywrightBrowsers(targetRoot)
}

// InstallAllBrowsersWithProgress installs Chromium and Firefox and reports progress updates.
func (m BrowserInstallMiddleware) InstallAllBrowsersWithProgress(targetRoot string, progress func(runtime.BrowserInstallProgress)) (runtime.BrowserInstallBatchResult, error) {
	return runtime.NewBrowserInstallManager(m.WorkspaceRoot).InstallPlaywrightBrowsersWithProgress(targetRoot, progress)
}

// ApplyBrowserInstallResult updates the frontend browser menu state from installed browser results.
func (m BrowserInstallMiddleware) ApplyBrowserInstallResult(state BrowserMenuState, result runtime.BrowserInstallBatchResult) BrowserMenuState {
	driverDir := filepath.Join(result.TargetRoot, "driver")
	for _, item := range result.Results {
		switch item.BrowserType {
		case runtime.BrowserTypeChromium:
			state = state.WithChromiumInstallRoot(result.TargetRoot).WithChromiumExecutablePath(item.ExecutablePath)
		case runtime.BrowserTypeFirefox:
			state = state.WithFirefoxInstallRoot(result.TargetRoot).WithFirefoxExecutablePath(item.ExecutablePath)
		}
		if strings.TrimSpace(item.DriverDirectory) != "" {
			driverDir = item.DriverDirectory
		}
	}
	return state.WithPlaywrightDriverDir(driverDir)
}

// InstallAllBrowsersAndApply installs both browsers and returns the updated frontend menu state.
func (m BrowserInstallMiddleware) InstallAllBrowsersAndApply(state BrowserMenuState, targetRoot string, progress func(runtime.BrowserInstallProgress)) (BrowserMenuState, runtime.BrowserInstallBatchResult, error) {
	result, err := m.InstallAllBrowsersWithProgress(targetRoot, progress)
	if err != nil {
		return state, runtime.BrowserInstallBatchResult{}, err
	}
	return m.ApplyBrowserInstallResult(state, result), result, nil
}

// BrowserInstallProgress describes the current phase of a browser installation.
type BrowserInstallProgress struct {
	Fraction float64
	Phase    string
	Message  string
	Browser  runtime.BrowserType
	Target   string
	Elapsed  time.Duration
}
