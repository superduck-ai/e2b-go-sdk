package e2b

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	rootvol "github.com/superduck-ai/e2b-go-sdk/volume"
)

func TestRootAliasesExposeJsStyleVolumeTypes(t *testing.T) {
	if VolumeFileTypeUnknown != rootvol.VolumeFileTypeUnknown ||
		VolumeFileTypeFile != rootvol.VolumeFileTypeFile ||
		VolumeFileTypeDirectory != rootvol.VolumeFileTypeDirectory ||
		VolumeFileTypeSymlink != rootvol.VolumeFileTypeSymlink {
		t.Fatalf("expected root volume file type constants to match volume package exports")
	}
	if ReadFileFormatText != rootvol.ReadFileFormatText ||
		ReadFileFormatBytes != rootvol.ReadFileFormatBytes ||
		ReadFileFormatStream != rootvol.ReadFileFormatStream ||
		ReadFileFormatBlob != rootvol.ReadFileFormatBlob {
		t.Fatalf("expected root volume read format constants to match volume package exports")
	}

	var _ *Volume = (*rootvol.Volume)(nil)
	var _ ReadFileFormat = rootvol.ReadFileFormat("")
	var _ VolumeFileType = rootvol.VolumeFileTypeFile
	var _ VolumeInfo = rootvol.VolumeInfo{}
	var _ VolumeAndToken = rootvol.VolumeAndToken{}
	var _ VolumeEntryStat = rootvol.VolumeEntryStat{}
	var _ VolumeMetadataOptions = rootvol.VolumeMetadataOptions{}
	var _ VolumeWriteOptions = rootvol.VolumeWriteOptions{}
	var _ VolumeApiOpts = rootvol.VolumeApiOpts{}
	var _ VolumeReadOpts = rootvol.VolumeReadOpts{}
	var _ VolumeListOpts = rootvol.VolumeListOpts{}
	var _ VolumeConnectionOpts = rootvol.ConnectionOpts{}
	var _ VolumeConnectionConfig = rootvol.VolumeConnectionConfig{}
}

func TestRootVolumeStaticWrappersDelegateToVolumePackage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if r.URL.Path != "/volumes" {
				t.Fatalf("unexpected create path: %s", r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"volumeID":"vol-1","name":"test-volume","token":"secret-token"}`))
		case http.MethodGet:
			switch r.URL.Path {
			case "/volumes":
				_, _ = w.Write([]byte(`[{"volumeID":"vol-1","name":"test-volume"}]`))
			case "/volumes/vol-1":
				_, _ = w.Write([]byte(`{"volumeID":"vol-1","name":"test-volume","token":"secret-token"}`))
			default:
				t.Fatalf("unexpected GET path: %s", r.URL.Path)
			}
		case http.MethodDelete:
			if r.URL.Path != "/volumes/vol-1" {
				t.Fatalf("unexpected destroy path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	defer server.Close()

	opts := &VolumeConnectionOpts{
		ApiKey: "e2b_0000000000000000000000000000000000000000",
		ApiUrl: server.URL,
	}

	created, err := CreateVolume(context.Background(), "test-volume", opts)
	if err != nil {
		t.Fatalf("CreateVolume returned error: %v", err)
	}
	if created == nil || created.VolumeID != "vol-1" || created.Name != "test-volume" || created.Token != "secret-token" {
		t.Fatalf("unexpected CreateVolume response: %#v", created)
	}

	connected, err := ConnectVolume(context.Background(), "vol-1", opts)
	if err != nil {
		t.Fatalf("ConnectVolume returned error: %v", err)
	}
	if connected == nil || connected.VolumeID != "vol-1" || connected.Name != "test-volume" || connected.Token != "secret-token" {
		t.Fatalf("unexpected ConnectVolume response: %#v", connected)
	}

	info, err := GetVolumeInfo(context.Background(), "vol-1", opts)
	if err != nil {
		t.Fatalf("GetVolumeInfo returned error: %v", err)
	}
	if info == nil || info.VolumeID != "vol-1" || info.Name != "test-volume" || info.Token != "secret-token" {
		t.Fatalf("unexpected GetVolumeInfo response: %#v", info)
	}

	list, err := ListVolumes(context.Background(), opts)
	if err != nil {
		t.Fatalf("ListVolumes returned error: %v", err)
	}
	if len(list) != 1 || list[0].VolumeID != "vol-1" || list[0].Name != "test-volume" {
		t.Fatalf("unexpected ListVolumes response: %#v", list)
	}

	destroyed, err := DestroyVolume(context.Background(), "vol-1", opts)
	if err != nil {
		t.Fatalf("DestroyVolume returned error: %v", err)
	}
	if !destroyed {
		t.Fatal("expected DestroyVolume to return true")
	}
}

func TestRootVolumeErrorsRemainAssignableToSdkTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"missing"}`, http.StatusNotFound)
	}))
	defer server.Close()

	_, err := rootvol.GetInfo(context.Background(), "vol-1", &rootvol.ConnectionOpts{
		ApiKey: "e2b_0000000000000000000000000000000000000000",
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
