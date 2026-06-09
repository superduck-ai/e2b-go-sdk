package template

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/api"
	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

func TestAssignTagsUsesJsTargetPayloadAndReturnsTagInfo(t *testing.T) {
	var body map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/templates/tags" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"buildID": "bld-123",
			"tags":    []string{"prod", "stable"},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := api.NewApiClient(&api.ClientConfig{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: 1000,
	})
	if err != nil {
		t.Fatalf("failed to create API client: %v", err)
	}

	info, err := assignTags(context.Background(), client, "tmpl:latest", []string{"prod", "stable"})
	if err != nil {
		t.Fatalf("expected assign tags to succeed, got %v", err)
	}
	if body["target"] != "tmpl:latest" {
		t.Fatalf("expected JS-style target field, got %#v", body)
	}
	if _, ok := body["templateName"]; ok {
		t.Fatalf("did not expect legacy templateName field, got %#v", body)
	}
	if info == nil || info.BuildID != "bld-123" || !reflect.DeepEqual(info.Tags, []string{"prod", "stable"}) {
		t.Fatalf("unexpected tag info: %#v", info)
	}
}

func TestAssignTagsAcceptsSingleTagLikeJsAndPython(t *testing.T) {
	var body map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"buildID": "bld-123",
			"tags":    []string{"prod"},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	info, err := AssignTags(context.Background(), "tmpl:latest", "prod", &ConnectionOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	})
	if err != nil {
		t.Fatalf("expected AssignTags to succeed, got %v", err)
	}
	if info == nil || !reflect.DeepEqual(info.Tags, []string{"prod"}) {
		t.Fatalf("unexpected tag info: %#v", info)
	}
	tags, ok := body["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "prod" {
		t.Fatalf("expected single tag to be normalized to array, got %#v", body)
	}
}

func TestRemoveTagsUsesJsNamePayload(t *testing.T) {
	var body map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := api.NewApiClient(&api.ClientConfig{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: 1000,
	})
	if err != nil {
		t.Fatalf("failed to create API client: %v", err)
	}

	if err := removeTags(context.Background(), client, "tmpl", []string{"old"}); err != nil {
		t.Fatalf("expected remove tags to succeed, got %v", err)
	}
	if body["name"] != "tmpl" {
		t.Fatalf("expected JS-style name field, got %#v", body)
	}
	if _, ok := body["templateName"]; ok {
		t.Fatalf("did not expect legacy templateName field, got %#v", body)
	}
}

func TestRemoveTagsAcceptsSingleTagLikeJsAndPython(t *testing.T) {
	var body map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := RemoveTags(context.Background(), "tmpl", "old", &ConnectionOpts{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	}); err != nil {
		t.Fatalf("expected RemoveTags to succeed, got %v", err)
	}
	tags, ok := body["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "old" {
		t.Fatalf("expected single tag to be normalized to array, got %#v", body)
	}
}

func TestTemplateTagsRejectUnsupportedTagsShape(t *testing.T) {
	_, err := AssignTags(context.Background(), "tmpl:latest", 123, nil)
	var templateErr *shared.TemplateError
	if !errors.As(err, &templateErr) {
		t.Fatalf("expected TemplateError, got %T %v", err, err)
	}

	err = RemoveTags(context.Background(), "tmpl", 123, nil)
	if !errors.As(err, &templateErr) {
		t.Fatalf("expected TemplateError, got %T %v", err, err)
	}
}

func TestGetBuildStatusFromAPIMapsJsBuildStatusShape(t *testing.T) {
	timestamp := time.Date(2026, 5, 26, 12, 34, 56, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/templates/tmpl-1/builds/bld-1/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("logsOffset"); got != "3" {
			t.Fatalf("expected logsOffset=3, got %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Fatalf("expected limit=100, got %q", got)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"buildID":    "bld-1",
			"templateID": "tmpl-1",
			"status":     "error",
			"logEntries": []map[string]any{
				{
					"timestamp": timestamp.Format(time.RFC3339),
					"level":     "info",
					"message":   "building",
				},
			},
			"logs": []string{"building"},
			"reason": map[string]any{
				"message": "step failed",
				"step":    "finalize",
				"logEntries": []map[string]any{
					{
						"timestamp": timestamp.Format(time.RFC3339),
						"level":     "error",
						"message":   "boom",
					},
				},
			},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := api.NewApiClient(&api.ClientConfig{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: 1000,
	})
	if err != nil {
		t.Fatalf("failed to create API client: %v", err)
	}

	status, err := getBuildStatusFromAPI(context.Background(), client, "tmpl-1", "bld-1", 3)
	if err != nil {
		t.Fatalf("expected build status to succeed, got %v", err)
	}
	if status.BuildID != "bld-1" || status.TemplateID != "tmpl-1" || status.Status != BuildStatusError {
		t.Fatalf("unexpected status envelope: %#v", status)
	}
	if len(status.LogEntries) != 1 || status.LogEntries[0].Message != "building" {
		t.Fatalf("expected mapped log entries, got %#v", status.LogEntries)
	}
	if !reflect.DeepEqual(status.Logs, []string{"building"}) {
		t.Fatalf("expected raw logs slice, got %#v", status.Logs)
	}
	if status.Reason == nil || status.Reason.Message != "step failed" || status.Reason.Step != "finalize" {
		t.Fatalf("expected mapped reason, got %#v", status.Reason)
	}
	if len(status.Reason.LogEntries) != 1 || status.Reason.LogEntries[0].Message != "boom" {
		t.Fatalf("expected mapped reason log entries, got %#v", status.Reason.LogEntries)
	}
}

func TestTemplateApisHonorCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	template := Template(nil).FromBaseImage()
	buildOpts := &BuildOptions{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	}

	if _, err := Build(ctx, template, "tmpl", buildOpts); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected Build to honor canceled context, got %T %v", err, err)
	}
	if _, err := BuildInBackground(ctx, template, "tmpl", buildOpts); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected BuildInBackground to honor canceled context, got %T %v", err, err)
	}
	if _, err := Exists(ctx, "tmpl", connectionOptsFromBuildOptions(buildOpts)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected Exists to honor canceled context, got %T %v", err, err)
	}
	if _, err := GetBuildStatus(ctx, &BuildInfo{TemplateID: "tmpl", BuildID: "bld"}, &GetBuildStatusOptions{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected GetBuildStatus to honor canceled context, got %T %v", err, err)
	}
	if _, err := AssignTags(ctx, "tmpl:latest", "stable", connectionOptsFromBuildOptions(buildOpts)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected AssignTags to honor canceled context, got %T %v", err, err)
	}
	if err := RemoveTags(ctx, "tmpl", "stable", connectionOptsFromBuildOptions(buildOpts)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected RemoveTags to honor canceled context, got %T %v", err, err)
	}
	if _, err := GetTags(ctx, "tmpl", connectionOptsFromBuildOptions(buildOpts)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected GetTags to honor canceled context, got %T %v", err, err)
	}
}

func TestTemplateApisHonorPreCanceledSignalContext(t *testing.T) {
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested <- struct{}{}
		t.Fatal("expected pre-canceled signal to prevent template request from being sent")
	}))
	defer server.Close()

	signal, cancel := context.WithCancel(context.Background())
	cancel()

	template := Template(nil).FromBaseImage()
	buildOpts := &BuildOptions{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
		Signal:           signal,
	}

	if _, err := Build(context.Background(), template, "tmpl", buildOpts); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected Build to honor pre-canceled signal, got %T %v", err, err)
	}
	if _, err := BuildInBackground(context.Background(), template, "tmpl", buildOpts); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected BuildInBackground to honor pre-canceled signal, got %T %v", err, err)
	}
	if _, err := Exists(context.Background(), "tmpl", connectionOptsFromBuildOptions(buildOpts)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected Exists to honor pre-canceled signal, got %T %v", err, err)
	}
	if _, err := GetBuildStatus(context.Background(), &BuildInfo{TemplateID: "tmpl", BuildID: "bld"}, &GetBuildStatusOptions{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
		Signal:           signal,
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected GetBuildStatus to honor pre-canceled signal, got %T %v", err, err)
	}
	if _, err := AssignTags(context.Background(), "tmpl:latest", "stable", connectionOptsFromBuildOptions(buildOpts)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected AssignTags to honor pre-canceled signal, got %T %v", err, err)
	}
	if err := RemoveTags(context.Background(), "tmpl", "stable", connectionOptsFromBuildOptions(buildOpts)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected RemoveTags to honor pre-canceled signal, got %T %v", err, err)
	}
	if _, err := GetTags(context.Background(), "tmpl", connectionOptsFromBuildOptions(buildOpts)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected GetTags to honor pre-canceled signal, got %T %v", err, err)
	}

	select {
	case <-requested:
		t.Fatal("unexpected template request despite pre-canceled signal")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestTemplateApisHonorInFlightCancellation(t *testing.T) {
	buildOpts := &BuildOptions{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		RequestTimeoutMs: intPtr(1000),
	}
	template := Template(nil).FromBaseImage()

	runCancellation := func(t *testing.T, wantPath string, invoke func(ctx context.Context, apiURL string) error) {
		t.Helper()

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
			done <- invoke(ctx, server.URL)
		}()

		select {
		case path := <-requestStarted:
			if path != wantPath {
				t.Fatalf("expected %s request, got %s", wantPath, path)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %s request to start", wantPath)
		}

		cancel()

		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected canceled context error, got %T %v", err, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %s cancellation", wantPath)
		}

		close(release)
	}

	t.Run("build", func(t *testing.T) {
		runCancellation(t, "/v3/templates", func(ctx context.Context, apiURL string) error {
			opts := *buildOpts
			opts.ApiUrl = apiURL
			_, err := Build(ctx, template, "tmpl", &opts)
			return err
		})
	})

	t.Run("build_in_background", func(t *testing.T) {
		runCancellation(t, "/v3/templates", func(ctx context.Context, apiURL string) error {
			opts := *buildOpts
			opts.ApiUrl = apiURL
			_, err := BuildInBackground(ctx, template, "tmpl", &opts)
			return err
		})
	})

	t.Run("exists", func(t *testing.T) {
		runCancellation(t, "/templates/aliases/tmpl", func(ctx context.Context, apiURL string) error {
			opts := *buildOpts
			opts.ApiUrl = apiURL
			_, err := Exists(ctx, "tmpl", connectionOptsFromBuildOptions(&opts))
			return err
		})
	})

	t.Run("get_build_status", func(t *testing.T) {
		runCancellation(t, "/templates/tmpl/builds/bld/status", func(ctx context.Context, apiURL string) error {
			_, err := GetBuildStatus(ctx, &BuildInfo{TemplateID: "tmpl", BuildID: "bld"}, &GetBuildStatusOptions{
				ApiKey:           "e2b_0000000000000000000000000000000000000000",
				ApiUrl:           apiURL,
				RequestTimeoutMs: intPtr(1000),
			})
			return err
		})
	})

	t.Run("assign_tags", func(t *testing.T) {
		runCancellation(t, "/templates/tags", func(ctx context.Context, apiURL string) error {
			opts := *buildOpts
			opts.ApiUrl = apiURL
			_, err := AssignTags(ctx, "tmpl:latest", "stable", connectionOptsFromBuildOptions(&opts))
			return err
		})
	})

	t.Run("remove_tags", func(t *testing.T) {
		runCancellation(t, "/templates/tags", func(ctx context.Context, apiURL string) error {
			opts := *buildOpts
			opts.ApiUrl = apiURL
			return RemoveTags(ctx, "tmpl", "stable", connectionOptsFromBuildOptions(&opts))
		})
	})

	t.Run("get_tags", func(t *testing.T) {
		runCancellation(t, "/templates/tmpl/tags", func(ctx context.Context, apiURL string) error {
			opts := *buildOpts
			opts.ApiUrl = apiURL
			_, err := GetTags(ctx, "tmpl", connectionOptsFromBuildOptions(&opts))
			return err
		})
	})
}

func TestTemplateApisHonorSignalContext(t *testing.T) {
	buildOpts := &BuildOptions{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		RequestTimeoutMs: intPtr(1000),
	}
	template := Template(nil).FromBaseImage()

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

	t.Run("build", func(t *testing.T) {
		runSignalCancellation(t, func(signal context.Context, apiURL string) error {
			opts := *buildOpts
			opts.ApiUrl = apiURL
			opts.Signal = signal
			_, err := Build(context.Background(), template, "tmpl", &opts)
			return err
		})
	})

	t.Run("build_in_background", func(t *testing.T) {
		runSignalCancellation(t, func(signal context.Context, apiURL string) error {
			opts := *buildOpts
			opts.ApiUrl = apiURL
			opts.Signal = signal
			_, err := BuildInBackground(context.Background(), template, "tmpl", &opts)
			return err
		})
	})

	t.Run("exists", func(t *testing.T) {
		runSignalCancellation(t, func(signal context.Context, apiURL string) error {
			opts := *buildOpts
			opts.ApiUrl = apiURL
			opts.Signal = signal
			_, err := Exists(context.Background(), "tmpl", connectionOptsFromBuildOptions(&opts))
			return err
		})
	})

	t.Run("get_build_status", func(t *testing.T) {
		runSignalCancellation(t, func(signal context.Context, apiURL string) error {
			_, err := GetBuildStatus(context.Background(), &BuildInfo{TemplateID: "tmpl", BuildID: "bld"}, &GetBuildStatusOptions{
				ApiKey:           "e2b_0000000000000000000000000000000000000000",
				ApiUrl:           apiURL,
				RequestTimeoutMs: intPtr(1000),
				Signal:           signal,
			})
			return err
		})
	})
}

func TestBuildInfoIncludesDeprecatedAliasField(t *testing.T) {
	info := &BuildInfo{
		Alias:      "tmpl:v1",
		Name:       "tmpl:v1",
		Tags:       []string{"stable"},
		TemplateID: "tmpl-1",
		BuildID:    "bld-1",
	}

	if info.Alias != info.Name {
		t.Fatalf("expected deprecated alias to mirror name, got %#v", info)
	}
}

func TestNormalizeBuildNameMatchesJsAndPython(t *testing.T) {
	cases := []struct {
		name string
		opts *BuildOptions
		want string
	}{
		{name: "my-template:v1.0", want: "my-template:v1.0"},
		{name: "my-template", want: "my-template"},
		{opts: &BuildOptions{BasicBuildOptions: BasicBuildOptions{Alias: "legacy-template"}}, want: "legacy-template"},
		{name: "from-name", opts: &BuildOptions{BasicBuildOptions: BasicBuildOptions{Alias: "from-alias"}}, want: "from-name"},
	}

	for _, tc := range cases {
		got, err := normalizeBuildName(tc.name, tc.opts)
		if err != nil {
			t.Fatalf("normalizeBuildName(%q, %#v) returned error: %v", tc.name, tc.opts, err)
		}
		if got != tc.want {
			t.Fatalf("normalizeBuildName(%q, %#v) = %q, want %q", tc.name, tc.opts, got, tc.want)
		}
	}
}

func TestBuildInBackgroundNormalizesLegacyAliasWithoutDroppingOptions(t *testing.T) {
	var createBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/templates":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("failed to decode create body: %v", err)
			}
			if err := json.NewEncoder(w).Encode(map[string]any{
				"templateID": "tmpl-legacy",
				"buildID":    "bld-legacy",
			}); err != nil {
				t.Fatalf("failed to encode build request response: %v", err)
			}
		case "/v2/templates/tmpl-legacy/builds/bld-legacy":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	info, err := BuildInBackground(context.Background(), Template(nil).FromBaseImage(), "", &BuildOptions{
		BasicBuildOptions: BasicBuildOptions{
			Alias:    "legacy-template",
			Tags:     []string{"stable"},
			CpuCount: 4,
			MemoryMB: 1024,
		},
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	})
	if err != nil {
		t.Fatalf("BuildInBackground with legacy alias returned error: %v", err)
	}
	if info == nil || info.Name != "legacy-template" || info.Alias != "legacy-template" {
		t.Fatalf("unexpected build info: %#v", info)
	}
	if createBody["name"] != "legacy-template" {
		t.Fatalf("expected legacy alias to be normalized to name, got %#v", createBody)
	}
	if _, ok := createBody["alias"]; ok {
		t.Fatalf("did not expect alias to be sent in build request, got %#v", createBody)
	}
	if got := int(createBody["cpuCount"].(float64)); got != 4 {
		t.Fatalf("expected cpuCount to be preserved, got %#v", createBody)
	}
	if got := int(createBody["memoryMB"].(float64)); got != 1024 {
		t.Fatalf("expected memoryMB to be preserved, got %#v", createBody)
	}
	tags, ok := createBody["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "stable" {
		t.Fatalf("expected tags to be preserved, got %#v", createBody)
	}
}

func TestTemplateApiClientsUseEnvFallbackAndDefaultTimeoutLikeJsAndPython(t *testing.T) {
	const apiKey = "e2b_0000000000000000000000000000000000000000"
	const apiURL = "http://127.0.0.1:8080"

	t.Setenv("E2B_API_KEY", apiKey)
	t.Setenv("E2B_API_URL", apiURL)

	cases := []struct {
		name      string
		newClient func() (*api.ApiClient, error)
	}{
		{
			name: "build",
			newClient: func() (*api.ApiClient, error) {
				return newApiClientFromBuildOptions(&BuildOptions{})
			},
		},
		{
			name: "status",
			newClient: func() (*api.ApiClient, error) {
				return newApiClientFromStatusOptions(&GetBuildStatusOptions{})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := tc.newClient()
			if err != nil {
				t.Fatalf("failed to create API client: %v", err)
			}
			if client.BaseUrl != apiURL {
				t.Fatalf("expected API URL from env, got %q", client.BaseUrl)
			}
			if got := client.Headers["X-API-Key"]; got != apiKey {
				t.Fatalf("expected API key from env, got %q", got)
			}
			if got := client.HttpClient.Timeout; got != 60*time.Second {
				t.Fatalf("expected default request timeout 60s, got %s", got)
			}
		})
	}
}

func TestTemplateApiClientsPreserveExplicitZeroRequestTimeoutLikeJsAndPython(t *testing.T) {
	const apiKey = "e2b_0000000000000000000000000000000000000000"
	const apiURL = "http://127.0.0.1:8080"

	t.Setenv("E2B_API_KEY", apiKey)
	t.Setenv("E2B_API_URL", apiURL)

	zero := 0
	buildClient, err := newApiClientFromBuildOptions(&BuildOptions{RequestTimeoutMs: &zero})
	if err != nil {
		t.Fatalf("failed to create build API client: %v", err)
	}
	if got := buildClient.HttpClient.Timeout; got != 0 {
		t.Fatalf("expected explicit zero timeout for build client, got %s", got)
	}

	statusClient, err := newApiClientFromStatusOptions(&GetBuildStatusOptions{RequestTimeoutMs: &zero})
	if err != nil {
		t.Fatalf("failed to create status API client: %v", err)
	}
	if got := statusClient.HttpClient.Timeout; got != 0 {
		t.Fatalf("expected explicit zero timeout for status client, got %s", got)
	}
}

func TestBuildInBackgroundUsesEnvFallbackAndDefaultsMemoryLikeJsAndPython(t *testing.T) {
	const apiKey = "e2b_0000000000000000000000000000000000000000"

	var createBody requestBuildRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/templates":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("failed to decode create body: %v", err)
			}
			if err := json.NewEncoder(w).Encode(requestBuildResponse{
				TemplateID: "tmpl-env",
				BuildID:    "bld-env",
			}); err != nil {
				t.Fatalf("failed to encode build request response: %v", err)
			}
		case "/v2/templates/tmpl-env/builds/bld-env":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("E2B_API_KEY", apiKey)
	t.Setenv("E2B_API_URL", server.URL)

	info, err := BuildInBackground(context.Background(), Template(nil).FromBaseImage(), "tmpl:v1", nil)
	if err != nil {
		t.Fatalf("BuildInBackground returned error: %v", err)
	}
	if info == nil || info.TemplateID != "tmpl-env" || info.BuildID != "bld-env" {
		t.Fatalf("unexpected build info: %#v", info)
	}
	if createBody.Name != "tmpl:v1" {
		t.Fatalf("expected build name to be preserved, got %#v", createBody)
	}
	if createBody.CpuCount != 2 {
		t.Fatalf("expected default cpuCount 2, got %#v", createBody)
	}
	if createBody.MemoryMB != 1024 {
		t.Fatalf("expected default memoryMB 1024, got %#v", createBody)
	}
}

func TestBuildUsesRequestBuildResponseTagsAndEmitsJsLifecycleLogs(t *testing.T) {
	var createBody requestBuildRequest
	var entries []LogEntry

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/templates":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("failed to decode create body: %v", err)
			}
			if err := json.NewEncoder(w).Encode(requestBuildResponse{
				TemplateID: "tmpl-1",
				BuildID:    "bld-1",
				Tags:       []string{"stable", "released"},
			}); err != nil {
				t.Fatalf("failed to encode build request response: %v", err)
			}
		case "/v2/templates/tmpl-1/builds/bld-1":
			w.WriteHeader(http.StatusOK)
		case "/templates/tmpl-1/builds/bld-1/status":
			if err := json.NewEncoder(w).Encode(buildStatusAPIResponse{
				TemplateID: "tmpl-1",
				BuildID:    "bld-1",
				Status:     BuildStatusReady,
			}); err != nil {
				t.Fatalf("failed to encode status response: %v", err)
			}
		case "/templates/tags":
			t.Fatal("did not expect Build to send extra assignTags request")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	info, err := Build(context.Background(), Template(nil).FromBaseImage(), "tmpl", &BuildOptions{
		BasicBuildOptions: BasicBuildOptions{
			Tags: []string{"stable"},
			OnBuildLogs: func(entry *LogEntry) {
				entries = append(entries, *entry)
			},
		},
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if got := info.Tags; !reflect.DeepEqual(got, []string{"stable", "released"}) {
		t.Fatalf("expected BuildInfo tags from requestBuild response, got %#v", got)
	}
	if createBody.Name != "tmpl" {
		t.Fatalf("expected build name tmpl, got %#v", createBody)
	}
	if got := createBody.Tags; !reflect.DeepEqual(got, []string{"stable"}) {
		t.Fatalf("expected requestBuild tags to be preserved, got %#v", got)
	}

	gotMessages := make([]string, 0, len(entries))
	gotLevels := make([]LogEntryLevel, 0, len(entries))
	for _, entry := range entries {
		gotMessages = append(gotMessages, entry.Message)
		gotLevels = append(gotLevels, entry.Level)
	}

	wantMessages := []string{
		"Build started",
		"Requesting build for template: tmpl with tags stable",
		"Template created with ID: tmpl-1, Build ID: bld-1",
		"All file uploads completed",
		"Starting building...",
		"Waiting for logs...",
		"Build finished",
	}
	wantLevels := []LogEntryLevel{
		LogLevelDebug,
		LogLevelInfo,
		LogLevelInfo,
		LogLevelInfo,
		LogLevelInfo,
		LogLevelInfo,
		LogLevelDebug,
	}
	if !reflect.DeepEqual(gotMessages, wantMessages) {
		t.Fatalf("unexpected lifecycle log messages:\n got %#v\nwant %#v", gotMessages, wantMessages)
	}
	if !reflect.DeepEqual(gotLevels, wantLevels) {
		t.Fatalf("unexpected lifecycle log levels:\n got %#v\nwant %#v", gotLevels, wantLevels)
	}
}

func TestBuildInBackgroundWithLoggerEmitsJsUploadLifecycleLogs(t *testing.T) {
	var entries []LogEntry
	contextDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contextDir, "src"), []byte("print('hi')\n"), 0o644); err != nil {
		t.Fatalf("failed to write source fixture: %v", err)
	}
	expectedHash, err := calculateFilesHash("src", "/app/src", contextDir, nil, false)
	if err != nil {
		t.Fatalf("failed to calculate expected hash: %v", err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/templates":
			if err := json.NewEncoder(w).Encode(requestBuildResponse{
				TemplateID: "tmpl-1",
				BuildID:    "bld-1",
			}); err != nil {
				t.Fatalf("failed to encode build request response: %v", err)
			}
		case "/templates/tmpl-1/files/" + expectedHash:
			if err := json.NewEncoder(w).Encode(map[string]any{
				"present": false,
				"url":     server.URL + "/upload",
			}); err != nil {
				t.Fatalf("failed to encode upload link response: %v", err)
			}
		case "/upload":
			w.WriteHeader(http.StatusOK)
		case "/v2/templates/tmpl-1/builds/bld-1":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	_, err = BuildInBackground(context.Background(), Template(&TemplateOptions{FileContextPath: contextDir}).FromBaseImage().Copy("src", "/app/src", nil), "tmpl:v1", &BuildOptions{
		BasicBuildOptions: BasicBuildOptions{
			OnBuildLogs: func(entry *LogEntry) {
				entries = append(entries, *entry)
			},
		},
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	})
	if err != nil {
		t.Fatalf("BuildInBackground returned error: %v", err)
	}

	gotMessages := make([]string, 0, len(entries))
	for _, entry := range entries {
		gotMessages = append(gotMessages, entry.Message)
	}

	wantMessages := []string{
		"Requesting build for template: tmpl:v1",
		"Template created with ID: tmpl-1, Build ID: bld-1",
		"Uploaded 'src'",
		"All file uploads completed",
		"Starting building...",
	}
	if !reflect.DeepEqual(gotMessages, wantMessages) {
		t.Fatalf("unexpected background-build log messages:\n got %#v\nwant %#v", gotMessages, wantMessages)
	}
	for _, unexpected := range []string{"Build started", "Waiting for logs...", "Build finished"} {
		if slicesContainsString(gotMessages, unexpected) {
			t.Fatalf("did not expect background build to emit %q, got %#v", unexpected, gotMessages)
		}
	}
}

func TestBuildWithoutLoggerDoesNotUseDefaultBuildLoggerLikeJsAndPython(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/templates":
			if err := json.NewEncoder(w).Encode(requestBuildResponse{
				TemplateID: "tmpl-1",
				BuildID:    "bld-1",
			}); err != nil {
				t.Fatalf("failed to encode build request response: %v", err)
			}
		case "/v2/templates/tmpl-1/builds/bld-1":
			w.WriteHeader(http.StatusOK)
		case "/templates/tmpl-1/builds/bld-1/status":
			if err := json.NewEncoder(w).Encode(buildStatusAPIResponse{
				TemplateID: "tmpl-1",
				BuildID:    "bld-1",
				Status:     BuildStatusReady,
			}); err != nil {
				t.Fatalf("failed to encode status response: %v", err)
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output := captureStdout(t, func() {
		if _, err := Build(context.Background(), Template(nil).FromBaseImage(), "tmpl:v1", &BuildOptions{
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			RequestTimeoutMs: intPtr(1000),
		}); err != nil {
			t.Fatalf("Build returned error: %v", err)
		}
	})
	if output != "" {
		t.Fatalf("expected Build without OnBuildLogs to stay silent, got %q", output)
	}
}

func TestDefaultBuildLoggerIgnoresDebugByDefaultLikeJsAndPython(t *testing.T) {
	logger := DefaultBuildLogger()
	output := captureStdout(t, func() {
		logger(&LogEntry{
			Timestamp: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
			Level:     LogLevelDebug,
			Message:   "Build started",
		})
		logger(&LogEntry{
			Timestamp: time.Date(2026, 5, 30, 12, 0, 1, 0, time.UTC),
			Level:     LogLevelInfo,
			Message:   "Uploaded 'src'\n",
		})
	})

	if strings.Contains(output, "Build started") {
		t.Fatalf("did not expect default logger to print debug messages, got %q", output)
	}
	if output != "[info] Uploaded 'src'\n" {
		t.Fatalf("unexpected default logger output: %q", output)
	}
}

func TestTemplateLogEntriesStripAnsiLikeJsAndPython(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(buildStatusAPIResponse{
			TemplateID: "tmpl-1",
			BuildID:    "bld-1",
			Status:     BuildStatusError,
			LogEntries: []buildLogEntryAPIResponse{{
				Timestamp: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
				Level:     LogLevelInfo,
				Message:   "\x1b[31mred\x1b[0m plain",
			}},
			Reason: &buildStatusReasonAPIResponse{
				Message: "failed",
				Step:    "finalize",
				LogEntries: []buildLogEntryAPIResponse{{
					Timestamp: time.Date(2026, 5, 30, 12, 0, 1, 0, time.UTC),
					Level:     LogLevelError,
					Message:   "\x1b[33mboom\x1b[0m",
				}},
			},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := api.NewApiClient(&api.ClientConfig{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: 1000,
	})
	if err != nil {
		t.Fatalf("failed to create API client: %v", err)
	}

	status, err := getBuildStatusFromAPI(context.Background(), client, "tmpl-1", "bld-1", 0)
	if err != nil {
		t.Fatalf("expected build status to succeed, got %v", err)
	}
	if got := status.LogEntries[0].Message; got != "red plain" {
		t.Fatalf("expected ANSI-stripped build log entry, got %q", got)
	}
	if got := status.Reason.LogEntries[0].Message; got != "boom" {
		t.Fatalf("expected ANSI-stripped reason log entry, got %q", got)
	}
}

func TestLogEntryStringStripsAnsiAndMatchesJsFormat(t *testing.T) {
	entry := &LogEntry{
		Timestamp: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
		Level:     LogLevelInfo,
		Message:   "\x1b[32mready\x1b[0m",
	}
	if got, want := entry.String(), "[2026-05-30T12:00:00Z] [info] ready"; got != want {
		t.Fatalf("unexpected LogEntry.String() output: got %q want %q", got, want)
	}
}

func TestBuildRejectsMissingNameBeforeApiConfig(t *testing.T) {
	_, err := BuildInBackground(context.Background(), Template(nil), "", nil)
	var templateErr *shared.TemplateError
	if !errors.As(err, &templateErr) {
		t.Fatalf("expected TemplateError, got %T %v", err, err)
	}
	if templateErr.Message != "Name must be provided" {
		t.Fatalf("unexpected TemplateError message: %q", templateErr.Message)
	}

	for _, opts := range []*BuildOptions{{}, {BasicBuildOptions: BasicBuildOptions{Alias: ""}}} {
		_, err := normalizeBuildName("", opts)
		var templateErr *shared.TemplateError
		if !errors.As(err, &templateErr) {
			t.Fatalf("expected TemplateError for opts %#v, got %T %v", opts, err, err)
		}
	}
}

func TestInstructionTypeAndArgsMatchJsShape(t *testing.T) {
	if InstructionCopy != "COPY" || InstructionEnv != "ENV" || InstructionRun != "RUN" ||
		InstructionWorkdir != "WORKDIR" || InstructionUser != "USER" {
		t.Fatalf("expected JS-style instruction type values, got %q %q %q %q %q",
			InstructionCopy, InstructionEnv, InstructionRun, InstructionWorkdir, InstructionUser)
	}

	instruction := Template(nil).Copy("src", "dst", nil).SetEnvs(map[string]string{"KEY": "VALUE"}).instructionsList()
	if len(instruction) < 2 {
		t.Fatalf("expected instructions to be recorded, got %#v", instruction)
	}
	if !reflect.DeepEqual(instruction[0].Args, []string{"src", "dst", "", ""}) {
		t.Fatalf("expected COPY args to be tokenized, got %#v", instruction[0].Args)
	}
	if !reflect.DeepEqual(instruction[1].Args, []string{"KEY", "VALUE"}) {
		t.Fatalf("expected ENV args to be tokenized, got %#v", instruction[1].Args)
	}
}

func TestTemplateSerializationMatchesJsStyleHelpers(t *testing.T) {
	template := Template(nil).
		FromPythonImage("3.12").
		Copy("src", "/app/src", nil).
		SetEnvs(map[string]string{"KEY": "VALUE"}).
		SetWorkdir("/app").
		SetUser("user").
		SetStartCmd("python main.py", nil)

	jsonText, err := ToJSON(template)
	if err != nil {
		t.Fatalf("ToJSON returned error: %v", err)
	}
	expectedJSONParts := []string{
		`"fromImage": "python:3.12"`,
		`"startCmd": "python main.py"`,
		`"steps": [`,
		`"type": "COPY"`,
		`"args": [`,
		`"/app/src"`,
	}
	for _, part := range expectedJSONParts {
		if !strings.Contains(jsonText, part) {
			t.Fatalf("expected ToJSON output to contain %q, got %s", part, jsonText)
		}
	}

	dockerfile, err := ToDockerfile(template)
	if err != nil {
		t.Fatalf("ToDockerfile returned error: %v", err)
	}
	expectedDockerfile := "FROM python:3.12\nCOPY src /app/src\nENV KEY=VALUE\nWORKDIR /app\nUSER user\nENTRYPOINT python main.py\n"
	if dockerfile != expectedDockerfile {
		t.Fatalf("unexpected dockerfile output:\n%s", dockerfile)
	}
}

func TestToDockerfileGroupsEnvInstructionsLikeJsAndPython(t *testing.T) {
	template := Template(nil).
		FromUbuntuImage("24.04").
		SetEnvs(map[string]string{"NODE_ENV": "production", "PORT": "8080"}).
		SetEnvs(map[string]string{"DEBUG": "false"})

	dockerfile, err := ToDockerfile(template)
	if err != nil {
		t.Fatalf("ToDockerfile returned error: %v", err)
	}

	expected := "FROM ubuntu:24.04\nENV NODE_ENV=production PORT=8080\nENV DEBUG=false\n"
	if dockerfile != expected {
		t.Fatalf("unexpected dockerfile output:\n%s", dockerfile)
	}
}

func TestUploadFileSetsContentLengthAndAvoidsChunkedEncoding(t *testing.T) {
	var headers http.Header
	var bodyLength int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read upload body: %v", err)
		}
		bodyLength = len(data)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := uploadFile(context.Background(), server.URL, []byte("archive bytes"), ""); err != nil {
		t.Fatalf("uploadFile returned error: %v", err)
	}

	contentLength := headers.Get("Content-Length")
	if contentLength == "" {
		t.Fatalf("expected Content-Length header, got %#v", headers)
	}
	if contentLength != strconv.Itoa(bodyLength) {
		t.Fatalf("expected Content-Length %d, got %q", bodyLength, contentLength)
	}
	if transferEncoding := headers.Get("Transfer-Encoding"); strings.Contains(strings.ToLower(transferEncoding), "chunked") {
		t.Fatalf("did not expect chunked transfer encoding, got %q", transferEncoding)
	}
}

func TestSetReadyCmdAcceptsStringLikeJs(t *testing.T) {
	template := Template(nil).SetReadyCmd("curl http://localhost:8000/health")

	serialized := template.serialize()
	if serialized.ReadyCmd != "curl http://localhost:8000/health" {
		t.Fatalf("expected string ready command to be serialized, got %#v", serialized)
	}
}

func TestReadyCmdHelpersMatchCurrentTs(t *testing.T) {
	cases := []struct {
		name string
		got  *ReadyCmd
		want string
	}{
		{name: "port", got: WaitForPort(8000), want: "ss -tuln | grep :8000"},
		{name: "url", got: WaitForURL("http://localhost:3000/health", 201), want: `curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/health | grep -q "201"`},
		{name: "process", got: WaitForProcess("nginx"), want: "pgrep nginx > /dev/null"},
		{name: "file", got: WaitForFile("/tmp/ready"), want: "[ -f /tmp/ready ]"},
		{name: "timeout-minimum", got: WaitForTimeout(500), want: "sleep 1"},
		{name: "timeout-floor", got: WaitForTimeout(2500), want: "sleep 2"},
	}

	for _, tc := range cases {
		if tc.got.GetCmd() != tc.want {
			t.Fatalf("%s ready command mismatch: got %q want %q", tc.name, tc.got.GetCmd(), tc.want)
		}
	}
}

func TestFromDockerfileAcceptsFilePathLikeJs(t *testing.T) {
	contextDir := t.TempDir()
	dockerfilePath := filepath.Join(contextDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM python:3.12\nWORKDIR /app\n"), 0o644); err != nil {
		t.Fatalf("failed to write Dockerfile fixture: %v", err)
	}

	template := Template(&TemplateOptions{FileContextPath: contextDir}).FromDockerfile("Dockerfile")
	serialized := template.serialize()
	if serialized.FromImage != "python:3.12" {
		t.Fatalf("expected Dockerfile path to set base image, got %#v", serialized)
	}
	if len(serialized.Steps) != 4 ||
		serialized.Steps[0].Type != InstructionUser ||
		serialized.Steps[0].Args[0] != "root" ||
		serialized.Steps[1].Type != InstructionWorkdir ||
		serialized.Steps[1].Args[0] != "/" ||
		serialized.Steps[2].Type != InstructionWorkdir ||
		serialized.Steps[2].Args[0] != "/app" ||
		serialized.Steps[3].Type != InstructionUser ||
		serialized.Steps[3].Args[0] != "user" {
		t.Fatalf("expected Dockerfile path to parse steps, got %#v", serialized.Steps)
	}
}

func TestTemplateDefaultsFileContextPathToCallerDirectory(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(workingDir)
	})

	callerDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get temp cwd: %v", err)
	}
	if callerDir != tmpDir {
		callerInfo, err := os.Stat(callerDir)
		if err != nil {
			t.Fatalf("failed to stat caller dir: %v", err)
		}
		tmpInfo, err := os.Stat(tmpDir)
		if err != nil {
			t.Fatalf("failed to stat temp dir: %v", err)
		}
		if !os.SameFile(callerInfo, tmpInfo) {
			t.Fatalf("expected cwd to change to temp dir %q, got %q", tmpDir, callerDir)
		}
	}

	template := Template(nil)
	if template.fileContextPath() == tmpDir {
		t.Fatalf("expected default file context path to come from caller directory, not cwd %q", tmpDir)
	}

	wantCallerDir := filepath.Dir(filepath.Clean(filepath.Join(workingDir, "template_alignment_test.go")))
	if template.fileContextPath() != wantCallerDir {
		t.Fatalf("expected default file context path %q, got %q", wantCallerDir, template.fileContextPath())
	}
}

func TestFromDockerfileUsesCallerDirectoryWhenFileContextPathOmitted(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	fixturePath := filepath.Join(workingDir, "CallerDockerfile.testfixture")
	if err := os.WriteFile(fixturePath, []byte("FROM node:24\nWORKDIR /caller\n"), 0o644); err != nil {
		t.Fatalf("failed to write caller Dockerfile fixture: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(fixturePath)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(workingDir)
	})

	template := Template(nil).FromDockerfile(filepath.Base(fixturePath))
	serialized := template.serialize()
	if serialized.FromImage != "node:24" {
		t.Fatalf("expected caller-relative Dockerfile path to set base image, got %#v", serialized)
	}
	if len(serialized.Steps) < 3 || serialized.Steps[2].Type != InstructionWorkdir || serialized.Steps[2].Args[0] != "/caller" {
		t.Fatalf("expected caller-relative Dockerfile WORKDIR to be parsed, got %#v", serialized.Steps)
	}
}

func TestFromDockerfileMatchesJsAndPythonInstructionOrder(t *testing.T) {
	dockerfile := `FROM node:24
WORKDIR /app
COPY package.json .
RUN npm install
ENTRYPOINT ["sleep", "20"]`

	template := Template(nil).FromDockerfile(dockerfile)
	instructions := template.instructionsList()

	if template.baseImage != "node:24" {
		t.Fatalf("expected base image node:24, got %q", template.baseImage)
	}
	if len(instructions) != 6 {
		t.Fatalf("expected 6 instructions, got %#v", instructions)
	}

	expected := []Instruction{
		{Type: InstructionUser, Args: []string{"root"}},
		{Type: InstructionWorkdir, Args: []string{"/"}},
		{Type: InstructionWorkdir, Args: []string{"/app"}},
		{Type: InstructionCopy, Args: []string{"package.json", ".", "", ""}},
		{Type: InstructionRun, Args: []string{"npm install"}},
		{Type: InstructionUser, Args: []string{"user"}},
	}
	for i, want := range expected {
		if instructions[i].Type != want.Type || !reflect.DeepEqual(instructions[i].Args, want.Args) {
			t.Fatalf("instruction %d mismatch: got %#v want %#v", i, instructions[i], want)
		}
	}
	if template.startCmd != "sleep 20" {
		t.Fatalf("expected JSON ENTRYPOINT to become shell command, got %q", template.startCmd)
	}
	if template.readyCmd != "sleep 20" {
		t.Fatalf("expected Dockerfile start command to get 20s ready timeout, got %q", template.readyCmd)
	}
}

func TestFromDockerfileQuotesJsonEntrypointArgsWithSpaces(t *testing.T) {
	template := Template(nil).FromDockerfile(`FROM python:3.12
ENTRYPOINT ["python", "-c", "print(\"hi there\")"]`)

	expected := `python -c 'print("hi there")'`
	if template.startCmd != expected {
		t.Fatalf("expected JSON ENTRYPOINT args to be shell-quoted, got %q", template.startCmd)
	}
}

func TestFromDockerfileAppliesE2BDefaultsLikeJsAndPython(t *testing.T) {
	template := Template(nil).FromDockerfile("FROM node:24")
	instructions := template.instructionsList()

	if len(instructions) < 2 {
		t.Fatalf("expected default user/workdir instructions, got %#v", instructions)
	}
	defaultUser := instructions[len(instructions)-2]
	defaultWorkdir := instructions[len(instructions)-1]
	if defaultUser.Type != InstructionUser || defaultUser.Args[0] != "user" {
		t.Fatalf("expected default USER user, got %#v", defaultUser)
	}
	if defaultWorkdir.Type != InstructionWorkdir || defaultWorkdir.Args[0] != "/home/user" {
		t.Fatalf("expected default WORKDIR /home/user, got %#v", defaultWorkdir)
	}
}

func TestFromDockerfileKeepsCustomUserAndWorkdirLikeJsAndPython(t *testing.T) {
	template := Template(nil).FromDockerfile("FROM node:24\nUSER mish\nWORKDIR /home/mish")
	instructions := template.instructionsList()

	if len(instructions) < 2 {
		t.Fatalf("expected custom user/workdir instructions, got %#v", instructions)
	}
	customUser := instructions[len(instructions)-2]
	customWorkdir := instructions[len(instructions)-1]
	if customUser.Type != InstructionUser || customUser.Args[0] != "mish" {
		t.Fatalf("expected custom USER mish, got %#v", customUser)
	}
	if customWorkdir.Type != InstructionWorkdir || customWorkdir.Args[0] != "/home/mish" {
		t.Fatalf("expected custom WORKDIR /home/mish, got %#v", customWorkdir)
	}
}

func TestFromDockerfileParsesCopyChownLikeJsAndPython(t *testing.T) {
	dockerfile := `FROM node:24
COPY --chown=myuser:mygroup app.js /app/
COPY --chown=anotheruser config.json /config/`

	instructions := Template(nil).FromDockerfile(dockerfile).instructionsList()
	if len(instructions) < 4 {
		t.Fatalf("expected COPY instructions after Docker defaults, got %#v", instructions)
	}

	copyInstruction1 := instructions[2]
	if copyInstruction1.Type != InstructionCopy ||
		!reflect.DeepEqual(copyInstruction1.Args, []string{"app.js", "/app/", "myuser:mygroup", ""}) {
		t.Fatalf("unexpected first COPY instruction: %#v", copyInstruction1)
	}

	copyInstruction2 := instructions[3]
	if copyInstruction2.Type != InstructionCopy ||
		!reflect.DeepEqual(copyInstruction2.Args, []string{"config.json", "/config/", "anotheruser", ""}) {
		t.Fatalf("unexpected second COPY instruction: %#v", copyInstruction2)
	}
}

func TestFromDockerfileExpandsMultiSourceCopyLikeJsAndPython(t *testing.T) {
	dockerfile := `FROM node:24
COPY file1.txt file2.txt file3.txt /dest/`

	instructions := Template(nil).FromDockerfile(dockerfile).instructionsList()
	var copyInstructions []Instruction
	for _, instruction := range instructions {
		if instruction.Type == InstructionCopy {
			copyInstructions = append(copyInstructions, instruction)
		}
	}

	if len(copyInstructions) != 3 {
		t.Fatalf("expected 3 COPY instructions for multi-source COPY, got %#v", copyInstructions)
	}
	expected := []Instruction{
		{Type: InstructionCopy, Args: []string{"file1.txt", "/dest/", "", ""}},
		{Type: InstructionCopy, Args: []string{"file2.txt", "/dest/", "", ""}},
		{Type: InstructionCopy, Args: []string{"file3.txt", "/dest/", "", ""}},
	}
	if !reflect.DeepEqual(copyInstructions, expected) {
		t.Fatalf("unexpected multi-source COPY expansion:\n got %#v\nwant %#v", copyInstructions, expected)
	}
}

func TestFromDockerfileExpandsMultiSourceCopyChownLikeJsAndPython(t *testing.T) {
	dockerfile := `FROM node:24
COPY --chown=myuser:mygroup pkg.json pkg-lock.json /app/`

	instructions := Template(nil).FromDockerfile(dockerfile).instructionsList()
	var copyInstructions []Instruction
	for _, instruction := range instructions {
		if instruction.Type == InstructionCopy {
			copyInstructions = append(copyInstructions, instruction)
		}
	}

	if len(copyInstructions) != 2 {
		t.Fatalf("expected 2 COPY instructions for multi-source COPY --chown, got %#v", copyInstructions)
	}
	expected := []Instruction{
		{Type: InstructionCopy, Args: []string{"pkg.json", "/app/", "myuser:mygroup", ""}},
		{Type: InstructionCopy, Args: []string{"pkg-lock.json", "/app/", "myuser:mygroup", ""}},
	}
	if !reflect.DeepEqual(copyInstructions, expected) {
		t.Fatalf("unexpected multi-source COPY --chown expansion:\n got %#v\nwant %#v", copyInstructions, expected)
	}
}

func TestFromDockerfileRejectsMissingFromLikeJsAndPython(t *testing.T) {
	assertBuildPanic(t, "Dockerfile must contain a FROM instruction", func() {
		Template(nil).FromDockerfile("RUN echo hi")
	})
}

func TestFromDockerfileRejectsMultiStageLikeJsAndPython(t *testing.T) {
	assertBuildPanic(t, "Multi-stage Dockerfiles are not supported", func() {
		Template(nil).FromDockerfile("FROM node:24\nFROM python:3.12")
	})
}

func TestFromDockerfileParsesEnvPairsLikeJsAndPython(t *testing.T) {
	instructions := Template(nil).FromDockerfile("FROM node:24\nENV A=1 B=2").instructionsList()
	if len(instructions) < 3 {
		t.Fatalf("expected ENV instruction after Docker defaults, got %#v", instructions)
	}
	envInstruction := instructions[2]
	if envInstruction.Type != InstructionEnv || !reflect.DeepEqual(envInstruction.Args, []string{"A", "1", "B", "2"}) {
		t.Fatalf("unexpected ENV instruction: %#v", envInstruction)
	}
}

func TestFromDockerfileParsesArgInstructionsLikeJsAndPython(t *testing.T) {
	instructions := Template(nil).FromDockerfile("FROM node:24\nARG FOO\nARG BAR=baz").instructionsList()
	if len(instructions) < 4 {
		t.Fatalf("expected ARG instructions after Docker defaults, got %#v", instructions)
	}
	firstArg := instructions[2]
	if firstArg.Type != InstructionEnv || !reflect.DeepEqual(firstArg.Args, []string{"FOO", ""}) {
		t.Fatalf("unexpected ARG-without-default instruction: %#v", firstArg)
	}
	secondArg := instructions[3]
	if secondArg.Type != InstructionEnv || !reflect.DeepEqual(secondArg.Args, []string{"BAR", "baz"}) {
		t.Fatalf("unexpected ARG-with-default instruction: %#v", secondArg)
	}
}

func TestTemplateBaseExposesJsDevcontainerAndMcpHelpers(t *testing.T) {
	mcpTemplate := Template(nil).
		FromTemplate("mcp-gateway").
		AddMcpServer("exa", "brave").
		serialize()
	if len(mcpTemplate.Steps) != 1 || mcpTemplate.Steps[0].Args[0] != "mcp-gateway pull exa brave" {
		t.Fatalf("unexpected MCP helper command: %#v", mcpTemplate.Steps)
	}

	devcontainerTemplate := Template(nil).
		FromTemplate("devcontainer").
		BetaDevContainerPrebuild("/workspace").
		BetaSetDevContainerStart("/workspace")

	serialized := devcontainerTemplate.serialize()
	if len(serialized.Steps) < 1 {
		t.Fatalf("expected helper methods to add steps, got %#v", serialized)
	}
	if serialized.Steps[0].Args[0] != "devcontainer build --workspace-folder /workspace" {
		t.Fatalf("unexpected devcontainer prebuild command: %#v", serialized.Steps[0])
	}
	if serialized.StartCmd == "" || serialized.ReadyCmd != "[ -f /devcontainer.up ]" {
		t.Fatalf("expected devcontainer start helper to set start and ready commands, got %#v", serialized)
	}
}

func TestTemplateBaseRejectsMcpServerWithoutMcpGatewayBase(t *testing.T) {
	assertBuildPanic(t, "MCP servers can only be added to mcp-gateway template", func() {
		Template(nil).FromBaseImage().AddMcpServer("exa")
	})
}

func TestTemplateBaseRejectsDevcontainerHelpersWithoutDevcontainerBase(t *testing.T) {
	assertBuildPanic(t, "Devcontainers can only used in the devcontainer template", func() {
		Template(nil).FromBaseImage().BetaDevContainerPrebuild("/workspace")
	})
	assertBuildPanic(t, "Devcontainers can only used in the devcontainer template", func() {
		Template(nil).FromBaseImage().BetaSetDevContainerStart("/workspace")
	})
}

func TestTemplateBaseRejectsInvalidCopySourcePathsAtCallSite(t *testing.T) {
	for _, src := range []string{"/absolute/path", "../escape"} {
		t.Run(strings.ReplaceAll(src, "/", "_"), func(t *testing.T) {
			assertBuildPanic(t, validateRelativePathMust(t, src).Error(), func() {
				Template(nil).FromBaseImage().Copy(src, "/app", nil)
			})
		})
	}
}

func TestTemplateBaseRejectsInvalidCopyItemsSourcePathsAtCallSite(t *testing.T) {
	for _, src := range []string{"/absolute/path", "../escape"} {
		t.Run(strings.ReplaceAll(src, "/", "_"), func(t *testing.T) {
			assertBuildPanic(t, validateRelativePathMust(t, src).Error(), func() {
				Template(nil).FromBaseImage().CopyItems([]CopyItem{{
					Src:  src,
					Dest: "/app",
				}})
			})
		})
	}
}

func TestTemplateBuilderDefaultsAndHelpersMatchCurrentTs(t *testing.T) {
	defaultTemplate := Template(nil).serialize()
	if defaultTemplate.FromImage != "e2bdev/base" {
		t.Fatalf("expected default base image e2bdev/base, got %#v", defaultTemplate.FromImage)
	}

	cases := []struct {
		name string
		got  string
		want string
	}{
		{name: "debian", got: Template(nil).FromDebianImage().serialize().FromImage, want: "debian:stable"},
		{name: "ubuntu", got: Template(nil).FromUbuntuImage().serialize().FromImage, want: "ubuntu:latest"},
		{name: "python", got: Template(nil).FromPythonImage().serialize().FromImage, want: "python:3"},
		{name: "node", got: Template(nil).FromNodeImage().serialize().FromImage, want: "node:lts"},
		{name: "bun", got: Template(nil).FromBunImage().serialize().FromImage, want: "oven/bun:latest"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Fatalf("%s default image mismatch: got %q want %q", tc.name, tc.got, tc.want)
		}
	}

	template := Template(nil).
		Copy([]string{"a.txt", "b.txt"}, "/app", &struct {
			ForceUpload     bool
			User            string
			Mode            int
			ResolveSymlinks bool
		}{ForceUpload: true, User: "root", Mode: 0o755, ResolveSymlinks: true}).
		Remove([]string{"/tmp/a", "/tmp/b"}, &struct {
			Recursive bool
			Force     bool
			User      string
		}{Recursive: true, Force: true, User: "root"}).
		Rename("/tmp/old", "/tmp/new", &struct {
			Force bool
			User  string
		}{Force: true, User: "root"}).
		MakeDir([]string{"/app/logs", "/app/cache"}, &struct {
			Mode int
			User string
		}{Mode: 0o755, User: "root"}).
		MakeSymlink("/usr/bin/python3", "/usr/bin/python", &struct {
			Force bool
			User  string
		}{Force: true, User: "root"}).
		RunCmd([]string{"echo one", "echo two"}, &struct{ User string }{User: "root"}).
		PipInstall("numpy", nil).
		PipInstall([]string{"pandas"}, &struct{ G bool }{G: false}).
		NpmInstall("tsx", &struct{ G bool }{G: true}).
		NpmInstall("typescript", &struct{ Dev bool }{Dev: true}).
		BunInstall("tsx", &struct{ G bool }{G: true}).
		BunInstall("typescript", &struct{ Dev bool }{Dev: true}).
		AptInstall([]string{"git", "curl"}, &struct {
			NoInstallRecommends bool
			FixMissing          bool
		}{NoInstallRecommends: true, FixMissing: true}).
		GitClone("https://github.com/e2b-dev/E2B.git", "/src", &struct {
			Branch string
			Depth  int
			User   string
		}{Branch: "main", Depth: 1, User: "root"})

	steps := template.serialize().Steps
	assertStepArgs := func(index int, want []string) {
		t.Helper()
		if !reflect.DeepEqual(steps[index].Args, want) {
			t.Fatalf("step %d args mismatch:\n got %#v\nwant %#v", index, steps[index].Args, want)
		}
	}
	assertStepArgs(0, []string{"a.txt", "/app", "root", "0755"})
	assertStepArgs(1, []string{"b.txt", "/app", "root", "0755"})
	assertStepArgs(2, []string{"rm -r -f /tmp/a /tmp/b", "root"})
	assertStepArgs(3, []string{"mv /tmp/old /tmp/new -f", "root"})
	assertStepArgs(4, []string{"mkdir -p -m 0755 /app/logs /app/cache", "root"})
	assertStepArgs(5, []string{"ln -s -f /usr/bin/python3 /usr/bin/python", "root"})
	assertStepArgs(6, []string{"echo one && echo two", "root"})
	assertStepArgs(7, []string{"pip install numpy", "root"})
	assertStepArgs(8, []string{"pip install --user pandas"})
	assertStepArgs(9, []string{"npm install -g tsx", "root"})
	assertStepArgs(10, []string{"npm install --save-dev typescript"})
	assertStepArgs(11, []string{"bun install -g tsx", "root"})
	assertStepArgs(12, []string{"bun install --dev typescript"})
	assertStepArgs(13, []string{"apt-get update && DEBIAN_FRONTEND=noninteractive DEBCONF_NOWARNINGS=yes apt-get install -y --no-install-recommends --fix-missing git curl", "root"})
	assertStepArgs(14, []string{"git clone https://github.com/e2b-dev/E2B.git --branch main --single-branch --depth 1 /src", "root"})
}

func TestTemplateBuilderOptionalArgsAndCopyItemsMatchCurrentTs(t *testing.T) {
	template := Template(nil).
		RunCmd("echo ok").
		PipInstall().
		NpmInstall().
		BunInstall().
		CopyItems([]CopyItem{{
			Src:  []string{"one.txt", "two.txt"},
			Dest: "/app",
			Mode: 0o644,
		}, {
			Src:  "plain.txt",
			Dest: "/app",
		}})

	steps := template.serialize().Steps
	got := [][]string{
		steps[0].Args,
		steps[1].Args,
		steps[2].Args,
		steps[3].Args,
		steps[4].Args,
		steps[5].Args,
		steps[6].Args,
	}
	want := [][]string{
		{"echo ok"},
		{"pip install .", "root"},
		{"npm install"},
		{"bun install"},
		{"one.txt", "/app", "", "0644"},
		{"two.txt", "/app", "", "0644"},
		{"plain.txt", "/app", "", ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("optional builder args mismatch:\n got %#v\nwant %#v", got, want)
	}
}

func TestFromGCPRegistryReadsServiceAccountLikeCurrentTs(t *testing.T) {
	contextDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contextDir, "service-account.json"), []byte(`{"project_id":"demo"}`), 0o644); err != nil {
		t.Fatalf("failed to write service account fixture: %v", err)
	}

	fromFile := Template(&TemplateOptions{FileContextPath: contextDir}).
		FromGCPRegistry("gcr.io/demo/image:latest", &GCPRegistryCredentials{ServiceAccountJSON: "service-account.json"}).
		serialize()
	if fromFile.FromImageRegistry == nil || fromFile.FromImageRegistry.ServiceAccountJSON != `{"project_id":"demo"}` {
		t.Fatalf("expected GCP credentials to be read from file, got %#v", fromFile.FromImageRegistry)
	}

	fromObject := Template(nil).
		FromGCPRegistry("gcr.io/demo/image:latest", &GCPRegistryCredentials{ServiceAccountJSON: map[string]string{"project_id": "demo"}}).
		serialize()
	if fromObject.FromImageRegistry == nil || fromObject.FromImageRegistry.ServiceAccountJSON != `{"project_id":"demo"}` {
		t.Fatalf("expected GCP object credentials to be stringified, got %#v", fromObject.FromImageRegistry)
	}
}

func assertBuildPanic(t *testing.T, message string, fn func()) {
	t.Helper()
	defer func() {
		got := recover()
		if got == nil {
			t.Fatalf("expected panic %q", message)
		}
		if err, ok := got.(error); !ok || err.Error() != message {
			t.Fatalf("expected panic %q, got %#v", message, got)
		}
	}()
	fn()
}

func assertBuildPanicWithTrace(t *testing.T, message, traceFragment string, fn func()) {
	t.Helper()
	defer func() {
		got := recover()
		if got == nil {
			t.Fatalf("expected panic %q", message)
		}
		err, ok := got.(error)
		if !ok || err.Error() != message {
			t.Fatalf("expected panic %q, got %#v", message, got)
		}

		var buildErr *shared.BuildError
		if !errors.As(err, &buildErr) {
			t.Fatalf("expected BuildError panic, got %T %v", err, err)
		}
		if !strings.Contains(buildErr.CallerTrace, traceFragment) {
			t.Fatalf("expected caller trace to contain %q, got %q", traceFragment, buildErr.CallerTrace)
		}
	}()
	fn()
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = oldStdout
	}()

	outputCh := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(reader)
		outputCh <- string(data)
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close stdout writer: %v", err)
	}
	return <-outputCh
}

func slicesContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validateRelativePathMust(t *testing.T, src string) error {
	t.Helper()
	err := validateRelativePath(src)
	if err == nil {
		t.Fatalf("expected validateRelativePath(%q) to fail", src)
	}
	return err
}

func TestCopyAbsolutePathPanicIncludesCallerTrace(t *testing.T) {
	assertBuildPanicWithTrace(
		t,
		`Invalid source path "/absolute/path": absolute paths are not allowed. Use a relative path within the context directory.`,
		"TestCopyAbsolutePathPanicIncludesCallerTrace",
		func() {
			Template(nil).FromBaseImage().Copy("/absolute/path", "/tmp/out")
		},
	)
}

func TestBuildInBackgroundUsesStructuredJsTemplatePayload(t *testing.T) {
	var triggerBody map[string]any
	var uploaded bool
	contextDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contextDir, "src"), []byte("print('hi')\n"), 0o644); err != nil {
		t.Fatalf("failed to write source fixture: %v", err)
	}
	expectedHash, err := calculateFilesHash("src", "/app/src", contextDir, nil, false)
	if err != nil {
		t.Fatalf("failed to calculate expected hash: %v", err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/templates":
			if err := json.NewEncoder(w).Encode(map[string]any{
				"templateID": "tmpl-1",
				"buildID":    "bld-1",
			}); err != nil {
				t.Fatalf("failed to encode build request response: %v", err)
			}
		case "/templates/tmpl-1/files/" + expectedHash:
			if err := json.NewEncoder(w).Encode(map[string]any{
				"present": false,
				"url":     server.URL + "/upload",
			}); err != nil {
				t.Fatalf("failed to encode upload link response: %v", err)
			}
		case "/upload":
			uploaded = true
			w.WriteHeader(http.StatusOK)
		case "/v2/templates/tmpl-1/builds/bld-1":
			if err := json.NewDecoder(r.Body).Decode(&triggerBody); err != nil {
				t.Fatalf("failed to decode trigger body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	template := Template(&TemplateOptions{FileContextPath: contextDir}).
		FromPythonImage("3.12").
		SetStartCmd("python main.py", WaitForFile("/ready")).
		Copy("src", "/app/src", nil)

	buildInfo, err := BuildInBackground(context.Background(), template, "tmpl:v1", &BuildOptions{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	})
	if err != nil {
		t.Fatalf("BuildInBackground returned error: %v", err)
	}
	if buildInfo == nil || buildInfo.TemplateID != "tmpl-1" || buildInfo.BuildID != "bld-1" {
		t.Fatalf("unexpected build info: %#v", buildInfo)
	}
	if _, ok := triggerBody["instructions"]; ok {
		t.Fatalf("did not expect legacy instructions payload, got %#v", triggerBody)
	}
	if triggerBody["fromImage"] != "python:3.12" {
		t.Fatalf("expected JS-style fromImage field, got %#v", triggerBody)
	}
	if triggerBody["startCmd"] != "python main.py" {
		t.Fatalf("expected JS-style startCmd field, got %#v", triggerBody)
	}
	if triggerBody["readyCmd"] != "[ -f /ready ]" {
		t.Fatalf("expected JS-style readyCmd field, got %#v", triggerBody)
	}
	steps, ok := triggerBody["steps"].([]any)
	if !ok || len(steps) != 1 {
		t.Fatalf("expected JS-style steps array, got %#v", triggerBody)
	}
	step, ok := steps[0].(map[string]any)
	if !ok || step["filesHash"] != expectedHash {
		t.Fatalf("expected COPY step to include filesHash, got %#v", steps)
	}
	if !uploaded {
		t.Fatal("expected copy source to be uploaded before triggering build")
	}
}

func TestBuildInBackgroundPreservesExplicitFalseForceFieldsLikeJsAndPython(t *testing.T) {
	var triggerBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/templates":
			if err := json.NewEncoder(w).Encode(map[string]any{
				"templateID": "tmpl-1",
				"buildID":    "bld-1",
			}); err != nil {
				t.Fatalf("failed to encode build request response: %v", err)
			}
		case "/v2/templates/tmpl-1/builds/bld-1":
			if err := json.NewDecoder(r.Body).Decode(&triggerBody); err != nil {
				t.Fatalf("failed to decode trigger body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	template := Template(nil).FromBaseImage().RunCmd("echo hi")
	buildInfo, err := BuildInBackground(context.Background(), template, "tmpl:v1", &BuildOptions{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: intPtr(1000),
	})
	if err != nil {
		t.Fatalf("BuildInBackground returned error: %v", err)
	}
	if buildInfo == nil || buildInfo.TemplateID != "tmpl-1" || buildInfo.BuildID != "bld-1" {
		t.Fatalf("unexpected build info: %#v", buildInfo)
	}
	if value, ok := triggerBody["force"].(bool); !ok || value {
		t.Fatalf("expected template-level force=false to be preserved, got %#v", triggerBody["force"])
	}
	steps, ok := triggerBody["steps"].([]any)
	if !ok || len(steps) != 1 {
		t.Fatalf("expected one step, got %#v", triggerBody["steps"])
	}
	step, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("expected step object, got %#v", steps[0])
	}
	if value, ok := step["force"].(bool); !ok || value {
		t.Fatalf("expected step-level force=false to be preserved, got %#v", step["force"])
	}
}

func TestWaitForBuildFinishRepollsAtJsCadence(t *testing.T) {
	pollCount := 0
	pollTimes := make([]time.Time, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/templates/tmpl-1/builds/bld-1/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		pollCount++
		pollTimes = append(pollTimes, time.Now())

		status := BuildStatusWaiting
		if pollCount == 2 {
			status = BuildStatusReady
		} else if pollCount > 2 {
			t.Fatalf("expected at most 2 polls, got %d", pollCount)
		}

		if err := json.NewEncoder(w).Encode(buildStatusAPIResponse{
			TemplateID: "tmpl-1",
			BuildID:    "bld-1",
			Status:     status,
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := api.NewApiClient(&api.ClientConfig{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: 1000,
	})
	if err != nil {
		t.Fatalf("failed to create API client: %v", err)
	}

	status, err := waitForBuildFinish(context.Background(), client, "tmpl-1", "bld-1", nil, nil)
	if err != nil {
		t.Fatalf("waitForBuildFinish returned error: %v", err)
	}
	if status == nil || status.Status != BuildStatusReady {
		t.Fatalf("expected ready status, got %#v", status)
	}
	if pollCount != 2 || len(pollTimes) != 2 {
		t.Fatalf("expected exactly 2 polls, got count=%d times=%d", pollCount, len(pollTimes))
	}

	if gap := pollTimes[1].Sub(pollTimes[0]); gap >= time.Second {
		t.Fatalf("expected Go repoll cadence to stay within JS/Python range, got %s", gap)
	}
}

func TestWaitForBuildFinishAdvancesLogsOffsetWithoutLoggerLikeJsAndPython(t *testing.T) {
	pollCount := 0
	timestamp := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/templates/tmpl-1/builds/bld-1/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		wantOffset := strconv.Itoa(pollCount)
		if got := r.URL.Query().Get("logsOffset"); got != wantOffset {
			t.Fatalf("expected logsOffset=%s on poll %d, got %q", wantOffset, pollCount+1, got)
		}
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Fatalf("expected limit=100 on poll %d, got %q", pollCount+1, got)
		}

		pollCount++
		resp := buildStatusAPIResponse{
			TemplateID: "tmpl-1",
			BuildID:    "bld-1",
			Status:     BuildStatusReady,
		}
		if pollCount == 1 {
			resp.Status = BuildStatusWaiting
			resp.LogEntries = []buildLogEntryAPIResponse{
				{
					Timestamp: timestamp,
					Level:     LogEntryLevel("info"),
					Message:   "building",
				},
			}
		}

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := api.NewApiClient(&api.ClientConfig{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: 1000,
	})
	if err != nil {
		t.Fatalf("failed to create API client: %v", err)
	}

	status, err := waitForBuildFinish(context.Background(), client, "tmpl-1", "bld-1", nil, nil)
	if err != nil {
		t.Fatalf("waitForBuildFinish returned error: %v", err)
	}
	if status == nil || status.Status != BuildStatusReady {
		t.Fatalf("expected ready status, got %#v", status)
	}
	if pollCount != 2 {
		t.Fatalf("expected exactly 2 polls, got %d", pollCount)
	}
}

func TestWaitForBuildFinishIncludesCallerTraceForFailedStep(t *testing.T) {
	timestamp := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/templates/tmpl-1/builds/bld-1/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"buildID":    "bld-1",
			"templateID": "tmpl-1",
			"status":     "error",
			"logEntries": []map[string]any{},
			"logs":       []string{},
			"reason": map[string]any{
				"message": "step failed",
				"step":    "1",
				"logEntries": []map[string]any{
					{
						"timestamp": timestamp.Format(time.RFC3339),
						"level":     "error",
						"message":   "boom",
					},
				},
			},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := api.NewApiClient(&api.ClientConfig{
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: 1000,
	})
	if err != nil {
		t.Fatalf("failed to create API client: %v", err)
	}

	_, err = waitForBuildFinish(context.Background(), client, "tmpl-1", "bld-1", nil, []string{
		"first trace",
		"TestWaitForBuildFinishIncludesCallerTraceForFailedStep",
		"final trace",
	})
	var buildErr *shared.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("expected BuildError, got %T %v", err, err)
	}
	if buildErr.Message != "build failed: step failed" {
		t.Fatalf("unexpected build error message: %q", buildErr.Message)
	}
	if !strings.Contains(buildErr.CallerTrace, "TestWaitForBuildFinishIncludesCallerTraceForFailedStep") {
		t.Fatalf("expected caller trace for failed step, got %q", buildErr.CallerTrace)
	}
}

func TestSkipCacheMarksWholeTemplateWhenAppliedBeforeBaseLayer(t *testing.T) {
	template := Template(nil).SkipCache().FromPythonImage("3.12")

	serialized := template.serialize()
	if !serialized.Force {
		t.Fatalf("expected SkipCache before base image to force whole template, got %#v", serialized)
	}
	if serialized.FromImage != "python:3.12" {
		t.Fatalf("expected serialized base image, got %#v", serialized)
	}
}

func TestBuildOptionSkipCacheMarksWholeTemplateForBuild(t *testing.T) {
	var createBody requestBuildRequest
	var triggerBody triggerBuildTemplate
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v3/templates":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("failed to decode create body: %v", err)
			}
			if err := json.NewEncoder(w).Encode(requestBuildResponse{
				TemplateID: "tmpl-1",
				BuildID:    "bld-1",
			}); err != nil {
				t.Fatalf("failed to encode create response: %v", err)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/v2/templates/tmpl-1/builds/bld-1":
			if err := json.NewDecoder(r.Body).Decode(&triggerBody); err != nil {
				t.Fatalf("failed to decode trigger body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/templates/tmpl-1/builds/bld-1/status"):
			if err := json.NewEncoder(w).Encode(buildStatusAPIResponse{
				TemplateID: "tmpl-1",
				BuildID:    "bld-1",
				Status:     BuildStatusReady,
			}); err != nil {
				t.Fatalf("failed to encode status response: %v", err)
			}
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	timeout := 1000
	template := Template(nil).FromBaseImage()
	if _, err := Build(context.Background(), template, "tmpl:v1", &BuildOptions{
		BasicBuildOptions: BasicBuildOptions{
			SkipCache: true,
			CpuCount:  1,
			MemoryMB:  1024,
		},
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: &timeout,
	}); err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if createBody.Name != "tmpl:v1" {
		t.Fatalf("expected create request name tmpl:v1, got %#v", createBody)
	}
	if !triggerBody.Force {
		t.Fatalf("expected BuildOptions.SkipCache to force whole template build, got %#v", triggerBody)
	}
}

func TestBuildOptionSkipCacheMarksWholeTemplateForBuildInBackground(t *testing.T) {
	var triggerBody triggerBuildTemplate
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v3/templates":
			if err := json.NewEncoder(w).Encode(requestBuildResponse{
				TemplateID: "tmpl-1",
				BuildID:    "bld-1",
			}); err != nil {
				t.Fatalf("failed to encode create response: %v", err)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/v2/templates/tmpl-1/builds/bld-1":
			if err := json.NewDecoder(r.Body).Decode(&triggerBody); err != nil {
				t.Fatalf("failed to decode trigger body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	timeout := 1000
	if _, err := BuildInBackground(context.Background(), Template(nil).FromBaseImage(), "tmpl:v1", &BuildOptions{
		BasicBuildOptions: BasicBuildOptions{
			SkipCache: true,
			CpuCount:  1,
			MemoryMB:  1024,
		},
		ApiKey:           "e2b_0000000000000000000000000000000000000000",
		ApiUrl:           server.URL,
		RequestTimeoutMs: &timeout,
	}); err != nil {
		t.Fatalf("BuildInBackground returned error: %v", err)
	}

	if !triggerBody.Force {
		t.Fatalf("expected BuildOptions.SkipCache to force whole template build, got %#v", triggerBody)
	}
}

func TestBuildOptionSkipCacheDoesNotStickAcrossReusedTemplates(t *testing.T) {
	var triggerBodies []triggerBuildTemplate
	buildNumber := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v3/templates":
			buildNumber++
			if err := json.NewEncoder(w).Encode(requestBuildResponse{
				TemplateID: fmt.Sprintf("tmpl-%d", buildNumber),
				BuildID:    fmt.Sprintf("bld-%d", buildNumber),
			}); err != nil {
				t.Fatalf("failed to encode create response: %v", err)
			}
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v2/templates/"):
			var triggerBody triggerBuildTemplate
			if err := json.NewDecoder(r.Body).Decode(&triggerBody); err != nil {
				t.Fatalf("failed to decode trigger body: %v", err)
			}
			triggerBodies = append(triggerBodies, triggerBody)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	timeout := 1000
	template := Template(nil).FromBaseImage()
	for _, skipCache := range []bool{true, false} {
		_, err := BuildInBackground(context.Background(), template, "tmpl:v1", &BuildOptions{
			BasicBuildOptions: BasicBuildOptions{
				SkipCache: skipCache,
				CpuCount:  1,
				MemoryMB:  1024,
			},
			ApiKey:           "e2b_0000000000000000000000000000000000000000",
			ApiUrl:           server.URL,
			RequestTimeoutMs: &timeout,
		})
		if err != nil {
			t.Fatalf("BuildInBackground(skipCache=%t) returned error: %v", skipCache, err)
		}
	}

	if len(triggerBodies) != 2 {
		t.Fatalf("expected 2 trigger builds, got %d", len(triggerBodies))
	}
	if !triggerBodies[0].Force {
		t.Fatalf("expected first build to force cache bypass, got %#v", triggerBodies[0])
	}
	if triggerBodies[1].Force {
		t.Fatalf("expected second build to preserve cache usage, got %#v", triggerBodies[1])
	}
}

func TestInstructionsWithHashesPreservesCallerTraceForImplicitBaseTemplate(t *testing.T) {
	template := Template(&TemplateOptions{FileContextPath: t.TempDir()}).
		Copy("missing.txt", "/app/missing.txt")

	_, err := template.instructionsWithHashes()
	var buildErr *shared.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("expected BuildError, got %T %v", err, err)
	}
	if !strings.Contains(buildErr.CallerTrace, "TestInstructionsWithHashesPreservesCallerTraceForImplicitBaseTemplate") {
		t.Fatalf("expected implicit-base instruction caller trace, got %q", buildErr.CallerTrace)
	}
}

func TestToDockerfileRejectsTemplateBaseWithoutDockerImage(t *testing.T) {
	_, err := ToDockerfile(Template(nil).FromTemplate("base"))
	if err == nil {
		t.Fatal("expected ToDockerfile to reject templates built from other templates")
	}
	expected := "Cannot convert template built from another template to Dockerfile. Templates based on other templates can only be built using the E2B API."
	if err.Error() != expected {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTemplateBuildOptionShapesIncludeSharedConnectionFields(t *testing.T) {
	buildOpts := reflect.TypeOf(BuildOptions{})
	if _, ok := buildOpts.FieldByName("SandboxUrl"); !ok {
		t.Fatal("expected BuildOptions to expose SandboxUrl like shared JS connection options")
	}
	if _, ok := buildOpts.FieldByName("Signal"); !ok {
		t.Fatal("expected BuildOptions to expose Signal like shared request cancellation options")
	}
	if field, ok := buildOpts.FieldByName("Debug"); !ok {
		t.Fatal("expected BuildOptions to expose Debug")
	} else if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected BuildOptions.Debug to be *bool, got %v", field.Type)
	}

	statusOpts := reflect.TypeOf(GetBuildStatusOptions{})
	if _, ok := statusOpts.FieldByName("SandboxUrl"); !ok {
		t.Fatal("expected GetBuildStatusOptions to expose SandboxUrl like shared JS connection options")
	}
	if _, ok := statusOpts.FieldByName("Signal"); !ok {
		t.Fatal("expected GetBuildStatusOptions to expose Signal like shared request cancellation options")
	}
	if field, ok := statusOpts.FieldByName("Debug"); !ok {
		t.Fatal("expected GetBuildStatusOptions to expose Debug")
	} else if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected GetBuildStatusOptions.Debug to be *bool, got %v", field.Type)
	}
}

func TestTemplateControlPlaneHelperShapesMatchJsAndPython(t *testing.T) {
	var _ ConnectionOpts = ConnectionOpts{}

	connectionOpts := reflect.TypeOf(ConnectionOpts{})
	for _, field := range []string{"Alias", "Tags", "CpuCount", "MemoryMB", "SkipCache", "OnBuildLogs", "LogsOffset"} {
		if _, ok := connectionOpts.FieldByName(field); ok {
			t.Fatalf("did not expect template ConnectionOpts to expose build-only field %q", field)
		}
	}
	if _, ok := connectionOpts.FieldByName("SandboxUrl"); !ok {
		t.Fatal("expected ConnectionOpts to expose SandboxUrl like shared JS connection options")
	}
	if field, ok := connectionOpts.FieldByName("Debug"); !ok {
		t.Fatal("expected ConnectionOpts to expose Debug")
	} else if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected ConnectionOpts.Debug to be *bool, got %v", field.Type)
	}
	if _, ok := connectionOpts.FieldByName("Signal"); !ok {
		t.Fatal("expected ConnectionOpts to expose Signal like shared request cancellation options")
	}

	if got := reflect.TypeOf(GetBuildStatus); got.In(1) != reflect.TypeOf((*BuildInfo)(nil)) {
		t.Fatalf("expected GetBuildStatus to accept *BuildInfo, got %v", got.In(1))
	}
	if got := reflect.TypeOf(GetBuildStatus); got.In(2) != reflect.TypeOf((*GetBuildStatusOptions)(nil)) {
		t.Fatalf("expected GetBuildStatus to accept *GetBuildStatusOptions, got %v", got.In(2))
	}
	for _, fn := range []struct {
		name string
		got  reflect.Type
	}{
		{name: "Exists", got: reflect.TypeOf(Exists)},
		{name: "AssignTags", got: reflect.TypeOf(AssignTags)},
		{name: "RemoveTags", got: reflect.TypeOf(RemoveTags)},
		{name: "GetTags", got: reflect.TypeOf(GetTags)},
	} {
		if fn.got.In(fn.got.NumIn()-1) != reflect.TypeOf((*ConnectionOpts)(nil)) {
			t.Fatalf("expected %s to accept *ConnectionOpts, got %v", fn.name, fn.got.In(fn.got.NumIn()-1))
		}
	}
}

func connectionOptsFromBuildOptions(opts *BuildOptions) *ConnectionOpts {
	if opts == nil {
		return nil
	}

	return &ConnectionOpts{
		ApiKey:           opts.ApiKey,
		AccessToken:      opts.AccessToken,
		Domain:           opts.Domain,
		ApiUrl:           opts.ApiUrl,
		SandboxUrl:       opts.SandboxUrl,
		Debug:            opts.Debug,
		Signal:           opts.Signal,
		RequestTimeoutMs: opts.RequestTimeoutMs,
		Headers:          opts.Headers,
		Logger:           opts.Logger,
		Proxy:            opts.Proxy,
	}
}

func intPtr(value int) *int {
	return &value
}
