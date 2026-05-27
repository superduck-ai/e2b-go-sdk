package git

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

func shellEscape(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func withCredentials(repoUrl, username, password string) string {
	u, err := url.Parse(repoUrl)
	if err != nil {
		return repoUrl
	}
	if (u.Scheme == "http" || u.Scheme == "https") && username != "" && password != "" {
		u.User = url.UserPassword(username, password)
	}
	return u.String()
}

func stripCredentials(repoUrl string) string {
	u, err := url.Parse(repoUrl)
	if err != nil {
		return repoUrl
	}
	u.User = nil
	return u.String()
}

func deriveRepoDirFromUrl(repoUrl string) string {
	u, err := url.Parse(repoUrl)
	if err != nil {
		return ""
	}
	base := path.Base(u.Path)
	return strings.TrimSuffix(base, ".git")
}

func buildGitCommand(args []string, repoPath string) string {
	if repoPath != "" {
		return fmt.Sprintf("git -C %s %s", shellEscape(repoPath), strings.Join(args, " "))
	}
	return "git " + strings.Join(args, " ")
}

type GitFileStatus struct {
	Name              string
	Status            string
	IndexStatus       string
	WorkingTreeStatus string
	Staged            bool
	RenamedFrom       string
}

type GitStatus struct {
	CurrentBranch  string
	Upstream       string
	Ahead          int
	Behind         int
	Detached       bool
	FileStatus     []GitFileStatus
	IsClean        bool
	HasChanges     bool
	HasStaged      bool
	HasUntracked   bool
	HasConflicts   bool
	TotalCount     int
	StagedCount    int
	UnstagedCount  int
	UntrackedCount int
	ConflictCount  int
}

type GitBranches struct {
	CurrentBranch string
	Branches      []string
}

type gitStatusLabel string

const (
	gitStatusConflict   gitStatusLabel = "conflict"
	gitStatusRenamed    gitStatusLabel = "renamed"
	gitStatusCopied     gitStatusLabel = "copied"
	gitStatusDeleted    gitStatusLabel = "deleted"
	gitStatusAdded      gitStatusLabel = "added"
	gitStatusModified   gitStatusLabel = "modified"
	gitStatusTypechange gitStatusLabel = "typechange"
	gitStatusUntracked  gitStatusLabel = "untracked"
	gitStatusUnknown    gitStatusLabel = "unknown"
)

type GitConfigScope string

const (
	GitConfigLocal  GitConfigScope = "local"
	GitConfigGlobal GitConfigScope = "global"
	GitConfigSystem GitConfigScope = "system"
)

type GitResetMode string

const (
	GitResetSoft  GitResetMode = "soft"
	GitResetMixed GitResetMode = "mixed"
	GitResetHard  GitResetMode = "hard"
	GitResetMerge GitResetMode = "merge"
	GitResetKeep  GitResetMode = "keep"
)

func parseGitStatus(output string) *GitStatus {
	status := &GitStatus{}
	lines := strings.Split(output, "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed = append(trimmed, line)
	}

	if len(trimmed) == 0 {
		status.IsClean = true
		return status
	}

	if strings.HasPrefix(trimmed[0], "## ") {
		parseBranchLine(strings.TrimPrefix(trimmed[0], "## "), status)
	}

	for _, line := range trimmed[1:] {
		if strings.HasPrefix(line, "?? ") {
			status.FileStatus = append(status.FileStatus, GitFileStatus{
				Name:              line[3:],
				Status:            string(gitStatusUntracked),
				IndexStatus:       "?",
				WorkingTreeStatus: "?",
				Staged:            false,
			})
			continue
		}
		if len(line) < 3 {
			continue
		}
		indexStatus := string(line[0])
		workingStatus := string(line[1])
		name := line[3:]
		renamedFrom := ""
		if strings.Contains(name, " -> ") {
			parts := strings.SplitN(name, " -> ", 2)
			renamedFrom = parts[0]
			name = parts[1]
		}

		entry := GitFileStatus{
			Name:              name,
			Status:            deriveStatus(indexStatus, workingStatus),
			IndexStatus:       indexStatus,
			WorkingTreeStatus: workingStatus,
			Staged:            indexStatus != " " && indexStatus != "?",
			RenamedFrom:       renamedFrom,
		}
		status.FileStatus = append(status.FileStatus, entry)
	}

	status.TotalCount = len(status.FileStatus)
	for _, item := range status.FileStatus {
		if item.Staged {
			status.StagedCount++
		}
		if item.Status == string(gitStatusUntracked) {
			status.UntrackedCount++
		}
		if item.Status == string(gitStatusConflict) {
			status.ConflictCount++
		}
	}
	status.UnstagedCount = status.TotalCount - status.StagedCount
	status.IsClean = status.TotalCount == 0
	status.HasChanges = status.TotalCount > 0
	status.HasStaged = status.StagedCount > 0
	status.HasUntracked = status.UntrackedCount > 0
	status.HasConflicts = status.ConflictCount > 0
	return status
}

func parseGitBranches(output string) *GitBranches {
	branches := &GitBranches{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		name := parts[0]
		branches.Branches = append(branches.Branches, name)
		if len(parts) > 1 && parts[1] == "*" {
			branches.CurrentBranch = name
		}
	}
	return branches
}

func isAuthFailure(errMsg string) bool {
	message := strings.ToLower(errMsg)
	authSnippets := []string{
		"authentication failed",
		"terminal prompts disabled",
		"could not read username",
		"invalid username or password",
		"access denied",
		"permission denied (publickey)",
		"not authorized",
	}
	for _, snippet := range authSnippets {
		if strings.Contains(message, snippet) {
			return true
		}
	}
	return false
}

func isMissingUpstream(errMsg string) bool {
	message := strings.ToLower(errMsg)
	upstreamSnippets := []string{
		"has no upstream branch",
		"no upstream branch",
		"no upstream configured",
		"no tracking information for the current branch",
		"no tracking information",
		"set the remote as upstream",
		"set the upstream branch",
		"please specify which branch you want to merge with",
	}
	for _, snippet := range upstreamSnippets {
		if strings.Contains(message, snippet) {
			return true
		}
	}
	return false
}

func getScopeFlag(scope GitConfigScope) string {
	return "--" + string(scope)
}

func validateConfigScope(scope GitConfigScope) error {
	switch scope {
	case GitConfigGlobal, GitConfigLocal, GitConfigSystem:
		return nil
	default:
		return fmt.Errorf("Git config scope must be one of: global, local, system.")
	}
}

func buildUpstreamErrorMessage(action string) string {
	if action == "push" {
		return "Git push failed because no upstream branch is configured. Set upstream once with { setUpstream: true } (and optional remote/branch), or pass remote and branch explicitly."
	}
	return "Git pull failed because no upstream branch is configured. Pass remote and branch explicitly, or set upstream once (push with { setUpstream: true } or run: git branch --set-upstream-to=origin/<branch> <branch>)."
}

func getRepoPathForScope(scope GitConfigScope, repoPath string) (string, error) {
	if scope != GitConfigLocal {
		return "", nil
	}
	if repoPath == "" {
		return "", fmt.Errorf("A repository path is required when using scope \"local\".")
	}
	return repoPath, nil
}

func parseAheadBehind(segment string) (ahead, behind int) {
	if strings.Contains(segment, "ahead ") {
		fmt.Sscanf(segment[strings.Index(segment, "ahead ")+6:], "%d", &ahead)
	}
	if strings.Contains(segment, "behind ") {
		fmt.Sscanf(segment[strings.Index(segment, "behind ")+7:], "%d", &behind)
	}
	return
}

func normalizeBranchName(name string) string {
	if strings.HasPrefix(name, "HEAD (detached at ") {
		return strings.TrimSuffix(strings.TrimPrefix(name, "HEAD (detached at "), ")")
	}
	name = strings.ReplaceAll(name, "HEAD (no branch)", "HEAD")
	name = strings.ReplaceAll(name, "No commits yet on ", "")
	name = strings.ReplaceAll(name, "Initial commit on ", "")
	return name
}

func parseBranchLine(line string, status *GitStatus) {
	branchPart := line
	aheadPart := ""
	if idx := strings.Index(line, " ["); idx != -1 && strings.HasSuffix(line, "]") {
		branchPart = line[:idx]
		aheadPart = line[idx+2 : len(line)-1]
	}

	normalized := normalizeBranchName(branchPart)
	if strings.HasPrefix(branchPart, "HEAD (detached at ") || strings.Contains(branchPart, "detached") || strings.HasPrefix(normalized, "HEAD") {
		status.Detached = true
	} else if strings.Contains(normalized, "...") {
		parts := strings.SplitN(normalized, "...", 2)
		status.CurrentBranch = parts[0]
		if len(parts) > 1 {
			status.Upstream = parts[1]
		}
	} else {
		status.CurrentBranch = normalized
	}

	status.Ahead, status.Behind = parseAheadBehind(aheadPart)
}

func deriveStatus(indexStatus, workingStatus string) string {
	statuses := map[string]bool{
		indexStatus:   true,
		workingStatus: true,
	}
	switch {
	case statuses["U"]:
		return string(gitStatusConflict)
	case statuses["R"]:
		return string(gitStatusRenamed)
	case statuses["C"]:
		return string(gitStatusCopied)
	case statuses["D"]:
		return string(gitStatusDeleted)
	case statuses["A"]:
		return string(gitStatusAdded)
	case statuses["M"]:
		return string(gitStatusModified)
	case statuses["T"]:
		return string(gitStatusTypechange)
	case statuses["?"]:
		return string(gitStatusUntracked)
	default:
		return string(gitStatusUnknown)
	}
}
