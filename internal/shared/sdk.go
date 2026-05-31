package shared

import (
	"context"
	"time"
)

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

func (e *TimeoutError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &e.SandboxError
}

func (e *InvalidArgumentError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &e.SandboxError
}

func (e *NotEnoughSpaceError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &e.SandboxError
}

func (e *NotFoundError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &e.SandboxError
}

func (e *FileNotFoundError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &e.NotFoundError
}

func (e *SandboxNotFoundError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &e.NotFoundError
}

func (e *GitUpstreamError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &e.SandboxError
}

func (e *TemplateError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &e.SandboxError
}

func (e *RateLimitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &e.SandboxError
}

type AuthenticationError struct {
	Message string
}

func (e *AuthenticationError) Error() string { return e.Message }

type GitAuthError struct{ AuthenticationError }

func (e *GitAuthError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &e.AuthenticationError
}

type BuildError struct {
	Message     string
	CallerTrace string
}

func (e *BuildError) Error() string { return e.Message }

type FileUploadError struct{ BuildError }

func (e *FileUploadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &e.BuildError
}

type VolumeError struct {
	Message string
}

func (e *VolumeError) Error() string { return e.Message }

func MergeContexts(primary context.Context, secondary context.Context) (context.Context, context.CancelFunc) {
	if primary == nil {
		primary = context.Background()
	}
	if secondary == nil {
		return primary, func() {}
	}

	if err := primary.Err(); err != nil {
		ctx, cancel := context.WithCancel(primary)
		cancel()
		return ctx, func() {}
	}
	if err := secondary.Err(); err != nil {
		ctx, cancel := context.WithCancel(primary)
		cancel()
		return ctx, func() {}
	}

	var deadline time.Time
	hasDeadline := false
	if d, ok := primary.Deadline(); ok {
		deadline = d
		hasDeadline = true
	}
	if d, ok := secondary.Deadline(); ok && (!hasDeadline || d.Before(deadline)) {
		deadline = d
		hasDeadline = true
	}

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if hasDeadline {
		ctx, cancel = context.WithDeadline(primary, deadline)
	} else {
		ctx, cancel = context.WithCancel(primary)
	}

	go func() {
		select {
		case <-primary.Done():
			cancel()
		case <-secondary.Done():
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}
