package e2b

import "github.com/superduck-ai/e2b-go-sdk/internal/shared"

type Logger = shared.Logger
type HTTPMiddleware = shared.HTTPMiddleware

func CreateRpcLogger(logger Logger) HTTPMiddleware {
	return shared.CreateRpcLogger(logger)
}

func CreateApiLogger(logger Logger) HTTPMiddleware {
	return shared.CreateApiLogger(logger)
}
