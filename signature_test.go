package e2b

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
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

func TestGetSignatureMatchesJsStaticGoldenValue(t *testing.T) {
	signature, expiration, err := GetSignature(
		"hello.txt",
		"read",
		"user",
		0,
		"0tQG31xiMp0IOQfaz9dcwi72L1CPo8e0",
	)
	if err != nil {
		t.Fatalf("GetSignature returned error: %v", err)
	}
	if expiration != nil {
		t.Fatalf("expected nil expiration for zero expiration input, got %v", *expiration)
	}
	if signature != "v1_gUtH/s9YCJWgCizjfUxuWfhFE4QSydOWEIIvfLwDr6E" {
		t.Fatalf("unexpected static signature: %q", signature)
	}
}

func TestGetSignatureMatchesJsExpirationFormatting(t *testing.T) {
	signature, expiration, err := GetSignature(
		"/home/user/hello.txt",
		"read",
		"root",
		120,
		"test-token",
	)
	if err != nil {
		t.Fatalf("GetSignature returned error: %v", err)
	}
	if expiration == nil {
		t.Fatal("expected expiration to be set")
	}

	raw := fmt.Sprintf(
		"%s:%s:%s:%s:%d",
		"/home/user/hello.txt",
		"read",
		"root",
		"test-token",
		*expiration,
	)
	sum := sha256.Sum256([]byte(raw))
	expected := "v1_" + base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString(sum[:])
	if signature != expected {
		t.Fatalf("unexpected signature with expiration: got %q want %q", signature, expected)
	}

	now := time.Now().Unix()
	if got := *expiration - now; got < 118 || got > 122 {
		t.Fatalf("expected expiration to be about 120s in the future, got delta %d", got)
	}
}
