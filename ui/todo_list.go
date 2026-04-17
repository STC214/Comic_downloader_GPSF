package ui

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"comic_downloader_go_playwright_stealth/runtime"
	"comic_downloader_go_playwright_stealth/tasks"
)

// TodoStatus is the frontend queue state for an item before it is started.
type TodoStatus string

const (
	TodoStatusPending   TodoStatus = "pending"
	TodoStatusRunning   TodoStatus = "running"
	TodoStatusCompleted TodoStatus = "completed"
	TodoStatusFailed    TodoStatus = "failed"
)

// TodoItem is one item in the frontend todo list.
type TodoItem struct {
	ID         string                     `json:"id"`
	Request    tasks.BrowserLaunchRequest `json:"request"`
	Status     TodoStatus                 `json:"status"`
	CreatedAt  time.Time                  `json:"createdAt"`
	StartedAt  time.Time                  `json:"startedAt"`
	FinishedAt time.Time                  `json:"finishedAt"`
	Result     tasks.BrowserRunResult     `json:"result,omitempty"`
	LastError  string                     `json:"lastError,omitempty"`
}

// TodoList manages items that are added as pending and started later in batch.
type TodoList struct {
	mu    sync.Mutex
	seq   int
	items []TodoItem
}

// NewTodoList builds an empty todo list.
func NewTodoList() *TodoList {
	return &TodoList{}
}

// AddPending appends one browser request in pending state.
func (l *TodoList) AddPending(req tasks.BrowserLaunchRequest) TodoItem {
	req = req.Normalize()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seq++
	item := TodoItem{
		ID:        fmt.Sprintf("todo-%03d", l.seq),
		Request:   req,
		Status:    TodoStatusPending,
		CreatedAt: time.Now().UTC(),
	}
	l.items = append(l.items, item)
	return item
}

// Items returns a copy of the current todo list.
func (l *TodoList) Items() []TodoItem {
	l.mu.Lock()
	defer l.mu.Unlock()
	items := make([]TodoItem, len(l.items))
	copy(items, l.items)
	return items
}

// Pending returns all items that have not been started yet.
func (l *TodoList) Pending() []TodoItem {
	l.mu.Lock()
	defer l.mu.Unlock()
	pending := make([]TodoItem, 0, len(l.items))
	for _, item := range l.items {
		if item.Status == TodoStatusPending {
			pending = append(pending, item)
		}
	}
	return pending
}

// ClearFinished removes completed and failed items from the list.
func (l *TodoList) ClearFinished() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := l.items[:0]
	removed := 0
	for _, item := range l.items {
		switch item.Status {
		case TodoStatusCompleted, TodoStatusFailed:
			removed++
		default:
			kept = append(kept, item)
		}
	}
	l.items = kept
	return removed
}

// BrowserRunning reports whether a browser session lock exists.
func (l *TodoList) BrowserRunning(workspaceRoot string) bool {
	return runtime.BrowserSessionLocked(runtime.NewPaths(workspaceRoot).Root)
}

// WaitForBrowserClose waits until the browser session lock disappears.
func (l *TodoList) WaitForBrowserClose(workspaceRoot string, pollInterval time.Duration) error {
	return runtime.WaitForBrowserSessionUnlock(runtime.NewPaths(workspaceRoot).Root, pollInterval)
}

// TodoRunner runs one browser request.
type TodoRunner func(tasks.BrowserLaunchRequest) (tasks.BrowserRunResult, error)

// StartAllUnfinishedAfterBrowserClose waits for the browser session lock to disappear, then starts all pending items.
func (l *TodoList) StartAllUnfinishedAfterBrowserClose(workspaceRoot string, pollInterval time.Duration, runner TodoRunner) ([]TodoItem, error) {
	if runner == nil {
		runner = tasks.RunBrowserRequest
	}
	runtimeRoot := runtime.NewPaths(workspaceRoot).Root
	if runtime.BrowserSessionLocked(runtimeRoot) {
		if err := runtime.WaitForBrowserSessionUnlock(runtimeRoot, pollInterval); err != nil {
			return nil, err
		}
	}
	return l.startAllPending(runner)
}

// StartAllUnfinished starts all pending items immediately. It returns an error if a browser session is still running.
func (l *TodoList) StartAllUnfinished(workspaceRoot string, runner TodoRunner) ([]TodoItem, error) {
	if runner == nil {
		runner = tasks.RunBrowserRequest
	}
	if runtime.BrowserSessionLocked(runtime.NewPaths(workspaceRoot).Root) {
		return nil, errors.New("browser is running; close the current browser window before starting unfinished tasks")
	}
	return l.startAllPending(runner)
}

func (l *TodoList) startAllPending(runner TodoRunner) ([]TodoItem, error) {
	l.mu.Lock()
	indices := make([]int, 0, len(l.items))
	for i, item := range l.items {
		if item.Status == TodoStatusPending {
			indices = append(indices, i)
		}
	}
	l.mu.Unlock()

	results := make([]TodoItem, 0, len(indices))
	var runErr error
	for _, index := range indices {
		l.mu.Lock()
		item := l.items[index]
		item.Status = TodoStatusRunning
		item.StartedAt = time.Now().UTC()
		l.items[index] = item
		l.mu.Unlock()

		result, err := runner(item.Request)

		l.mu.Lock()
		item = l.items[index]
		item.FinishedAt = time.Now().UTC()
		if err != nil {
			item.Status = TodoStatusFailed
			item.LastError = err.Error()
			l.items[index] = item
			l.mu.Unlock()
			results = append(results, item)
			if runErr == nil {
				runErr = err
			} else {
				runErr = errors.Join(runErr, err)
			}
			continue
		}
		item.Status = TodoStatusCompleted
		item.Result = result
		item.LastError = ""
		l.items[index] = item
		l.mu.Unlock()

		results = append(results, item)
	}
	return results, runErr
}
