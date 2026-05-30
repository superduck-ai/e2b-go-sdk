package e2b

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/api"
	"github.com/superduck-ai/e2b-go-sdk/envd"
	"github.com/superduck-ai/e2b-go-sdk/filesystem"
)

type testLogger struct {
	warns []string
}

func (l *testLogger) Debug(args ...interface{}) {}
func (l *testLogger) Info(args ...interface{})  {}
func (l *testLogger) Error(args ...interface{}) {}
func (l *testLogger) Warn(args ...interface{}) {
	if len(args) == 0 {
		l.warns = append(l.warns, "")
		return
	}
	if message, ok := args[0].(string); ok {
		l.warns = append(l.warns, message)
		return
	}
	l.warns = append(l.warns, "unexpected warn payload")
}

func testSandboxApiOpts(serverURL string) SandboxApiOpts {
	return SandboxApiOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		apiUrl:           serverURL,
		Domain:           "e2b.app",
		RequestTimeoutMs: intPtr(1000),
		Headers:          map[string]string{},
	}
}

func testSandboxApiOptsPtr(serverURL string) *SandboxApiOpts {
	opts := testSandboxApiOpts(serverURL)
	return &opts
}

func writeProcessEnvelope(t *testing.T, buf *bytes.Buffer, flags byte, payload []byte) {
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

func TestUploadURLUsesUserAndSignatureExpiration(t *testing.T) {
	expiration := 3600
	sandbox := &Sandbox{
		envdVersion:      envd.EnvdDefaultUser,
		envdApiUrl:       "https://envd.example",
		envdDirectUrl:    "https://envd-direct.example",
		envdAccessToken:  "token",
		connectionConfig: &ConnectionConfig{},
	}

	got, err := sandbox.UploadUrl("/tmp/file.txt", &struct {
		UseSignatureExpiration *int
		User                   string
	}{
		User:                   "alice",
		UseSignatureExpiration: &expiration,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if value := mustQueryValue(t, got, "path"); value != "/tmp/file.txt" {
		t.Fatalf("expected path query to be /tmp/file.txt, got %q", value)
	}
	if value := mustQueryValue(t, got, "username"); value != "alice" {
		t.Fatalf("expected username query to be alice, got %q", value)
	}
	if value := mustQueryValue(t, got, "signature"); value == "" {
		t.Fatalf("expected signature query to be set")
	}
	if value := mustQueryValue(t, got, "signature_expiration"); value == "" {
		t.Fatalf("expected signature_expiration query to be set")
	}
}

func TestDownloadURLUsesDefaultUserForOldEnvd(t *testing.T) {
	sandbox := &Sandbox{
		envdVersion:      "0.3.0",
		envdApiUrl:       "https://envd.example",
		envdDirectUrl:    "https://envd-direct.example",
		connectionConfig: &ConnectionConfig{},
	}

	got, err := sandbox.DownloadUrl("/tmp/file.txt", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if value := mustQueryValue(t, got, "username"); value != defaultUsername {
		t.Fatalf("expected username query to default to %q, got %q", defaultUsername, value)
	}
}

func TestDownloadURLMatchesJsDirectUrlSerialization(t *testing.T) {
	sandbox := &Sandbox{
		SandboxID:     "sbx-test",
		SandboxDomain: "e2b.app",
		envdVersion:   "0.2.4",
		envdApiUrl:    "https://sandbox.e2b.app",
		envdDirectUrl: "https://49983-sbx-test.e2b.app",
		connectionConfig: &ConnectionConfig{
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	got, err := sandbox.DownloadUrl("/hello.txt", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "https://49983-sbx-test.e2b.app/files?username=user&path=%2Fhello.txt"
	if got != want {
		t.Fatalf("expected direct download URL %q, got %q", want, got)
	}
}

func TestFileURLPreservesJsQueryParameterOrder(t *testing.T) {
	sandbox := &Sandbox{
		envdApiUrl:    "https://envd.example",
		envdDirectUrl: "https://envd-direct.example",
	}

	got := sandbox.fileURL("/hello.txt", "user")
	want := "https://envd-direct.example/files?username=user&path=%2Fhello.txt"
	if got != want {
		t.Fatalf("expected file URL %q, got %q", want, got)
	}
}

func TestFileURLFallsBackToEnvdApiUrlWhenDirectUrlMissing(t *testing.T) {
	sandbox := &Sandbox{envdApiUrl: "https://envd.example"}

	got := sandbox.fileURL("/hello.txt", "user")
	want := "https://envd.example/files?username=user&path=%2Fhello.txt"
	if got != want {
		t.Fatalf("expected file URL fallback %q, got %q", want, got)
	}
}

func TestUploadURLRejectsSignatureExpirationWithoutSecureSandbox(t *testing.T) {
	expiration := 60
	sandbox := &Sandbox{
		envdVersion:      envd.EnvdDefaultUser,
		envdApiUrl:       "https://envd.example",
		envdDirectUrl:    "https://envd-direct.example",
		connectionConfig: &ConnectionConfig{},
	}

	_, err := sandbox.UploadUrl("/tmp/file.txt", &struct {
		UseSignatureExpiration *int
		User                   string
	}{
		UseSignatureExpiration: &expiration,
	})
	if err == nil {
		t.Fatal("expected error when signature expiration is requested without secure sandbox")
	}
	expected := "Signature expiration can be used only when sandbox is created as secured."
	if err.Error() != expected {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestDownloadURLRejectsSignatureExpirationWithoutSecureSandbox(t *testing.T) {
	expiration := 60
	sandbox := &Sandbox{
		envdVersion:      envd.EnvdDefaultUser,
		envdApiUrl:       "https://envd.example",
		envdDirectUrl:    "https://envd-direct.example",
		connectionConfig: &ConnectionConfig{},
	}

	_, err := sandbox.DownloadUrl("/tmp/file.txt", &struct {
		UseSignatureExpiration *int
		User                   string
	}{
		UseSignatureExpiration: &expiration,
	})
	if err == nil {
		t.Fatal("expected error when signature expiration is requested without secure sandbox")
	}
	expected := "Signature expiration can be used only when sandbox is created as secured."
	if err.Error() != expected {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestListSandboxSnapshotsUsesSnapshotsEndpoint(t *testing.T) {
	var gotPath string
	var gotSandboxID string
	var gotLimit string
	var gotNextToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSandboxID = r.URL.Query().Get("sandboxID")
		gotLimit = r.URL.Query().Get("limit")
		gotNextToken = r.URL.Query().Get("nextToken")
		w.Header().Set("x-next-token", "next-page")
		if err := json.NewEncoder(w).Encode([]api.SnapshotInfo{{SnapshotID: "snap-1", Names: []string{"team/snap-1:latest"}}}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sandbox-123",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	paginator := sandbox.ListSnapshots(&struct {
		SandboxApiOpts
		Limit     int
		NextToken string
	}{
		Limit:     10,
		NextToken: "cursor-1",
	})
	items, err := paginator.NextItems()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/snapshots" {
		t.Fatalf("expected request path /snapshots, got %q", gotPath)
	}
	if gotSandboxID != "sandbox-123" {
		t.Fatalf("expected sandboxID query sandbox-123, got %q", gotSandboxID)
	}
	if gotLimit != "10" {
		t.Fatalf("expected limit query 10, got %q", gotLimit)
	}
	if gotNextToken != "cursor-1" {
		t.Fatalf("expected nextToken query cursor-1, got %q", gotNextToken)
	}
	if len(items) != 1 || items[0].SnapshotID != "snap-1" {
		t.Fatalf("unexpected snapshot items: %#v", items)
	}
	if paginator.NextToken != "next-page" {
		t.Fatalf("expected paginator next token next-page, got %q", paginator.NextToken)
	}
}

func TestCreateSandboxHonorsCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Create(ctx, "base", &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context error, got %T %v", err, err)
	}
}

func TestCreateSandboxHonorsPreCanceledSignalContext(t *testing.T) {
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested <- struct{}{}
		t.Fatal("expected pre-canceled signal to prevent create request from being sent")
	}))
	defer server.Close()

	signal, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Create(context.Background(), "base", &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Signal:           signal,
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected pre-canceled signal error, got %T %v", err, err)
	}

	select {
	case <-requested:
		t.Fatal("unexpected create request despite pre-canceled signal")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestKillSandboxHonorsCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Kill(ctx, "sbx-1", &SandboxApiOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		apiUrl:           server.URL,
		Domain:           "e2b.app",
		RequestTimeoutMs: intPtr(1000),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context error, got %T %v", err, err)
	}
}

func TestKillSandboxHonorsPreCanceledSignalContext(t *testing.T) {
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested <- struct{}{}
		t.Fatal("expected pre-canceled signal to prevent kill request from being sent")
	}))
	defer server.Close()

	signal, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Kill(context.Background(), "sbx-1", &SandboxApiOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		apiUrl:           server.URL,
		Domain:           "e2b.app",
		RequestTimeoutMs: intPtr(1000),
		Signal:           signal,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected pre-canceled signal error, got %T %v", err, err)
	}

	select {
	case <-requested:
		t.Fatal("unexpected kill request despite pre-canceled signal")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestUpdateNetworkHonorsPreCanceledSignalContext(t *testing.T) {
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested <- struct{}{}
		t.Fatal("expected pre-canceled signal to prevent update-network request from being sent")
	}))
	defer server.Close()

	signal, cancel := context.WithCancel(context.Background())
	cancel()

	err := UpdateNetwork(context.Background(), "sbx-1", SandboxNetworkUpdate{}, &SandboxApiOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		apiUrl:           server.URL,
		Domain:           "e2b.app",
		RequestTimeoutMs: intPtr(1000),
		Signal:           signal,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected pre-canceled signal error, got %T %v", err, err)
	}

	select {
	case <-requested:
		t.Fatal("unexpected update-network request despite pre-canceled signal")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSandboxPaginatorNextItemsContextHonorsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	paginator := List(&SandboxListOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		apiUrl:           server.URL,
		Domain:           "e2b.app",
		RequestTimeoutMs: intPtr(1000),
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := paginator.NextItemsContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context error, got %T %v", err, err)
	}
}

func TestSandboxPaginatorNextItemsContextHonorsPerCallOverrides(t *testing.T) {
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	paginator := List(&SandboxListOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		apiUrl:           server.URL,
		Domain:           "e2b.app",
		RequestTimeoutMs: intPtr(1000),
	})

	_, err := paginator.NextItemsContext(context.Background(), &SandboxApiOpts{
		ApiKey: "e2b_1111111111111111111111111111111111111111",
	})
	if err != nil {
		t.Fatalf("expected paginator override call to succeed, got %v", err)
	}
	if gotAPIKey != "e2b_1111111111111111111111111111111111111111" {
		t.Fatalf("expected per-call paginator API key override, got %q", gotAPIKey)
	}
}

func TestSandboxAPIsHonorInFlightCancellation(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		requestStarted := make(chan string, 1)
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestStarted <- r.URL.Path
			<-release
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)

		go func() {
			_, err := Create(ctx, "base", &SandboxOpts{
				ConnectionOpts: ConnectionOpts{
					ApiKey:           "e2b_0000000000000000000000000000000000000000",
					ApiUrl:           server.URL,
					Domain:           "e2b.app",
					RequestTimeoutMs: intPtr(1000),
				},
			})
			done <- err
		}()

		select {
		case path := <-requestStarted:
			if path != "/sandboxes" {
				t.Fatalf("expected /sandboxes request, got %s", path)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for create request to start")
		}

		cancel()

		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected canceled context error, got %T %v", err, err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for create cancellation")
		}

		close(release)
	})

	t.Run("kill", func(t *testing.T) {
		requestStarted := make(chan string, 1)
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestStarted <- r.URL.Path
			<-release
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)

		go func() {
			_, err := Kill(ctx, "sbx-1", &SandboxApiOpts{
				ApiKey:           "e2b_0000000000000000000000000000000000000000",
				apiUrl:           server.URL,
				Domain:           "e2b.app",
				RequestTimeoutMs: intPtr(1000),
			})
			done <- err
		}()

		select {
		case path := <-requestStarted:
			if path != "/sandboxes/sbx-1" {
				t.Fatalf("expected /sandboxes/sbx-1 request, got %s", path)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for kill request to start")
		}

		cancel()

		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected canceled context error, got %T %v", err, err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for kill cancellation")
		}

		close(release)
	})
}

func TestSandboxApisHonorSignalContext(t *testing.T) {
	runSignalCancellation := func(t *testing.T, invoke func(signal context.Context, apiURL string) error) {
		t.Helper()

		requestStarted := make(chan struct{}, 1)
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestStarted <- struct{}{}
			<-release
		}))
		defer server.Close()

		signal, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)

		go func() {
			done <- invoke(signal, server.URL)
		}()

		select {
		case <-requestStarted:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for request to start")
		}

		cancel()

		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected signal context cancellation, got %T %v", err, err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for signal cancellation")
		}

		close(release)
	}

	t.Run("create", func(t *testing.T) {
		runSignalCancellation(t, func(signal context.Context, apiURL string) error {
			_, err := Create(context.Background(), "base", &SandboxOpts{
				ConnectionOpts: ConnectionOpts{
					ApiKey:           "e2b_0000000000000000000000000000000000000000",
					ApiUrl:           apiURL,
					Domain:           "e2b.app",
					RequestTimeoutMs: intPtr(1000),
					Signal:           signal,
				},
			})
			return err
		})
	})

	t.Run("kill", func(t *testing.T) {
		runSignalCancellation(t, func(signal context.Context, apiURL string) error {
			_, err := Kill(context.Background(), "sbx-1", &SandboxApiOpts{
				ApiKey:           "e2b_0000000000000000000000000000000000000000",
				apiUrl:           apiURL,
				Domain:           "e2b.app",
				RequestTimeoutMs: intPtr(1000),
				Signal:           signal,
			})
			return err
		})
	})

	t.Run("update_network", func(t *testing.T) {
		runSignalCancellation(t, func(signal context.Context, apiURL string) error {
			return UpdateNetwork(context.Background(), "sbx-1", SandboxNetworkUpdate{}, &SandboxApiOpts{
				ApiKey:           "e2b_0000000000000000000000000000000000000000",
				apiUrl:           apiURL,
				Domain:           "e2b.app",
				RequestTimeoutMs: intPtr(1000),
				Signal:           signal,
			})
		})
	})

	t.Run("instance_kill", func(t *testing.T) {
		runSignalCancellation(t, func(signal context.Context, apiURL string) error {
			sandbox := &Sandbox{
				SandboxID: "sbx-1",
				connectionConfig: &ConnectionConfig{
					ApiKey:           "e2b_0000000000000000000000000000000000000000",
					ApiUrl:           apiURL,
					Domain:           "e2b.app",
					RequestTimeoutMs: 1000,
					Headers:          map[string]string{},
				},
			}
			return sandbox.Kill(context.Background(), &struct {
				RequestTimeoutMs *int
				Signal           context.Context
			}{RequestTimeoutMs: intPtr(1000), Signal: signal})
		})
	})

	t.Run("instance_update_network", func(t *testing.T) {
		runSignalCancellation(t, func(signal context.Context, apiURL string) error {
			sandbox := &Sandbox{
				SandboxID: "sbx-1",
				connectionConfig: &ConnectionConfig{
					ApiKey:           "e2b_0000000000000000000000000000000000000000",
					ApiUrl:           apiURL,
					Domain:           "e2b.app",
					RequestTimeoutMs: 1000,
					Headers:          map[string]string{},
				},
			}
			return sandbox.UpdateNetwork(context.Background(), SandboxNetworkUpdate{}, &struct {
				RequestTimeoutMs *int
				Signal           context.Context
			}{RequestTimeoutMs: intPtr(1000), Signal: signal})
		})
	})
}

func TestListSandboxSnapshotsIgnoresOverriddenSandboxID(t *testing.T) {
	var gotSandboxID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSandboxID = r.URL.Query().Get("sandboxID")
		if err := json.NewEncoder(w).Encode([]api.SnapshotInfo{{SnapshotID: "snap-1"}}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sandbox-123",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	paginator := sandbox.ListSnapshots(&struct {
		SandboxApiOpts
		Limit     int
		NextToken string
	}{})
	if _, err := paginator.NextItems(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotSandboxID != "sandbox-123" {
		t.Fatalf("expected sandboxID query sandbox-123, got %q", gotSandboxID)
	}
}

func TestCreateSandboxSnapshotPreservesNamesAndSendsName(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandboxes/sbx-1/snapshots" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SnapshotInfo{
			SnapshotID: "snap-1",
			Names:      []string{"team/snap-1:latest"},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	info, err := sandbox.CreateSnapshot(context.Background(), &CreateSnapshotOpts{Name: "named-snapshot"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil || info.SnapshotID != "snap-1" {
		t.Fatalf("unexpected snapshot info: %#v", info)
	}
	if len(info.Names) != 1 || info.Names[0] != "team/snap-1:latest" {
		t.Fatalf("expected snapshot names to be preserved, got %#v", info.Names)
	}
	if gotBody["name"] != "named-snapshot" {
		t.Fatalf("expected snapshot name request body, got %#v", gotBody)
	}
}

func TestSandboxApiCreateSnapshotPreservesNamesAndSendsName(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandboxes/sbx-1/snapshots" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SnapshotInfo{
			SnapshotID: "snap-1",
			Names:      []string{"team/snap-1:latest"},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	opts := &CreateSnapshotOpts{
		SandboxApiOpts: *testSandboxApiOptsPtr(server.URL),
		Name:           "named-snapshot",
	}
	info, err := apiClient.CreateSnapshot(context.Background(), "sbx-1", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil || info.SnapshotID != "snap-1" {
		t.Fatalf("unexpected snapshot info: %#v", info)
	}
	if len(info.Names) != 1 || info.Names[0] != "team/snap-1:latest" {
		t.Fatalf("expected snapshot names to be preserved, got %#v", info.Names)
	}
	if gotBody["name"] != "named-snapshot" {
		t.Fatalf("expected snapshot name request body, got %#v", gotBody)
	}
}

func TestSandboxApiCreateSnapshotErrorsWhenResponseDataIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	opts := &CreateSnapshotOpts{SandboxApiOpts: *testSandboxApiOptsPtr(server.URL)}
	_, err := apiClient.CreateSnapshot(context.Background(), "sbx-1", opts)
	if err == nil || err.Error() != "Response data is missing" {
		t.Fatalf("expected missing response data error, got %v", err)
	}
}

func TestSandboxApiListSnapshotsIgnoresNames(t *testing.T) {
	var gotSandboxID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/snapshots" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		gotSandboxID = r.URL.Query().Get("sandboxID")
		w.Header().Set("x-next-token", "next-page")
		if err := json.NewEncoder(w).Encode([]api.SnapshotInfo{
			{SnapshotID: "snap-1", Names: []string{"team/snap-1:latest"}},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	paginator := apiClient.ListSnapshots(&SnapshotListOpts{
		ApiKey:           testSandboxApiOpts(server.URL).ApiKey,
		apiUrl:           testSandboxApiOpts(server.URL).apiUrl,
		Domain:           testSandboxApiOpts(server.URL).Domain,
		RequestTimeoutMs: testSandboxApiOpts(server.URL).RequestTimeoutMs,
		Headers:          testSandboxApiOpts(server.URL).Headers,
		SandboxID:        "sbx-1",
	})

	items, err := paginator.NextItems()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSandboxID != "sbx-1" {
		t.Fatalf("expected sandboxID query sbx-1, got %q", gotSandboxID)
	}
	if len(items) != 1 || items[0].SnapshotID != "snap-1" {
		t.Fatalf("unexpected snapshot items: %#v", items)
	}
	if paginator.NextToken != "next-page" {
		t.Fatalf("expected paginator next token next-page, got %q", paginator.NextToken)
	}
}

func TestListSandboxesUsesQueryFilters(t *testing.T) {
	var gotPath string
	var gotMetadata string
	var gotStates []string
	var gotLimit string
	var gotNextToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMetadata = r.URL.Query().Get("metadata")
		gotStates = r.URL.Query()["state"]
		gotLimit = r.URL.Query().Get("limit")
		gotNextToken = r.URL.Query().Get("nextToken")
		w.Header().Set("x-next-token", "next-sandboxes")
		if err := json.NewEncoder(w).Encode([]api.SandboxResponse{{SandboxID: "sbx-1"}}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	paginator := List(&SandboxListOpts{
		ApiKey:           testSandboxApiOpts(server.URL).ApiKey,
		apiUrl:           testSandboxApiOpts(server.URL).apiUrl,
		Domain:           testSandboxApiOpts(server.URL).Domain,
		RequestTimeoutMs: testSandboxApiOpts(server.URL).RequestTimeoutMs,
		Headers:          testSandboxApiOpts(server.URL).Headers,
		Query: &struct {
			Metadata map[string]string
			State    []SandboxState
		}{
			Metadata: map[string]string{"team/id": "alpha beta"},
			State:    []SandboxState{sandboxStateRunning, sandboxStatePaused},
		},
		Limit:     25,
		NextToken: "cursor-2",
	})

	items, err := paginator.NextItems()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/v2/sandboxes" {
		t.Fatalf("expected request path /v2/sandboxes, got %q", gotPath)
	}
	if gotMetadata != "team%2Fid=alpha+beta" {
		t.Fatalf("expected encoded metadata, got %q", gotMetadata)
	}
	if len(gotStates) != 2 || gotStates[0] != string(sandboxStateRunning) || gotStates[1] != string(sandboxStatePaused) {
		t.Fatalf("unexpected state filters: %#v", gotStates)
	}
	if gotLimit != "25" {
		t.Fatalf("expected limit query 25, got %q", gotLimit)
	}
	if gotNextToken != "cursor-2" {
		t.Fatalf("expected nextToken query cursor-2, got %q", gotNextToken)
	}
	if len(items) != 1 || items[0].SandboxID != "sbx-1" {
		t.Fatalf("unexpected sandboxes: %#v", items)
	}
	if paginator.NextToken != "next-sandboxes" {
		t.Fatalf("expected paginator next token next-sandboxes, got %q", paginator.NextToken)
	}
}

func TestListSandboxesDoesNotExposeRawNameWithoutAlias(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode([]api.SandboxResponse{{
			SandboxID: "sbx-1",
			Name:      "raw-api-name",
		}}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	paginator := List(&SandboxListOpts{
		ApiKey:           testSandboxApiOpts(server.URL).ApiKey,
		apiUrl:           testSandboxApiOpts(server.URL).apiUrl,
		Domain:           testSandboxApiOpts(server.URL).Domain,
		RequestTimeoutMs: testSandboxApiOpts(server.URL).RequestTimeoutMs,
		Headers:          testSandboxApiOpts(server.URL).Headers,
	})

	items, err := paginator.NextItems()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one sandbox, got %#v", items)
	}
	if items[0].Name != "" {
		t.Fatalf("expected raw API name to stay hidden without alias, got %q", items[0].Name)
	}
}

func TestSandboxResponseToInfoPreservesNetworkMaskRequestHost(t *testing.T) {
	info := sandboxResponseToInfo(&api.SandboxResponse{
		SandboxID: "sbx-1",
		Network: &api.NetworkOpts{
			AllowPublicTraffic: boolRef(true),
			MaskRequestHost:    "${PORT}-custom.e2b.app",
		},
	})

	if info.Network == nil {
		t.Fatal("expected network info")
	}
	if info.Network.AllowPublicTraffic == nil || !*info.Network.AllowPublicTraffic {
		t.Fatalf("expected allowPublicTraffic to be preserved, got %#v", info.Network.AllowPublicTraffic)
	}
	if info.Network.MaskRequestHost != "${PORT}-custom.e2b.app" {
		t.Fatalf("expected maskRequestHost to be preserved, got %q", info.Network.MaskRequestHost)
	}
}

func TestSandboxResponseToInfoLeavesAllowPublicTrafficNilWhenOmitted(t *testing.T) {
	info := sandboxResponseToInfo(&api.SandboxResponse{
		SandboxID: "sbx-1",
		Network: &api.NetworkOpts{
			MaskRequestHost: "${PORT}-custom.e2b.app",
		},
	})

	if info.Network == nil {
		t.Fatal("expected network info")
	}
	if info.Network.AllowPublicTraffic != nil {
		t.Fatalf("expected allowPublicTraffic to stay nil when omitted, got %#v", info.Network.AllowPublicTraffic)
	}
}

func TestSandboxResponseToInfoUsesInfoOnlyNetworkType(t *testing.T) {
	info := sandboxResponseToInfo(&api.SandboxResponse{SandboxID: "sbx-1"})
	if info.Network != nil {
		t.Fatalf("expected nil network info, got %#v", info.Network)
	}

	infoType := reflect.TypeOf(SandboxInfo{})
	networkField, ok := infoType.FieldByName("Network")
	if !ok {
		t.Fatal("expected SandboxInfo.Network field")
	}
	if networkField.Type != reflect.TypeOf(&SandboxNetworkInfo{}) {
		t.Fatalf("expected SandboxInfo.Network type *SandboxNetworkInfo, got %v", networkField.Type)
	}
}

func TestSandboxResponseToInfoPreservesNetworkRules(t *testing.T) {
	info := sandboxResponseToInfo(&api.SandboxResponse{
		SandboxID: "sbx-1",
		Network: &api.NetworkOpts{
			Rules: map[string][]api.NetworkRule{
				"httpbin.e2b.team": {
					{
						Transform: &api.NetworkTransform{
							Headers: map[string]string{"X-Test": "value"},
						},
					},
					{},
				},
			},
		},
	})

	if info.Network == nil {
		t.Fatal("expected network info")
	}
	got := info.Network.Rules["httpbin.e2b.team"]
	if len(got) != 2 {
		t.Fatalf("expected two network rules, got %#v", info.Network.Rules)
	}
	if got[0].Transform == nil || got[0].Transform.Headers["X-Test"] != "value" {
		t.Fatalf("expected first rule transform headers to be preserved, got %#v", got[0].Transform)
	}
	if got[1].Transform != nil {
		t.Fatalf("expected second rule transform to stay nil, got %#v", got[1].Transform)
	}
}

func TestSandboxResponseToInfoIncludesAllowInternetAccess(t *testing.T) {
	allowInternetAccess := false

	info := sandboxResponseToInfo(&api.SandboxResponse{
		SandboxID:           "sbx-1",
		AllowInternetAccess: &allowInternetAccess,
	})

	if info.AllowInternetAccess == nil {
		t.Fatal("expected allowInternetAccess to be preserved")
	}
	if *info.AllowInternetAccess != allowInternetAccess {
		t.Fatalf("expected allowInternetAccess to be %t, got %t", allowInternetAccess, *info.AllowInternetAccess)
	}
}

func TestSandboxResponseToInfoFallsBackToAliasForName(t *testing.T) {
	info := sandboxResponseToInfo(&api.SandboxResponse{
		SandboxID: "sbx-1",
		Alias:     "template-alias",
	})

	if info.Name != "template-alias" {
		t.Fatalf("expected sandbox name to fall back to alias, got %q", info.Name)
	}
}

func TestSandboxResponseToInfoDoesNotExposeRawNameWithoutAlias(t *testing.T) {
	info := sandboxResponseToInfo(&api.SandboxResponse{
		SandboxID: "sbx-1",
		Name:      "raw-api-name",
	})

	if info.Name != "" {
		t.Fatalf("expected raw API name to stay hidden without alias, got %q", info.Name)
	}
}

func TestSandboxResponseToInfoDefaultsMetadataAndVolumeMounts(t *testing.T) {
	info := sandboxResponseToInfo(&api.SandboxResponse{
		SandboxID: "sbx-1",
	})

	if info.Metadata == nil {
		t.Fatal("expected metadata to default to an empty map")
	}
	if len(info.Metadata) != 0 {
		t.Fatalf("expected empty metadata map, got %#v", info.Metadata)
	}
	if info.VolumeMounts == nil {
		t.Fatal("expected volumeMounts to default to an empty slice")
	}
	if len(info.VolumeMounts) != 0 {
		t.Fatalf("expected empty volumeMounts slice, got %#v", info.VolumeMounts)
	}
}

func TestSandboxResponseToInfoMapsVolumeMountNameAndPath(t *testing.T) {
	info := sandboxResponseToInfo(&api.SandboxResponse{
		SandboxID: "sbx-1",
		VolumeMounts: []api.VolumeMount{
			{Name: "data", Path: "/mnt/data"},
		},
	})

	if len(info.VolumeMounts) != 1 {
		t.Fatalf("expected one volume mount, got %d", len(info.VolumeMounts))
	}

	mount := info.VolumeMounts[0]
	if mount.Name != "data" {
		t.Fatalf("expected volume mount name data, got %q", mount.Name)
	}
	if mount.Path != "/mnt/data" {
		t.Fatalf("expected volume mount path /mnt/data, got %q", mount.Path)
	}
}

func TestSandboxResponseToInfoMapsLegacyVolumeMountFieldsToNameAndPath(t *testing.T) {
	info := sandboxResponseToInfo(&api.SandboxResponse{
		SandboxID: "sbx-1",
		VolumeMounts: []api.VolumeMount{
			{VolumeID: "data", MountPath: "/mnt/data"},
		},
	})

	if len(info.VolumeMounts) != 1 {
		t.Fatalf("expected one volume mount, got %d", len(info.VolumeMounts))
	}

	mount := info.VolumeMounts[0]
	if mount.Name != "data" {
		t.Fatalf("expected legacy volumeID to map to name data, got %q", mount.Name)
	}
	if mount.Path != "/mnt/data" {
		t.Fatalf("expected legacy mountPath to map to path /mnt/data, got %q", mount.Path)
	}
}

func TestSandboxApiCreateSandboxUsesNameAndPathVolumeMounts(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandboxes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
		VolumeMounts: map[string]any{
			"/mnt/data": "data",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rawMounts, ok := gotBody["volumeMounts"].([]any)
	if !ok || len(rawMounts) != 1 {
		t.Fatalf("expected one volumeMount in request body, got %#v", gotBody["volumeMounts"])
	}

	mount, ok := rawMounts[0].(map[string]any)
	if !ok {
		t.Fatalf("expected volume mount object, got %#v", rawMounts[0])
	}
	if mount["name"] != "data" {
		t.Fatalf("expected volume mount name data, got %#v", mount["name"])
	}
	if mount["path"] != "/mnt/data" {
		t.Fatalf("expected volume mount path /mnt/data, got %#v", mount["path"])
	}
	if _, ok := mount["volumeID"]; ok {
		t.Fatalf("did not expect legacy volumeID field in request payload: %#v", mount)
	}
	if _, ok := mount["mountPath"]; ok {
		t.Fatalf("did not expect legacy mountPath field in request payload: %#v", mount)
	}
}

func TestSandboxApiCreateSandboxAcceptsVolumeLikeObjectsInVolumeMountMap(t *testing.T) {
	type volumeLike struct {
		Name string
	}

	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
		VolumeMounts: map[string]any{
			"/mnt/data": &volumeLike{Name: "data"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rawMounts, ok := gotBody["volumeMounts"].([]any)
	if !ok || len(rawMounts) != 1 {
		t.Fatalf("expected one volumeMount in request body, got %#v", gotBody["volumeMounts"])
	}

	mount, ok := rawMounts[0].(map[string]any)
	if !ok {
		t.Fatalf("expected volume mount object, got %#v", rawMounts[0])
	}
	if mount["name"] != "data" || mount["path"] != "/mnt/data" {
		t.Fatalf("expected volume-like object to map to name/path payload, got %#v", mount)
	}
}

func TestSandboxApiCreateSandboxPreservesNetworkOptions(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
		Network: &SandboxNetworkOpts{
			AllowOut:           []string{"1.1.1.1"},
			DenyOut:            []string{ALL_TRAFFIC},
			AllowPublicTraffic: boolRef(false),
			MaskRequestHost:    "custom-host.example.com:${PORT}",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	network, ok := gotBody["network"].(map[string]any)
	if !ok {
		t.Fatalf("expected network object in request body, got %#v", gotBody["network"])
	}
	if !reflect.DeepEqual(network["allowOut"], []any{"1.1.1.1"}) {
		t.Fatalf("unexpected allowOut payload: %#v", network["allowOut"])
	}
	if !reflect.DeepEqual(network["denyOut"], []any{ALL_TRAFFIC}) {
		t.Fatalf("unexpected denyOut payload: %#v", network["denyOut"])
	}
	if value, ok := network["allowPublicTraffic"].(bool); !ok || value {
		t.Fatalf("expected allowPublicTraffic=false to be preserved, got %#v", network["allowPublicTraffic"])
	}
	if network["maskRequestHost"] != "custom-host.example.com:${PORT}" {
		t.Fatalf("unexpected maskRequestHost payload: %#v", network["maskRequestHost"])
	}
}

func TestSandboxApiCreateSandboxOmitsAllowPublicTrafficWhenUnset(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
		Network: &SandboxNetworkOpts{
			AllowOut: []string{"1.1.1.1"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	network, ok := gotBody["network"].(map[string]any)
	if !ok {
		t.Fatalf("expected network object in request body, got %#v", gotBody["network"])
	}
	if _, ok := network["allowPublicTraffic"]; ok {
		t.Fatalf("did not expect allowPublicTraffic when unset, got %#v", network["allowPublicTraffic"])
	}
}

func TestSandboxApiCreateSandboxPreservesNetworkRules(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
		Network: &SandboxNetworkOpts{
			Rules: SandboxNetworkRules{
				"httpbin.e2b.team": {
					{
						Transform: &SandboxNetworkTransform{
							Headers: map[string]string{"X-Test": "value"},
						},
					},
					{},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	network, ok := gotBody["network"].(map[string]any)
	if !ok {
		t.Fatalf("expected network object in request body, got %#v", gotBody["network"])
	}
	rawRules, ok := network["rules"].(map[string]any)
	if !ok {
		t.Fatalf("expected rules object in request body, got %#v", network["rules"])
	}
	hostRules, ok := rawRules["httpbin.e2b.team"].([]any)
	if !ok || len(hostRules) != 2 {
		t.Fatalf("expected two rules for host, got %#v", rawRules["httpbin.e2b.team"])
	}
	firstRule, ok := hostRules[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first rule object, got %#v", hostRules[0])
	}
	transform, ok := firstRule["transform"].(map[string]any)
	if !ok {
		t.Fatalf("expected transform object in first rule, got %#v", firstRule["transform"])
	}
	headers, ok := transform["headers"].(map[string]any)
	if !ok || headers["X-Test"] != "value" {
		t.Fatalf("expected transform headers to be preserved, got %#v", transform["headers"])
	}
	secondRule, ok := hostRules[1].(map[string]any)
	if !ok {
		t.Fatalf("expected second rule object, got %#v", hostRules[1])
	}
	if len(secondRule) != 0 {
		t.Fatalf("expected second rule to stay empty, got %#v", secondRule)
	}
}

func TestSandboxApiCreateSandboxPreservesExplicitEmptyNetworkFields(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
		Network: &SandboxNetworkOpts{
			AllowOut: []string{},
			DenyOut:  []string{},
			Rules:    SandboxNetworkRules{},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	network, ok := gotBody["network"].(map[string]any)
	if !ok {
		t.Fatalf("expected network object in request body, got %#v", gotBody["network"])
	}
	if !reflect.DeepEqual(network["allowOut"], []any{}) {
		t.Fatalf("expected explicit empty allowOut array, got %#v", network["allowOut"])
	}
	if !reflect.DeepEqual(network["denyOut"], []any{}) {
		t.Fatalf("expected explicit empty denyOut array, got %#v", network["denyOut"])
	}
	if !reflect.DeepEqual(network["rules"], map[string]any{}) {
		t.Fatalf("expected explicit empty rules object, got %#v", network["rules"])
	}
}

func TestSandboxApiCreateSandboxDefaultsAllowInternetAccessToTrue(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	value, ok := gotBody["allow_internet_access"].(bool)
	if !ok {
		t.Fatalf("expected allow_internet_access bool in request body, got %#v", gotBody["allow_internet_access"])
	}
	if !value {
		t.Fatal("expected allowInternetAccess default to true")
	}
	autoPause, ok := gotBody["autoPause"].(bool)
	if !ok {
		t.Fatalf("expected autoPause bool in request body, got %#v", gotBody["autoPause"])
	}
	if autoPause {
		t.Fatal("expected default autoPause to be false")
	}
	autoResume, ok := gotBody["autoResume"].(map[string]any)
	if !ok {
		t.Fatalf("expected autoResume object in default request body, got %#v", gotBody["autoResume"])
	}
	enabled, ok := autoResume["enabled"].(bool)
	if !ok {
		t.Fatalf("expected autoResume.enabled bool, got %#v", autoResume["enabled"])
	}
	if enabled {
		t.Fatal("expected default autoResume.enabled to be false")
	}
}

func TestSandboxApiCreateSandboxPreservesExplicitZeroTimeout(t *testing.T) {
	var gotBody map[string]any
	zero := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template:  "base",
		TimeoutMs: &zero,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	timeout, ok := gotBody["timeout"].(float64)
	if !ok || timeout != 0 {
		t.Fatalf("expected timeout 0 in request body, got %#v", gotBody["timeout"])
	}
}

func TestSandboxApiCreateSandboxDefaultsSecureToTrue(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	value, ok := gotBody["secure"].(bool)
	if !ok {
		t.Fatalf("expected secure bool in request body, got %#v", gotBody["secure"])
	}
	if !value {
		t.Fatal("expected secure default to true")
	}
}

func TestSandboxApiCreateSandboxPreservesSecureFalse(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	secure := false
	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
		Secure:   &secure,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	value, ok := gotBody["secure"].(bool)
	if !ok {
		t.Fatalf("expected secure bool in request body, got %#v", gotBody["secure"])
	}
	if value {
		t.Fatal("expected secure false to be preserved")
	}
}

func TestSandboxApiCreateSandboxPreservesAllowInternetAccessFalse(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	allowInternetAccess := false
	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template:            "base",
		AllowInternetAccess: &allowInternetAccess,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	value, ok := gotBody["allow_internet_access"].(bool)
	if !ok {
		t.Fatalf("expected allow_internet_access bool in request body, got %#v", gotBody["allow_internet_access"])
	}
	if value {
		t.Fatal("expected allowInternetAccess false to be preserved")
	}
}

func TestSandboxApiCreateSandboxResolvesNetworkSelectors(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
		Network: &SandboxNetworkOpts{
			Rules: SandboxNetworkRules{
				"httpbin.e2b.team": {{}},
			},
			AllowOut: func(ctx SandboxNetworkSelectorContext) []string {
				if len(ctx.Rules["httpbin.e2b.team"]) != 1 {
					t.Fatalf("expected selector context rules to be preserved, got %#v", ctx.Rules)
				}
				return []string{"httpbin.e2b.team"}
			},
			DenyOut: func(ctx SandboxNetworkSelectorContext) []string {
				if ctx.AllTraffic != ALL_TRAFFIC {
					t.Fatalf("expected allTraffic sentinel %q, got %q", ALL_TRAFFIC, ctx.AllTraffic)
				}
				return []string{ctx.AllTraffic}
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	network, ok := gotBody["network"].(map[string]any)
	if !ok {
		t.Fatalf("expected network object in request body, got %#v", gotBody["network"])
	}
	if !reflect.DeepEqual(network["allowOut"], []any{"httpbin.e2b.team"}) {
		t.Fatalf("unexpected selector-resolved allowOut payload: %#v", network["allowOut"])
	}
	if !reflect.DeepEqual(network["denyOut"], []any{ALL_TRAFFIC}) {
		t.Fatalf("unexpected selector-resolved denyOut payload: %#v", network["denyOut"])
	}
}

func TestSandboxApiCreateSandboxRejectsInvalidNetworkSelectorType(t *testing.T) {
	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           "http://127.0.0.1:1",
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
		Network: &SandboxNetworkOpts{
			AllowOut: 123,
		},
	})
	var invalidErr *InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %T %v", err, err)
	}
}

func TestSandboxApiUpdateNetworkPreservesRulesAndAllowInternetAccess(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	allowInternetAccess := false
	apiClient := &sandboxApi{}
	err := apiClient.UpdateNetwork(context.Background(), "sbx-1", SandboxNetworkUpdate{
		AllowOut:            []string{"httpbin.e2b.team"},
		DenyOut:             []string{"8.8.8.8"},
		AllowInternetAccess: &allowInternetAccess,
		Rules: SandboxNetworkRules{
			"httpbin.e2b.team": {
				{
					Transform: &SandboxNetworkTransform{
						Headers: map[string]string{"X-Test": "value"},
					},
				},
			},
		},
	}, testSandboxApiOptsPtr(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Fatalf("expected PUT request, got %s", gotMethod)
	}
	if gotPath != "/sandboxes/sbx-1/network" {
		t.Fatalf("expected network update path, got %q", gotPath)
	}
	if !reflect.DeepEqual(gotBody["allowOut"], []any{"httpbin.e2b.team"}) {
		t.Fatalf("unexpected allowOut payload: %#v", gotBody["allowOut"])
	}
	if !reflect.DeepEqual(gotBody["denyOut"], []any{"8.8.8.8"}) {
		t.Fatalf("unexpected denyOut payload: %#v", gotBody["denyOut"])
	}
	if value, ok := gotBody["allow_internet_access"].(bool); !ok || value {
		t.Fatalf("expected allow_internet_access=false, got %#v", gotBody["allow_internet_access"])
	}
	rawRules, ok := gotBody["rules"].(map[string]any)
	if !ok {
		t.Fatalf("expected rules object in update body, got %#v", gotBody["rules"])
	}
	hostRules, ok := rawRules["httpbin.e2b.team"].([]any)
	if !ok || len(hostRules) != 1 {
		t.Fatalf("expected one rule for host, got %#v", rawRules["httpbin.e2b.team"])
	}
	rule, ok := hostRules[0].(map[string]any)
	if !ok {
		t.Fatalf("expected host rule object, got %#v", hostRules[0])
	}
	transform, ok := rule["transform"].(map[string]any)
	if !ok {
		t.Fatalf("expected transform object, got %#v", rule["transform"])
	}
	headers, ok := transform["headers"].(map[string]any)
	if !ok || headers["X-Test"] != "value" {
		t.Fatalf("expected transform headers to be preserved, got %#v", transform["headers"])
	}
}

func TestSandboxApiUpdateNetworkPreservesExplicitEmptyFields(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	err := apiClient.UpdateNetwork(context.Background(), "sbx-1", SandboxNetworkUpdate{
		AllowOut: []string{},
		DenyOut:  []string{},
		Rules:    SandboxNetworkRules{},
	}, testSandboxApiOptsPtr(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(gotBody["allowOut"], []any{}) {
		t.Fatalf("expected explicit empty allowOut array, got %#v", gotBody["allowOut"])
	}
	if !reflect.DeepEqual(gotBody["denyOut"], []any{}) {
		t.Fatalf("expected explicit empty denyOut array, got %#v", gotBody["denyOut"])
	}
	if !reflect.DeepEqual(gotBody["rules"], map[string]any{}) {
		t.Fatalf("expected explicit empty rules object, got %#v", gotBody["rules"])
	}
}

func TestSandboxApiUpdateNetworkResolvesSelectors(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	err := apiClient.UpdateNetwork(context.Background(), "sbx-1", SandboxNetworkUpdate{
		Rules: SandboxNetworkRules{
			"httpbin.e2b.team": {{}},
		},
		AllowOut: func(ctx SandboxNetworkSelectorContext) []string {
			return []string{"httpbin.e2b.team"}
		},
		DenyOut: SandboxNetworkSelectorFunc(func(ctx SandboxNetworkSelectorContext) []string {
			return []string{ctx.AllTraffic}
		}),
	}, testSandboxApiOptsPtr(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(gotBody["allowOut"], []any{"httpbin.e2b.team"}) {
		t.Fatalf("unexpected selector-resolved allowOut payload: %#v", gotBody["allowOut"])
	}
	if !reflect.DeepEqual(gotBody["denyOut"], []any{ALL_TRAFFIC}) {
		t.Fatalf("unexpected selector-resolved denyOut payload: %#v", gotBody["denyOut"])
	}
}

func TestSandboxApiUpdateNetworkSendsEmptyBodyForZeroValueUpdate(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	if err := apiClient.UpdateNetwork(context.Background(), "sbx-1", SandboxNetworkUpdate{}, testSandboxApiOptsPtr(server.URL)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotBody) != 0 {
		t.Fatalf("expected empty update body, got %#v", gotBody)
	}
}

func TestSandboxApiUpdateNetworkRejectsInvalidSelectorType(t *testing.T) {
	apiClient := &sandboxApi{}
	err := apiClient.UpdateNetwork(context.Background(), "sbx-1", SandboxNetworkUpdate{
		DenyOut: 123,
	}, testSandboxApiOptsPtr("http://127.0.0.1:1"))
	var invalidErr *InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %T %v", err, err)
	}
}

func TestSandboxApiCreateSandboxUsesAutoPauseAndAutoResumeForLifecycle(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
		Lifecycle: &SandboxLifecycle{
			OnTimeout:  "pause",
			AutoResume: boolRef(true),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	autoPause, ok := gotBody["autoPause"].(bool)
	if !ok {
		t.Fatalf("expected autoPause bool in request body, got %#v", gotBody["autoPause"])
	}
	if !autoPause {
		t.Fatal("expected autoPause true when lifecycle.onTimeout is pause")
	}

	autoResume, ok := gotBody["autoResume"].(map[string]any)
	if !ok {
		t.Fatalf("expected autoResume object in request body, got %#v", gotBody["autoResume"])
	}
	enabled, ok := autoResume["enabled"].(bool)
	if !ok {
		t.Fatalf("expected autoResume.enabled bool, got %#v", autoResume["enabled"])
	}
	if !enabled {
		t.Fatal("expected autoResume.enabled true when lifecycle.autoResume is true")
	}

	if _, ok := gotBody["lifecycle"]; ok {
		t.Fatalf("did not expect legacy lifecycle field in request body, got %#v", gotBody["lifecycle"])
	}
}

func TestBetaCreateSupportsDeprecatedAutoPause(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	_, err := BetaCreate(context.Background(), "base", &SandboxBetaCreateOpts{
		SandboxOpts: SandboxOpts{
			ConnectionOpts: ConnectionOpts{
				ApiKey:           "e2b_0000000000000000000000000000000000000000",
				ApiUrl:           server.URL,
				SandboxUrl:       server.URL,
				Domain:           "e2b.app",
				RequestTimeoutMs: intPtr(1000),
				Headers:          map[string]string{},
			},
		},
		AutoPause: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	autoPause, ok := gotBody["autoPause"].(bool)
	if !ok {
		t.Fatalf("expected autoPause bool in request body, got %#v", gotBody["autoPause"])
	}
	if !autoPause {
		t.Fatal("expected autoPause true when deprecated autoPause option is set")
	}

	autoResume, ok := gotBody["autoResume"].(map[string]any)
	if !ok {
		t.Fatalf("expected autoResume object in request body, got %#v", gotBody["autoResume"])
	}
	enabled, ok := autoResume["enabled"].(bool)
	if !ok {
		t.Fatalf("expected autoResume.enabled bool, got %#v", autoResume["enabled"])
	}
	if enabled {
		t.Fatal("expected autoResume.enabled false when deprecated autoPause is used")
	}
}

func TestBetaCreateLifecycleOverridesDeprecatedAutoPause(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	_, err := BetaCreate(context.Background(), "base", &SandboxBetaCreateOpts{
		SandboxOpts: SandboxOpts{
			ConnectionOpts: ConnectionOpts{
				ApiKey:           "e2b_0000000000000000000000000000000000000000",
				ApiUrl:           server.URL,
				SandboxUrl:       server.URL,
				Domain:           "e2b.app",
				RequestTimeoutMs: intPtr(1000),
				Headers:          map[string]string{},
			},
			Lifecycle: &SandboxLifecycle{
				OnTimeout: "kill",
			},
		},
		AutoPause: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	autoPause, ok := gotBody["autoPause"].(bool)
	if !ok {
		t.Fatalf("expected autoPause bool in request body, got %#v", gotBody["autoPause"])
	}
	if autoPause {
		t.Fatal("expected lifecycle to override deprecated autoPause and keep autoPause false")
	}
	autoResume, ok := gotBody["autoResume"].(map[string]any)
	if !ok {
		t.Fatalf("expected autoResume object in request body, got %#v", gotBody["autoResume"])
	}
	enabled, ok := autoResume["enabled"].(bool)
	if !ok {
		t.Fatalf("expected autoResume.enabled bool, got %#v", autoResume["enabled"])
	}
	if enabled {
		t.Fatal("expected lifecycle override to keep autoResume.enabled false")
	}
}

func TestSandboxApiCreateSandboxRejectsAutoResumeWithoutPauseLifecycle(t *testing.T) {
	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           "http://127.0.0.1:1",
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
		Lifecycle: &SandboxLifecycle{
			OnTimeout:  "kill",
			AutoResume: boolRef(true),
		},
	})

	var invalidErr *InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %T %v", err, err)
	}
	if invalidErr.Message != "autoResume can only be true when the resolved onTimeout is 'pause'." {
		t.Fatalf("unexpected error message: %q", invalidErr.Message)
	}
}

func TestSandboxApiCreateSandboxIncludesMcpConfig(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "mcp-gateway",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "mcp-gateway",
		Mcp: McpServer{
			"playwright": map[string]any{
				"command": "npx",
				"args":    []any{"@playwright/mcp@latest"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mcp, ok := gotBody["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("expected mcp object in request body, got %#v", gotBody["mcp"])
	}
	playwright, ok := mcp["playwright"].(map[string]any)
	if !ok {
		t.Fatalf("expected playwright MCP config, got %#v", mcp["playwright"])
	}
	if playwright["command"] != "npx" {
		t.Fatalf("expected mcp command npx, got %#v", playwright["command"])
	}
}

func TestCreateSandboxReturnsJsStyleMcpGatewayFailureMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sandboxes":
			if err := json.NewEncoder(w).Encode(api.SandboxResponse{
				SandboxID:   "sbx-1",
				TemplateID:  "mcp-gateway",
				EnvdVersion: "1.0.0",
			}); err != nil {
				t.Fatalf("failed to encode sandbox response: %v", err)
			}
		case "/process.Process/Start":
			w.WriteHeader(http.StatusOK)
			var stream bytes.Buffer
			writeProcessEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
			writeProcessEnvelope(t, &stream, 0x00, []byte(`{"data":{"stderr":"Z2F0ZXdheSBib29t"}}`))
			writeProcessEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":1,"error":"command failed"}}`))
			if _, err := w.Write(stream.Bytes()); err != nil {
				t.Fatalf("failed to write command stream: %v", err)
			}
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := Create(context.Background(), "mcp-gateway", &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			SandboxUrl:       server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Mcp: McpServer{
			"playwright": map[string]any{
				"command": "npx",
			},
		},
	})
	if err == nil {
		t.Fatal("expected MCP gateway startup failure")
	}
	expected := "Failed to start MCP gateway: gateway boom"
	if err.Error() != expected {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSandboxApiCreateSandboxRejectsOldEnvdAndDeletesSandbox(t *testing.T) {
	var requests []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sandboxes":
			if err := json.NewEncoder(w).Encode(api.SandboxResponse{
				SandboxID:   "sbx-old",
				TemplateID:  "base",
				EnvdVersion: "0.0.9",
			}); err != nil {
				t.Fatalf("failed to encode response: %v", err)
			}
		case r.Method == http.MethodDelete && r.URL.Path == "/sandboxes/sbx-old":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
	})
	if err == nil {
		t.Fatal("expected old envd template to fail")
	}
	var templateErr *TemplateError
	if !errors.As(err, &templateErr) {
		t.Fatalf("expected TemplateError, got %T %v", err, err)
	}
	expectedMessage := "You need to update the template to use the new SDK. You can do this by running `e2b template build` in the directory with the template."
	if err.Error() != expectedMessage {
		t.Fatalf("unexpected template error message: %v", err)
	}
	if len(requests) != 2 || requests[0] != "POST /sandboxes" || requests[1] != "DELETE /sandboxes/sbx-old" {
		t.Fatalf("expected create then delete requests, got %#v", requests)
	}
}

func TestCreateSandboxRejectsOldEnvdAndDeletesSandbox(t *testing.T) {
	var requests []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sandboxes":
			if err := json.NewEncoder(w).Encode(api.SandboxResponse{
				SandboxID:   "sbx-old",
				TemplateID:  "base",
				EnvdVersion: "0.0.9",
			}); err != nil {
				t.Fatalf("failed to encode response: %v", err)
			}
		case r.Method == http.MethodDelete && r.URL.Path == "/sandboxes/sbx-old":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := Create(context.Background(), "base", &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
	})
	if err == nil {
		t.Fatal("expected old envd template to fail")
	}
	var templateErr *TemplateError
	if !errors.As(err, &templateErr) {
		t.Fatalf("expected TemplateError, got %T %v", err, err)
	}
	expectedMessage := "You need to update the template to use the new SDK. You can do this by running `e2b template build` in the directory with the template."
	if err.Error() != expectedMessage {
		t.Fatalf("unexpected template error message: %v", err)
	}
	if len(requests) != 2 || requests[0] != "POST /sandboxes" || requests[1] != "DELETE /sandboxes/sbx-old" {
		t.Fatalf("expected create then delete requests, got %#v", requests)
	}
}

func TestSandboxApiCreateSandboxErrorsWhenResponseDataIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
	})
	if err == nil || err.Error() != "Response data is missing" {
		t.Fatalf("expected missing response data error, got %v", err)
	}
}

func TestCreateSandboxErrorsWhenResponseDataIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := Create(context.Background(), "base", &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
	})
	if err == nil || err.Error() != "Response data is missing" {
		t.Fatalf("expected missing response data error, got %v", err)
	}
}

func TestCreateSandboxSurfacesControlPlaneErrorWithoutWrapper(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"bad request"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	_, err := Create(context.Background(), "base", &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
	})
	if err == nil || err.Error() != "400: bad request" {
		t.Fatalf("expected unwrapped control-plane error, got %v", err)
	}
}

func TestCreateSandboxDoesNotCallEnvdHealthOrInit(t *testing.T) {
	var gotBody map[string]any
	zero := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sandboxes":
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("failed to decode request body: %v", err)
			}
			if err := json.NewEncoder(w).Encode(api.SandboxResponse{
				SandboxID:   "sbx-1",
				TemplateID:  "base",
				EnvdVersion: "1.0.0",
			}); err != nil {
				t.Fatalf("failed to encode sandbox response: %v", err)
			}
		case "/health", "/init":
			t.Fatalf("did not expect envd bootstrap request: %s", r.URL.Path)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	sandbox, err := Create(context.Background(), "base", &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			SandboxUrl:       server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		TimeoutMs: &zero,
	})
	if err != nil {
		t.Fatalf("expected create sandbox to succeed without envd bootstrap, got %v", err)
	}
	if sandbox == nil || sandbox.SandboxID != "sbx-1" {
		t.Fatalf("unexpected sandbox: %#v", sandbox)
	}
	timeout, ok := gotBody["timeout"].(float64)
	if !ok || timeout != 0 {
		t.Fatalf("expected timeout 0 in request body, got %#v", gotBody["timeout"])
	}
}

func TestBetaCreateUsesDefaultTemplate(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/sandboxes" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode sandbox response: %v", err)
		}
	}))
	defer server.Close()

	sandbox, err := BetaCreate(context.Background(), "", &SandboxBetaCreateOpts{
		SandboxOpts: SandboxOpts{
			ConnectionOpts: ConnectionOpts{
				ApiKey:           "e2b_0000000000000000000000000000000000000000",
				ApiUrl:           server.URL,
				SandboxUrl:       server.URL,
				Domain:           "e2b.app",
				RequestTimeoutMs: intPtr(1000),
				Headers:          map[string]string{},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected BetaCreate to succeed, got %v", err)
	}
	if sandbox == nil || sandbox.SandboxID != "sbx-1" {
		t.Fatalf("unexpected sandbox: %#v", sandbox)
	}
	if gotBody["templateID"] != defaultSandboxTemplate {
		t.Fatalf("expected default template %q, got %#v", defaultSandboxTemplate, gotBody["templateID"])
	}
}

func TestBetaCreateUsesDefaultMcpTemplate(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sandboxes":
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("failed to decode request body: %v", err)
			}
			if err := json.NewEncoder(w).Encode(api.SandboxResponse{
				SandboxID:   "sbx-1",
				TemplateID:  "mcp-gateway",
				EnvdVersion: "1.0.0",
			}); err != nil {
				t.Fatalf("failed to encode sandbox response: %v", err)
			}
		case "/process.Process/Start":
			w.WriteHeader(http.StatusOK)
			var stream bytes.Buffer
			writeProcessEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
			writeProcessEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
			if _, err := w.Write(stream.Bytes()); err != nil {
				t.Fatalf("failed to write command stream: %v", err)
			}
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	sandbox, err := BetaCreate(context.Background(), "", &SandboxBetaCreateOpts{
		SandboxOpts: SandboxOpts{
			ConnectionOpts: ConnectionOpts{
				ApiKey:           "e2b_0000000000000000000000000000000000000000",
				ApiUrl:           server.URL,
				SandboxUrl:       server.URL,
				Domain:           "e2b.app",
				RequestTimeoutMs: intPtr(1000),
				Headers:          map[string]string{},
			},
			Mcp: McpServer{
				"playwright": map[string]any{
					"command": "npx",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected BetaCreate to succeed, got %v", err)
	}
	if sandbox == nil || sandbox.SandboxID != "sbx-1" {
		t.Fatalf("unexpected sandbox: %#v", sandbox)
	}
	if gotBody["templateID"] != defaultSandboxMcpTemplate {
		t.Fatalf("expected default MCP template %q, got %#v", defaultSandboxMcpTemplate, gotBody["templateID"])
	}
}

func TestSandboxInternalsDoNotExposeJsInternalDefaultsOrClientAdapter(t *testing.T) {
	source, err := os.ReadFile("sandbox_api.go")
	if err != nil {
		t.Fatalf("failed to read sandbox_api.go: %v", err)
	}

	text := string(source)
	if strings.Contains(text, "DefaultSandboxTemplate") {
		t.Fatal("did not expect DefaultSandboxTemplate to be exported")
	}
	if strings.Contains(text, "DefaultSandboxMcpTemplate") {
		t.Fatal("did not expect DefaultSandboxMcpTemplate to be exported")
	}
	if strings.Contains(text, "func ToClientConfig(") {
		t.Fatal("did not expect ToClientConfig to be exported")
	}
	if strings.Contains(text, "type McpConfig ") {
		t.Fatal("did not expect McpConfig to be exported")
	}
	if strings.Contains(text, "type SandboxConnectionInfo ") {
		t.Fatal("did not expect SandboxConnectionInfo to be exported")
	}
	if !strings.Contains(text, "type SandboxFullInfo struct") {
		t.Fatal("expected SandboxFullInfo to be exported")
	}
	if strings.Contains(text, "type SandboxUrlOpts ") {
		t.Fatal("did not expect SandboxUrlOpts to be exported")
	}
	if strings.Contains(text, "type VolumeMountInfo struct") {
		t.Fatal("did not expect VolumeMountInfo to be exported")
	}
	if strings.Contains(text, "type SandboxApi struct") {
		t.Fatal("did not expect SandboxApi to be exported")
	}
	if strings.Contains(text, "func (a *SandboxApi) GetFullInfo(") {
		t.Fatal("did not expect SandboxApi.GetFullInfo to be exported")
	}
	if strings.Contains(text, "SandboxStateRunning") {
		t.Fatal("did not expect SandboxStateRunning to be exported")
	}
	if strings.Contains(text, "SandboxStatePaused") {
		t.Fatal("did not expect SandboxStatePaused to be exported")
	}
}

func TestSandboxRespToFullInfoFallsBackToAliasForName(t *testing.T) {
	info := sandboxRespToFullInfo(&api.SandboxResponse{
		SandboxID: "sbx-1",
		Alias:     "template-alias",
	})

	if info.Name != "template-alias" {
		t.Fatalf("expected full info name to fall back to alias, got %q", info.Name)
	}
}

func TestSandboxRespToConnectionInfoUsesOnlyConnectionFields(t *testing.T) {
	info := sandboxRespToConnectionInfo(&api.SandboxResponse{
		SandboxID:          "sbx-1",
		Domain:             "sandbox.example.com",
		EnvdVersion:        "1.0.0",
		EnvdAccessToken:    "envd-token",
		TrafficAccessToken: "traffic-token",
	})

	if info.SandboxID != "sbx-1" {
		t.Fatalf("expected sandbox id to be preserved, got %q", info.SandboxID)
	}
	if info.SandboxDomain != "sandbox.example.com" {
		t.Fatalf("expected connection info sandboxDomain to use domain field, got %q", info.SandboxDomain)
	}
	if info.EnvdVersion != "1.0.0" {
		t.Fatalf("expected envd version to be preserved, got %q", info.EnvdVersion)
	}
	if info.EnvdAccessToken != "envd-token" {
		t.Fatalf("expected envd access token to be preserved, got %q", info.EnvdAccessToken)
	}
	if info.TrafficAccessToken != "traffic-token" {
		t.Fatalf("expected traffic access token to be preserved, got %q", info.TrafficAccessToken)
	}

	infoType := reflect.TypeOf(*info)
	if _, ok := infoType.FieldByName("TemplateID"); ok {
		t.Fatal("did not expect TemplateID on connection info")
	}
	if _, ok := infoType.FieldByName("Metadata"); ok {
		t.Fatal("did not expect Metadata on connection info")
	}
}

func TestSandboxRespToFullInfoDefaultsMetadataAndVolumeMounts(t *testing.T) {
	info := sandboxRespToFullInfo(&api.SandboxResponse{
		SandboxID: "sbx-1",
	})

	if info.Metadata == nil {
		t.Fatal("expected metadata to default to an empty map")
	}
	if len(info.Metadata) != 0 {
		t.Fatalf("expected empty metadata map, got %#v", info.Metadata)
	}
	if info.VolumeMounts == nil {
		t.Fatal("expected volumeMounts to default to an empty slice")
	}
	if len(info.VolumeMounts) != 0 {
		t.Fatalf("expected empty volumeMounts slice, got %#v", info.VolumeMounts)
	}
}

func TestSandboxRespToFullInfoPreservesExtendedFields(t *testing.T) {
	allowInternetAccess := false

	info := sandboxRespToFullInfo(&api.SandboxResponse{
		SandboxID:           "sbx-1",
		AllowInternetAccess: &allowInternetAccess,
		Network: &api.NetworkOpts{
			AllowOut:           []string{"1.1.1.1"},
			DenyOut:            []string{"2.2.2.2"},
			AllowPublicTraffic: boolRef(true),
			MaskRequestHost:    "${PORT}-masked.e2b.app",
		},
		Lifecycle: &api.LifecycleInfoOpts{
			OnTimeout:  "pause",
			AutoResume: true,
		},
		VolumeMounts: []api.VolumeMount{
			{Name: "data", Path: "/mnt/data"},
		},
	})

	if info.AllowInternetAccess == nil || *info.AllowInternetAccess != allowInternetAccess {
		t.Fatalf("expected allowInternetAccess to be preserved, got %#v", info.AllowInternetAccess)
	}
	if info.Network == nil || info.Network.MaskRequestHost != "${PORT}-masked.e2b.app" {
		t.Fatalf("expected network to be preserved, got %#v", info.Network)
	}
	if info.Network.AllowPublicTraffic == nil || !*info.Network.AllowPublicTraffic {
		t.Fatalf("expected allowPublicTraffic to be preserved, got %#v", info.Network.AllowPublicTraffic)
	}
	if info.Lifecycle == nil || info.Lifecycle.OnTimeout != "pause" || !info.Lifecycle.AutoResume {
		t.Fatalf("expected lifecycle to be preserved, got %#v", info.Lifecycle)
	}
	if len(info.VolumeMounts) != 1 || info.VolumeMounts[0].Path != "/mnt/data" {
		t.Fatalf("expected volume mounts to be preserved, got %#v", info.VolumeMounts)
	}

	infoType := reflect.TypeOf(*info)
	if _, ok := infoType.FieldByName("TrafficAccessToken"); ok {
		t.Fatal("did not expect TrafficAccessToken on full info")
	}
}

func TestSandboxApiGetFullInfoReturnsExtendedFields(t *testing.T) {
	allowInternetAccess := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandboxes/sbx-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:           "sbx-1",
			TemplateID:          "base",
			Alias:               "template-alias",
			Metadata:            nil,
			AllowInternetAccess: &allowInternetAccess,
			EnvdVersion:         "1.0.0",
			EnvdAccessToken:     "envd-token",
			StartedAt:           testTime(t, "2024-01-02T03:04:05Z"),
			EndAt:               testTime(t, "2024-01-02T04:04:05Z"),
			State:               "running",
			CpuCount:            2,
			MemoryMB:            512,
			Domain:              "sandbox.example.com",
			Network: &api.NetworkOpts{
				MaskRequestHost: "${PORT}-masked.e2b.app",
			},
			Lifecycle: &api.LifecycleInfoOpts{
				OnTimeout:  "pause",
				AutoResume: true,
			},
			VolumeMounts: nil,
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	info, err := apiClient.getFullInfo(context.Background(), "sbx-1", testSandboxApiOptsPtr(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil || info.SandboxID != "sbx-1" {
		t.Fatalf("unexpected full info: %#v", info)
	}
	if info.Name != "template-alias" {
		t.Fatalf("expected alias-backed name, got %#v", info.Name)
	}
	if info.Metadata == nil || len(info.Metadata) != 0 {
		t.Fatalf("expected metadata to default to empty map, got %#v", info.Metadata)
	}
	if info.AllowInternetAccess == nil || *info.AllowInternetAccess != allowInternetAccess {
		t.Fatalf("expected allowInternetAccess to be preserved, got %#v", info.AllowInternetAccess)
	}
	if info.EnvdAccessToken != "envd-token" {
		t.Fatalf("expected envd access token to be preserved, got %q", info.EnvdAccessToken)
	}
	if info.SandboxDomain != "sandbox.example.com" {
		t.Fatalf("expected sandbox domain to use domain field, got %q", info.SandboxDomain)
	}
	if info.Network == nil || info.Network.MaskRequestHost != "${PORT}-masked.e2b.app" {
		t.Fatalf("expected network to be preserved, got %#v", info.Network)
	}
	if info.Network.AllowPublicTraffic != nil {
		t.Fatalf("expected allowPublicTraffic to stay nil when omitted, got %#v", info.Network.AllowPublicTraffic)
	}
	if info.Lifecycle == nil || info.Lifecycle.OnTimeout != "pause" || !info.Lifecycle.AutoResume {
		t.Fatalf("expected lifecycle to be preserved, got %#v", info.Lifecycle)
	}
	if info.VolumeMounts == nil || len(info.VolumeMounts) != 0 {
		t.Fatalf("expected volumeMounts to default to empty slice, got %#v", info.VolumeMounts)
	}
	if _, ok := reflect.TypeOf(*info).FieldByName("TrafficAccessToken"); ok {
		t.Fatal("did not expect TrafficAccessToken on GetFullInfo result")
	}
}

func TestSandboxApiCreateSandboxReturnsOnlyConnectionFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:          "sbx-1",
			TemplateID:         "base",
			Alias:              "template-alias",
			EnvdVersion:        "1.0.0",
			EnvdAccessToken:    "envd-token",
			TrafficAccessToken: "traffic-token",
			Domain:             "sandbox.example.com",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	info, err := apiClient.createSandbox(context.Background(), &SandboxOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		Template: "base",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.SandboxID != "sbx-1" || info.SandboxDomain != "sandbox.example.com" {
		t.Fatalf("unexpected connection info: %#v", info)
	}
	if info.EnvdVersion != "1.0.0" || info.EnvdAccessToken != "envd-token" || info.TrafficAccessToken != "traffic-token" {
		t.Fatalf("expected connection tokens/version to be preserved, got %#v", info)
	}
	infoType := reflect.TypeOf(*info)
	if _, ok := infoType.FieldByName("TemplateID"); ok {
		t.Fatal("did not expect TemplateID on CreateSandbox result")
	}
	if _, ok := infoType.FieldByName("Name"); ok {
		t.Fatal("did not expect Name on CreateSandbox result")
	}
}

func TestSandboxDoesNotExposeEnvdVersionField(t *testing.T) {
	sandboxType := reflect.TypeOf(Sandbox{})

	if _, ok := sandboxType.FieldByName("EnvdVersion"); ok {
		t.Fatal("did not expect Sandbox to expose EnvdVersion")
	}
}

func TestSandboxDoesNotExposeSandboxApiMethods(t *testing.T) {
	sandboxType := reflect.TypeOf(&Sandbox{})

	if _, ok := sandboxType.MethodByName("DeleteSnapshot"); ok {
		t.Fatal("did not expect Sandbox to expose DeleteSnapshot")
	}
	if _, ok := sandboxType.MethodByName("GetFullInfo"); ok {
		t.Fatal("did not expect Sandbox to expose GetFullInfo")
	}
	if _, ok := sandboxType.MethodByName("CreateSandbox"); ok {
		t.Fatal("did not expect Sandbox to expose CreateSandbox")
	}
	if _, ok := sandboxType.MethodByName("ConnectSandbox"); ok {
		t.Fatal("did not expect Sandbox to expose ConnectSandbox")
	}
	if _, ok := sandboxType.MethodByName("SetSandboxTimeout"); ok {
		t.Fatal("did not expect Sandbox to expose SetSandboxTimeout")
	}
	if _, ok := sandboxType.MethodByName("PauseSandbox"); ok {
		t.Fatal("did not expect Sandbox to expose PauseSandbox")
	}
	if _, ok := sandboxType.MethodByName("BetaPauseSandbox"); ok {
		t.Fatal("did not expect Sandbox to expose BetaPauseSandbox")
	}
	if _, ok := sandboxType.MethodByName("CreateSandboxSnapshot"); ok {
		t.Fatal("did not expect Sandbox to expose CreateSandboxSnapshot")
	}
	if _, ok := sandboxType.MethodByName("ListSandboxSnapshots"); ok {
		t.Fatal("did not expect Sandbox to expose ListSandboxSnapshots")
	}
	if _, ok := sandboxType.MethodByName("GetSandboxInfo"); ok {
		t.Fatal("did not expect Sandbox to expose GetSandboxInfo")
	}
	if _, ok := sandboxType.MethodByName("GetSandboxMetrics"); ok {
		t.Fatal("did not expect Sandbox to expose GetSandboxMetrics")
	}
}

func TestSandboxApiDoesNotExposeInternalCreateConnectHelpers(t *testing.T) {
	source, err := os.ReadFile("sandbox_api.go")
	if err != nil {
		t.Fatalf("failed to read sandbox_api.go: %v", err)
	}

	text := string(source)
	if strings.Contains(text, "type SandboxApi struct") {
		t.Fatal("did not expect SandboxApi to be exported")
	}
	if strings.Contains(text, "func CreateSandbox(") {
		t.Fatal("did not expect CreateSandbox root name to be exported")
	}
	if strings.Contains(text, "func ConnectSandbox(") {
		t.Fatal("did not expect ConnectSandbox root name to be exported")
	}
	if strings.Contains(text, "func ListSandboxes(") {
		t.Fatal("did not expect ListSandboxes root name to be exported")
	}
	if strings.Contains(text, "func KillSandbox(") {
		t.Fatal("did not expect KillSandbox root wrapper to be exported")
	}
	if strings.Contains(text, "func PauseSandbox(") {
		t.Fatal("did not expect PauseSandbox root wrapper to be exported")
	}
	if strings.Contains(text, "func BetaPauseSandbox(") {
		t.Fatal("did not expect BetaPauseSandbox root wrapper to be exported")
	}
	if strings.Contains(text, "func GetSandboxInfo(") {
		t.Fatal("did not expect GetSandboxInfo root wrapper to be exported")
	}
	if strings.Contains(text, "func GetSandboxMetrics(") {
		t.Fatal("did not expect GetSandboxMetrics root wrapper to be exported")
	}
	if strings.Contains(text, "func SetSandboxTimeout(") {
		t.Fatal("did not expect SetSandboxTimeout root wrapper to be exported")
	}
	if !strings.Contains(text, "func GetFullInfo(") {
		t.Fatal("expected GetFullInfo root wrapper to be exported")
	}
	if strings.Contains(text, "func (a *SandboxApi) GetFullInfo(") {
		t.Fatal("did not expect SandboxApi.GetFullInfo to be exported")
	}
}

func TestSandboxListOptsDoesNotExposeLegacyTopLevelFilters(t *testing.T) {
	optsType := reflect.TypeOf(SandboxListOpts{})

	if _, ok := optsType.FieldByName("State"); ok {
		t.Fatal("did not expect SandboxListOpts to expose top-level State")
	}
	if _, ok := optsType.FieldByName("Metadata"); ok {
		t.Fatal("did not expect SandboxListOpts to expose top-level Metadata")
	}

	queryField, ok := optsType.FieldByName("Query")
	if !ok {
		t.Fatal("expected SandboxListOpts to expose Query")
	}
	if queryField.Type.Kind() != reflect.Pointer || queryField.Type.Elem().Kind() != reflect.Struct {
		t.Fatalf("expected SandboxListOpts.Query to be a pointer to struct, got %v", queryField.Type)
	}
	if _, ok := queryField.Type.Elem().FieldByName("Metadata"); !ok {
		t.Fatal("expected SandboxListOpts.Query to expose Metadata")
	}
	if _, ok := queryField.Type.Elem().FieldByName("State"); !ok {
		t.Fatal("expected SandboxListOpts.Query to expose State")
	}
	if _, ok := optsType.FieldByName("Signal"); ok {
		t.Fatal("did not expect SandboxListOpts to expose Signal")
	}
	for _, field := range []string{"ApiKey", "Domain", "Debug", "RequestTimeoutMs", "Headers"} {
		if _, ok := optsType.FieldByName(field); !ok {
			t.Fatalf("expected SandboxListOpts to expose %s", field)
		}
	}
	if field, ok := optsType.FieldByName("Debug"); !ok {
		t.Fatal("expected SandboxListOpts to expose Debug")
	} else if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected SandboxListOpts.Debug to be *bool, got %v", field.Type)
	}
}

func TestSnapshotListOptsMatchJsAndPythonRequestFieldShape(t *testing.T) {
	optsType := reflect.TypeOf(SnapshotListOpts{})

	if _, ok := optsType.FieldByName("Signal"); ok {
		t.Fatal("did not expect SnapshotListOpts to expose Signal")
	}
	for _, field := range []string{"ApiKey", "Domain", "Debug", "RequestTimeoutMs", "Headers", "SandboxID", "Limit", "NextToken"} {
		if _, ok := optsType.FieldByName(field); !ok {
			t.Fatalf("expected SnapshotListOpts to expose %s", field)
		}
	}
	if field, ok := optsType.FieldByName("Debug"); !ok {
		t.Fatal("expected SnapshotListOpts to expose Debug")
	} else if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected SnapshotListOpts.Debug to be *bool, got %v", field.Type)
	}
}

func TestSandboxListOptsDoNotExposeStandaloneQueryHelperType(t *testing.T) {
	source, err := os.ReadFile("sandbox_api.go")
	if err != nil {
		t.Fatalf("failed to read sandbox_api.go: %v", err)
	}

	if strings.Contains(string(source), "type SandboxListQuery struct") {
		t.Fatal("did not expect SandboxListQuery helper type to be exported")
	}
}

func TestCreateOptionsKeepDeprecatedAutoPauseBetaOnly(t *testing.T) {
	createOptsType := reflect.TypeOf(SandboxOpts{})
	if _, ok := createOptsType.FieldByName("AutoPause"); ok {
		t.Fatal("did not expect SandboxOpts to expose deprecated AutoPause")
	}

	betaOptsType := reflect.TypeOf(SandboxBetaCreateOpts{})
	if _, ok := betaOptsType.FieldByName("AutoPause"); !ok {
		t.Fatal("expected SandboxBetaCreateOpts to expose deprecated AutoPause")
	}

	createType := reflect.TypeOf(Create)
	if got := createType.In(2); got != reflect.TypeOf(&SandboxOpts{}) {
		t.Fatalf("expected Create to accept *SandboxOpts, got %v", got)
	}

	betaCreateType := reflect.TypeOf(BetaCreate)
	if got := betaCreateType.In(2); got != reflect.TypeOf(&SandboxBetaCreateOpts{}) {
		t.Fatalf("expected BetaCreate to accept *SandboxBetaCreateOpts, got %v", got)
	}
}

func TestSandboxLifecycleAndNetworkOptionalBooleanShapesMatchJsAndPython(t *testing.T) {
	networkField, ok := reflect.TypeOf(SandboxNetworkOpts{}).FieldByName("AllowPublicTraffic")
	if !ok {
		t.Fatal("expected SandboxNetworkOpts to expose AllowPublicTraffic")
	}
	if networkField.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected SandboxNetworkOpts.AllowPublicTraffic to be *bool, got %v", networkField.Type)
	}

	infoField, ok := reflect.TypeOf(SandboxNetworkInfo{}).FieldByName("AllowPublicTraffic")
	if !ok {
		t.Fatal("expected SandboxNetworkInfo to expose AllowPublicTraffic")
	}
	if infoField.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected SandboxNetworkInfo.AllowPublicTraffic to be *bool, got %v", infoField.Type)
	}

	lifecycleField, ok := reflect.TypeOf(SandboxLifecycle{}).FieldByName("AutoResume")
	if !ok {
		t.Fatal("expected SandboxLifecycle to expose AutoResume")
	}
	if lifecycleField.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected SandboxLifecycle.AutoResume to be *bool, got %v", lifecycleField.Type)
	}

	infoLifecycleField, ok := reflect.TypeOf(SandboxInfoLifecycle{}).FieldByName("AutoResume")
	if !ok {
		t.Fatal("expected SandboxInfoLifecycle to expose AutoResume")
	}
	if infoLifecycleField.Type != reflect.TypeOf(false) {
		t.Fatalf("expected SandboxInfoLifecycle.AutoResume to remain bool, got %v", infoLifecycleField.Type)
	}
}

func TestSandboxApiOptsExposeOnlyPublicJsFields(t *testing.T) {
	optsType := reflect.TypeOf(SandboxApiOpts{})

	if _, ok := optsType.FieldByName("AccessToken"); ok {
		t.Fatal("did not expect SandboxApiOpts to expose AccessToken")
	}
	if _, ok := optsType.FieldByName("ApiUrl"); ok {
		t.Fatal("did not expect SandboxApiOpts to expose ApiUrl")
	}
	if _, ok := optsType.FieldByName("SandboxUrl"); ok {
		t.Fatal("did not expect SandboxApiOpts to expose SandboxUrl")
	}
	if _, ok := optsType.FieldByName("Logger"); ok {
		t.Fatal("did not expect SandboxApiOpts to expose Logger")
	}
	if _, ok := optsType.FieldByName("ApiKey"); !ok {
		t.Fatal("expected SandboxApiOpts to expose ApiKey")
	}
	if _, ok := optsType.FieldByName("Domain"); !ok {
		t.Fatal("expected SandboxApiOpts to expose Domain")
	}
	if _, ok := optsType.FieldByName("Debug"); !ok {
		t.Fatal("expected SandboxApiOpts to expose Debug")
	}
	if field, ok := optsType.FieldByName("Debug"); !ok {
		t.Fatal("expected SandboxApiOpts to expose Debug")
	} else if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected SandboxApiOpts.Debug to be *bool, got %v", field.Type)
	}
	if _, ok := optsType.FieldByName("RequestTimeoutMs"); !ok {
		t.Fatal("expected SandboxApiOpts to expose RequestTimeoutMs")
	}
	if _, ok := optsType.FieldByName("Signal"); !ok {
		t.Fatal("expected SandboxApiOpts to expose Signal")
	}
	if _, ok := optsType.FieldByName("Headers"); !ok {
		t.Fatal("expected SandboxApiOpts to expose Headers")
	}
}

func TestSandboxApiMethodsUseSandboxApiOpts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fn       any
		optsType reflect.Type
	}{
		{name: "Kill", fn: Kill, optsType: reflect.TypeOf(&SandboxApiOpts{})},
		{name: "GetInfo", fn: GetInfo, optsType: reflect.TypeOf(&SandboxApiOpts{})},
		{name: "SetTimeout", fn: SetTimeout, optsType: reflect.TypeOf(&SandboxApiOpts{})},
		{name: "UpdateNetwork", fn: UpdateNetwork, optsType: reflect.TypeOf(&SandboxApiOpts{})},
		{name: "Pause", fn: Pause, optsType: reflect.TypeOf(&SandboxApiOpts{})},
		{name: "BetaPause", fn: BetaPause, optsType: reflect.TypeOf(&SandboxApiOpts{})},
		{name: "CreateSnapshot", fn: CreateSnapshot, optsType: reflect.TypeOf(&CreateSnapshotOpts{})},
		{name: "DeleteSnapshot", fn: DeleteSnapshot, optsType: reflect.TypeOf(&SandboxApiOpts{})},
		{name: "GetMetrics", fn: GetMetrics, optsType: reflect.TypeOf(&SandboxMetricsOpts{})},
		{name: "ListSnapshots", fn: ListSnapshots, optsType: reflect.TypeOf(&SnapshotListOpts{})},
		{name: "List", fn: List, optsType: reflect.TypeOf(&SandboxListOpts{})},
	} {
		fnType := reflect.TypeOf(tc.fn)
		got := fnType.In(fnType.NumIn() - 1)
		if got != tc.optsType {
			t.Fatalf("expected %s to use %v, got %v", tc.name, tc.optsType, got)
		}
	}
}

func TestListFactoriesDoNotExposeContextParameter(t *testing.T) {
	sandboxType := reflect.TypeOf(&Sandbox{})
	listSnapshotsMethod, ok := sandboxType.MethodByName("ListSnapshots")
	if !ok {
		t.Fatal("expected Sandbox.ListSnapshots to exist")
	}
	if listSnapshotsMethod.Type.NumIn() != 2 {
		t.Fatalf("expected Sandbox.ListSnapshots to accept only opts, got %d inputs", listSnapshotsMethod.Type.NumIn()-1)
	}
	listSnapshotsOpts := listSnapshotsMethod.Type.In(1)
	if listSnapshotsOpts.Kind() != reflect.Pointer || listSnapshotsOpts.Elem().Kind() != reflect.Struct {
		t.Fatalf("expected Sandbox.ListSnapshots opts to be a pointer to struct, got %v", listSnapshotsOpts)
	}
	if _, ok := listSnapshotsOpts.Elem().FieldByName("SandboxID"); ok {
		t.Fatal("did not expect Sandbox.ListSnapshots opts to expose SandboxID")
	}
	if _, ok := listSnapshotsOpts.Elem().FieldByName("Limit"); !ok {
		t.Fatal("expected Sandbox.ListSnapshots opts to expose Limit")
	}

	listSnapshotsType := reflect.TypeOf(ListSnapshots)
	if listSnapshotsType.NumIn() != 1 {
		t.Fatalf("expected ListSnapshots to accept only opts, got %d inputs", listSnapshotsType.NumIn())
	}
	if got := listSnapshotsType.In(0); got != reflect.TypeOf(&SnapshotListOpts{}) {
		t.Fatalf("expected ListSnapshots opts type *SnapshotListOpts, got %v", got)
	}
	if got := listSnapshotsType.Out(0); got != reflect.TypeOf(&SnapshotPaginator{}) {
		t.Fatalf("expected ListSnapshots to return *SnapshotPaginator, got %v", got)
	}

	listSandboxesType := reflect.TypeOf(List)
	if listSandboxesType.NumIn() != 1 {
		t.Fatalf("expected List to accept only opts, got %d inputs", listSandboxesType.NumIn())
	}
	if got := listSandboxesType.In(0); got != reflect.TypeOf(&SandboxListOpts{}) {
		t.Fatalf("expected List opts type *SandboxListOpts, got %v", got)
	}
	if got := listSandboxesType.Out(0); got != reflect.TypeOf(&SandboxPaginator{}) {
		t.Fatalf("expected List to return *SandboxPaginator, got %v", got)
	}

	sandboxPaginatorType := reflect.TypeOf(&SandboxPaginator{})
	nextItemsMethod, ok := sandboxPaginatorType.MethodByName("NextItems")
	if !ok {
		t.Fatal("expected SandboxPaginator.NextItems to exist")
	}
	if nextItemsMethod.Type.NumIn() != 2 {
		t.Fatalf("expected SandboxPaginator.NextItems to accept optional opts, got %d inputs", nextItemsMethod.Type.NumIn()-1)
	}
	if got := nextItemsMethod.Type.In(1); got != reflect.TypeOf([]*SandboxApiOpts{}) {
		t.Fatalf("expected SandboxPaginator.NextItems variadic opts type []*SandboxApiOpts, got %v", got)
	}

	snapshotPaginatorType := reflect.TypeOf(&SnapshotPaginator{})
	nextSnapshotItemsMethod, ok := snapshotPaginatorType.MethodByName("NextItems")
	if !ok {
		t.Fatal("expected SnapshotPaginator.NextItems to exist")
	}
	if nextSnapshotItemsMethod.Type.NumIn() != 2 {
		t.Fatalf("expected SnapshotPaginator.NextItems to accept optional opts, got %d inputs", nextSnapshotItemsMethod.Type.NumIn()-1)
	}
	if got := nextSnapshotItemsMethod.Type.In(1); got != reflect.TypeOf([]*SandboxApiOpts{}) {
		t.Fatalf("expected SnapshotPaginator.NextItems variadic opts type []*SandboxApiOpts, got %v", got)
	}
}

func TestSandboxHelpersDoNotExposeContextParameter(t *testing.T) {
	sandboxType := reflect.TypeOf(&Sandbox{})

	getMcpTokenMethod, ok := sandboxType.MethodByName("GetMcpToken")
	if !ok {
		t.Fatal("expected Sandbox.GetMcpToken to exist")
	}
	if getMcpTokenMethod.Type.NumIn() != 1 {
		t.Fatalf("expected Sandbox.GetMcpToken to accept no arguments, got %d inputs", getMcpTokenMethod.Type.NumIn()-1)
	}

	uploadUrlMethod, ok := sandboxType.MethodByName("UploadUrl")
	if !ok {
		t.Fatal("expected Sandbox.UploadUrl to exist")
	}
	if uploadUrlMethod.Type.NumIn() != 3 {
		t.Fatalf("expected Sandbox.UploadUrl to accept path and opts, got %d inputs", uploadUrlMethod.Type.NumIn()-1)
	}
	if got := uploadUrlMethod.Type.In(1); got.Kind() != reflect.String {
		t.Fatalf("expected Sandbox.UploadUrl first argument to be string, got %v", got)
	}
	uploadUrlOpts := uploadUrlMethod.Type.In(2)
	if uploadUrlOpts.Kind() != reflect.Pointer || uploadUrlOpts.Elem().Kind() != reflect.Struct {
		t.Fatalf("expected Sandbox.UploadUrl second argument to be a pointer to struct, got %v", uploadUrlOpts)
	}
	if _, ok := uploadUrlOpts.Elem().FieldByName("UseSignatureExpiration"); !ok {
		t.Fatal("expected Sandbox.UploadUrl opts to expose UseSignatureExpiration")
	}
	if _, ok := uploadUrlOpts.Elem().FieldByName("User"); !ok {
		t.Fatal("expected Sandbox.UploadUrl opts to expose User")
	}

	downloadUrlMethod, ok := sandboxType.MethodByName("DownloadUrl")
	if !ok {
		t.Fatal("expected Sandbox.DownloadUrl to exist")
	}
	if downloadUrlMethod.Type.NumIn() != 3 {
		t.Fatalf("expected Sandbox.DownloadUrl to accept path and opts, got %d inputs", downloadUrlMethod.Type.NumIn()-1)
	}
	if got := downloadUrlMethod.Type.In(1); got.Kind() != reflect.String {
		t.Fatalf("expected Sandbox.DownloadUrl first argument to be string, got %v", got)
	}
	downloadUrlOpts := downloadUrlMethod.Type.In(2)
	if downloadUrlOpts.Kind() != reflect.Pointer || downloadUrlOpts.Elem().Kind() != reflect.Struct {
		t.Fatalf("expected Sandbox.DownloadUrl second argument to be a pointer to struct, got %v", downloadUrlOpts)
	}
	if _, ok := downloadUrlOpts.Elem().FieldByName("UseSignatureExpiration"); !ok {
		t.Fatal("expected Sandbox.DownloadUrl opts to expose UseSignatureExpiration")
	}
	if _, ok := downloadUrlOpts.Elem().FieldByName("User"); !ok {
		t.Fatal("expected Sandbox.DownloadUrl opts to expose User")
	}
}

func TestSandboxOptsUseJsStyleVolumeMountMap(t *testing.T) {
	sandboxOptsType := reflect.TypeOf(SandboxOpts{})
	field, ok := sandboxOptsType.FieldByName("VolumeMounts")
	if !ok {
		t.Fatal("expected SandboxOpts.VolumeMounts field to exist")
	}
	if field.Type.Kind() != reflect.Map {
		t.Fatalf("expected SandboxOpts.VolumeMounts to be a map, got %v", field.Type)
	}
	if field.Type.Key().Kind() != reflect.String {
		t.Fatalf("expected SandboxOpts.VolumeMounts keys to be strings, got %v", field.Type.Key())
	}
	if field.Type.Elem().Kind() != reflect.Interface {
		t.Fatalf("expected SandboxOpts.VolumeMounts values to be interface-like, got %v", field.Type.Elem())
	}
}

func TestSandboxWrapperMethodsUseNarrowJsOptionShapes(t *testing.T) {
	sandboxType := reflect.TypeOf(&Sandbox{})

	isRunningMethod, ok := sandboxType.MethodByName("IsRunning")
	if !ok {
		t.Fatal("expected Sandbox.IsRunning to exist")
	}
	isRunningOpts := isRunningMethod.Type.In(2)
	if isRunningOpts.Kind() != reflect.Pointer || isRunningOpts.Elem().Kind() != reflect.Struct {
		t.Fatalf("expected Sandbox.IsRunning opts to be a pointer to struct, got %v", isRunningOpts)
	}
	if isRunningOpts.Elem().NumField() != 2 || isRunningOpts.Elem().Field(0).Name != "RequestTimeoutMs" || isRunningOpts.Elem().Field(1).Name != "Signal" {
		t.Fatalf("expected Sandbox.IsRunning opts to expose RequestTimeoutMs and Signal, got %v", isRunningOpts.Elem())
	}

	setTimeoutMethod, ok := sandboxType.MethodByName("SetTimeout")
	if !ok {
		t.Fatal("expected Sandbox.SetTimeout to exist")
	}
	setTimeoutOpts := setTimeoutMethod.Type.In(3)
	if setTimeoutOpts.Kind() != reflect.Pointer || setTimeoutOpts.Elem().Kind() != reflect.Struct {
		t.Fatalf("expected Sandbox.SetTimeout opts to be a pointer to struct, got %v", setTimeoutOpts)
	}
	if setTimeoutOpts.Elem().NumField() != 2 || setTimeoutOpts.Elem().Field(0).Name != "RequestTimeoutMs" || setTimeoutOpts.Elem().Field(1).Name != "Signal" {
		t.Fatalf("expected Sandbox.SetTimeout opts to expose RequestTimeoutMs and Signal, got %v", setTimeoutOpts.Elem())
	}

	updateNetworkMethod, ok := sandboxType.MethodByName("UpdateNetwork")
	if !ok {
		t.Fatal("expected Sandbox.UpdateNetwork to exist")
	}
	if got := updateNetworkMethod.Type.In(2); got != reflect.TypeOf(SandboxNetworkUpdate{}) {
		t.Fatalf("expected Sandbox.UpdateNetwork network arg type SandboxNetworkUpdate, got %v", got)
	}
	updateNetworkOpts := updateNetworkMethod.Type.In(3)
	if updateNetworkOpts.Kind() != reflect.Pointer || updateNetworkOpts.Elem().Kind() != reflect.Struct {
		t.Fatalf("expected Sandbox.UpdateNetwork opts to be a pointer to struct, got %v", updateNetworkOpts)
	}
	if updateNetworkOpts.Elem().NumField() != 2 || updateNetworkOpts.Elem().Field(0).Name != "RequestTimeoutMs" || updateNetworkOpts.Elem().Field(1).Name != "Signal" {
		t.Fatalf("expected Sandbox.UpdateNetwork opts to expose RequestTimeoutMs and Signal, got %v", updateNetworkOpts.Elem())
	}

	killMethod, ok := sandboxType.MethodByName("Kill")
	if !ok {
		t.Fatal("expected Sandbox.Kill to exist")
	}
	killOpts := killMethod.Type.In(2)
	if killOpts.Kind() != reflect.Pointer || killOpts.Elem().Kind() != reflect.Struct {
		t.Fatalf("expected Sandbox.Kill opts to be a pointer to struct, got %v", killOpts)
	}
	if killOpts.Elem().NumField() != 2 || killOpts.Elem().Field(0).Name != "RequestTimeoutMs" || killOpts.Elem().Field(1).Name != "Signal" {
		t.Fatalf("expected Sandbox.Kill opts to expose RequestTimeoutMs and Signal, got %v", killOpts.Elem())
	}

	getInfoMethod, ok := sandboxType.MethodByName("GetInfo")
	if !ok {
		t.Fatal("expected Sandbox.GetInfo to exist")
	}
	getInfoOpts := getInfoMethod.Type.In(2)
	if getInfoOpts.Kind() != reflect.Pointer || getInfoOpts.Elem().Kind() != reflect.Struct {
		t.Fatalf("expected Sandbox.GetInfo opts to be a pointer to struct, got %v", getInfoOpts)
	}
	if getInfoOpts.Elem().NumField() != 2 || getInfoOpts.Elem().Field(0).Name != "RequestTimeoutMs" || getInfoOpts.Elem().Field(1).Name != "Signal" {
		t.Fatalf("expected Sandbox.GetInfo opts to expose RequestTimeoutMs and Signal, got %v", getInfoOpts.Elem())
	}

	createSnapshotMethod, ok := sandboxType.MethodByName("CreateSnapshot")
	if !ok {
		t.Fatal("expected Sandbox.CreateSnapshot to exist")
	}
	if got := createSnapshotMethod.Type.In(2); got != reflect.TypeOf(&CreateSnapshotOpts{}) {
		t.Fatalf("expected Sandbox.CreateSnapshot opts type *CreateSnapshotOpts, got %v", got)
	}
}

func TestSandboxApiGetFullInfoWrapsNotFoundAsSandboxNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.getFullInfo(context.Background(), "sbx-1", testSandboxApiOptsPtr(server.URL))
	var notFoundErr *SandboxNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected SandboxNotFoundError, got %T %v", err, err)
	}
}

func TestSandboxApiGetFullInfoReturnsSandboxNotFoundOnEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.getFullInfo(context.Background(), "sbx-1", testSandboxApiOptsPtr(server.URL))
	if err == nil || err.Error() != "Sandbox not found" {
		t.Fatalf("expected Sandbox not found error, got %v", err)
	}
}

func TestSandboxApiGetInfoReturnsSandboxNotFoundOnEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.GetInfo(context.Background(), "sbx-1", testSandboxApiOptsPtr(server.URL))
	if err == nil || err.Error() != "Sandbox not found" {
		t.Fatalf("expected Sandbox not found error, got %v", err)
	}
}

func TestSandboxApiGetMetricsUsesCurrentMetricFieldNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandboxes/sbx-1/metrics" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewEncoder(w).Encode([]api.SandboxMetrics{
			{
				CpuUsedPct: 12.5,
				CpuCount:   2,
				MemUsed:    1234,
				MemTotal:   5678,
				MemCache:   2468,
				DiskUsed:   9012,
				DiskTotal:  3456,
			},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	metrics, err := apiClient.GetMetrics(context.Background(), "sbx-1", &SandboxMetricsOpts{
		SandboxApiOpts: testSandboxApiOpts(server.URL),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected one metrics entry, got %d", len(metrics))
	}

	metric := metrics[0]
	if metric.MemUsed != 1234 || metric.MemTotal != 5678 {
		t.Fatalf("expected current memory metric fields to be populated, got %#v", metric)
	}
	if metric.MemCache != 2468 {
		t.Fatalf("expected mem cache to be populated, got %#v", metric)
	}
	if metric.DiskUsed != 9012 || metric.DiskTotal != 3456 {
		t.Fatalf("expected current disk metric fields to be populated, got %#v", metric)
	}
}

func TestSandboxApiGetMetricsFallsBackToLegacyMetricFieldNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandboxes/sbx-1/metrics" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewEncoder(w).Encode([]api.SandboxMetrics{
			{
				CpuUsedPct:   12.5,
				CpuCount:     2,
				MemUsedMiB:   1234,
				MemTotalMiB:  5678,
				MemCache:     2468,
				DiskUsedMiB:  9012,
				DiskTotalMiB: 3456,
			},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	metrics, err := apiClient.GetMetrics(context.Background(), "sbx-1", &SandboxMetricsOpts{
		SandboxApiOpts: testSandboxApiOpts(server.URL),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected one metrics entry, got %d", len(metrics))
	}

	metric := metrics[0]
	if metric.MemUsed != 1234 || metric.MemTotal != 5678 {
		t.Fatalf("expected legacy memory metric fields to backfill current fields, got %#v", metric)
	}
	if metric.MemCache != 2468 {
		t.Fatalf("expected mem cache to remain populated, got %#v", metric)
	}
	if metric.DiskUsed != 9012 || metric.DiskTotal != 3456 {
		t.Fatalf("expected legacy disk metric fields to backfill current fields, got %#v", metric)
	}
}

func TestSandboxApiGetMetricsRoundsTimeFiltersToNearestSecond(t *testing.T) {
	var gotStart string
	var gotEnd string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotStart = r.URL.Query().Get("start")
		gotEnd = r.URL.Query().Get("end")

		if err := json.NewEncoder(w).Encode([]api.SandboxMetrics{}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	start := time.Unix(10, 600_000_000)
	end := time.Unix(20, 400_000_000)

	apiClient := &sandboxApi{}
	_, err := apiClient.GetMetrics(context.Background(), "sbx-1", &SandboxMetricsOpts{
		SandboxApiOpts: testSandboxApiOpts(server.URL),
		Start:          &start,
		End:            &end,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotStart != "11" {
		t.Fatalf("expected start query to round to 11, got %q", gotStart)
	}
	if gotEnd != "20" {
		t.Fatalf("expected end query to round to 20, got %q", gotEnd)
	}
}

func TestGetSandboxMetricsRejectsOldEnvdVersion(t *testing.T) {
	sandbox := &Sandbox{
		SandboxID:   "sbx-1",
		envdVersion: "0.1.4",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	_, err := sandbox.GetMetrics(context.Background(), nil)
	if err == nil {
		t.Fatal("expected old envd version to be rejected")
	}
	expectedMessage := "You need to update the template to use the new SDK. You can do this by running `e2b template build` in the directory with the template."
	if err.Error() != expectedMessage {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestGetSandboxMetricsWarnsWhenDiskMetricsUnsupported(t *testing.T) {
	logger := &testLogger{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandboxes/sbx-1/metrics" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewEncoder(w).Encode([]api.SandboxMetrics{
			{
				CpuUsedPct: 12.5,
				CpuCount:   2,
				MemUsed:    1234,
				MemTotal:   5678,
			},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID:   "sbx-1",
		envdVersion: "0.2.3",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Logger:           logger,
			Headers:          map[string]string{},
		},
	}

	_, err := sandbox.GetMetrics(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logger.warns) != 1 {
		t.Fatalf("expected one warning, got %#v", logger.warns)
	}
	expectedWarning := "Disk metrics are not supported in this version of the sandbox, please rebuild the template to get disk metrics."
	if logger.warns[0] != expectedWarning {
		t.Fatalf("unexpected warning: %#v", logger.warns)
	}
}

func TestGetSandboxMetricsRoundsTimeFiltersToNearestSecond(t *testing.T) {
	var gotStart string
	var gotEnd string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotStart = r.URL.Query().Get("start")
		gotEnd = r.URL.Query().Get("end")

		if err := json.NewEncoder(w).Encode([]api.SandboxMetrics{}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	start := time.Unix(10, 600_000_000)
	end := time.Unix(20, 400_000_000)

	sandbox := &Sandbox{
		SandboxID:   "sbx-1",
		envdVersion: "1.0.0",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	_, err := sandbox.GetMetrics(context.Background(), &SandboxMetricsOpts{
		Start: &start,
		End:   &end,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotStart != "11" {
		t.Fatalf("expected start query to round to 11, got %q", gotStart)
	}
	if gotEnd != "20" {
		t.Fatalf("expected end query to round to 20, got %q", gotEnd)
	}
}

func TestNewSandboxFromResponseUsesDomainField(t *testing.T) {
	sandbox := newSandboxFromResponse(&api.SandboxResponse{
		SandboxID:   "sbx-1",
		Domain:      "sandbox.example.com",
		EnvdVersion: "1.0.0",
	}, &ConnectionConfig{
		Domain:  "fallback.e2b.app",
		Headers: map[string]string{},
	})

	if sandbox.SandboxDomain != "sandbox.example.com" {
		t.Fatalf("expected sandbox domain to use response domain field, got %q", sandbox.SandboxDomain)
	}
}

func TestNewSandboxFromResponseUsesStableApiUrlAndDirectFileUrlForSupportedDomains(t *testing.T) {
	sandbox := newSandboxFromResponse(&api.SandboxResponse{
		SandboxID:   "sbx-1",
		Domain:      "e2b.app",
		EnvdVersion: "1.0.0",
	}, &ConnectionConfig{
		Domain:           "e2b.app",
		RequestTimeoutMs: 1000,
		Headers:          map[string]string{},
	})

	if sandbox.envdApiUrl != "https://sandbox.e2b.app" {
		t.Fatalf("expected stable envd API URL, got %q", sandbox.envdApiUrl)
	}
	if sandbox.envdDirectUrl != "https://49983-sbx-1.e2b.app" {
		t.Fatalf("expected direct envd URL, got %q", sandbox.envdDirectUrl)
	}

	got := sandbox.fileURL("/hello.txt", "user")
	want := "https://49983-sbx-1.e2b.app/files?username=user&path=%2Fhello.txt"
	if got != want {
		t.Fatalf("expected direct file URL %q, got %q", want, got)
	}
}

func TestNewSandboxFromResponseKeepsPerSandboxApiUrlOutsideSupportedDomains(t *testing.T) {
	sandbox := newSandboxFromResponse(&api.SandboxResponse{
		SandboxID:   "sbx-1",
		Domain:      "sandbox.example.com",
		EnvdVersion: "1.0.0",
	}, &ConnectionConfig{
		Domain:           "e2b.app",
		RequestTimeoutMs: 1000,
		Headers:          map[string]string{},
	})

	want := "https://49983-sbx-1.sandbox.example.com"
	if sandbox.envdApiUrl != want {
		t.Fatalf("expected envd API URL %q, got %q", want, sandbox.envdApiUrl)
	}
	if sandbox.envdDirectUrl != want {
		t.Fatalf("expected envd direct URL %q, got %q", want, sandbox.envdDirectUrl)
	}
}

func TestNewSandboxFromResponseAddsSandboxHeadersToEnvdHealthRequests(t *testing.T) {
	var gotSandboxID string
	var gotSandboxPort string
	var gotAccessToken string
	var gotCustomHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotSandboxID = r.Header.Get("E2b-Sandbox-Id")
		gotSandboxPort = r.Header.Get("E2b-Sandbox-Port")
		gotAccessToken = r.Header.Get("X-Access-Token")
		gotCustomHeader = r.Header.Get("X-Test")
		if err := json.NewEncoder(w).Encode(envd.HealthResponse{
			Status:  "ok",
			Version: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode health response: %v", err)
		}
	}))
	defer server.Close()

	sandbox := newSandboxFromResponse(&api.SandboxResponse{
		SandboxID:       "sbx-1",
		Domain:          "sandbox.example.com",
		EnvdVersion:     "1.0.0",
		EnvdAccessToken: "envd-token",
	}, &ConnectionConfig{
		Domain:           "e2b.app",
		SandboxUrl:       server.URL,
		RequestTimeoutMs: 1000,
		Headers: map[string]string{
			"X-Test": "value",
		},
	})

	running, err := sandbox.IsRunning(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !running {
		t.Fatal("expected sandbox to be running")
	}
	if gotSandboxID != "sbx-1" {
		t.Fatalf("expected E2b-Sandbox-Id header, got %q", gotSandboxID)
	}
	if gotSandboxPort != "49983" {
		t.Fatalf("expected E2b-Sandbox-Port header 49983, got %q", gotSandboxPort)
	}
	if gotAccessToken != "envd-token" {
		t.Fatalf("expected X-Access-Token header envd-token, got %q", gotAccessToken)
	}
	if gotCustomHeader != "" {
		t.Fatalf("expected custom header to be omitted from envd HTTP requests, got %q", gotCustomHeader)
	}
}

func TestIsRunningUsesPerCallRequestTimeoutOverride(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		if err := json.NewEncoder(w).Encode(envd.HealthResponse{
			Status:  "ok",
			Version: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode health response: %v", err)
		}
	}))
	defer server.Close()

	sandbox := newSandboxFromResponse(&api.SandboxResponse{
		SandboxID:   "sbx-1",
		Domain:      "sandbox.example.com",
		EnvdVersion: "1.0.0",
	}, &ConnectionConfig{
		Domain:           "e2b.app",
		SandboxUrl:       server.URL,
		RequestTimeoutMs: 1000,
		Headers:          map[string]string{},
	})

	timeout := 20
	start := time.Now()
	_, err := sandbox.IsRunning(context.Background(), &struct {
		RequestTimeoutMs *int
		Signal           context.Context
	}{
		RequestTimeoutMs: &timeout,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed >= 150*time.Millisecond {
		t.Fatalf("expected per-call timeout to trigger early, elapsed=%s", elapsed)
	}
}

func TestIsRunningHonorsPreCanceledSignalContext(t *testing.T) {
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested <- struct{}{}
		t.Fatal("expected pre-canceled signal to prevent IsRunning request from being sent")
	}))
	defer server.Close()

	sandbox := newSandboxFromResponse(&api.SandboxResponse{
		SandboxID:   "sbx-1",
		Domain:      "sandbox.example.com",
		EnvdVersion: "1.0.0",
	}, &ConnectionConfig{
		Domain:           "e2b.app",
		SandboxUrl:       server.URL,
		RequestTimeoutMs: 1000,
		Headers:          map[string]string{},
	})

	signal, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := sandbox.IsRunning(context.Background(), &struct {
		RequestTimeoutMs *int
		Signal           context.Context
	}{
		Signal: signal,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected pre-canceled signal error, got %T %v", err, err)
	}

	select {
	case <-requested:
		t.Fatal("unexpected IsRunning request despite pre-canceled signal")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestIsRunningHonorsSignalContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	sandbox := newSandboxFromResponse(&api.SandboxResponse{
		SandboxID:   "sbx-1",
		Domain:      "sandbox.example.com",
		EnvdVersion: "1.0.0",
	}, &ConnectionConfig{
		Domain:           "e2b.app",
		SandboxUrl:       server.URL,
		RequestTimeoutMs: 1000,
		Headers:          map[string]string{},
	})

	signal, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := sandbox.IsRunning(context.Background(), &struct {
		RequestTimeoutMs *int
		Signal           context.Context
	}{
		Signal: signal,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled signal error, got %T %v", err, err)
	}
}

func TestUpdateNetworkForwardsPerCallSignal(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandboxes/sbx-1/network" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		requestStarted <- struct{}{}
		<-release
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	signal, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- sandbox.UpdateNetwork(context.Background(), SandboxNetworkUpdate{}, &struct {
			RequestTimeoutMs *int
			Signal           context.Context
		}{
			Signal: signal,
		})
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for update-network request to start")
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled signal error, got %T %v", err, err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for update-network signal cancellation")
	}

	close(release)
}

func TestNewSandboxFromResponseAddsSandboxHeadersToCommandRequests(t *testing.T) {
	var gotSandboxID string
	var gotSandboxPort string
	var gotAccessToken string
	var gotCustomHeader string
	var gotUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/List" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotSandboxID = r.Header.Get("E2b-Sandbox-Id")
		gotSandboxPort = r.Header.Get("E2b-Sandbox-Port")
		gotAccessToken = r.Header.Get("X-Access-Token")
		gotCustomHeader = r.Header.Get("X-Test")
		gotUserAgent = r.Header.Get("User-Agent")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"processes": []any{},
		}); err != nil {
			t.Fatalf("failed to encode list response: %v", err)
		}
	}))
	defer server.Close()

	timeout := 1000
	connConfig := NewConnectionConfig(&ConnectionOpts{
		Domain:           "e2b.app",
		SandboxUrl:       server.URL,
		RequestTimeoutMs: &timeout,
		Headers: map[string]string{
			"X-Test": "value",
		},
	})

	sandbox := newSandboxFromResponse(&api.SandboxResponse{
		SandboxID:       "sbx-1",
		Domain:          "sandbox.example.com",
		EnvdVersion:     "1.0.0",
		EnvdAccessToken: "envd-token",
	}, connConfig)

	processes, err := sandbox.Commands.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(processes) != 0 {
		t.Fatalf("expected no processes, got %#v", processes)
	}
	if gotSandboxID != "sbx-1" {
		t.Fatalf("expected E2b-Sandbox-Id header, got %q", gotSandboxID)
	}
	if gotSandboxPort != "49983" {
		t.Fatalf("expected E2b-Sandbox-Port header 49983, got %q", gotSandboxPort)
	}
	if gotAccessToken != "envd-token" {
		t.Fatalf("expected X-Access-Token header envd-token, got %q", gotAccessToken)
	}
	if gotCustomHeader != "value" {
		t.Fatalf("expected custom header to be preserved, got %q", gotCustomHeader)
	}
	if gotUserAgent != "e2b-go-sdk/dev" {
		t.Fatalf("expected User-Agent to be preserved, got %q", gotUserAgent)
	}
}

func TestNewSandboxFromResponseAddsSandboxHeadersToFilesystemRequests(t *testing.T) {
	var gotSandboxID string
	var gotSandboxPort string
	var gotAccessToken string
	var gotCustomHeader string
	var gotUserAgent string
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotSandboxID = r.Header.Get("E2b-Sandbox-Id")
		gotSandboxPort = r.Header.Get("E2b-Sandbox-Port")
		gotAccessToken = r.Header.Get("X-Access-Token")
		gotCustomHeader = r.Header.Get("X-Test")
		gotUserAgent = r.Header.Get("User-Agent")
		gotPath = r.URL.Query().Get("path")
		if _, err := w.Write([]byte("hello")); err != nil {
			t.Fatalf("failed to write filesystem response: %v", err)
		}
	}))
	defer server.Close()

	timeout := 1000
	connConfig := NewConnectionConfig(&ConnectionOpts{
		Domain:           "e2b.app",
		SandboxUrl:       server.URL,
		RequestTimeoutMs: &timeout,
		Headers: map[string]string{
			"X-Test": "value",
		},
	})

	sandbox := newSandboxFromResponse(&api.SandboxResponse{
		SandboxID:       "sbx-1",
		Domain:          "sandbox.example.com",
		EnvdVersion:     "1.0.0",
		EnvdAccessToken: "envd-token",
	}, connConfig)

	readValue, err := sandbox.Files.Read(context.Background(), "/tmp/hello.txt", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text, ok := readValue.(string)
	if !ok {
		t.Fatalf("expected text read result, got %T", readValue)
	}
	if text != "hello" {
		t.Fatalf("expected filesystem body %q, got %q", "hello", text)
	}
	if gotPath != "/tmp/hello.txt" {
		t.Fatalf("expected filesystem path query to be preserved, got %q", gotPath)
	}
	if gotSandboxID != "sbx-1" {
		t.Fatalf("expected E2b-Sandbox-Id header, got %q", gotSandboxID)
	}
	if gotSandboxPort != "49983" {
		t.Fatalf("expected E2b-Sandbox-Port header 49983, got %q", gotSandboxPort)
	}
	if gotAccessToken != "envd-token" {
		t.Fatalf("expected X-Access-Token header envd-token, got %q", gotAccessToken)
	}
	if gotCustomHeader != "value" {
		t.Fatalf("expected custom header to be preserved, got %q", gotCustomHeader)
	}
	if gotUserAgent != "e2b-go-sdk/dev" {
		t.Fatalf("expected User-Agent to be preserved, got %q", gotUserAgent)
	}
}

func TestNewSandboxFromResponseAddsSandboxHeadersToPtyRequests(t *testing.T) {
	var gotSandboxID string
	var gotSandboxPort string
	var gotAccessToken string
	var gotCustomHeader string
	var gotUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotSandboxID = r.Header.Get("E2b-Sandbox-Id")
		gotSandboxPort = r.Header.Get("E2b-Sandbox-Port")
		gotAccessToken = r.Header.Get("X-Access-Token")
		gotCustomHeader = r.Header.Get("X-Test")
		gotUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		var stream bytes.Buffer
		writeProcessEnvelope(t, &stream, 0x00, []byte(`{"start":{"pid":123}}`))
		writeProcessEnvelope(t, &stream, 0x00, []byte(`{"end":{"exitCode":0}}`))
		if _, err := w.Write(stream.Bytes()); err != nil {
			t.Fatalf("failed to write pty response: %v", err)
		}
	}))
	defer server.Close()

	timeout := 1000
	connConfig := NewConnectionConfig(&ConnectionOpts{
		Domain:           "e2b.app",
		SandboxUrl:       server.URL,
		RequestTimeoutMs: &timeout,
		Headers: map[string]string{
			"X-Test": "value",
		},
	})

	sandbox := newSandboxFromResponse(&api.SandboxResponse{
		SandboxID:       "sbx-1",
		Domain:          "sandbox.example.com",
		EnvdVersion:     "1.0.0",
		EnvdAccessToken: "envd-token",
	}, connConfig)

	handle, err := sandbox.Pty.Create(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("unexpected wait error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected pty exit code 0, got %#v", result)
	}
	if gotSandboxID != "sbx-1" {
		t.Fatalf("expected E2b-Sandbox-Id header, got %q", gotSandboxID)
	}
	if gotSandboxPort != "49983" {
		t.Fatalf("expected E2b-Sandbox-Port header 49983, got %q", gotSandboxPort)
	}
	if gotAccessToken != "envd-token" {
		t.Fatalf("expected X-Access-Token header envd-token, got %q", gotAccessToken)
	}
	if gotCustomHeader != "value" {
		t.Fatalf("expected custom header to be preserved, got %q", gotCustomHeader)
	}
	if gotUserAgent != "e2b-go-sdk/dev" {
		t.Fatalf("expected User-Agent to be preserved, got %q", gotUserAgent)
	}
}

func TestGetMcpTokenSurfacesFileNotFoundWhenMcpNotEnabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	sandbox := &Sandbox{
		Files: filesystem.NewFilesystem(&struct {
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
		}, envd.EnvdDefaultUser),
	}

	_, err := sandbox.GetMcpToken()
	var fileErr *FileNotFoundError
	if !errors.As(err, &fileErr) {
		t.Fatalf("expected FileNotFoundError, got %T %v", err, err)
	}
}

func TestGetSandboxInfoWrapsNotFoundAsSandboxNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	_, err := sandbox.GetInfo(context.Background(), nil)
	var notFoundErr *SandboxNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected SandboxNotFoundError, got %T %v", err, err)
	}
}

func TestGetSandboxInfoReturnsSandboxNotFoundOnEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	_, err := sandbox.GetInfo(context.Background(), nil)
	if err == nil || err.Error() != "Sandbox not found" {
		t.Fatalf("expected Sandbox not found error, got %v", err)
	}
}

func TestCreateSandboxSnapshotErrorsWhenResponseDataIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	_, err := sandbox.CreateSnapshot(context.Background(), nil)
	if err == nil || err.Error() != "Response data is missing" {
		t.Fatalf("expected missing response data error, got %v", err)
	}
}

func TestCreateSandboxSnapshotWrapsNotFoundAsSandboxNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	_, err := sandbox.CreateSnapshot(context.Background(), nil)
	var notFoundErr *SandboxNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected SandboxNotFoundError, got %T %v", err, err)
	}
}

func TestPauseSandboxWrapsNotFoundAsSandboxNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	_, err := sandbox.Pause(context.Background(), nil)
	var notFoundErr *SandboxNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected SandboxNotFoundError, got %T %v", err, err)
	}
}

func TestUpdateNetworkWrapsNotFoundAsSandboxNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	err := sandbox.UpdateNetwork(context.Background(), SandboxNetworkUpdate{}, nil)
	var notFoundErr *SandboxNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected SandboxNotFoundError, got %T %v", err, err)
	}
}

func TestSandboxApiUpdateNetworkWrapsNotFoundAsSandboxNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	err := apiClient.UpdateNetwork(context.Background(), "sbx-1", SandboxNetworkUpdate{}, testSandboxApiOptsPtr(server.URL))
	var notFoundErr *SandboxNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected SandboxNotFoundError, got %T %v", err, err)
	}
}

func TestConnectSandboxWrapsNotFoundAsSandboxNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	_, err := Connect(context.Background(), "sbx-1", &SandboxConnectOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
	})
	var notFoundErr *SandboxNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected SandboxNotFoundError, got %T %v", err, err)
	}
	if err.Error() != "Paused sandbox sbx-1 not found" {
		t.Fatalf("expected paused sandbox not found message, got %v", err)
	}
}

func TestSandboxApiConnectSandboxUsesPausedSandboxNotFoundMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.connectSandbox(context.Background(), "sbx-1", &SandboxConnectOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
	})
	var notFoundErr *SandboxNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected SandboxNotFoundError, got %T %v", err, err)
	}
	if err.Error() != "Paused sandbox sbx-1 not found" {
		t.Fatalf("expected paused sandbox not found message, got %v", err)
	}
}

func TestSandboxApiConnectSandboxPreservesExplicitZeroTimeout(t *testing.T) {
	var gotBody map[string]any
	zero := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandboxes/sbx-1/connect" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.connectSandbox(context.Background(), "sbx-1", &SandboxConnectOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		TimeoutMs: &zero,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	timeout, ok := gotBody["timeout"].(float64)
	if !ok || timeout != 0 {
		t.Fatalf("expected timeout 0 in request body, got %#v", gotBody["timeout"])
	}
}

func TestConnectSandboxErrorsWhenResponseDataIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := Connect(context.Background(), "sbx-1", &SandboxConnectOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
	})
	if err == nil || err.Error() != "Response data is missing" {
		t.Fatalf("expected missing response data error, got %v", err)
	}
}

func TestSandboxApiConnectSandboxErrorsWhenResponseDataIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	_, err := apiClient.connectSandbox(context.Background(), "sbx-1", &SandboxConnectOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
	})
	if err == nil || err.Error() != "Response data is missing" {
		t.Fatalf("expected missing response data error, got %v", err)
	}
}

func TestConnectSandboxDoesNotCallEnvdHealth(t *testing.T) {
	var gotBody map[string]any
	zero := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sandboxes/sbx-1/connect":
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("failed to decode request body: %v", err)
			}
			if err := json.NewEncoder(w).Encode(api.SandboxResponse{
				SandboxID:   "sbx-1",
				TemplateID:  "base",
				EnvdVersion: "1.0.0",
			}); err != nil {
				t.Fatalf("failed to encode sandbox response: %v", err)
			}
		case "/health":
			t.Fatalf("did not expect health check during connect")
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	sandbox, err := Connect(context.Background(), "sbx-1", &SandboxConnectOpts{
		ConnectionOpts: ConnectionOpts{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			SandboxUrl:       server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: intPtr(1000),
			Headers:          map[string]string{},
		},
		TimeoutMs: &zero,
	})
	if err != nil {
		t.Fatalf("expected connect sandbox to succeed without health check, got %v", err)
	}
	if sandbox == nil || sandbox.SandboxID != "sbx-1" {
		t.Fatalf("unexpected sandbox: %#v", sandbox)
	}
	timeout, ok := gotBody["timeout"].(float64)
	if !ok || timeout != 0 {
		t.Fatalf("expected timeout 0 in request body, got %#v", gotBody["timeout"])
	}
}

func TestSandboxApiKillMatchesKillSandboxNotFoundBehavior(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	apiClient := &sandboxApi{}
	killed, err := apiClient.Kill(context.Background(), "sbx-1", testSandboxApiOptsPtr(server.URL))
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}
	if killed {
		t.Fatal("expected Kill to return false on not found")
	}
}

func TestKillIgnoresNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	if err := sandbox.Kill(context.Background(), nil); err != nil {
		t.Fatalf("expected Kill to ignore not found errors, got %v", err)
	}
}

func TestSandboxApiBetaPauseMatchesPauseConflictBehavior(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"already paused"}`, http.StatusConflict)
	}))
	defer server.Close()

	paused, err := BetaPause(context.Background(), "sbx-1", testSandboxApiOptsPtr(server.URL))
	if err != nil {
		t.Fatalf("expected no error for 409 conflict, got %v", err)
	}
	if paused {
		t.Fatal("expected BetaPause to return false on 409 conflict")
	}
}

func TestBetaPauseMatchesPauseConflictBehavior(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"already paused"}`, http.StatusConflict)
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	paused, err := sandbox.BetaPause(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected no error for 409 conflict, got %v", err)
	}
	if paused {
		t.Fatal("expected BetaPause to return false on 409 conflict")
	}
}

func testTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("failed to parse time %q: %v", value, err)
	}
	return parsed
}

func TestKillIsNoopInDebugMode(t *testing.T) {
	sandbox := newDebugSandbox(&ConnectionConfig{
		Debug:   true,
		Domain:  "e2b.app",
		Headers: map[string]string{},
	})

	if err := sandbox.Kill(context.Background(), nil); err != nil {
		t.Fatalf("expected debug Kill to be a no-op, got %v", err)
	}
}

func TestSetTimeoutAliasIsNoopInDebugMode(t *testing.T) {
	sandbox := newDebugSandbox(&ConnectionConfig{
		Debug:   true,
		Domain:  "e2b.app",
		Headers: map[string]string{},
	})

	if err := sandbox.SetTimeout(context.Background(), 30_000, nil); err != nil {
		t.Fatalf("expected debug SetTimeout to be a no-op, got %v", err)
	}
}

func TestPauseAliasMatchesPauseSandboxConflictBehavior(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"already paused"}`, http.StatusConflict)
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	paused, err := sandbox.Pause(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected no error for 409 conflict, got %v", err)
	}
	if paused {
		t.Fatal("expected Pause alias to return false on 409 conflict")
	}
}

func TestPausePassesInheritedConnectionConfigWithoutOverrides(t *testing.T) {
	var gotAPIKey string
	var gotTestHeader string
	var gotUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		gotTestHeader = r.Header.Get("X-Test")
		gotUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	timeout := 1111
	connConfig := NewConnectionConfig(&ConnectionOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		Domain:           "base.e2b.dev",
		RequestTimeoutMs: &timeout,
		Headers: map[string]string{
			"X-Test": "base",
		},
	})
	connConfig.Debug = false

	sandbox := &Sandbox{
		SandboxID:        "sbx-test",
		connectionConfig: connConfig,
	}

	paused, err := sandbox.Pause(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected pause without overrides to succeed, got %v", err)
	}
	if !paused {
		t.Fatal("expected Pause to return true on successful pause")
	}
	if gotAPIKey != "e2b_0000000000000000000000000000000000000000" {
		t.Fatalf("expected inherited API key to be forwarded, got %q", gotAPIKey)
	}
	if gotTestHeader != "base" {
		t.Fatalf("expected inherited X-Test header to be forwarded, got %q", gotTestHeader)
	}
	if gotUserAgent != "e2b-go-sdk/dev" {
		t.Fatalf("expected inherited User-Agent to be forwarded, got %q", gotUserAgent)
	}
}

func TestPauseLetsPerCallOverridesWinOverInheritedConnectionConfig(t *testing.T) {
	var gotAPIKey string
	var gotTestHeader string
	var gotExtraHeader string
	var gotUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		gotTestHeader = r.Header.Get("X-Test")
		gotExtraHeader = r.Header.Get("X-Extra")
		gotUserAgent = r.Header.Get("User-Agent")
		time.Sleep(60 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	baseTimeout := 20
	overrideTimeout := 200
	connConfig := NewConnectionConfig(&ConnectionOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		Domain:           "base.e2b.dev",
		RequestTimeoutMs: &baseTimeout,
		Headers: map[string]string{
			"X-Test": "base",
		},
	})
	connConfig.Debug = false

	sandbox := &Sandbox{
		SandboxID:        "sbx-test",
		connectionConfig: connConfig,
	}

	start := time.Now()
	paused, err := sandbox.Pause(context.Background(), &ConnectionOpts{
		ApiKey:           "e2b_1111111111111111111111111111111111111111",
		Domain:           "override.e2b.dev",
		RequestTimeoutMs: &overrideTimeout,
		Headers: map[string]string{
			"X-Test":  "override",
			"X-Extra": "1",
		},
	})
	if err != nil {
		t.Fatalf("expected pause override call to succeed, got %v", err)
	}
	if !paused {
		t.Fatal("expected Pause to return true when override timeout allows response to complete")
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Fatalf("expected override request timeout to allow delayed response, elapsed=%s", elapsed)
	}
	if gotAPIKey != "e2b_1111111111111111111111111111111111111111" {
		t.Fatalf("expected per-call API key override to win, got %q", gotAPIKey)
	}
	if gotTestHeader != "override" {
		t.Fatalf("expected per-call X-Test header override to win, got %q", gotTestHeader)
	}
	if gotExtraHeader != "1" {
		t.Fatalf("expected per-call X-Extra header to be forwarded, got %q", gotExtraHeader)
	}
	if gotUserAgent != "e2b-go-sdk/dev" {
		t.Fatalf("expected inherited User-Agent to be preserved, got %q", gotUserAgent)
	}
}

func TestPauseMergesPerCallHeadersOverInheritedConnectionHeaders(t *testing.T) {
	var gotTestHeader string
	var gotExtraHeader string
	var gotUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTestHeader = r.Header.Get("X-Test")
		gotExtraHeader = r.Header.Get("X-Extra")
		gotUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"already paused"}`))
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers: map[string]string{
				"User-Agent": "e2b-go-sdk/dev",
				"X-Test":     "base",
			},
		},
	}

	paused, err := sandbox.Pause(context.Background(), &ConnectionOpts{
		Headers: map[string]string{
			"X-Extra": "1",
		},
	})
	if err != nil {
		t.Fatalf("expected no error for 409 conflict, got %v", err)
	}
	if paused {
		t.Fatal("expected Pause alias to return false on 409 conflict")
	}
	if gotTestHeader != "base" {
		t.Fatalf("expected inherited X-Test header to be preserved, got %q", gotTestHeader)
	}
	if gotExtraHeader != "1" {
		t.Fatalf("expected per-call X-Extra header to be forwarded, got %q", gotExtraHeader)
	}
	if gotUserAgent != "e2b-go-sdk/dev" {
		t.Fatalf("expected inherited User-Agent to be preserved, got %q", gotUserAgent)
	}
}

func TestConnectMethodReturnsSameSandboxHandle(t *testing.T) {
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewEncoder(w).Encode(api.SandboxResponse{
			SandboxID:   "sbx-1",
			TemplateID:  "base",
			EnvdVersion: "1.0.0",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	sameSandbox, err := sandbox.Connect(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sameSandbox != sandbox {
		t.Fatal("expected Connect to return the same sandbox handle")
	}
	if gotPath != "/sandboxes/sbx-1/connect" {
		t.Fatalf("expected connect path /sandboxes/sbx-1/connect, got %q", gotPath)
	}
}

func TestResolveConnectionConfigAllowsExplicitZeroRequestTimeout(t *testing.T) {
	sandbox := &Sandbox{
		connectionConfig: &ConnectionConfig{
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	resolved := sandbox.resolveConnectionConfig(&ConnectionOpts{
		RequestTimeoutMs: intPtr(0),
	})

	if resolved.RequestTimeoutMs != 0 {
		t.Fatalf("expected explicit zero request timeout to override inherited config, got %d", resolved.RequestTimeoutMs)
	}
}

func TestResolveConnectionConfigCarriesSandboxUrlAndLoggerOverrides(t *testing.T) {
	baseLogger := &testLogger{}
	overrideLogger := &testLogger{}

	sandbox := &Sandbox{
		connectionConfig: &ConnectionConfig{
			SandboxUrl:       "https://sandbox.base",
			Logger:           baseLogger,
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	resolved := sandbox.resolveConnectionConfig(&ConnectionOpts{
		SandboxUrl: "https://sandbox.override",
		Logger:     overrideLogger,
	})

	if resolved.SandboxUrl != "https://sandbox.override" {
		t.Fatalf("expected SandboxUrl override to be preserved, got %q", resolved.SandboxUrl)
	}
	if resolved.Logger != overrideLogger {
		t.Fatalf("expected logger override to be preserved, got %#v", resolved.Logger)
	}
}

func TestResolveConnectionConfigMergesHeadersWithPerCallOverrides(t *testing.T) {
	sandbox := &Sandbox{
		connectionConfig: &ConnectionConfig{
			RequestTimeoutMs: 1000,
			Headers: map[string]string{
				"User-Agent": "e2b-go-sdk/dev",
				"X-Test":     "base",
			},
		},
	}

	resolved := sandbox.resolveConnectionConfig(&ConnectionOpts{
		Headers: map[string]string{
			"X-Extra": "1",
		},
	})

	if resolved.Headers["User-Agent"] != "e2b-go-sdk/dev" {
		t.Fatalf("expected base User-Agent to be preserved, got %#v", resolved.Headers)
	}
	if resolved.Headers["X-Test"] != "base" {
		t.Fatalf("expected base header to be preserved, got %#v", resolved.Headers)
	}
	if resolved.Headers["X-Extra"] != "1" {
		t.Fatalf("expected override header to be merged, got %#v", resolved.Headers)
	}
}

func TestResolveConnectionConfigLetsPerCallHeadersOverrideBaseValues(t *testing.T) {
	sandbox := &Sandbox{
		connectionConfig: &ConnectionConfig{
			RequestTimeoutMs: 1000,
			Headers: map[string]string{
				"X-Test":  "base",
				"X-Other": "keep",
			},
		},
	}

	resolved := sandbox.resolveConnectionConfig(&ConnectionOpts{
		Headers: map[string]string{
			"X-Test": "override",
		},
	})

	if resolved.Headers["X-Test"] != "override" {
		t.Fatalf("expected override header to win, got %#v", resolved.Headers)
	}
	if resolved.Headers["X-Other"] != "keep" {
		t.Fatalf("expected unrelated base header to be preserved, got %#v", resolved.Headers)
	}
}

func TestResolveConnectionConfigAllowsExplicitFalseDebugOverride(t *testing.T) {
	sandbox := &Sandbox{
		connectionConfig: &ConnectionConfig{
			Debug:            true,
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	resolved := sandbox.resolveConnectionConfig(&ConnectionOpts{
		Debug: boolRef(false),
	})

	if resolved.Debug {
		t.Fatalf("expected explicit false debug override to disable inherited debug, got %#v", resolved)
	}
}

func TestResolveSandboxApiConnectionConfigMergesHeadersWithOverrides(t *testing.T) {
	sandbox := &Sandbox{
		connectionConfig: &ConnectionConfig{
			RequestTimeoutMs: 1000,
			Headers: map[string]string{
				"User-Agent": "e2b-go-sdk/dev",
				"X-Test":     "base",
			},
		},
	}

	resolved := sandbox.resolveSandboxApiConnectionConfig(&SandboxApiOpts{
		Headers: map[string]string{
			"X-Extra": "1",
		},
	})

	if resolved.Headers["User-Agent"] != "e2b-go-sdk/dev" {
		t.Fatalf("expected base User-Agent to be preserved, got %#v", resolved.Headers)
	}
	if resolved.Headers["X-Test"] != "base" {
		t.Fatalf("expected base header to be preserved, got %#v", resolved.Headers)
	}
	if resolved.Headers["X-Extra"] != "1" {
		t.Fatalf("expected override header to be merged, got %#v", resolved.Headers)
	}
}

func TestResolveSandboxApiConnectionConfigAllowsExplicitFalseDebugOverride(t *testing.T) {
	sandbox := &Sandbox{
		connectionConfig: &ConnectionConfig{
			Debug:            true,
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	resolved := sandbox.resolveSandboxApiConnectionConfig(&SandboxApiOpts{
		Debug: boolRef(false),
	})

	if resolved.Debug {
		t.Fatalf("expected explicit false debug override to disable inherited debug, got %#v", resolved)
	}
}

func TestListSnapshotsAliasUsesSandboxSnapshotsEndpoint(t *testing.T) {
	var gotPath string
	var gotSandboxID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSandboxID = r.URL.Query().Get("sandboxID")
		if err := json.NewEncoder(w).Encode([]api.SnapshotInfo{{SnapshotID: "snap-1"}}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	sandbox := &Sandbox{
		SandboxID: "sbx-1",
		connectionConfig: &ConnectionConfig{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			Domain:           "e2b.app",
			RequestTimeoutMs: 1000,
			Headers:          map[string]string{},
		},
	}

	paginator := sandbox.ListSnapshots(nil)
	items, err := paginator.NextItems()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/snapshots" {
		t.Fatalf("expected path /snapshots, got %q", gotPath)
	}
	if gotSandboxID != "sbx-1" {
		t.Fatalf("expected sandboxID query sbx-1, got %q", gotSandboxID)
	}
	if len(items) != 1 || items[0].SnapshotID != "snap-1" {
		t.Fatalf("unexpected snapshot items: %#v", items)
	}
}

func mustQueryValue(t *testing.T, rawURL string, key string) string {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("failed to parse URL %q: %v", rawURL, err)
	}

	return req.URL.Query().Get(key)
}
