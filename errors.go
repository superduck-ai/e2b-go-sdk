package e2b

import (
	"fmt"

	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

// formatSandboxTimeoutError wraps message with sandbox timeout hint.
func formatSandboxTimeoutError(message string) *TimeoutError {
	return &TimeoutError{
		SandboxError: SandboxError{
			Message: fmt.Sprintf("%s. You can increase the timeout by passing a longer timeout to the sandbox or by calling sandbox.SetTimeout()", message),
		},
	}
}

type SandboxError = shared.SandboxError
type TimeoutError = shared.TimeoutError
type InvalidArgumentError = shared.InvalidArgumentError
type NotEnoughSpaceError = shared.NotEnoughSpaceError

// Deprecated: Use FileNotFoundError or SandboxNotFoundError instead.
type NotFoundError = shared.NotFoundError
type FileNotFoundError = shared.FileNotFoundError
type SandboxNotFoundError = shared.SandboxNotFoundError
type GitUpstreamError = shared.GitUpstreamError
type TemplateError = shared.TemplateError
type RateLimitError = shared.RateLimitError

type AuthenticationError = shared.AuthenticationError
type GitAuthError = shared.GitAuthError
type BuildError = shared.BuildError
type FileUploadError = shared.FileUploadError
type VolumeError = shared.VolumeError
