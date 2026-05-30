package e2b

import (
	"context"

	rootvol "github.com/superduck-ai/e2b-go-sdk/volume"
)

const (
	VolumeFileTypeUnknown   = rootvol.VolumeFileTypeUnknown
	VolumeFileTypeFile      = rootvol.VolumeFileTypeFile
	VolumeFileTypeDirectory = rootvol.VolumeFileTypeDirectory
	VolumeFileTypeSymlink   = rootvol.VolumeFileTypeSymlink
)

const (
	ReadFileFormatText   = rootvol.ReadFileFormatText
	ReadFileFormatBytes  = rootvol.ReadFileFormatBytes
	ReadFileFormatStream = rootvol.ReadFileFormatStream
	ReadFileFormatBlob   = rootvol.ReadFileFormatBlob
)

type Volume = rootvol.Volume
type ReadFileFormat = rootvol.ReadFileFormat
type VolumeFileType = rootvol.VolumeFileType
type VolumeInfo = rootvol.VolumeInfo
type VolumeAndToken = rootvol.VolumeAndToken
type VolumeEntryStat = rootvol.VolumeEntryStat
type VolumeMetadataOptions = rootvol.VolumeMetadataOptions
type VolumeWriteOptions = rootvol.VolumeWriteOptions
type VolumeApiOpts = rootvol.VolumeApiOpts
type VolumeReadOpts = rootvol.VolumeReadOpts
type VolumeListOpts = rootvol.VolumeListOpts
type VolumeConnectionConfig = rootvol.VolumeConnectionConfig
type VolumeConnectionOpts = rootvol.ConnectionOpts

func CreateVolume(ctx context.Context, name string, opts *VolumeConnectionOpts) (*Volume, error) {
	return rootvol.Create(ctx, name, opts)
}

func ConnectVolume(ctx context.Context, volumeID string, opts *VolumeConnectionOpts) (*Volume, error) {
	return rootvol.Connect(ctx, volumeID, opts)
}

func GetVolumeInfo(ctx context.Context, volumeID string, opts *VolumeConnectionOpts) (*VolumeAndToken, error) {
	return rootvol.GetInfo(ctx, volumeID, opts)
}

func ListVolumes(ctx context.Context, opts *VolumeConnectionOpts) ([]VolumeInfo, error) {
	return rootvol.List(ctx, opts)
}

func DestroyVolume(ctx context.Context, volumeID string, opts *VolumeConnectionOpts) (bool, error) {
	return rootvol.Destroy(ctx, volumeID, opts)
}
