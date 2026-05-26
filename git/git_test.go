package git

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/e2b-dev/e2b-go-sdk/commands"
	"github.com/e2b-dev/e2b-go-sdk/envd/process"
)

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
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode start request: %v", err)
		}

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

func TestPushWithoutRemoteDoesNotDefaultToOrigin(t *testing.T) {
	var got process.StartRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode start request: %v", err)
		}

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

		var got process.StartRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode start request: %v", err)
		}
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
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode start request: %v", err)
		}
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
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode start request: %v", err)
		}
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
	if len(commandsSeen) != 1 || commandsSeen[0] != "git -C '/tmp/repo' remote" {
		t.Fatalf("unexpected commands: %#v", commandsSeen)
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

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}
	return data
}
