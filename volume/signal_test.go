package volume

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func runSignalCancellation(t *testing.T, expectedMethod, expectedPath string, invoke func(signal context.Context, apiURL string) error) {
	t.Helper()

	requestStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if expectedMethod != "" && r.Method != expectedMethod {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != expectedPath {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		requestStarted <- struct{}{}
		<-release
	}))
	defer server.Close()

	signal, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- invoke(signal, server.URL)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for request to start")
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected signal context cancellation, got %T %v", err, err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for signal cancellation")
	}

	close(release)
}

func TestVolumeOptionSurfacesExposeSignalContext(t *testing.T) {
	if _, ok := reflect.TypeOf(ConnectionOpts{}).FieldByName("Signal"); !ok {
		t.Fatal("expected ConnectionOpts to expose Signal like shared connection options")
	}
	if _, ok := reflect.TypeOf(VolumeApiOpts{}).FieldByName("Signal"); !ok {
		t.Fatal("expected VolumeApiOpts to expose Signal like JS volume connection options")
	}
	if _, ok := reflect.TypeOf(VolumeWriteOptions{}).FieldByName("Signal"); !ok {
		t.Fatal("expected VolumeWriteOptions to expose Signal alongside request options")
	}
}

func TestVolumeApisHonorSignalContext(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		runSignalCancellation(t, http.MethodPost, "/volumes", func(signal context.Context, apiURL string) error {
			timeout := 1000
			_, err := Create(context.Background(), "test-volume", &ConnectionOpts{
				ApiKey:           "e2b_0000000000000000000000000000000000000000",
				ApiUrl:           apiURL,
				RequestTimeoutMs: &timeout,
				Signal:           signal,
			})
			return err
		})
	})

	t.Run("get_info", func(t *testing.T) {
		runSignalCancellation(t, http.MethodGet, "/volumecontent/vol-1/path", func(signal context.Context, apiURL string) error {
			timeout := 1000
			v := testVolumeClient(apiURL)
			_, err := v.GetInfo(context.Background(), "/file.txt", &VolumeApiOpts{
				ApiUrl:           apiURL,
				RequestTimeoutMs: &timeout,
				Signal:           signal,
			})
			return err
		})
	})

	t.Run("read_file_stream", func(t *testing.T) {
		runSignalCancellation(t, http.MethodGet, "/volumecontent/vol-1/file", func(signal context.Context, apiURL string) error {
			timeout := 1000
			v := testVolumeClient(apiURL)
			_, err := v.ReadFile(context.Background(), "/file.txt", &VolumeReadOpts{
				VolumeApiOpts: VolumeApiOpts{
					ApiUrl:           apiURL,
					RequestTimeoutMs: &timeout,
					Signal:           signal,
				},
				Format: ReadFileFormatStream,
			})
			return err
		})
	})

	t.Run("write_file", func(t *testing.T) {
		runSignalCancellation(t, http.MethodPut, "/volumecontent/vol-1/file", func(signal context.Context, apiURL string) error {
			timeout := 1000
			v := testVolumeClient(apiURL)
			_, err := v.WriteFile(context.Background(), "/file.txt", http.NoBody, &VolumeWriteOptions{
				ApiUrl:           apiURL,
				RequestTimeoutMs: &timeout,
				Signal:           signal,
			})
			return err
		})
	})
}
