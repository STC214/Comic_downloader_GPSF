//go:build playwright

package browser

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/playwright-community/playwright-go"

	projectruntime "comic_downloader_go_playwright_stealth/runtime"
)

func (m FirefoxMiddleware) toPlaywrightLaunchOptions(opts BrowserSessionOptions) playwright.BrowserTypeLaunchOptions {
	data := m.LaunchData(opts)
	return playwright.BrowserTypeLaunchOptions{
		ExecutablePath:   playwright.String(data.ExecutablePath),
		Headless:         playwright.Bool(data.Headless),
		FirefoxUserPrefs: m.resolveFirefoxUserPrefs(opts),
	}
}

func (m FirefoxMiddleware) toPlaywrightContextOptions(opts BrowserSessionOptions) playwright.BrowserNewContextOptions {
	data := m.ContextData(opts)
	contextOptions := playwright.BrowserNewContextOptions{}
	if strings.TrimSpace(data.BaseURL) != "" {
		contextOptions.BaseURL = playwright.String(data.BaseURL)
	}
	if userAgent := strings.TrimSpace(m.resolveUserAgent(opts)); userAgent != "" {
		contextOptions.UserAgent = playwright.String(userAgent)
	}
	if locale := strings.TrimSpace(m.resolveLocale(opts)); locale != "" {
		contextOptions.Locale = playwright.String(locale)
	}
	if timezoneID := strings.TrimSpace(m.resolveTimezoneID(opts)); timezoneID != "" {
		contextOptions.TimezoneId = playwright.String(timezoneID)
	}
	if width, height := m.resolveViewport(opts); width > 0 && height > 0 {
		contextOptions.Viewport = &playwright.Size{Width: width, Height: height}
	}
	return contextOptions
}

func (m FirefoxMiddleware) toPlaywrightPersistentContextOptions(opts BrowserSessionOptions) playwright.BrowserTypeLaunchPersistentContextOptions {
	data := m.ContextData(opts)
	contextOptions := playwright.BrowserTypeLaunchPersistentContextOptions{
		ExecutablePath:   playwright.String(m.BrowserPath()),
		Headless:         playwright.Bool(m.resolveHeadless(opts)),
		FirefoxUserPrefs: m.resolveFirefoxUserPrefs(opts),
	}
	if strings.TrimSpace(data.BaseURL) != "" {
		contextOptions.BaseURL = playwright.String(data.BaseURL)
	}
	if userAgent := strings.TrimSpace(m.resolveUserAgent(opts)); userAgent != "" {
		contextOptions.UserAgent = playwright.String(userAgent)
	}
	if locale := strings.TrimSpace(m.resolveLocale(opts)); locale != "" {
		contextOptions.Locale = playwright.String(locale)
	}
	if timezoneID := strings.TrimSpace(m.resolveTimezoneID(opts)); timezoneID != "" {
		contextOptions.TimezoneId = playwright.String(timezoneID)
	}
	if width, height := m.resolveViewport(opts); width > 0 && height > 0 {
		contextOptions.Viewport = &playwright.Size{Width: width, Height: height}
	}
	return contextOptions
}

func openFirefoxSession(m FirefoxMiddleware, opts BrowserSessionOptions) (*FirefoxSession, error) {
	spec := m.LaunchSpec(opts)
	if strings.TrimSpace(spec.URL) == "" {
		return nil, errors.New("browser middleware url is empty")
	}
	if strings.TrimSpace(spec.BrowserPath) == "" {
		return nil, errors.New("browser path is empty")
	}
	if _, err := os.Stat(spec.BrowserPath); err != nil {
		return nil, fmt.Errorf("browser executable %q: %w", spec.BrowserPath, err)
	}
	if _, err := os.Stat(spec.StealthScript.Path); err != nil {
		return nil, fmt.Errorf("stealth script %q: %w", spec.StealthScript.Path, err)
	}

	releaseLock, err := projectruntime.AcquireBrowserSessionLock(m.RuntimeRoot())
	if err != nil {
		return nil, err
	}

	pw, err := playwright.Run()
	if err != nil {
		_ = releaseLock()
		return nil, fmt.Errorf("start playwright: %w", err)
	}

	persistentOptions := m.toPlaywrightPersistentContextOptions(opts)
	context, err := pw.Firefox.LaunchPersistentContext(spec.UserDataDir, persistentOptions)
	if err != nil {
		_ = pw.Stop()
		_ = releaseLock()
		return nil, fmt.Errorf("launch firefox: %w", err)
	}

	if err := context.AddInitScript(playwright.Script{
		Path: playwright.String(spec.StealthScript.Path),
	}); err != nil {
		_ = context.Close()
		_ = pw.Stop()
		_ = releaseLock()
		return nil, fmt.Errorf("add stealth init script: %w", err)
	}

	var page playwright.Page
	pages := context.Pages()
	if len(pages) > 0 {
		page = pages[0]
	} else {
		page, err = context.NewPage()
	}
	if err != nil {
		_ = context.Close()
		_ = pw.Stop()
		_ = releaseLock()
		return nil, fmt.Errorf("create firefox page: %w", err)
	}

	closed := make(chan struct{})
	var closedOnce sync.Once
	page.OnClose(func(playwright.Page) {
		closedOnce.Do(func() {
			close(closed)
		})
	})

	if _, err := page.Goto(spec.URL); err != nil {
		_ = page.Close()
		_ = context.Close()
		_ = pw.Stop()
		_ = releaseLock()
		return nil, fmt.Errorf("goto %q: %w", spec.URL, err)
	}

	return &FirefoxSession{
		Middleware:  m,
		URL:         spec.URL,
		Playwright:  pw,
		Browser:     nil,
		Context:     context,
		Page:        page,
		releaseLock: releaseLock,
		closed:      closed,
	}, nil
}

func closeFirefoxSession(s *FirefoxSession) error {
	if s == nil {
		return nil
	}
	var firstErr error
	if page, ok := s.Page.(playwright.Page); ok && page != nil {
		if err := page.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if context, ok := s.Context.(playwright.BrowserContext); ok && context != nil {
		if err := context.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if browser, ok := s.Browser.(playwright.Browser); ok && browser != nil {
		if err := browser.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if pw, ok := s.Playwright.(*playwright.Playwright); ok && pw != nil {
		if err := pw.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.releaseLock != nil {
		if err := s.releaseLock(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func sessionTitle(s *FirefoxSession) (string, error) {
	if s == nil {
		return "", errors.New("browser session is nil")
	}
	page, ok := s.Page.(playwright.Page)
	if !ok || page == nil {
		return "", errors.New("browser session page is not a playwright.Page")
	}
	return page.Title()
}

func waitFirefoxSessionClosed(s *FirefoxSession) error {
	if s == nil {
		return errors.New("browser session is nil")
	}
	if s.closed == nil {
		return errors.New("browser session close channel is nil")
	}
	<-s.closed
	return nil
}
