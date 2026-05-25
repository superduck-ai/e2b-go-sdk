package git

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

func ShellEscape(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func WithCredentials(repoUrl, username, password string) string {
	u, err := url.Parse(repoUrl)
	if err != nil {
		return repoUrl
	}
	if username != "" || password != "" {
		u.User = url.UserPassword(username, password)
	}
	return u.String()
}

func StripCredentials(repoUrl string) string {
	u, err := url.Parse(repoUrl)
	if err != nil {
		return repoUrl
	}
	u.User = nil
	return u.String()
}

func DeriveRepoDirFromUrl(repoUrl string) string {
	u, err := url.Parse(repoUrl)
	if err != nil {
		return ""
	}
	base := path.Base(u.Path)
	return strings.TrimSuffix(base, ".git")
}

func BuildGitCommand(args []string, repoPath string) string {
	if repoPath != "" {
		return fmt.Sprintf("git -C %s %s", ShellEscape(repoPath), strings.Join(args, " "))
	}
	return "git " + strings.Join(args, " ")
}

type GitFileStatus struct {
	Path        string
	IndexStatus string
	WorkStatus  string
}

type GitStatus struct {
	Branch string
	Ahead  int
	Behind int
	Files  []GitFileStatus
}

type GitBranches struct {
	Current  string
	Branches []string
}

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
)

func ParseGitStatus(output string) *GitStatus {
	status := &GitStatus{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			branch := strings.TrimPrefix(line, "## ")
			if idx := strings.Index(branch, "..."); idx != -1 {
				status.Branch = branch[:idx]
			} else {
				status.Branch = branch
			}
		} else if len(line) >= 3 {
			status.Files = append(status.Files, GitFileStatus{
				IndexStatus: string(line[0]),
				WorkStatus:  string(line[1]),
				Path:        line[3:],
			})
		}
	}
	return status
}

func ParseGitBranches(output string) *GitBranches {
	branches := &GitBranches{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "* ") {
			branches.Current = strings.TrimPrefix(line, "* ")
			branches.Branches = append(branches.Branches, branches.Current)
		} else {
			branches.Branches = append(branches.Branches, line)
		}
	}
	return branches
}

func IsAuthFailure(errMsg string) bool {
	return strings.Contains(errMsg, "Authentication failed") ||
		strings.Contains(errMsg, "could not read Username") ||
		strings.Contains(errMsg, "terminal prompts disabled")
}

func IsMissingUpstream(errMsg string) bool {
	return strings.Contains(errMsg, "has no upstream branch")
}

func GetScopeFlag(scope GitConfigScope) string {
	return "--" + string(scope)
}
