package git

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/superduck-ai/e2b-go-sdk/commands"
	"github.com/superduck-ai/e2b-go-sdk/envd/process"
	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

func boolPtr(v bool) *bool { return &v }

func directFieldNames(typ reflect.Type) []string {
	names := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		names = append(names, typ.Field(i).Name)
	}
	return names
}

func TestResolveOptionalBool(t *testing.T) {
	if !resolveOptionalBool(nil, true) {
		t.Fatal("expected nil to use default true")
	}

	value := false
	if resolveOptionalBool(&value, true) {
		t.Fatal("expected explicit false to be preserved")
	}
}

func TestBuildStartOptsMapsTimeouts(t *testing.T) {
	timeoutMs := 1234
	requestTimeoutMs := 5678

	g := &Git{}
	opts := g.buildStartOpts(&GitRequestOpts{
		TimeoutMs:        &timeoutMs,
		RequestTimeoutMs: &requestTimeoutMs,
	})

	if opts.TimeoutMs == nil || *opts.TimeoutMs != timeoutMs {
		t.Fatalf("expected timeoutMs %d, got %+v", timeoutMs, opts.TimeoutMs)
	}
	if opts.RequestTimeoutMs == nil || *opts.RequestTimeoutMs != requestTimeoutMs {
		t.Fatalf("expected requestTimeoutMs %d, got %+v", requestTimeoutMs, opts.RequestTimeoutMs)
	}
}

func TestInitPassesPathAsArgumentInsteadOfWorkingDirectory(t *testing.T) {
	var got process.StartRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		got = decodeStartRequest(t, r)

		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write stream: %v", err)
		}
	}))
	defer server.Close()

	cmds := commands.NewCommands(&struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl: server.URL,
		Headers:    map[string]string{},
	}, "1.0.0")
	g := NewGit(cmds)

	_, err := g.Init(context.Background(), "/tmp/repo", &GitInitOpts{
		Bare:          true,
		InitialBranch: "main",
	})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	if got.Process == nil {
		t.Fatal("expected process config in start request")
	}
	if got.Process.Cmd != "/bin/bash" {
		t.Fatalf("expected shell wrapper, got %q", got.Process.Cmd)
	}
	if len(got.Process.Args) != 3 {
		t.Fatalf("expected shell args [-l -c <cmd>], got %#v", got.Process.Args)
	}
	if got.Process.Args[2] != "git init --initial-branch 'main' --bare '/tmp/repo'" {
		t.Fatalf("unexpected git init command: %q", got.Process.Args[2])
	}
	if got.Process.Cwd != "" {
		t.Fatalf("expected init to avoid repo cwd, got %q", got.Process.Cwd)
	}
}

func TestRemoteAddRejectsMissingNameOrURL(t *testing.T) {
	g := NewGit(nil)

	if _, err := g.RemoteAdd(context.Background(), "/tmp/repo", "", "https://example.com/repo.git", nil); err == nil {
		t.Fatal("expected missing remote name to fail")
	} else if err.Error() != "Both remote name and URL are required to add a git remote." {
		t.Fatalf("unexpected error for missing remote name: %v", err)
	}

	if _, err := g.RemoteAdd(context.Background(), "/tmp/repo", "origin", "", nil); err == nil {
		t.Fatal("expected missing remote url to fail")
	} else if err.Error() != "Both remote name and URL are required to add a git remote." {
		t.Fatalf("unexpected error for missing remote url: %v", err)
	}
}

func TestRemoteGetRejectsMissingName(t *testing.T) {
	g := NewGit(nil)

	_, err := g.RemoteGet(context.Background(), "/tmp/repo", "", nil)
	if err == nil {
		t.Fatal("expected missing remote name to fail")
	}
	if err.Error() != "Remote name is required." {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGitOptionStructsMatchJsAndPythonFieldShapes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		typ  reflect.Type
		want []string
	}{
		{
			name: "clone",
			typ:  reflect.TypeOf(GitCloneOpts{}),
			want: []string{"GitRequestOpts", "Path", "Branch", "Depth", "Username", "Password", "DangerouslyStoreCredentials"},
		},
		{
			name: "commit",
			typ:  reflect.TypeOf(GitCommitOpts{}),
			want: []string{"GitRequestOpts", "AuthorName", "AuthorEmail", "AllowEmpty"},
		},
		{
			name: "push",
			typ:  reflect.TypeOf(GitPushOpts{}),
			want: []string{"GitRequestOpts", "Remote", "Branch", "SetUpstream", "Username", "Password"},
		},
		{
			name: "pull",
			typ:  reflect.TypeOf(GitPullOpts{}),
			want: []string{"GitRequestOpts", "Remote", "Branch", "Username", "Password"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := directFieldNames(tc.typ); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected field shape for %s opts: got %v want %v", tc.name, got, tc.want)
			}
		})
	}

	cloneOptsType := reflect.TypeOf(GitCloneOpts{})
	if field, ok := cloneOptsType.FieldByName("Depth"); !ok {
		t.Fatal("expected GitCloneOpts to expose Depth")
	} else if field.Type != reflect.TypeOf((*int)(nil)) {
		t.Fatalf("expected GitCloneOpts.Depth to be *int, got %v", field.Type)
	}
}

func TestCloneUsesOptionalDepthLikeJsAndPython(t *testing.T) {
	var commandsSeen []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		got := decodeStartRequest(t, r)
		if got.Process == nil || len(got.Process.Args) != 3 {
			t.Fatalf("unexpected process config: %#v", got.Process)
		}
		commandsSeen = append(commandsSeen, got.Process.Args[2])

		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write stream: %v", err)
		}
	}))
	defer server.Close()

	cmds := commands.NewCommands(&struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl: server.URL,
		Headers:    map[string]string{},
	}, "1.0.0")
	g := NewGit(cmds)

	if _, err := g.Clone(context.Background(), "https://example.com/repo.git", &GitCloneOpts{Path: "/tmp/repo"}); err != nil {
		t.Fatalf("Clone without depth returned error: %v", err)
	}
	depth := 3
	if _, err := g.Clone(context.Background(), "https://example.com/repo.git", &GitCloneOpts{Path: "/tmp/repo", Depth: &depth}); err != nil {
		t.Fatalf("Clone with depth returned error: %v", err)
	}

	want := []string{
		"git clone 'https://example.com/repo.git' '/tmp/repo'",
		"git clone 'https://example.com/repo.git' --depth 3 '/tmp/repo'",
	}
	if !reflect.DeepEqual(commandsSeen, want) {
		t.Fatalf("unexpected clone commands: got %#v want %#v", commandsSeen, want)
	}
}

func TestPushWithoutRemoteDoesNotDefaultToOrigin(t *testing.T) {
	var got process.StartRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		got = decodeStartRequest(t, r)

		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write stream: %v", err)
		}
	}))
	defer server.Close()

	cmds := commands.NewCommands(&struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl: server.URL,
		Headers:    map[string]string{},
	}, "1.0.0")
	g := NewGit(cmds)

	_, err := g.Push(context.Background(), "/tmp/repo", nil)
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}

	if got.Process == nil || len(got.Process.Args) != 3 {
		t.Fatalf("unexpected process config: %#v", got.Process)
	}
	if got.Process.Args[2] != "git -C '/tmp/repo' push" {
		t.Fatalf("unexpected git push command: %q", got.Process.Args[2])
	}
}

func TestCommitUsesJsAndPythonAuthorConfigOrder(t *testing.T) {
	var got process.StartRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		got = decodeStartRequest(t, r)

		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write stream: %v", err)
		}
	}))
	defer server.Close()

	cmds := commands.NewCommands(&struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl: server.URL,
		Headers:    map[string]string{},
	}, "1.0.0")
	g := NewGit(cmds)

	_, err := g.Commit(context.Background(), "/tmp/repo", "Initial commit", &GitCommitOpts{
		AuthorName:  "Sandbox Bot",
		AuthorEmail: "sandbox@example.com",
		AllowEmpty:  true,
	})
	if err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if got.Process == nil || len(got.Process.Args) != 3 {
		t.Fatalf("unexpected process config: %#v", got.Process)
	}
	expected := "git -C '/tmp/repo' -c 'user.name=Sandbox Bot' -c 'user.email=sandbox@example.com' commit -m 'Initial commit' --allow-empty"
	if got.Process.Args[2] != expected {
		t.Fatalf("unexpected git commit command: %q", got.Process.Args[2])
	}
}

func TestBranchesEscapesFormatArgument(t *testing.T) {
	var got process.StartRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		got = decodeStartRequest(t, r)

		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"data":{"stdout":"bWFzdGVyCSoK"}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write stream: %v", err)
		}
	}))
	defer server.Close()

	cmds := commands.NewCommands(&struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl: server.URL,
		Headers:    map[string]string{},
	}, "1.0.0")
	g := NewGit(cmds)

	branches, err := g.Branches(context.Background(), "/tmp/repo", nil)
	if err != nil {
		t.Fatalf("Branches returned error: %v", err)
	}
	if branches.CurrentBranch != "master" {
		t.Fatalf("unexpected current branch: %#v", branches)
	}
	if got.Process == nil || len(got.Process.Args) != 3 {
		t.Fatalf("unexpected process config: %#v", got.Process)
	}
	expected := "git -C '/tmp/repo' branch '--format=%(refname:short)\t%(HEAD)'"
	if got.Process.Args[2] != expected {
		t.Fatalf("unexpected git branches command: %q", got.Process.Args[2])
	}
}

func TestPushRejectsPasswordWithoutUsername(t *testing.T) {
	g := NewGit(nil)

	_, err := g.Push(context.Background(), "/tmp/repo", &GitPushOpts{
		Password: "token",
	})
	if err == nil {
		t.Fatal("expected password-only push auth to fail")
	}
	if err.Error() != "Username is required when using a password or token for git push." {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPullRejectsPasswordWithoutUsername(t *testing.T) {
	g := NewGit(nil)

	_, err := g.Pull(context.Background(), "/tmp/repo", &GitPullOpts{
		Password: "token",
	})
	if err == nil {
		t.Fatal("expected password-only pull auth to fail")
	}
	if err.Error() != "Username is required when using a password or token for git pull." {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGitRestoreOptsExposeJsStylePathsField(t *testing.T) {
	optsType := reflect.TypeOf(GitRestoreOpts{})

	pathsField, ok := optsType.FieldByName("Paths")
	if !ok {
		t.Fatal("expected GitRestoreOpts to expose Paths")
	}
	if pathsField.Type != reflect.TypeOf([]string{}) {
		t.Fatalf("expected GitRestoreOpts.Paths to be []string, got %v", pathsField.Type)
	}

	filesField, ok := optsType.FieldByName("Files")
	if !ok {
		t.Fatal("expected GitRestoreOpts to keep legacy Files alias for compatibility")
	}
	if filesField.Type != reflect.TypeOf([]string{}) {
		t.Fatalf("expected GitRestoreOpts.Files to be []string, got %v", filesField.Type)
	}
}

func TestRestoreUsesPathsFieldLikeJsAndPython(t *testing.T) {
	var got process.StartRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		got = decodeStartRequest(t, r)

		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write stream: %v", err)
		}
	}))
	defer server.Close()

	cmds := commands.NewCommands(&struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl: server.URL,
		Headers:    map[string]string{},
	}, "1.0.0")
	g := NewGit(cmds)

	_, err := g.Restore(context.Background(), "/tmp/repo", &GitRestoreOpts{
		Paths:    []string{"README.md"},
		Staged:   boolPtr(true),
		Worktree: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}

	if got.Process == nil || len(got.Process.Args) != 3 {
		t.Fatalf("unexpected process config: %#v", got.Process)
	}
	expected := "git -C '/tmp/repo' restore --staged -- 'README.md'"
	if got.Process.Args[2] != expected {
		t.Fatalf("unexpected git restore command: %q", got.Process.Args[2])
	}
}

func TestRestoreFallsBackToLegacyFilesFieldForCompatibility(t *testing.T) {
	var got process.StartRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		got = decodeStartRequest(t, r)

		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write stream: %v", err)
		}
	}))
	defer server.Close()

	cmds := commands.NewCommands(&struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl: server.URL,
		Headers:    map[string]string{},
	}, "1.0.0")
	g := NewGit(cmds)

	_, err := g.Restore(context.Background(), "/tmp/repo", &GitRestoreOpts{
		Files: []string{"README.md"},
	})
	if err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}

	if got.Process == nil || len(got.Process.Args) != 3 {
		t.Fatalf("unexpected process config: %#v", got.Process)
	}
	expected := "git -C '/tmp/repo' restore --worktree -- 'README.md'"
	if got.Process.Args[2] != expected {
		t.Fatalf("unexpected git restore command: %q", got.Process.Args[2])
	}
}

func TestSetConfigRejectsEmptyKey(t *testing.T) {
	g := NewGit(nil)

	_, err := g.SetConfig(context.Background(), "", "value", nil)
	if err == nil {
		t.Fatal("expected empty git config key to fail")
	}
	if err.Error() != "Git config key is required." {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetConfigRequiresPathForLocalScope(t *testing.T) {
	g := NewGit(nil)

	_, err := g.SetConfig(context.Background(), "user.name", "Alice", &GitConfigOpts{
		Scope: GitConfigLocal,
	})
	if err == nil {
		t.Fatal("expected local scope without path to fail")
	}
	if err.Error() != "A repository path is required when using scope \"local\"." {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigRejectsInvalidScope(t *testing.T) {
	g := NewGit(nil)

	if _, err := g.SetConfig(context.Background(), "user.name", "Alice", &GitConfigOpts{Scope: GitConfigScope("workspace")}); err == nil {
		t.Fatal("expected SetConfig with invalid scope to fail")
	} else if err.Error() != "Git config scope must be one of: global, local, system." {
		t.Fatalf("unexpected SetConfig error: %v", err)
	}

	if _, err := g.GetConfig(context.Background(), "user.name", &GitConfigOpts{Scope: GitConfigScope("workspace")}); err == nil {
		t.Fatal("expected GetConfig with invalid scope to fail")
	} else if err.Error() != "Git config scope must be one of: global, local, system." {
		t.Fatalf("unexpected GetConfig error: %v", err)
	}
}

func TestWrapErrorDoesNotTreatLocalPermissionDeniedAsAuth(t *testing.T) {
	g := NewGit(nil)
	err := g.wrapError(&commands.CommandExitError{
		CommandResult: commands.CommandResult{
			ExitCode: 1,
			Stderr:   "error: could not lock config file /etc/gitconfig: Permission denied",
		},
		Message: "exit status 1",
	})

	var authErr *shared.AuthenticationError
	if errors.As(err, &authErr) {
		t.Fatalf("local permission error should not be classified as auth: %T %v", err, err)
	}
	var exitErr *commands.CommandExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected original CommandExitError, got %T %v", err, err)
	}
}

func TestWrapErrorTreatsSshPublicKeyPermissionDeniedAsAuth(t *testing.T) {
	g := NewGit(nil)
	err := g.wrapError(&commands.CommandExitError{
		CommandResult: commands.CommandResult{
			ExitCode: 1,
			Stderr:   "git@github.com: Permission denied (publickey).",
		},
		Message: "exit status 1",
	})

	var authErr *shared.AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected SSH public key error to be classified as auth, got %T %v", err, err)
	}
}

func TestGetConfigRejectsEmptyKey(t *testing.T) {
	g := NewGit(nil)

	_, err := g.GetConfig(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected empty git config key to fail")
	}
	if err.Error() != "Git config key is required." {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDangerouslyAuthenticateRequiresUsernameAndPassword(t *testing.T) {
	g := NewGit(nil)

	_, err := g.DangerouslyAuthenticate(context.Background(), &GitDangerouslyAuthenticateOpts{
		Username: "alice",
	})
	if err == nil {
		t.Fatal("expected incomplete git auth opts to fail")
	}
	if err.Error() != "Both username and password are required to authenticate git." {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDangerouslyAuthenticateUsesPrintfCredentialApprove(t *testing.T) {
	var commandsSeen []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		got := decodeStartRequest(t, r)
		if got.Process == nil || len(got.Process.Args) != 3 {
			t.Fatalf("unexpected process config: %#v", got.Process)
		}
		commandsSeen = append(commandsSeen, got.Process.Args[2])

		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write stream: %v", err)
		}
	}))
	defer server.Close()

	cmds := commands.NewCommands(&struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl: server.URL,
		Headers:    map[string]string{},
	}, "1.0.0")
	g := NewGit(cmds)

	_, err := g.DangerouslyAuthenticate(context.Background(), &GitDangerouslyAuthenticateOpts{
		Username: "alice",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("DangerouslyAuthenticate returned error: %v", err)
	}
	if len(commandsSeen) != 2 {
		t.Fatalf("expected two git commands, got %#v", commandsSeen)
	}
	if commandsSeen[0] != "git config --global credential.helper store" {
		t.Fatalf("unexpected config command: %q", commandsSeen[0])
	}
	expectedApprove := "printf %s 'protocol=https\nhost=github.com\nusername=alice\npassword=secret\n\n' | git credential approve"
	if commandsSeen[1] != expectedApprove {
		t.Fatalf("unexpected credential approve command: %q", commandsSeen[1])
	}
}

func TestConfigureUserRequiresNameAndEmail(t *testing.T) {
	g := NewGit(nil)

	_, err := g.ConfigureUser(context.Background(), "Alice", "", nil)
	if err == nil {
		t.Fatal("expected missing email to fail")
	}
	if err.Error() != "Both name and email are required." {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPushWithCredentialsRequiresExplicitRemoteWhenMultipleRemotes(t *testing.T) {
	var commandsSeen []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got process.StartRequest
		got = decodeStartRequest(t, r)
		if got.Process == nil || len(got.Process.Args) != 3 {
			t.Fatalf("unexpected process config: %#v", got.Process)
		}
		commandsSeen = append(commandsSeen, got.Process.Args[2])

		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, mustJSON(t, process.ProcessEvent{Start: &process.ProcessStartEvent{Pid: 123}}))
		writeEnvelope(t, &stream, 0x00, mustJSON(t, process.ProcessEvent{Data: &process.ProcessDataEvent{Stdout: []byte("origin\nupstream\n")}}))
		writeEnvelope(t, &stream, 0x00, mustJSON(t, process.ProcessEvent{End: &process.ProcessEndEvent{ExitCode: 0}}))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write stream: %v", err)
		}
	}))
	defer server.Close()

	cmds := commands.NewCommands(&struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl: server.URL,
		Headers:    map[string]string{},
	}, "1.0.0")
	g := NewGit(cmds)

	_, err := g.Push(context.Background(), "/tmp/repo", &GitPushOpts{
		Username: "alice",
		Password: "secret",
	})
	if err == nil {
		t.Fatal("expected push with multiple remotes to fail")
	}
	if err.Error() != "Remote is required when using username/password and the repository has multiple remotes." {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commandsSeen) != 1 || commandsSeen[0] != "git -C '/tmp/repo' remote" {
		t.Fatalf("unexpected commands: %#v", commandsSeen)
	}
}

func TestPullWithCredentialsRequiresExplicitRemoteWhenMultipleRemotes(t *testing.T) {
	var commandsSeen []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got process.StartRequest
		got = decodeStartRequest(t, r)
		if got.Process == nil || len(got.Process.Args) != 3 {
			t.Fatalf("unexpected process config: %#v", got.Process)
		}
		commandsSeen = append(commandsSeen, got.Process.Args[2])

		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, mustJSON(t, process.ProcessEvent{Start: &process.ProcessStartEvent{Pid: 123}}))
		writeEnvelope(t, &stream, 0x00, mustJSON(t, process.ProcessEvent{Data: &process.ProcessDataEvent{Stdout: []byte("origin\nupstream\n")}}))
		writeEnvelope(t, &stream, 0x00, mustJSON(t, process.ProcessEvent{End: &process.ProcessEndEvent{ExitCode: 0}}))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write stream: %v", err)
		}
	}))
	defer server.Close()

	cmds := commands.NewCommands(&struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl: server.URL,
		Headers:    map[string]string{},
	}, "1.0.0")
	g := NewGit(cmds)

	_, err := g.Pull(context.Background(), "/tmp/repo", &GitPullOpts{
		Username: "alice",
		Password: "secret",
	})
	if err == nil {
		t.Fatal("expected pull with multiple remotes to fail")
	}
	if err.Error() != "Remote is required when using username/password and the repository has multiple remotes." {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedCommands := []string{
		"git -C '/tmp/repo' rev-parse --abbrev-ref --symbolic-full-name @{u}",
		"git -C '/tmp/repo' remote",
	}
	if len(commandsSeen) != len(expectedCommands) {
		t.Fatalf("unexpected commands: %#v", commandsSeen)
	}
	for i := range expectedCommands {
		if commandsSeen[i] != expectedCommands[i] {
			t.Fatalf("unexpected commands: %#v", commandsSeen)
		}
	}
}

func TestPullRunsPullWhenUpstreamPreflightFailsForOtherReason(t *testing.T) {
	var commandsSeen []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := decodeStartRequest(t, r)
		if got.Process == nil || len(got.Process.Args) != 3 {
			t.Fatalf("unexpected process config: %#v", got.Process)
		}
		command := got.Process.Args[2]
		commandsSeen = append(commandsSeen, command)

		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, mustJSON(t, process.ProcessEvent{Start: &process.ProcessStartEvent{Pid: 123}}))
		writeEnvelope(t, &stream, 0x00, mustJSON(t, process.ProcessEvent{Data: &process.ProcessDataEvent{Stderr: []byte("fatal: not a git repository (or any of the parent directories): .git\n")}}))
		writeEnvelope(t, &stream, 0x00, mustJSON(t, process.ProcessEvent{End: &process.ProcessEndEvent{ExitCode: 1, Error: "exit status 1"}}))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write stream: %v", err)
		}
	}))
	defer server.Close()

	cmds := commands.NewCommands(&struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl: server.URL,
		Headers:    map[string]string{},
	}, "1.0.0")
	g := NewGit(cmds)

	_, err := g.Pull(context.Background(), "/tmp/not-repo", nil)
	if err == nil {
		t.Fatal("expected Pull from non-repository path to fail")
	}
	var upstreamErr *shared.GitUpstreamError
	if errors.As(err, &upstreamErr) {
		t.Fatalf("non-repository pull should preserve the git failure, got upstream error: %v", err)
	}
	var exitErr *commands.CommandExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected CommandExitError, got %T %v", err, err)
	}
	expectedCommands := []string{
		"git -C '/tmp/not-repo' rev-parse --abbrev-ref --symbolic-full-name @{u}",
		"git -C '/tmp/not-repo' pull",
	}
	if len(commandsSeen) != len(expectedCommands) {
		t.Fatalf("unexpected commands: %#v", commandsSeen)
	}
	for i := range expectedCommands {
		if commandsSeen[i] != expectedCommands[i] {
			t.Fatalf("unexpected commands: %#v", commandsSeen)
		}
	}
}

func writeEnvelope(t *testing.T, buf *bytes.Buffer, flags byte, payload []byte) {
	t.Helper()

	header := make([]byte, 5)
	header[0] = flags
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	if _, err := buf.Write(header); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}
	if _, err := buf.Write(payload); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}
}

func decodeStartRequest(t *testing.T, r *http.Request) process.StartRequest {
	t.Helper()
	if got := r.Header.Get("Content-Type"); got != "application/connect+json" {
		t.Fatalf("expected connect content type, got %q", got)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read start request: %v", err)
	}
	if len(body) < 5 {
		t.Fatalf("expected connect envelope body, got %d bytes", len(body))
	}
	if body[0] != 0 {
		t.Fatalf("expected uncompressed envelope flag 0, got %d", body[0])
	}
	length := int(binary.BigEndian.Uint32(body[1:5]))
	if length != len(body)-5 {
		t.Fatalf("expected envelope length %d, got %d payload bytes", length, len(body)-5)
	}

	var got process.StartRequest
	if err := json.Unmarshal(body[5:], &got); err != nil {
		t.Fatalf("failed to decode start request: %v", err)
	}
	return got
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}
	return data
}
