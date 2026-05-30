package git

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/superduck-ai/e2b-go-sdk/commands"
	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

type GitRequestOpts struct {
	RequestTimeoutMs *int
	Signal           context.Context
	TimeoutMs        *int
	User             string
	Cwd              string
	Envs             map[string]string
}

type GitCloneOpts struct {
	GitRequestOpts
	Path                        string
	Branch                      string
	Depth                       *int
	Username                    string
	Password                    string
	DangerouslyStoreCredentials bool
}

type GitInitOpts struct {
	GitRequestOpts
	Bare          bool
	InitialBranch string
}

type GitCommitOpts struct {
	GitRequestOpts
	AuthorName  string
	AuthorEmail string
	AllowEmpty  bool
}

type GitRemoteAddOpts struct {
	GitRequestOpts
	Fetch     bool
	Overwrite bool
}

type GitAddOpts struct {
	GitRequestOpts
	All   *bool
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
	Paths []string
	// Deprecated: use Paths to match the JS/Python SDK surface.
	Files    []string
	Staged   *bool
	Worktree *bool
	Source   string
}

type GitPushOpts struct {
	GitRequestOpts
	Remote      string
	Branch      string
	SetUpstream *bool
	Username    string
	Password    string
}

type GitPullOpts struct {
	GitRequestOpts
	Remote   string
	Branch   string
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
	Host     string
	Protocol string
}

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
	cmd := buildGitCommand(args, repoPath)
	startOpts := g.buildStartOpts(opts)
	execution, err := g.commands.Run(ctx, cmd, startOpts)
	if err != nil {
		return nil, g.wrapError(err)
	}
	result, ok := execution.(*commands.CommandResult)
	if !ok {
		return nil, g.wrapError(fmt.Errorf("expected foreground command result, got %T", execution))
	}
	return result, nil
}

func (g *Git) runShell(ctx context.Context, cmd string, opts *GitRequestOpts) (*commands.CommandResult, error) {
	startOpts := g.buildStartOpts(opts)
	execution, err := g.commands.Run(ctx, cmd, startOpts)
	if err != nil {
		return nil, g.wrapError(err)
	}
	result, ok := execution.(*commands.CommandResult)
	if !ok {
		return nil, g.wrapError(fmt.Errorf("expected foreground command result, got %T", execution))
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
		startOpts.Cwd = opts.Cwd
		startOpts.TimeoutMs = opts.TimeoutMs
		startOpts.Signal = opts.Signal
		if opts.RequestTimeoutMs != nil {
			startOpts.RequestTimeoutMs = opts.RequestTimeoutMs
		}
	}
	return startOpts
}

func (g *Git) wrapError(err error) error {
	var exitErr *commands.CommandExitError
	if errors.As(err, &exitErr) {
		errMsg := exitErr.Stderr + "\n" + exitErr.Stdout
		if isAuthFailure(errMsg) {
			return &shared.GitAuthError{
				AuthenticationError: shared.AuthenticationError{
					Message: "git authentication failed: " + err.Error(),
				},
			}
		}
		if isMissingUpstream(errMsg) {
			return &shared.GitUpstreamError{
				SandboxError: shared.SandboxError{
					Message: buildUpstreamErrorMessage("pull"),
				},
			}
		}
	}
	return err
}

func wrapGitUpstreamActionError(err error, action string) error {
	var upstreamErr *shared.GitUpstreamError
	if errors.As(err, &upstreamErr) {
		return &shared.GitUpstreamError{
			SandboxError: shared.SandboxError{
				Message: buildUpstreamErrorMessage(action),
			},
		}
	}
	return err
}

func (g *Git) Clone(ctx context.Context, repoUrl string, opts *GitCloneOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitCloneOpts{}
	}
	if opts.Password != "" && opts.Username == "" {
		return nil, fmt.Errorf("Username is required when using a password or token for git clone.")
	}

	cloneUrl := repoUrl
	if opts.Username != "" && opts.Password != "" {
		cloneUrl = withCredentials(repoUrl, opts.Username, opts.Password)
	}

	args := []string{"clone", shellEscape(cloneUrl)}
	if opts.Branch != "" {
		args = append(args, "--branch", shellEscape(opts.Branch), "--single-branch")
	}
	if opts.Depth != nil && *opts.Depth != 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", *opts.Depth))
	}
	repoPath := opts.Path
	if !opts.DangerouslyStoreCredentials && opts.Username != "" && opts.Password != "" && repoPath == "" {
		repoPath = deriveRepoDirFromUrl(repoUrl)
		if repoPath == "" {
			return nil, fmt.Errorf("A destination path is required when using credentials without storing them.")
		}
	}
	if repoPath != "" {
		args = append(args, shellEscape(repoPath))
	}

	result, err := g.runGit(ctx, args, "", &opts.GitRequestOpts)
	if err != nil {
		return nil, err
	}

	// Strip inline credentials after clone by resetting the remote URL
	if !opts.DangerouslyStoreCredentials && opts.Username != "" && opts.Password != "" {
		strippedUrl := stripCredentials(cloneUrl)
		setUrlArgs := []string{"remote", "set-url", "origin", shellEscape(strippedUrl)}
		_, _ = g.runGit(ctx, setUrlArgs, repoPath, &opts.GitRequestOpts)
	}

	return result, nil
}

func (g *Git) Init(ctx context.Context, path string, opts *GitInitOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitInitOpts{}
	}

	args := []string{"init"}
	if opts.InitialBranch != "" {
		args = append(args, "--initial-branch", shellEscape(opts.InitialBranch))
	}
	if opts.Bare {
		args = append(args, "--bare")
	}
	args = append(args, shellEscape(path))

	return g.runGit(ctx, args, "", &opts.GitRequestOpts)
}

func (g *Git) RemoteAdd(ctx context.Context, path, name, url string, opts *GitRemoteAddOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitRemoteAddOpts{}
	}
	if name == "" || url == "" {
		return nil, fmt.Errorf("Both remote name and URL are required to add a git remote.")
	}

	args := []string{"remote", "add"}
	if opts.Fetch {
		args = append(args, "-f")
	}
	args = append(args, shellEscape(name), shellEscape(url))
	if !opts.Overwrite {
		return g.runGit(ctx, args, path, &opts.GitRequestOpts)
	}

	addCmd := buildGitCommand(args, path)
	setURLCmd := buildGitCommand([]string{"remote", "set-url", shellEscape(name), shellEscape(url)}, path)
	cmd := fmt.Sprintf("%s || %s", addCmd, setURLCmd)
	if opts.Fetch {
		cmd = fmt.Sprintf("(%s) && %s", cmd, buildGitCommand([]string{"fetch", shellEscape(name)}, path))
	}
	return g.runShell(ctx, cmd, &opts.GitRequestOpts)
}

func (g *Git) RemoteGet(ctx context.Context, path, name string, opts *GitRequestOpts) (string, error) {
	if opts == nil {
		opts = &GitRequestOpts{}
	}
	if name == "" {
		return "", fmt.Errorf("Remote name is required.")
	}

	cmd := buildGitCommand([]string{"remote", "get-url", shellEscape(name)}, path) + " || true"
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
	return parseGitStatus(result.Stdout), nil
}

func (g *Git) Branches(ctx context.Context, path string, opts *GitRequestOpts) (*GitBranches, error) {
	if opts == nil {
		opts = &GitRequestOpts{}
	}

	args := []string{"branch", shellEscape("--format=%(refname:short)\t%(HEAD)")}
	result, err := g.runGit(ctx, args, path, opts)
	if err != nil {
		return nil, err
	}
	return parseGitBranches(result.Stdout), nil
}

func (g *Git) CreateBranch(ctx context.Context, path, branch string, opts *GitRequestOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitRequestOpts{}
	}
	args := []string{"checkout", "-b", shellEscape(branch)}
	return g.runGit(ctx, args, path, opts)
}

func (g *Git) CheckoutBranch(ctx context.Context, path, branch string, opts *GitRequestOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitRequestOpts{}
	}
	args := []string{"checkout", shellEscape(branch)}
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
	args := []string{"branch", flag, shellEscape(branch)}
	return g.runGit(ctx, args, path, &opts.GitRequestOpts)
}

func (g *Git) Add(ctx context.Context, path string, opts *GitAddOpts) (*commands.CommandResult, error) {
	if opts == nil {
		all := true
		opts = &GitAddOpts{All: &all}
	}

	var args []string
	if len(opts.Files) == 0 {
		args = []string{"add"}
		if resolveOptionalBool(opts.All, true) {
			args = append(args, "-A")
		} else {
			args = append(args, ".")
		}
	} else {
		args = []string{"add", "--"}
		for _, f := range opts.Files {
			args = append(args, shellEscape(f))
		}
	}
	return g.runGit(ctx, args, path, &opts.GitRequestOpts)
}

func (g *Git) Commit(ctx context.Context, path, message string, opts *GitCommitOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitCommitOpts{}
	}

	args := []string{"commit", "-m", shellEscape(message)}
	if opts.AllowEmpty {
		args = append(args, "--allow-empty")
	}
	var authorArgs []string
	if opts.AuthorName != "" {
		authorArgs = append(authorArgs, "-c", shellEscape("user.name="+opts.AuthorName))
	}
	if opts.AuthorEmail != "" {
		authorArgs = append(authorArgs, "-c", shellEscape("user.email="+opts.AuthorEmail))
	}
	args = append(authorArgs, args...)
	return g.runGit(ctx, args, path, &opts.GitRequestOpts)
}

func (g *Git) Reset(ctx context.Context, path string, opts *GitResetOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitResetOpts{}
	}

	args := []string{"reset"}
	if opts.Mode != "" {
		switch opts.Mode {
		case GitResetSoft, GitResetMixed, GitResetHard, GitResetMerge, GitResetKeep:
		default:
			return nil, fmt.Errorf("Reset mode must be one of soft, mixed, hard, merge, keep.")
		}
		args = append(args, "--"+string(opts.Mode))
	}
	if opts.Target != "" {
		args = append(args, shellEscape(opts.Target))
	}
	if len(opts.Paths) > 0 {
		args = append(args, "--")
		for _, p := range opts.Paths {
			args = append(args, shellEscape(p))
		}
	}
	return g.runGit(ctx, args, path, &opts.GitRequestOpts)
}

func (g *Git) Restore(ctx context.Context, path string, opts *GitRestoreOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitRestoreOpts{}
	}
	paths := opts.Paths
	if len(paths) == 0 {
		paths = opts.Files
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("At least one path is required.")
	}

	resolvedStaged := opts.Staged
	resolvedWorktree := opts.Worktree

	if resolvedStaged == nil && resolvedWorktree == nil {
		defaultWorktree := true
		resolvedWorktree = &defaultWorktree
	} else if resolvedStaged != nil && *resolvedStaged && resolvedWorktree == nil {
		defaultWorktree := false
		resolvedWorktree = &defaultWorktree
	} else if resolvedStaged == nil && resolvedWorktree != nil {
		defaultStaged := false
		resolvedStaged = &defaultStaged
	}

	if !resolveOptionalBool(resolvedStaged, false) && !resolveOptionalBool(resolvedWorktree, false) {
		return nil, fmt.Errorf("At least one of staged or worktree must be true.")
	}

	args := []string{"restore"}
	if resolveOptionalBool(resolvedWorktree, false) {
		args = append(args, "--worktree")
	}
	if resolveOptionalBool(resolvedStaged, false) {
		args = append(args, "--staged")
	}
	if opts.Source != "" {
		args = append(args, "--source", shellEscape(opts.Source))
	}
	args = append(args, "--")
	for _, f := range paths {
		args = append(args, shellEscape(f))
	}
	return g.runGit(ctx, args, path, &opts.GitRequestOpts)
}

func (g *Git) Push(ctx context.Context, path string, opts *GitPushOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitPushOpts{}
	}
	if opts.Password != "" && opts.Username == "" {
		return nil, fmt.Errorf("Username is required when using a password or token for git push.")
	}
	setUpstream := resolveOptionalBool(opts.SetUpstream, true)

	operation := func(reqOpts *GitRequestOpts) (*commands.CommandResult, error) {
		args := []string{"push"}
		if setUpstream && opts.Remote != "" {
			args = append(args, "--set-upstream")
		}
		if opts.Remote != "" {
			args = append(args, shellEscape(opts.Remote))
		}
		if opts.Branch != "" {
			args = append(args, shellEscape(opts.Branch))
		}
		result, err := g.runGit(ctx, args, path, reqOpts)
		if err != nil {
			return nil, wrapGitUpstreamActionError(err, "push")
		}
		return result, nil
	}

	if opts.Username != "" && opts.Password != "" {
		remote, err := g.resolveRemoteName(ctx, path, opts.Remote, &opts.GitRequestOpts)
		if err != nil {
			return nil, err
		}
		return g.withRemoteCredentials(ctx, path, remote, opts.Username, opts.Password, &opts.GitRequestOpts, operation)
	}
	return operation(&opts.GitRequestOpts)
}

func resolveOptionalBool(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

func (g *Git) Pull(ctx context.Context, path string, opts *GitPullOpts) (*commands.CommandResult, error) {
	if opts == nil {
		opts = &GitPullOpts{}
	}
	if opts.Password != "" && opts.Username == "" {
		return nil, fmt.Errorf("Username is required when using a password or token for git pull.")
	}
	if opts.Remote == "" && opts.Branch == "" {
		hasUpstream, upstreamCheckErr := g.hasUpstream(ctx, path, &opts.GitRequestOpts)
		if upstreamCheckErr == nil && !hasUpstream {
			return nil, &shared.GitUpstreamError{
				SandboxError: shared.SandboxError{
					Message: buildUpstreamErrorMessage("pull"),
				},
			}
		}
	}

	operation := func(reqOpts *GitRequestOpts) (*commands.CommandResult, error) {
		args := []string{"pull"}
		if opts.Remote != "" {
			args = append(args, shellEscape(opts.Remote))
		}
		if opts.Branch != "" {
			args = append(args, shellEscape(opts.Branch))
		}
		result, err := g.runGit(ctx, args, path, reqOpts)
		if err != nil {
			return nil, wrapGitUpstreamActionError(err, "pull")
		}
		return result, nil
	}

	if opts.Username != "" && opts.Password != "" {
		remote, err := g.resolveRemoteName(ctx, path, opts.Remote, &opts.GitRequestOpts)
		if err != nil {
			return nil, err
		}
		return g.withRemoteCredentials(ctx, path, remote, opts.Username, opts.Password, &opts.GitRequestOpts, operation)
	}
	return operation(&opts.GitRequestOpts)
}

func (g *Git) SetConfig(ctx context.Context, key, value string, opts *GitConfigOpts) (*commands.CommandResult, error) {
	if key == "" {
		return nil, fmt.Errorf("Git config key is required.")
	}
	if opts == nil {
		opts = &GitConfigOpts{Scope: GitConfigGlobal}
	}

	scope := opts.Scope
	if scope == "" {
		scope = GitConfigGlobal
	}
	if err := validateConfigScope(scope); err != nil {
		return nil, err
	}
	repoPath, err := getRepoPathForScope(scope, opts.Path)
	if err != nil {
		return nil, err
	}

	args := []string{"config", getScopeFlag(scope), shellEscape(key), shellEscape(value)}
	return g.runGit(ctx, args, repoPath, &opts.GitRequestOpts)
}

func (g *Git) GetConfig(ctx context.Context, key string, opts *GitConfigOpts) (string, error) {
	if key == "" {
		return "", fmt.Errorf("Git config key is required.")
	}
	if opts == nil {
		opts = &GitConfigOpts{Scope: GitConfigGlobal}
	}

	scope := opts.Scope
	if scope == "" {
		scope = GitConfigGlobal
	}
	if err := validateConfigScope(scope); err != nil {
		return "", err
	}
	repoPath, err := getRepoPathForScope(scope, opts.Path)
	if err != nil {
		return "", err
	}

	cmd := buildGitCommand([]string{"config", getScopeFlag(scope), "--get", shellEscape(key)}, repoPath) + " || true"
	result, err := g.runShell(ctx, cmd, &opts.GitRequestOpts)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (g *Git) DangerouslyAuthenticate(ctx context.Context, opts *GitDangerouslyAuthenticateOpts) (*commands.CommandResult, error) {
	if opts == nil || opts.Username == "" || opts.Password == "" {
		return nil, fmt.Errorf("Both username and password are required to authenticate git.")
	}
	host := opts.Host
	if host == "" {
		host = "github.com"
	}
	protocol := opts.Protocol
	if protocol == "" {
		protocol = "https"
	}

	// Set credential helper to store
	configArgs := []string{"config", "--global", "credential.helper", "store"}
	_, err := g.runGit(ctx, configArgs, "", &opts.GitRequestOpts)
	if err != nil {
		return nil, err
	}

	// Pipe credential approve
	credentialInput := fmt.Sprintf("protocol=%s\nhost=%s\nusername=%s\npassword=%s\n\n", protocol, host, opts.Username, opts.Password)
	cmd := fmt.Sprintf("printf %%s %s | %s", shellEscape(credentialInput), buildGitCommand([]string{"credential", "approve"}, ""))
	return g.runShell(ctx, cmd, &opts.GitRequestOpts)
}

func (g *Git) ConfigureUser(ctx context.Context, name, email string, opts *GitConfigOpts) (*commands.CommandResult, error) {
	if name == "" || email == "" {
		return nil, fmt.Errorf("Both name and email are required.")
	}
	if opts == nil {
		opts = &GitConfigOpts{Scope: GitConfigGlobal}
	}
	if opts.Scope == "" {
		opts.Scope = GitConfigGlobal
	}

	if _, err := g.SetConfig(ctx, "user.name", name, opts); err != nil {
		return nil, err
	}
	return g.SetConfig(ctx, "user.email", email, opts)
}

// withRemoteCredentials temporarily sets credentials on the remote URL, runs the operation, then restores the original URL.
func (g *Git) withRemoteCredentials(ctx context.Context, path, remote, username, password string, opts *GitRequestOpts, operation func(*GitRequestOpts) (*commands.CommandResult, error)) (*commands.CommandResult, error) {
	// Get current remote URL
	originalUrl, err := g.getRemoteUrl(ctx, path, remote, opts)
	if err != nil {
		return nil, err
	}

	// Set URL with credentials
	credUrl := withCredentials(originalUrl, username, password)
	setArgs := []string{"remote", "set-url", shellEscape(remote), shellEscape(credUrl)}
	_, err = g.runGit(ctx, setArgs, path, opts)
	if err != nil {
		return nil, err
	}

	// Run the operation
	result, opErr := operation(opts)

	// Restore original URL
	restoreArgs := []string{"remote", "set-url", shellEscape(remote), shellEscape(originalUrl)}
	_, _ = g.runGit(ctx, restoreArgs, path, opts)

	return result, opErr
}

func (g *Git) getRemoteUrl(ctx context.Context, path, remote string, opts *GitRequestOpts) (string, error) {
	result, err := g.RemoteGet(ctx, path, remote, opts)
	if err != nil {
		return "", err
	}
	if result == "" {
		return "", fmt.Errorf("Remote %q URL not found in repository.", remote)
	}
	return result, nil
}

func (g *Git) resolveRemoteName(ctx context.Context, path, remote string, opts *GitRequestOpts) (string, error) {
	if remote != "" {
		return remote, nil
	}

	result, err := g.runGit(ctx, []string{"remote"}, path, opts)
	if err != nil {
		return "", err
	}

	remotes := make([]string, 0)
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			remotes = append(remotes, line)
		}
	}

	if len(remotes) == 1 {
		return remotes[0], nil
	}

	return "", fmt.Errorf("Remote is required when using username/password and the repository has multiple remotes.")
}

func (g *Git) hasUpstream(ctx context.Context, path string, opts *GitRequestOpts) (bool, error) {
	args := []string{"rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"}
	_, err := g.runGit(ctx, args, path, opts)
	if err == nil {
		return true, nil
	}
	var upstreamErr *shared.GitUpstreamError
	if errors.As(err, &upstreamErr) {
		return false, nil
	}
	return false, err
}
