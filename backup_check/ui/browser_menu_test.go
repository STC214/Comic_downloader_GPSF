package ui

import (
	"path/filepath"
	"testing"

	"comic_downloader_go_playwright_stealth/runtime"
)

func TestDefaultBrowserMenuStateIncludesFirefoxProfileDir(t *testing.T) {
	t.Setenv("APPDATA", filepath.Join(t.TempDir(), "AppData", "Roaming"))
	menu := DefaultBrowserMenuState()
	if menu.SelectedBrowser != "firefox" {
		t.Fatalf("SelectedBrowser = %q, want firefox", menu.SelectedBrowser)
	}
	want := runtime.DefaultFirefoxProfileDir()
	if menu.FirefoxProfileDir != want {
		t.Fatalf("FirefoxProfileDir = %q, want %q", menu.FirefoxProfileDir, want)
	}
}
