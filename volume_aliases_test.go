package e2b

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	rootvol "github.com/e2b-dev/e2b-go-sdk/volume"
)

func TestRootAliasesExposeJsStyleVolumeTypes(t *testing.T) {
	if VolumeFileTypeUnknown != rootvol.VolumeFileTypeUnknown ||
		VolumeFileTypeFile != rootvol.VolumeFileTypeFile ||
		VolumeFileTypeDirectory != rootvol.VolumeFileTypeDirectory ||
		VolumeFileTypeSymlink != rootvol.VolumeFileTypeSymlink {
		t.Fatalf("expected root volume file type constants to match volume package exports")
	}

	var _ *Volume = (*rootvol.Volume)(nil)
	var _ VolumeFileType = rootvol.VolumeFileTypeFile
	var _ VolumeInfo = rootvol.VolumeInfo{}
	var _ VolumeAndToken = rootvol.VolumeAndToken{}
	var _ VolumeEntryStat = rootvol.VolumeEntryStat{}
	var _ VolumeMetadataOptions = rootvol.VolumeMetadataOptions{}
	var _ VolumeWriteOptions = rootvol.VolumeWriteOptions{}
	var _ VolumeApiOpts = rootvol.VolumeApiOpts{}
	var _ VolumeConnectionConfig = rootvol.VolumeConnectionConfig{}
}

func TestRootVolumeErrorsRemainAssignableToSdkTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	_, err := rootvol.GetInfo(context.Background(), "vol-1", &rootvol.ConnectionOpts{
		ApiKey: "test-api-key",
		ApiUrl: server.URL,
	})
	if err == nil {
		t.Fatal("expected volume info error")
	}

	var notFoundErr *NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T %v", err, err)
	}
	if notFoundErr.Message != "Volume vol-1 not found" {
		t.Fatalf("unexpected error message: %q", notFoundErr.Message)
	}
}
