package assets

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDownloadImagesContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://hitomi.la/" {
			t.Errorf("Referer = %q", r.Header.Get("Referer"))
		}
		w.Header().Set("Content-Type", "image/png")
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		img.Set(0, 0, color.White)
		if err := png.Encode(w, img); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	root := t.TempDir()
	result, err := DownloadImagesContext(context.Background(), CollectionSummary{
		Site: "hitomi", Title: `Title: invalid/name`, BaseURL: "https://hitomi.la/",
	}, []string{server.URL + "/a.webp", server.URL + "/b.webp"}, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 2 || result.Bytes == 0 {
		t.Fatalf("result = %#v", result)
	}
	for _, file := range result.Files {
		if _, err := os.Stat(file); err != nil {
			t.Fatal(err)
		}
		if filepath.Dir(file) != result.OutputDir {
			t.Fatalf("file outside output dir: %q", file)
		}
	}
}

func TestDownloadImagesContextRejectsInvalidImageAndRemovesPartFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>not an image</html>"))
	}))
	defer server.Close()
	root := t.TempDir()
	_, err := DownloadImagesContext(context.Background(), CollectionSummary{
		Site: "example", Title: "invalid",
	}, []string{server.URL + "/page.jpg"}, root, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid image data") {
		t.Fatalf("error = %v, want invalid-image error", err)
	}
	parts, err := filepath.Glob(filepath.Join(root, "invalid", "*.part"))
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 0 {
		t.Fatalf("partial files remain: %v", parts)
	}
}

func TestDownloadProgressIsSerializedAndMonotonic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	}))
	defer server.Close()
	var mu sync.Mutex
	var values []int
	_, err := DownloadImagesContext(context.Background(), CollectionSummary{
		Site: "example", Title: "progress",
	}, []string{server.URL + "/a.png", server.URL + "/b.png", server.URL + "/c.png"}, t.TempDir(), func(update DownloadProgress) {
		mu.Lock()
		values = append(values, update.Current)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, value := range values {
		if value != i+1 {
			t.Fatalf("progress = %v, want monotonic sequence", values)
		}
	}
}

func TestDownloadImagesContextHonorsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := DownloadImagesContext(ctx, CollectionSummary{Title: "canceled"}, []string{"https://example.com/a.jpg"}, t.TempDir(), nil)
	if err != context.Canceled {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestSanitizePathPartAvoidsWindowsDeviceNames(t *testing.T) {
	if got := sanitizePathPart("CON"); got != "CON_" {
		t.Fatalf("sanitizePathPart(CON) = %q", got)
	}
}

func TestFailedBatchLeavesExistingOutputDirectoryUnchanged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "bad") {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>bad</html>"))
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	}))
	defer server.Close()
	root := t.TempDir()
	outputDir := filepath.Join(root, "transaction")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(outputDir, "existing.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := DownloadImagesContext(context.Background(), CollectionSummary{
		Site: "example", Title: "transaction",
	}, []string{server.URL + "/good.png", server.URL + "/bad.png"}, root, nil)
	if err == nil {
		t.Fatal("DownloadImagesContext() error = nil, want failure")
	}
	data, readErr := os.ReadFile(marker)
	if readErr != nil || string(data) != "keep" {
		t.Fatalf("existing output changed after failed batch: data=%q err=%v", data, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "0001.png")); !os.IsNotExist(statErr) {
		t.Fatalf("partially downloaded file was committed: %v", statErr)
	}
	staging, globErr := filepath.Glob(filepath.Join(root, ".transaction.staging-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(staging) != 0 {
		t.Fatalf("staging directories remain: %v", staging)
	}
}

func TestSuccessfulBatchAtomicallyReplacesExistingOutputDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	}))
	defer server.Close()
	root := t.TempDir()
	outputDir := filepath.Join(root, "replace")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "stale.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := DownloadImagesContext(context.Background(), CollectionSummary{
		Site: "example", Title: "replace",
	}, []string{server.URL + "/new.png"}, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale output survived successful replacement: %v", err)
	}
	if len(result.Files) != 1 || filepath.Dir(result.Files[0]) != outputDir {
		t.Fatalf("result files = %v, outputDir = %q", result.Files, outputDir)
	}
}

func TestRecoverInterruptedCommitRestoresPreviousOutput(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "recover")
	backupDir := filepath.Join(root, ".recover.previous-crash")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(backupDir, "existing.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := recoverInterruptedCommit(outputDir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(outputDir, "existing.txt"))
	if err != nil || string(data) != "keep" {
		t.Fatalf("recovered marker = %q, err = %v", data, err)
	}
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Fatalf("backup still exists after recovery: %v", err)
	}
}

func TestRecoverInterruptedCommitRemovesStaleBackupWhenOutputExists(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "recover")
	backupDir := filepath.Join(root, ".recover.previous-stale")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := recoverInterruptedCommit(outputDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outputDir); err != nil {
		t.Fatalf("current output was removed: %v", err)
	}
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Fatalf("stale backup still exists: %v", err)
	}
}

func TestConcurrentBatchesForSameTitleCommitAsWholeDirectories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pixel := color.RGBA{R: 255, A: 255}
		if strings.HasPrefix(r.URL.Path, "/blue/") {
			pixel = color.RGBA{B: 255, A: 255}
		}
		w.Header().Set("Content-Type", "image/png")
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.SetRGBA(0, 0, pixel)
		_ = png.Encode(w, img)
	}))
	defer server.Close()

	root := t.TempDir()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, batch := range []string{"red", "blue"} {
		batch := batch
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := DownloadImagesContext(context.Background(), CollectionSummary{
				Site: "example", Title: "same-title",
			}, []string{
				server.URL + "/" + batch + "/1.png",
				server.URL + "/" + batch + "/2.png",
			}, root, nil)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	files, err := filepath.Glob(filepath.Join(root, "same-title", "*.png"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("committed files = %v, want exactly two", files)
	}
	var first color.Color
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		img, _, err := image.Decode(file)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		got := img.At(0, 0)
		if first == nil {
			first = got
			continue
		}
		r1, g1, b1, a1 := first.RGBA()
		r2, g2, b2, a2 := got.RGBA()
		if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
			t.Fatal("final output contains files mixed from concurrent batches")
		}
	}
}
