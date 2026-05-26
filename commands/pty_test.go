package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
