package doctest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsUseCasesCICDDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/use-cases/ci-cd.mdx"); err != nil {
		t.Fatalf("use-cases ci-cd doc is missing: %v", err)
	}
}

// This test keeps docs/use-cases/ci-cd.mdx aligned with the exported Go SDK
// workflow for CI sandbox review jobs. The closures are compile-only examples
// and are intentionally never executed.
func TestDocsUseCasesCICDExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "review-script",
			fn: func() {
				reviewDiff := func(ctx context.Context, diff string) (string, error) {
					return fmt.Sprintf("Review placeholder for diff with %d bytes", len(diff)), nil
				}

				postPRComment := func(ctx context.Context, repo string, prNumber int, token string, body string) error {
					payload, err := json.Marshal(map[string]string{
						"body": body,
					})
					if err != nil {
						return err
					}

					req, err := http.NewRequestWithContext(
						ctx,
						http.MethodPost,
						fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments", repo, prNumber),
						bytes.NewReader(payload),
					)
					if err != nil {
						return err
					}

					req.Header.Set("Authorization", "Bearer "+token)
					req.Header.Set("Content-Type", "application/json")

					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						return err
					}
					defer resp.Body.Close()

					if resp.StatusCode >= 300 {
						return fmt.Errorf("github comment failed: %s", resp.Status)
					}

					return nil
				}

				ctx := context.Background()
				timeoutMs := int((5 * time.Minute) / time.Millisecond)
				depth := 1

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					TimeoutMs: &timeoutMs,
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				repoURL := "https://github.com/" + os.Getenv("PR_REPO") + ".git"
				repoPath := "/home/user/repo"

				cloneResult, cloneErr := sandbox.Git.Clone(ctx, repoURL, &e2b.GitCloneOpts{
					Path:     repoPath,
					Branch:   os.Getenv("PR_BRANCH"),
					Depth:    &depth,
					Username: "x-access-token",
					Password: os.Getenv("GITHUB_TOKEN"),
				})

				diffExecution, diffErr := sandbox.Commands.Run(ctx, "cd /home/user/repo && git diff origin/"+os.Getenv("PR_BASE_BRANCH")+"...HEAD", nil)
				diffResult := diffExecution.(*e2b.CommandResult)

				review, reviewErr := reviewDiff(ctx, diffResult.Stdout)

				_, testErr := sandbox.Commands.Run(ctx, "cd /home/user/repo && go test ./...", &e2b.CommandStartOpts{
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
					OnStderr: func(data e2b.Stderr) {
						_ = data
					},
				})
				if testErr != nil {
					var exitErr *e2b.CommandExitError
					_ = errors.As(testErr, &exitErr)
					if exitErr != nil {
						_ = exitErr.ExitCode
					}
				}

				prNumber, atoiErr := strconv.Atoi(os.Getenv("PR_NUMBER"))
				commentErr := postPRComment(ctx, os.Getenv("GITHUB_REPOSITORY"), prNumber, os.Getenv("GITHUB_TOKEN"), "## AI Code Review\n\n"+review)

				_ = cloneResult
				_ = cloneErr
				_ = diffResult
				_ = diffErr
				_ = reviewErr
				_ = testErr
				_ = atoiErr
				_ = commentErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 use-cases ci-cd doc snippet, got %d", got)
	}
}
