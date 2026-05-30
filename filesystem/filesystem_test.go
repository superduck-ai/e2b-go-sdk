package filesystem

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

func testFilesystemConfig(sandboxURL string, requestTimeoutMs int) *struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            bool
	RequestTimeoutMs int
	Headers          map[string]string
} {
	return &struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
	}{
		SandboxUrl:       sandboxURL,
		RequestTimeoutMs: requestTimeoutMs,
		Headers:          map[string]string{},
	}
}

func directFieldNames(typ reflect.Type) []string {
	names := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		names = append(names, typ.Field(i).Name)
	}
	return names
}

func readTextValue(t *testing.T, fs *Filesystem, path string, opts *FilesystemReadOpts) string {
	t.Helper()
	value, err := fs.Read(context.Background(), path, opts)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("expected string, got %T", value)
	}
	return text
}

func readBytesValue(t *testing.T, fs *Filesystem, path string, opts *FilesystemReadOpts) []byte {
	t.Helper()
	readOpts := &FilesystemReadOpts{Format: ReadFormatBytes}
	if opts != nil {
		*readOpts = *opts
		readOpts.Format = ReadFormatBytes
	}
	value, err := fs.Read(context.Background(), path, readOpts)
	if err != nil {
		t.Fatalf("Read(bytes) returned error: %v", err)
	}
	data, ok := value.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", value)
	}
	return data
}

func readStreamValue(t *testing.T, fs *Filesystem, path string, opts *FilesystemReadOpts) io.ReadCloser {
	t.Helper()
	readOpts := &FilesystemReadOpts{Format: ReadFormatStream}
	if opts != nil {
		*readOpts = *opts
		readOpts.Format = ReadFormatStream
	}
	value, err := fs.Read(context.Background(), path, readOpts)
	if err != nil {
		t.Fatalf("Read(stream) returned error: %v", err)
	}
	stream, ok := value.(io.ReadCloser)
	if !ok {
		t.Fatalf("expected io.ReadCloser, got %T", value)
	}
	return stream
}

func TestReadReturnsEmptyStringForEmptyFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	text := readTextValue(t, fs, "/tmp/empty.txt", nil)
	if text != "" {
		t.Fatalf("expected empty string for empty file, got %q", text)
	}
}

func TestReadWrapsNotFoundAsFileNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing file", http.StatusNotFound)
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	_, err := fs.Read(context.Background(), "/tmp/missing.txt", nil)
	var notFoundErr *shared.FileNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected FileNotFoundError, got %T %v", err, err)
	}
}

func TestReadReturnsStreamResponseBody(t *testing.T) {
	releaseBody := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("path"); got != "/tmp/stream.txt" {
			t.Fatalf("unexpected path query: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-releaseBody
		if _, err := w.Write([]byte("streamed")); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	body := readStreamValue(t, fs, "/tmp/stream.txt", nil)
	close(releaseBody)
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read stream: %v", err)
	}
	if err := body.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if string(data) != "streamed" {
		t.Fatalf("unexpected streamed data: %q", string(data))
	}
}

func TestFilesystemConnectUnaryRetriesTransientTransportErrors(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.URL.Path != "/filesystem.Filesystem/Stat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if attempts == 1 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("expected hijacker support")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("failed to hijack connection: %v", err)
			}
			_ = conn.Close()
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"entry": map[string]any{
				"name":        "file.txt",
				"path":        "/file.txt",
				"type":        "file",
				"size":        float64(1),
				"mode":        float64(420),
				"permissions": "-rw-r--r--",
				"owner":       "user",
				"group":       "user",
			},
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")
	info, err := fs.GetInfo(context.Background(), "/file.txt", nil)
	if err != nil {
		t.Fatalf("expected retry to recover transient transport error, got %v", err)
	}
	if info == nil || info.Path != "/file.txt" {
		t.Fatalf("unexpected info: %#v", info)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestReadStreamWrapsNotFoundAsFileNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing stream", http.StatusNotFound)
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	_, err := fs.Read(context.Background(), "/tmp/missing.txt", &FilesystemReadOpts{Format: ReadFormatStream})
	var notFoundErr *shared.FileNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected FileNotFoundError, got %T %v", err, err)
	}
}

func TestGetInfoErrorsWhenEntryMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/Stat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	_, err := fs.GetInfo(context.Background(), "/tmp/file.txt", nil)
	if err == nil {
		t.Fatal("expected GetInfo to fail when entry is missing")
	}
	if err.Error() != "Expected to receive information about the file or directory" {
		t.Fatalf("unexpected GetInfo error: %v", err)
	}
}

func TestGetInfoWrapsNotFoundAsFileNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/Stat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"code":"not_found","message":"missing entry"}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	_, err := fs.GetInfo(context.Background(), "/tmp/missing.txt", nil)
	var notFoundErr *shared.FileNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected FileNotFoundError, got %T %v", err, err)
	}
}

func TestFilesystemReadAndWriteOptsMatchJsAndPythonFieldShape(t *testing.T) {
	t.Parallel()

	writeOptsType := reflect.TypeOf(FilesystemWriteOpts{})
	if got, want := directFieldNames(writeOptsType), []string{"FilesystemRequestOpts", "Gzip", "UseOctetStream"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected FilesystemWriteOpts field shape: got %v want %v", got, want)
	}
	if field, ok := writeOptsType.FieldByName("UseOctetStream"); !ok {
		t.Fatal("expected FilesystemWriteOpts to expose UseOctetStream")
	} else if field.Type != reflect.TypeOf(false) {
		t.Fatalf("expected FilesystemWriteOpts.UseOctetStream to be bool, got %v", field.Type)
	}

	readOptsType := reflect.TypeOf(FilesystemReadOpts{})
	if got, want := directFieldNames(readOptsType), []string{"FilesystemRequestOpts", "Gzip", "Format"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected FilesystemReadOpts field shape: got %v want %v", got, want)
	}
	if field, ok := readOptsType.FieldByName("Format"); !ok {
		t.Fatal("expected FilesystemReadOpts to expose Format")
	} else if field.Type != reflect.TypeOf(ReadFormat("")) {
		t.Fatalf("expected FilesystemReadOpts.Format to be ReadFormat, got %v", field.Type)
	}

	listOptsType := reflect.TypeOf(FilesystemListOpts{})
	if got, want := directFieldNames(listOptsType), []string{"FilesystemRequestOpts", "Depth"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected FilesystemListOpts field shape: got %v want %v", got, want)
	}
	if field, ok := listOptsType.FieldByName("Depth"); !ok {
		t.Fatal("expected FilesystemListOpts to expose Depth")
	} else if field.Type != reflect.TypeOf((*int)(nil)) {
		t.Fatalf("expected FilesystemListOpts.Depth to be *int, got %v", field.Type)
	}
}

func TestFilesystemUsesSingleEntryReadSurfaceAndBroadWriteInputs(t *testing.T) {
	t.Parallel()

	fsType := reflect.TypeOf(&Filesystem{})
	var _ Blob = Blob([]byte("blob"))

	readMethod, ok := fsType.MethodByName("Read")
	if !ok {
		t.Fatal("expected Filesystem.Read to exist")
	}
	if got, want := readMethod.Type.NumIn(), 4; got != want {
		t.Fatalf("unexpected Filesystem.Read input count: got %d want %d", got, want)
	}
	if got, want := readMethod.Type.In(2), reflect.TypeOf(""); got != want {
		t.Fatalf("expected Filesystem.Read path param to be string, got %v", got)
	}
	if got, want := readMethod.Type.In(3), reflect.TypeOf(&FilesystemReadOpts{}); got != want {
		t.Fatalf("expected Filesystem.Read opts param to be *FilesystemReadOpts, got %v", got)
	}
	if got, want := readMethod.Type.Out(0), reflect.TypeOf((*any)(nil)).Elem(); got != want {
		t.Fatalf("expected Filesystem.Read to return any, got %v", got)
	}

	if _, ok := fsType.MethodByName("ReadBytes"); ok {
		t.Fatal("did not expect Filesystem.ReadBytes to exist")
	}
	if _, ok := fsType.MethodByName("ReadText"); ok {
		t.Fatal("did not expect Filesystem.ReadText to exist")
	}
	if _, ok := fsType.MethodByName("ReadStream"); ok {
		t.Fatal("did not expect Filesystem.ReadStream to exist")
	}
	if _, ok := fsType.MethodByName("ReadWithFormat"); ok {
		t.Fatal("did not expect Filesystem.ReadWithFormat to exist")
	}

	writeMethod, ok := fsType.MethodByName("Write")
	if !ok {
		t.Fatal("expected Filesystem.Write to exist")
	}
	if got, want := writeMethod.Type.In(3), reflect.TypeOf((*any)(nil)).Elem(); got != want {
		t.Fatalf("expected Filesystem.Write data param to be any, got %v", got)
	}
	if got, want := writeMethod.Type.Out(0), reflect.TypeOf(&WriteInfo{}); got != want {
		t.Fatalf("expected Filesystem.Write to return *WriteInfo, got %v", got)
	}

	writeFilesMethod, ok := fsType.MethodByName("WriteFiles")
	if !ok {
		t.Fatal("expected Filesystem.WriteFiles to exist")
	}
	if got, want := writeFilesMethod.Type.In(2), reflect.TypeOf([]WriteEntry(nil)); got != want {
		t.Fatalf("expected Filesystem.WriteFiles files param to be []WriteEntry, got %v", got)
	}

	writeEntryType := reflect.TypeOf(WriteEntry{})
	if got, want := directFieldNames(writeEntryType), []string{"Path", "Data"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected WriteEntry field shape: got %v want %v", got, want)
	}
	if field, ok := writeEntryType.FieldByName("Data"); !ok {
		t.Fatal("expected WriteEntry to expose Data")
	} else if field.Type != reflect.TypeOf((*any)(nil)).Elem() {
		t.Fatalf("expected WriteEntry.Data to be any, got %v", field.Type)
	}
}

func TestReadMatchesJsStyleSingleEntrySurface(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("path"); got != "/tmp/file.txt" {
			t.Fatalf("unexpected path query: %q", got)
		}
		_, _ = w.Write([]byte("hello"))
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	textValue, err := fs.Read(context.Background(), "/tmp/file.txt", nil)
	if err != nil {
		t.Fatalf("Read(text default) returned error: %v", err)
	}
	if got := textValue.(string); got != "hello" {
		t.Fatalf("unexpected text payload: %q", got)
	}

	bytesValue, err := fs.Read(context.Background(), "/tmp/file.txt", &FilesystemReadOpts{Format: ReadFormatBytes})
	if err != nil {
		t.Fatalf("Read(bytes) returned error: %v", err)
	}
	if got := string(bytesValue.([]byte)); got != "hello" {
		t.Fatalf("unexpected bytes payload: %q", got)
	}

	blobValue, err := fs.Read(context.Background(), "/tmp/file.txt", &FilesystemReadOpts{Format: ReadFormatBlob})
	if err != nil {
		t.Fatalf("Read(blob) returned error: %v", err)
	}
	blob, ok := blobValue.(Blob)
	if !ok {
		t.Fatalf("expected Blob, got %T", blobValue)
	}
	if got := blob.Text(); got != "hello" {
		t.Fatalf("unexpected blob payload: %q", got)
	}

	streamValue, err := fs.Read(context.Background(), "/tmp/file.txt", &FilesystemReadOpts{Format: ReadFormatStream})
	if err != nil {
		t.Fatalf("Read(stream) returned error: %v", err)
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

	readBytes := readBytesValue(t, fs, "/tmp/file.txt", nil)
	if got := string(readBytes); got != "hello" {
		t.Fatalf("unexpected ReadBytes payload: %q", got)
	}
}

func TestReadRejectsUnsupportedFormats(t *testing.T) {
	fs := NewFilesystem(testFilesystemConfig("http://example.test", 0), "1.0.0")

	_, err := fs.Read(context.Background(), "/tmp/file.txt", &FilesystemReadOpts{Format: ReadFormat("xml")})
	var invalidErr *shared.InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %T %v", err, err)
	}
	if invalidErr.Message != "Unsupported read format xml" {
		t.Fatalf("unexpected error message: %q", invalidErr.Message)
	}
}

func TestWriteFilesAcceptsBlobInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("failed to parse content type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("expected multipart/form-data content type, got %q", mediaType)
		}

		reader := multipart.NewReader(r.Body, params["boundary"])
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("failed reading multipart body: %v", err)
		}
		if got := part.FileName(); got != "blob.txt" {
			t.Fatalf("unexpected multipart filename: %q", got)
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("failed reading part body: %v", err)
		}
		if got := string(body); got != "blob payload" {
			t.Fatalf("unexpected blob body: %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]WriteInfo{
			{Name: "blob.txt", Type: FileTypeFile, Path: "/tmp/blob.txt"},
		}); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "0.5.7")

	infos, err := fs.WriteFiles(context.Background(), []WriteEntry{
		{Path: "/tmp/blob.txt", Data: Blob([]byte("blob payload"))},
	}, nil)
	if err != nil {
		t.Fatalf("WriteFiles returned error: %v", err)
	}
	if len(infos) != 1 || infos[0].Path != "/tmp/blob.txt" {
		t.Fatalf("unexpected write infos: %#v", infos)
	}
}

func TestWriteErrorsWhenExplicitOctetStreamUploadReturnsNoInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("expected application/octet-stream upload, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "0.5.7")

	_, err := fs.Write(context.Background(), "/tmp/file.txt", bytes.NewBufferString("hello"), &FilesystemWriteOpts{
		UseOctetStream: true,
	})
	if err == nil {
		t.Fatal("expected Write to fail when explicit octet-stream upload returns no file info")
	}
	if err.Error() != "Expected to receive information about written file" {
		t.Fatalf("unexpected Write error: %v", err)
	}
}

func TestWriteErrorsWhenMultipartUploadReturnsNoInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "0.5.6")

	_, err := fs.Write(context.Background(), "/tmp/file.txt", bytes.NewBufferString("hello"), nil)
	if err == nil {
		t.Fatal("expected Write to fail when multipart upload returns no file info")
	}
	if err.Error() != "Expected to receive information about written file" {
		t.Fatalf("unexpected Write error: %v", err)
	}
}

func TestWriteUsesMultipartByDefaultOnEnvdThatSupportsOctetStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if path := r.URL.Query().Get("path"); path != "/tmp/file.txt" {
			t.Fatalf("expected path query for single-file multipart upload, got %q", path)
		}
		if got := r.Header.Get("Content-Encoding"); got != "" {
			t.Fatalf("expected multipart default upload to ignore gzip flag, got content-encoding %q", got)
		}

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("failed to parse content type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("expected multipart/form-data content type, got %q", mediaType)
		}

		reader := multipart.NewReader(r.Body, params["boundary"])
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("failed reading multipart body: %v", err)
		}
		if got := part.FileName(); got != "file.txt" {
			t.Fatalf("unexpected multipart filename: %q", got)
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("failed reading multipart part: %v", err)
		}
		if string(body) != "hello" {
			t.Fatalf("unexpected multipart content: %q", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]WriteInfo{
			{Name: "file.txt", Type: FileTypeFile, Path: "/tmp/file.txt"},
		}); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "0.5.7")

	info, err := fs.Write(context.Background(), "/tmp/file.txt", "hello", &FilesystemWriteOpts{
		Gzip: true,
	})
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if info == nil || info.Path != "/tmp/file.txt" {
		t.Fatalf("unexpected write info: %#v", info)
	}
}

func TestWriteFilesUsesSingleMultipartRequestOnOldEnvd(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if queryPath := r.URL.Query().Get("path"); queryPath != "" {
			t.Fatalf("expected multipart multi-file upload to omit path query, got %q", queryPath)
		}
		if username := r.URL.Query().Get("username"); username != "" {
			t.Fatalf("expected username query to be omitted on modern default-user envd, got %q", username)
		}

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("failed to parse content type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("expected multipart/form-data content type, got %q", mediaType)
		}

		reader := multipart.NewReader(r.Body, params["boundary"])
		var filenames []string
		var contents []string
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("failed reading multipart body: %v", err)
			}
			filenames = append(filenames, part.FileName())
			body, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("failed reading part body: %v", err)
			}
			contents = append(contents, string(body))
		}

		expectedFiles := []string{"one.txt", "two.txt"}
		expectedContents := []string{"one", "two"}
		if strings.Join(filenames, ",") != strings.Join(expectedFiles, ",") {
			t.Fatalf("unexpected multipart filenames: %#v", filenames)
		}
		if strings.Join(contents, ",") != strings.Join(expectedContents, ",") {
			t.Fatalf("unexpected multipart contents: %#v", contents)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]WriteInfo{
			{Name: "one.txt", Type: FileTypeFile, Path: "/tmp/one.txt"},
			{Name: "two.txt", Type: FileTypeFile, Path: "/tmp/two.txt"},
		}); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "0.5.6")

	infos, err := fs.WriteFiles(context.Background(), []WriteEntry{
		{Path: "/tmp/one.txt", Data: "one"},
		{Path: "/tmp/two.txt", Data: []byte("two")},
	}, nil)
	if err != nil {
		t.Fatalf("WriteFiles returned error: %v", err)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("expected a single multipart request, got %d", requestCount.Load())
	}
	if len(infos) != 2 {
		t.Fatalf("expected two write infos, got %#v", infos)
	}
	if infos[0].Path != "/tmp/one.txt" || infos[1].Path != "/tmp/two.txt" {
		t.Fatalf("unexpected write infos: %#v", infos)
	}
}

func TestWriteFilesUsesSingleMultipartRequestByDefaultOnEnvdThatSupportsOctetStream(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if queryPath := r.URL.Query().Get("path"); queryPath != "" {
			t.Fatalf("expected multipart multi-file upload to omit path query, got %q", queryPath)
		}
		if got := r.Header.Get("Content-Encoding"); got != "" {
			t.Fatalf("expected multipart default upload to ignore gzip flag, got content-encoding %q", got)
		}

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("failed to parse content type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("expected multipart/form-data content type, got %q", mediaType)
		}

		reader := multipart.NewReader(r.Body, params["boundary"])
		var filenames []string
		var contents []string
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("failed reading multipart body: %v", err)
			}
			filenames = append(filenames, part.FileName())
			body, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("failed reading part body: %v", err)
			}
			contents = append(contents, string(body))
		}

		expectedFiles := []string{"one.txt", "two.txt"}
		expectedContents := []string{"one", "two"}
		if strings.Join(filenames, ",") != strings.Join(expectedFiles, ",") {
			t.Fatalf("unexpected multipart filenames: %#v", filenames)
		}
		if strings.Join(contents, ",") != strings.Join(expectedContents, ",") {
			t.Fatalf("unexpected multipart contents: %#v", contents)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]WriteInfo{
			{Name: "one.txt", Type: FileTypeFile, Path: "/tmp/one.txt"},
			{Name: "two.txt", Type: FileTypeFile, Path: "/tmp/two.txt"},
		}); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "0.5.7")

	infos, err := fs.WriteFiles(context.Background(), []WriteEntry{
		{Path: "/tmp/one.txt", Data: "one"},
		{Path: "/tmp/two.txt", Data: []byte("two")},
	}, &FilesystemWriteOpts{Gzip: true})
	if err != nil {
		t.Fatalf("WriteFiles returned error: %v", err)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("expected a single multipart request, got %d", requestCount.Load())
	}
	if len(infos) != 2 {
		t.Fatalf("expected two write infos, got %#v", infos)
	}
	if infos[0].Path != "/tmp/one.txt" || infos[1].Path != "/tmp/two.txt" {
		t.Fatalf("unexpected write infos: %#v", infos)
	}
}

func TestWriteFilesUsesOctetStreamPerFileWhenExplicitlyRequested(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := int(requestCount.Add(1)) - 1

		expectedPaths := []string{"/tmp/one.txt", "/tmp/two.txt"}
		expectedBodies := []string{"one", "two"}
		if index >= len(expectedPaths) {
			t.Fatalf("unexpected extra request %d", index+1)
		}

		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("expected application/octet-stream content type, got %q", got)
		}
		if got := r.URL.Query().Get("path"); got != expectedPaths[index] {
			t.Fatalf("unexpected path query for request %d: %q", index+1, got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed reading request body: %v", err)
		}
		if string(body) != expectedBodies[index] {
			t.Fatalf("unexpected request body for request %d: %q", index+1, string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]WriteInfo{
			{Name: strings.TrimPrefix(expectedPaths[index], "/tmp/"), Type: FileTypeFile, Path: expectedPaths[index]},
		}); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "0.5.7")

	infos, err := fs.WriteFiles(context.Background(), []WriteEntry{
		{Path: "/tmp/one.txt", Data: "one"},
		{Path: "/tmp/two.txt", Data: []byte("two")},
	}, &FilesystemWriteOpts{UseOctetStream: true})
	if err != nil {
		t.Fatalf("WriteFiles returned error: %v", err)
	}
	if requestCount.Load() != 2 {
		t.Fatalf("expected one octet-stream request per file, got %d", requestCount.Load())
	}
	if len(infos) != 2 {
		t.Fatalf("expected two write infos, got %#v", infos)
	}
	if infos[0].Path != "/tmp/one.txt" || infos[1].Path != "/tmp/two.txt" {
		t.Fatalf("unexpected write infos: %#v", infos)
	}
}

func TestWriteFilesEmptyArrayMatchesJsNoop(t *testing.T) {
	fs := NewFilesystem(testFilesystemConfig("http://127.0.0.1", 0), "1.0.0")

	infos, err := fs.WriteFiles(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("WriteFiles empty array returned error: %v", err)
	}
	if len(infos) != 0 {
		t.Fatalf("expected empty result for empty WriteFiles, got %#v", infos)
	}
}

func TestWriteMultipartUsesPathQueryForSingleFileOnOldEnvd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if path := r.URL.Query().Get("path"); path != "/tmp/file.txt" {
			t.Fatalf("expected path query for single-file multipart upload, got %q", path)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]WriteInfo{
			{Name: "file.txt", Type: FileTypeFile, Path: "/tmp/file.txt"},
		}); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "0.5.6")

	info, err := fs.Write(context.Background(), "/tmp/file.txt", []byte("hello"), nil)
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if info == nil || info.Path != "/tmp/file.txt" {
		t.Fatalf("unexpected write info: %#v", info)
	}
}

func TestWriteRejectsUnsupportedDataTypeWithInvalidArgumentError(t *testing.T) {
	fs := NewFilesystem(testFilesystemConfig("http://127.0.0.1", 0), "1.0.0")

	_, err := fs.Write(context.Background(), "/tmp/file.txt", 123, nil)
	if err == nil {
		t.Fatal("expected Write to reject unsupported data type")
	}
	var invalidErr *shared.InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %T %v", err, err)
	}
	if invalidErr.Message != "Unsupported data type for file /tmp/file.txt" {
		t.Fatalf("unexpected InvalidArgumentError message: %q", invalidErr.Message)
	}
}

func TestRenameErrorsWhenMovedEntryMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/Move" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	_, err := fs.Rename(context.Background(), "/tmp/old.txt", "/tmp/new.txt", nil)
	if err == nil {
		t.Fatal("expected Rename to fail when moved entry is missing")
	}
}

func TestRemoveIgnoresNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/Remove" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"code":"not_found","message":"missing entry"}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	err := fs.Remove(context.Background(), "/tmp/missing.txt", nil)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
}

func TestExistsReturnsFalseForFileNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/Stat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"code":"not_found","message":"missing entry"}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	exists, err := fs.Exists(context.Background(), "/tmp/missing.txt", nil)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if exists {
		t.Fatal("expected Exists to return false for missing file")
	}
}

func TestListSkipsEntriesWithUnknownType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/ListDir" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"entries":[{"name":"mystery","type":0,"path":"/tmp/mystery"},{"name":"file.txt","type":1,"path":"/tmp/file.txt"}]}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	entries, err := fs.List(context.Background(), "/tmp", nil)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only one valid entry, got %#v", entries)
	}
	if entries[0].Name != "file.txt" {
		t.Fatalf("expected valid file entry to remain, got %#v", entries[0])
	}
}

func TestListDefaultsDepthWhenOmittedAndRejectsExplicitZero(t *testing.T) {
	var seenDepth []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/ListDir" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request: %v", err)
		}
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		seenDepth = append(seenDepth, int(req["depth"].(float64)))

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"entries":[]}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	if _, err := fs.List(context.Background(), "/tmp", nil); err != nil {
		t.Fatalf("List(nil opts) returned error: %v", err)
	}
	if _, err := fs.List(context.Background(), "/tmp", &FilesystemListOpts{}); err != nil {
		t.Fatalf("List(empty opts) returned error: %v", err)
	}
	depthTwo := 2
	if _, err := fs.List(context.Background(), "/tmp", &FilesystemListOpts{Depth: &depthTwo}); err != nil {
		t.Fatalf("List(depth=2) returned error: %v", err)
	}
	if got, want := seenDepth, []int{1, 1, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected depth sequence: got %v want %v", got, want)
	}

	depthZero := 0
	if _, err := fs.List(context.Background(), "/tmp", &FilesystemListOpts{Depth: &depthZero}); err == nil {
		t.Fatal("expected explicit depth zero to fail")
	} else if !strings.Contains(err.Error(), "depth should be at least one") {
		t.Fatalf("unexpected explicit depth zero error: %v", err)
	}
}

func TestWatchDirSkipsUnknownEventTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/WatchDir" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		writeEnvelope(t, w, 0x00, []byte(`{"started":true}`))
		writeEnvelope(t, w, 0x00, []byte(`{"event":{"name":"mystery","type":0}}`))

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	eventCh := make(chan FilesystemEvent, 1)
	handle, err := fs.WatchDir(context.Background(), "/tmp", func(event FilesystemEvent) {
		eventCh <- event
	}, nil)
	if err != nil {
		t.Fatalf("WatchDir returned error: %v", err)
	}
	defer handle.Stop()

	select {
	case event := <-eventCh:
		t.Fatalf("expected unknown event type to be skipped, got %#v", event)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestWatchDirSendsConnectEnvelopeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/WatchDir" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		payload := assertConnectEnvelopeRequest(t, r)
		if !bytes.Contains(payload, []byte(`"/tmp"`)) {
			t.Fatalf("unexpected watch payload: %s", string(payload))
		}
		w.WriteHeader(http.StatusOK)
		writeEnvelope(t, w, 0x00, []byte(`{"started":true}`))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")
	handle, err := fs.WatchDir(context.Background(), "/tmp", nil, nil)
	if err != nil {
		t.Fatalf("WatchDir returned error: %v", err)
	}
	handle.Stop()
}

func TestWatchDirHandlesCurrentConnectJSONEventShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/WatchDir" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		writeEnvelope(t, w, 0x00, []byte(`{"start":{}}`))
		writeEnvelope(t, w, 0x00, []byte(`{"filesystem":{"name":"file.txt","type":1}}`))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")
	eventCh := make(chan FilesystemEvent, 1)
	handle, err := fs.WatchDir(context.Background(), "/tmp", func(event FilesystemEvent) {
		eventCh <- event
	}, nil)
	if err != nil {
		t.Fatalf("WatchDir returned error: %v", err)
	}
	defer handle.Stop()

	select {
	case event := <-eventCh:
		if event.Name != "file.txt" || event.Type != FilesystemEventCreate {
			t.Fatalf("unexpected event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for filesystem event")
	}
}

func TestFilesystemEventTypeValuesMatchCurrentTs(t *testing.T) {
	expected := map[FilesystemEventType]string{
		FilesystemEventChmod:  "chmod",
		FilesystemEventCreate: "create",
		FilesystemEventRemove: "remove",
		FilesystemEventRename: "rename",
		FilesystemEventWrite:  "write",
	}
	for got, want := range expected {
		if string(got) != want {
			t.Fatalf("unexpected filesystem event value: got %q want %q", got, want)
		}
	}
}

func TestWatchDirStopCallsOnExitWithoutError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/WatchDir" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		writeEnvelope(t, w, 0x00, []byte(`{"started":true}`))

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flushing")
		}
		flusher.Flush()

		<-r.Context().Done()
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	exitCh := make(chan error, 2)
	var exitCalls atomic.Int32

	handle, err := fs.WatchDir(context.Background(), "/tmp", nil, &WatchOpts{
		OnExit: func(err error) {
			exitCalls.Add(1)
			exitCh <- err
		},
	})
	if err != nil {
		t.Fatalf("WatchDir returned error: %v", err)
	}

	handle.Stop()

	select {
	case err := <-exitCh:
		if err != nil {
			t.Fatalf("expected nil onExit error after Stop, got: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for onExit callback")
	}

	time.Sleep(100 * time.Millisecond)
	if calls := exitCalls.Load(); calls != 1 {
		t.Fatalf("expected onExit to be called once, got %d", calls)
	}
}

func TestWatchDirUsesDefaultRequestTimeoutBeforeResponseStarts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 20), "1.0.0")

	start := time.Now()
	_, err := fs.WatchDir(context.Background(), "/tmp", nil, nil)
	if err == nil {
		t.Fatal("expected startup timeout error")
	}
	if elapsed := time.Since(start); elapsed >= 150*time.Millisecond {
		t.Fatalf("expected startup request timeout to trigger early, elapsed=%s", elapsed)
	}
}

func TestWatchDirErrorsWhenFirstEventIsNotStart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/WatchDir" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		writeEnvelope(t, w, 0x00, []byte(`{"event":{"name":"file.txt","type":1}}`))
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	_, err := fs.WatchDir(context.Background(), "/tmp", nil, nil)
	if err == nil {
		t.Fatal("expected watch startup error")
	}
	if err.Error() != "Expected start event" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWatchDirErrorsWhenStreamClosesBeforeFirstEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/WatchDir" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	_, err := fs.WatchDir(context.Background(), "/tmp", nil, nil)
	if err == nil {
		t.Fatal("expected watch startup error")
	}
	if err.Error() != "Expected start event" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWatchDirRejectsRecursiveWatchOnOldEnvdWithAlignedMessage(t *testing.T) {
	fs := NewFilesystem(testFilesystemConfig("", 0), "0.1.3")

	_, err := fs.WatchDir(context.Background(), "/tmp", nil, &WatchOpts{
		Recursive: true,
	})
	if err == nil {
		t.Fatal("expected recursive watch on old envd to fail")
	}
	expected := "You need to update the template to use recursive watching. You can do this by running `e2b template build` in the directory with the template."
	if err.Error() != expected {
		t.Fatalf("unexpected recursive watch error: %v", err)
	}
}

func writeEnvelope(t *testing.T, w http.ResponseWriter, flags byte, payload []byte) {
	t.Helper()

	var buf bytes.Buffer
	header := make([]byte, 5)
	header[0] = flags
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	if _, err := buf.Write(header); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}
	if _, err := buf.Write(payload); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		t.Fatalf("failed to write envelope: %v", err)
	}
}

func assertConnectEnvelopeRequest(t *testing.T, r *http.Request) []byte {
	t.Helper()
	if got := r.Header.Get("Content-Type"); got != "application/connect+json" {
		t.Fatalf("expected connect content type, got %q", got)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}
	if len(body) < 5 {
		t.Fatalf("expected connect envelope body, got %d bytes", len(body))
	}
	if body[0] != 0 {
		t.Fatalf("expected uncompressed envelope flag 0, got %d", body[0])
	}
	length := int(binary.BigEndian.Uint32(body[1:5]))
	if length != len(body)-5 {
		t.Fatalf("expected envelope length %d, got %d payload bytes", length, len(body)-5)
	}
	return body[5:]
}
