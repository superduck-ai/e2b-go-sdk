package shared

// Logger interface compatible with standard logging.
type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
}

type SandboxError struct {
	Message string
}

func (e *SandboxError) Error() string { return e.Message }

type TimeoutError struct{ SandboxError }
type InvalidArgumentError struct{ SandboxError }
type NotEnoughSpaceError struct{ SandboxError }

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
