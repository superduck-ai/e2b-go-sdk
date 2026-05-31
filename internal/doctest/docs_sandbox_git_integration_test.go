package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxGitIntegrationDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/git-integration.mdx"); err != nil {
		t.Fatalf("sandbox git integration doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/git-integration.mdx aligned with the exported
// Go SDK git surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsSandboxGitIntegrationExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "inline-credentials",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				repoPath := "/home/user/repo"
				pushResult, pushErr := sandbox.Git.Push(ctx, repoPath, &e2b.GitPushOpts{
					Username: os.Getenv("GIT_USERNAME"),
					Password: os.Getenv("GIT_TOKEN"),
				})
				pullResult, pullErr := sandbox.Git.Pull(ctx, repoPath, &e2b.GitPullOpts{
					Username: os.Getenv("GIT_USERNAME"),
					Password: os.Getenv("GIT_TOKEN"),
				})

				_ = pushResult
				_ = pullResult
				_ = pushErr
				_ = pullErr
			},
		},
		{
			name: "dangerously-authenticate",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				authResult, authErr := sandbox.Git.DangerouslyAuthenticate(ctx, &e2b.GitDangerouslyAuthenticateOpts{
					Username: os.Getenv("GIT_USERNAME"),
					Password: os.Getenv("GIT_TOKEN"),
				})
				customAuthResult, customAuthErr := sandbox.Git.DangerouslyAuthenticate(ctx, &e2b.GitDangerouslyAuthenticateOpts{
					Username: os.Getenv("GIT_USERNAME"),
					Password: os.Getenv("GIT_TOKEN"),
					Host:     "git.example.com",
					Protocol: "https",
				})
				cloneResult, cloneErr := sandbox.Git.Clone(ctx, "https://git.example.com/org/repo.git", &e2b.GitCloneOpts{
					Path: "/home/user/repo",
				})
				pushResult, pushErr := sandbox.Git.Push(ctx, "/home/user/repo", nil)

				_ = authResult
				_ = customAuthResult
				_ = cloneResult
				_ = pushResult
				_ = authErr
				_ = customAuthErr
				_ = cloneErr
				_ = pushErr
			},
		},
		{
			name: "clone-repository",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				repoURL := "https://git.example.com/org/repo.git"
				repoPath := "/home/user/repo"
				depth := 1

				defaultClone, defaultCloneErr := sandbox.Git.Clone(ctx, repoURL, &e2b.GitCloneOpts{
					Path: repoPath,
				})
				branchClone, branchCloneErr := sandbox.Git.Clone(ctx, repoURL, &e2b.GitCloneOpts{
					Path:   repoPath,
					Branch: "main",
				})
				shallowClone, shallowCloneErr := sandbox.Git.Clone(ctx, repoURL, &e2b.GitCloneOpts{
					Path:  repoPath,
					Depth: &depth,
				})
				storedCredsClone, storedCredsCloneErr := sandbox.Git.Clone(ctx, repoURL, &e2b.GitCloneOpts{
					Path:                        repoPath,
					Username:                    "username",
					Password:                    "token",
					DangerouslyStoreCredentials: true,
				})

				_ = defaultClone
				_ = branchClone
				_ = shallowClone
				_ = storedCredsClone
				_ = defaultCloneErr
				_ = branchCloneErr
				_ = shallowCloneErr
				_ = storedCredsCloneErr
			},
		},
		{
			name: "configure-user",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				repoPath := "/home/user/repo"
				globalResult, globalErr := sandbox.Git.ConfigureUser(ctx, "E2B Bot", "bot@example.com", nil)
				localResult, localErr := sandbox.Git.ConfigureUser(ctx, "E2B Bot", "bot@example.com", &e2b.GitConfigOpts{
					Scope: e2b.GitConfigScope("local"),
					Path:  repoPath,
				})

				_ = globalResult
				_ = localResult
				_ = globalErr
				_ = localErr
			},
		},
		{
			name: "status-and-branches",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				repoPath := "/home/user/repo"
				status, statusErr := sandbox.Git.Status(ctx, repoPath, nil)
				branches, branchesErr := sandbox.Git.Branches(ctx, repoPath, nil)

				_ = status.CurrentBranch
				_ = status.Ahead
				_ = status.Behind
				_ = status.FileStatus
				_ = branches.CurrentBranch
				_ = branches.Branches
				_ = statusErr
				_ = branchesErr
			},
		},
		{
			name: "branch-management",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				repoPath := "/home/user/repo"
				createResult, createErr := sandbox.Git.CreateBranch(ctx, repoPath, "feature/new-docs", nil)
				checkoutResult, checkoutErr := sandbox.Git.CheckoutBranch(ctx, repoPath, "main", nil)
				deleteResult, deleteErr := sandbox.Git.DeleteBranch(ctx, repoPath, "feature/old-docs", nil)
				forceDeleteResult, forceDeleteErr := sandbox.Git.DeleteBranch(ctx, repoPath, "feature/stale-docs", &e2b.GitDeleteBranchOpts{
					Force: true,
				})

				_ = createResult
				_ = checkoutResult
				_ = deleteResult
				_ = forceDeleteResult
				_ = createErr
				_ = checkoutErr
				_ = deleteErr
				_ = forceDeleteErr
			},
		},
	}

	if got := len(snippets); got != 6 {
		t.Fatalf("expected 6 sandbox git integration doc snippets, got %d", got)
	}
}
