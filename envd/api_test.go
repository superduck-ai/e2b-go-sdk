package envd

import (
	"errors"
	"testing"
)

func TestHandleEnvdApiErrorUsesAlignedGenericFormat(t *testing.T) {
	err := HandleEnvdApiError(418, []byte("bad request"))
	if err == nil {
		t.Fatal("expected envd api error")
	}

	apiErr, ok := err.(*EnvdApiError)
	if !ok {
		t.Fatalf("expected EnvdApiError, got %T %v", err, err)
	}
	if apiErr.Error() != "418: bad request" {
		t.Fatalf("unexpected envd api error string: %q", apiErr.Error())
	}
}

func TestNewEnvdApiClientAllowsExplicitZeroTimeout(t *testing.T) {
	client := NewEnvdApiClient("https://envd.example", "", nil, 0)

	if client.HttpClient.Timeout != 0 {
		t.Fatalf("expected explicit zero timeout to disable HTTP client timeout, got %s", client.HttpClient.Timeout)
	}
}

func TestHandleEnvdApiErrorMapsKnownStatusesToTypedErrors(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    []byte
		target  interface{}
		message string
	}{
		{name: "bad request", status: 400, body: []byte(`{"message":"bad input"}`), target: &InvalidArgumentError{}, message: "bad input"},
		{name: "unauthorized", status: 401, body: []byte(`{"message":"missing token"}`), target: &AuthenticationError{}, message: "missing token"},
		{name: "not found", status: 404, body: []byte(`{"message":"missing file"}`), target: &NotFoundError{}, message: "missing file"},
		{name: "not enough space", status: 507, body: []byte(`{"message":"disk full"}`), target: &NotEnoughSpaceError{}, message: "disk full"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := HandleEnvdApiError(tc.status, tc.body)
			if err == nil {
				t.Fatal("expected envd api error")
			}

			switch tc.target.(type) {
			case *InvalidArgumentError:
				var typed *InvalidArgumentError
				if !errors.As(err, &typed) {
					t.Fatalf("expected InvalidArgumentError, got %T %v", err, err)
				}
				if typed.Message != tc.message {
					t.Fatalf("unexpected message: %q", typed.Message)
				}
			case *AuthenticationError:
				var typed *AuthenticationError
				if !errors.As(err, &typed) {
					t.Fatalf("expected AuthenticationError, got %T %v", err, err)
				}
				if typed.Message != tc.message {
					t.Fatalf("unexpected message: %q", typed.Message)
				}
			case *NotFoundError:
				var typed *NotFoundError
				if !errors.As(err, &typed) {
					t.Fatalf("expected NotFoundError, got %T %v", err, err)
				}
				if typed.Message != tc.message {
					t.Fatalf("unexpected message: %q", typed.Message)
				}
			case *NotEnoughSpaceError:
				var typed *NotEnoughSpaceError
				if !errors.As(err, &typed) {
					t.Fatalf("expected NotEnoughSpaceError, got %T %v", err, err)
				}
				if typed.Message != tc.message {
					t.Fatalf("unexpected message: %q", typed.Message)
				}
			}
		})
	}
}

func TestHandleEnvdApiErrorMapsTimeoutStatus(t *testing.T) {
	err := HandleEnvdApiError(502, []byte(`{"message":"sandbox unavailable"}`))
	if err == nil {
		t.Fatal("expected envd api error")
	}

	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("expected TimeoutError, got %T %v", err, err)
	}
	if timeoutErr.Message == "" {
		t.Fatal("expected timeout error message")
	}
}

func TestHandleEnvdApiErrorMapsRateLimitStatus(t *testing.T) {
	err := HandleEnvdApiError(429, []byte(`{"message":"too many requests"}`))
	if err == nil {
		t.Fatal("expected envd api error")
	}

	var sandboxErr *SandboxError
	if !errors.As(err, &sandboxErr) {
		t.Fatalf("expected SandboxError, got %T %v", err, err)
	}
	if sandboxErr.Message != "too many requests: The requests are being rate limited." {
		t.Fatalf("unexpected rate limit message: %q", sandboxErr.Message)
	}
}
