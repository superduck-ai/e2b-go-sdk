package volume

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

func TestGetVolumeInfoWrapsNotFoundAsSdkNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	_, err := GetInfo(context.Background(), "vol-1", &ConnectionOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: ptr(1000),
	})
	if err == nil {
		t.Fatal("expected volume info error")
	}

	var notFoundErr *shared.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T %v", err, err)
	}
	if notFoundErr.Message != "Volume vol-1 not found" {
		t.Fatalf("unexpected error message: %q", notFoundErr.Message)
	}
}

func TestCreateWrapsGenericApiErrorsAsVolumeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"quota exceeded"}`, http.StatusForbidden)
	}))
	defer server.Close()

	_, err := Create(context.Background(), "test", &ConnectionOpts{
		ApiKey: "e2b_0000000000000000000000000000000000000000",
		ApiUrl: server.URL,
	})
	if err == nil {
		t.Fatal("expected create error")
	}

	var volumeErr *shared.VolumeError
	if !errors.As(err, &volumeErr) {
		t.Fatalf("expected VolumeError, got %T %v", err, err)
	}
	if volumeErr.Message != "quota exceeded" {
		t.Fatalf("unexpected volume error message: %q", volumeErr.Message)
	}
}

func TestListWrapsGenericApiErrorsAsVolumeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"backend unavailable"}`, http.StatusBadGateway)
	}))
	defer server.Close()

	_, err := List(context.Background(), &ConnectionOpts{
		ApiKey: "e2b_0000000000000000000000000000000000000000",
		ApiUrl: server.URL,
	})
	if err == nil {
		t.Fatal("expected list error")
	}

	var volumeErr *shared.VolumeError
	if !errors.As(err, &volumeErr) {
		t.Fatalf("expected VolumeError, got %T %v", err, err)
	}
	if volumeErr.Message != "backend unavailable" {
		t.Fatalf("unexpected volume error message: %q", volumeErr.Message)
	}
}

func TestDestroyWrapsGenericApiErrorsAsVolumeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"backend unavailable"}`, http.StatusBadGateway)
	}))
	defer server.Close()

	_, err := Destroy(context.Background(), "vol-1", &ConnectionOpts{
		ApiKey: "e2b_0000000000000000000000000000000000000000",
		ApiUrl: server.URL,
	})
	if err == nil {
		t.Fatal("expected destroy error")
	}

	var volumeErr *shared.VolumeError
	if !errors.As(err, &volumeErr) {
		t.Fatalf("expected VolumeError, got %T %v", err, err)
	}
	if volumeErr.Message != "backend unavailable" {
		t.Fatalf("unexpected volume error message: %q", volumeErr.Message)
	}
}

func TestCreateErrorsWhenResponseDataIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := Create(context.Background(), "test", &ConnectionOpts{
		ApiKey: "e2b_0000000000000000000000000000000000000000",
		ApiUrl: server.URL,
	})
	if err == nil {
		t.Fatal("expected missing response data error")
	}
	if err.Error() != "Response data is missing" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetVolumeInfoErrorsWhenResponseDataIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := GetInfo(context.Background(), "vol-1", &ConnectionOpts{
		ApiKey: "e2b_0000000000000000000000000000000000000000",
		ApiUrl: server.URL,
	})
	if err == nil {
		t.Fatal("expected missing response data error")
	}
	if err.Error() != "Response data is missing" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVolumeExistsReturnsFalseForTypedNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	exists, err := v.Exists(context.Background(), "/missing.txt", testVolumeApiOpts(server.URL))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if exists {
		t.Fatal("expected Exists to return false for missing path")
	}
}

func TestVolumeReadFileWrapsNotFoundAsSdkNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	_, err := v.ReadFile(context.Background(), "/missing.txt", testVolumeReadOpts(server.URL))
	if err == nil {
		t.Fatal("expected read file error")
	}

	var notFoundErr *shared.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T %v", err, err)
	}
	if notFoundErr.Message != "Path /missing.txt not found" {
		t.Fatalf("unexpected error message: %q", notFoundErr.Message)
	}
}

func TestVolumeListWrapsNotFoundAsSdkNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	_, err := v.List(context.Background(), "/missing-dir", testVolumeListOpts(server.URL))
	if err == nil {
		t.Fatal("expected list error")
	}

	var notFoundErr *shared.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T %v", err, err)
	}
	if notFoundErr.Message != "Path /missing-dir not found" {
		t.Fatalf("unexpected error message: %q", notFoundErr.Message)
	}
}

func TestVolumeUpdateMetadataWrapsNotFoundAsSdkNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	mode := 0o644
	_, err := v.UpdateMetadata(context.Background(), "/missing.txt", &VolumeMetadataOptions{Mode: &mode}, testVolumeApiOpts(server.URL))
	if err == nil {
		t.Fatal("expected update metadata error")
	}

	var notFoundErr *shared.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T %v", err, err)
	}
	if notFoundErr.Message != "Path /missing.txt not found" {
		t.Fatalf("unexpected error message: %q", notFoundErr.Message)
	}
}

func TestVolumeMakeDirWrapsNotFoundAsSdkNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	_, err := v.MakeDir(context.Background(), "/missing-parent/child", &VolumeWriteOptions{ApiUrl: server.URL})
	if err == nil {
		t.Fatal("expected make dir error")
	}

	var notFoundErr *shared.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T %v", err, err)
	}
	if notFoundErr.Message != "Path /missing-parent/child not found" {
		t.Fatalf("unexpected error message: %q", notFoundErr.Message)
	}
}

func TestVolumeRemoveWrapsNotFoundAsSdkNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	err := v.Remove(context.Background(), "/missing.txt", testVolumeApiOpts(server.URL))
	if err == nil {
		t.Fatal("expected remove error")
	}

	var notFoundErr *shared.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T %v", err, err)
	}
	if notFoundErr.Message != "Path /missing.txt not found" {
		t.Fatalf("unexpected error message: %q", notFoundErr.Message)
	}
}

func TestVolumeReadFileReturnsStreamResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumecontent/vol-1/file" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("path"); got != "/stream.txt" {
			t.Fatalf("unexpected path query: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("volume stream")); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	body := readVolumeStreamValue(t, v, "/stream.txt", testVolumeApiOpts(server.URL))
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read stream: %v", err)
	}
	if err := body.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if string(data) != "volume stream" {
		t.Fatalf("unexpected stream body: %q", string(data))
	}
}

func TestVolumeRemoveWrapsGenericApiErrorsAsVolumeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "permission denied", http.StatusForbidden)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	err := v.Remove(context.Background(), "/file.txt", testVolumeApiOpts(server.URL))
	if err == nil {
		t.Fatal("expected remove error")
	}

	var volumeErr *shared.VolumeError
	if !errors.As(err, &volumeErr) {
		t.Fatalf("expected VolumeError, got %T %v", err, err)
	}
	if volumeErr.Message != "permission denied\n" {
		t.Fatalf("unexpected volume error message: %q", volumeErr.Message)
	}
}

func testVolumeClient(_ string) *Volume {
	return &Volume{
		VolumeID: "vol-1",
		Name:     "test",
		Token:    "token",
		Domain:   "e2b.app",
		Debug:    boolPtr(false),
	}
}

func testVolumeApiOpts(apiURL string) *VolumeApiOpts {
	return &VolumeApiOpts{
		ApiUrl: apiURL,
	}
}

func testVolumeReadOpts(apiURL string) *VolumeReadOpts {
	return &VolumeReadOpts{
		VolumeApiOpts: VolumeApiOpts{
			ApiUrl: apiURL,
		},
	}
}

func testVolumeListOpts(apiURL string) *VolumeListOpts {
	return &VolumeListOpts{
		VolumeApiOpts: VolumeApiOpts{
			ApiUrl: apiURL,
		},
	}
}

func ptr(value int) *int {
	return &value
}
