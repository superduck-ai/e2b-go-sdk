package e2b_test

import (
	"errors"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsErrorsReferenceDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sdk-reference/go-sdk/errors.mdx"); err != nil {
		t.Fatalf("errors reference doc is missing: %v", err)
	}
}

// This test keeps docs/sdk-reference/go-sdk/errors.mdx aligned with the
// exported Go SDK error surface. The closures are compile-only examples and
// are intentionally never executed.
func TestDocsErrorsReferenceExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "not-found-chain",
			fn: func() {
				err := &e2b.FileNotFoundError{
					NotFoundError: e2b.NotFoundError{
						SandboxError: e2b.SandboxError{Message: "missing file"},
					},
				}

				var fileErr *e2b.FileNotFoundError
				var notFoundErr *e2b.NotFoundError
				var sandboxErr *e2b.SandboxError

				_ = errors.As(err, &fileErr)
				_ = errors.As(err, &notFoundErr)
				_ = errors.As(err, &sandboxErr)
			},
		},
		{
			name: "auth-chain",
			fn: func() {
				err := &e2b.GitAuthError{
					AuthenticationError: e2b.AuthenticationError{Message: "authentication failed"},
				}

				var gitAuthErr *e2b.GitAuthError
				var authErr *e2b.AuthenticationError

				_ = errors.As(err, &gitAuthErr)
				_ = errors.As(err, &authErr)
			},
		},
		{
			name: "build-chain",
			fn: func() {
				err := &e2b.FileUploadError{
					BuildError: e2b.BuildError{
						Message:     "upload failed",
						CallerTrace: "main.main\n\t/app/main.go:42",
					},
				}

				var uploadErr *e2b.FileUploadError
				var buildErr *e2b.BuildError

				_ = errors.As(err, &uploadErr)
				_ = errors.As(err, &buildErr)
				_ = buildErr.CallerTrace
			},
		},
		{
			name: "command-exit",
			fn: func() {
				err := &e2b.CommandExitError{
					CommandResult: e2b.CommandResult{
						ExitCode: 23,
						Error:    "command failed",
						Stdout:   "stdout",
						Stderr:   "stderr",
					},
					Message: "command failed",
				}

				var exitErr *e2b.CommandExitError
				_ = errors.As(err, &exitErr)
				_ = exitErr.ExitCode
				_ = exitErr.Stdout
				_ = exitErr.Stderr
				_ = exitErr.Error
			},
		},
		{
			name: "other-exported-errors",
			fn: func() {
				timeoutErr := &e2b.TimeoutError{SandboxError: e2b.SandboxError{Message: "timed out"}}
				invalidErr := &e2b.InvalidArgumentError{SandboxError: e2b.SandboxError{Message: "bad input"}}
				spaceErr := &e2b.NotEnoughSpaceError{SandboxError: e2b.SandboxError{Message: "full"}}
				sandboxNotFoundErr := &e2b.SandboxNotFoundError{
					NotFoundError: e2b.NotFoundError{
						SandboxError: e2b.SandboxError{Message: "sandbox missing"},
					},
				}
				upstreamErr := &e2b.GitUpstreamError{SandboxError: e2b.SandboxError{Message: "missing upstream"}}
				templateErr := &e2b.TemplateError{SandboxError: e2b.SandboxError{Message: "old envd"}}
				rateErr := &e2b.RateLimitError{SandboxError: e2b.SandboxError{Message: "slow down"}}
				volumeErr := &e2b.VolumeError{Message: "volume failed"}

				_ = timeoutErr
				_ = invalidErr
				_ = spaceErr
				_ = sandboxNotFoundErr
				_ = upstreamErr
				_ = templateErr
				_ = rateErr
				_ = volumeErr
			},
		},
		{
			name: "match-file-not-found",
			fn: func() {
				var err error = &e2b.FileNotFoundError{
					NotFoundError: e2b.NotFoundError{
						SandboxError: e2b.SandboxError{Message: "missing path"},
					},
				}

				var fileErr *e2b.FileNotFoundError
				_ = errors.As(err, &fileErr)
			},
		},
		{
			name: "match-sandbox-not-found",
			fn: func() {
				var err error = &e2b.SandboxNotFoundError{
					NotFoundError: e2b.NotFoundError{
						SandboxError: e2b.SandboxError{Message: "missing sandbox"},
					},
				}

				var sandboxErr *e2b.SandboxNotFoundError
				_ = errors.As(err, &sandboxErr)
			},
		},
		{
			name: "match-timeout",
			fn: func() {
				var err error = &e2b.TimeoutError{
					SandboxError: e2b.SandboxError{Message: "timed out"},
				}

				var timeoutErr *e2b.TimeoutError
				_ = errors.As(err, &timeoutErr)
			},
		},
		{
			name: "match-generic-sandbox-error",
			fn: func() {
				var err error = &e2b.TemplateError{
					SandboxError: e2b.SandboxError{Message: "template failed"},
				}

				var sandboxErr *e2b.SandboxError
				_ = errors.As(err, &sandboxErr)
			},
		},
	}

	if got := len(snippets); got != 9 {
		t.Fatalf("expected 9 errors doc snippets, got %d", got)
	}
}
