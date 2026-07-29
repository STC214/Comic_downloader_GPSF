package tasks

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	projectruntime "comic_downloader_go_playwright_stealth/runtime"
)

type canceledWaitSession struct {
	closed chan struct{}
	once   sync.Once
}

type stuckWaitSession struct {
	*canceledWaitSession
}

func (*stuckWaitSession) Close() error {
	return nil
}

func newCanceledWaitSession() *canceledWaitSession {
	return &canceledWaitSession{closed: make(chan struct{})}
}

func (s *canceledWaitSession) Close() error {
	s.once.Do(func() { close(s.closed) })
	return errors.New("browser close noise")
}

func (s *canceledWaitSession) WaitClosed() error {
	<-s.closed
	return errors.New("browser wait noise")
}

func (*canceledWaitSession) Title() (string, error)            { return "", nil }
func (*canceledWaitSession) PageURL() string                   { return "" }
func (*canceledWaitSession) Content() (string, error)          { return "", nil }
func (*canceledWaitSession) Goto(string) error                 { return nil }
func (*canceledWaitSession) ClickText(string) error            { return nil }
func (*canceledWaitSession) LoadLazyContent() error            { return nil }
func (*canceledWaitSession) LoadLazyContentForCount(int) error { return nil }

func TestRunBrowserRequestRejectsEmptyURL(t *testing.T) {
	_, err := RunBrowserRequest(BrowserLaunchRequest{})
	if err == nil {
		t.Fatal("RunBrowserRequest() error = nil, want error")
	}
}

func TestRunBrowserRequestRejectsUnsupportedSiteWithoutLaunchingBrowser(t *testing.T) {
	_, err := RunBrowserRequest(BrowserLaunchRequest{URL: "https://example.com/gallery/1"})
	if err == nil || !strings.Contains(err.Error(), "unsupported download site") {
		t.Fatalf("RunBrowserRequest() error = %v, want unsupported-site error", err)
	}
}

func TestRunBrowserRequestHonorsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunBrowserRequest(BrowserLaunchRequest{
		URL:     "https://zeri-m.top/example",
		Context: ctx,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunBrowserRequest() error = %v, want context.Canceled", err)
	}
}

func TestCreateTaskThumbnailUsesPortableDataThumbnailDirectory(t *testing.T) {
	portableDataRoot := filepath.Join(t.TempDir(), "portable-data")
	sourceDir := filepath.Join(t.TempDir(), "download")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceDir, "1.png")
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	pixel := image.NewRGBA(image.Rect(0, 0, 1, 1))
	pixel.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(file, pixel); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := createTaskThumbnail(projectruntime.NewPathsFromRuntimeRoot(portableDataRoot), "todo-42", sourceDir, []string{source})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(portableDataRoot, "thumbnails", "task-todo-42", "thumb.jpg")
	if got != want {
		t.Fatalf("thumbnail path = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("thumbnail was not created at %q: %v", want, err)
	}
}

func TestWaitForBrowserCloseReturnsCancellationInsteadOfCloseNoise(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	session := newCanceledWaitSession()
	cancel()

	err := waitForBrowserCloseOrSignal(ctx, session)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForBrowserCloseOrSignal() error = %v, want context.Canceled", err)
	}
}

func TestWaitForBrowserCloseCancellationDoesNotBlockOnStuckSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	base := newCanceledWaitSession()
	session := &stuckWaitSession{canceledWaitSession: base}
	cancel()

	start := time.Now()
	err := waitForBrowserCloseOrSignalWithTimeout(ctx, session, 20*time.Millisecond)
	base.once.Do(func() { close(base.closed) })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForBrowserCloseOrSignalWithTimeout() error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("cancellation blocked for %s", elapsed)
	}
}
