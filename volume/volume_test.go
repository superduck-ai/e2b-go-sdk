package volume

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildApiClientConfigUsesDebugApiURL(t *testing.T) {
	config := buildApiClientConfig(&ConnectionOpts{
		Debug: boolPtr(true),
	})

	if config.ApiUrl != "http://localhost:3000" {
		t.Fatalf("expected debug API URL http://localhost:3000, got %q", config.ApiUrl)
	}
}

func TestBuildApiClientConfigUsesDebugApiURLFromEnv(t *testing.T) {
	t.Setenv("E2B_API_URL", "")
	t.Setenv("E2B_DEBUG", "true")

	config := buildApiClientConfig(&ConnectionOpts{})

	if config.ApiUrl != "http://localhost:3000" {
		t.Fatalf("expected debug API URL from env http://localhost:3000, got %q", config.ApiUrl)
	}
}

func TestBuildApiClientConfigAllowsExplicitZeroRequestTimeout(t *testing.T) {
	zero := 0
	config := buildApiClientConfig(&ConnectionOpts{
		RequestTimeoutMs: &zero,
	})

	if config.RequestTimeoutMs != 0 {
		t.Fatalf("expected explicit zero request timeout to be preserved, got %d", config.RequestTimeoutMs)
	}
}

func TestNewVolumeConnectionConfigUsesApiDomainByDefault(t *testing.T) {
	config := NewVolumeConnectionConfig(&VolumeApiOpts{
		Domain: "example.test",
	})

	if config.ApiUrl != "https://api.example.test" {
		t.Fatalf("expected default volume API URL https://api.example.test, got %q", config.ApiUrl)
	}
}

func TestNewVolumeConnectionConfigUsesDebugApiURL(t *testing.T) {
	config := NewVolumeConnectionConfig(&VolumeApiOpts{
		Debug: boolPtr(true),
	})

	if config.ApiUrl != "http://localhost:8080" {
		t.Fatalf("expected debug volume API URL http://localhost:8080, got %q", config.ApiUrl)
	}
}

func TestNewVolumeConnectionConfigPreservesExplicitFalseDebugOverEnv(t *testing.T) {
	t.Setenv("E2B_VOLUME_API_URL", "")
	t.Setenv("E2B_DEBUG", "true")

	config := NewVolumeConnectionConfig(&VolumeApiOpts{
		Domain: "example.test",
		Debug:  boolPtr(false),
	})

	if config.Debug {
		t.Fatal("expected explicit false debug to override env debug=true")
	}
	if config.ApiUrl != "https://api.example.test" {
		t.Fatalf("expected explicit false debug to keep hosted volume API URL, got %q", config.ApiUrl)
	}
}

func TestNewVolumeConnectionConfigUsesApiUrlFromEnv(t *testing.T) {
	t.Setenv("E2B_VOLUME_API_URL", "http://localhost:8080")

	config := NewVolumeConnectionConfig(&VolumeApiOpts{})

	if config.ApiUrl != "http://localhost:8080" {
		t.Fatalf("expected volume API URL from env, got %q", config.ApiUrl)
	}
}

func TestNewVolumeConnectionConfigApiUrlArgsHavePriorityOverEnv(t *testing.T) {
	t.Setenv("E2B_VOLUME_API_URL", "http://localhost:1111")

	config := NewVolumeConnectionConfig(&VolumeApiOpts{
		ApiUrl: "http://localhost:8080",
	})

	if config.ApiUrl != "http://localhost:8080" {
		t.Fatalf("expected API URL arg to win over env, got %q", config.ApiUrl)
	}
}

func TestNewVolumeConnectionConfigUsesDebugApiURLFromEnv(t *testing.T) {
	t.Setenv("E2B_VOLUME_API_URL", "")
	t.Setenv("E2B_DOMAIN", "")
	t.Setenv("E2B_DEBUG", "true")

	config := NewVolumeConnectionConfig(&VolumeApiOpts{})

	if config.ApiUrl != "http://localhost:8080" {
		t.Fatalf("expected debug env to select local volume API URL, got %q", config.ApiUrl)
	}
}

func TestNewVolumeConnectionConfigUsesDomainFromEnv(t *testing.T) {
	t.Setenv("E2B_VOLUME_API_URL", "")
	t.Setenv("E2B_DEBUG", "")
	t.Setenv("E2B_DOMAIN", "custom.com")

	config := NewVolumeConnectionConfig(&VolumeApiOpts{})

	if config.ApiUrl != "https://api.custom.com" {
		t.Fatalf("expected volume API URL to use env domain, got %q", config.ApiUrl)
	}
}

func TestNewVolumeConnectionConfigAllowsExplicitZeroRequestTimeout(t *testing.T) {
	zero := 0
	config := NewVolumeConnectionConfig(&VolumeApiOpts{
		RequestTimeoutMs: &zero,
	})

	if config.RequestTimeoutMs == nil || *config.RequestTimeoutMs != 0 {
		t.Fatalf("expected explicit zero request timeout to be preserved, got %#v", config.RequestTimeoutMs)
	}
}

func TestNewVolumeConnectionConfigIncludesDefaultHeaders(t *testing.T) {
	config := NewVolumeConnectionConfig(&VolumeApiOpts{})

	if config.Headers["lang"] != "go" {
		t.Fatalf("expected default lang header, got %#v", config.Headers)
	}
	if config.Headers["publisher"] != "e2b" {
		t.Fatalf("expected default publisher header, got %#v", config.Headers)
	}
}

func TestNewVolumeConnectionConfigAllowsHeaderOverrides(t *testing.T) {
	config := NewVolumeConnectionConfig(&VolumeApiOpts{
		Headers: map[string]string{
			"publisher": "custom",
			"X-Test":    "value",
		},
	})

	if config.Headers["publisher"] != "custom" {
		t.Fatalf("expected custom publisher header override, got %#v", config.Headers)
	}
	if config.Headers["X-Test"] != "value" {
		t.Fatalf("expected custom header to be preserved, got %#v", config.Headers)
	}
}

func TestNewVolumeApiClientUsesVolumeApiDefaultsInsteadOfControlPlaneApiURL(t *testing.T) {
	client := newVolumeApiClient("vol-1", "token", &ConnectionOpts{
		Domain: "example.test",
		ApiUrl: "https://custom-volume-api.example.test",
	})

	if client.config.ApiUrl != "https://api.example.test" {
		t.Fatalf("expected volume client to use volume API default, got %q", client.config.ApiUrl)
	}
}

func TestCreateExposesJsStyleVolumeMetadataFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"volumeID":"vol-1","name":"test-volume","token":"secret-token"}`))
	}))
	defer server.Close()

	volume, err := Create(context.Background(), "test-volume", &ConnectionOpts{
		ApiKey: "e2b_0000000000000000000000000000000000000000",
		Domain: "example.test",
		Debug:  boolPtr(true),
		ApiUrl: server.URL,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if volume.Token != "secret-token" {
		t.Fatalf("expected token to be exported on Volume, got %q", volume.Token)
	}
	if volume.Domain != "example.test" {
		t.Fatalf("expected domain to be exported on Volume, got %q", volume.Domain)
	}
	if volume.Debug == nil || !*volume.Debug {
		t.Fatal("expected debug flag to be exported on Volume")
	}
}

func TestResolveClientUsesPersistedVolumeFieldsWhenOptsNil(t *testing.T) {
	v := &Volume{
		VolumeID: "vol-1",
		Name:     "test-volume",
		Token:    "secret-token",
		Domain:   "example.test",
	}

	client := v.resolveClient(nil)

	if client.config.Token != "secret-token" {
		t.Fatalf("expected persisted token to be used, got %q", client.config.Token)
	}
	if client.config.Domain != "example.test" {
		t.Fatalf("expected persisted domain to be used, got %q", client.config.Domain)
	}
	if client.config.ApiUrl != "https://api.example.test" {
		t.Fatalf("expected default volume API URL from persisted domain, got %q", client.config.ApiUrl)
	}
	if client.config.RequestTimeoutMs != nil {
		t.Fatalf("expected request timeout to stay unset, got %#v", client.config.RequestTimeoutMs)
	}
	if client.config.Headers["X-Test"] != "" {
		t.Fatalf("did not expect inherited create/connect-only headers, got %#v", client.config.Headers)
	}
}

func TestConnectAllowsNilOptsAndExposesJsStyleVolumeMetadataFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumes/vol-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"volumeID":"vol-1","name":"test-volume","token":"secret-token"}`))
	}))
	defer server.Close()

	t.Setenv("E2B_API_URL", server.URL)
	t.Setenv("E2B_API_KEY", "e2b_0000000000000000000000000000000000000000")
	t.Setenv("E2B_DOMAIN", "example.test")
	t.Setenv("E2B_DEBUG", "")

	volume, err := Connect(context.Background(), "vol-1", nil)
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}

	if volume.VolumeID != "vol-1" {
		t.Fatalf("expected volume ID to be exported on Volume, got %q", volume.VolumeID)
	}
	if volume.Name != "test-volume" {
		t.Fatalf("expected name to be exported on Volume, got %q", volume.Name)
	}
	if volume.Token != "secret-token" {
		t.Fatalf("expected token to be exported on Volume, got %q", volume.Token)
	}
	if volume.Domain != "example.test" {
		t.Fatalf("expected domain to be exported on Volume, got %q", volume.Domain)
	}
	if volume.Debug == nil || *volume.Debug {
		t.Fatal("expected debug flag to default to false when opts are nil")
	}
}

func TestResolveClientPreservesPersistedExplicitFalseDebugOverEnv(t *testing.T) {
	t.Setenv("E2B_VOLUME_API_URL", "")
	t.Setenv("E2B_DEBUG", "true")

	v := &Volume{
		VolumeID: "vol-1",
		Name:     "test-volume",
		Token:    "secret-token",
		Domain:   "example.test",
		Debug:    boolPtr(false),
	}

	client := v.resolveClient(nil)

	if client.config.Debug {
		t.Fatal("expected persisted debug=false to override env debug=true")
	}
	if client.config.ApiUrl != "https://api.example.test" {
		t.Fatalf("expected persisted debug=false to keep hosted volume API URL, got %q", client.config.ApiUrl)
	}
}

func TestResolveClientAllowsExplicitFalseDebugOverride(t *testing.T) {
	v := &Volume{
		VolumeID: "vol-1",
		Name:     "test-volume",
		Token:    "secret-token",
		Domain:   "example.test",
		Debug:    boolPtr(true),
	}

	client := v.resolveClient(&VolumeApiOpts{
		Debug: boolPtr(false),
	})

	if client.config.Debug {
		t.Fatal("expected per-call debug=false to override persisted debug=true")
	}
	if client.config.ApiUrl != "https://api.example.test" {
		t.Fatalf("expected per-call debug=false to keep hosted volume API URL, got %q", client.config.ApiUrl)
	}
}
