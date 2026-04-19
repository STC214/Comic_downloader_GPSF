package tasks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrowserLaunchRequestBuildsMiddleware(t *testing.T) {
	workspace := t.TempDir()
	projectMother := filepath.Join(workspace, "runtime", "browser-profiles", "firefox")
	if err := os.MkdirAll(projectMother, 0o755); err != nil {
		t.Fatalf("create project mother dir: %v", err)
	}

	req := BrowserLaunchRequest{
		URL:                  "https://example.com",
		Headless:             false,
		RuntimeRoot:          filepath.Join(workspace, "runtime"),
		ProfileDir:           "C:/Users/stc52/AppData/Roaming/Mozilla/Firefox/Profiles/q6nkoa5l.default-default-1",
		UserAgent:            "Mozilla/5.0 test agent",
		FirefoxUserPrefsJSON: `{"browser.tabs.warnOnClose":false}`,
	}
	middleware := req.FirefoxMiddleware()
	spec := middleware.LaunchSpec(req.BrowserOptions())
	if spec.URL != "https://example.com" {
		t.Fatalf("spec.URL = %q", spec.URL)
	}
	if spec.Headless {
		t.Fatalf("spec.Headless = true, want false")
	}
	wantPath := `C:\Program Files\Mozilla Firefox\firefox.exe`
	if spec.BrowserPath != wantPath {
		t.Fatalf("spec.BrowserPath = %q, want %q", spec.BrowserPath, wantPath)
	}
	wantProfile := filepath.Clean("C:/Users/stc52/AppData/Roaming/Mozilla/Firefox/Profiles/q6nkoa5l.default-default-1")
	if spec.ProfileDir != wantProfile {
		t.Fatalf("spec.ProfileDir = %q, want %q", spec.ProfileDir, wantProfile)
	}
	if spec.UserDataDir != wantProfile {
		t.Fatalf("spec.UserDataDir = %q, want %q", spec.UserDataDir, wantProfile)
	}
	if spec.UserAgent != "Mozilla/5.0 test agent" {
		t.Fatalf("spec.UserAgent = %q", spec.UserAgent)
	}
	if spec.FirefoxUserPrefs["browser.tabs.warnOnClose"] != false {
		t.Fatalf("spec.FirefoxUserPrefs = %#v", spec.FirefoxUserPrefs)
	}
}

func TestBrowserLaunchRequestDefaultsChromium(t *testing.T) {
	workspace := t.TempDir()
	req := BrowserLaunchRequest{
		URL:         "https://example.com",
		BrowserType: "chromium",
		RuntimeRoot: filepath.Join(workspace, "runtime"),
	}
	req = req.Normalize()
	if req.BrowserType != "chromium" {
		t.Fatalf("BrowserType = %q, want chromium", req.BrowserType)
	}
	wantProfile := filepath.Clean(`C:\Users\stc52\AppData\Local\Google\Chrome for Testing\User Data`)
	if req.ProfileDir != wantProfile {
		t.Fatalf("ProfileDir = %q, want %q", req.ProfileDir, wantProfile)
	}
	if req.UserDataDir != wantProfile {
		t.Fatalf("UserDataDir = %q, want %q", req.UserDataDir, wantProfile)
	}
	if req.BrowserPath != "" {
		t.Fatalf("BrowserPath = %q, want empty and resolved later", req.BrowserPath)
	}
}
