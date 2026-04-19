package ui

import projectruntime "comic_downloader_go_playwright_stealth/runtime"

// BrowserMenuState is the top-bar browser selection surface used by the frontend.
type BrowserMenuState struct {
	SelectedBrowser          string
	FirefoxExecutablePath    string
	FirefoxInstallRoot       string
	ChromiumExecutablePath   string
	ChromiumInstallRoot      string
	PlaywrightDriverDir      string
	FirefoxMotherProfileDir  string
	FirefoxWorkingProfileDir string
}

// DefaultBrowserMenuState returns the current browser selection defaults for the frontend.
func DefaultBrowserMenuState() BrowserMenuState {
	paths := projectruntime.NewPaths(".")
	return BrowserMenuState{
		SelectedBrowser:          "firefox",
		FirefoxExecutablePath:    projectruntime.DefaultFirefoxExecutablePath(paths.Root),
		FirefoxInstallRoot:       projectruntime.DefaultFirefoxInstallDir(paths.Root),
		ChromiumExecutablePath:   "",
		ChromiumInstallRoot:      projectruntime.DefaultChromiumInstallDir(paths.Root),
		PlaywrightDriverDir:      projectruntime.DefaultPlaywrightDriverDir(paths.Root),
		FirefoxMotherProfileDir:  projectruntime.DefaultFirefoxProfileSourceDir(),
		FirefoxWorkingProfileDir: projectruntime.DefaultFirefoxProfileDir(),
	}
}

// WithFirefoxExecutablePath updates the Firefox executable path shown in the top menu.
func (m BrowserMenuState) WithFirefoxExecutablePath(executablePath string) BrowserMenuState {
	m.FirefoxExecutablePath = executablePath
	return m
}

// WithFirefoxInstallRoot updates the Playwright Firefox install directory.
func (m BrowserMenuState) WithFirefoxInstallRoot(installRoot string) BrowserMenuState {
	m.FirefoxInstallRoot = installRoot
	return m
}

// WithChromiumExecutablePath updates the Playwright Chromium executable path.
func (m BrowserMenuState) WithChromiumExecutablePath(executablePath string) BrowserMenuState {
	m.ChromiumExecutablePath = executablePath
	return m
}

// WithChromiumInstallRoot updates the Playwright Chromium install directory.
func (m BrowserMenuState) WithChromiumInstallRoot(installRoot string) BrowserMenuState {
	m.ChromiumInstallRoot = installRoot
	return m
}

// WithPlaywrightDriverDir updates the Playwright driver directory.
func (m BrowserMenuState) WithPlaywrightDriverDir(driverDir string) BrowserMenuState {
	m.PlaywrightDriverDir = driverDir
	return m
}

// WithFirefoxMotherProfileDir updates the selected Firefox mother profile directory.
func (m BrowserMenuState) WithFirefoxMotherProfileDir(profileDir string) BrowserMenuState {
	m.FirefoxMotherProfileDir = profileDir
	return m
}

// WithFirefoxWorkingProfileDir updates the project-owned Firefox working profile directory.
func (m BrowserMenuState) WithFirefoxWorkingProfileDir(profileDir string) BrowserMenuState {
	m.FirefoxWorkingProfileDir = profileDir
	return m
}

// WithSelectedBrowser updates the browser picked in the top menu.
func (m BrowserMenuState) WithSelectedBrowser(browser string) BrowserMenuState {
	m.SelectedBrowser = browser
	return m
}
