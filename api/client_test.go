package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
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

func TestHandleApiErrorUnauthorizedHandlesEmptyBody(t *testing.T) {
	err := HandleApiError(http.StatusUnauthorized, []byte(""))

	var authErr *AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthenticationError, got %T %v", err, err)
	}
	if authErr.Message != "Unauthorized, please check your credentials." {
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

func TestHandleApiErrorRateLimitHandlesEmptyBody(t *testing.T) {
	err := HandleApiError(http.StatusTooManyRequests, []byte(""))

	var rateErr *RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("expected RateLimitError, got %T %v", err, err)
	}
	if rateErr.Message != "Rate limit exceeded, please try again later" {
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

func TestHandleApiErrorGenericEmptyBodiesIncludeStatus(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
		err := HandleApiError(status, []byte(""))
		var apiErr *ApiError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected ApiError for status %d, got %T %v", status, err, err)
		}
		if !strings.Contains(apiErr.Error(), strconv.Itoa(status)) {
			t.Fatalf("expected status %d to appear in error, got %q", status, apiErr.Error())
		}
	}
}

func TestHandleApiErrorNotFoundMatchesJsStatusMessageShape(t *testing.T) {
	err := HandleApiError(http.StatusNotFound, []byte(""))
	var notFoundErr *NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T %v", err, err)
	}
	if notFoundErr.Message != "404: " {
		t.Fatalf("expected empty 404 body to include status, got %q", notFoundErr.Message)
	}

	err = HandleApiError(http.StatusNotFound, []byte(`{"message":"Not found"}`))
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T %v", err, err)
	}
	if notFoundErr.Message != "404: Not found" {
		t.Fatalf("expected 404 JSON body message, got %q", notFoundErr.Message)
	}
}

func TestHandleApiErrorReturnsNilForSuccess(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated} {
		if err := HandleApiError(status, nil); err != nil {
			t.Fatalf("expected nil for status %d, got %v", status, err)
		}
	}
}
