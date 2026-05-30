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
	var _ Blob = Blob([]byte("blob"))

	var _ ConnectionOpts = ConnectionOpts{}

	optsType := reflect.TypeOf(ConnectionOpts{})
	if _, ok := optsType.FieldByName("Logger"); !ok {
		t.Fatal("expected ConnectionOpts to expose Logger like shared JS connection options")
	}
	if _, ok := optsType.FieldByName("SandboxUrl"); !ok {
		t.Fatal("expected ConnectionOpts to expose SandboxUrl like shared JS connection options")
	}
	if field, ok := optsType.FieldByName("Debug"); !ok {
		t.Fatal("expected ConnectionOpts to expose Debug like shared JS/Python connection options")
	} else if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected ConnectionOpts.Debug to be *bool, got %v", field.Type)
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
	if field, ok := reflect.TypeOf(Volume{}).FieldByName("Debug"); !ok {
		t.Fatal("expected Volume to expose Debug like JS/Python volume instances")
	} else if field.Type != reflect.TypeOf((*bool)(nil)) {
		t.Fatalf("expected Volume.Debug to be *bool, got %v", field.Type)
	}

	writeFileMethod, ok := reflect.TypeOf(&Volume{}).MethodByName("WriteFile")
	if !ok {
		t.Fatal("expected Volume.WriteFile to exist")
	}
	if got, want := writeFileMethod.Type.In(3), reflect.TypeOf((*any)(nil)).Elem(); got != want {
		t.Fatalf("expected Volume.WriteFile data param to be any, got %v", got)
	}

	readFileMethod, ok := reflect.TypeOf(&Volume{}).MethodByName("ReadFile")
	if !ok {
		t.Fatal("expected Volume.ReadFile to exist")
	}
	if got, want := readFileMethod.Type.In(3), reflect.TypeOf(&VolumeReadOpts{}); got != want {
		t.Fatalf("expected Volume.ReadFile opts param to be *VolumeReadOpts, got %v", got)
	}
	if got, want := readFileMethod.Type.Out(0), reflect.TypeOf((*any)(nil)).Elem(); got != want {
		t.Fatalf("expected Volume.ReadFile to return any, got %v", got)
	}

	if _, ok := reflect.TypeOf(&Volume{}).MethodByName("ReadFileBytes"); ok {
		t.Fatal("did not expect Volume.ReadFileBytes to exist")
	}
	if _, ok := reflect.TypeOf(&Volume{}).MethodByName("ReadFileText"); ok {
		t.Fatal("did not expect Volume.ReadFileText to exist")
	}
	if _, ok := reflect.TypeOf(&Volume{}).MethodByName("ReadFileStream"); ok {
		t.Fatal("did not expect Volume.ReadFileStream to exist")
	}
	if _, ok := reflect.TypeOf(&Volume{}).MethodByName("ReadFileWithFormat"); ok {
		t.Fatal("did not expect Volume.ReadFileWithFormat to exist")
	}
}
