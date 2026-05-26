package api

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewApiClientRequiresApiKeyWithAlignedMessage(t *testing.T) {
	_, err := NewApiClient(&ClientConfig{Domain: "e2b.app"}, WithRequireApiKey())
	if err == nil {
		t.Fatal("expected missing API key to fail")
	}

	var authErr *AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthenticationError, got %T %v", err, err)
	}
	expected := "API key is required, please visit the Team tab at https://e2b.dev/dashboard to get your API key. You can either set the environment variable `E2B_API_KEY` or you can pass it directly to the sandbox like Sandbox.create({ apiKey: 'e2b_...' })"
	if authErr.Message != expected {
		t.Fatalf("unexpected API key auth message: %q", authErr.Message)
	}
}

func TestNewApiClientRequiresAccessTokenWithAlignedMessage(t *testing.T) {
	_, err := NewApiClient(&ClientConfig{Domain: "e2b.app"}, WithRequireAccessToken())
	if err == nil {
		t.Fatal("expected missing access token to fail")
	}

	var authErr *AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthenticationError, got %T %v", err, err)
	}
	expected := "Access token is required, please visit the Personal tab at https://e2b.dev/dashboard to get your access token. You can set the environment variable `E2B_ACCESS_TOKEN` or pass the `accessToken` in options."
	if authErr.Message != expected {
		t.Fatalf("unexpected access token auth message: %q", authErr.Message)
	}
}

func TestHandleApiErrorUnauthorizedUsesAlignedMessage(t *testing.T) {
	err := HandleApiError(http.StatusUnauthorized, []byte(`{"message":"bad token"}`))

	var authErr *AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthenticationError, got %T %v", err, err)
	}
	if authErr.Message != "Unauthorized, please check your credentials. - bad token" {
		t.Fatalf("unexpected auth error message: %q", authErr.Message)
	}
}

func TestHandleApiErrorRateLimitUsesAlignedMessage(t *testing.T) {
	err := HandleApiError(http.StatusTooManyRequests, []byte(`{"message":"slow down"}`))

	var rateErr *RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("expected RateLimitError, got %T %v", err, err)
	}
	if rateErr.Message != "Rate limit exceeded, please try again later - slow down" {
		t.Fatalf("unexpected rate limit message: %q", rateErr.Message)
	}
}

func TestHandleApiErrorGenericUsesAlignedFormat(t *testing.T) {
	err := HandleApiError(http.StatusBadGateway, []byte(`{"message":"backend down"}`))

	var apiErr *ApiError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected ApiError, got %T %v", err, err)
	}
	if apiErr.Error() != "502: backend down" {
		t.Fatalf("unexpected api error string: %q", apiErr.Error())
	}
}
