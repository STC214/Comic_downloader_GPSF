//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPortableChildEnvironmentPinsRuntimeArtifactsBesideLauncher(t *testing.T) {
	portableRoot := filepath.Clean(`D:\apps\comic\portable-data`)
	t.Setenv("COMIC_DOWNLOADER_WORKSPACE_ROOT", `X:\wrong-workspace`)
	t.Setenv("COMIC_DOWNLOADER_RUNTIME_ROOT", `X:\wrong-runtime`)
	t.Setenv("COMIC_DOWNLOADER_FRONTEND_STATE_PATH", `X:\wrong-frontend.json`)
	t.Setenv("COMIC_DOWNLOADER_STATE_PATH", `X:\wrong-state.json`)

	env := portableChildEnvironment([]string{
		`Path=C:\Windows`,
		`comic_downloader_runtime_root=X:\inherited-runtime`,
		`COMIC_DOWNLOADER_WORKSPACE_ROOT=X:\inherited-workspace`,
	}, portableRoot, filepath.Join(portableRoot, "playwright-browsers"))

	got := environmentMap(env)
	for key, want := range map[string]string{
		"COMIC_DOWNLOADER_WORKSPACE_ROOT":      portableRoot,
		"COMIC_DOWNLOADER_RUNTIME_ROOT":        portableRoot,
		"COMIC_DOWNLOADER_LOG_ROOT":            filepath.Join(portableRoot, "logs"),
		"COMIC_DOWNLOADER_FRONTEND_STATE_PATH": filepath.Join(portableRoot, "frontend_state.json"),
		"COMIC_DOWNLOADER_STATE_PATH":          filepath.Join(portableRoot, "comic_downloader_state.json"),
	} {
		if got[key] != want {
			t.Fatalf("%s = %q, want %q", key, got[key], want)
		}
	}
}

func environmentMap(env []string) map[string]string {
	result := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			result[strings.ToUpper(key)] = value
		}
	}
	return result
}
