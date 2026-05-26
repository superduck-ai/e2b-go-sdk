package volume

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVolumeListReturnsEmptySliceWhenResponseBodyMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	entries, err := v.List(context.Background(), "/dir", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if entries == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty slice, got %#v", entries)
	}
}

func TestVolumeGetInfoErrorsWhenResponseDataIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	_, err := v.GetInfo(context.Background(), "/file.txt", nil)
	if err == nil {
		t.Fatal("expected missing response data error")
	}
	if err.Error() != "Response data is missing" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVolumeMakeDirErrorsWhenResponseDataIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	_, err := v.MakeDir(context.Background(), "/dir", nil)
	if err == nil {
		t.Fatal("expected missing response data error")
	}
	if err.Error() != "Response data is missing" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVolumeUpdateMetadataErrorsWhenResponseDataIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	_, err := v.UpdateMetadata(context.Background(), "/file.txt", &VolumeMetadataOptions{}, nil)
	if err == nil {
		t.Fatal("expected missing response data error")
	}
	if err.Error() != "Response data is missing" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVolumeWriteFileErrorsWhenResponseDataIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	_, err := v.WriteFile(context.Background(), "/file.txt", http.NoBody, nil)
	if err == nil {
		t.Fatal("expected missing response data error")
	}
	if err.Error() != "Response data is missing" {
		t.Fatalf("unexpected error: %v", err)
	}
}
