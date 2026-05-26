package volume

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildApiClientConfigUsesDebugApiURL(t *testing.T) {
	config := buildApiClientConfig(&ConnectionOpts{
		Debug: true,
	})

	if config.ApiUrl != "http://localhost:3000" {
		t.Fatalf("expected debug API URL http://localhost:3000, got %q", config.ApiUrl)
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
		Debug: true,
	})

	if config.ApiUrl != "http://localhost:8080" {
		t.Fatalf("expected debug volume API URL http://localhost:8080, got %q", config.ApiUrl)
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
		ApiKey: "test-api-key",
		Domain: "example.test",
		Debug:  true,
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
	if !volume.Debug {
		t.Fatal("expected debug flag to be exported on Volume")
	}
}
