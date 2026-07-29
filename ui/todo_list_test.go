package ui

import (
	"context"
	"os"
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

func TestTodoListRunImmediatelyCompletesItem(t *testing.T) {
	list := NewTodoList()
	runtimeRoot := t.TempDir()
	item, err := list.RunImmediately(tasks.BrowserLaunchRequest{URL: "https://example.com", RuntimeRoot: runtimeRoot}, func(req tasks.BrowserLaunchRequest) (tasks.BrowserRunResult, error) {
		return tasks.BrowserRunResult{URL: req.URL, Title: "ok"}, nil
	})
	if err != nil {
		t.Fatalf("RunImmediately() error = %v", err)
	}
	if item.Status != TodoStatusCompleted {
		t.Fatalf("item.Status = %q, want completed", item.Status)
	}
	if item.Result.Title != "ok" {
		t.Fatalf("item.Result.Title = %q, want ok", item.Result.Title)
	}
	if len(list.Items()) != 1 {
		t.Fatalf("len(Items()) = %d, want 1", len(list.Items()))
	}
	reportPath := filepath.Join(runtimeRoot, "tasks", "task-"+item.ID, "report.json")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("saved report not found: %v", err)
	}
	logPath := runtime.NewPathsFromRuntimeRoot(runtimeRoot).TaskLogPath(item.ID)
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("successful task log not found: %v", err)
	}
	if !strings.Contains(string(logData), "State: completed") {
		t.Fatalf("successful task log does not contain completed state: %s", logData)
	}
}

func TestTodoListConcurrencyLimitSerializesImmediateRuns(t *testing.T) {
	list := NewTodoList()
	list.SetConcurrencyLimit(1)
	runtimeRoot := t.TempDir()

	started := make(chan string, 2)
	releaseFirst := make(chan struct{})
	runner := func(req tasks.BrowserLaunchRequest) (tasks.BrowserRunResult, error) {
		started <- req.TaskID
		if req.TaskID == "todo-001" {
			<-releaseFirst
		}
		return tasks.BrowserRunResult{URL: req.URL, Title: req.TaskID}, nil
	}

	done1 := make(chan error, 1)
	go func() {
		_, err := list.RunImmediately(tasks.BrowserLaunchRequest{URL: "https://one.example", RuntimeRoot: runtimeRoot}, runner)
		done1 <- err
	}()

	select {
	case id := <-started:
		if id != "todo-001" {
			t.Fatalf("first started task = %q, want todo-001", id)
		}
	case <-time.After(time.Second):
		t.Fatal("first task did not start")
	}

	done2 := make(chan error, 1)
	go func() {
		_, err := list.RunImmediately(tasks.BrowserLaunchRequest{URL: "https://two.example", RuntimeRoot: runtimeRoot}, runner)
		done2 <- err
	}()

	select {
	case id := <-started:
		t.Fatalf("second task started early: %q", id)
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseFirst)

	select {
	case id := <-started:
		if id != "todo-002" {
			t.Fatalf("second started task = %q, want todo-002", id)
		}
	case <-time.After(time.Second):
		t.Fatal("second task did not start after slot was released")
	}

	if err := <-done1; err != nil {
		t.Fatalf("first task error = %v", err)
	}
	if err := <-done2; err != nil {
		t.Fatalf("second task error = %v", err)
	}
}

func TestTodoListWaitsForBrowserLockBeforeStarting(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(workspace, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(workspace, "AppData", "Local"))
	runtimeRoot := runtime.NewPaths(workspace).Root

	list := NewTodoList()
	list.AddPending(tasks.BrowserLaunchRequest{URL: "https://example.com", RuntimeRoot: runtimeRoot})

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

func TestTodoListStartAllDoesNotCheckBrowserLock(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(workspace, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(workspace, "AppData", "Local"))
	runtimeRoot := runtime.NewPaths(workspace).Root

	list := NewTodoList()
	list.AddPending(tasks.BrowserLaunchRequest{URL: "https://example.com", RuntimeRoot: runtimeRoot})

	release, err := runtime.AcquireBrowserSessionLock(runtimeRoot)
	if err != nil {
		t.Fatalf("AcquireBrowserSessionLock() error = %v", err)
	}
	defer func() {
		_ = release()
	}()

	items, err := list.StartAllUnfinished(workspace, func(req tasks.BrowserLaunchRequest) (tasks.BrowserRunResult, error) {
		return tasks.BrowserRunResult{URL: req.URL, Title: "ok"}, nil
	})
	if err != nil {
		t.Fatalf("StartAllUnfinished() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Status != TodoStatusCompleted {
		t.Fatalf("items[0].Status = %q, want completed", items[0].Status)
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

func TestTodoListSortsRunningFailedCompletedByIDDesc(t *testing.T) {
	list := NewTodoList()
	list.mu.Lock()
	list.items = []TodoItem{
		{ID: "todo-10", Status: TodoStatusCompleted, CreatedAt: time.Unix(10, 0).UTC()},
		{ID: "todo-2", Status: TodoStatusFailed, CreatedAt: time.Unix(20, 0).UTC()},
		{ID: "todo-7", Status: TodoStatusRunning, CreatedAt: time.Unix(30, 0).UTC()},
		{ID: "todo-5", Status: TodoStatusCompleted, CreatedAt: time.Unix(40, 0).UTC()},
		{ID: "todo-8", Status: TodoStatusFailed, CreatedAt: time.Unix(50, 0).UTC()},
		{ID: "todo-3", Status: TodoStatusRunning, CreatedAt: time.Unix(60, 0).UTC()},
	}
	list.mu.Unlock()

	items := list.Items()
	want := []string{"todo-7", "todo-3", "todo-8", "todo-2", "todo-10", "todo-5"}
	if len(items) != len(want) {
		t.Fatalf("len(items) = %d, want %d", len(items), len(want))
	}
	for i, item := range items {
		if item.ID != want[i] {
			t.Fatalf("items[%d].ID = %q, want %q", i, item.ID, want[i])
		}
	}
}

func TestTodoListStartAllIncludesRestartableStatuses(t *testing.T) {
	list := NewTodoList()
	workspace := t.TempDir()
	runtimeRoot := filepath.Join(workspace, "runtime")
	list.mu.Lock()
	list.items = []TodoItem{
		{ID: "todo-1", Status: TodoStatusPaused, Request: tasks.BrowserLaunchRequest{URL: "https://paused.example", RuntimeRoot: runtimeRoot}},
		{ID: "todo-2", Status: TodoStatusFailed, Request: tasks.BrowserLaunchRequest{URL: "https://failed.example", RuntimeRoot: runtimeRoot}},
		{ID: "todo-3", Status: TodoStatusPending, Request: tasks.BrowserLaunchRequest{URL: "https://pending.example", RuntimeRoot: runtimeRoot}},
		{ID: "todo-4", Status: TodoStatusCompleted, Request: tasks.BrowserLaunchRequest{URL: "https://done.example", RuntimeRoot: runtimeRoot}},
	}
	list.mu.Unlock()

	items, err := list.StartAllUnfinished(workspace, func(req tasks.BrowserLaunchRequest) (tasks.BrowserRunResult, error) {
		return tasks.BrowserRunResult{URL: req.URL, Title: req.TaskID}, nil
	})
	if err != nil {
		t.Fatalf("StartAllUnfinished() error = %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	if item, ok := list.ItemByID("todo-4"); !ok || item.Status != TodoStatusCompleted {
		t.Fatalf("completed item changed unexpectedly: ok=%v status=%v", ok, item.Status)
	}
	for _, id := range []string{"todo-1", "todo-2", "todo-3"} {
		item, ok := list.ItemByID(id)
		if !ok {
			t.Fatalf("missing item %s", id)
		}
		if item.Status != TodoStatusCompleted {
			t.Fatalf("item %s status = %q, want completed", id, item.Status)
		}
	}
}

func TestPauseRunningTaskCancelsRunnerAndKeepsPausedState(t *testing.T) {
	list := NewTodoList()
	item := list.AddPending(tasks.BrowserLaunchRequest{URL: "https://example.com", RuntimeRoot: t.TempDir()})
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := list.RunByIDs([]string{item.ID}, func(req tasks.BrowserLaunchRequest) (tasks.BrowserRunResult, error) {
			close(started)
			<-req.Context.Done()
			return tasks.BrowserRunResult{}, req.Context.Err()
		})
		done <- err
	}()
	<-started
	if changed := list.SetStatusByIDs([]string{item.ID}, TodoStatusPaused, "paused"); changed != 1 {
		t.Fatalf("SetStatusByIDs() = %d, want 1", changed)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunByIDs() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runner was not canceled")
	}
	got, ok := list.ItemByID(item.ID)
	if !ok || got.Status != TodoStatusPaused || got.LastError != "" {
		t.Fatalf("paused item = %#v, ok=%v", got, ok)
	}
}

func TestRemoveRunningTaskCancelsRunner(t *testing.T) {
	list := NewTodoList()
	item := list.AddPending(tasks.BrowserLaunchRequest{URL: "https://example.com", RuntimeRoot: t.TempDir()})
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := list.RunByIDs([]string{item.ID}, func(req tasks.BrowserLaunchRequest) (tasks.BrowserRunResult, error) {
			close(started)
			<-req.Context.Done()
			return tasks.BrowserRunResult{}, context.Canceled
		})
		done <- err
	}()
	<-started
	if removed := list.RemoveByIDs([]string{item.ID}); removed != 1 {
		t.Fatalf("RemoveByIDs() = %d, want 1", removed)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunByIDs() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runner was not canceled")
	}
	if _, ok := list.ItemByID(item.ID); ok {
		t.Fatal("removed item reappeared")
	}
}

func TestPauseTaskWaitingForRunSlotDoesNotStartRunner(t *testing.T) {
	list := NewTodoList()
	list.SetConcurrencyLimit(1)
	first := list.AddPending(tasks.BrowserLaunchRequest{URL: "https://one.example", RuntimeRoot: t.TempDir()})
	second := list.AddPending(tasks.BrowserLaunchRequest{URL: "https://two.example", RuntimeRoot: t.TempDir()})
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		_, err := list.RunByIDs([]string{first.ID}, func(req tasks.BrowserLaunchRequest) (tasks.BrowserRunResult, error) {
			close(firstStarted)
			<-releaseFirst
			return tasks.BrowserRunResult{URL: req.URL}, nil
		})
		firstDone <- err
	}()
	<-firstStarted

	secondCalled := make(chan struct{}, 1)
	secondDone := make(chan error, 1)
	go func() {
		_, err := list.RunByIDs([]string{second.ID}, func(req tasks.BrowserLaunchRequest) (tasks.BrowserRunResult, error) {
			secondCalled <- struct{}{}
			return tasks.BrowserRunResult{URL: req.URL}, nil
		})
		secondDone <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		list.mu.Lock()
		registered := list.activeRuns[second.ID] != nil
		list.mu.Unlock()
		if registered {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second task did not register its run context")
		}
		time.Sleep(time.Millisecond)
	}
	list.SetStatusByIDs([]string{second.ID}, TodoStatusPaused, "paused")
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("second RunByIDs() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiting task did not stop after pause")
	}
	select {
	case <-secondCalled:
		t.Fatal("paused waiting task started its runner")
	default:
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first RunByIDs() error = %v", err)
	}
}

func TestStartAllUsesStableTaskIDsWhenItemIsRemoved(t *testing.T) {
	list := NewTodoList()
	runtimeRoot := t.TempDir()
	first := list.AddPending(tasks.BrowserLaunchRequest{URL: "https://one.example", RuntimeRoot: runtimeRoot})
	second := list.AddPending(tasks.BrowserLaunchRequest{URL: "https://two.example", RuntimeRoot: runtimeRoot})
	third := list.AddPending(tasks.BrowserLaunchRequest{URL: "https://three.example", RuntimeRoot: runtimeRoot})
	_ = first

	started := make(chan string, 3)
	release := make(chan struct{})
	done := make(chan error, 1)
	workspace := t.TempDir()
	go func() {
		_, err := list.StartAllUnfinishedWithConcurrency(workspace, 1, func(req tasks.BrowserLaunchRequest) (tasks.BrowserRunResult, error) {
			started <- req.TaskID
			if req.TaskID == third.ID {
				<-release
			}
			return tasks.BrowserRunResult{URL: req.URL}, nil
		})
		done <- err
	}()
	if id := <-started; id != third.ID {
		t.Fatalf("first started ID = %q, want %q", id, third.ID)
	}
	if removed := list.RemoveByIDs([]string{second.ID}); removed != 1 {
		t.Fatalf("RemoveByIDs() = %d, want 1", removed)
	}
	close(release)
	select {
	case id := <-started:
		if id != first.ID {
			t.Fatalf("next started ID = %q, want %q", id, first.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("remaining task did not start")
	}
	if err := <-done; err != nil {
		t.Fatalf("StartAllUnfinishedWithConcurrency() error = %v", err)
	}
	select {
	case id := <-started:
		t.Fatalf("unexpected extra task started: %s", id)
	default:
	}
}

func TestAttachRunContextCancelsWhenTaskWasPausedBeforeRegistration(t *testing.T) {
	list := NewTodoList()
	item := list.AddPending(tasks.BrowserLaunchRequest{URL: "https://example.com"})
	list.SetStatusByIDs([]string{item.ID}, TodoStatusPaused, "paused")
	req, cleanup := list.attachRunContext(item.ID, item.Request)
	defer cleanup()
	select {
	case <-req.Context.Done():
	default:
		t.Fatal("run context was not canceled for an already-paused task")
	}
}

func TestAttachRunContextCancelsWhenTaskWasRemovedBeforeRegistration(t *testing.T) {
	list := NewTodoList()
	item := list.AddPending(tasks.BrowserLaunchRequest{URL: "https://example.com"})
	list.RemoveByIDs([]string{item.ID})
	req, cleanup := list.attachRunContext(item.ID, item.Request)
	defer cleanup()
	select {
	case <-req.Context.Done():
	default:
		t.Fatal("run context was not canceled for a removed task")
	}
}
