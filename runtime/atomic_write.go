package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFileAtomic writes data to a temporary file in the destination directory
// and renames it into place only after the full payload has been flushed.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("atomic write path is empty")
	}
	path = filepath.Clean(path)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create atomic write dir %q: %w", dir, err)
	}
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %q: %w", path, err)
	}
	tempPath := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(perm); err != nil {
		return fmt.Errorf("set temporary file mode for %q: %w", path, err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write temporary file for %q: %w", path, err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("flush temporary file for %q: %w", path, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file for %q: %w", path, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace %q atomically: %w", path, err)
	}
	committed = true
	return nil
}
