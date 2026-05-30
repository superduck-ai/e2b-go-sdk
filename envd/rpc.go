package envd

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const rpcTransportRetries = 3

// RpcError represents an error from an envd RPC call.
type RpcError struct {
	Code    string
	Message string
}

func (e *RpcError) Error() string {
	return fmt.Sprintf("envd RPC error [%s]: %s", e.Code, e.Message)
}

// HandleRpcError maps gRPC/Connect error codes to an RpcError.
func HandleRpcError(code string, message string) error {
	return &RpcError{Code: code, Message: message}
}

func EncodeConnectEnvelope(payload []byte) []byte {
	envelope := make([]byte, 5+len(payload))
	binary.BigEndian.PutUint32(envelope[1:5], uint32(len(payload)))
	copy(envelope[5:], payload)
	return envelope
}

func HandleRequestTimeoutError() error {
	return HandleRpcError("canceled", "request timeout exceeded before receiving first stream event: This error is likely due to exceeding 'requestTimeoutMs'. You can pass the request timeout value as an option when making the request.")
}

func HandleStreamContextError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return HandleRpcError("deadline_exceeded", "stream deadline exceeded: This error is likely due to exceeding 'timeoutMs' — the total time a long running request (like command execution or directory watch) can be active. It can be modified by passing 'timeoutMs' when making the request. Use '0' to disable the timeout.")
	case errors.Is(err, context.Canceled):
		return HandleRpcError("canceled", "stream canceled: This error is likely due to exceeding 'requestTimeoutMs'. You can pass the request timeout value as an option when making the request.")
	default:
		return err
	}
}

func RetryRPCTransportError(ctx context.Context, fn func() error) error {
	return RetryRPCTransportErrorWithBeforeAttempt(ctx, nil, fn)
}

func RetryRPCTransportErrorWithBeforeAttempt(ctx context.Context, beforeAttempt func() error, fn func() error) error {
	_, err := retryRPCTransport(ctx, func() (struct{}, error) {
		if beforeAttempt != nil {
			if err := beforeAttempt(); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, fn()
	})
	return err
}

func retryRPCTransport[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var zero T
	for attempt := 0; attempt <= rpcTransportRetries; attempt++ {
		value, err := fn()
		if err == nil {
			return value, nil
		}
		if !isRetryableRPCTransportError(err) || ctx == nil || ctx.Err() != nil || attempt == rpcTransportRetries {
			return value, err
		}
	}
	return zero, nil
}

func isRetryableRPCTransportError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "server closed idle connection") ||
		strings.Contains(message, "transport connection broken")
}

func ParseConnectEndStreamError(payload []byte) error {
	if len(payload) == 0 {
		return nil
	}

	var endStream struct {
		Error *struct {
			Code    interface{} `json:"code"`
			Message string      `json:"message"`
		} `json:"error"`
		Code    interface{} `json:"code"`
		Message string      `json:"message"`
	}
	if err := json.Unmarshal(payload, &endStream); err != nil {
		return err
	}

	codeValue := endStream.Code
	message := endStream.Message
	if endStream.Error != nil {
		codeValue = endStream.Error.Code
		if message == "" {
			message = endStream.Error.Message
		}
	}

	code := normalizeRPCCode(codeValue)
	if code == "" && message == "" {
		return nil
	}
	if code == "" {
		code = "unknown"
	}
	return HandleRpcError(code, message)
}

func normalizeRPCCode(value interface{}) string {
	switch v := value.(type) {
	case string:
		v = strings.TrimSpace(strings.ToLower(v))
		v = strings.ReplaceAll(v, "-", "_")
		v = strings.ReplaceAll(v, " ", "_")
		return v
	case float64:
		switch int(v) {
		case 1:
			return "canceled"
		case 3:
			return "invalid_argument"
		case 4:
			return "deadline_exceeded"
		case 5:
			return "not_found"
		case 14:
			return "unavailable"
		case 16:
			return "unauthenticated"
		default:
			return fmt.Sprintf("%d", int(v))
		}
	default:
		return ""
	}
}

// AuthenticationHeader returns Basic auth header for envd.
func AuthenticationHeader(envdVersion string, username string) map[string]string {
	if username == "" && versionGTE(envdVersion, EnvdDefaultUser) {
		return map[string]string{}
	}
	if username == "" {
		username = "user"
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(username + ":"))
	return map[string]string{
		"Authorization": "Basic " + encoded,
	}
}

func versionGTE(version, minVersion string) bool {
	if version == "" {
		return true
	}

	var major1, minor1, patch1 int
	var major2, minor2, patch2 int
	fmt.Sscanf(version, "%d.%d.%d", &major1, &minor1, &patch1)
	fmt.Sscanf(minVersion, "%d.%d.%d", &major2, &minor2, &patch2)

	if major1 != major2 {
		return major1 > major2
	}
	if minor1 != minor2 {
		return minor1 > minor2
	}
	return patch1 >= patch2
}
