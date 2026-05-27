package e2b

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestErrorsDoNotExposeInternalTimeoutFormatter(t *testing.T) {
	source, err := os.ReadFile("errors.go")
	if err != nil {
		t.Fatalf("failed to read errors.go: %v", err)
	}

	text := string(source)
	if strings.Contains(text, "func FormatSandboxTimeoutError(") {
		t.Fatal("did not expect FormatSandboxTimeoutError to be exported")
	}
}

func TestFileNotFoundErrorMatchesDeprecatedNotFoundAndSandboxError(t *testing.T) {
	err := &FileNotFoundError{
		NotFoundError: NotFoundError{
			SandboxError: SandboxError{Message: "missing file"},
		},
	}

	var fileErr *FileNotFoundError
	if !errors.As(err, &fileErr) {
		t.Fatalf("expected FileNotFoundError, got %T %v", err, err)
	}

	var notFoundErr *NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected deprecated NotFoundError, got %T %v", err, err)
	}

	var sandboxErr *SandboxError
	if !errors.As(err, &sandboxErr) {
		t.Fatalf("expected SandboxError, got %T %v", err, err)
	}
}
