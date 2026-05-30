package volume

import (
	"context"
	"encoding/json"
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
	entries, err := v.List(context.Background(), "/dir", testVolumeListOpts(server.URL))
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
	_, err := v.GetInfo(context.Background(), "/file.txt", testVolumeApiOpts(server.URL))
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
	_, err := v.MakeDir(context.Background(), "/dir", &VolumeWriteOptions{ApiUrl: server.URL})
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
	_, err := v.UpdateMetadata(context.Background(), "/file.txt", &VolumeMetadataOptions{}, testVolumeApiOpts(server.URL))
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
	_, err := v.WriteFile(context.Background(), "/file.txt", http.NoBody, &VolumeWriteOptions{ApiUrl: server.URL})
	if err == nil {
		t.Fatal("expected missing response data error")
	}
	if err.Error() != "Response data is missing" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVolumeEntryStatDecodesCurrentTsSchemaFields(t *testing.T) {
	var entry VolumeEntryStat
	if err := json.Unmarshal([]byte(`{
		"name":"file.txt",
		"path":"/file.txt",
		"type":"symlink",
		"size":12,
		"uid":1000,
		"gid":1001,
		"mode":420,
		"target":"/target"
	}`), &entry); err != nil {
		t.Fatalf("failed to decode entry: %v", err)
	}

	if entry.UID != 1000 || entry.GID != 1001 || entry.Mode != 420 || entry.Target != "/target" {
		t.Fatalf("entry metadata did not decode like TS schema: %#v", entry)
	}
}
