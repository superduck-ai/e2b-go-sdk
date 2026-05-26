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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/e2b-dev/e2b-go-sdk/internal/shared"
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

func TestReadTextReturnsEmptyStringForEmptyFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	text, err := fs.ReadText(context.Background(), "/tmp/empty.txt", nil)
	if err != nil {
		t.Fatalf("ReadText returned error: %v", err)
	}
	if text != "" {
		t.Fatalf("expected empty string for empty file, got %q", text)
	}
}

func TestReadTextWrapsNotFoundAsFileNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing file", http.StatusNotFound)
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "1.0.0")

	_, err := fs.ReadText(context.Background(), "/tmp/missing.txt", nil)
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

func TestWriteErrorsWhenOctetStreamUploadReturnsNoInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fs := NewFilesystem(testFilesystemConfig(server.URL, 0), "0.5.7")

	_, err := fs.Write(context.Background(), "/tmp/file.txt", bytes.NewBufferString("hello"), nil)
	if err == nil {
		t.Fatal("expected Write to fail when octet-stream upload returns no file info")
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
		{Path: "/tmp/one.txt", Data: bytes.NewBufferString("one")},
		{Path: "/tmp/two.txt", Data: bytes.NewBufferString("two")},
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

	info, err := fs.Write(context.Background(), "/tmp/file.txt", bytes.NewBufferString("hello"), nil)
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if info == nil || info.Path != "/tmp/file.txt" {
		t.Fatalf("unexpected write info: %#v", info)
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

func TestRemoveWrapsNotFoundAsFileNotFoundError(t *testing.T) {
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
	var notFoundErr *shared.FileNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected FileNotFoundError, got %T %v", err, err)
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
