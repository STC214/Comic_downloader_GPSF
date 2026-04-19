package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrowserProfileSourceResolverFirefox(t *testing.T) {
	workspace := t.TempDir()
	appData := filepath.Join(workspace, "AppData", "Roaming")
	t.Setenv("APPDATA", appData)
	t.Setenv("LOCALAPPDATA", filepath.Join(workspace, "AppData", "Local"))

	want := filepath.Join(appData, "Mozilla", "Firefox", "Profiles", "it9mcsht.default")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("create firefox source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appData, "Mozilla", "Firefox", "profiles.ini"), []byte(`
[Profile0]
Name=default
IsRelative=1
Path=Profiles/it9mcsht.default
Default=1
`), 0o644); err != nil {
		t.Fatalf("write profiles.ini: %v", err)
	}

	got, err := NewBrowserProfileSourceResolver().ResolveFirefox()
	if err != nil {
		t.Fatalf("ResolveFirefox() error = %v", err)
	}
	if got != filepath.Clean(want) {
		t.Fatalf("ResolveFirefox() = %q, want %q", got, want)
	}
}

func TestBrowserProfileSourceResolverFirefoxRequiresProfilesINI(t *testing.T) {
	workspace := t.TempDir()
	appData := filepath.Join(workspace, "AppData", "Roaming")
	t.Setenv("APPDATA", appData)
	t.Setenv("LOCALAPPDATA", filepath.Join(workspace, "AppData", "Local"))

	if err := os.MkdirAll(filepath.Join(appData, "Mozilla", "Firefox", "Profiles", "it9mcsht.default"), 0o755); err != nil {
		t.Fatalf("create firefox source: %v", err)
	}

	_, err := NewBrowserProfileSourceResolver().ResolveFirefox()
	if err == nil {
		t.Fatal("ResolveFirefox() error = nil, want non-nil")
	}
}

func TestBrowserProfileSourceResolverChromium(t *testing.T) {
	workspace := t.TempDir()
	localAppData := filepath.Join(workspace, "AppData", "Local")
	t.Setenv("APPDATA", filepath.Join(workspace, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", localAppData)

	want := filepath.Join(localAppData, "Chromium", "User Data")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("create chromium source: %v", err)
	}

	got, err := NewBrowserProfileSourceResolver().ResolveChromium()
	if err != nil {
		t.Fatalf("ResolveChromium() error = %v", err)
	}
	if got != filepath.Clean(want) {
		t.Fatalf("ResolveChromium() = %q, want %q", got, want)
	}
}
