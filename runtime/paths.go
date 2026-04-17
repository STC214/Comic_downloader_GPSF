package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Paths describes the runtime directory layout used by the UI.
type Paths struct {
	Root            string
	BrowserRoot     string
	BrowserTasks    string
	BrowserVerify   string
	BrowserBaseline string
	TasksRoot       string
	LogsRoot        string
	OutputRoot      string
	ThumbnailsRoot  string
}

// NewPaths builds the default runtime layout under workspaceRoot.
func NewPaths(workspaceRoot string) Paths {
	root := filepath.Join(workspaceRoot, "runtime")
	return Paths{
		Root:            root,
		BrowserRoot:     filepath.Join(root, "browser-profiles"),
		BrowserTasks:    filepath.Join(root, "browser-profiles", "tasks"),
		BrowserVerify:   filepath.Join(root, "browser-profiles", "verification"),
		BrowserBaseline: filepath.Join(root, "browser-profiles", "baseline-userdata"),
		TasksRoot:       filepath.Join(root, "tasks"),
		LogsRoot:        filepath.Join(root, "logs"),
		OutputRoot:      filepath.Join(root, "output"),
		ThumbnailsRoot:  filepath.Join(root, "thumbnails"),
	}
}

// Ensure creates the runtime directories used by the current UI package.
func (p Paths) Ensure() error {
	for _, dir := range []string{
		p.Root,
		p.BrowserRoot,
		p.BrowserTasks,
		p.BrowserVerify,
		p.BrowserBaseline,
		p.TasksRoot,
		p.LogsRoot,
		p.OutputRoot,
		p.ThumbnailsRoot,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create runtime dir %q: %w", dir, err)
		}
	}
	return nil
}

// TaskReportPath returns the path to one task report file.
func (p Paths) TaskReportPath(taskID string) string {
	return filepath.Join(p.TasksRoot, "task-"+taskID, "report.json")
}

// DefaultFirefoxProfileDir returns the exact project-owned Firefox profile directory used by tasks.
func DefaultFirefoxProfileDir() string {
	return filepath.Join("runtime", "browser-profiles", "baseline-userdata")
}

// DefaultFirefoxProfileSourceDir returns the exact Firefox source profile directory selected on this machine.
func DefaultFirefoxProfileSourceDir() string {
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "Mozilla", "Firefox", "Profiles", "jo2klram.default-release")
}
