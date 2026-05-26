package volume

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestVolumeInternalsDoNotExposeJsInternalHelpers(t *testing.T) {
	files := []string{
		"schema.go",
		"client.go",
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		text := string(data)

		disallowed := []string{
			"const FileTimeoutMs",
			"type ListDirResponse struct",
			"type CreateDirResponse struct",
			"type StatResponse struct",
			"type UpdateMetadataRequest struct",
			"type VolumeApiClient struct",
			"type VolumeApiError struct",
			"func NewVolumeApiClient(",
		}

		for _, needle := range disallowed {
			if strings.Contains(text, needle) {
				t.Fatalf("did not expect volume package to export %q in %s", needle, file)
			}
		}
	}
}

func TestVolumePackageExposesJsStyleStaticSurfaceNames(t *testing.T) {
	var _ ConnectionOpts = ConnectionOpts{}

	optsType := reflect.TypeOf(ConnectionOpts{})
	if _, ok := optsType.FieldByName("Logger"); !ok {
		t.Fatal("expected ConnectionOpts to expose Logger like shared JS connection options")
	}
	if _, ok := optsType.FieldByName("SandboxUrl"); !ok {
		t.Fatal("expected ConnectionOpts to expose SandboxUrl like shared JS connection options")
	}

	if got := reflect.TypeOf(GetInfo); got.Out(0) != reflect.TypeOf((*VolumeAndToken)(nil)) {
		t.Fatalf("expected GetInfo to return *VolumeAndToken, got %v", got.Out(0))
	}
	if got := reflect.TypeOf(Create).In(1).String(); got != "string" {
		t.Fatalf("expected Create to accept volume name as second parameter, got %s", got)
	}
	if got := reflect.TypeOf(Create); got.In(2) != reflect.TypeOf((*ConnectionOpts)(nil)) {
		t.Fatalf("expected Create to accept *ConnectionOpts, got %v", got.In(2))
	}
}
