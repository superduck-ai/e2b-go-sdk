package e2b

import "fmt"

// FormatSandboxTimeoutError wraps message with sandbox timeout hint
func FormatSandboxTimeoutError(message string) *TimeoutError {
	return &TimeoutError{
		SandboxError{
			Message: fmt.Sprintf("%s. You can increase the timeout by passing a longer timeout to the sandbox or by calling sandbox.SetTimeout()", message),
		},
	}
}

type SandboxError struct {
	Message string
}

func (e *SandboxError) Error() string { return e.Message }

type TimeoutError struct{ SandboxError }
type InvalidArgumentError struct{ SandboxError }
type NotEnoughSpaceError struct{ SandboxError }

// Deprecated: Use FileNotFoundError or SandboxNotFoundError instead.
type NotFoundError struct{ SandboxError }
type FileNotFoundError struct{ NotFoundError }
type SandboxNotFoundError struct{ NotFoundError }
type GitUpstreamError struct{ SandboxError }
type TemplateError struct{ SandboxError }
type RateLimitError struct{ SandboxError }

type AuthenticationError struct {
	Message string
}

func (e *AuthenticationError) Error() string { return e.Message }

type GitAuthError struct{ AuthenticationError }

type BuildError struct {
	Message string
}

func (e *BuildError) Error() string { return e.Message }

type FileUploadError struct{ BuildError }

type VolumeError struct {
	Message string
}

func (e *VolumeError) Error() string { return e.Message }
