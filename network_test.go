package e2b

import (
	"os"
	"strings"
	"testing"
)

func TestAllTrafficConstantMatchesJsExportName(t *testing.T) {
	if ALL_TRAFFIC != "0.0.0.0/0" {
		t.Fatalf("expected ALL_TRAFFIC to match JS network export, got %q", ALL_TRAFFIC)
	}

	source, err := os.ReadFile("network.go")
	if err != nil {
		t.Fatalf("failed to read network.go: %v", err)
	}

	text := string(source)
	if strings.Contains(text, "const AllTraffic") {
		t.Fatal("did not expect legacy AllTraffic export to remain")
	}
}
