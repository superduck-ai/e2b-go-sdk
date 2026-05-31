package e2b_test

import (
	"context"
	"errors"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxGitReferenceDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sdk-reference/go-sdk/sandbox-git.mdx"); err != nil {
		t.Fatalf("sandbox git reference doc is missing: %v", err)
	}
}

// This test keeps docs/sdk-reference/go-sdk/sandbox-git.mdx aligned with the
// exported Go SDK surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsSandboxGitReferenceExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "init-and-clone",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				initResult, initErr := sandbox.Git.Init(ctx, "/tmp/repo", &e2b.GitInitOpts{
					InitialBranch: "main",
				})

				depth := 1
				cloneResult, cloneErr := sandbox.Git.Clone(ctx, "https://github.com/e2b-dev/e2b.git", &e2b.GitCloneOpts{
					Path:   "/tmp/e2b",
					Branch: "main",
					Depth:  &depth,
				})

				_ = initResult
				_ = cloneResult
				_ = initErr
				_ = cloneErr
			},
		},
		{
			name: "clone-with-inline-credentials",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				cloneResult, cloneErr := sandbox.Git.Clone(ctx, "https://git.example.com/org/repo.git", &e2b.GitCloneOpts{
					Path:                        "/tmp/repo",
					Username:                    "git",
					Password:                    "token",
					DangerouslyStoreCredentials: true,
				})

				_ = cloneResult
				_ = cloneErr
			},
		},
		{
			name: "dangerous-authenticate",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				result, authErr := sandbox.Git.DangerouslyAuthenticate(ctx, &e2b.GitDangerouslyAuthenticateOpts{
					Username: "git",
					Password: "token",
					Host:     "git.example.com",
					Protocol: "https",
				})

				_ = result
				_ = authErr
			},
		},
		{
			name: "auth-and-config",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				authResult, authErr := sandbox.Git.DangerouslyAuthenticate(ctx, &e2b.GitDangerouslyAuthenticateOpts{
					Username: "git",
					Password: "token",
					Host:     "git.example.com",
					Protocol: "https",
				})

				configureResult, configureErr := sandbox.Git.ConfigureUser(ctx, "E2B Bot", "bot@example.com", &e2b.GitConfigOpts{
					Scope: e2b.GitConfigScope("local"),
					Path:  "/tmp/repo",
				})

				remoteAddResult, remoteAddErr := sandbox.Git.RemoteAdd(ctx, "/tmp/repo", "origin", "https://git.example.com/org/repo.git", &e2b.GitRemoteAddOpts{
					Overwrite: true,
				})

				remoteURL, remoteGetErr := sandbox.Git.RemoteGet(ctx, "/tmp/repo", "origin", nil)
				configValue, getConfigErr := sandbox.Git.GetConfig(ctx, "user.name", &e2b.GitConfigOpts{
					Scope: e2b.GitConfigScope("local"),
					Path:  "/tmp/repo",
				})
				setConfigResult, setConfigErr := sandbox.Git.SetConfig(ctx, "pull.ff", "only", &e2b.GitConfigOpts{
					Scope: e2b.GitConfigScope("local"),
					Path:  "/tmp/repo",
				})

				_ = authResult
				_ = configureResult
				_ = remoteAddResult
				_ = remoteURL
				_ = configValue
				_ = setConfigResult
				_ = authErr
				_ = configureErr
				_ = remoteAddErr
				_ = remoteGetErr
				_ = getConfigErr
				_ = setConfigErr
			},
		},
		{
			name: "status-and-branches",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				status, statusErr := sandbox.Git.Status(ctx, "/tmp/repo", nil)
				if status != nil {
					_ = status.CurrentBranch
					_ = status.Upstream
					_ = status.Ahead
					_ = status.Behind
					_ = status.Detached
					_ = status.FileStatus
					_ = status.IsClean
					_ = status.HasChanges
					_ = status.HasStaged
					_ = status.HasUntracked
					_ = status.HasConflicts
					_ = status.TotalCount
					_ = status.StagedCount
					_ = status.UnstagedCount
					_ = status.UntrackedCount
					_ = status.ConflictCount
				}

				branches, branchesErr := sandbox.Git.Branches(ctx, "/tmp/repo", nil)
				if branches != nil {
					_ = branches.CurrentBranch
					_ = branches.Branches
				}

				_ = statusErr
				_ = branchesErr
			},
		},
		{
			name: "branch-management",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				createResult, createErr := sandbox.Git.CreateBranch(ctx, "/tmp/repo", "feature/docs", nil)
				checkoutResult, checkoutErr := sandbox.Git.CheckoutBranch(ctx, "/tmp/repo", "main", nil)
				deleteResult, deleteErr := sandbox.Git.DeleteBranch(ctx, "/tmp/repo", "feature/docs", &e2b.GitDeleteBranchOpts{
					Force: true,
				})

				_ = createResult
				_ = checkoutResult
				_ = deleteResult
				_ = createErr
				_ = checkoutErr
				_ = deleteErr
			},
		},
		{
			name: "branching-and-history",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				createResult, createErr := sandbox.Git.CreateBranch(ctx, "/tmp/repo", "feature/docs", nil)
				checkoutResult, checkoutErr := sandbox.Git.CheckoutBranch(ctx, "/tmp/repo", "main", nil)
				deleteResult, deleteErr := sandbox.Git.DeleteBranch(ctx, "/tmp/repo", "feature/docs", &e2b.GitDeleteBranchOpts{
					Force: true,
				})

				addResult, addErr := sandbox.Git.Add(ctx, "/tmp/repo", &e2b.GitAddOpts{
					Files: []string{"README.md"},
				})

				commitResult, commitErr := sandbox.Git.Commit(ctx, "/tmp/repo", "Update docs", &e2b.GitCommitOpts{
					AuthorName:  "E2B Bot",
					AuthorEmail: "bot@example.com",
					AllowEmpty:  false,
				})

				staged := true
				worktree := false
				restoreResult, restoreErr := sandbox.Git.Restore(ctx, "/tmp/repo", &e2b.GitRestoreOpts{
					Paths:    []string{"README.md"},
					Staged:   &staged,
					Worktree: &worktree,
				})

				resetResult, resetErr := sandbox.Git.Reset(ctx, "/tmp/repo", &e2b.GitResetOpts{
					Mode:   e2b.GitResetMode("hard"),
					Target: "HEAD",
				})

				_ = createResult
				_ = checkoutResult
				_ = deleteResult
				_ = addResult
				_ = commitResult
				_ = restoreResult
				_ = resetResult
				_ = createErr
				_ = checkoutErr
				_ = deleteErr
				_ = addErr
				_ = commitErr
				_ = restoreErr
				_ = resetErr
			},
		},
		{
			name: "push-pull-and-errors",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				pushResult, pushErr := sandbox.Git.Push(ctx, "/tmp/repo", &e2b.GitPushOpts{
					Remote: "origin",
					Branch: "main",
				})
				pullResult, pullErr := sandbox.Git.Pull(ctx, "/tmp/repo", &e2b.GitPullOpts{
					Remote: "origin",
					Branch: "main",
				})

				var authErr *e2b.GitAuthError
				var upstreamErr *e2b.GitUpstreamError
				var exitErr *e2b.CommandExitError
				_ = errors.As(pushErr, &authErr)
				_ = errors.As(pullErr, &upstreamErr)
				_ = errors.As(pullErr, &exitErr)

				_ = pushResult
				_ = pullResult
			},
		},
		{
			name: "error-matching",
			fn: func() {
				authFailure := error(&e2b.GitAuthError{
					AuthenticationError: e2b.AuthenticationError{Message: "credentials failed"},
				})
				upstreamFailure := error(&e2b.GitUpstreamError{
					SandboxError: e2b.SandboxError{Message: "no upstream"},
				})
				exitFailure := error(&e2b.CommandExitError{
					CommandResult: e2b.CommandResult{
						ExitCode: 1,
						Error:    "git failed",
					},
					Message: "git failed",
				})

				var authErr *e2b.GitAuthError
				var upstreamErr *e2b.GitUpstreamError
				var exitErr *e2b.CommandExitError

				_ = errors.As(authFailure, &authErr)
				_ = errors.As(upstreamFailure, &upstreamErr)
				_ = errors.As(exitFailure, &exitErr)
			},
		},
	}

	if got := len(snippets); got != 9 {
		t.Fatalf("expected 9 git doc snippets, got %d", got)
	}
}
