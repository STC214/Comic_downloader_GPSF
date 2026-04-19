//go:build playwright

package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"comic_downloader_go_playwright_stealth/tasks"
	"github.com/playwright-community/playwright-go"
)

func main() {
	url := flag.String("url", "", "url to open")
	runtimeRoot := flag.String("runtime-root", "runtime", "runtime root")
	browserPath := flag.String("browser-path", "", "firefox executable path")
	userDataDir := flag.String("user-data-dir", "", "firefox profile directory")
	headless := flag.Bool("headless", true, "run browser headless")
	adblock := flag.Bool("adblock", true, "enable adblock flag in middleware")
	flag.Parse()

	if strings.TrimSpace(*url) == "" {
		log.Fatal("missing --url")
	}

	req := tasks.BrowserLaunchRequest{
		URL:         *url,
		Headless:    *headless,
		RuntimeRoot: *runtimeRoot,
		BrowserPath: *browserPath,
		UserDataDir: *userDataDir,
		Adblock:     *adblock,
	}

	session, err := req.FirefoxMiddleware().Open(req.BrowserOptions())
	if err != nil {
		log.Fatalf("open browser: %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			log.Printf("close browser: %v", err)
		}
	}()

	page, ok := session.Page.(playwright.Page)
	if !ok {
		log.Fatal("browser session page is not a playwright.Page")
	}
	title, err := page.Title()
	if err != nil {
		log.Fatalf("page title: %v", err)
	}
	fmt.Println(title)
}
