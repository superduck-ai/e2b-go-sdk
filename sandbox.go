package e2b

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/e2b-dev/e2b-go-sdk/api"
	"github.com/e2b-dev/e2b-go-sdk/commands"
	"github.com/e2b-dev/e2b-go-sdk/envd"
	"github.com/e2b-dev/e2b-go-sdk/filesystem"
	"github.com/e2b-dev/e2b-go-sdk/git"
)

const mcpPort = 49984

type Sandbox struct {
	SandboxApi
	SandboxID          string
	SandboxDomain      string
	TrafficAccessToken string
	EnvdVersion        string
	envdPort           int
	mcpPort            int
	connectionConfig   *ConnectionConfig
	envdAccessToken    string
	envdApiUrl         string
	envdApi            *envd.EnvdApiClient
	mcpToken           string

	Files    *filesystem.Filesystem
	Commands *commands.Commands
	Pty      *commands.Pty
	Git      *git.Git
}

// CreateSandbox creates a new sandbox from a template and initializes all sub-modules.
func CreateSandbox(ctx context.Context, template string, opts *SandboxOpts) (*Sandbox, error) {
	if opts == nil {
		opts = &SandboxOpts{}
	}

	connConfig := NewConnectionConfig(&opts.ConnectionOpts)

	apiClient, err := api.NewApiClient(ToClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	timeoutMs := opts.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = DefaultSandboxTimeoutMs
	}

	createReq := &api.CreateSandboxRequest{
		TemplateID:          template,
		Timeout:             int(math.Ceil(float64(timeoutMs) / 1000.0)),
		Metadata:            opts.Metadata,
		EnvVars:             opts.Envs,
		Secure:              opts.Secure,
		AllowInternetAccess: opts.AllowInternetAccess,
	}
	if opts.Network != nil {
		createReq.Network = &api.NetworkOpts{
			AllowOut:           opts.Network.AllowOut,
			DenyOut:            opts.Network.DenyOut,
			AllowPublicTraffic: opts.Network.AllowPublicTraffic,
			MaskRequestHost:    opts.Network.MaskRequestHost,
		}
	}
	if opts.Lifecycle != nil {
		createReq.Lifecycle = &api.LifecycleOpts{
			OnTimeout:  opts.Lifecycle.OnTimeout,
			AutoResume: opts.Lifecycle.AutoResume,
		}
	}
	if len(opts.VolumeMounts) > 0 {
		mounts := make([]api.VolumeMount, len(opts.VolumeMounts))
		for i, m := range opts.VolumeMounts {
			mounts[i] = api.VolumeMount{VolumeID: m.VolumeID, MountPath: m.MountPath}
		}
		createReq.VolumeMounts = mounts
	}

	var sandboxResp api.SandboxResponse
	_, err = apiClient.Post(ctx, "/sandboxes", createReq, &sandboxResp)
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox: %w", err)
	}

	sbx := newSandboxFromResponse(&sandboxResp, connConfig)

	// Initialize envd
	if err := sbx.initEnvd(ctx, opts.Envs); err != nil {
		return nil, fmt.Errorf("failed to initialize sandbox envd: %w", err)
	}

	return sbx, nil
}

// ConnectSandbox connects to an existing running sandbox.
func ConnectSandbox(ctx context.Context, sandboxId string, opts *SandboxConnectOpts) (*Sandbox, error) {
	if opts == nil {
		opts = &SandboxConnectOpts{}
	}

	connConfig := NewConnectionConfig(&opts.ConnectionOpts)

	apiClient, err := api.NewApiClient(ToClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	var sandboxResp api.ConnectSandboxResponse
	_, err = apiClient.Get(ctx, "/sandboxes/"+sandboxId, &sandboxResp)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sandbox: %w", err)
	}

	sbx := newSandboxFromResponse(&sandboxResp.SandboxResponse, connConfig)

	// Fetch envd version via health check
	health, err := sbx.envdApi.Health(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check sandbox health: %w", err)
	}
	sbx.EnvdVersion = health.Version
	sbx.envdApi.Version = health.Version

	return sbx, nil
}

// ListSandboxes returns a paginator for listing sandboxes.
func ListSandboxes(opts *SandboxListOpts) *Paginator[SandboxInfo] {
	if opts == nil {
		opts = &SandboxListOpts{}
	}

	connConfig := NewConnectionConfig(&opts.ConnectionOpts)

	return NewPaginator(func(ctx context.Context, nextToken string) ([]SandboxInfo, string, error) {
		apiClient, err := api.NewApiClient(ToClientConfig(connConfig), api.WithRequireApiKey())
		if err != nil {
			return nil, "", err
		}

		path := "/v2/sandboxes"
		params := url.Values{}
		if opts.State != "" {
			params.Set("state", string(opts.State))
		}
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
		if nextToken != "" {
			params.Set("nextToken", nextToken)
		}
		for k, v := range opts.Metadata {
			params.Set("metadata."+k, v)
		}
		if q := params.Encode(); q != "" {
			path += "?" + q
		}

		var sandboxes []api.SandboxResponse
		resp, err := apiClient.Get(ctx, path, &sandboxes)
		if err != nil {
			return nil, "", err
		}

		nextTok := ""
		if resp != nil {
			nextTok = resp.Header.Get("x-next-token")
		}

		infos := make([]SandboxInfo, len(sandboxes))
		for i, s := range sandboxes {
			infos[i] = sandboxResponseToInfo(&s)
		}
		return infos, nextTok, nil
	})
}

// GetHost returns the hostname for accessing a specific port on the sandbox.
func (s *Sandbox) GetHost(port int) string {
	return s.connectionConfig.GetHost(s.SandboxID, port, s.SandboxDomain)
}

// IsRunning checks if the sandbox envd is healthy.
func (s *Sandbox) IsRunning(ctx context.Context) (bool, error) {
	_, err := s.envdApi.Health(ctx)
	if err != nil {
		// If it's a connection error, the sandbox is not running
		return false, nil
	}
	return true, nil
}

// SetSandboxTimeout sets the sandbox timeout.
func (s *Sandbox) SetSandboxTimeout(ctx context.Context, timeoutMs int, opts *ConnectionOpts) error {
	connConfig := s.resolveConnectionConfig(opts)
	apiClient, err := api.NewApiClient(ToClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return err
	}
	reqBody := &api.SetTimeoutRequest{Timeout: timeoutMs}
	_, err = apiClient.Post(ctx, "/sandboxes/"+s.SandboxID+"/timeout", reqBody, nil)
	return err
}

// Kill terminates the sandbox.
func (s *Sandbox) Kill(ctx context.Context, opts *ConnectionOpts) error {
	connConfig := s.resolveConnectionConfig(opts)
	apiClient, err := api.NewApiClient(ToClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return err
	}
	_, err = apiClient.Delete(ctx, "/sandboxes/"+s.SandboxID, nil)
	return err
}

// PauseSandbox pauses the sandbox.
func (s *Sandbox) PauseSandbox(ctx context.Context, opts *ConnectionOpts) (bool, error) {
	connConfig := s.resolveConnectionConfig(opts)
	apiClient, err := api.NewApiClient(ToClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return false, err
	}
	_, err = apiClient.Post(ctx, "/sandboxes/"+s.SandboxID+"/pause", nil, nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateSandboxSnapshot creates a snapshot of the sandbox.
func (s *Sandbox) CreateSandboxSnapshot(ctx context.Context, opts *ConnectionOpts) (*SnapshotInfo, error) {
	connConfig := s.resolveConnectionConfig(opts)
	apiClient, err := api.NewApiClient(ToClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}
	var snapshot api.SnapshotInfo
	_, err = apiClient.Post(ctx, "/sandboxes/"+s.SandboxID+"/snapshots", nil, &snapshot)
	if err != nil {
		return nil, err
	}
	return &SnapshotInfo{SnapshotID: snapshot.SnapshotID}, nil
}

// ListSandboxSnapshots returns a paginator for listing snapshots of this sandbox.
func (s *Sandbox) ListSandboxSnapshots(ctx context.Context, opts *SnapshotListOpts) *Paginator[SnapshotInfo] {
	if opts == nil {
		opts = &SnapshotListOpts{}
	}

	connConfig := s.resolveConnectionConfig(&opts.ConnectionOpts)

	sandboxID := s.SandboxID
	if opts.SandboxID != "" {
		sandboxID = opts.SandboxID
	}

	return NewPaginator(func(ctx context.Context, nextToken string) ([]SnapshotInfo, string, error) {
		apiClient, err := api.NewApiClient(ToClientConfig(connConfig), api.WithRequireApiKey())
		if err != nil {
			return nil, "", err
		}

		path := "/sandboxes/" + sandboxID + "/snapshots"
		params := url.Values{}
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
		if nextToken != "" {
			params.Set("nextToken", nextToken)
		}
		if q := params.Encode(); q != "" {
			path += "?" + q
		}

		var listResp api.SnapshotListResponse
		_, err = apiClient.Get(ctx, path, &listResp)
		if err != nil {
			return nil, "", err
		}

		infos := make([]SnapshotInfo, len(listResp.Data))
		for i, s := range listResp.Data {
			infos[i] = SnapshotInfo{SnapshotID: s.SnapshotID}
		}
		return infos, listResp.NextToken, nil
	})
}

// GetMcpUrl returns the MCP endpoint URL for this sandbox.
func (s *Sandbox) GetMcpUrl() string {
	return fmt.Sprintf("https://%s", s.connectionConfig.GetHost(s.SandboxID, s.mcpPort, s.SandboxDomain))
}

// GetMcpToken retrieves or returns the cached MCP authentication token.
func (s *Sandbox) GetMcpToken(ctx context.Context) (string, error) {
	if s.mcpToken != "" {
		return s.mcpToken, nil
	}

	// The MCP token is typically the envd access token or traffic access token
	if s.envdAccessToken != "" {
		s.mcpToken = s.envdAccessToken
		return s.mcpToken, nil
	}
	if s.TrafficAccessToken != "" {
		s.mcpToken = s.TrafficAccessToken
		return s.mcpToken, nil
	}

	return "", fmt.Errorf("no MCP token available: sandbox has no access token configured")
}

// UploadUrl generates a signed URL for uploading a file to the sandbox.
func (s *Sandbox) UploadUrl(ctx context.Context, path string, opts *ConnectionOpts) (string, error) {
	user := DefaultUsername

	sig, err := GetSignature(SignatureOpts{
		Path:            path,
		Operation:       "write",
		User:            user,
		EnvdAccessToken: s.envdAccessToken,
	})
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Set("path", path)
	params.Set("username", user)
	if sig.Signature != "" {
		params.Set("signature", sig.Signature)
	}
	if sig.Expiration != nil {
		params.Set("expiration", strconv.FormatInt(*sig.Expiration, 10))
	}

	uploadUrl := fmt.Sprintf("%s/files?%s", s.envdApiUrl, params.Encode())
	return uploadUrl, nil
}

// DownloadUrl generates a signed URL for downloading a file from the sandbox.
func (s *Sandbox) DownloadUrl(ctx context.Context, path string, opts *ConnectionOpts) (string, error) {
	user := DefaultUsername

	sig, err := GetSignature(SignatureOpts{
		Path:            path,
		Operation:       "read",
		User:            user,
		EnvdAccessToken: s.envdAccessToken,
	})
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Set("path", path)
	params.Set("username", user)
	if sig.Signature != "" {
		params.Set("signature", sig.Signature)
	}
	if sig.Expiration != nil {
		params.Set("expiration", strconv.FormatInt(*sig.Expiration, 10))
	}

	downloadUrl := fmt.Sprintf("%s/files?%s", s.envdApiUrl, params.Encode())
	return downloadUrl, nil
}

// GetSandboxInfo retrieves information about this sandbox from the API.
func (s *Sandbox) GetSandboxInfo(ctx context.Context, opts *ConnectionOpts) (*SandboxInfo, error) {
	connConfig := s.resolveConnectionConfig(opts)
	apiClient, err := api.NewApiClient(ToClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	var sandboxResp api.SandboxResponse
	_, err = apiClient.Get(ctx, "/sandboxes/"+s.SandboxID, &sandboxResp)
	if err != nil {
		return nil, err
	}

	info := sandboxResponseToInfo(&sandboxResp)
	return &info, nil
}

// GetSandboxMetrics retrieves resource usage metrics for this sandbox.
func (s *Sandbox) GetSandboxMetrics(ctx context.Context, opts *SandboxMetricsOpts) ([]SandboxMetrics, error) {
	if opts == nil {
		opts = &SandboxMetricsOpts{}
	}

	connConfig := s.resolveConnectionConfig(&opts.ConnectionOpts)
	apiClient, err := api.NewApiClient(ToClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	path := "/sandboxes/" + s.SandboxID + "/metrics"
	params := url.Values{}
	if opts.Start != nil {
		params.Set("start", opts.Start.Format(http.TimeFormat))
	}
	if opts.End != nil {
		params.Set("end", opts.End.Format(http.TimeFormat))
	}
	if q := params.Encode(); q != "" {
		path += "?" + q
	}

	var metricsResp []api.SandboxMetrics
	_, err = apiClient.Get(ctx, path, &metricsResp)
	if err != nil {
		return nil, err
	}

	metrics := make([]SandboxMetrics, len(metricsResp))
	for i, m := range metricsResp {
		metrics[i] = SandboxMetrics{
			Timestamp:    m.Timestamp,
			CpuUsedPct:   m.CpuUsedPct,
			CpuCount:     m.CpuCount,
			MemUsedMiB:   m.MemUsedMiB,
			MemTotalMiB:  m.MemTotalMiB,
			DiskUsedMiB:  m.DiskUsedMiB,
			DiskTotalMiB: m.DiskTotalMiB,
		}
	}
	return metrics, nil
}

// --- internal helpers ---

func newSandboxFromResponse(resp *api.SandboxResponse, connConfig *ConnectionConfig) *Sandbox {
	sandboxDomain := resp.SandboxDomain
	if sandboxDomain == "" {
		sandboxDomain = connConfig.Domain
	}

	envdApiUrl := connConfig.GetSandboxUrl(resp.SandboxID, sandboxDomain, EnvdPort)

	envdApiClient := envd.NewEnvdApiClient(
		envdApiUrl,
		resp.EnvdAccessToken,
		connConfig.Headers,
		connConfig.RequestTimeoutMs,
	)

	cmdConnConfig := &commands.ConnectionConfig{
		ApiKey:           connConfig.ApiKey,
		AccessToken:      connConfig.AccessToken,
		Domain:           connConfig.Domain,
		ApiUrl:           connConfig.ApiUrl,
		SandboxUrl:       envdApiUrl,
		Debug:            connConfig.Debug,
		RequestTimeoutMs: connConfig.RequestTimeoutMs,
		Headers:          connConfig.Headers,
	}

	fsConnConfig := &filesystem.ConnectionConfig{
		ApiKey:           connConfig.ApiKey,
		AccessToken:      connConfig.AccessToken,
		Domain:           connConfig.Domain,
		ApiUrl:           connConfig.ApiUrl,
		SandboxUrl:       envdApiUrl,
		Debug:            connConfig.Debug,
		RequestTimeoutMs: connConfig.RequestTimeoutMs,
		Headers:          connConfig.Headers,
	}

	envdVersion := resp.EnvdVersion

	cmds := commands.NewCommands(cmdConnConfig, envdVersion)
	pty := commands.NewPty(cmdConnConfig, envdVersion)
	fs := filesystem.NewFilesystem(fsConnConfig, envdVersion)
	g := git.NewGit(cmds)

	return &Sandbox{
		SandboxID:          resp.SandboxID,
		SandboxDomain:      sandboxDomain,
		TrafficAccessToken: resp.TrafficAccessToken,
		EnvdVersion:        envdVersion,
		envdPort:           EnvdPort,
		mcpPort:            mcpPort,
		connectionConfig:   connConfig,
		envdAccessToken:    resp.EnvdAccessToken,
		envdApiUrl:         envdApiUrl,
		envdApi:            envdApiClient,
		Files:              fs,
		Commands:           cmds,
		Pty:                pty,
		Git:                g,
	}
}

func (s *Sandbox) initEnvd(ctx context.Context, envVars map[string]string) error {
	// Check health - confirms sandbox is reachable
	health, err := s.envdApi.Health(ctx)
	if err != nil {
		return fmt.Errorf("sandbox health check failed: %w", err)
	}
	if health.Version != "" {
		s.EnvdVersion = health.Version
		s.envdApi.Version = health.Version
	}

	// Initialize with env vars
	if envVars == nil {
		envVars = map[string]string{}
	}
	if err := s.envdApi.Init(ctx, &envd.InitRequest{EnvVars: envVars}); err != nil {
		return fmt.Errorf("sandbox init failed: %w", err)
	}

	return nil
}

func (s *Sandbox) resolveConnectionConfig(opts *ConnectionOpts) *ConnectionConfig {
	if opts == nil {
		return s.connectionConfig
	}
	// Merge: use sandbox's config as base, override with opts if provided
	merged := *s.connectionConfig
	if opts.ApiKey != "" {
		merged.ApiKey = opts.ApiKey
	}
	if opts.AccessToken != "" {
		merged.AccessToken = opts.AccessToken
	}
	if opts.Domain != "" {
		merged.Domain = opts.Domain
	}
	if opts.ApiUrl != "" {
		merged.ApiUrl = opts.ApiUrl
	}
	if opts.RequestTimeoutMs != 0 {
		merged.RequestTimeoutMs = opts.RequestTimeoutMs
	}
	if opts.Headers != nil {
		merged.Headers = opts.Headers
	}
	return &merged
}

func sandboxResponseToInfo(s *api.SandboxResponse) SandboxInfo {
	info := SandboxInfo{
		SandboxID:   s.SandboxID,
		TemplateID:  s.TemplateID,
		Name:        s.Name,
		Metadata:    s.Metadata,
		StartedAt:   s.StartedAt,
		EndAt:       s.EndAt,
		State:       SandboxState(s.State),
		CpuCount:    s.CpuCount,
		MemoryMB:    s.MemoryMB,
		EnvdVersion: s.EnvdVersion,
	}
	if s.Network != nil {
		info.Network = &SandboxNetworkOpts{
			AllowOut:           s.Network.AllowOut,
			DenyOut:            s.Network.DenyOut,
			AllowPublicTraffic: s.Network.AllowPublicTraffic,
			MaskRequestHost:    s.Network.MaskRequestHost,
		}
	}
	if s.Lifecycle != nil {
		info.Lifecycle = &SandboxInfoLifecycle{
			OnTimeout:  s.Lifecycle.OnTimeout,
			AutoResume: s.Lifecycle.AutoResume,
		}
	}
	if len(s.VolumeMounts) > 0 {
		info.VolumeMounts = make([]VolumeMountInfo, len(s.VolumeMounts))
		for i, m := range s.VolumeMounts {
			info.VolumeMounts[i] = VolumeMountInfo{VolumeID: m.VolumeID, MountPath: m.MountPath}
		}
	}
	return info
}
