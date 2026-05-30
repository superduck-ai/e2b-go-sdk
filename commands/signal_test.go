package commands

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

func TestCommandsApisHonorSignalContext(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		runSignalCancellation(t, "/process.Process/List", func(signal context.Context, sandboxURL string) error {
			timeout := 1000
			cmds := NewCommands(testCommandsConfig(sandboxURL, 0), "1.0.0")
			_, err := cmds.List(context.Background(), &CommandRequestOpts{
				RequestTimeoutMs: &timeout,
				Signal:           signal,
			})
			return err
		})
	})

	t.Run("connect", func(t *testing.T) {
		runSignalCancellation(t, "/process.Process/Connect", func(signal context.Context, sandboxURL string) error {
			timeout := 1000
			cmds := NewCommands(testCommandsConfig(sandboxURL, 0), "1.0.0")
			_, err := cmds.Connect(context.Background(), 123, &CommandConnectOpts{
				CommandRequestOpts: CommandRequestOpts{
					RequestTimeoutMs: &timeout,
					Signal:           signal,
				},
			})
			return err
		})
	})

	t.Run("run", func(t *testing.T) {
		runSignalCancellation(t, "/process.Process/Start", func(signal context.Context, sandboxURL string) error {
			timeout := 1000
			cmds := NewCommands(testCommandsConfig(sandboxURL, 0), "1.0.0")
			_, err := cmds.Run(context.Background(), "sleep 10", &CommandStartOpts{
				CommandRequestOpts: CommandRequestOpts{
					RequestTimeoutMs: &timeout,
					Signal:           signal,
				},
			})
			return err
		})
	})
}

func TestPtyApisHonorSignalContext(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		runSignalCancellation(t, "/process.Process/Start", func(signal context.Context, sandboxURL string) error {
			timeout := 1000
			pty := NewPty(testCommandsConfig(sandboxURL, 0), "1.0.0")
			_, err := pty.Create(context.Background(), &PtyCreateOpts{
				RequestTimeoutMs: &timeout,
				Signal:           signal,
				OnData:           func(PtyOutput) {},
			})
			return err
		})
	})

	t.Run("connect", func(t *testing.T) {
		runSignalCancellation(t, "/process.Process/Connect", func(signal context.Context, sandboxURL string) error {
			timeout := 1000
			pty := NewPty(testCommandsConfig(sandboxURL, 0), "1.0.0")
			_, err := pty.Connect(context.Background(), 123, &PtyConnectOpts{
				RequestTimeoutMs: &timeout,
				Signal:           signal,
				OnData:           func(PtyOutput) {},
			})
			return err
		})
	})

	t.Run("kill", func(t *testing.T) {
		runSignalCancellation(t, "/process.Process/SendSignal", func(signal context.Context, sandboxURL string) error {
			timeout := 1000
			pty := NewPty(testCommandsConfig(sandboxURL, 0), "1.0.0")
			_, err := pty.Kill(context.Background(), 123, &CommandRequestOpts{
				RequestTimeoutMs: &timeout,
				Signal:           signal,
			})
			return err
		})
	})
}
