//go:build windows

package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	projectruntime "comic_downloader_go_playwright_stealth/runtime"
)

// payloadZip contains the packaged portable app payload. The build script replaces
// this file before building the launcher exe.
//
//go:embed payload.zip
var payloadZip []byte

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	launcherExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve launcher executable: %w", err)
	}
	launcherRoot := filepath.Dir(launcherExe)
	// Portable metadata belongs to the launcher, not to the shell that happened
	// to start it. Thumbnails therefore always remain beside the executable.
	portableDataRoot := filepath.Join(launcherRoot, "portable-data")
	if err := os.MkdirAll(portableDataRoot, 0o755); err != nil {
		return fmt.Errorf("create portable data root: %w", err)
	}
	if err := migratePortableDataRoot(portableDataRoot); err != nil {
		return err
	}
	if cleanup, logPath, err := projectruntime.InitProcessLogging(portableDataRoot, "portable-launcher"); err != nil {
		return fmt.Errorf("init portable launcher logging: %w", err)
	} else {
		log.Printf("portable launcher logging: %s", logPath)
		defer func() {
			if err := cleanup(); err != nil {
				log.Printf("close portable launcher log: %v", err)
			}
		}()
	}
	tempRoot, err := os.MkdirTemp(portableDataRoot, "payload-*")
	if err != nil {
		return fmt.Errorf("create temp root: %w", err)
	}
	defer func() {
		if err := removeAllWithRetry(tempRoot, 8, 250*time.Millisecond); err != nil {
			log.Printf("remove portable payload %q failed: %v", tempRoot, err)
		}
	}()

	if err := unzipPayload(payloadZip, tempRoot); err != nil {
		return fmt.Errorf("extract portable payload: %w", err)
	}
	if err := syncPortableAssets(tempRoot, portableDataRoot); err != nil {
		return err
	}

	appPath := filepath.Join(tempRoot, "comic_downloader.exe")
	if _, err := os.Stat(appPath); err != nil {
		return fmt.Errorf("portable app not found after extraction: %w", err)
	}

	cmd := exec.Command(appPath, os.Args[1:]...)
	cmd.Dir = tempRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	playwrightRoot := filepath.Join(portableDataRoot, "playwright-browsers")
	cmd.Env = portableChildEnvironment(os.Environ(), portableDataRoot, playwrightRoot)

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start portable app: %w", err)
	}
	waitErr := make(chan error, 1)
	go func() {
		waitErr <- cmd.Wait()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	var errWait error
	select {
	case errWait = <-waitErr:
	case sig := <-sigCh:
		log.Printf("portable launcher interrupted by %s; closing child app and cleaning temp files", sig)
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case errWait = <-waitErr:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			errWait = <-waitErr
		}
	}
	log.Printf("portable app exited after %s: %v", time.Since(start).Round(time.Millisecond), errWait)
	return nil
}

func portableChildEnvironment(base []string, portableDataRoot, playwrightRoot string) []string {
	overrides := map[string]string{
		"PLAYWRIGHT_BROWSERS_PATH":             defaultEnv("PLAYWRIGHT_BROWSERS_PATH", playwrightRoot),
		"PLAYWRIGHT_DRIVER_PATH":               defaultEnv("PLAYWRIGHT_DRIVER_PATH", filepath.Join(playwrightRoot, "driver")),
		"COMIC_DOWNLOADER_WORKSPACE_ROOT":      portableDataRoot,
		"COMIC_DOWNLOADER_RUNTIME_ROOT":        portableDataRoot,
		"COMIC_DOWNLOADER_LOG_ROOT":            filepath.Join(portableDataRoot, "logs"),
		"COMIC_DOWNLOADER_DOWNLOAD_DIR":        defaultEnv("COMIC_DOWNLOADER_DOWNLOAD_DIR", filepath.Join(portableDataRoot, "output")),
		"COMIC_DOWNLOADER_FRONTEND_STATE_PATH": filepath.Join(portableDataRoot, "frontend_state.json"),
		"COMIC_DOWNLOADER_STATE_PATH":          filepath.Join(portableDataRoot, "comic_downloader_state.json"),
	}
	env := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replaced := overrides[strings.ToUpper(key)]; replaced {
				continue
			}
		}
		env = append(env, entry)
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}

func migratePortableDataRoot(portableDataRoot string) error {
	oldRuntimeRoot := filepath.Join(portableDataRoot, "runtime")
	info, err := os.Stat(oldRuntimeRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat legacy portable runtime root: %w", err)
	}
	if !info.IsDir() {
		return nil
	}
	if err := copyTree(oldRuntimeRoot, portableDataRoot); err != nil {
		return fmt.Errorf("migrate legacy portable runtime root: %w", err)
	}
	if err := os.RemoveAll(oldRuntimeRoot); err != nil {
		return fmt.Errorf("remove legacy portable runtime root: %w", err)
	}
	log.Printf("migrated legacy portable runtime root into %s", portableDataRoot)
	return nil
}

func unzipPayload(data []byte, dest string) error {
	if len(data) == 0 {
		return fmt.Errorf("payload zip is empty")
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		target := filepath.Join(dest, filepath.FromSlash(file.Name))
		relative, err := filepath.Rel(dest, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("zip entry escapes destination: %q", file.Name)
		}
		isDir := file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/") || strings.HasSuffix(file.Name, "\\")
		log.Printf("extract entry: %s dir=%v target=%s", file.Name, isDir, target)
		if isDir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				log.Printf("skip dir entry mkdir failed: %v", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			log.Printf("mkdir parent failed: %v", err)
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			_ = src.Close()
			return err
		}
		if _, err := io.Copy(dst, src); err != nil {
			_ = dst.Close()
			_ = src.Close()
			return err
		}
		_ = dst.Close()
		_ = src.Close()
	}
	return nil
}

func syncPortableAssets(tempRoot, portableDataRoot string) error {
	runtimeSource := filepath.Join(tempRoot, "runtime")
	if err := copyTree(runtimeSource, portableDataRoot); err != nil {
		return fmt.Errorf("sync portable runtime assets: %w", err)
	}
	adblockSource := filepath.Join(tempRoot, "adblock")
	adblockTarget := filepath.Join(portableDataRoot, "adblock")
	if err := copyTree(adblockSource, adblockTarget); err != nil {
		return fmt.Errorf("sync portable adblock assets: %w", err)
	}
	if err := removeStalePortableDataFiles(portableDataRoot); err != nil {
		return err
	}
	return nil
}

func removeStalePortableDataFiles(portableDataRoot string) error {
	staleFiles := []string{
		filepath.Join(portableDataRoot, "runtime", "comic_downloader_state.json"),
		filepath.Join(portableDataRoot, "runtime", "frontend_state.json"),
	}
	for _, path := range staleFiles {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale portable data file %q: %w", path, err)
		}
	}
	return nil
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return copyFile(src, dst)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(src, entry.Name())
		targetPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyTree(sourcePath, targetPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(sourcePath, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	temp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".copy-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := io.Copy(temp, in); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, dst); err != nil {
		if removeErr := os.Remove(dst); removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
		if err := os.Rename(tempPath, dst); err != nil {
			return err
		}
	}
	committed = true
	return nil
}

func removeAllWithRetry(path string, attempts int, delay time.Duration) error {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := os.RemoveAll(path); err != nil {
			lastErr = err
		} else if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil
		} else if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("path still exists")
		}
		if attempt < attempts {
			time.Sleep(delay)
		}
	}
	return lastErr
}

func defaultEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
