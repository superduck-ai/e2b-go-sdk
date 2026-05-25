package envd

import (
	"encoding/base64"
	"fmt"
)

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

// AuthenticationHeader returns Basic auth header for envd.
func AuthenticationHeader(envdVersion string, username string) map[string]string {
	if username == "" {
		username = "user"
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(username + ":"))
	return map[string]string{
		"Authorization": "Basic " + encoded,
	}
}
