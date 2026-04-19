package ui

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"comic_downloader_go_playwright_stealth/runtime"
	"comic_downloader_go_playwright_stealth/siteflow/zeri"
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
	ID          string                     `json:"id"`
	Request     tasks.BrowserLaunchRequest `json:"request"`
	Status      TodoStatus                 `json:"status"`
	Progress    float64                    `json:"progress,omitempty"`
	Phase       string                     `json:"phase,omitempty"`
	StepCurrent int                        `json:"stepCurrent,omitempty"`
	StepTotal   int                        `json:"stepTotal,omitempty"`
	StepMessage string                     `json:"stepMessage,omitempty"`
	CreatedAt   time.Time                  `json:"createdAt"`
	StartedAt   time.Time                  `json:"startedAt"`
	FinishedAt  time.Time                  `json:"finishedAt"`
	Result      tasks.BrowserRunResult     `json:"result,omitempty"`
	LastError   string                     `json:"lastError,omitempty"`
}

// TodoList manages items that are added as pending and started later in batch.
type TodoList struct {
	mu       sync.Mutex
	seq      int
	items    []TodoItem
	notifier func()
}

// NewTodoList builds an empty todo list.
func NewTodoList() *TodoList {
	return &TodoList{}
}

// SetNotifier sets a callback that is invoked whenever the list changes.
func (l *TodoList) SetNotifier(fn func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.notifier = fn
}

// AddPending appends one browser request in pending state.
func (l *TodoList) AddPending(req tasks.BrowserLaunchRequest) TodoItem {
	req = req.Normalize()
	l.mu.Lock()
	l.seq++
	item := TodoItem{
		ID:        fmt.Sprintf("todo-%03d", l.seq),
		Request:   req,
		Status:    TodoStatusPending,
		Progress:  0,
		Phase:     "pending",
		CreatedAt: time.Now().UTC(),
	}
	l.items = append(l.items, item)
	notifier := l.notifier
	l.mu.Unlock()
	if notifier != nil {
		notifier()
	}
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
	notifier := l.notifier
	l.mu.Unlock()
	if notifier != nil {
		notifier()
	}
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

// RunImmediately appends one request as running, executes it, and updates the item in place.
func (l *TodoList) RunImmediately(req tasks.BrowserLaunchRequest, runner TodoRunner) (TodoItem, error) {
	if runner == nil {
		runner = tasks.RunBrowserRequest
	}
	req = req.Normalize()

	l.mu.Lock()
	l.seq++
	item := TodoItem{
		ID:        fmt.Sprintf("todo-%03d", l.seq),
		Request:   req,
		Status:    TodoStatusRunning,
		Progress:  0,
		Phase:     "running",
		CreatedAt: time.Now().UTC(),
		StartedAt: time.Now().UTC(),
	}
	l.items = append(l.items, item)
	index := len(l.items) - 1
	notifier := l.notifier
	l.mu.Unlock()

	req.WorkerID = "ui"
	req.TaskID = item.ID
	req.Progress = l.makeProgressUpdater(index)
	result, err := runner(req)

	l.mu.Lock()
	item = l.items[index]
	item.FinishedAt = time.Now().UTC()
	if err != nil {
		item.Status = TodoStatusFailed
		item.Progress = 1
		item.Phase = "failed"
		item.StepMessage = err.Error()
		item.LastError = err.Error()
		l.items[index] = item
		l.mu.Unlock()
		if notifier != nil {
			notifier()
		}
		return item, err
	}
	item.Status = TodoStatusCompleted
	item.Progress = 1
	item.Phase = "completed"
	item.StepCurrent = item.StepTotal
	item.StepMessage = "completed"
	item.Result = result
	item.LastError = ""
	l.items[index] = item
	l.mu.Unlock()
	if notifier != nil {
		notifier()
	}
	return item, nil
}

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

// StartAllUnfinished starts all pending items immediately.
func (l *TodoList) StartAllUnfinished(workspaceRoot string, runner TodoRunner) ([]TodoItem, error) {
	if runner == nil {
		runner = tasks.RunBrowserRequest
	}
	_ = workspaceRoot
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
		item.Progress = 0
		item.Phase = "running"
		item.StepCurrent = 0
		item.StepTotal = 0
		item.StepMessage = ""
		l.items[index] = item
		notifier := l.notifier
		l.mu.Unlock()

		req := item.Request
		if strings.TrimSpace(req.WorkerID) == "" {
			req.WorkerID = "ui"
		}
		if strings.TrimSpace(req.TaskID) == "" {
			req.TaskID = item.ID
		}
		req.Progress = l.makeProgressUpdater(index)
		result, err := runner(req)

		l.mu.Lock()
		item = l.items[index]
		item.FinishedAt = time.Now().UTC()
		if err != nil {
			item.Status = TodoStatusFailed
			item.Progress = 1
			item.Phase = "failed"
			item.StepMessage = err.Error()
			item.LastError = err.Error()
			l.items[index] = item
			l.mu.Unlock()
			results = append(results, item)
			if notifier != nil {
				notifier()
			}
			if runErr == nil {
				runErr = err
			} else {
				runErr = errors.Join(runErr, err)
			}
			continue
		}
		item.Status = TodoStatusCompleted
		item.Progress = 1
		item.Phase = "completed"
		item.StepCurrent = item.StepTotal
		item.StepMessage = "completed"
		item.Result = result
		item.LastError = ""
		l.items[index] = item
		l.mu.Unlock()
		if notifier != nil {
			notifier()
		}

		results = append(results, item)
	}
	return results, runErr
}

func (l *TodoList) makeProgressUpdater(index int) func(zeri.DownloadProgress) {
	return func(update zeri.DownloadProgress) {
		l.mu.Lock()
		if index < 0 || index >= len(l.items) {
			l.mu.Unlock()
			return
		}
		item := l.items[index]
		if update.Fraction >= 0 {
			item.Progress = clamp01(update.Fraction)
		}
		if update.Total > 0 {
			item.StepCurrent = update.Current
			item.StepTotal = update.Total
		}
		if strings.TrimSpace(update.Phase) != "" {
			item.Phase = update.Phase
		}
		if strings.TrimSpace(update.Message) != "" {
			item.StepMessage = update.Message
		}
		l.items[index] = item
		notifier := l.notifier
		l.mu.Unlock()
		if notifier != nil {
			notifier()
		}
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
