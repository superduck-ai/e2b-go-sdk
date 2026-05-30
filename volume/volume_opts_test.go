package volume

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/api"
)

type testVolumeLogger struct{}

func (l *testVolumeLogger) Debug(args ...interface{}) {}
func (l *testVolumeLogger) Info(args ...interface{})  {}
func (l *testVolumeLogger) Warn(args ...interface{})  {}
func (l *testVolumeLogger) Error(args ...interface{}) {}

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
		ApiUrl:           server.URL,
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
	if _, err := v.List(context.Background(), "/dir", &VolumeListOpts{
		VolumeApiOpts: VolumeApiOpts{ApiUrl: server.URL},
		Depth:         &depth,
	}); err != nil {
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

func TestVolumeReadOptsMatchJsAndPythonReadShape(t *testing.T) {
	optsType := reflect.TypeOf(VolumeReadOpts{})
	want := []string{"VolumeApiOpts", "Format"}

	got := make([]string, 0, optsType.NumField())
	for i := 0; i < optsType.NumField(); i++ {
		got = append(got, optsType.Field(i).Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected VolumeReadOpts field shape: got %v want %v", got, want)
	}
	if field, ok := optsType.FieldByName("Format"); !ok {
		t.Fatal("expected VolumeReadOpts to expose Format")
	} else if field.Type != reflect.TypeOf(ReadFileFormat("")) {
		t.Fatalf("expected VolumeReadOpts.Format to be ReadFileFormat, got %v", field.Type)
	}
}

func TestVolumeListOptsMatchJsAndPythonDepthShape(t *testing.T) {
	optsType := reflect.TypeOf(VolumeListOpts{})
	want := []string{"VolumeApiOpts", "Depth"}

	got := make([]string, 0, optsType.NumField())
	for i := 0; i < optsType.NumField(); i++ {
		got = append(got, optsType.Field(i).Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected VolumeListOpts field shape: got %v want %v", got, want)
	}
	if field, ok := optsType.FieldByName("Depth"); !ok {
		t.Fatal("expected VolumeListOpts to expose Depth")
	} else if field.Type != reflect.TypeOf((*int)(nil)) {
		t.Fatalf("expected VolumeListOpts.Depth to be *int, got %v", field.Type)
	}
}

func TestVolumeWriteOptionsExposeLoggerLikeJsVolumeApiOpts(t *testing.T) {
	optsType := reflect.TypeOf(VolumeWriteOptions{})
	field, ok := optsType.FieldByName("Logger")
	if !ok {
		t.Fatal("expected VolumeWriteOptions to expose Logger like JS VolumeApiOpts")
	}
	if field.Type != reflect.TypeOf((*api.Logger)(nil)).Elem() {
		t.Fatalf("expected VolumeWriteOptions.Logger to be api.Logger, got %v", field.Type)
	}
}

func TestVolumeApiOptsDebugMatchesJsAndPythonOptionalShape(t *testing.T) {
	field, ok := reflect.TypeOf(VolumeApiOpts{}).FieldByName("Debug")
	if !ok {
		t.Fatal("expected VolumeApiOpts to expose Debug")
	}
	if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected VolumeApiOpts.Debug to be *bool, got %v", field.Type)
	}
}

func TestVolumeWriteOptionsDebugMatchesJsAndPythonOptionalShape(t *testing.T) {
	field, ok := reflect.TypeOf(VolumeWriteOptions{}).FieldByName("Debug")
	if !ok {
		t.Fatal("expected VolumeWriteOptions to expose Debug")
	}
	if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected VolumeWriteOptions.Debug to be *bool, got %v", field.Type)
	}
}

func TestVolumeWriteOptionsForceMatchesJsAndPythonOptionalShape(t *testing.T) {
	field, ok := reflect.TypeOf(VolumeWriteOptions{}).FieldByName("Force")
	if !ok {
		t.Fatal("expected VolumeWriteOptions to expose Force")
	}
	if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected VolumeWriteOptions.Force to be *bool, got %v", field.Type)
	}
}

func TestQueryFromVolumeWriteOptsPreservesExplicitFalseForce(t *testing.T) {
	if got := queryFromVolumeWriteOpts(&VolumeWriteOptions{}).Get("force"); got != "" {
		t.Fatalf("expected nil force to be omitted, got %q", got)
	}

	force := false
	if got := queryFromVolumeWriteOpts(&VolumeWriteOptions{Force: &force}).Get("force"); got != "false" {
		t.Fatalf("expected explicit false force to be preserved, got %q", got)
	}
}

func TestVolumeWriteOptsToApiOptsPreservesLogger(t *testing.T) {
	logger := &testVolumeLogger{}
	apiOpts := volumeWriteOptsToApiOpts(&VolumeWriteOptions{
		Logger: logger,
	})
	if apiOpts == nil {
		t.Fatal("expected write opts to map to api opts")
	}
	if apiOpts.Logger != logger {
		t.Fatalf("expected logger to be preserved, got %#v", apiOpts.Logger)
	}
}

func TestVolumeWriteOptsToApiOptsPreservesExplicitFalseDebug(t *testing.T) {
	apiOpts := volumeWriteOptsToApiOpts(&VolumeWriteOptions{
		Debug: boolPtr(false),
	})
	if apiOpts == nil {
		t.Fatal("expected write opts to map to api opts")
	}
	if apiOpts.Debug == nil || *apiOpts.Debug {
		t.Fatalf("expected explicit false debug to be preserved, got %#v", apiOpts.Debug)
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

func TestVolumeCreateDoesNotPersistHeadersIntoFutureInstanceCalls(t *testing.T) {
	createServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(VolumeAndToken{
			VolumeInfo: VolumeInfo{
				VolumeID: "vol-1",
				Name:     "test-volume",
			},
			Token: "token",
		}); err != nil {
			t.Fatalf("failed to encode create response: %v", err)
		}
	}))
	defer createServer.Close()

	var gotTestHeader string
	contentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumecontent/vol-1/path" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotTestHeader = r.Header.Get("X-Test")
		if err := json.NewEncoder(w).Encode(VolumeEntryStat{Name: "file.txt", Path: "/file.txt"}); err != nil {
			t.Fatalf("failed to encode info response: %v", err)
		}
	}))
	defer contentServer.Close()

	volume, err := Create(context.Background(), "test-volume", &ConnectionOpts{
		ApiKey: "e2b_0000000000000000000000000000000000000000",
		ApiUrl: createServer.URL,
		Headers: map[string]string{
			"X-Test": "base",
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	_, err = volume.GetInfo(context.Background(), "/file.txt", &VolumeApiOpts{
		ApiUrl: contentServer.URL,
	})
	if err != nil {
		t.Fatalf("GetInfo returned error: %v", err)
	}

	if gotTestHeader != "" {
		t.Fatalf("expected create-time header not to persist into instance calls, got %q", gotTestHeader)
	}
}

func TestVolumeCreateDoesNotPersistRequestTimeoutIntoFutureInstanceCalls(t *testing.T) {
	createServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(VolumeAndToken{
			VolumeInfo: VolumeInfo{
				VolumeID: "vol-1",
				Name:     "test-volume",
			},
			Token: "token",
		}); err != nil {
			t.Fatalf("failed to encode create response: %v", err)
		}
	}))
	defer createServer.Close()

	contentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		if err := json.NewEncoder(w).Encode(VolumeEntryStat{Name: "file.txt", Path: "/file.txt"}); err != nil {
			t.Fatalf("failed to encode info response: %v", err)
		}
	}))
	defer contentServer.Close()

	timeout := 25
	volume, err := Create(context.Background(), "test-volume", &ConnectionOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           createServer.URL,
		RequestTimeoutMs: &timeout,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	_, err = volume.GetInfo(context.Background(), "/file.txt", &VolumeApiOpts{
		ApiUrl: contentServer.URL,
	})
	if err != nil {
		t.Fatalf("expected instance call not to inherit create-time timeout, got %v", err)
	}
}
