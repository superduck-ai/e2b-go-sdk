package e2b

import (
	"os"
	"strings"
	"testing"
)

func TestUtilsAreNotExported(t *testing.T) {
	source, err := os.ReadFile("utils.go")
	if err != nil {
		t.Fatalf("failed to read utils.go: %v", err)
	}

	text := string(source)
	for _, name := range []string{
		"Sha256Hash",
		"TimeoutToSeconds",
		"StripAnsi",
		"Wait",
		"ShellQuote",
	} {
		if strings.Contains(text, "func "+name+"(") {
			t.Fatalf("did not expect %s to be exported", name)
		}
	}
}
