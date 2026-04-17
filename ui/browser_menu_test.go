package ui

import (
	"path/filepath"
	"testing"

	"comic_downloader_go_playwright_stealth/runtime"
)

func TestDefaultBrowserMenuStateIncludesFirefoxPaths(t *testing.T) {
	t.Setenv("APPDATA", filepath.Join(t.TempDir(), "AppData", "Roaming"))
	menu := DefaultBrowserMenuState()
	if menu.SelectedBrowser != "firefox" {
		t.Fatalf("SelectedBrowser = %q, want firefox", menu.SelectedBrowser)
	}
	if menu.FirefoxExecutablePath != `C:\Program Files\Mozilla Firefox\firefox.exe` {
		t.Fatalf("FirefoxExecutablePath = %q, want system Firefox path", menu.FirefoxExecutablePath)
	}
	wantMother := runtime.DefaultFirefoxProfileSourceDir()
	if menu.FirefoxMotherProfileDir != wantMother {
		t.Fatalf("FirefoxMotherProfileDir = %q, want %q", menu.FirefoxMotherProfileDir, wantMother)
	}
	wantWorking := runtime.DefaultFirefoxProfileDir()
	if menu.FirefoxWorkingProfileDir != wantWorking {
		t.Fatalf("FirefoxWorkingProfileDir = %q, want %q", menu.FirefoxWorkingProfileDir, wantWorking)
	}
}
