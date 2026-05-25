package volume

// Volume content API types (auto-generated equivalent)

type ListDirResponse struct {
	Entries []VolumeEntryStat `json:"entries"`
}

type CreateDirResponse struct {
	Entry VolumeEntryStat `json:"entry"`
}

type StatResponse struct {
	Entry VolumeEntryStat `json:"entry"`
}

type UpdateMetadataRequest struct {
	UID  *int `json:"uid,omitempty"`
	GID  *int `json:"gid,omitempty"`
	Mode *int `json:"mode,omitempty"`
}
