package e2b_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
	cmdpkg "github.com/superduck-ai/e2b-go-sdk/commands"
	fspkg "github.com/superduck-ai/e2b-go-sdk/filesystem"
)

func externalSurfaceConnConfig(sandboxURL string) *e2b.ConnectionConfig {
	timeout := 1000
	return e2b.NewConnectionConfig(&e2b.ConnectionOpts{
		Domain:           "e2b.app",
		SandboxUrl:       sandboxURL,
		RequestTimeoutMs: &timeout,
		Headers: map[string]string{
			"X-Test": "value",
		},
	})
}

func writeExternalProcessEnvelope(t *testing.T, buf *bytes.Buffer, flags byte, payload []byte) {
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

func TestExternalPackageCanPassRootConnectionConfigToSubpackageConstructors(t *testing.T) {
	t.Run("commands", func(t *testing.T) {
		var gotUserAgent string
		var gotHeader string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/process.Process/List" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			gotUserAgent = r.Header.Get("User-Agent")
			gotHeader = r.Header.Get("X-Test")
			if err := json.NewEncoder(w).Encode(map[string]any{
				"processes": []any{},
			}); err != nil {
				t.Fatalf("failed to encode list response: %v", err)
			}
		}))
		defer server.Close()

		var cmds *e2b.Commands = cmdpkg.NewCommands(externalSurfaceConnConfig(server.URL), "1.0.0")
		processes, err := cmds.List(context.Background(), nil)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(processes) != 0 {
			t.Fatalf("expected no processes, got %#v", processes)
		}
		if gotHeader != "value" {
			t.Fatalf("expected X-Test header to be preserved, got %q", gotHeader)
		}
		if gotUserAgent != "e2b-go-sdk/dev" {
			t.Fatalf("expected User-Agent to be preserved, got %q", gotUserAgent)
		}
	})

	t.Run("filesystem", func(t *testing.T) {
		var gotUserAgent string
		var gotHeader string
		var gotPath string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/files" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			gotUserAgent = r.Header.Get("User-Agent")
			gotHeader = r.Header.Get("X-Test")
			gotPath = r.URL.Query().Get("path")
			if _, err := w.Write([]byte("hello")); err != nil {
				t.Fatalf("failed to write filesystem response: %v", err)
			}
		}))
		defer server.Close()

		var files *e2b.Filesystem = fspkg.NewFilesystem(externalSurfaceConnConfig(server.URL), "1.0.0")
		value, err := files.Read(context.Background(), "/tmp/hello.txt", nil)
		if err != nil {
			t.Fatalf("Read returned error: %v", err)
		}
		text, ok := value.(string)
		if !ok {
			t.Fatalf("expected string read result, got %T", value)
		}
		if text != "hello" {
			t.Fatalf("expected filesystem text %q, got %q", "hello", text)
		}
		if gotPath != "/tmp/hello.txt" {
			t.Fatalf("expected filesystem path query to be preserved, got %q", gotPath)
		}
		if gotHeader != "value" {
			t.Fatalf("expected X-Test header to be preserved, got %q", gotHeader)
		}
		if gotUserAgent != "e2b-go-sdk/dev" {
			t.Fatalf("expected User-Agent to be preserved, got %q", gotUserAgent)
		}
	})

	t.Run("pty", func(t *testing.T) {
		var gotUserAgent string
		var gotHeader string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/process.Process/Start" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			gotUserAgent = r.Header.Get("User-Agent")
			gotHeader = r.Header.Get("X-Test")
			w.WriteHeader(http.StatusOK)
			var stream bytes.Buffer
			writeExternalProcessEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
			writeExternalProcessEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
			if _, err := w.Write(stream.Bytes()); err != nil {
				t.Fatalf("failed to write pty response: %v", err)
			}
		}))
		defer server.Close()

		var pty *e2b.Pty = cmdpkg.NewPty(externalSurfaceConnConfig(server.URL), "1.0.0")
		handle, err := pty.Create(context.Background(), nil)
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		result, err := handle.Wait()
		if err != nil {
			t.Fatalf("Wait returned error: %v", err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %#v", result)
		}
		if gotHeader != "value" {
			t.Fatalf("expected X-Test header to be preserved, got %q", gotHeader)
		}
		if gotUserAgent != "e2b-go-sdk/dev" {
			t.Fatalf("expected User-Agent to be preserved, got %q", gotUserAgent)
		}
	})
}

func TestExternalPackageCanUseRunReturnAssertionsAndState(t *testing.T) {
	t.Run("foreground", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/process.Process/Start" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			var stream bytes.Buffer
			writeExternalProcessEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
			writeExternalProcessEnvelope(t, &stream, 0x00, []byte(`{"data":{"stdout":"aGkK"}}`))
			writeExternalProcessEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
			if _, err := w.Write(stream.Bytes()); err != nil {
				t.Fatalf("failed to write command response: %v", err)
			}
		}))
		defer server.Close()

		var cmds *e2b.Commands = cmdpkg.NewCommands(externalSurfaceConnConfig(server.URL), "1.0.0")
		execution, err := cmds.Run(context.Background(), "echo hi", nil)
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		result, ok := execution.(*e2b.CommandResult)
		if !ok {
			t.Fatalf("expected foreground result type assertion to succeed, got %T", execution)
		}
		if result.Stdout != "hi\n" {
			t.Fatalf("expected stdout %q, got %#v", "hi\n", result)
		}
	})

	t.Run("background", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/process.Process/Start" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			var stream bytes.Buffer
			writeExternalProcessEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":456}}`))
			writeExternalProcessEnvelope(t, &stream, 0x00, []byte(`{"data":{"stdout":"aGkK"}}`))
			writeExternalProcessEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
			if _, err := w.Write(stream.Bytes()); err != nil {
				t.Fatalf("failed to write command response: %v", err)
			}
		}))
		defer server.Close()

		var cmds *e2b.Commands = cmdpkg.NewCommands(externalSurfaceConnConfig(server.URL), "1.0.0")
		execution, err := cmds.Run(context.Background(), "echo hi", &e2b.CommandStartOpts{Background: true})
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		handle, ok := execution.(*e2b.CommandHandle)
		if !ok {
			t.Fatalf("expected background handle type assertion to succeed, got %T", execution)
		}
		if handle.Pid != 456 {
			t.Fatalf("expected pid 456, got %d", handle.Pid)
		}
		result, err := handle.Wait()
		if err != nil {
			t.Fatalf("Wait returned error: %v", err)
		}
		if result.Stdout != "hi\n" {
			t.Fatalf("expected stdout %q, got %#v", "hi\n", result)
		}
		state := handle.State()
		if state.Stdout != "hi\n" {
			t.Fatalf("expected state stdout %q, got %#v", "hi\n", state)
		}
		if state.ExitCode == nil || *state.ExitCode != 0 {
			t.Fatalf("expected state exit code 0, got %#v", state.ExitCode)
		}
	})
}
