package envd

// Envd HTTP API types (health, files, metrics, init, envs)

type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type MetricsResponse struct {
	Timestamp    string  `json:"timestamp"`
	CpuUsedPct   float64 `json:"cpuUsedPct"`
	CpuCount     int     `json:"cpuCount"`
	MemUsedMiB   int64   `json:"memUsedMiB"`
	MemTotalMiB  int64   `json:"memTotalMiB"`
	DiskUsedMiB  int64   `json:"diskUsedMiB"`
	DiskTotalMiB int64   `json:"diskTotalMiB"`
}

type InitRequest struct {
	EnvVars map[string]string `json:"envVars,omitempty"`
}
