package git

import "testing"

func TestParseGitBranches(t *testing.T) {
	branches := parseGitBranches("main\t*\nfeature/test\t\n")

	if branches.CurrentBranch != "main" {
		t.Fatalf("expected current branch main, got %q", branches.CurrentBranch)
	}
	if len(branches.Branches) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(branches.Branches))
	}
	if branches.Branches[0] != "main" || branches.Branches[1] != "feature/test" {
		t.Fatalf("unexpected branches: %#v", branches.Branches)
	}
}

func TestParseGitStatusBranchAndRename(t *testing.T) {
	status := parseGitStatus("## main...origin/main [ahead 2, behind 1]\nR  old.txt -> new.txt\n?? extra.txt\n")

	if status.CurrentBranch != "main" {
		t.Fatalf("expected current branch main, got %q", status.CurrentBranch)
	}
	if status.Upstream != "origin/main" {
		t.Fatalf("expected upstream origin/main, got %q", status.Upstream)
	}
	if status.Ahead != 2 || status.Behind != 1 {
		t.Fatalf("expected ahead/behind 2/1, got %d/%d", status.Ahead, status.Behind)
	}
	if status.TotalCount != 2 || status.StagedCount != 1 || status.UntrackedCount != 1 {
		t.Fatalf("unexpected counts: %+v", status)
	}
	if status.FileStatus[0].Status != "renamed" || status.FileStatus[0].RenamedFrom != "old.txt" || status.FileStatus[0].Name != "new.txt" {
		t.Fatalf("unexpected rename parsing: %+v", status.FileStatus[0])
	}
}

func TestWithCredentialsRequiresCompleteHTTPSCredentials(t *testing.T) {
	if got := withCredentials("https://example.com/repo.git", "alice", "secret"); got != "https://alice:secret@example.com/repo.git" {
		t.Fatalf("expected credentials in https url, got %q", got)
	}

	if got := withCredentials("https://example.com/repo.git", "alice", ""); got != "https://example.com/repo.git" {
		t.Fatalf("expected username-only credentials to be ignored, got %q", got)
	}

	if got := withCredentials("ssh://git@example.com/repo.git", "alice", "secret"); got != "ssh://git@example.com/repo.git" {
		t.Fatalf("expected non-http(s) credentials to be ignored, got %q", got)
	}
}

func TestGetRepoPathForScope(t *testing.T) {
	repoPath, err := getRepoPathForScope(GitConfigGlobal, "/tmp/repo")
	if err != nil {
		t.Fatalf("expected non-local scope to ignore repo path requirement, got %v", err)
	}
	if repoPath != "" {
		t.Fatalf("expected non-local scope to omit repo path, got %q", repoPath)
	}

	repoPath, err = getRepoPathForScope(GitConfigLocal, "/tmp/repo")
	if err != nil {
		t.Fatalf("expected local scope with repo path to succeed, got %v", err)
	}
	if repoPath != "/tmp/repo" {
		t.Fatalf("expected local repo path to be preserved, got %q", repoPath)
	}

	_, err = getRepoPathForScope(GitConfigLocal, "")
	if err == nil {
		t.Fatal("expected local scope without repo path to fail")
	}
	if err.Error() != "A repository path is required when using scope \"local\"." {
		t.Fatalf("unexpected error: %v", err)
	}
}
