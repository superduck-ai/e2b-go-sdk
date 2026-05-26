package commands

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testCommandsConfig(sandboxURL string, requestTimeoutMs int) *struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            bool
	RequestTimeoutMs int
	Headers          map[string]string
} {
	return &struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl:       sandboxURL,
		RequestTimeoutMs: requestTimeoutMs,
		Headers:          map[string]string{},
	}
}

func TestReadStreamEnvelopesEmitsEndStreamError(t *testing.T) {
	var stream bytes.Buffer
	writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":1}}`))
	writeEnvelope(t, &stream, 0x02, []byte(`{"error":{"code":"not_found","message":"missing"}}`))

	ch := make(chan streamEnvelope, 4)
	readStreamEnvelopes(&stream, ch)

	first, ok := <-ch
	if !ok {
		t.Fatal("expected first payload envelope")
	}
	if first.err != nil {
		t.Fatalf("unexpected first envelope error: %v", first.err)
	}

	second, ok := <-ch
	if !ok {
		t.Fatal("expected end-stream error envelope")
	}
	if second.err == nil {
		t.Fatal("expected end-stream error")
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

func TestCommandsListUsesDefaultRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"processes":[]}`))
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 20), "1.0.0")

	start := time.Now()
	_, err := cmds.List(context.Background(), nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed >= 150*time.Millisecond {
		t.Fatalf("expected default request timeout to trigger early, elapsed=%s", elapsed)
	}
}

func TestCommandsConnectWaitErrorsWhenStreamClosesWithoutResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Connect" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")

	handle, err := cmds.Connect(context.Background(), 123, nil)
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, waitErr := handle.Wait()
		done <- waitErr
	}()

	select {
	case waitErr := <-done:
		if !errors.Is(waitErr, errProcessExitedWithoutResult) {
			t.Fatalf("expected missing-result error, got %T %v", waitErr, waitErr)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait hung after stream closed without result")
	}
}

func TestCommandsConnectUsesDefaultRequestTimeoutBeforeResponseStarts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 20), "1.0.0")

	start := time.Now()
	_, err := cmds.Connect(context.Background(), 123, nil)
	if err == nil {
		t.Fatal("expected startup timeout error")
	}
	if elapsed := time.Since(start); elapsed >= 150*time.Millisecond {
		t.Fatalf("expected startup request timeout to trigger early, elapsed=%s", elapsed)
	}
}

func TestCommandsConnectErrorsWhenFirstEventIsNotStart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Connect" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"keepalive":true}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")

	_, err := cmds.Connect(context.Background(), 123, nil)
	if err == nil {
		t.Fatal("expected connect startup error")
	}
	if err.Error() != "Expected start event" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommandsConnectErrorsWhenStreamClosesBeforeFirstEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Connect" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")

	_, err := cmds.Connect(context.Background(), 123, nil)
	if err == nil {
		t.Fatal("expected connect startup error")
	}
	if err.Error() != "Expected start event" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunBackgroundRejectsStdinFalseOnOldEnvdBeforeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("did not expect start request for old envd stdin=false")
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "0.2.9")

	disabled := false
	_, err := cmds.RunBackground(context.Background(), "echo hi", &CommandStartOpts{
		StdinOpt: &disabled,
	})
	if err == nil {
		t.Fatal("expected stdin=false to be rejected on old envd")
	}
	if !strings.Contains(err.Error(), "can't specify stdin, it's always turned on") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunBackgroundAllowsOmittedStdinOnOldEnvd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "0.2.9")

	handle, err := cmds.RunBackground(context.Background(), "echo hi", &CommandStartOpts{})
	if err != nil {
		t.Fatalf("expected omitted stdin to be allowed on old envd, got %v", err)
	}
	if handle == nil {
		t.Fatal("expected command handle")
	}
}

func TestRunBackgroundErrorsWhenFirstEventIsNotStart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"keepalive":true}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")

	_, err := cmds.RunBackground(context.Background(), "echo hi", nil)
	if err == nil {
		t.Fatal("expected start error")
	}
	if err.Error() != "Expected start event" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunBackgroundErrorsWhenStreamClosesBeforeFirstEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")

	_, err := cmds.RunBackground(context.Background(), "echo hi", nil)
	if err == nil {
		t.Fatal("expected start error")
	}
	if err.Error() != "Expected start event" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCloseStdinRejectsUnsupportedEnvdWithAlignedMessage(t *testing.T) {
	cmds := NewCommands(testCommandsConfig("", 0), "0.5.1")

	err := cmds.closeStdin(context.Background(), 123, nil)
	if err == nil {
		t.Fatal("expected closeStdin to fail on unsupported envd")
	}
	if !strings.Contains(err.Error(), "doesn't support closeStdin") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "Please rebuild your template") {
		t.Fatalf("expected rebuild hint in error, got: %v", err)
	}
}

func TestRunRejectsBackgroundModeAndDoesNotStartProcess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("did not expect process start request when Run is used with background=true")
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")

	_, err := cmds.Run(context.Background(), "echo hi", &CommandStartOpts{
		Background: true,
	})
	if err == nil {
		t.Fatal("expected Run to reject background execution")
	}
	if !strings.Contains(err.Error(), "use RunBackground instead") {
		t.Fatalf("unexpected error: %v", err)
	}
}
