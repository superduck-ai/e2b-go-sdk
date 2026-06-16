package applecontainer

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrVolumeNotFound = errors.New("volume not found")

const (
	xpcProtocolErrorKey = "com.apple.container.xpc.error"

	appleErrorCodeInternalError = "internal_error"
	appleErrorCodeInvalidArg    = "invalid_argument"
	appleErrorCodeInvalidState  = "invalid_state"
	appleErrorCodeNotFound      = "not_found"
)

type xpcProtocolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *xpcProtocolError) Error() string {
	if e == nil {
		return ""
	}
	code := strings.TrimSpace(e.Code)
	message := strings.TrimSpace(e.Message)
	switch {
	case code != "" && message != "":
		return fmt.Sprintf("apple container XPC error %s: %s", code, message)
	case code != "":
		return fmt.Sprintf("apple container XPC error %s", code)
	case message != "":
		return "apple container XPC error: " + message
	default:
		return "apple container XPC error"
	}
}

func decodeXPCProtocolError(data []byte) (*xpcProtocolError, error) {
	var protocolErr xpcProtocolError
	if err := json.Unmarshal(data, &protocolErr); err != nil {
		return nil, fmt.Errorf("decode apple container XPC error: %w", err)
	}
	if strings.TrimSpace(protocolErr.Code) == "" && strings.TrimSpace(protocolErr.Message) == "" {
		return nil, fmt.Errorf("decode apple container XPC error: empty error payload")
	}
	return &protocolErr, nil
}

func normalizeAppleErrorCode(code string) string {
	code = strings.Trim(strings.ToLower(strings.TrimSpace(code)), ". ")
	code = strings.NewReplacer("-", "_", " ", "_").Replace(code)
	switch code {
	case "internalerror":
		return appleErrorCodeInternalError
	case "invalidargument":
		return appleErrorCodeInvalidArg
	case "invalidstate":
		return appleErrorCodeInvalidState
	case "notfound":
		return appleErrorCodeNotFound
	default:
		return code
	}
}

func appleErrorHasCode(err error, codes ...string) bool {
	var protocolErr *xpcProtocolError
	if !errors.As(err, &protocolErr) {
		return false
	}
	got := normalizeAppleErrorCode(protocolErr.Code)
	for _, code := range codes {
		if got == normalizeAppleErrorCode(code) {
			return true
		}
	}
	return false
}

func isAppleNotFound(err error) bool {
	if appleErrorHasCode(err, appleErrorCodeNotFound) {
		return true
	}

	var protocolErr *xpcProtocolError
	if !errors.As(err, &protocolErr) {
		return false
	}
	code := normalizeAppleErrorCode(protocolErr.Code)
	message := strings.ToLower(protocolErr.Message)
	return (code == "" || code == appleErrorCodeInternalError || code == appleErrorCodeInvalidArg) &&
		strings.Contains(message, "not found") &&
		(strings.Contains(message, "container") || strings.Contains(message, "volume"))
}

func isAppleAlreadyStopped(err error) bool {
	if !appleErrorHasCode(err, appleErrorCodeInvalidState, appleErrorCodeInternalError) {
		return false
	}
	var protocolErr *xpcProtocolError
	if !errors.As(err, &protocolErr) {
		return false
	}
	message := strings.ToLower(protocolErr.Message)
	return strings.Contains(message, "container") &&
		(strings.Contains(message, "not running") || strings.Contains(message, "already stopped") || strings.Contains(message, "is stopped"))
}
