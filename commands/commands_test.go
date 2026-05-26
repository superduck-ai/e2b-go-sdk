package commands

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
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

func assertConnectEnvelopeRequest(t *testing.T, r *http.Request) []byte {
	t.Helper()
	if got := r.Header.Get("Content-Type"); got != "application/connect+json" {
		t.Fatalf("expected connect content type, got %q", got)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
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
	return body[5:]
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

func TestRunBackgroundSendsConnectEnvelopeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		payload := assertConnectEnvelopeRequest(t, r)
		if !bytes.Contains(payload, []byte(`"/bin/bash"`)) || !bytes.Contains(payload, []byte(`"echo hi"`)) {
			t.Fatalf("unexpected start payload: %s", string(payload))
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
	handle, err := cmds.RunBackground(context.Background(), "echo hi", nil)
	if err != nil {
		t.Fatalf("RunBackground returned error: %v", err)
	}
	handle.Disconnect()
}

func TestCommandsConnectSendsProcessSelectorRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Connect" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		payload := assertConnectEnvelopeRequest(t, r)
		var req map[string]any
		if err := json.Unmarshal(payload, &req); err != nil {
			t.Fatalf("failed to unmarshal connect request: %v", err)
		}
		if _, ok := req["pid"]; ok {
			t.Fatalf("did not expect legacy top-level pid request: %s", payload)
		}
		processReq, ok := req["process"].(map[string]any)
		if !ok || processReq["pid"] != float64(123) {
			t.Fatalf("expected process selector pid request, got %s", payload)
		}

		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"event":{"start":{"pid":123}}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"event":{"end":{"exited":true}}}`))
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
	_, _ = handle.Wait()
}

func TestSendStdinSendsProcessSelectorAndInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/SendInput" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request: %v", err)
		}
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if _, ok := req["pid"]; ok {
			t.Fatalf("did not expect legacy top-level pid request: %s", body)
		}
		processReq, ok := req["process"].(map[string]any)
		if !ok || processReq["pid"] != float64(123) {
			t.Fatalf("expected process selector pid request, got %s", body)
		}
		inputReq, ok := req["input"].(map[string]any)
		if !ok || inputReq["stdin"] != "aGVsbG8K" {
			t.Fatalf("expected stdin input payload, got %s", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")
	if err := cmds.SendStdin(context.Background(), 123, []byte("hello\n"), nil); err != nil {
		t.Fatalf("SendStdin returned error: %v", err)
	}
}

func TestRunHandlesCurrentConnectJSONEventWrapper(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = assertConnectEnvelopeRequest(t, r)
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"event":{"start":{"pid":123}}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"event":{"data":{"stdout":"SGVsbG8gZnJvbSBFMkIhCg=="}}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"event":{"end":{"exited":true,"status":"exit status 0"}}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")
	result, err := cmds.Run(context.Background(), `echo "Hello from E2B!"`, nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "Hello from E2B!\n" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
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

func TestCommandsConnectWaitSucceedsWhenEndArrivesBeforeStreamCloses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Connect" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"event":{"start":{"pid":123}}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"event":{"data":{"stdout":"Q09OTkVDVF9PSwo="}}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"event":{"end":{"exited":true,"status":"exit status 0"}}}`))
		writeEnvelope(t, &stream, 0x02, []byte(`{}`))
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

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if result.Stdout != "CONNECT_OK\n" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
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
