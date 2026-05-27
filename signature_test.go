package e2b

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestGetSignatureUsesNegativeExpirationLikeJsAndPython(t *testing.T) {
	_, expiration, err := GetSignature("hello.txt", "read", "", -10, "token")
	if err != nil {
		t.Fatalf("GetSignature returned error: %v", err)
	}
	if expiration == nil {
		t.Fatal("expected negative expiration to produce signature_expiration")
	}
	if *expiration >= time.Now().Unix() {
		t.Fatalf("expected expiration to be in the past, got %d", *expiration)
	}
}
