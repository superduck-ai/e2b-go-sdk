package volume

// Volume content API types (auto-generated equivalent)

type listDirResponse struct {
	Entries []VolumeEntryStat `json:"entries"`
}

type createDirResponse struct {
	Entry VolumeEntryStat `json:"entry"`
}

type statResponse struct {
	Entry VolumeEntryStat `json:"entry"`
}

type updateMetadataRequest struct {
	UID  *int `json:"uid,omitempty"`
	GID  *int `json:"gid,omitempty"`
	Mode *int `json:"mode,omitempty"`
}
