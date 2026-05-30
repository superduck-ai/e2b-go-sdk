package volume

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

func readVolumeTextValue(t *testing.T, v *Volume, path string, opts *VolumeApiOpts) string {
	t.Helper()
	readOpts := &VolumeReadOpts{}
	if opts != nil {
		readOpts.VolumeApiOpts = *opts
	}
	value, err := v.ReadFile(context.Background(), path, readOpts)
	if err != nil {
		t.Fatalf("ReadFile(text) returned error: %v", err)
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("expected string, got %T", value)
	}
	return text
}

func readVolumeBytesValue(t *testing.T, v *Volume, path string, opts *VolumeApiOpts) []byte {
	t.Helper()
	readOpts := &VolumeReadOpts{Format: ReadFileFormatBytes}
	if opts != nil {
		readOpts.VolumeApiOpts = *opts
	}
	value, err := v.ReadFile(context.Background(), path, readOpts)
	if err != nil {
		t.Fatalf("ReadFile(bytes) returned error: %v", err)
	}
	data, ok := value.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", value)
	}
	return data
}

func readVolumeStreamValue(t *testing.T, v *Volume, path string, opts *VolumeApiOpts) io.ReadCloser {
	t.Helper()
	readOpts := &VolumeReadOpts{Format: ReadFileFormatStream}
	if opts != nil {
		readOpts.VolumeApiOpts = *opts
	}
	value, err := v.ReadFile(context.Background(), path, readOpts)
	if err != nil {
		t.Fatalf("ReadFile(stream) returned error: %v", err)
	}
	stream, ok := value.(io.ReadCloser)
	if !ok {
		t.Fatalf("expected io.ReadCloser, got %T", value)
	}
	return stream
}

func readVolumeBlobValue(t *testing.T, v *Volume, path string, opts *VolumeApiOpts) Blob {
	t.Helper()
	readOpts := &VolumeReadOpts{Format: ReadFileFormatBlob}
	if opts != nil {
		readOpts.VolumeApiOpts = *opts
	}
	value, err := v.ReadFile(context.Background(), path, readOpts)
	if err != nil {
		t.Fatalf("ReadFile(blob) returned error: %v", err)
	}
	blob, ok := value.(Blob)
	if !ok {
		t.Fatalf("expected Blob, got %T", value)
	}
	return blob
}

func TestVolumeFileParityWriteAndReadText(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		switch r.Method {
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			gotBody = string(data)
			respondEntry(t, w, VolumeEntryStat{
				Name:  "test.txt",
				Path:  "/test.txt",
				Type:  VolumeFileTypeFile,
				Atime: time.Now(),
				Mtime: time.Now(),
				Ctime: time.Now(),
			})
		case http.MethodGet:
			_, _ = w.Write([]byte("Hello, World!"))
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	stat, err := v.WriteFile(context.Background(), "/test.txt", "Hello, World!", &VolumeWriteOptions{ApiUrl: server.URL})
	if err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if stat == nil || stat.Type != VolumeFileTypeFile || stat.Path != "/test.txt" {
		t.Fatalf("unexpected write stat: %#v", stat)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("expected PUT, got %s", gotMethod)
	}
	if gotBody != "Hello, World!" {
		t.Fatalf("unexpected request body: %q", gotBody)
	}
	if !strings.HasPrefix(gotPath, "/volumecontent/vol-1/file?") {
		t.Fatalf("unexpected request path: %q", gotPath)
	}
	if value, _ := url.ParseQuery(strings.SplitN(gotPath, "?", 2)[1]); value.Get("path") != "/test.txt" {
		t.Fatalf("expected path query /test.txt, got %q", value.Get("path"))
	}

	readValue, err := v.ReadFile(context.Background(), "/test.txt", testVolumeReadOpts(server.URL))
	if err != nil {
		t.Fatalf("ReadFile default returned error: %v", err)
	}
	read := readValue.(string)
	if read != "Hello, World!" {
		t.Fatalf("unexpected read content: %q", read)
	}
}

func TestVolumeFileParityWriteAndReadBytes(t *testing.T) {
	content := []byte{0, 1, 2, 3, 4}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			if !bytes.Equal(data, content) {
				t.Fatalf("unexpected request body: %#v", data)
			}
			respondEntry(t, w, VolumeEntryStat{
				Name:  "binary.bin",
				Path:  "/binary.bin",
				Type:  VolumeFileTypeFile,
				Atime: time.Now(),
				Mtime: time.Now(),
				Ctime: time.Now(),
			})
		case http.MethodGet:
			_, _ = w.Write(content)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	if _, err := v.WriteFile(context.Background(), "/binary.bin", content, &VolumeWriteOptions{ApiUrl: server.URL}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	readValue, err := v.ReadFile(context.Background(), "/binary.bin", &VolumeReadOpts{
		VolumeApiOpts: *testVolumeApiOpts(server.URL),
		Format:        ReadFileFormatBytes,
	})
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	read := readValue.([]byte)
	if !bytes.Equal(read, content) {
		t.Fatalf("unexpected read bytes: %#v", read)
	}
}

func TestVolumeFileParityWriteAndReadStreamInput(t *testing.T) {
	content := "stream content"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			if string(data) != content {
				t.Fatalf("unexpected request body: %q", string(data))
			}
			respondEntry(t, w, VolumeEntryStat{
				Name:  "stream.txt",
				Path:  "/stream.txt",
				Type:  VolumeFileTypeFile,
				Atime: time.Now(),
				Mtime: time.Now(),
				Ctime: time.Now(),
			})
		case http.MethodGet:
			_, _ = w.Write([]byte(content))
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	if _, err := v.WriteFile(context.Background(), "/stream.txt", strings.NewReader(content), &VolumeWriteOptions{ApiUrl: server.URL}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	readValue, err := v.ReadFile(context.Background(), "/stream.txt", testVolumeReadOpts(server.URL))
	if err != nil {
		t.Fatalf("ReadFile default returned error: %v", err)
	}
	read := readValue.(string)
	if read != content {
		t.Fatalf("unexpected read content: %q", read)
	}
}

func TestVolumeFileParityWriteAndReadBlob(t *testing.T) {
	content := Blob([]byte("blob content"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			if got := string(data); got != "blob content" {
				t.Fatalf("unexpected request body: %q", got)
			}
			respondEntry(t, w, VolumeEntryStat{
				Name:  "blob.txt",
				Path:  "/blob.txt",
				Type:  VolumeFileTypeFile,
				Atime: time.Now(),
				Mtime: time.Now(),
				Ctime: time.Now(),
			})
		case http.MethodGet:
			_, _ = w.Write([]byte(content))
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	if _, err := v.WriteFile(context.Background(), "/blob.txt", content, &VolumeWriteOptions{ApiUrl: server.URL}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	readBlob := readVolumeBlobValue(t, v, "/blob.txt", testVolumeApiOpts(server.URL))
	if got := readBlob.Text(); got != "blob content" {
		t.Fatalf("unexpected blob payload: %q", got)
	}
}

func TestVolumeFileParityWriteAndReadEmptyFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			if len(data) != 0 {
				t.Fatalf("expected empty write body, got %q", string(data))
			}
			respondEntry(t, w, VolumeEntryStat{
				Name:  "empty.txt",
				Path:  "/empty.txt",
				Type:  VolumeFileTypeFile,
				Atime: time.Now(),
				Mtime: time.Now(),
				Ctime: time.Now(),
			})
		case http.MethodGet:
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	if _, err := v.WriteFile(context.Background(), "/empty.txt", "", &VolumeWriteOptions{ApiUrl: server.URL}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	readValue, err := v.ReadFile(context.Background(), "/empty.txt", testVolumeReadOpts(server.URL))
	if err != nil {
		t.Fatalf("ReadFile default returned error: %v", err)
	}
	read := readValue.(string)
	if read != "" {
		t.Fatalf("expected empty content, got %q", read)
	}
}

func TestVolumeFileParityWriteFileWithMetadataAndForce(t *testing.T) {
	var gotQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		gotQuery = r.URL.Query()
		respondEntry(t, w, VolumeEntryStat{
			Name:  "metadata.txt",
			Path:  "/metadata.txt",
			Type:  VolumeFileTypeFile,
			UID:   1000,
			GID:   1000,
			Mode:  0o644,
			Atime: time.Now(),
			Mtime: time.Now(),
			Ctime: time.Now(),
		})
	}))
	defer server.Close()

	uid := 1000
	gid := 1000
	mode := 0o644
	force := true
	v := testVolumeClient(server.URL)
	stat, err := v.WriteFile(context.Background(), "/metadata.txt", "content", &VolumeWriteOptions{
		ApiUrl: server.URL,
		VolumeMetadataOptions: VolumeMetadataOptions{
			UID:  &uid,
			GID:  &gid,
			Mode: &mode,
		},
		Force: &force,
	})
	if err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if stat == nil || stat.UID != 1000 || stat.GID != 1000 || stat.Mode != 0o644 {
		t.Fatalf("unexpected write stat: %#v", stat)
	}
	if gotQuery.Get("uid") != "1000" || gotQuery.Get("gid") != "1000" || gotQuery.Get("mode") != strconv.Itoa(0o644) || gotQuery.Get("force") != "true" {
		t.Fatalf("unexpected metadata query: %#v", gotQuery)
	}
}

func TestVolumeFileParityWriteFileRejectsUnsupportedDataType(t *testing.T) {
	v := testVolumeClient("http://127.0.0.1")

	_, err := v.WriteFile(context.Background(), "/bad.txt", 123, &VolumeWriteOptions{ApiUrl: "http://127.0.0.1"})
	if err == nil {
		t.Fatal("expected WriteFile to reject unsupported data type")
	}
	var invalidErr *shared.InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %T %v", err, err)
	}
	if invalidErr.Message != "Unsupported data type for file /bad.txt" {
		t.Fatalf("unexpected InvalidArgumentError message: %q", invalidErr.Message)
	}
}

func TestVolumeFileParityMakeDirWithMetadataAndForce(t *testing.T) {
	var gotQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		gotQuery = r.URL.Query()
		respondEntry(t, w, VolumeEntryStat{
			Name:  "dir",
			Path:  "/dir",
			Type:  VolumeFileTypeDirectory,
			UID:   1000,
			GID:   1000,
			Mode:  0o755,
			Atime: time.Now(),
			Mtime: time.Now(),
			Ctime: time.Now(),
		})
	}))
	defer server.Close()

	uid := 1000
	gid := 1000
	mode := 0o755
	force := true
	v := testVolumeClient(server.URL)
	stat, err := v.MakeDir(context.Background(), "/dir", &VolumeWriteOptions{
		ApiUrl: server.URL,
		VolumeMetadataOptions: VolumeMetadataOptions{
			UID:  &uid,
			GID:  &gid,
			Mode: &mode,
		},
		Force: &force,
	})
	if err != nil {
		t.Fatalf("MakeDir returned error: %v", err)
	}
	if stat == nil || stat.Type != VolumeFileTypeDirectory || stat.UID != 1000 || stat.GID != 1000 || stat.Mode != 0o755 {
		t.Fatalf("unexpected mkdir stat: %#v", stat)
	}
	if gotQuery.Get("uid") != "1000" || gotQuery.Get("gid") != "1000" || gotQuery.Get("mode") != strconv.Itoa(0o755) || gotQuery.Get("force") != "true" {
		t.Fatalf("unexpected mkdir query: %#v", gotQuery)
	}
}

func TestVolumeFileParityContentTypesMatchJsAndPythonRequestShapes(t *testing.T) {
	contentTypes := map[string]string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		contentTypes[key] = r.Header.Get("Content-Type")

		switch key {
		case "GET /volumecontent/vol-1/dir":
			if err := json.NewEncoder(w).Encode([]VolumeEntryStat{
				{
					Name:  "dir",
					Path:  "/dir",
					Type:  VolumeFileTypeDirectory,
					Atime: time.Now(),
					Mtime: time.Now(),
					Ctime: time.Now(),
				},
			}); err != nil {
				t.Fatalf("failed to encode list response: %v", err)
			}
		case "POST /volumecontent/vol-1/dir":
			w.WriteHeader(http.StatusCreated)
			respondEntry(t, w, VolumeEntryStat{
				Name:  "dir",
				Path:  "/dir",
				Type:  VolumeFileTypeDirectory,
				Atime: time.Now(),
				Mtime: time.Now(),
				Ctime: time.Now(),
			})
		case "GET /volumecontent/vol-1/path":
			respondEntry(t, w, VolumeEntryStat{
				Name:  "dir",
				Path:  "/dir",
				Type:  VolumeFileTypeDirectory,
				Atime: time.Now(),
				Mtime: time.Now(),
				Ctime: time.Now(),
			})
		case "PATCH /volumecontent/vol-1/path":
			respondEntry(t, w, VolumeEntryStat{
				Name:  "dir",
				Path:  "/dir",
				Type:  VolumeFileTypeDirectory,
				UID:   1001,
				GID:   1002,
				Mode:  0o644,
				Atime: time.Now(),
				Mtime: time.Now(),
				Ctime: time.Now(),
			})
		case "GET /volumecontent/vol-1/file":
			_, _ = io.WriteString(w, "hello")
		case "PUT /volumecontent/vol-1/file":
			w.WriteHeader(http.StatusCreated)
			respondEntry(t, w, VolumeEntryStat{
				Name:  "file.txt",
				Path:  "/file.txt",
				Type:  VolumeFileTypeFile,
				Atime: time.Now(),
				Mtime: time.Now(),
				Ctime: time.Now(),
			})
		case "DELETE /volumecontent/vol-1/path":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s", key)
		}
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	depth := 2
	if _, err := v.List(context.Background(), "/dir", &VolumeListOpts{
		VolumeApiOpts: VolumeApiOpts{ApiUrl: server.URL},
		Depth:         &depth,
	}); err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	force := false
	if _, err := v.MakeDir(context.Background(), "/dir", &VolumeWriteOptions{
		ApiUrl: server.URL,
		Force:  &force,
	}); err != nil {
		t.Fatalf("MakeDir returned error: %v", err)
	}

	if _, err := v.GetInfo(context.Background(), "/dir", testVolumeApiOpts(server.URL)); err != nil {
		t.Fatalf("GetInfo returned error: %v", err)
	}

	uid := 1001
	gid := 1002
	mode := 0o644
	if _, err := v.UpdateMetadata(context.Background(), "/dir", &VolumeMetadataOptions{
		UID:  &uid,
		GID:  &gid,
		Mode: &mode,
	}, testVolumeApiOpts(server.URL)); err != nil {
		t.Fatalf("UpdateMetadata returned error: %v", err)
	}

	if _, err := v.ReadFile(context.Background(), "/file.txt", testVolumeReadOpts(server.URL)); err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}

	if _, err := v.WriteFile(context.Background(), "/file.txt", "hello", &VolumeWriteOptions{
		ApiUrl: server.URL,
		Force:  &force,
	}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if err := v.Remove(context.Background(), "/file.txt", testVolumeApiOpts(server.URL)); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	for _, key := range []string{
		"GET /volumecontent/vol-1/dir",
		"POST /volumecontent/vol-1/dir",
		"GET /volumecontent/vol-1/path",
		"GET /volumecontent/vol-1/file",
		"DELETE /volumecontent/vol-1/path",
	} {
		if got := contentTypes[key]; got != "" {
			t.Fatalf("expected %s to omit Content-Type like JS/Python, got %q", key, got)
		}
	}

	if got := contentTypes["PATCH /volumecontent/vol-1/path"]; !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected PATCH to keep application/json, got %q", got)
	}
	if got := contentTypes["PUT /volumecontent/vol-1/file"]; !strings.HasPrefix(got, "application/octet-stream") {
		t.Fatalf("expected PUT to keep application/octet-stream, got %q", got)
	}
}

func TestVolumeFileParityListDepthOption(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.URL.Query().Get("depth"); got != "2" {
			t.Fatalf("expected depth query 2, got %q", got)
		}
		if got := r.URL.Query().Get("path"); got != "/deep" {
			t.Fatalf("expected path query /deep, got %q", got)
		}
		if err := json.NewEncoder(w).Encode([]VolumeEntryStat{
			{
				Name:  "nested",
				Path:  "/deep/nested",
				Type:  VolumeFileTypeDirectory,
				Atime: time.Now(),
				Mtime: time.Now(),
				Ctime: time.Now(),
			},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	depth := 2
	v := testVolumeClient(server.URL)
	entries, err := v.List(context.Background(), "/deep", &VolumeListOpts{
		VolumeApiOpts: VolumeApiOpts{ApiUrl: server.URL},
		Depth:         &depth,
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "/deep/nested" {
		t.Fatalf("unexpected list response: %#v", entries)
	}
}

func TestVolumeFileParityUpdateMetadataRequestAndResponse(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		respondEntry(t, w, VolumeEntryStat{
			Name:  "metadata.txt",
			Path:  "/metadata.txt",
			Type:  VolumeFileTypeFile,
			UID:   1001,
			GID:   1001,
			Mode:  0o755,
			Atime: time.Now(),
			Mtime: time.Now(),
			Ctime: time.Now(),
		})
	}))
	defer server.Close()

	uid := 1001
	gid := 1001
	mode := 0o755
	v := testVolumeClient(server.URL)
	info, err := v.UpdateMetadata(context.Background(), "/metadata.txt", &VolumeMetadataOptions{
		UID:  &uid,
		GID:  &gid,
		Mode: &mode,
	}, testVolumeApiOpts(server.URL))
	if err != nil {
		t.Fatalf("UpdateMetadata returned error: %v", err)
	}
	if gotBody["uid"] != float64(1001) || gotBody["gid"] != float64(1001) || gotBody["mode"] != float64(0o755) {
		t.Fatalf("unexpected metadata request body: %#v", gotBody)
	}
	if info == nil || info.UID != 1001 || info.GID != 1001 || info.Mode != 0o755 {
		t.Fatalf("unexpected update metadata response: %#v", info)
	}
}

func TestVolumeFileParityGetInfoForDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.URL.Path; got != "/volumecontent/vol-1/path" {
			t.Fatalf("unexpected path: %s", got)
		}
		if got := r.URL.Query().Get("path"); got != "/info-dir" {
			t.Fatalf("expected path query /info-dir, got %q", got)
		}
		respondEntry(t, w, VolumeEntryStat{
			Name:  "info-dir",
			Path:  "/info-dir",
			Type:  VolumeFileTypeDirectory,
			Atime: time.Now(),
			Mtime: time.Now(),
			Ctime: time.Now(),
		})
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	info, err := v.GetInfo(context.Background(), "/info-dir", testVolumeApiOpts(server.URL))
	if err != nil {
		t.Fatalf("GetInfo returned error: %v", err)
	}
	if info == nil || info.Name != "info-dir" || info.Type != VolumeFileTypeDirectory || info.Path != "/info-dir" {
		t.Fatalf("unexpected directory info: %#v", info)
	}
}

func TestVolumeFileParityListOmitsDepthQueryByDefaultLikeJsAndPython(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.URL.Query().Get("depth"); got != "" {
			t.Fatalf("expected default depth to be omitted, got %q", got)
		}
		if err := json.NewEncoder(w).Encode([]VolumeEntryStat{}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	entries, err := v.List(context.Background(), "/", testVolumeListOpts(server.URL))
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty entries, got %#v", entries)
	}
}

func TestVolumeFileParityRemoveUsesDeletePathEndpoint(t *testing.T) {
	var gotMethod string
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	if err := v.Remove(context.Background(), "/to-remove.txt", testVolumeApiOpts(server.URL)); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", gotMethod)
	}
	if !strings.HasPrefix(gotPath, "/volumecontent/vol-1/path?") || !strings.Contains(gotPath, "path=%2Fto-remove.txt") {
		t.Fatalf("unexpected remove path: %q", gotPath)
	}
}

func TestVolumeFileParityReadFileStreamHonorsBytesAndText(t *testing.T) {
	content := "stream content"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)
	stream := readVolumeStreamValue(t, v, "/stream.txt", testVolumeApiOpts(server.URL))
	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("failed to read stream: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("failed to close stream: %v", err)
	}
	if string(data) != content {
		t.Fatalf("unexpected stream data: %q", string(data))
	}
}

func TestVolumeFileParityReadFileMatchesJsStyleSingleEntrySurface(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		_, _ = w.Write([]byte("hello"))
	}))
	defer server.Close()

	v := testVolumeClient(server.URL)

	textValue, err := v.ReadFile(context.Background(), "/test.txt", testVolumeReadOpts(server.URL))
	if err != nil {
		t.Fatalf("ReadFile(text default) returned error: %v", err)
	}
	if got := textValue.(string); got != "hello" {
		t.Fatalf("unexpected text payload: %q", got)
	}

	bytesValue, err := v.ReadFile(context.Background(), "/test.txt", &VolumeReadOpts{
		VolumeApiOpts: *testVolumeApiOpts(server.URL),
		Format:        ReadFileFormatBytes,
	})
	if err != nil {
		t.Fatalf("ReadFile(bytes) returned error: %v", err)
	}
	if got := string(bytesValue.([]byte)); got != "hello" {
		t.Fatalf("unexpected bytes payload: %q", got)
	}

	blobValue, err := v.ReadFile(context.Background(), "/test.txt", &VolumeReadOpts{
		VolumeApiOpts: *testVolumeApiOpts(server.URL),
		Format:        ReadFileFormatBlob,
	})
	if err != nil {
		t.Fatalf("ReadFile(blob) returned error: %v", err)
	}
	blob, ok := blobValue.(Blob)
	if !ok {
		t.Fatalf("expected Blob, got %T", blobValue)
	}
	if got := blob.Text(); got != "hello" {
		t.Fatalf("unexpected blob payload: %q", got)
	}

	streamValue, err := v.ReadFile(context.Background(), "/test.txt", &VolumeReadOpts{
		VolumeApiOpts: *testVolumeApiOpts(server.URL),
		Format:        ReadFileFormatStream,
	})
	if err != nil {
		t.Fatalf("ReadFile(stream) returned error: %v", err)
	}
	stream, ok := streamValue.(io.ReadCloser)
	if !ok {
		t.Fatalf("expected io.ReadCloser, got %T", streamValue)
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("failed to read stream payload: %v", err)
	}
	if got := string(data); got != "hello" {
		t.Fatalf("unexpected stream payload: %q", got)
	}

	readBytes := readVolumeBytesValue(t, v, "/test.txt", testVolumeApiOpts(server.URL))
	if got := string(readBytes); got != "hello" {
		t.Fatalf("unexpected ReadFileBytes payload: %q", got)
	}
}

func TestVolumeFileParityReadFileRejectsUnsupportedFormats(t *testing.T) {
	v := testVolumeClient("http://example.test")

	_, err := v.ReadFile(context.Background(), "/test.txt", &VolumeReadOpts{
		VolumeApiOpts: *testVolumeApiOpts("http://example.test"),
		Format:        ReadFileFormat("xml"),
	})
	var invalidErr *shared.InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %T %v", err, err)
	}
	if invalidErr.Message != "Unsupported read format xml" {
		t.Fatalf("unexpected error message: %q", invalidErr.Message)
	}
}

func respondEntry(t *testing.T, w http.ResponseWriter, entry VolumeEntryStat) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(entry); err != nil {
		t.Fatalf("failed to encode response: %v", err)
	}
}
