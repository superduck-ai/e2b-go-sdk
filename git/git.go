package git

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/e2b-dev/e2b-go-sdk/commands"
)

type GitRequestOpts struct {
	RequestTimeoutMs *int
	User             string
	Envs             map[string]string
}

type GitCloneOpts struct {
	GitRequestOpts
	Path     string
	Branch   string
	Depth    int
	Username string
	Password string
}

type GitInitOpts struct {
	GitRequestOpts
	Bare          bool
	InitialBranch string
}

type GitCommitOpts struct {
	GitRequestOpts
	Author string
}

type GitAddOpts struct {
	GitRequestOpts
	All   bool
	Files []string
}

type GitResetOpts struct {
	GitRequestOpts
	Mode   GitResetMode
	Target string
	Paths  []string
}

type GitRestoreOpts struct {
	GitRequestOpts
	Files  []string
	Staged bool
	Source string
}

type GitPushOpts struct {
	GitRequestOpts
	Remote   string
	Branch   string
	Force    bool
	SetUpstream bool
	Username string
	Password string
}

type GitPullOpts struct {
	GitRequestOpts
	Remote   string
	Branch   string
	Rebase   bool
	Username string
	Password string
}

type GitDeleteBranchOpts struct {
	GitRequestOpts
	Force bool
}

type GitConfigOpts struct {
	GitRequestOpts
	Scope GitConfigScope
	Path  string
}

type GitDangerouslyAuthenticateOpts struct {
	GitRequestOpts
	Username string
	Password string
}

type GitAuthError struct {
	Err error
}

func (e *GitAuthError) Error() string { return "git authentication failed: " + e.Err.Error() }
func (e *GitAuthError) Unwrap() error { return e.Err }

type GitUpstreamError struct {
	Err error
}

func (e *GitUpstreamError) Error() string { return "git upstream not set: " + e.Err.Error() }
func (e *GitUpstreamError) Unwrap() error { return e.Err }

var defaultEnvs = map[string]string{
	"GIT_TERMINAL_PROMPT": "0",
}

type Git struct {
	commands *commands.Commands
}

func NewGit(cmds *commands.Commands) *Git {
	return &Git{commands: cmds}
}

func (g *Git) runGit(ctx context.Context, args []string, repoPath string, opts *GitRequestOpts) (*commands.CommandResult, error) {
	cmd := BuildGitCommand(args, repoPath)
	startOpts := g.buildStartOpts(opts)
	result, err := g.commands.Run(ctx, cmd, startOpts)
	if err != nil {
		return nil, g.wrapError(err)
	}
	return result, nil
}

func (g *Git) runShell(ctx context.Context, cmd string, opts *GitRequestOpts) (*commands.CommandResult, error) {
	startOpts := g.buildStartOpts(opts)
	result, err := g.commands.Run(ctx, cmd, startOpts)
	if err != nil {
		return nil, g.wrapError(err)
	}
	return result, nil
}

func (g *Git) buildStartOpts(opts *GitRequestOpts) *commands.CommandStartOpts {
	envs := make(map[string]string)
	for k, v := range defaultEnvs {
		envs[k] = v
	}
	if opts != nil {
		for k, v := range opts.Envs {
			envs[k] = v
		}
	}
	startOpts := &commands.CommandStartOpts{
		Envs: envs,
	}
	if opts != nil {
		startOpts.User = opts.User
		if opts.RequestTimeoutMs != nil {
			startOpts.TimeoutMs = opts.RequestTimeoutMs
		}
	}
	return startOpts
}

func (g *Git) wrapError(err error) error {
	var exitErr *commands.CommandExitError
	if errors.As(err, &exitErr) {
		errMsg := exitErr.Stderr
		if IsAuthFailure(errMsg) {
			return &GitAuthError{Err: err}
		}
		if IsMissingUpstream(errMsg) {
			return &GitUpstreamError{Err: err}
		}
	}
	return err
}

func (g *Git) Clone(ctx context.Context, repoUrl string, opts *GitCloneOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitCloneOpts{}
	}

	cloneUrl := repoUrl
	if opts.Username != "" || opts.Password != "" {
		cloneUrl = WithCredentials(repoUrl, opts.Username, opts.Password)
	}

	args := []string{"clone", ShellEscape(cloneUrl)}
	if opts.Branch != "" {
		args = append(args, "--branch", ShellEscape(opts.Branch))
	}
	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}
	if opts.Path != "" {
		args = append(args, ShellEscape(opts.Path))
	}

	result, err := g.runGit(ctx, args, "", &opts.GitRequestOpts)
	if err != nil {
		return nil, err
	}

	// Strip inline credentials after clone by resetting the remote URL
	if opts.Username != "" || opts.Password != "" {
		repoPath := opts.Path
		if repoPath == "" {
			repoPath = DeriveRepoDirFromUrl(repoUrl)
		}
		strippedUrl := StripCredentials(cloneUrl)
		setUrlArgs := []string{"remote", "set-url", "origin", ShellEscape(strippedUrl)}
		_, _ = g.runGit(ctx, setUrlArgs, repoPath, &opts.GitRequestOpts)
	}

	return result, nil
}

func (g *Git) Init(ctx context.Context, path string, opts *GitInitOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitInitOpts{}
	}

	args := []string{"init"}
	if opts.Bare {
		args = append(args, "--bare")
	}
	if opts.InitialBranch != "" {
		args = append(args, "--initial-branch", ShellEscape(opts.InitialBranch))
	}

	return g.runGit(ctx, args, path, &opts.GitRequestOpts)
}

func (g *Git) RemoteAdd(ctx context.Context, path, name, url string, opts *GitRequestOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitRequestOpts{}
	}

	args := []string{"remote", "add", ShellEscape(name), ShellEscape(url)}
	result, err := g.runGit(ctx, args, path, opts)
	if err != nil {
		// If remote already exists, overwrite with set-url
		setArgs := []string{"remote", "set-url", ShellEscape(name), ShellEscape(url)}
		return g.runGit(ctx, setArgs, path, opts)
	}
	return result, nil
}

func (g *Git) RemoteGet(ctx context.Context, path, name string, opts *GitRequestOpts) (string, error) {
	if opts == nil {
		opts = &GitRequestOpts{}
	}

	cmd := BuildGitCommand([]string{"remote", "get-url", ShellEscape(name)}, path) + " || true"
	result, err := g.runShell(ctx, cmd, opts)
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (g *Git) Status(ctx context.Context, path string, opts *GitRequestOpts) (*GitStatus, error) {
	if opts == nil {
		opts = &GitRequestOpts{}
	}

	args := []string{"status", "--porcelain=1", "-b"}
	result, err := g.runGit(ctx, args, path, opts)
	if err != nil {
		return nil, err
	}
	return ParseGitStatus(result.Stdout), nil
}

func (g *Git) Branches(ctx context.Context, path string, opts *GitRequestOpts) (*GitBranches, error) {
	if opts == nil {
		opts = &GitRequestOpts{}
	}

	args := []string{"branch", "--format=%(refname:short)\t%(HEAD)"}
	result, err := g.runGit(ctx, args, path, opts)
	if err != nil {
		return nil, err
	}

	// Parse tab-separated format: branchname\t*  or branchname\t
	branches := &GitBranches{}
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		name := parts[0]
		branches.Branches = append(branches.Branches, name)
		if len(parts) > 1 && strings.TrimSpace(parts[1]) == "*" {
			branches.Current = name
		}
	}
	return branches, nil
}

func (g *Git) CreateBranch(ctx context.Context, path, branch string, opts *GitRequestOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitRequestOpts{}
	}
	args := []string{"checkout", "-b", ShellEscape(branch)}
	return g.runGit(ctx, args, path, opts)
}

func (g *Git) CheckoutBranch(ctx context.Context, path, branch string, opts *GitRequestOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitRequestOpts{}
	}
	args := []string{"checkout", ShellEscape(branch)}
	return g.runGit(ctx, args, path, opts)
}

func (g *Git) DeleteBranch(ctx context.Context, path, branch string, opts *GitDeleteBranchOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitDeleteBranchOpts{}
	}
	flag := "-d"
	if opts.Force {
		flag = "-D"
	}
	args := []string{"branch", flag, ShellEscape(branch)}
	return g.runGit(ctx, args, path, &opts.GitRequestOpts)
}

func (g *Git) Add(ctx context.Context, path string, opts *GitAddOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitAddOpts{All: true}
	}

	var args []string
	if opts.All || len(opts.Files) == 0 {
		args = []string{"add", "-A"}
	} else {
		args = []string{"add", "--"}
		for _, f := range opts.Files {
			args = append(args, ShellEscape(f))
		}
	}
	return g.runGit(ctx, args, path, &opts.GitRequestOpts)
}

func (g *Git) Commit(ctx context.Context, path, message string, opts *GitCommitOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitCommitOpts{}
	}

	args := []string{"commit", "-m", ShellEscape(message)}
	if opts.Author != "" {
		args = append(args, "--author", ShellEscape(opts.Author))
	}
	return g.runGit(ctx, args, path, &opts.GitRequestOpts)
}

func (g *Git) Reset(ctx context.Context, path string, opts *GitResetOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitResetOpts{}
	}

	args := []string{"reset"}
	if opts.Mode != "" {
		args = append(args, "--"+string(opts.Mode))
	}
	if opts.Target != "" {
		args = append(args, ShellEscape(opts.Target))
	}
	if len(opts.Paths) > 0 {
		args = append(args, "--")
		for _, p := range opts.Paths {
			args = append(args, ShellEscape(p))
		}
	}
	return g.runGit(ctx, args, path, &opts.GitRequestOpts)
}

func (g *Git) Restore(ctx context.Context, path string, opts *GitRestoreOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitRestoreOpts{}
	}

	args := []string{"restore", "--worktree"}
	if opts.Staged {
		args = append(args, "--staged")
	}
	if opts.Source != "" {
		args = append(args, "--source", ShellEscape(opts.Source))
	}
	if len(opts.Files) > 0 {
		args = append(args, "--")
		for _, f := range opts.Files {
			args = append(args, ShellEscape(f))
		}
	}
	return g.runGit(ctx, args, path, &opts.GitRequestOpts)
}

func (g *Git) Push(ctx context.Context, path string, opts *GitPushOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitPushOpts{}
	}

	operation := func(reqOpts *GitRequestOpts) (*commands.CommandResult, error) {
		args := []string{"push"}
		if opts.Force {
			args = append(args, "--force")
		}
		if opts.SetUpstream {
			args = append(args, "--set-upstream")
		}
		remote := opts.Remote
		if remote == "" {
			remote = "origin"
		}
		args = append(args, ShellEscape(remote))
		if opts.Branch != "" {
			args = append(args, ShellEscape(opts.Branch))
		}
		return g.runGit(ctx, args, path, reqOpts)
	}

	if opts.Username != "" || opts.Password != "" {
		return g.withRemoteCredentials(ctx, path, opts.Remote, opts.Username, opts.Password, &opts.GitRequestOpts, operation)
	}
	return operation(&opts.GitRequestOpts)
}

func (g *Git) Pull(ctx context.Context, path string, opts *GitPullOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitPullOpts{}
	}

	operation := func(reqOpts *GitRequestOpts) (*commands.CommandResult, error) {
		args := []string{"pull"}
		if opts.Rebase {
			args = append(args, "--rebase")
		}
		if opts.Remote != "" {
			args = append(args, ShellEscape(opts.Remote))
		}
		if opts.Branch != "" {
			args = append(args, ShellEscape(opts.Branch))
		}
		return g.runGit(ctx, args, path, reqOpts)
	}

	if opts.Username != "" || opts.Password != "" {
		remote := opts.Remote
		if remote == "" {
			remote = "origin"
		}
		return g.withRemoteCredentials(ctx, path, remote, opts.Username, opts.Password, &opts.GitRequestOpts, operation)
	}
	return operation(&opts.GitRequestOpts)
}

func (g *Git) SetConfig(ctx context.Context, key, value string, opts *GitConfigOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitConfigOpts{Scope: GitConfigLocal}
	}

	scope := opts.Scope
	if scope == "" {
		scope = GitConfigLocal
	}

	args := []string{"config", GetScopeFlag(scope), ShellEscape(key), ShellEscape(value)}
	path := opts.Path
	return g.runGit(ctx, args, path, &opts.GitRequestOpts)
}

func (g *Git) GetConfig(ctx context.Context, key string, opts *GitConfigOpts) (string, error) {
	if opts == nil {
		opts = &GitConfigOpts{Scope: GitConfigLocal}
	}

	scope := opts.Scope
	if scope == "" {
		scope = GitConfigLocal
	}

	cmd := BuildGitCommand([]string{"config", GetScopeFlag(scope), "--get", ShellEscape(key)}, opts.Path) + " || true"
	result, err := g.runShell(ctx, cmd, &opts.GitRequestOpts)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (g *Git) DangerouslyAuthenticate(ctx context.Context, opts *GitDangerouslyAuthenticateOpts) (*commands.CommandResult, error) {
	if opts == nil {
		return nil, fmt.Errorf("opts with username and password are required")
	}

	// Set credential helper to store
	configArgs := []string{"config", "--global", "credential.helper", "store"}
	_, err := g.runGit(ctx, configArgs, "", &opts.GitRequestOpts)
	if err != nil {
		return nil, err
	}

	// Pipe credential approve
	credentialInput := fmt.Sprintf("protocol=https\nhost=github.com\nusername=%s\npassword=%s\n\n", opts.Username, opts.Password)
	cmd := fmt.Sprintf("echo %s | git credential approve", ShellEscape(credentialInput))
	return g.runShell(ctx, cmd, &opts.GitRequestOpts)
}

func (g *Git) ConfigureUser(ctx context.Context, name, email string, opts *GitRequestOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitRequestOpts{}
	}

	nameArgs := []string{"config", "--global", "user.name", ShellEscape(name)}
	_, err := g.runGit(ctx, nameArgs, "", opts)
	if err != nil {
		return nil, err
	}

	emailArgs := []string{"config", "--global", "user.email", ShellEscape(email)}
	return g.runGit(ctx, emailArgs, "", opts)
}

// withRemoteCredentials temporarily sets credentials on the remote URL, runs the operation, then restores the original URL.
func (g *Git) withRemoteCredentials(ctx context.Context, path, remote, username, password string, opts *GitRequestOpts, operation func(*GitRequestOpts) (*commands.CommandResult, error)) (*commands.CommandResult, error) {
	if remote == "" {
		remote = "origin"
	}

	// Get current remote URL
	originalUrl, err := g.getRemoteUrl(ctx, path, remote, opts)
	if err != nil {
		return nil, err
	}

	// Set URL with credentials
	credUrl := WithCredentials(originalUrl, username, password)
	setArgs := []string{"remote", "set-url", ShellEscape(remote), ShellEscape(credUrl)}
	_, err = g.runGit(ctx, setArgs, path, opts)
	if err != nil {
		return nil, err
	}

	// Run the operation
	result, opErr := operation(opts)

	// Restore original URL
	restoreArgs := []string{"remote", "set-url", ShellEscape(remote), ShellEscape(originalUrl)}
	_, _ = g.runGit(ctx, restoreArgs, path, opts)

	return result, opErr
}

func (g *Git) getRemoteUrl(ctx context.Context, path, remote string, opts *GitRequestOpts) (string, error) {
	result, err := g.RemoteGet(ctx, path, remote, opts)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (g *Git) hasUpstream(ctx context.Context, path string, opts *GitRequestOpts) bool {
	args := []string{"rev-parse", "--abbrev-ref", "@{u}"}
	_, err := g.runGit(ctx, args, path, opts)
	return err == nil
}
