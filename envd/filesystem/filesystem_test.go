package filesystem

import (
	"encoding/json"
	"testing"
)

func TestEntryInfoUnmarshalCurrentEnvdJSONShape(t *testing.T) {
	var resp StatResponse
	err := json.Unmarshal([]byte(`{"entry":{"name":"hello.txt","type":"FILE_TYPE_FILE","path":"/tmp/hello.txt","size":"8","mode":420,"permissions":"-rw-r--r--","owner":"user","group":"user","modifiedTime":"2026-05-26T07:20:19.682467536Z"}}`), &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal stat response: %v", err)
	}
	if resp.Entry == nil {
		t.Fatal("expected entry")
	}
	if resp.Entry.Type != FileTypeFile {
		t.Fatalf("expected file type, got %v", resp.Entry.Type)
	}
	if resp.Entry.Size != 8 {
		t.Fatalf("expected size 8, got %d", resp.Entry.Size)
	}
}

func TestFilesystemEventUnmarshalCurrentEnvdJSONShape(t *testing.T) {
	var event FilesystemEvent
	if err := json.Unmarshal([]byte(`{"name":"watch.txt","type":"EVENT_TYPE_CREATE"}`), &event); err != nil {
		t.Fatalf("failed to unmarshal filesystem event: %v", err)
	}
	if event.Type != EventTypeCreate {
		t.Fatalf("expected create event, got %v", event.Type)
	}
}
