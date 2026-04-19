package runtime

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// BrowserProfileManager manages project-managed browser mother profiles and task copies.
type BrowserProfileManager struct {
	Paths  BrowserPaths
	Source BrowserProfileSourceResolver
}

// BrowserPlaywrightProfile describes one temporary Playwright profile copied from a source profile.
type BrowserPlaywrightProfile struct {
	BrowserType      BrowserType `json:"browserType"`
	SourceProfileDir string      `json:"sourceProfileDir"`
	RootDir          string      `json:"rootDir"`
}

// NewBrowserProfileManager builds a profile manager rooted at workspaceRoot.
func NewBrowserProfileManager(workspaceRoot string) BrowserProfileManager {
	return BrowserProfileManager{
		Paths:  NewBrowserPaths(workspaceRoot),
		Source: NewBrowserProfileSourceResolver(),
	}
}

// MotherProfileDir returns the project-owned mother profile directory for the browser family.
func (m BrowserProfileManager) MotherProfileDir(browserType BrowserType) string {
	return m.Paths.BrowserUserDataDir(browserType)
}

// SourceProfileDir returns the actual system browser profile directory for the browser family.
func (m BrowserProfileManager) SourceProfileDir(browserType BrowserType) (string, error) {
	return m.Source.Resolve(browserType)
}

// TaskProfileRoot returns the exact task-scoped profile root for a browser task.
func (m BrowserProfileManager) TaskProfileRoot(browserType BrowserType, workerID, taskID string) string {
	return m.Paths.TaskBrowserRoot(browserType, workerID, taskID)
}

// TaskOriginalUserData returns the exact task-scoped original-userdata directory.
func (m BrowserProfileManager) TaskOriginalUserData(browserType BrowserType, workerID, taskID string) string {
	return filepath.Join(m.TaskProfileRoot(browserType, workerID, taskID), "original-userdata")
}

// TaskContentUserData returns the exact task-scoped content profile directory.
func (m BrowserProfileManager) TaskContentUserData(browserType BrowserType, workerID, taskID string) string {
	return filepath.Join(m.TaskProfileRoot(browserType, workerID, taskID), "content")
}

// TaskVerifyUserData returns the exact task-scoped verification profile directory.
func (m BrowserProfileManager) TaskVerifyUserData(browserType BrowserType, workerID, taskID string) string {
	return filepath.Join(m.TaskProfileRoot(browserType, workerID, taskID), "verify")
}

// PrepareTaskProfile copies the latest mother profile into a fresh task-scoped temp directory.
func (m BrowserProfileManager) PrepareTaskProfile(browserType BrowserType, workerID, taskID string) (BrowserTaskProfile, error) {
	return m.PrepareTaskProfileFromSource(browserType, m.MotherProfileDir(browserType), workerID, taskID)
}

// PreparePlaywrightProfileFromSource copies a selected source profile into a fresh temporary Playwright profile dir.
func (m BrowserProfileManager) PreparePlaywrightProfileFromSource(browserType BrowserType, sourceDir string) (BrowserPlaywrightProfile, error) {
	sourceDir = filepath.Clean(strings.TrimSpace(sourceDir))
	if sourceDir == "" {
		return BrowserPlaywrightProfile{}, fmt.Errorf("source profile dir is empty")
	}
	if _, err := os.Stat(sourceDir); err != nil {
		return BrowserPlaywrightProfile{}, fmt.Errorf("source profile %q: %w", sourceDir, err)
	}

	tasksRoot, err := filepath.Abs(m.Paths.BrowserTasksRoot)
	if err != nil {
		return BrowserPlaywrightProfile{}, fmt.Errorf("resolve browser tasks root %q: %w", m.Paths.BrowserTasksRoot, err)
	}
	tasksRoot = filepath.Clean(tasksRoot)
	if err := os.MkdirAll(tasksRoot, 0o755); err != nil {
		return BrowserPlaywrightProfile{}, fmt.Errorf("create browser tasks root %q: %w", tasksRoot, err)
	}

	tempRoot, err := os.MkdirTemp(tasksRoot, string(browserType)+"-playwright-*")
	if err != nil {
		return BrowserPlaywrightProfile{}, fmt.Errorf("create temp playwright profile root: %w", err)
	}
	tempRoot = filepath.Clean(tempRoot)
	if err := copyDir(sourceDir, tempRoot); err != nil {
		_ = os.RemoveAll(tempRoot)
		return BrowserPlaywrightProfile{}, fmt.Errorf("copy source profile to temp playwright profile: %w", err)
	}

	return BrowserPlaywrightProfile{
		BrowserType:      browserType,
		SourceProfileDir: sourceDir,
		RootDir:          tempRoot,
	}, nil
}

// PrepareTaskProfileFromSource copies a selected source profile into a fresh task-scoped temp directory.
func (m BrowserProfileManager) PrepareTaskProfileFromSource(browserType BrowserType, sourceDir, workerID, taskID string) (BrowserTaskProfile, error) {
	sourceDir = filepath.Clean(strings.TrimSpace(sourceDir))
	if sourceDir == "" {
		return BrowserTaskProfile{}, fmt.Errorf("source profile dir is empty")
	}
	if _, err := os.Stat(sourceDir); err != nil {
		return BrowserTaskProfile{}, fmt.Errorf("source profile %q: %w", sourceDir, err)
	}

	taskRoot := m.TaskProfileRoot(browserType, workerID, taskID)
	if err := os.RemoveAll(taskRoot); err != nil {
		return BrowserTaskProfile{}, fmt.Errorf("clear task profile root %q: %w", taskRoot, err)
	}

	original := m.TaskOriginalUserData(browserType, workerID, taskID)
	content := m.TaskContentUserData(browserType, workerID, taskID)
	verify := m.TaskVerifyUserData(browserType, workerID, taskID)

	if err := copyDir(sourceDir, original); err != nil {
		_ = os.RemoveAll(taskRoot)
		return BrowserTaskProfile{}, fmt.Errorf("copy source profile to task profile: %w", err)
	}
	if err := copyDir(original, content); err != nil {
		_ = os.RemoveAll(taskRoot)
		return BrowserTaskProfile{}, fmt.Errorf("copy original-userdata to content: %w", err)
	}
	if err := copyDir(original, verify); err != nil {
		_ = os.RemoveAll(taskRoot)
		return BrowserTaskProfile{}, fmt.Errorf("copy original-userdata to verify: %w", err)
	}

	return BrowserTaskProfile{
		BrowserType:      browserType,
		WorkerID:         workerID,
		TaskID:           taskID,
		MotherProfileDir: sourceDir,
		RootDir:          taskRoot,
		OriginalUserData: original,
		ContentUserData:  content,
		VerifyUserData:   verify,
	}, nil
}

// CleanupTaskProfile removes the entire task-scoped profile root.
func (m BrowserProfileManager) CleanupTaskProfile(browserType BrowserType, workerID, taskID string) error {
	return os.RemoveAll(m.TaskProfileRoot(browserType, workerID, taskID))
}

// CleanupPlaywrightProfile removes one temporary Playwright profile dir.
func (m BrowserProfileManager) CleanupPlaywrightProfile(profile BrowserPlaywrightProfile) error {
	return os.RemoveAll(profile.RootDir)
}

// BrowserTaskProfile describes one prepared task profile copy.
type BrowserTaskProfile struct {
	BrowserType      BrowserType `json:"browserType"`
	WorkerID         string      `json:"workerId"`
	TaskID           string      `json:"taskId"`
	MotherProfileDir string      `json:"motherProfileDir"`
	RootDir          string      `json:"rootDir"`
	OriginalUserData string      `json:"originalUserData"`
	ContentUserData  string      `json:"contentUserData"`
	VerifyUserData   string      `json:"verifyUserData"`
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source %q is not a directory", src)
	}
	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := copyFile(srcPath, dstPath, info.Mode()); err != nil {
				return err
			}
		default:
			if err := copyFile(srcPath, dstPath, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
