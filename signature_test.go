package e2b

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGetSignatureDoesNotExposeHelperTypes(t *testing.T) {
	source, err := os.ReadFile("signature.go")
	if err != nil {
		t.Fatalf("failed to read signature.go: %v", err)
	}

	text := string(source)
	if strings.Contains(text, "type SignatureOpts struct") {
		t.Fatal("did not expect SignatureOpts to be exported")
	}
	if strings.Contains(text, "type SignatureResult struct") {
		t.Fatal("did not expect SignatureResult to be exported")
	}

	fnType := reflect.TypeOf(GetSignature)
	if fnType.NumIn() != 5 {
		t.Fatalf("expected GetSignature to take 5 arguments, got %d", fnType.NumIn())
	}
	if fnType.NumOut() != 3 {
		t.Fatalf("expected GetSignature to return 3 values, got %d", fnType.NumOut())
	}
}
