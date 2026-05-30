package volume

import (
	"context"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/api"
)

type VolumeFileType string

const (
	VolumeFileTypeUnknown   VolumeFileType = "unknown"
	VolumeFileTypeFile      VolumeFileType = "file"
	VolumeFileTypeDirectory VolumeFileType = "directory"
	VolumeFileTypeSymlink   VolumeFileType = "symlink"
)

type VolumeInfo struct {
	VolumeID string
	Name     string
}

type VolumeAndToken struct {
	VolumeInfo
	Token string
}

type VolumeEntryStat struct {
	Atime  time.Time
	Mtime  time.Time
	Ctime  time.Time
	Type   VolumeFileType
	Name   string
	Path   string
	Size   int64
	UID    int
	GID    int
	Mode   int
	Target string
}

type VolumeMetadataOptions struct {
	UID  *int
	GID  *int
	Mode *int
}

type VolumeWriteOptions struct {
	VolumeMetadataOptions
	Force            *bool
	Token            string
	Domain           string
	Debug            *bool
	ApiUrl           string
	RequestTimeoutMs *int
	Signal           context.Context
	Logger           api.Logger
	Headers          map[string]string
	Proxy            string
}
