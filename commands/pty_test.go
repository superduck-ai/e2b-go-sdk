package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/envd/process"
)

func directFieldNames(typ reflect.Type) []string {
	names := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		names = append(names, typ.Field(i).Name)
	}
	return names
}

func TestPtyKillReturnsFalseForGenericNotFoundMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/SendSignal" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"code":"unknown","message":"pty not found"}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	pty := NewPty(testCommandsConfig(server.URL, 0), "1.0.0")

	killed, err := pty.Kill(context.Background(), 123, nil)
	if err != nil {
		t.Fatalf("expected no error for missing process, got %v", err)
	}
	if killed {
		t.Fatal("expected Kill to return false when process is missing")
	}
}

func TestPtyCreateSendsConnectEnvelopeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		payload := assertConnectEnvelopeRequest(t, r)
		if !bytes.Contains(payload, []byte(`"/bin/bash"`)) || !bytes.Contains(payload, []byte(`"pty"`)) {
			t.Fatalf("unexpected pty payload: %s", string(payload))
		}
		var req map[string]any
		if err := json.Unmarshal(payload, &req); err != nil {
			t.Fatalf("failed to unmarshal pty request: %v", err)
		}
		ptyReq, ok := req["pty"].(map[string]any)
		if !ok {
			t.Fatalf("expected pty request, got %s", payload)
		}
		if _, ok := ptyReq["cols"]; ok {
			t.Fatalf("did not expect legacy top-level pty cols request: %s", payload)
		}
		sizeReq, ok := ptyReq["size"].(map[string]any)
		if !ok || sizeReq["cols"] != float64(80) || sizeReq["rows"] != float64(24) {
			t.Fatalf("expected pty.size cols/rows request, got %s", payload)
		}
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	pty := NewPty(testCommandsConfig(server.URL, 0), "1.0.0")
	handle, err := pty.Create(context.Background(), &PtyCreateOpts{OnData: func(PtyOutput) {}})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	handle.Disconnect()
}

func TestPtyOnDataReceivesRawBytes(t *testing.T) {
	raw := []byte{0xff, 0xfe, 'O', 'K'}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeEnvelope(t, &stream, 0x00, mustJSON(t, process.ProcessEvent{Start: &process.ProcessStartEvent{Pid: 123}}))
		writeEnvelope(t, &stream, 0x00, mustJSON(t, process.ProcessEvent{Data: &process.ProcessDataEvent{Pty: raw}}))
		writeEnvelope(t, &stream, 0x00, mustJSON(t, process.ProcessEvent{End: &process.ProcessEndEvent{ExitCode: 0}}))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	pty := NewPty(testCommandsConfig(server.URL, 0), "1.0.0")
	var got []byte
	handle, err := pty.Create(context.Background(), &PtyCreateOpts{
		OnData: func(data PtyOutput) {
			got = append([]byte(nil), data...)
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("expected raw PTY bytes %#v, got %#v", raw, got)
	}
	if result.Stdout == string(raw) {
		t.Fatal("expected stdout aggregation to sanitize invalid UTF-8, not preserve raw bytes")
	}
}

func TestPtyResizeUsesJsAndPythonStyleSizeSurface(t *testing.T) {
	t.Parallel()

	sizeType := reflect.TypeOf(PtySize{})
	if got, want := directFieldNames(sizeType), []string{"Cols", "Rows"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected PtySize field shape: got %v want %v", got, want)
	}

	resize, ok := reflect.TypeOf(&Pty{}).MethodByName("Resize")
	if !ok {
		t.Fatal("expected Pty to expose Resize")
	}
	if got := resize.Type.NumIn(); got != 5 {
		t.Fatalf("unexpected Resize arity: got %d want 5", got)
	}
	if got := resize.Type.In(3); got != reflect.TypeOf(PtySize{}) {
		t.Fatalf("expected Resize size parameter to be PtySize, got %v", got)
	}
	if got := resize.Type.In(4); got != reflect.TypeOf((*CommandRequestOpts)(nil)) {
		t.Fatalf("expected Resize opts parameter to be *CommandRequestOpts, got %v", got)
	}
}

func TestPtyResizeUsesNestedSizeRequestLikeJsAndPython(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Update" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		ptyReq, ok := req["pty"].(map[string]any)
		if !ok {
			t.Fatalf("expected pty request, got %#v", req)
		}
		if _, ok := ptyReq["cols"]; ok {
			t.Fatalf("did not expect legacy top-level pty cols request: %#v", req)
		}
		sizeReq, ok := ptyReq["size"].(map[string]any)
		if !ok {
			t.Fatalf("expected nested pty size request, got %#v", req)
		}
		if sizeReq["cols"] != float64(100) || sizeReq["rows"] != float64(24) {
			t.Fatalf("unexpected pty size payload: %#v", sizeReq)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pty := NewPty(testCommandsConfig(server.URL, 0), "1.0.0")
	if err := pty.Resize(context.Background(), 123, PtySize{Cols: 100, Rows: 24}, nil); err != nil {
		t.Fatalf("Resize returned error: %v", err)
	}
}

func TestPtyConnectUnaryRetriesTransientTransportErrors(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.URL.Path != "/process.Process/SendSignal" {
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
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pty := NewPty(testCommandsConfig(server.URL, 0), "1.0.0")
	killed, err := pty.Kill(context.Background(), 123, nil)
	if err != nil {
		t.Fatalf("expected retry to recover transient transport error, got %v", err)
	}
	if !killed {
		t.Fatal("expected Kill to report true after retry succeeds")
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestPtyKillUsesDefaultRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pty := NewPty(testCommandsConfig(server.URL, 20), "1.0.0")

	start := time.Now()
	_, err := pty.Kill(context.Background(), 123, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed >= 150*time.Millisecond {
		t.Fatalf("expected default request timeout to trigger early, elapsed=%s", elapsed)
	}
}

func TestPtyConnectWaitErrorsWhenStreamClosesWithoutResult(t *testing.T) {
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

	pty := NewPty(testCommandsConfig(server.URL, 0), "1.0.0")

	handle, err := pty.Connect(context.Background(), 123, nil)
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
		t.Fatal("Wait hung after PTY stream closed without result")
	}
}

func TestPtyCreateErrorsWhenFirstEventIsNotStart(t *testing.T) {
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

	pty := NewPty(testCommandsConfig(server.URL, 0), "1.0.0")

	_, err := pty.Create(context.Background(), nil)
	if err == nil {
		t.Fatal("expected start error")
	}
	if err.Error() != "Expected start event" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPtyCreateErrorsWhenStreamClosesBeforeFirstEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pty := NewPty(testCommandsConfig(server.URL, 0), "1.0.0")

	_, err := pty.Create(context.Background(), nil)
	if err == nil {
		t.Fatal("expected start error")
	}
	if err.Error() != "Expected start event" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPtyConnectErrorsWhenFirstEventIsNotStart(t *testing.T) {
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

	pty := NewPty(testCommandsConfig(server.URL, 0), "1.0.0")

	_, err := pty.Connect(context.Background(), 123, nil)
	if err == nil {
		t.Fatal("expected connect startup error")
	}
	if err.Error() != "Expected start event" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPtyConnectErrorsWhenStreamClosesBeforeFirstEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Connect" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pty := NewPty(testCommandsConfig(server.URL, 0), "1.0.0")

	_, err := pty.Connect(context.Background(), 123, nil)
	if err == nil {
		t.Fatal("expected connect startup error")
	}
	if err.Error() != "Expected start event" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return data
}
