package envd

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestHandleRequestTimeoutError(t *testing.T) {
	err := HandleRequestTimeoutError()
	rpcErr, ok := err.(*RpcError)
	if !ok {
		t.Fatalf("expected RpcError, got %T", err)
	}
	if rpcErr.Code != "canceled" {
		t.Fatalf("expected canceled code, got %q", rpcErr.Code)
	}
	if rpcErr.Message == "" {
		t.Fatal("expected timeout message")
	}
}

func TestHandleStreamContextError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{name: "deadline", err: context.DeadlineExceeded, code: "deadline_exceeded"},
		{name: "canceled", err: context.Canceled, code: "canceled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := HandleStreamContextError(tt.err)
			rpcErr, ok := err.(*RpcError)
			if !ok {
				t.Fatalf("expected RpcError, got %T", err)
			}
			if rpcErr.Code != tt.code {
				t.Fatalf("expected code %q, got %q", tt.code, rpcErr.Code)
			}
			if rpcErr.Message == "" {
				t.Fatal("expected error message")
			}
		})
	}
}

func TestParseConnectEndStreamError(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		code    string
	}{
		{
			name:    "string code",
			payload: `{"error":{"code":"not_found","message":"missing"}}`,
			code:    "not_found",
		},
		{
			name:    "numeric code",
			payload: `{"error":{"code":14,"message":"down"}}`,
			code:    "unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ParseConnectEndStreamError([]byte(tt.payload))
			rpcErr, ok := err.(*RpcError)
			if !ok {
				t.Fatalf("expected RpcError, got %T", err)
			}
			if rpcErr.Code != tt.code {
				t.Fatalf("expected code %q, got %q", tt.code, rpcErr.Code)
			}
		})
	}
}

func TestRetryRPCTransportError(t *testing.T) {
	t.Run("retries_expected_transport_errors", func(t *testing.T) {
		attempts := 0
		err := RetryRPCTransportError(context.Background(), func() error {
			attempts++
			if attempts < 3 {
				return io.ErrUnexpectedEOF
			}
			return nil
		})
		if err != nil {
			t.Fatalf("expected retry to eventually succeed, got %v", err)
		}
		if attempts != 3 {
			t.Fatalf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("does_not_retry_unexpected_errors", func(t *testing.T) {
		attempts := 0
		want := errors.New("boom")
		err := RetryRPCTransportError(context.Background(), func() error {
			attempts++
			return want
		})
		if !errors.Is(err, want) {
			t.Fatalf("expected original error, got %v", err)
		}
		if attempts != 1 {
			t.Fatalf("expected single attempt, got %d", attempts)
		}
	})

	t.Run("does_not_retry_after_context_cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		attempts := 0
		err := RetryRPCTransportError(ctx, func() error {
			attempts++
			return io.ErrUnexpectedEOF
		})
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("expected original transport error, got %v", err)
		}
		if attempts != 1 {
			t.Fatalf("expected canceled context to stop retries, got %d attempts", attempts)
		}
	})
}
