//go:build playwright

package browser

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"

	projectruntime "comic_downloader_go_playwright_stealth/runtime"
)

func (m ChromiumMiddleware) toPlaywrightLaunchOptions(opts BrowserSessionOptions) playwright.BrowserTypeLaunchOptions {
	data := m.LaunchData(opts)
	return playwright.BrowserTypeLaunchOptions{
		ExecutablePath: playwright.String(data.ExecutablePath),
		Headless:       playwright.Bool(data.Headless),
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-infobars",
			"--no-sandbox",
			"--disable-features=IsolateOrigins,site-per-process",
			"--start-maximized", // Request the browser window to start maximized.
		},
	}
}

func (m ChromiumMiddleware) toPlaywrightContextOptions(opts BrowserSessionOptions) playwright.BrowserNewContextOptions {
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
	contextOptions.NoViewport = playwright.Bool(true)

	return contextOptions
}

func (m ChromiumMiddleware) toPlaywrightPersistentContextOptions(opts BrowserSessionOptions) playwright.BrowserTypeLaunchPersistentContextOptions {
	contextOptions := playwright.BrowserTypeLaunchPersistentContextOptions{
		ExecutablePath: playwright.String(m.BrowserPath()),
		Headless:       playwright.Bool(m.resolveHeadless(opts)),
		Timeout:        playwright.Float(float64(m.resolveLaunchTimeoutMS(opts))),
		IgnoreDefaultArgs: []string{
			"--disable-extensions",
			"--disable-component-extensions-with-background-pages",
		},
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-infobars",
			"--no-sandbox",
			"--disable-features=IsolateOrigins,site-per-process",
			"--disable-web-security",
			"--profile-directory=Default",
			"--start-maximized", // Keep the persistent launch maximized as well.
		},
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
	contextOptions.NoViewport = playwright.Bool(true)

	return contextOptions
}

func openChromiumSession(m ChromiumMiddleware, opts BrowserSessionOptions) (*ChromiumSession, error) {
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

	previousDriverPath := os.Getenv("PLAYWRIGHT_DRIVER_PATH")
	if driverDir := strings.TrimSpace(spec.DriverDir); driverDir != "" {
		if err := os.Setenv("PLAYWRIGHT_DRIVER_PATH", driverDir); err != nil {
			return nil, fmt.Errorf("set PLAYWRIGHT_DRIVER_PATH: %w", err)
		}
		defer func() {
			if previousDriverPath == "" {
				_ = os.Unsetenv("PLAYWRIGHT_DRIVER_PATH")
				return
			}
			_ = os.Setenv("PLAYWRIGHT_DRIVER_PATH", previousDriverPath)
		}()
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
	context, err := pw.Chromium.LaunchPersistentContext(spec.UserDataDir, persistentOptions)
	if err != nil {
		_ = pw.Stop()
		_ = releaseLock()
		return nil, fmt.Errorf("launch chromium: %w", err)
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
		return nil, fmt.Errorf("create chromium page: %w", err)
	}

	closed := make(chan struct{})
	var closedOnce sync.Once
	page.OnClose(func(playwright.Page) {
		closedOnce.Do(func() {
			close(closed)
		})
	})

	targetURL := spec.URL
	if strings.TrimSpace(targetURL) == "" {
		targetURL = m.URL()
	}

	// Navigate first, then apply a small amount of post-load interaction noise.
	if _, err := page.Goto(targetURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		_ = page.Close()
		_ = context.Close()
		_ = pw.Stop()
		_ = releaseLock()
		return nil, fmt.Errorf("goto %q: %w", targetURL, err)
	}

	// Add a small amount of interaction noise after navigation.
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 3; i++ {
		_ = page.Mouse().Move(float64(100+rand.Intn(400)), float64(100+rand.Intn(400)))
		time.Sleep(time.Duration(200+rand.Intn(300)) * time.Millisecond)
	}

	_ = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})

	return &ChromiumSession{
		Middleware:  m,
		URL:         targetURL,
		Playwright:  pw,
		Browser:     nil,
		Context:     context,
		Page:        page,
		releaseLock: releaseLock,
		closed:      closed,
	}, nil
}

func closeChromiumSession(s *ChromiumSession) error {
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

func chromiumSessionTitle(s *ChromiumSession) (string, error) {
	if s == nil {
		return "", errors.New("browser session is nil")
	}
	page, ok := s.Page.(playwright.Page)
	if !ok || page == nil {
		return "", errors.New("browser session page is not a playwright.Page")
	}
	return page.Title()
}

func waitChromiumSessionClosed(s *ChromiumSession) error {
	if s == nil {
		return errors.New("browser session is nil")
	}
	if s.closed == nil {
		return errors.New("browser session close channel is nil")
	}
	<-s.closed
	return nil
}

func chromiumSessionContent(s *ChromiumSession) (string, error) {
	if s == nil {
		return "", errors.New("browser session is nil")
	}
	page, ok := s.Page.(playwright.Page)
	if !ok || page == nil {
		return "", errors.New("browser session page is not a playwright.Page")
	}
	return page.Content()
}

func chromiumSessionGoto(s *ChromiumSession, url string) error {
	if s == nil {
		return errors.New("browser session is nil")
	}
	page, ok := s.Page.(playwright.Page)
	if !ok || page == nil {
		return errors.New("browser session page is not a playwright.Page")
	}
	if _, err := page.Goto(url); err != nil {
		return err
	}
	s.URL = url
	return nil
}

func chromiumSessionClickText(s *ChromiumSession, text string) error {
	if s == nil {
		return errors.New("browser session is nil")
	}
	page, ok := s.Page.(playwright.Page)
	if !ok || page == nil {
		return errors.New("browser session page is not a playwright.Page")
	}
	if strings.TrimSpace(text) == "100%" {
		button := page.Locator("#image_width1 button")
		if count, err := button.Count(); err == nil && count > 0 {
			box, err := button.First().BoundingBox()
			if err == nil && box != nil {
				return page.Mouse().Click(box.X+box.Width/2, box.Y+box.Height/2)
			}
			return button.First().Click()
		}
	}
	locator := page.GetByText(text, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)})
	return locator.Click()
}

func chromiumSessionLoadLazyContent(s *ChromiumSession) error {
	return chromiumSessionLoadLazyContentForCount(s, 0)
}

func chromiumSessionLoadLazyContentForCount(s *ChromiumSession, expectedImageCount int) error {
	if s == nil {
		return errors.New("browser session is nil")
	}
	page, ok := s.Page.(playwright.Page)
	if !ok || page == nil {
		return errors.New("browser session page is not a playwright.Page")
	}
	result, err := page.Evaluate(fmt.Sprintf(`async () => {
		const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
		const expected = Math.max(0, Number(%d || 0));
		const imageStats = () => {
			const images = Array.from(document.images || []);
			const total = images.length;
			const loaded = images.filter(img => img.complete && img.naturalWidth > 0).length;
			const target = expected > 0 ? Math.min(expected, total) : total;
			return { total, loaded, target, allLoaded: total > 0 && loaded === total };
		};
		const scrollTop = () => window.scrollTo(0, 0);
		const scrollBottom = () => window.scrollTo(0, Math.max(0, document.documentElement.scrollHeight - window.innerHeight));
		const scrollBounce = async () => {
			const maxScroll = Math.max(0, document.documentElement.scrollHeight - window.innerHeight);
			const points = [
				0,
				maxScroll * 0.25,
				maxScroll * 0.5,
				maxScroll * 0.75,
				maxScroll,
				maxScroll * 0.75,
				maxScroll * 0.5,
				maxScroll * 0.25,
			];
			for (const point of points) {
				window.scrollTo(0, Math.max(0, Math.floor(point)));
				window.dispatchEvent(new Event('scroll'));
				await sleep(180);
			}
		};
		for (let i = 0; i < 60; i++) {
			const stats = imageStats();
			if (stats.allLoaded) {
				scrollTop();
				await sleep(150);
				return stats;
			}
			await scrollBounce();
			scrollBottom();
			window.dispatchEvent(new Event('scroll'));
			await sleep(180);
		}
		const stats = imageStats();
		scrollTop();
		await sleep(150);
		return stats;
	}`, expectedImageCount))
	if err != nil {
		return err
	}
	if stats, ok := result.(map[string]any); ok {
		total := int(asFloat64(stats["total"]))
		loaded := int(asFloat64(stats["loaded"]))
		target := int(asFloat64(stats["target"]))
		if target <= 0 {
			target = total
		}
		if target > 0 && loaded < target {
			return fmt.Errorf("lazy images timed out: %d/%d loaded", loaded, target)
		}
	}
	return nil
}
