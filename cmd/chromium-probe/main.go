//go:build playwright

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"comic_downloader_go_playwright_stealth/browser"
	"github.com/playwright-community/playwright-go"
)

func main() {
	url := flag.String("url", "", "url to open")
	runtimeRoot := flag.String("runtime-root", "runtime", "runtime root")
	browserPath := flag.String("browser-path", "", "chromium executable path")
	browsersPath := flag.String("browsers-path", "", "path that contains Playwright browsers")
	userDataDir := flag.String("user-data-dir", "", "chromium profile directory")
	headless := flag.Bool("headless", false, "run browser headless")
	locale := flag.String("locale", "", "browser locale")
	timezoneID := flag.String("timezone-id", "", "browser timezone")
	userAgent := flag.String("user-agent", "", "browser user agent")
	viewportWidth := flag.Int("viewport-width", 1365, "browser viewport width")
	viewportHeight := flag.Int("viewport-height", 768, "browser viewport height")
	driverDir := flag.String("driver-dir", "", "playwright driver directory")
	keepOpen := flag.Bool("keep-open", true, "keep browser open until you close it manually")
	flag.Parse()

	if strings.TrimSpace(*url) == "" {
		log.Fatal("missing --url")
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
	if strings.TrimSpace(*userDataDir) != "" {
		middleware = middleware.WithUserDataDir(*userDataDir)
	}
	if strings.TrimSpace(*userAgent) != "" {
		middleware = middleware.WithUserAgent(*userAgent)
	}

	if strings.TrimSpace(*driverDir) != "" {
		absDriverDir, err := filepath.Abs(strings.TrimSpace(*driverDir))
		if err != nil {
			log.Fatalf("resolve driver directory: %v", err)
		}
		if err := os.Setenv("PLAYWRIGHT_DRIVER_PATH", absDriverDir); err != nil {
			log.Fatalf("set PLAYWRIGHT_DRIVER_PATH: %v", err)
		}
		driver, err := playwright.NewDriver(&playwright.RunOptions{
			DriverDirectory:     absDriverDir,
			SkipInstallBrowsers: true,
			Verbose:             true,
		})
		if err != nil {
			log.Fatalf("create driver: %v", err)
		}
		if err := driver.DownloadDriver(); err != nil {
			log.Fatalf("download driver: %v", err)
		}
	}

	session, err := middleware.Open(browser.BrowserSessionOptions{
		URL:            *url,
		Headless:       headless,
		ProfileDir:     *userDataDir,
		UserAgent:      *userAgent,
		Locale:         *locale,
		TimezoneID:     *timezoneID,
		ViewportWidth:  *viewportWidth,
		ViewportHeight: *viewportHeight,
	})
	if err != nil {
		log.Fatalf("open browser: %v", err)
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
		log.Fatal("browser session page does not expose Title()")
	}
	title, err := page.Title()
	if err != nil {
		log.Fatalf("page title: %v", err)
	}
	fmt.Println(title)
	if *keepOpen {
		if err := session.WaitClosed(); err != nil {
			log.Fatalf("wait for browser close: %v", err)
		}
	}
}
