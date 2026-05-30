package commands

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

func boolPtr(v bool) *bool { return &v }

func runForegroundResult(cmds *Commands, ctx context.Context, cmd string, opts *CommandStartOpts) (*CommandResult, error) {
	execution, err := cmds.Run(ctx, cmd, opts)
	if err != nil {
		return nil, err
	}
	result, ok := execution.(*CommandResult)
	if !ok {
		return nil, fmt.Errorf("expected foreground command result, got %T", execution)
	}
	return result, nil
}

func runBackgroundHandle(cmds *Commands, ctx context.Context, cmd string, opts *CommandStartOpts) (*CommandHandle, error) {
	if opts == nil {
		opts = &CommandStartOpts{}
	} else {
		cloned := *opts
		opts = &cloned
	}
	opts.Background = true
	execution, err := cmds.Run(ctx, cmd, opts)
	if err != nil {
		return nil, err
	}
	handle, ok := execution.(*CommandHandle)
	if !ok {
		return nil, fmt.Errorf("expected background command handle, got %T", execution)
	}
	return handle, nil
}

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

func TestRunLogsStreamPayloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"data":{"stdout":"aGkK"}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	logger := &recordingCommandLogger{}
	cfg := &struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
		Logger           *recordingCommandLogger
	}{
		SandboxUrl: server.URL,
		Headers:    map[string]string{},
		Logger:     logger,
	}
	cmds := NewCommands(cfg, "1.0.0")

	result, err := runForegroundResult(cmds, context.Background(), "echo hi", nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Stdout != "hi\n" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
	if logger.debugCount == 0 {
		t.Fatal("expected stream payloads to be debug logged")
	}
}

func TestCommandsConnectUnaryRetriesTransientTransportErrors(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.URL.Path != "/process.Process/List" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if attempts == 1 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("expected hijacker support")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("failed to hijack connection: %v", err)
			}
			_ = conn.Close()
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"processes": []map[string]any{}}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")
	processes, err := cmds.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected retry to recover transient transport error, got %v", err)
	}
	if len(processes) != 0 {
		t.Fatalf("unexpected processes: %#v", processes)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
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

type recordingCommandLogger struct {
	debugCount int
}

func (l *recordingCommandLogger) Debug(args ...interface{}) { l.debugCount++ }
func (l *recordingCommandLogger) Info(args ...interface{})  {}
func (l *recordingCommandLogger) Warn(args ...interface{})  {}
func (l *recordingCommandLogger) Error(args ...interface{}) {}

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

func TestCommandsConnectMapsNotFoundToSdkNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Connect" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"code":"not_found","message":"process missing"}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")

	_, err := cmds.Connect(context.Background(), 999999, nil)
	var notFoundErr *shared.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T %v", err, err)
	}
	if notFoundErr.Message != "process missing" {
		t.Fatalf("unexpected not found message: %q", notFoundErr.Message)
	}
}

func TestRunMapsDeadlineExceededStreamErrorToSdkTimeoutError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeEnvelope(t, &stream, 0x02, []byte(`{"error":{"code":"deadline_exceeded","message":"command timed out"}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")

	_, err := runForegroundResult(cmds, context.Background(), "sleep 10", nil)
	var timeoutErr *shared.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("expected TimeoutError, got %T %v", err, err)
	}
	if timeoutErr.Message != "command timed out" {
		t.Fatalf("unexpected timeout message: %q", timeoutErr.Message)
	}
}

func TestWrapProcessErrorMapsContextDeadlineToSdkTimeoutError(t *testing.T) {
	err := wrapProcessError(context.DeadlineExceeded)

	var timeoutErr *shared.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("expected TimeoutError, got %T %v", err, err)
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
	handle, err := runBackgroundHandle(cmds, context.Background(), "echo hi", nil)
	if err != nil {
		t.Fatalf("RunBackground returned error: %v", err)
	}
	handle.Disconnect()
}

func TestRunBackgroundWaitAccumulatesOutputAndInvokesCallbacks(t *testing.T) {
	var stdoutChunks []string
	var stderrChunks []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"data":{"stdout":"SGVsbG8="}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"data":{"stderr":"Ym9vbQ=="}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")
	handle, err := runBackgroundHandle(cmds, context.Background(), "echo hi", &CommandStartOpts{
		OnStdout: func(data Stdout) {
			stdoutChunks = append(stdoutChunks, string(data))
		},
		OnStderr: func(data Stderr) {
			stderrChunks = append(stderrChunks, string(data))
		},
	})
	if err != nil {
		t.Fatalf("RunBackground returned error: %v", err)
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if result.ExitCode != 0 || result.Stdout != "Hello" || result.Stderr != "boom" {
		t.Fatalf("unexpected background result: %#v", result)
	}
	if got := handle.State().Stdout; got != "Hello" {
		t.Fatalf("expected handle stdout to accumulate background output, got %q", got)
	}
	if got := handle.State().Stderr; got != "boom" {
		t.Fatalf("expected handle stderr to accumulate background output, got %q", got)
	}
	if !reflect.DeepEqual(stdoutChunks, []string{"Hello"}) {
		t.Fatalf("unexpected stdout callback chunks: %#v", stdoutChunks)
	}
	if !reflect.DeepEqual(stderrChunks, []string{"boom"}) {
		t.Fatalf("unexpected stderr callback chunks: %#v", stderrChunks)
	}
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
	result, err := runForegroundResult(cmds, context.Background(), `echo "Hello from E2B!"`, nil)
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

func TestHandleProcessEventReplacesInvalidUTF8(t *testing.T) {
	handle := newCommandHandle(123, func() {}, func() (bool, error) { return false, nil }, nil, nil)
	cmds := &Commands{}

	cmds.handleProcessEvent([]byte(`{"data":{"stdout":"4g==","stderr":"4g=="}}`), handle)

	if got := handle.State().Stdout; got != "\uFFFD" {
		t.Fatalf("expected invalid stdout bytes to be replaced, got %q", got)
	}
	if got := handle.State().Stderr; got != "\uFFFD" {
		t.Fatalf("expected invalid stderr bytes to be replaced, got %q", got)
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

	_, err := runBackgroundHandle(cmds, context.Background(), "echo hi", &CommandStartOpts{
		Stdin: boolPtr(false),
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

	handle, err := runBackgroundHandle(cmds, context.Background(), "echo hi", &CommandStartOpts{})
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

	_, err := runBackgroundHandle(cmds, context.Background(), "echo hi", nil)
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

	_, err := runBackgroundHandle(cmds, context.Background(), "echo hi", nil)
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

func TestCommandStartOptsMatchJsAndPythonBackgroundSurface(t *testing.T) {
	optsType := reflect.TypeOf(CommandStartOpts{})
	want := []string{
		"CommandRequestOpts",
		"Background",
		"Cwd",
		"User",
		"Envs",
		"OnStdout",
		"OnStderr",
		"Stdin",
		"TimeoutMs",
	}

	got := make([]string, 0, optsType.NumField())
	for i := 0; i < optsType.NumField(); i++ {
		got = append(got, optsType.Field(i).Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected CommandStartOpts field shape: got %v want %v", got, want)
	}
	if field, ok := optsType.FieldByName("Background"); !ok {
		t.Fatal("expected CommandStartOpts to expose Background like JS/Python run options")
	} else if field.Type != reflect.TypeOf(false) {
		t.Fatalf("expected CommandStartOpts.Background to be bool, got %v", field.Type)
	}
	if field, ok := optsType.FieldByName("Stdin"); !ok {
		t.Fatal("expected CommandStartOpts to expose Stdin")
	} else if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected CommandStartOpts.Stdin to be *bool, got %v", field.Type)
	}
	if _, ok := optsType.FieldByName("StdinOpt"); ok {
		t.Fatal("did not expect CommandStartOpts to expose StdinOpt")
	}

	runMethod, ok := reflect.TypeOf(&Commands{}).MethodByName("Run")
	if !ok {
		t.Fatal("expected Commands to expose Run")
	}
	if got := runMethod.Type.Out(0).String(); got != "commands.commandExecution" {
		t.Fatalf("expected Run to return commands.commandExecution, got %s", got)
	}
	if _, ok := reflect.TypeOf(&Commands{}).MethodByName("RunForeground"); ok {
		t.Fatal("did not expect Commands to expose RunForeground")
	}
	if _, ok := reflect.TypeOf(&Commands{}).MethodByName("RunBackground"); ok {
		t.Fatal("did not expect Commands to expose RunBackground")
	}
	if _, ok := reflect.TypeOf(&Commands{}).MethodByName("RunWithMode"); ok {
		t.Fatal("did not expect Commands to expose RunWithMode")
	}
}

func TestRunUsesForegroundSemanticsWithoutBackgroundFlag(t *testing.T) {
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

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")

	execution, err := cmds.Run(context.Background(), "echo hi", nil)
	if err != nil {
		t.Fatalf("expected Run to execute in foreground, got %v", err)
	}
	result, ok := execution.(*CommandResult)
	if !ok {
		t.Fatalf("expected foreground Run to return *CommandResult, got %T", execution)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected foreground run exit code 0, got %#v", result)
	}
}

func TestRunUsesBackgroundFlagWhenRequested(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":456}}`))
		writeEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cmds := NewCommands(testCommandsConfig(server.URL, 0), "1.0.0")

	execution, err := cmds.Run(context.Background(), "sleep 1", &CommandStartOpts{Background: true})
	if err != nil {
		t.Fatalf("expected background Run to succeed, got %v", err)
	}
	handle, ok := execution.(*CommandHandle)
	if !ok {
		t.Fatalf("expected background Run to return *CommandHandle, got %T", execution)
	}
	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected background exit code 0, got %#v", result)
	}
}
