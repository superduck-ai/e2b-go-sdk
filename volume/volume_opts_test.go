package volume

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/e2b-dev/e2b-go-sdk/api"
)

func TestVolumeGetInfoUsesPerCallApiURLOverride(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumecontent/vol-1/path" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(VolumeEntryStat{Name: "file.txt", Path: "/file.txt"}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	v := testVolumeClient("http://127.0.0.1:1")
	info, err := v.GetInfo(context.Background(), "/file.txt", &VolumeApiOpts{
		ApiUrl: server.URL,
	})
	if err != nil {
		t.Fatalf("expected per-call apiUrl override to be used, got %v", err)
	}
	if info == nil || info.Path != "/file.txt" {
		t.Fatalf("unexpected info: %#v", info)
	}
}

func TestVolumeGetInfoUsesPerCallRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		if err := json.NewEncoder(w).Encode(VolumeEntryStat{Name: "file.txt", Path: "/file.txt"}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	timeout := 20
	v := testVolumeClient(server.URL)

	start := time.Now()
	_, err := v.GetInfo(context.Background(), "/file.txt", &VolumeApiOpts{
		RequestTimeoutMs: &timeout,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed >= 150*time.Millisecond {
		t.Fatalf("expected per-call timeout to trigger early, elapsed=%s", elapsed)
	}
}

func TestVolumeListUsesDepthOption(t *testing.T) {
	var gotDepth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDepth = r.URL.Query().Get("depth")
		if err := json.NewEncoder(w).Encode([]VolumeEntryStat{}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	depth := 3
	v := testVolumeClient(server.URL)
	if _, err := v.List(context.Background(), "/dir", &struct {
		Token            string
		Domain           string
		Debug            bool
		ApiUrl           string
		RequestTimeoutMs *int
		Logger           api.Logger
		Headers          map[string]string
		Depth            *int
	}{Depth: &depth}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotDepth != "3" {
		t.Fatalf("expected depth query 3, got %q", gotDepth)
	}
}

func TestVolumeApiOptsOnlyExposeConnectionFields(t *testing.T) {
	optsType := reflect.TypeOf(VolumeApiOpts{})
	for _, field := range []string{"Depth", "UID", "GID", "Mode", "Force"} {
		if _, ok := optsType.FieldByName(field); ok {
			t.Fatalf("did not expect VolumeApiOpts to expose %s", field)
		}
	}
}

func TestVolumeWriteFileUsesPerCallApiURLAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		time.Sleep(50 * time.Millisecond)
		if err := json.NewEncoder(w).Encode(VolumeEntryStat{Name: "file.txt", Path: "/file.txt"}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	timeout := 0
	v := testVolumeClient("http://127.0.0.1:1")
	info, err := v.WriteFile(context.Background(), "/file.txt", http.NoBody, &VolumeWriteOptions{
		ApiUrl:           server.URL,
		RequestTimeoutMs: &timeout,
	})
	if err != nil {
		t.Fatalf("expected per-call write opts to be used, got %v", err)
	}
	if info == nil || info.Path != "/file.txt" {
		t.Fatalf("unexpected info: %#v", info)
	}
}
