package browser

import "strings"

// LaunchData is the Playwright launch input distilled into a dependency-free shape.
type LaunchData struct {
	ExecutablePath string `json:"executablePath"`
	Headless       bool   `json:"headless"`
}

// ContextData is the Playwright context input distilled into a dependency-free shape.
type ContextData struct {
	BaseURL string `json:"baseURL"`
}

// FirefoxSession is the runtime handle returned by Open().
type FirefoxSession struct {
	Middleware  FirefoxMiddleware
	URL         string
	Playwright  any
	Browser     any
	Context     any
	Page        any
	releaseLock func() error
	closed      chan struct{}
}

// LaunchData returns the launch inputs needed by the Playwright-backed implementation.
func (m FirefoxMiddleware) LaunchData(opts BrowserSessionOptions) LaunchData {
	spec := m.LaunchSpec(opts)
	return LaunchData{
		ExecutablePath: spec.BrowserPath,
		Headless:       m.resolveHeadless(opts),
	}
}

// ContextData returns the browser context inputs needed by the Playwright-backed implementation.
func (m FirefoxMiddleware) ContextData(opts BrowserSessionOptions) ContextData {
	return ContextData{BaseURL: m.URL()}
}

// Open delegates to the build-specific implementation helper.
func (m FirefoxMiddleware) Open(opts BrowserSessionOptions) (*FirefoxSession, error) {
	return openFirefoxSession(m, opts)
}

// Close delegates to the build-specific implementation helper.
func (s *FirefoxSession) Close() error {
	return closeFirefoxSession(s)
}

// Title returns the page title from the live browser session.
func (s *FirefoxSession) Title() (string, error) {
	return sessionTitle(s)
}

// WaitClosed blocks until the browser page is closed by the user or browser runtime.
func (s *FirefoxSession) WaitClosed() error {
	return waitFirefoxSessionClosed(s)
}

func normalizedURL(value string) string {
	return strings.TrimSpace(value)
}
