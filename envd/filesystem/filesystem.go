package filesystem

// FileType for filesystem entries
type FileType int32

const (
	FileTypeUnspecified FileType = 0
	FileTypeFile        FileType = 1
	FileTypeDirectory   FileType = 2
)

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

// FilesystemEvent represents a watch event
type FilesystemEvent struct {
	Name string    `json:"name"`
	Type EventType `json:"type"`
}

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
	Started *bool            `json:"started,omitempty"`
	Event   *FilesystemEvent `json:"event,omitempty"`
}
