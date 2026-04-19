package browser

import "strings"

// ChromiumSession is the runtime handle returned by Chromium Open().
type ChromiumSession struct {
	Middleware  ChromiumMiddleware
	URL         string
	Playwright  any
	Browser     any
	Context     any
	Page        any
	releaseLock func() error
	closed      chan struct{}
}

// ChromiumPageActions is the minimal page operation surface used by site flows.
type ChromiumPageActions interface {
	PageURL() string
	Content() (string, error)
	Goto(url string) error
	ClickText(text string) error
}

// Open delegates to the build-specific implementation helper.
func (m ChromiumMiddleware) Open(opts BrowserSessionOptions) (*ChromiumSession, error) {
	return openChromiumSession(m, opts)
}

// Close delegates to the build-specific implementation helper.
func (s *ChromiumSession) Close() error {
	return closeChromiumSession(s)
}

// Title returns the page title from the live browser session.
func (s *ChromiumSession) Title() (string, error) {
	return chromiumSessionTitle(s)
}

// WaitClosed blocks until the browser page is closed by the user or browser runtime.
func (s *ChromiumSession) WaitClosed() error {
	return waitChromiumSessionClosed(s)
}

// PageURL returns the current page URL for the live browser session.
func (s *ChromiumSession) PageURL() string {
	if s == nil {
		return ""
	}
	return s.URL
}

// Content returns the current page HTML for the live browser session.
func (s *ChromiumSession) Content() (string, error) {
	return chromiumSessionContent(s)
}

// Goto navigates the live browser session to the provided URL.
func (s *ChromiumSession) Goto(url string) error {
	return chromiumSessionGoto(s, url)
}

// ClickText clicks a visible text node in the live browser session.
func (s *ChromiumSession) ClickText(text string) error {
	return chromiumSessionClickText(s, text)
}

func normalizedChromiumURL(value string) string {
	return strings.TrimSpace(value)
}
