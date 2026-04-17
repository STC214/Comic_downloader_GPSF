package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const browserSessionLockFileName = "browser-session.lock"

// BrowserSessionLockPath returns the exact lock-file path used to signal an active browser session.
func BrowserSessionLockPath(runtimeRoot string) string {
	return filepath.Join(strings.TrimSpace(runtimeRoot), browserSessionLockFileName)
}

// BrowserSessionLocked reports whether an active browser session lock currently exists.
func BrowserSessionLocked(runtimeRoot string) bool {
	_, err := os.Stat(BrowserSessionLockPath(runtimeRoot))
	return err == nil
}

// AcquireBrowserSessionLock creates the active browser session lock file and returns a release function.
func AcquireBrowserSessionLock(runtimeRoot string) (func() error, error) {
	lockPath := BrowserSessionLockPath(runtimeRoot)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("create browser session lock dir: %w", err)
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, errors.New("browser session is already running")
		}
		return nil, fmt.Errorf("create browser session lock %q: %w", lockPath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	_, _ = file.WriteString(fmt.Sprintf("pid=%d\nstarted=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano)))

	return func() error {
		return os.Remove(lockPath)
	}, nil
}

// WaitForBrowserSessionUnlock waits until the active browser session lock disappears.
func WaitForBrowserSessionUnlock(runtimeRoot string, pollInterval time.Duration) error {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	lockPath := BrowserSessionLockPath(runtimeRoot)
	for {
		if _, err := os.Stat(lockPath); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("stat browser session lock %q: %w", lockPath, err)
		}
		time.Sleep(pollInterval)
	}
}

// BrowserSessionLockInfo returns a human-readable description of the active lock if present.
func BrowserSessionLockInfo(runtimeRoot string) (string, error) {
	lockPath := BrowserSessionLockPath(runtimeRoot)
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
