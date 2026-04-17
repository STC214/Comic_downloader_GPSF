package ui

import projectruntime "comic_downloader_go_playwright_stealth/runtime"

// BrowserMenuState is the top-bar browser selection surface used by the frontend.
type BrowserMenuState struct {
	SelectedBrowser          string
	FirefoxExecutablePath    string
	FirefoxMotherProfileDir  string
	FirefoxWorkingProfileDir string
}

// DefaultBrowserMenuState returns the current browser selection defaults for the frontend.
func DefaultBrowserMenuState() BrowserMenuState {
	paths := projectruntime.NewPaths(".")
	return BrowserMenuState{
		SelectedBrowser:          "firefox",
		FirefoxExecutablePath:    projectruntime.DefaultFirefoxExecutablePath(paths.Root),
		FirefoxMotherProfileDir:  projectruntime.DefaultFirefoxProfileSourceDir(),
		FirefoxWorkingProfileDir: projectruntime.DefaultFirefoxProfileDir(),
	}
}

// WithFirefoxExecutablePath updates the Firefox executable path shown in the top menu.
func (m BrowserMenuState) WithFirefoxExecutablePath(executablePath string) BrowserMenuState {
	m.FirefoxExecutablePath = executablePath
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
