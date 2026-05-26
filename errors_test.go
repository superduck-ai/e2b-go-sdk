package e2b

import (
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
