//go:build playwright

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"comic_downloader_go_playwright_stealth/browser"
	projectruntime "comic_downloader_go_playwright_stealth/runtime"
	"github.com/playwright-community/playwright-go"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	url := flag.String("url", "", "url to open")
	runtimeRoot := flag.String("runtime-root", "runtime", "runtime root")
	browserPath := flag.String("browser-path", "", "chromium executable path")
	browsersPath := flag.String("browsers-path", "", "path that contains Playwright browsers")
	motherProfileDir := flag.String("mother-profile-dir", "", "chromium mother profile directory")
	userDataDir := flag.String("user-data-dir", "", "chromium mother profile directory (legacy alias)")
	headless := flag.Bool("headless", false, "run browser headless")
	locale := flag.String("locale", "", "browser locale")
	timezoneID := flag.String("timezone-id", "", "browser timezone")
	userAgent := flag.String("user-agent", "", "browser user agent")
	viewportWidth := flag.Int("viewport-width", 1365, "browser viewport width")
	viewportHeight := flag.Int("viewport-height", 768, "browser viewport height")
	driverDir := flag.String("driver-dir", "", "playwright driver directory")
	screenshotDir := flag.String("screenshot-dir", "", "directory to save full-page screenshots of all open tabs")
	keepOpen := flag.Bool("keep-open", true, "keep browser open until you close it manually")
	flag.Parse()
	if cleanup, logPath, err := projectruntime.InitProcessLogging(*runtimeRoot, "chromium-probe"); err != nil {
		return fmt.Errorf("init chromium-probe logging: %w", err)
	} else {
		log.Printf("chromium-probe logging: %s", logPath)
		defer func() {
			if err := cleanup(); err != nil {
				log.Printf("close chromium-probe log: %v", err)
			}
		}()
	}

	if strings.TrimSpace(*url) == "" {
		return fmt.Errorf("missing --url")
	}

	middleware := browser.NewChromiumMiddleware(*url).
		WithRuntimeRoot(*runtimeRoot).
		WithHeadless(*headless).
		WithLocale(*locale).
		WithTimezoneID(*timezoneID).
		WithViewport(*viewportWidth, *viewportHeight)
	if strings.TrimSpace(*browserPath) != "" {
		middleware = middleware.WithBrowserPath(*browserPath)
	}
	if strings.TrimSpace(*browsersPath) != "" {
		middleware = middleware.WithBrowserInstallDir(*browsersPath)
	}
	if strings.TrimSpace(*userAgent) != "" {
		middleware = middleware.WithUserAgent(*userAgent)
	}

	manager := projectruntime.NewBrowserProfileManager(workspaceRootFromRuntimeRoot(*runtimeRoot))
	playwrightProfile, err := manager.PrepareFreshPlaywrightProfile(projectruntime.BrowserTypeChromium)
	if err != nil {
		return fmt.Errorf("prepare chromium fresh profile: %w", err)
	}
	defer func() {
		if err := manager.CleanupFreshPlaywrightProfile(playwrightProfile); err != nil {
			log.Printf("cleanup chromium playwright profile: %v", err)
		}
	}()
	playwrightProfileDir := playwrightProfile.RootDir
	logChromiumProfileCopyAudit("(fresh)", playwrightProfileDir)
	middleware = middleware.WithUserDataDir(playwrightProfileDir).WithProfileDir(playwrightProfileDir)

	if strings.TrimSpace(*driverDir) != "" {
		absDriverDir, err := filepath.Abs(strings.TrimSpace(*driverDir))
		if err != nil {
			return fmt.Errorf("resolve driver directory: %w", err)
		}
		if err := os.Setenv("PLAYWRIGHT_DRIVER_PATH", absDriverDir); err != nil {
			return fmt.Errorf("set PLAYWRIGHT_DRIVER_PATH: %w", err)
		}
		driver, err := playwright.NewDriver(&playwright.RunOptions{
			DriverDirectory:     absDriverDir,
			SkipInstallBrowsers: true,
			Verbose:             true,
		})
		if err != nil {
			return fmt.Errorf("create driver: %w", err)
		}
		if err := driver.DownloadDriver(); err != nil {
			return fmt.Errorf("download driver: %w", err)
		}
	}

	session, err := middleware.Open(browser.BrowserSessionOptions{
		URL:            *url,
		Headless:       headless,
		ProfileDir:     playwrightProfileDir,
		UserAgent:      *userAgent,
		Locale:         *locale,
		TimezoneID:     *timezoneID,
		ViewportWidth:  *viewportWidth,
		ViewportHeight: *viewportHeight,
	})
	if err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			log.Printf("close browser: %v", err)
		}
	}()

	page, ok := session.Page.(interface {
		Title() (string, error)
	})
	if !ok {
		return fmt.Errorf("browser session page does not expose Title()")
	}
	title, err := page.Title()
	if err != nil {
		return fmt.Errorf("page title: %w", err)
	}
	log.Printf("chromium probe title: %s", title)
	fmt.Println(title)
	if strings.TrimSpace(*screenshotDir) != "" {
		if err := waitForEnterToCapture(); err != nil {
			return fmt.Errorf("wait for enter: %w", err)
		}
		if err := captureChromiumTabs(session, *screenshotDir); err != nil {
			return fmt.Errorf("capture chromium tabs: %w", err)
		}
	}
	if *keepOpen {
		if err := waitForBrowserCloseOrSignal(session); err != nil {
			return fmt.Errorf("wait for browser close: %w", err)
		}
	}
	return nil
}

func workspaceRootFromRuntimeRoot(runtimeRoot string) string {
	runtimeRoot = filepath.Clean(strings.TrimSpace(runtimeRoot))
	if runtimeRoot == "" || runtimeRoot == "." {
		return "."
	}
	return filepath.Dir(runtimeRoot)
}

func logChromiumProfileCopyAudit(sourceRoot, tempRoot string) {
	sourceRoot = filepath.Clean(strings.TrimSpace(sourceRoot))
	tempRoot = filepath.Clean(strings.TrimSpace(tempRoot))
	if sourceRoot == "" || tempRoot == "" {
		return
	}
	log.Printf("chromium profile source: %s", sourceRoot)
	log.Printf("chromium profile temp:   %s", tempRoot)
	for _, rel := range []string{
		"Local State",
		filepath.Join("Default", "Preferences"),
		filepath.Join("Default", "Secure Preferences"),
		filepath.Join("Default", "Extensions"),
		filepath.Join("Default", "Local Extension Settings"),
		filepath.Join("Default", "Extension Rules"),
		filepath.Join("Default", "Extension Scripts"),
		filepath.Join("Default", "Extension State"),
	} {
		logProfilePathAudit("source", filepath.Join(sourceRoot, rel))
		logProfilePathAudit("temp", filepath.Join(tempRoot, rel))
	}
}

func logProfilePathAudit(label, path string) {
	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			log.Printf("%s dir: %s (read error: %v)", label, path, readErr)
			return
		}
		log.Printf("%s dir: %s (entries=%d)", label, path, len(entries))
	case err == nil:
		log.Printf("%s file: %s (size=%d)", label, path, info.Size())
	default:
		log.Printf("%s missing: %s (%v)", label, path, err)
	}
}

func waitForEnterToCapture() error {
	fmt.Println("Press Enter to capture all open tabs as full-page screenshots...")
	reader := bufio.NewReader(os.Stdin)
	_, err := reader.ReadString('\n')
	return err
}

func captureChromiumTabs(session *browser.ChromiumSession, dir string) error {
	context, ok := session.Context.(interface {
		Pages() []playwright.Page
	})
	if !ok || context == nil {
		return fmt.Errorf("browser session context does not expose Pages()")
	}
	pages := context.Pages()
	if len(pages) == 0 {
		return fmt.Errorf("no open pages to capture")
	}
	absDir, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		return fmt.Errorf("resolve screenshot directory: %w", err)
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return fmt.Errorf("create screenshot directory: %w", err)
	}
	for i, page := range pages {
		if page == nil {
			continue
		}
		title, _ := page.Title()
		if strings.TrimSpace(title) == "" {
			title = page.URL()
		}
		name := fmt.Sprintf("%02d-%s.png", i+1, sanitizeFileName(title))
		path := filepath.Join(absDir, name)
		if _, err := page.Screenshot(playwright.PageScreenshotOptions{
			Path:     playwright.String(path),
			FullPage: playwright.Bool(true),
		}); err != nil {
			return fmt.Errorf("screenshot %q: %w", path, err)
		}
		log.Printf("screenshot saved: %s", path)
	}
	return nil
}

func sanitizeFileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "page"
	}
	var b strings.Builder
	for _, r := range value {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			b.WriteByte('_')
		default:
			if r < 32 {
				b.WriteByte('_')
			} else {
				b.WriteRune(r)
			}
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "page"
	}
	return out
}

func waitForBrowserCloseOrSignal(session *browser.ChromiumSession) error {
	waitErr := make(chan error, 1)
	go func() {
		waitErr <- session.WaitClosed()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-waitErr:
		return err
	case sig := <-sigCh:
		log.Printf("chromium probe interrupted by %s; closing browser and cleaning temp files", sig)
		_ = session.Close()
		if err := <-waitErr; err != nil {
			return err
		}
		return fmt.Errorf("browser session interrupted by %s", sig)
	}
}
