package e2b

import rootvol "github.com/superduck-ai/e2b-go-sdk/volume"

const (
	VolumeFileTypeUnknown   = rootvol.VolumeFileTypeUnknown
	VolumeFileTypeFile      = rootvol.VolumeFileTypeFile
	VolumeFileTypeDirectory = rootvol.VolumeFileTypeDirectory
	VolumeFileTypeSymlink   = rootvol.VolumeFileTypeSymlink
)

type Volume = rootvol.Volume
type VolumeFileType = rootvol.VolumeFileType
type VolumeInfo = rootvol.VolumeInfo
type VolumeAndToken = rootvol.VolumeAndToken
type VolumeEntryStat = rootvol.VolumeEntryStat
type VolumeMetadataOptions = rootvol.VolumeMetadataOptions
type VolumeWriteOptions = rootvol.VolumeWriteOptions
type VolumeApiOpts = rootvol.VolumeApiOpts
type VolumeConnectionConfig = rootvol.VolumeConnectionConfig
