package e2b_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxAutoResumeDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/auto-resume.mdx"); err != nil {
		t.Fatalf("sandbox auto-resume doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/auto-resume.mdx aligned with the exported Go
// SDK lifecycle and host access surface. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsSandboxAutoResumeExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "configure-auto-resume",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 10 * 60 * 1000
				autoResume := true

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					TimeoutMs: &timeoutMs,
					Lifecycle: &e2b.SandboxLifecycle{
						OnTimeout:  "pause",
						AutoResume: &autoResume,
					},
				})
				if sandbox != nil {
					defer sandbox.Kill(context.Background(), nil)
				}

				_ = sandbox
				_ = err
			},
		},
		{
			name: "sdk-activity-resume",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 10 * 60 * 1000
				autoResume := true

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					TimeoutMs: &timeoutMs,
					Lifecycle: &e2b.SandboxLifecycle{
						OnTimeout:  "pause",
						AutoResume: &autoResume,
					},
				})
				if err != nil {
					return
				}

				_, writeErr := sandbox.Files.Write(ctx, "/home/user/hello.txt", "hello from go", nil)
				_, pauseErr := sandbox.Pause(ctx, nil)
				value, readErr := sandbox.Files.Read(ctx, "/home/user/hello.txt", nil)
				info, infoErr := sandbox.GetInfo(ctx, nil)
				content := value.(string)

				_ = content
				_ = info.State
				_ = writeErr
				_ = pauseErr
				_ = readErr
				_ = infoErr
			},
		},
		{
			name: "incoming-traffic-resume",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 5 * 60 * 1000
				autoResume := true

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					TimeoutMs: &timeoutMs,
					Lifecycle: &e2b.SandboxLifecycle{
						OnTimeout:  "pause",
						AutoResume: &autoResume,
					},
				})
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "python3 -m http.server 3000", &e2b.CommandStartOpts{
					Background: true,
				})
				handle := execution.(*e2b.CommandHandle)
				req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+sandbox.GetHost(3000), nil)
				if req != nil && sandbox.TrafficAccessToken != "" {
					req.Header.Set("e2b-traffic-access-token", sandbox.TrafficAccessToken)
				}

				_ = handle
				_ = req
				_ = runErr
				_ = reqErr
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 sandbox auto-resume doc snippets, got %d", got)
	}
}
