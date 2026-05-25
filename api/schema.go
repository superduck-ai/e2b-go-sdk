package api

import "time"

// Request/Response types for the E2B Cloud API

type CreateSandboxRequest struct {
	TemplateID          string            `json:"templateID"`
	Timeout             int               `json:"timeout,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	EnvVars             map[string]string `json:"envVars,omitempty"`
	Secure              bool              `json:"secure,omitempty"`
	AllowInternetAccess *bool             `json:"allowInternetAccess,omitempty"`
	Network             *NetworkOpts      `json:"network,omitempty"`
	Lifecycle           *LifecycleOpts    `json:"lifecycle,omitempty"`
	VolumeMounts        []VolumeMount     `json:"volumeMounts,omitempty"`
}

type NetworkOpts struct {
	AllowOut           []string `json:"allowOut,omitempty"`
	DenyOut            []string `json:"denyOut,omitempty"`
	AllowPublicTraffic bool     `json:"allowPublicTraffic,omitempty"`
	MaskRequestHost    bool     `json:"maskRequestHost,omitempty"`
}

type LifecycleOpts struct {
	OnTimeout  string `json:"onTimeout,omitempty"`
	AutoResume bool   `json:"autoResume,omitempty"`
}

type VolumeMount struct {
	VolumeID  string `json:"volumeID"`
	MountPath string `json:"mountPath"`
}

type SandboxResponse struct {
	SandboxID          string             `json:"sandboxID"`
	TemplateID         string             `json:"templateID"`
	Name               string             `json:"name"`
	Metadata           map[string]string  `json:"metadata"`
	StartedAt          time.Time          `json:"startedAt"`
	EndAt              time.Time          `json:"endAt"`
	State              string             `json:"state"`
	CpuCount           int                `json:"cpuCount"`
	MemoryMB           int                `json:"memoryMB"`
	EnvdVersion        string             `json:"envdVersion"`
	SandboxDomain      string             `json:"sandboxDomain,omitempty"`
	EnvdAccessToken    string             `json:"envdAccessToken,omitempty"`
	TrafficAccessToken string             `json:"trafficAccessToken,omitempty"`
	Network            *NetworkOpts       `json:"network,omitempty"`
	Lifecycle          *LifecycleInfoOpts `json:"lifecycle,omitempty"`
	VolumeMounts       []VolumeMount      `json:"volumeMounts,omitempty"`
}

type LifecycleInfoOpts struct {
	OnTimeout  string `json:"onTimeout,omitempty"`
	AutoResume bool   `json:"autoResume,omitempty"`
}

type ConnectSandboxResponse struct {
	SandboxResponse
}

type SetTimeoutRequest struct {
	Timeout int `json:"timeout"`
}

type SandboxListResponse struct {
	Data      []SandboxResponse `json:"data"`
	NextToken string            `json:"nextToken,omitempty"`
}

type SandboxMetrics struct {
	Timestamp    time.Time `json:"timestamp"`
	CpuUsedPct   float64   `json:"cpuUsedPct"`
	CpuCount     int       `json:"cpuCount"`
	MemUsedMiB   int64     `json:"memUsedMiB"`
	MemTotalMiB  int64     `json:"memTotalMiB"`
	DiskUsedMiB  int64     `json:"diskUsedMiB"`
	DiskTotalMiB int64     `json:"diskTotalMiB"`
}

type SnapshotInfo struct {
	SnapshotID string `json:"snapshotID"`
}

type SnapshotListResponse struct {
	Data      []SnapshotInfo `json:"data"`
	NextToken string         `json:"nextToken,omitempty"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

// Template build types

type CreateTemplateRequest struct {
	Name     string `json:"name,omitempty"`
	CpuCount int    `json:"cpuCount,omitempty"`
	MemoryMB int    `json:"memoryMB,omitempty"`
}

type CreateTemplateResponse struct {
	TemplateID string `json:"templateID"`
	BuildID    string `json:"buildID"`
}

type BuildStatusResponse struct {
	BuildID    string `json:"buildID"`
	TemplateID string `json:"templateID"`
	Status     string `json:"status"`
	Logs       string `json:"logs,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// Volume types

type CreateVolumeRequest struct {
	Name string `json:"name"`
}

type VolumeResponse struct {
	VolumeID string `json:"volumeID"`
	Name     string `json:"name"`
}

type VolumeAndTokenResponse struct {
	VolumeID string `json:"volumeID"`
	Name     string `json:"name"`
	Token    string `json:"token"`
}

// Tags

type AssignTagsRequest struct {
	TemplateName string   `json:"templateName"`
	Tags         []string `json:"tags"`
}

type RemoveTagsRequest struct {
	TemplateName string   `json:"templateName"`
	Tags         []string `json:"tags"`
}

type TemplateTag struct {
	Tag       string    `json:"tag"`
	BuildID   string    `json:"buildId"`
	CreatedAt time.Time `json:"createdAt"`
}
