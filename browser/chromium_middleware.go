package browser

import (
	"os"
	"path/filepath"
	"strings"

	projectruntime "comic_downloader_go_playwright_stealth/runtime"
)

// ChromiumMiddleware carries the Chromium browser-side state for one URL.
type ChromiumMiddleware struct {
	url               string
	runtimeRoot       string
	downloadRoot      string
	outputDir         string
	driverDir         string
	launchTimeoutMS   int
	profileDir        string
	userDataDir       string
	userAgent         string
	locale            string
	timezoneID        string
	viewportWidth     int
	viewportHeight    int
	browserPath       string
	browserInstallDir string
	headless          bool
	adblock           bool
}

// LaunchData returns the Chromium launch inputs needed by the Playwright-backed implementation.
func (m ChromiumMiddleware) LaunchData(opts BrowserSessionOptions) LaunchData {
	spec := m.LaunchSpec(opts)
	return LaunchData{
		ExecutablePath: spec.BrowserPath,
		Headless:       m.resolveHeadless(opts),
	}
}

// ContextData returns the Chromium context inputs needed by the Playwright-backed implementation.
func (m ChromiumMiddleware) ContextData(opts BrowserSessionOptions) ContextData {
	return ContextData{BaseURL: m.resolveURL(opts)}
}

// NewChromiumMiddleware builds a Chromium-only browser middleware state object.
func NewChromiumMiddleware(url string) ChromiumMiddleware {
	return ChromiumMiddleware{
		url:               strings.TrimSpace(url),
		runtimeRoot:       "runtime",
		downloadRoot:      "",
		outputDir:         "",
		driverDir:         "",
		launchTimeoutMS:   120000,
		profileDir:        "",
		userDataDir:       "",
		userAgent:         "",
		locale:            "",
		timezoneID:        "",
		viewportWidth:     1365,
		viewportHeight:    768,
		browserPath:       "",
		browserInstallDir: "",
		headless:          true,
		adblock:           true,
	}
}

// WithRuntimeRoot sets the runtime root used to resolve chrome_stealth.js.
func (m ChromiumMiddleware) WithRuntimeRoot(runtimeRoot string) ChromiumMiddleware {
	m.runtimeRoot = normalizePath(runtimeRoot)
	if m.runtimeRoot == "" {
		m.runtimeRoot = "runtime"
	}
	return m
}

// WithDownloadRoot sets the output/download root used by the worker layer.
func (m ChromiumMiddleware) WithDownloadRoot(downloadRoot string) ChromiumMiddleware {
	m.downloadRoot = normalizePath(downloadRoot)
	return m
}

// WithOutputDir sets the resolved output directory.
func (m ChromiumMiddleware) WithOutputDir(outputDir string) ChromiumMiddleware {
	m.outputDir = normalizePath(outputDir)
	return m
}

// WithDriverDir sets the Playwright driver directory.
func (m ChromiumMiddleware) WithDriverDir(driverDir string) ChromiumMiddleware {
	m.driverDir = normalizePath(driverDir)
	return m
}

// WithLaunchTimeoutMS sets the maximum time to wait for Playwright to start the browser.
func (m ChromiumMiddleware) WithLaunchTimeoutMS(launchTimeoutMS int) ChromiumMiddleware {
	if launchTimeoutMS >= 0 {
		m.launchTimeoutMS = launchTimeoutMS
	}
	return m
}

// WithUserAgent sets the browser user agent for the launched context.
func (m ChromiumMiddleware) WithUserAgent(userAgent string) ChromiumMiddleware {
	m.userAgent = strings.TrimSpace(userAgent)
	return m
}

// WithProfileDir sets the selected profile directory.
func (m ChromiumMiddleware) WithProfileDir(profileDir string) ChromiumMiddleware {
	m.profileDir = normalizePath(profileDir)
	return m
}

// WithUserDataDir sets the Chromium user data directory.
func (m ChromiumMiddleware) WithUserDataDir(userDataDir string) ChromiumMiddleware {
	m.userDataDir = normalizePath(userDataDir)
	return m
}

// WithLocale sets the browser locale for a stable testing environment.
func (m ChromiumMiddleware) WithLocale(locale string) ChromiumMiddleware {
	m.locale = strings.TrimSpace(locale)
	return m
}

// WithTimezoneID sets the browser timezone for a stable testing environment.
func (m ChromiumMiddleware) WithTimezoneID(timezoneID string) ChromiumMiddleware {
	m.timezoneID = strings.TrimSpace(timezoneID)
	return m
}

// WithViewport sets the browser viewport for a stable testing environment.
func (m ChromiumMiddleware) WithViewport(width, height int) ChromiumMiddleware {
	if width > 0 {
		m.viewportWidth = width
	}
	if height > 0 {
		m.viewportHeight = height
	}
	return m
}

// WithBrowserPath sets the Chromium executable path.
func (m ChromiumMiddleware) WithBrowserPath(browserPath string) ChromiumMiddleware {
	m.browserPath = normalizePath(browserPath)
	return m
}

// WithBrowserInstallDir sets the Chromium install directory used to resolve chrome.exe.
func (m ChromiumMiddleware) WithBrowserInstallDir(browserInstallDir string) ChromiumMiddleware {
	m.browserInstallDir = normalizePath(browserInstallDir)
	return m
}

// WithHeadless switches the launch mode.
func (m ChromiumMiddleware) WithHeadless(headless bool) ChromiumMiddleware {
	m.headless = headless
	return m
}

// WithAdblock switches adblock injection on or off.
func (m ChromiumMiddleware) WithAdblock(adblock bool) ChromiumMiddleware {
	m.adblock = adblock
	return m
}

func (m ChromiumMiddleware) URL() string {
	return m.url
}

func (m ChromiumMiddleware) BrowserType() BrowserType {
	return BrowserTypeChromium
}

func (m ChromiumMiddleware) RuntimeRoot() string {
	if strings.TrimSpace(m.runtimeRoot) == "" {
		return "runtime"
	}
	return m.runtimeRoot
}

func (m ChromiumMiddleware) BrowserPath() string {
	if trimmed := strings.TrimSpace(m.browserPath); trimmed != "" {
		return trimmed
	}
	installRoot := strings.TrimSpace(m.browserInstallDir)
	if installRoot == "" {
		if envRoot := strings.TrimSpace(os.Getenv("PLAYWRIGHT_BROWSERS_PATH")); envRoot != "" {
			installRoot = envRoot
		} else {
			installRoot = projectruntime.DefaultChromiumInstallDir(m.RuntimeRoot())
		}
	}
	if resolved, err := projectruntime.ResolveInstalledBrowserExecutable(installRoot, projectruntime.BrowserTypeChromium); err == nil {
		return resolved
	}
	if strings.HasSuffix(strings.ToLower(installRoot), ".exe") {
		return filepath.Clean(installRoot)
	}
	return projectruntime.DefaultChromiumExecutablePath(m.RuntimeRoot())
}

func (m ChromiumMiddleware) StealthScript() ScriptRef {
	return resolveRuntimeScript(m.RuntimeRoot(), "chrome_stealth.js")
}

func (m ChromiumMiddleware) ProfileDir() string {
	return strings.TrimSpace(m.profileDir)
}

func (m ChromiumMiddleware) resolveProfileDir(opts BrowserSessionOptions) string {
	if trimmed := strings.TrimSpace(opts.ProfileDir); trimmed != "" {
		return filepath.Clean(trimmed)
	}
	return m.ProfileDir()
}

func (m ChromiumMiddleware) resolveURL(opts BrowserSessionOptions) string {
	if trimmed := strings.TrimSpace(opts.URL); trimmed != "" {
		return trimmed
	}
	return m.URL()
}

func (m ChromiumMiddleware) resolveHeadless(opts BrowserSessionOptions) bool {
	if opts.Headless != nil {
		return *opts.Headless
	}
	return m.headless
}

func (m ChromiumMiddleware) resolveUserDataDir(opts BrowserSessionOptions) string {
	if trimmed := strings.TrimSpace(m.userDataDir); trimmed != "" {
		return filepath.Clean(trimmed)
	}
	return m.resolveProfileDir(opts)
}

func (m ChromiumMiddleware) resolveDriverDir(opts BrowserSessionOptions) string {
	if trimmed := strings.TrimSpace(opts.DriverDir); trimmed != "" {
		return filepath.Clean(trimmed)
	}
	return m.driverDir
}

func (m ChromiumMiddleware) resolveDriverDirOrDefault(opts BrowserSessionOptions) string {
	if resolved := m.resolveDriverDir(opts); strings.TrimSpace(resolved) != "" {
		return resolved
	}
	if strings.TrimSpace(m.browserInstallDir) != "" {
		driverDir := filepath.Join(m.browserInstallDir, "driver")
		if _, err := os.Stat(driverDir); err == nil {
			return driverDir
		}
		return driverDir
	}
	if trimmed := strings.TrimSpace(m.browserPath); trimmed != "" {
		driverDir := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(trimmed))), "driver")
		if _, err := os.Stat(driverDir); err == nil {
			return driverDir
		}
	}
	return projectruntime.DefaultPlaywrightDriverDir(m.RuntimeRoot())
}

func (m ChromiumMiddleware) resolveLaunchTimeoutMS(opts BrowserSessionOptions) int {
	if opts.LaunchTimeoutMS >= 0 {
		return opts.LaunchTimeoutMS
	}
	if m.launchTimeoutMS >= 0 {
		return m.launchTimeoutMS
	}
	return 120000
}

func (m ChromiumMiddleware) resolveLocale(opts BrowserSessionOptions) string {
	if trimmed := strings.TrimSpace(opts.Locale); trimmed != "" {
		return trimmed
	}
	return m.locale
}

func (m ChromiumMiddleware) resolveUserAgent(opts BrowserSessionOptions) string {
	if trimmed := strings.TrimSpace(opts.UserAgent); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(m.userAgent)
}

func (m ChromiumMiddleware) resolveTimezoneID(opts BrowserSessionOptions) string {
	if trimmed := strings.TrimSpace(opts.TimezoneID); trimmed != "" {
		return trimmed
	}
	return m.timezoneID
}

func (m ChromiumMiddleware) resolveViewport(opts BrowserSessionOptions) (int, int) {
	width := m.viewportWidth
	height := m.viewportHeight
	if opts.ViewportWidth > 0 {
		width = opts.ViewportWidth
	}
	if opts.ViewportHeight > 0 {
		height = opts.ViewportHeight
	}
	return width, height
}

func (m ChromiumMiddleware) LaunchSpec(opts BrowserSessionOptions) LaunchSpec {
	width, height := m.resolveViewport(opts)
	return LaunchSpec{
		BrowserType:     m.BrowserType(),
		BrowserPath:     m.BrowserPath(),
		StealthScript:   m.StealthScript(),
		RuntimeRoot:     m.RuntimeRoot(),
		URL:             m.resolveURL(opts),
		ProfileDir:      m.resolveProfileDir(opts),
		UserDataDir:     m.resolveUserDataDir(opts),
		UserAgent:       m.resolveUserAgent(opts),
		Locale:          m.resolveLocale(opts),
		TimezoneID:      m.resolveTimezoneID(opts),
		ViewportWidth:   width,
		ViewportHeight:  height,
		Headless:        m.resolveHeadless(opts),
		Adblock:         m.adblock,
		DriverDir:       m.resolveDriverDirOrDefault(opts),
		LaunchTimeoutMS: m.resolveLaunchTimeoutMS(opts),
	}
}

func (m ChromiumMiddleware) Payload(opts BrowserSessionOptions) Payload {
	width, height := m.resolveViewport(opts)
	return Payload{
		URL:            m.resolveURL(opts),
		DownloadRoot:   m.downloadRoot,
		OutputDir:      m.outputDir,
		RuntimeRoot:    m.RuntimeRoot(),
		ProfileDir:     m.resolveProfileDir(opts),
		UserAgent:      m.resolveUserAgent(opts),
		Locale:         m.resolveLocale(opts),
		TimezoneID:     m.resolveTimezoneID(opts),
		ViewportWidth:  width,
		ViewportHeight: height,
		Headless:       m.resolveHeadless(opts),
		Adblock:        m.adblock,
		BrowserType:    string(m.BrowserType()),
		DriverDir:      m.resolveDriverDirOrDefault(opts),
	}
}
