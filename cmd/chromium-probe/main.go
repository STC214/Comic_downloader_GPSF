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
	keepOpen := flag.Bool("keep-open", true, "keep browser open until you close it manually")
	flag.Parse()

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
	sourceProfileDir := strings.TrimSpace(*motherProfileDir)
	if sourceProfileDir == "" {
		sourceProfileDir = strings.TrimSpace(*userDataDir)
	}
	if sourceProfileDir == "" {
		resolved, err := manager.SourceProfileDir(projectruntime.BrowserTypeChromium)
		if err != nil {
			return fmt.Errorf("resolve chromium mother profile: %w", err)
		}
		sourceProfileDir = resolved
	}
	playwrightProfile, err := manager.PreparePlaywrightProfileFromSource(projectruntime.BrowserTypeChromium, sourceProfileDir)
	if err != nil {
		return fmt.Errorf("prepare chromium playwright profile: %w", err)
	}
	defer func() {
		if err := manager.CleanupPlaywrightProfile(playwrightProfile); err != nil {
			log.Printf("cleanup chromium playwright profile: %v", err)
		}
	}()
	playwrightProfileDir := playwrightProfile.RootDir
	logChromiumProfileCopyAudit(sourceProfileDir, playwrightProfileDir)
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
	fmt.Println(title)
	if *keepOpen {
		if err := session.WaitClosed(); err != nil {
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
