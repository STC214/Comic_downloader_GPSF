package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"comic_downloader_go_playwright_stealth/runtime"
	"comic_downloader_go_playwright_stealth/tasks"
)

func TestTodoListAddsPendingItems(t *testing.T) {
	list := NewTodoList()
	item := list.AddPending(tasks.BrowserLaunchRequest{
		URL:         "https://example.com",
		Headless:    true,
		BrowserPath: `C:\Program Files\Mozilla Firefox\firefox.exe`,
	})

	if item.Status != TodoStatusPending {
		t.Fatalf("item.Status = %q, want pending", item.Status)
	}
	if len(list.Pending()) != 1 {
		t.Fatalf("len(Pending()) = %d, want 1", len(list.Pending()))
	}
}

func TestTodoListWaitsForBrowserLockBeforeStarting(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(workspace, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(workspace, "AppData", "Local"))
	runtimeRoot := runtime.NewPaths(workspace).Root

	list := NewTodoList()
	list.AddPending(tasks.BrowserLaunchRequest{URL: "https://example.com"})

	release, err := runtime.AcquireBrowserSessionLock(runtimeRoot)
	if err != nil {
		t.Fatalf("AcquireBrowserSessionLock() error = %v", err)
	}

	done := make(chan []TodoItem, 1)
	runErr := make(chan error, 1)
	go func() {
		items, err := list.StartAllUnfinishedAfterBrowserClose(workspace, 10*time.Millisecond, func(req tasks.BrowserLaunchRequest) (tasks.BrowserRunResult, error) {
			return tasks.BrowserRunResult{URL: req.URL, Title: "ok"}, nil
		})
		done <- items
		runErr <- err
	}()

	select {
	case <-done:
		t.Fatal("start returned before browser lock was released")
	case <-time.After(50 * time.Millisecond):
	}

	if err := release(); err != nil {
		t.Fatalf("release lock: %v", err)
	}

	var items []TodoItem
	select {
	case items = <-done:
	case <-time.After(time.Second):
		t.Fatal("start did not finish after lock release")
	}
	if err := <-runErr; err != nil {
		t.Fatalf("StartAllUnfinishedAfterBrowserClose() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Status != TodoStatusCompleted {
		t.Fatalf("items[0].Status = %q, want completed", items[0].Status)
	}
	if items[0].Result.Title != "ok" {
		t.Fatalf("items[0].Result.Title = %q, want ok", items[0].Result.Title)
	}
}

func TestTodoListStartAllReturnsErrorWhenBrowserBusy(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(workspace, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(workspace, "AppData", "Local"))
	runtimeRoot := runtime.NewPaths(workspace).Root

	list := NewTodoList()
	list.AddPending(tasks.BrowserLaunchRequest{URL: "https://example.com"})

	release, err := runtime.AcquireBrowserSessionLock(runtimeRoot)
	if err != nil {
		t.Fatalf("AcquireBrowserSessionLock() error = %v", err)
	}
	defer func() {
		_ = release()
	}()

	_, err = list.StartAllUnfinished(workspace, func(req tasks.BrowserLaunchRequest) (tasks.BrowserRunResult, error) {
		return tasks.BrowserRunResult{}, nil
	})
	if err == nil {
		t.Fatal("StartAllUnfinished() error = nil, want browser busy error")
	}
	if !strings.Contains(err.Error(), "browser is running") {
		t.Fatalf("busy error = %v, want browser is running message", err)
	}
}

func TestTodoListClearFinishedRemovesCompletedItems(t *testing.T) {
	list := NewTodoList()
	list.AddPending(tasks.BrowserLaunchRequest{URL: "https://example.com"})
	list.AddPending(tasks.BrowserLaunchRequest{URL: "https://example.org"})

	list.mu.Lock()
	list.items[0].Status = TodoStatusCompleted
	list.items[1].Status = TodoStatusFailed
	list.mu.Unlock()

	removed := list.ClearFinished()
	if removed != 2 {
		t.Fatalf("ClearFinished() removed = %d, want 2", removed)
	}
	if len(list.Items()) != 0 {
		t.Fatalf("len(Items()) = %d, want 0", len(list.Items()))
	}
}
