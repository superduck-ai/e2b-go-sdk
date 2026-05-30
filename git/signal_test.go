package git

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/commands"
)

func TestGitApisHonorSignalContext(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		requestStarted <- struct{}{}
		<-release
	}))
	defer server.Close()

	signal, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		timeout := 1000
		cmds := commands.NewCommands(&struct {
			ApiKey           string
			AccessToken      string
			Domain           string
			ApiUrl           string
			SandboxUrl       string
			Debug            bool
			RequestTimeoutMs int
			Headers          map[string]string
		}{
			SandboxUrl: server.URL,
			Headers:    map[string]string{},
		}, "1.0.0")
		g := NewGit(cmds)
		_, err := g.Init(context.Background(), "/tmp/repo", &GitInitOpts{
			GitRequestOpts: GitRequestOpts{
				RequestTimeoutMs: &timeout,
				Signal:           signal,
			},
		})
		done <- err
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
