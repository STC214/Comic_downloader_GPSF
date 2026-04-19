package ui

import projectruntime "comic_downloader_go_playwright_stealth/runtime"

// BrowserMenuState is the top-bar browser selection surface used by the frontend.
type BrowserMenuState struct {
	SelectedBrowser    string
	FirefoxProfileDir  string
	ChromiumProfileDir string
}

// DefaultBrowserMenuState returns the current browser selection defaults for the frontend.
func DefaultBrowserMenuState() BrowserMenuState {
	firefoxProfileDir := projectruntime.DefaultFirefoxProfileDir()
	return BrowserMenuState{
		SelectedBrowser:    "firefox",
		FirefoxProfileDir:  firefoxProfileDir,
		ChromiumProfileDir: projectruntime.DefaultChromiumUserDataDir("runtime"),
	}
}

// WithFirefoxProfileDir updates the selected Firefox profile directory.
func (m BrowserMenuState) WithFirefoxProfileDir(profileDir string) BrowserMenuState {
	m.FirefoxProfileDir = profileDir
	return m
}

// WithSelectedBrowser updates the browser picked in the top menu.
func (m BrowserMenuState) WithSelectedBrowser(browser string) BrowserMenuState {
	m.SelectedBrowser = browser
	return m
}
