package filesystem

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// FileType for filesystem entries
type FileType int32

const (
	FileTypeUnspecified FileType = 0
	FileTypeFile        FileType = 1
	FileTypeDirectory   FileType = 2
)

func (t *FileType) UnmarshalJSON(data []byte) error {
	var value int32
	if err := json.Unmarshal(data, &value); err == nil {
		*t = FileType(value)
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	switch normalizeEnumString(text) {
	case "file", "filetypefile", "file_type_file":
		*t = FileTypeFile
	case "dir", "directory", "filetypedirectory", "file_type_directory":
		*t = FileTypeDirectory
	default:
		*t = FileTypeUnspecified
	}
	return nil
}

// EventType for filesystem watch events
type EventType int32

const (
	EventTypeUnspecified EventType = 0
	EventTypeCreate      EventType = 1
	EventTypeWrite       EventType = 2
	EventTypeRemove      EventType = 3
	EventTypeRename      EventType = 4
	EventTypeChmod       EventType = 5
)

func (t *EventType) UnmarshalJSON(data []byte) error {
	var value int32
	if err := json.Unmarshal(data, &value); err == nil {
		*t = EventType(value)
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	switch normalizeEnumString(text) {
	case "create", "eventtypecreate", "event_type_create":
		*t = EventTypeCreate
	case "write", "eventtypewrite", "event_type_write":
		*t = EventTypeWrite
	case "remove", "eventtyperemove", "event_type_remove":
		*t = EventTypeRemove
	case "rename", "eventtyperename", "event_type_rename":
		*t = EventTypeRename
	case "chmod", "eventtypechmod", "event_type_chmod":
		*t = EventTypeChmod
	default:
		*t = EventTypeUnspecified
	}
	return nil
}

func normalizeEnumString(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

// EntryInfo represents a filesystem entry
type EntryInfo struct {
	Name          string   `json:"name"`
	Type          FileType `json:"type"`
	Path          string   `json:"path"`
	Size          int64    `json:"size"`
	Mode          uint32   `json:"mode"`
	Permissions   string   `json:"permissions"`
	Owner         string   `json:"owner"`
	Group         string   `json:"group"`
	ModifiedTime  string   `json:"modifiedTime,omitempty"`
	SymlinkTarget string   `json:"symlinkTarget,omitempty"`
}

func (e *EntryInfo) UnmarshalJSON(data []byte) error {
	type entryInfoJSON struct {
		Name          string          `json:"name"`
		Type          FileType        `json:"type"`
		Path          string          `json:"path"`
		Size          json.RawMessage `json:"size"`
		Mode          json.RawMessage `json:"mode"`
		Permissions   string          `json:"permissions"`
		Owner         string          `json:"owner"`
		Group         string          `json:"group"`
		ModifiedTime  json.RawMessage `json:"modifiedTime,omitempty"`
		SymlinkTarget string          `json:"symlinkTarget,omitempty"`
	}

	var decoded entryInfoJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	e.Name = decoded.Name
	e.Type = decoded.Type
	e.Path = decoded.Path
	e.Size, _ = parseJSONInt64(decoded.Size)
	if mode, ok := parseJSONInt64(decoded.Mode); ok {
		e.Mode = uint32(mode)
	}
	e.Permissions = decoded.Permissions
	e.Owner = decoded.Owner
	e.Group = decoded.Group
	e.ModifiedTime = parseModifiedTime(decoded.ModifiedTime)
	e.SymlinkTarget = decoded.SymlinkTarget
	return nil
}

func parseModifiedTime(data json.RawMessage) string {
	if len(data) == 0 || string(data) == "null" {
		return ""
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		return text
	}

	var ts struct {
		Seconds json.RawMessage `json:"seconds"`
		Nanos   int64           `json:"nanos"`
	}
	if err := json.Unmarshal(data, &ts); err != nil || len(ts.Seconds) == 0 {
		return ""
	}

	seconds, ok := parseJSONInt64(ts.Seconds)
	if !ok {
		return ""
	}
	return time.Unix(seconds, ts.Nanos).UTC().Format(time.RFC3339Nano)
}

func parseJSONInt64(data json.RawMessage) (int64, bool) {
	var value int64
	if err := json.Unmarshal(data, &value); err == nil {
		return value, true
	}

	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return 0, false
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	return parsed, err == nil
}

// FilesystemEvent represents a watch event
type FilesystemEvent struct {
	Name string    `json:"name"`
	Type EventType `json:"type"`
}

type WatchDirStartEvent struct{}
type WatchDirKeepaliveEvent struct{}

// Request/Response messages

type StatRequest struct {
	Path string `json:"path"`
}

type StatResponse struct {
	Entry *EntryInfo `json:"entry"`
}

type MakeDirRequest struct {
	Path string `json:"path"`
}

type MakeDirResponse struct {
	Entry *EntryInfo `json:"entry"`
}

type MoveRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type MoveResponse struct {
	Entry *EntryInfo `json:"entry"`
}

type ListDirRequest struct {
	Path  string `json:"path"`
	Depth int32  `json:"depth,omitempty"`
}

type ListDirResponse struct {
	Entries []*EntryInfo `json:"entries"`
}

type RemoveRequest struct {
	Path string `json:"path"`
}

type RemoveResponse struct{}

type WatchDirRequest struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
}

type WatchDirResponse struct {
	Start      *WatchDirStartEvent     `json:"start,omitempty"`
	Filesystem *FilesystemEvent        `json:"filesystem,omitempty"`
	Keepalive  *WatchDirKeepaliveEvent `json:"keepalive,omitempty"`
	Started    *bool                   `json:"started,omitempty"`
	Event      *FilesystemEvent        `json:"event,omitempty"`
}

func (r *WatchDirResponse) UnmarshalJSON(data []byte) error {
	type responseJSON struct {
		Start      *WatchDirStartEvent `json:"start,omitempty"`
		Filesystem *FilesystemEvent    `json:"filesystem,omitempty"`
		Keepalive  json.RawMessage     `json:"keepalive,omitempty"`
		Started    *bool               `json:"started,omitempty"`
		Event      *FilesystemEvent    `json:"event,omitempty"`
	}

	var decoded responseJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	r.Start = decoded.Start
	r.Filesystem = decoded.Filesystem
	r.Started = decoded.Started
	r.Event = decoded.Event
	if len(decoded.Keepalive) > 0 {
		r.Keepalive = &WatchDirKeepaliveEvent{}
	}
	return nil
}
