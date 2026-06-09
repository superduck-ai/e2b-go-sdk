package api

import (
	"encoding/json"
	"time"
)

// Request/Response types for the E2B Cloud API

type CreateSandboxRequest struct {
	TemplateID          string            `json:"templateID"`
	Timeout             int               `json:"timeout"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	Mcp                 map[string]any    `json:"mcp,omitempty"`
	EnvVars             map[string]string `json:"envVars,omitempty"`
	Secure              *bool             `json:"secure,omitempty"`
	AllowInternetAccess *bool             `json:"allow_internet_access,omitempty"`
	AutoPause           *bool             `json:"autoPause,omitempty"`
	AutoResume          *AutoResumeConfig `json:"autoResume,omitempty"`
	Network             *NetworkOpts      `json:"network,omitempty"`
	VolumeMounts        []VolumeMount     `json:"volumeMounts,omitempty"`
}

type NetworkOpts struct {
	AllowOut           []string                 `json:"allowOut,omitempty"`
	DenyOut            []string                 `json:"denyOut,omitempty"`
	AllowPublicTraffic *bool                    `json:"allowPublicTraffic,omitempty"`
	MaskRequestHost    string                   `json:"maskRequestHost,omitempty"`
	Rules              map[string][]NetworkRule `json:"rules,omitempty"`
}

type NetworkRule struct {
	Transform *NetworkTransform `json:"transform,omitempty"`
}

type NetworkTransform struct {
	Headers map[string]string `json:"headers,omitempty"`
}

type SandboxNetworkUpdateConfig struct {
	AllowOut            []string                 `json:"allowOut,omitempty"`
	DenyOut             []string                 `json:"denyOut,omitempty"`
	Rules               map[string][]NetworkRule `json:"rules,omitempty"`
	AllowInternetAccess *bool                    `json:"allow_internet_access,omitempty"`
}

type AutoResumeConfig struct {
	Enabled bool `json:"enabled"`
}

type VolumeMount struct {
	Name      string `json:"name,omitempty"`
	Path      string `json:"path,omitempty"`
	VolumeID  string `json:"volumeID,omitempty"`
	MountPath string `json:"mountPath,omitempty"`
}

type SandboxResponse struct {
	SandboxID           string             `json:"sandboxID"`
	TemplateID          string             `json:"templateID"`
	Name                string             `json:"name"`
	Alias               string             `json:"alias,omitempty"`
	Metadata            map[string]string  `json:"metadata"`
	StartedAt           time.Time          `json:"startedAt"`
	EndAt               time.Time          `json:"endAt"`
	State               string             `json:"state"`
	CpuCount            int                `json:"cpuCount"`
	MemoryMB            int                `json:"memoryMB"`
	EnvdURL             string             `json:"envdURL,omitempty"`
	EnvdVersion         string             `json:"envdVersion"`
	AllowInternetAccess *bool              `json:"allowInternetAccess,omitempty"`
	Domain              string             `json:"domain,omitempty"`
	SandboxDomain       string             `json:"sandboxDomain,omitempty"`
	EnvdAccessToken     string             `json:"envdAccessToken,omitempty"`
	TrafficAccessToken  string             `json:"trafficAccessToken,omitempty"`
	Network             *NetworkOpts       `json:"network,omitempty"`
	Lifecycle           *LifecycleInfoOpts `json:"lifecycle,omitempty"`
	VolumeMounts        []VolumeMount      `json:"volumeMounts,omitempty"`
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
	Timestamp     time.Time `json:"timestamp"`
	TimestampUnix int64     `json:"timestampUnix"`
	CpuUsedPct    float64   `json:"cpuUsedPct"`
	CpuCount      int       `json:"cpuCount"`
	MemUsed       int64     `json:"memUsed"`
	MemTotal      int64     `json:"memTotal"`
	MemCache      int64     `json:"memCache"`
	DiskUsed      int64     `json:"diskUsed"`
	DiskTotal     int64     `json:"diskTotal"`
	MemUsedMiB    int64     `json:"memUsedMiB"`
	MemTotalMiB   int64     `json:"memTotalMiB"`
	DiskUsedMiB   int64     `json:"diskUsedMiB"`
	DiskTotalMiB  int64     `json:"diskTotalMiB"`
}

type SandboxMetricsList []SandboxMetrics

func (m *SandboxMetricsList) UnmarshalJSON(data []byte) error {
	var direct []SandboxMetrics
	if err := json.Unmarshal(data, &direct); err == nil {
		*m = direct
		return nil
	}

	var wrapped struct {
		Data    []SandboxMetrics `json:"data"`
		Metrics []SandboxMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return err
	}
	if wrapped.Data != nil {
		*m = wrapped.Data
		return nil
	}
	if wrapped.Metrics != nil {
		*m = wrapped.Metrics
		return nil
	}

	var bySandbox map[string][]SandboxMetrics
	if err := json.Unmarshal(data, &bySandbox); err == nil {
		result := make([]SandboxMetrics, 0)
		for _, metrics := range bySandbox {
			result = append(result, metrics...)
		}
		*m = result
		return nil
	}

	var bySandboxSingle map[string]SandboxMetrics
	result := make([]SandboxMetrics, 0)
	if err := json.Unmarshal(data, &bySandboxSingle); err == nil {
		for _, metric := range bySandboxSingle {
			result = append(result, metric)
		}
		*m = result
		return nil
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	if _, ok := object["sandboxID"]; ok {
		*m = nil
		return nil
	}
	if _, ok := object["templateID"]; ok {
		*m = nil
		return nil
	}

	*m = result
	return nil
}

type SnapshotInfo struct {
	SnapshotID string   `json:"snapshotID"`
	Names      []string `json:"names,omitempty"`
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
	BuildID    string             `json:"buildID"`
	TemplateID string             `json:"templateID"`
	Status     string             `json:"status"`
	LogEntries []BuildLogEntry    `json:"logEntries,omitempty"`
	Logs       []string           `json:"logs,omitempty"`
	Reason     *BuildStatusReason `json:"reason,omitempty"`
}

type BuildLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

type BuildStatusReason struct {
	Message    string          `json:"message"`
	Step       string          `json:"step,omitempty"`
	LogEntries []BuildLogEntry `json:"logEntries,omitempty"`
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
	Target string   `json:"target"`
	Tags   []string `json:"tags"`
}

type RemoveTagsRequest struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type TemplateTag struct {
	Tag       string    `json:"tag"`
	BuildID   string    `json:"buildId"`
	CreatedAt time.Time `json:"createdAt"`
}
