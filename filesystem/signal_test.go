package filesystem

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func runSignalCancellation(t *testing.T, expectedPath string, invoke func(signal context.Context, sandboxURL string) error) {
	t.Helper()

	requestStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectedPath {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		requestStarted <- struct{}{}
		<-release
	}))
	defer server.Close()

	signal, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- invoke(signal, server.URL)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for request to start")
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected signal context cancellation, got %T %v", err, err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for signal cancellation")
	}

	close(release)
}

func TestFilesystemApisHonorSignalContext(t *testing.T) {
	t.Run("read_stream", func(t *testing.T) {
		runSignalCancellation(t, "/files", func(signal context.Context, sandboxURL string) error {
			timeout := 1000
			fs := NewFilesystem(testFilesystemConfig(sandboxURL, 0), "1.0.0")
			_, err := fs.Read(context.Background(), "/tmp/file.txt", &FilesystemReadOpts{
				FilesystemRequestOpts: FilesystemRequestOpts{
					RequestTimeoutMs: &timeout,
					Signal:           signal,
				},
				Format: ReadFormatStream,
			})
			return err
		})
	})

	t.Run("get_info", func(t *testing.T) {
		runSignalCancellation(t, "/filesystem.Filesystem/Stat", func(signal context.Context, sandboxURL string) error {
			timeout := 1000
			fs := NewFilesystem(testFilesystemConfig(sandboxURL, 0), "1.0.0")
			_, err := fs.GetInfo(context.Background(), "/tmp/file.txt", &FilesystemRequestOpts{
				RequestTimeoutMs: &timeout,
				Signal:           signal,
			})
			return err
		})
	})

	t.Run("watch_dir", func(t *testing.T) {
		runSignalCancellation(t, "/filesystem.Filesystem/WatchDir", func(signal context.Context, sandboxURL string) error {
			timeout := 1000
			fs := NewFilesystem(testFilesystemConfig(sandboxURL, 0), "1.0.0")
			_, err := fs.WatchDir(context.Background(), "/tmp", nil, &WatchOpts{
				FilesystemRequestOpts: FilesystemRequestOpts{
					RequestTimeoutMs: &timeout,
					Signal:           signal,
				},
			})
			return err
		})
	})
}
