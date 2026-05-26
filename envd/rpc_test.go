package envd

import (
	"context"
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
