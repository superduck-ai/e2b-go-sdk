package e2b

import (
	rootapi "github.com/e2b-dev/e2b-go-sdk/api"
	"testing"
)

func TestRootApiClientAliasesExposeJsStyleApiClient(t *testing.T) {
	var _ *ApiClient = (*rootapi.ApiClient)(nil)

	client, err := NewApiClient(&ConnectionConfig{
		Domain:           "e2b.app",
		ApiUrl:           "https://api.example.test",
		RequestTimeoutMs: 1234,
	}, &struct {
		RequireAccessToken bool
		RequireApiKey      bool
	}{RequireAccessToken: true})
	if err == nil {
		t.Fatal("expected authentication error when access token is required")
	}
	if client != nil {
		t.Fatalf("expected nil client when constructor fails, got %#v", client)
	}
}

func TestRootApiClientUsesRootConnectionConfig(t *testing.T) {
	client, err := NewApiClient(&ConnectionConfig{
		Domain:           "e2b.app",
		ApiUrl:           "https://api.example.test",
		RequestTimeoutMs: 1234,
		AccessToken:      "token",
	}, &struct {
		RequireAccessToken bool
		RequireApiKey      bool
	}{RequireAccessToken: true})
	if err != nil {
		t.Fatalf("expected root NewApiClient to succeed, got %v", err)
	}
	if client.BaseUrl != "https://api.example.test" {
		t.Fatalf("expected client base URL to use root connection config, got %q", client.BaseUrl)
	}
	if client.HttpClient == nil {
		t.Fatal("expected HTTP client to be initialized")
	}
}

func TestRootApiClientAllowsNilConfig(t *testing.T) {
	t.Setenv("E2B_DOMAIN", "")
	t.Setenv("E2B_API_URL", "")
	t.Setenv("E2B_DEBUG", "")

	client, err := NewApiClient(nil, nil)
	if err != nil {
		t.Fatalf("expected nil config to use defaults, got %v", err)
	}
	if client == nil {
		t.Fatal("expected client to be created")
	}
	if client.BaseUrl != "https://api.e2b.app" {
		t.Fatalf("expected default base URL https://api.e2b.app, got %q", client.BaseUrl)
	}
}
