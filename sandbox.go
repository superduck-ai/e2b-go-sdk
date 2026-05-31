package e2b

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/superduck-ai/e2b-go-sdk/api"
	"github.com/superduck-ai/e2b-go-sdk/commands"
	"github.com/superduck-ai/e2b-go-sdk/envd"
	"github.com/superduck-ai/e2b-go-sdk/filesystem"
	"github.com/superduck-ai/e2b-go-sdk/git"
	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

const mcpPort = 50005

type Sandbox struct {
	SandboxID          string
	SandboxDomain      string
	TrafficAccessToken string
	envdPort           int
	mcpPort            int
	connectionConfig   *ConnectionConfig
	envdVersion        string
	envdAccessToken    string
	envdApiUrl         string
	envdDirectUrl      string
	envdApi            *envd.EnvdApiClient
	mcpToken           string

	Files    *filesystem.Filesystem
	Commands *commands.Commands
	Pty      *commands.Pty
	Git      *git.Git
}

func createSandbox(ctx context.Context, template string, opts *SandboxOpts, autoPause bool) (*Sandbox, error) {
	if opts == nil {
		opts = &SandboxOpts{}
	}
	ctx, cancel := shared.MergeContexts(ctx, opts.Signal)
	defer cancel()
	if template == "" {
		template = opts.Template
	}
	if template == "" {
		if opts.Mcp != nil {
			template = defaultSandboxMcpTemplate
		} else {
			template = defaultSandboxTemplate
		}
	}

	connConfig := NewConnectionConfig(&opts.ConnectionOpts)
	if connConfig.Debug {
		return newDebugSandbox(connConfig), nil
	}

	apiClient, err := api.NewApiClient(toClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	timeoutMs := defaultSandboxTimeoutMs
	if opts.TimeoutMs != nil {
		timeoutMs = *opts.TimeoutMs
	}

	createReq, err := buildCreateSandboxRequest(template, opts, autoPause, timeoutMs)
	if err != nil {
		return nil, err
	}

	var sandboxResp api.SandboxResponse
	_, err = apiClient.Post(ctx, "/sandboxes", &createReq, &sandboxResp)
	if err != nil {
		return nil, err
	}
	if err := ensureSandboxConnectionResponseData(&sandboxResp); err != nil {
		return nil, err
	}
	if err := ensureSupportedTemplateEnvd(ctx, apiClient, sandboxResp.SandboxID, sandboxResp.EnvdVersion); err != nil {
		return nil, err
	}

	sbx := newSandboxFromResponse(&sandboxResp, connConfig)

	if opts.Mcp != nil {
		configJSON, err := json.Marshal(opts.Mcp)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal MCP config: %w", err)
		}
		sbx.mcpToken = uuid.NewString()
		execution, err := sbx.Commands.Run(ctx, "mcp-gateway --config "+shellQuote(string(configJSON)), &commands.CommandStartOpts{
			User: "root",
			Envs: map[string]string{
				"GATEWAY_ACCESS_TOKEN": sbx.mcpToken,
			},
		})
		if err != nil {
			var exitErr *commands.CommandExitError
			if errors.As(err, &exitErr) {
				return nil, fmt.Errorf("Failed to start MCP gateway: %s", exitErr.Stderr)
			}
			return nil, fmt.Errorf("Failed to start MCP gateway: %w", err)
		}
		res, ok := execution.(*commands.CommandResult)
		if !ok {
			return nil, fmt.Errorf("Failed to start MCP gateway: expected foreground command result, got %T", execution)
		}
		if res.ExitCode != 0 {
			return nil, fmt.Errorf("Failed to start MCP gateway: %s", res.Stderr)
		}
	}

	return sbx, nil
}

// Create creates a new sandbox from a template and initializes all sub-modules.
func Create(ctx context.Context, template string, opts *SandboxOpts) (*Sandbox, error) {
	return createSandbox(ctx, template, opts, false)
}

// BetaCreate is deprecated. Use CreateSandbox instead.
func BetaCreate(ctx context.Context, template string, opts *SandboxBetaCreateOpts) (*Sandbox, error) {
	if opts == nil {
		return createSandbox(ctx, template, nil, false)
	}

	return createSandbox(ctx, template, &opts.SandboxOpts, opts.AutoPause)
}

// Connect connects to an existing running sandbox.
func Connect(ctx context.Context, sandboxId string, opts *SandboxConnectOpts) (*Sandbox, error) {
	if opts == nil {
		opts = &SandboxConnectOpts{}
	}
	ctx, cancel := shared.MergeContexts(ctx, opts.Signal)
	defer cancel()

	connConfig := NewConnectionConfig(&opts.ConnectionOpts)

	apiClient, err := api.NewApiClient(toClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	timeoutMs := defaultSandboxTimeoutMs
	if opts.TimeoutMs != nil {
		timeoutMs = *opts.TimeoutMs
	}

	reqBody := struct {
		Timeout int `json:"timeout"`
	}{
		Timeout: int(math.Ceil(float64(timeoutMs) / 1000.0)),
	}

	var sandboxResp api.SandboxResponse
	_, err = apiClient.Post(ctx, "/sandboxes/"+sandboxId+"/connect", reqBody, &sandboxResp)
	if err != nil {
		if wrapped := wrapPausedSandboxNotFoundError(sandboxId, err); wrapped != err {
			return nil, wrapped
		}
		return nil, err
	}
	if err := ensureSandboxConnectionResponseData(&sandboxResp); err != nil {
		return nil, err
	}

	sbx := newSandboxFromResponse(&sandboxResp, connConfig)

	return sbx, nil
}

// Connect reconnects to this sandbox and returns the same sandbox handle.
func (s *Sandbox) Connect(ctx context.Context, opts *SandboxConnectOpts) (*Sandbox, error) {
	mergedOpts := &SandboxConnectOpts{}
	if opts != nil {
		*mergedOpts = *opts
	}
	ctx, cancel := shared.MergeContexts(ctx, mergedOpts.Signal)
	defer cancel()

	connConfig := s.resolveConnectionConfig(&mergedOpts.ConnectionOpts)
	mergedOpts.ConnectionOpts = ConnectionOpts{
		ApiKey:           connConfig.ApiKey,
		AccessToken:      connConfig.AccessToken,
		Domain:           connConfig.Domain,
		ApiUrl:           connConfig.ApiUrl,
		SandboxUrl:       connConfig.SandboxUrl,
		Debug:            boolRef(connConfig.Debug),
		Signal:           mergedOpts.Signal,
		RequestTimeoutMs: intPtr(connConfig.RequestTimeoutMs),
		Logger:           connConfig.Logger,
		Headers:          connConfig.Headers,
		Proxy:            connConfig.Proxy,
	}

	apiSandbox := &sandboxApi{}
	if _, err := apiSandbox.connectSandbox(ctx, s.SandboxID, mergedOpts); err != nil {
		return nil, err
	}

	return s, nil
}

// List returns a paginator for listing sandboxes.
func List(opts *SandboxListOpts) *SandboxPaginator {
	if opts == nil {
		opts = &SandboxListOpts{}
	}

	queryMetadata, queryStates := resolveSandboxListQuery(opts)
	fetchPage := func(ctx context.Context, nextToken string, override *SandboxApiOpts) ([]SandboxInfo, string, error) {
		effectiveOpts := sandboxApiOptsFromSandboxListOpts(opts)
		if override != nil {
			effectiveOpts = mergeSandboxApiOpts(effectiveOpts, *override)
		}
		ctx, cancel := mergeSandboxApiSignal(ctx, &effectiveOpts)
		defer cancel()
		connConfig := newConnectionConfigFromSandboxApiOpts(&effectiveOpts)
		apiClient, err := api.NewApiClient(toClientConfig(connConfig), api.WithRequireApiKey())
		if err != nil {
			return nil, "", err
		}

		path := "/v2/sandboxes"
		params := url.Values{}
		for _, state := range queryStates {
			if state != "" {
				params.Add("state", string(state))
			}
		}
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
		if nextToken != "" {
			params.Set("nextToken", nextToken)
		}
		if len(queryMetadata) > 0 {
			metadata := url.Values{}
			for k, v := range queryMetadata {
				metadata.Set(k, v)
			}
			params.Set("metadata", metadata.Encode())
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
	}

	return &SandboxPaginator{
		paginator: newPaginatorWithInitialToken(func(ctx context.Context, nextToken string) ([]SandboxInfo, string, error) {
			return fetchPage(ctx, nextToken, nil)
		}, opts.NextToken),
		fetchWithOpts: fetchPage,
	}
}

// GetHost returns the hostname for accessing a specific port on the sandbox.
func (s *Sandbox) GetHost(port int) string {
	return s.connectionConfig.GetHost(s.SandboxID, port, s.SandboxDomain)
}

// IsRunning checks if the sandbox envd is healthy.
func (s *Sandbox) IsRunning(ctx context.Context, opts *struct {
	RequestTimeoutMs *int
	Signal           context.Context
}) (bool, error) {
	var signal context.Context
	if opts != nil {
		signal = opts.Signal
	}
	ctx, cancel := shared.MergeContexts(ctx, signal)
	defer cancel()

	if opts == nil || opts.RequestTimeoutMs == nil {
		_, err := s.envdApi.Health(ctx)
		if err != nil {
			var timeoutErr *envd.TimeoutError
			if errors.As(err, &timeoutErr) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}

	connConfig := s.resolveSandboxRequestTimeoutConnectionConfig(opts.RequestTimeoutMs)
	client := shared.NewEnvdRESTHTTPClient(time.Duration(connConfig.RequestTimeoutMs)*time.Millisecond, connConfig.Proxy, connConfig.Logger)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.envdApiUrl+"/health", nil)
	if err != nil {
		return false, err
	}
	for k, v := range s.envdApi.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	if apiErr := envd.HandleEnvdApiError(resp.StatusCode, body); apiErr != nil {
		var timeoutErr *envd.TimeoutError
		if errors.As(apiErr, &timeoutErr) {
			return false, nil
		}
		return false, apiErr
	}

	return true, nil
}

// SetTimeout sets the sandbox timeout.
func (s *Sandbox) SetTimeout(ctx context.Context, timeoutMs int, opts *struct {
	RequestTimeoutMs *int
	Signal           context.Context
}) error {
	var signal context.Context
	var requestTimeoutMs *int
	if opts != nil {
		requestTimeoutMs = opts.RequestTimeoutMs
		signal = opts.Signal
	}
	ctx, cancel := shared.MergeContexts(ctx, signal)
	defer cancel()
	connConfig := s.resolveSandboxRequestTimeoutConnectionConfig(requestTimeoutMs)
	if connConfig.Debug {
		return nil
	}
	apiClient, err := api.NewApiClient(toClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return err
	}
	reqBody := &api.SetTimeoutRequest{Timeout: int(math.Ceil(float64(timeoutMs) / 1000.0))}
	_, err = apiClient.Post(ctx, "/sandboxes/"+s.SandboxID+"/timeout", reqBody, nil)
	return wrapSandboxNotFoundError(s.SandboxID, err)
}

// UpdateNetwork updates the sandbox egress configuration atomically.
func (s *Sandbox) UpdateNetwork(ctx context.Context, network SandboxNetworkUpdate, opts *struct {
	RequestTimeoutMs *int
	Signal           context.Context
}) error {
	var signal context.Context
	var requestTimeoutMs *int
	if opts != nil {
		requestTimeoutMs = opts.RequestTimeoutMs
		signal = opts.Signal
	}
	ctx, cancel := shared.MergeContexts(ctx, signal)
	defer cancel()
	connConfig := s.resolveSandboxRequestTimeoutConnectionConfig(requestTimeoutMs)
	apiClient, err := api.NewApiClient(toClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return err
	}

	reqBody, err := buildNetworkUpdateBody(network)
	if err != nil {
		return err
	}
	_, err = apiClient.Put(ctx, "/sandboxes/"+s.SandboxID+"/network", reqBody, nil)
	return wrapSandboxNotFoundError(s.SandboxID, err)
}

// Kill terminates the sandbox.
func (s *Sandbox) Kill(ctx context.Context, opts *struct {
	RequestTimeoutMs *int
	Signal           context.Context
}) error {
	var signal context.Context
	var requestTimeoutMs *int
	if opts != nil {
		requestTimeoutMs = opts.RequestTimeoutMs
		signal = opts.Signal
	}
	ctx, cancel := shared.MergeContexts(ctx, signal)
	defer cancel()
	connConfig := s.resolveSandboxRequestTimeoutConnectionConfig(requestTimeoutMs)
	if connConfig.Debug {
		return nil
	}
	apiClient, err := api.NewApiClient(toClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return err
	}
	_, err = apiClient.Delete(ctx, "/sandboxes/"+s.SandboxID, nil)
	if err == nil {
		return nil
	}

	var nfe *api.NotFoundError
	if errors.As(err, &nfe) {
		return nil
	}

	return err
}

// Pause pauses the sandbox.
func (s *Sandbox) Pause(ctx context.Context, opts *ConnectionOpts) (bool, error) {
	var signal context.Context
	if opts != nil {
		signal = opts.Signal
	}
	ctx, cancel := shared.MergeContexts(ctx, signal)
	defer cancel()
	connConfig := s.resolveConnectionConfig(opts)
	apiClient, err := api.NewApiClient(toClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return false, err
	}
	resp, err := apiClient.Post(ctx, "/sandboxes/"+s.SandboxID+"/pause", struct{}{}, nil)
	if err != nil {
		var apiErr *api.ApiError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 409 {
			return false, nil
		}
		if resp != nil && resp.StatusCode == 409 {
			return false, nil
		}
		return false, wrapSandboxNotFoundError(s.SandboxID, err)
	}
	return true, nil
}

// BetaPause is deprecated. Use Pause instead.
func (s *Sandbox) BetaPause(ctx context.Context, opts *ConnectionOpts) (bool, error) {
	return s.Pause(ctx, opts)
}

// CreateSnapshot creates a snapshot of the sandbox.
func (s *Sandbox) CreateSnapshot(ctx context.Context, opts *CreateSnapshotOpts) (*SnapshotInfo, error) {
	var signal context.Context
	var apiOpts *SandboxApiOpts
	if opts != nil {
		apiOpts = &opts.SandboxApiOpts
		signal = opts.Signal
	}
	ctx, cancel := shared.MergeContexts(ctx, signal)
	defer cancel()
	connConfig := s.resolveSandboxApiConnectionConfig(apiOpts)
	apiClient, err := api.NewApiClient(toClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}
	var snapshot api.SnapshotInfo
	body := struct {
		Name string `json:"name,omitempty"`
	}{}
	if opts != nil {
		body.Name = opts.Name
	}
	_, err = apiClient.Post(ctx, "/sandboxes/"+s.SandboxID+"/snapshots", body, &snapshot)
	if err != nil {
		return nil, wrapSandboxNotFoundError(s.SandboxID, err)
	}
	if err := ensureSnapshotResponseData(&snapshot); err != nil {
		return nil, err
	}
	info := snapshotInfoFromAPI(snapshot)
	return &info, nil
}

// ListSnapshots returns a paginator for listing snapshots of this sandbox.
func (s *Sandbox) ListSnapshots(opts *struct {
	SandboxApiOpts
	Limit     int
	NextToken string
}) *SnapshotPaginator {
	var apiOpts *SandboxApiOpts
	limit := 0
	initialToken := ""
	if opts != nil {
		apiOpts = &opts.SandboxApiOpts
		limit = opts.Limit
		initialToken = opts.NextToken
	}

	sandboxID := s.SandboxID

	fetchPage := func(ctx context.Context, nextToken string, override *SandboxApiOpts) ([]SnapshotInfo, string, error) {
		var effectiveOpts *SandboxApiOpts
		switch {
		case override != nil && apiOpts != nil:
			merged := mergeSandboxApiOpts(*apiOpts, *override)
			effectiveOpts = &merged
		case override != nil:
			copied := *override
			effectiveOpts = &copied
		default:
			effectiveOpts = apiOpts
		}
		ctx, cancel := mergeSandboxApiSignal(ctx, effectiveOpts)
		defer cancel()

		connConfig := s.resolveSandboxApiConnectionConfig(effectiveOpts)
		apiClient, err := api.NewApiClient(toClientConfig(connConfig), api.WithRequireApiKey())
		if err != nil {
			return nil, "", err
		}

		path := "/snapshots"
		params := url.Values{}
		if sandboxID != "" {
			params.Set("sandboxID", sandboxID)
		}
		if limit > 0 {
			params.Set("limit", strconv.Itoa(limit))
		}
		if nextToken != "" {
			params.Set("nextToken", nextToken)
		}
		if q := params.Encode(); q != "" {
			path += "?" + q
		}

		var snapshots []api.SnapshotInfo
		resp, err := apiClient.Get(ctx, path, &snapshots)
		if err != nil {
			return nil, "", err
		}

		next := ""
		if resp != nil {
			next = resp.Header.Get("x-next-token")
		}

		infos := make([]SnapshotInfo, len(snapshots))
		for i, s := range snapshots {
			infos[i] = snapshotInfoFromAPI(s)
		}
		return infos, next, nil
	}

	return &SnapshotPaginator{
		paginator: newPaginatorWithInitialToken(func(ctx context.Context, nextToken string) ([]SnapshotInfo, string, error) {
			return fetchPage(ctx, nextToken, nil)
		}, initialToken),
		fetchWithOpts: fetchPage,
	}
}

// GetMcpUrl returns the MCP endpoint URL for this sandbox.
func (s *Sandbox) GetMcpUrl() string {
	return fmt.Sprintf("https://%s/mcp", s.connectionConfig.GetHost(s.SandboxID, s.mcpPort, s.SandboxDomain))
}

// GetMcpToken retrieves or returns the cached MCP authentication token.
func (s *Sandbox) GetMcpToken() (string, error) {
	if s.mcpToken != "" {
		return s.mcpToken, nil
	}

	tokenValue, err := s.Files.Read(context.Background(), "/etc/mcp-gateway/.token", &filesystem.FilesystemReadOpts{
		FilesystemRequestOpts: filesystem.FilesystemRequestOpts{
			User: "root",
		},
	})
	if err != nil {
		return "", err
	}
	token, ok := tokenValue.(string)
	if !ok {
		return "", fmt.Errorf("expected MCP token read to return string, got %T", tokenValue)
	}

	s.mcpToken = token
	return s.mcpToken, nil
}

// UploadUrl generates a signed URL for uploading a file to the sandbox.
func (s *Sandbox) UploadUrl(path string, opts *struct {
	UseSignatureExpiration *int
	User                   string
}) (string, error) {
	useSignature := s.envdAccessToken != ""
	if !useSignature && opts != nil && opts.UseSignatureExpiration != nil {
		return "", fmt.Errorf("Signature expiration can be used only when sandbox is created as secured.")
	}

	username := ""
	if opts != nil {
		username = opts.User
	}
	username = s.resolveSandboxURLUser(username)
	fileURL := s.fileURL(path, username)
	if !useSignature {
		return fileURL, nil
	}

	expirationInSeconds := 0
	if opts != nil && opts.UseSignatureExpiration != nil {
		expirationInSeconds = *opts.UseSignatureExpiration
	}

	signature, expiration, err := GetSignature(path, "write", username, expirationInSeconds, s.envdAccessToken)
	if err != nil {
		return "", err
	}

	parsed, err := url.Parse(fileURL)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	values.Set("signature", signature)
	if expiration != nil {
		values.Set("signature_expiration", strconv.FormatInt(*expiration, 10))
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

// DownloadUrl generates a signed URL for downloading a file from the sandbox.
func (s *Sandbox) DownloadUrl(path string, opts *struct {
	UseSignatureExpiration *int
	User                   string
}) (string, error) {
	useSignature := s.envdAccessToken != ""
	if !useSignature && opts != nil && opts.UseSignatureExpiration != nil {
		return "", fmt.Errorf("Signature expiration can be used only when sandbox is created as secured.")
	}

	username := ""
	if opts != nil {
		username = opts.User
	}
	username = s.resolveSandboxURLUser(username)
	fileURL := s.fileURL(path, username)
	if !useSignature {
		return fileURL, nil
	}

	expirationInSeconds := 0
	if opts != nil && opts.UseSignatureExpiration != nil {
		expirationInSeconds = *opts.UseSignatureExpiration
	}

	signature, expiration, err := GetSignature(path, "read", username, expirationInSeconds, s.envdAccessToken)
	if err != nil {
		return "", err
	}

	parsed, err := url.Parse(fileURL)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	values.Set("signature", signature)
	if expiration != nil {
		values.Set("signature_expiration", strconv.FormatInt(*expiration, 10))
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

// GetInfo retrieves information about this sandbox from the API.
func (s *Sandbox) GetInfo(ctx context.Context, opts *struct {
	RequestTimeoutMs *int
	Signal           context.Context
}) (*SandboxInfo, error) {
	var signal context.Context
	var requestTimeoutMs *int
	if opts != nil {
		requestTimeoutMs = opts.RequestTimeoutMs
		signal = opts.Signal
	}
	ctx, cancel := shared.MergeContexts(ctx, signal)
	defer cancel()
	connConfig := s.resolveSandboxRequestTimeoutConnectionConfig(requestTimeoutMs)
	apiClient, err := api.NewApiClient(toClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	sandboxResp, err := getSandboxResponse(ctx, apiClient, s.SandboxID)
	if err != nil {
		return nil, wrapSandboxNotFoundError(s.SandboxID, err)
	}

	info := sandboxResponseToInfo(sandboxResp)
	return &info, nil
}

// GetMetrics retrieves resource usage metrics for this sandbox.
func (s *Sandbox) GetMetrics(ctx context.Context, opts *SandboxMetricsOpts) ([]SandboxMetrics, error) {
	if opts == nil {
		opts = &SandboxMetricsOpts{}
	}
	ctx, cancel := shared.MergeContexts(ctx, opts.Signal)
	defer cancel()
	envdVersion := s.envdVersion
	if envdVersion == "" && s.envdApi != nil {
		envdVersion = s.envdApi.Version
	}
	if envdVersion != "" {
		if !sandboxVersionGTE(envdVersion, "0.1.5") {
			return nil, &SandboxError{Message: "You need to update the template to use the new SDK. You can do this by running `e2b template build` in the directory with the template."}
		}
		if !sandboxVersionGTE(envdVersion, "0.2.4") && s.connectionConfig != nil && s.connectionConfig.Logger != nil {
			s.connectionConfig.Logger.Warn("Disk metrics are not supported in this version of the sandbox, please rebuild the template to get disk metrics.")
		}
	}

	connConfig := s.resolveSandboxApiConnectionConfig(&opts.SandboxApiOpts)
	apiClient, err := api.NewApiClient(toClientConfig(connConfig), api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	path := "/sandboxes/" + s.SandboxID + "/metrics"
	params := url.Values{}
	if opts.Start != nil {
		params.Set("start", strconv.FormatInt(roundUnixSeconds(*opts.Start), 10))
	}
	if opts.End != nil {
		params.Set("end", strconv.FormatInt(roundUnixSeconds(*opts.End), 10))
	}
	if q := params.Encode(); q != "" {
		path += "?" + q
	}

	var metricsResp api.SandboxMetricsList
	_, err = apiClient.Get(ctx, path, &metricsResp)
	if err != nil {
		return nil, err
	}

	metrics := make([]SandboxMetrics, len(metricsResp))
	for i, m := range metricsResp {
		timestamp := m.Timestamp
		if timestamp.IsZero() && m.TimestampUnix != 0 {
			timestamp = time.Unix(m.TimestampUnix, 0)
		}
		memUsed := resolveMetricValue(m.MemUsed, m.MemUsedMiB)
		memTotal := resolveMetricValue(m.MemTotal, m.MemTotalMiB)
		diskUsed := resolveMetricValue(m.DiskUsed, m.DiskUsedMiB)
		diskTotal := resolveMetricValue(m.DiskTotal, m.DiskTotalMiB)
		metrics[i] = SandboxMetrics{
			Timestamp:  timestamp,
			CpuUsedPct: m.CpuUsedPct,
			CpuCount:   m.CpuCount,
			MemUsed:    memUsed,
			MemTotal:   memTotal,
			DiskUsed:   diskUsed,
			DiskTotal:  diskTotal,
		}
	}
	return metrics, nil
}

func wrapSandboxNotFoundError(sandboxID string, err error) error {
	if err == nil {
		return nil
	}

	var nfe *api.NotFoundError
	if errors.As(err, &nfe) {
		return &SandboxNotFoundError{NotFoundError: NotFoundError{SandboxError: SandboxError{Message: fmt.Sprintf("Sandbox %s not found", sandboxID)}}}
	}

	return err
}

func wrapPausedSandboxNotFoundError(sandboxID string, err error) error {
	if err == nil {
		return nil
	}

	var nfe *api.NotFoundError
	if errors.As(err, &nfe) {
		return &SandboxNotFoundError{NotFoundError: NotFoundError{SandboxError: SandboxError{Message: fmt.Sprintf("Paused sandbox %s not found", sandboxID)}}}
	}

	return err
}

// --- internal helpers ---

func newSandboxFromResponse(resp *api.SandboxResponse, connConfig *ConnectionConfig) *Sandbox {
	sandboxDomain := resolveSandboxDomain(resp)
	if sandboxDomain == "" {
		sandboxDomain = connConfig.Domain
	}

	envdApiUrl := connConfig.GetSandboxUrl(resp.SandboxID, sandboxDomain, envdPort)
	envdDirectUrl := connConfig.GetSandboxDirectUrl(resp.SandboxID, sandboxDomain, envdPort)
	sandboxHeaders := sandboxTransportHeaders(resp.SandboxID, envdPort, resp.EnvdAccessToken, connConfig.Headers)
	envdHeaders := sandboxHTTPHeaders(resp.SandboxID, envdPort, resp.EnvdAccessToken, connConfig.Headers)

	envdApiClient := envd.NewEnvdApiClient(
		envdApiUrl,
		resp.EnvdAccessToken,
		envdHeaders,
		connConfig.RequestTimeoutMs,
	)
	envdApiClient.HttpClient = shared.NewEnvdRESTHTTPClient(time.Duration(connConfig.RequestTimeoutMs)*time.Millisecond, connConfig.Proxy, connConfig.Logger)

	cmdConnConfig := &struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
		Logger           Logger
		Proxy            string
	}{
		ApiKey:           connConfig.ApiKey,
		AccessToken:      connConfig.AccessToken,
		Domain:           connConfig.Domain,
		ApiUrl:           connConfig.ApiUrl,
		SandboxUrl:       envdApiUrl,
		Debug:            connConfig.Debug,
		RequestTimeoutMs: connConfig.RequestTimeoutMs,
		Headers:          sandboxHeaders,
		Logger:           connConfig.Logger,
		Proxy:            connConfig.Proxy,
	}

	fsConnConfig := &struct {
		ApiKey           string
		AccessToken      string
		Domain           string
		ApiUrl           string
		SandboxUrl       string
		Debug            bool
		RequestTimeoutMs int
		Headers          map[string]string
		Logger           Logger
		Proxy            string
	}{
		ApiKey:           connConfig.ApiKey,
		AccessToken:      connConfig.AccessToken,
		Domain:           connConfig.Domain,
		ApiUrl:           connConfig.ApiUrl,
		SandboxUrl:       envdApiUrl,
		Debug:            connConfig.Debug,
		RequestTimeoutMs: connConfig.RequestTimeoutMs,
		Headers:          sandboxHeaders,
		Logger:           connConfig.Logger,
		Proxy:            connConfig.Proxy,
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
		envdPort:           envdPort,
		mcpPort:            mcpPort,
		connectionConfig:   connConfig,
		envdVersion:        envdVersion,
		envdAccessToken:    resp.EnvdAccessToken,
		envdApiUrl:         envdApiUrl,
		envdDirectUrl:      envdDirectUrl,
		envdApi:            envdApiClient,
		Files:              fs,
		Commands:           cmds,
		Pty:                pty,
		Git:                g,
	}
}

func sandboxTransportHeaders(sandboxID string, port int, accessToken string, base map[string]string) map[string]string {
	headers := map[string]string{}
	for k, v := range base {
		headers[k] = v
	}
	headers["E2b-Sandbox-Id"] = sandboxID
	headers["E2b-Sandbox-Port"] = strconv.Itoa(port)
	if accessToken != "" {
		headers["X-Access-Token"] = accessToken
	}
	return headers
}

func sandboxHTTPHeaders(sandboxID string, port int, accessToken string, base map[string]string) map[string]string {
	headers := map[string]string{}
	if userAgent := base["User-Agent"]; userAgent != "" {
		headers["User-Agent"] = userAgent
	}
	headers["E2b-Sandbox-Id"] = sandboxID
	headers["E2b-Sandbox-Port"] = strconv.Itoa(port)
	if accessToken != "" {
		headers["X-Access-Token"] = accessToken
	}
	return headers
}

func newDebugSandbox(connConfig *ConnectionConfig) *Sandbox {
	resp := &api.SandboxResponse{
		SandboxID:     "debug_sandbox_id",
		EnvdVersion:   envd.EnvdDebugFallback,
		SandboxDomain: connConfig.Domain,
	}
	return newSandboxFromResponse(resp, connConfig)
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
	if opts.SandboxUrl != "" {
		merged.SandboxUrl = opts.SandboxUrl
	}
	if opts.Debug != nil {
		merged.Debug = *opts.Debug
	}
	if opts.RequestTimeoutMs != nil {
		merged.RequestTimeoutMs = *opts.RequestTimeoutMs
	}
	if opts.Logger != nil {
		merged.Logger = opts.Logger
	}
	merged.Headers = mergeHeaders(merged.Headers, opts.Headers)
	if opts.Proxy != "" {
		merged.Proxy = opts.Proxy
	}
	return &merged
}

func (s *Sandbox) resolveSandboxApiConnectionConfig(opts *SandboxApiOpts) *ConnectionConfig {
	if opts == nil {
		return s.connectionConfig
	}

	merged := *s.connectionConfig
	if opts.ApiKey != "" {
		merged.ApiKey = opts.ApiKey
	}
	if opts.Domain != "" {
		merged.Domain = opts.Domain
	}
	if opts.Debug != nil {
		merged.Debug = *opts.Debug
	}
	if opts.apiUrl != "" {
		merged.ApiUrl = opts.apiUrl
	}
	if opts.RequestTimeoutMs != nil {
		merged.RequestTimeoutMs = *opts.RequestTimeoutMs
	}
	merged.Headers = mergeHeaders(merged.Headers, opts.Headers)
	if opts.Proxy != "" {
		merged.Proxy = opts.Proxy
	}
	return &merged
}

func (s *Sandbox) resolveSandboxRequestTimeoutConnectionConfig(requestTimeoutMs *int) *ConnectionConfig {
	if requestTimeoutMs == nil {
		return s.connectionConfig
	}

	merged := *s.connectionConfig
	merged.RequestTimeoutMs = *requestTimeoutMs
	return &merged
}

func (s *Sandbox) resolveSandboxURLUser(user string) string {
	if user != "" {
		return user
	}
	if sandboxVersionGTE(s.envdVersion, envd.EnvdDefaultUser) {
		return ""
	}
	return defaultUsername
}

func (s *Sandbox) fileURL(path string, username string) string {
	baseURL := s.envdDirectUrl
	if baseURL == "" {
		baseURL = s.envdApiUrl
	}

	fileURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Sprintf("%s/files", baseURL)
	}
	fileURL.Path = "/files"
	fileURL.RawQuery = sandboxFileQuery(path, username)
	return fileURL.String()
}

func sandboxFileQuery(path string, username string) string {
	parts := make([]string, 0, 2)
	if username != "" {
		parts = append(parts, "username="+url.QueryEscape(username))
	}
	if path != "" {
		parts = append(parts, "path="+url.QueryEscape(path))
	}
	return strings.Join(parts, "&")
}

func mergeHeaders(base map[string]string, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}

	headers := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		headers[k] = v
	}
	for k, v := range override {
		headers[k] = v
	}
	return headers
}

func sandboxVersionGTE(version, minVersion string) bool {
	if version == "" {
		return true
	}

	var major1, minor1, patch1 int
	var major2, minor2, patch2 int
	fmt.Sscanf(version, "%d.%d.%d", &major1, &minor1, &patch1)
	fmt.Sscanf(minVersion, "%d.%d.%d", &major2, &minor2, &patch2)

	if major1 != major2 {
		return major1 > major2
	}
	if minor1 != minor2 {
		return minor1 > minor2
	}
	return patch1 >= patch2
}

func sandboxResponseToInfo(s *api.SandboxResponse) SandboxInfo {
	info := SandboxInfo{
		SandboxID:           s.SandboxID,
		TemplateID:          s.TemplateID,
		Name:                s.Alias,
		Metadata:            map[string]string{},
		StartedAt:           s.StartedAt,
		EndAt:               s.EndAt,
		State:               SandboxState(s.State),
		CpuCount:            s.CpuCount,
		MemoryMB:            s.MemoryMB,
		EnvdVersion:         s.EnvdVersion,
		AllowInternetAccess: s.AllowInternetAccess,
		VolumeMounts: []struct {
			Name string
			Path string
		}{},
	}
	for k, v := range s.Metadata {
		info.Metadata[k] = v
	}
	if s.Network != nil {
		var allowPublicTraffic *bool
		if s.Network.AllowPublicTraffic != nil {
			allowPublicTraffic = boolRef(*s.Network.AllowPublicTraffic)
		}
		info.Network = &SandboxNetworkInfo{
			AllowOut:           s.Network.AllowOut,
			DenyOut:            s.Network.DenyOut,
			Rules:              networkRulesFromAPI(s.Network.Rules),
			AllowPublicTraffic: allowPublicTraffic,
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
		info.VolumeMounts = make([]struct {
			Name string
			Path string
		}, len(s.VolumeMounts))
		for i, m := range s.VolumeMounts {
			name := m.Name
			if name == "" {
				name = m.VolumeID
			}
			path := m.Path
			if path == "" {
				path = m.MountPath
			}
			info.VolumeMounts[i] = struct {
				Name string
				Path string
			}{
				Name: name,
				Path: path,
			}
		}
	}
	return info
}
