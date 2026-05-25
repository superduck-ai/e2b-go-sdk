package e2b

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/e2b-dev/e2b-go-sdk/api"
)

type SandboxState string

const (
	SandboxStateRunning SandboxState = "running"
	SandboxStatePaused  SandboxState = "paused"
)

type SandboxInfo struct {
	SandboxID    string
	TemplateID   string
	Name         string
	Metadata     map[string]string
	StartedAt    time.Time
	EndAt        time.Time
	State        SandboxState
	CpuCount     int
	MemoryMB     int
	EnvdVersion  string
	Network      *SandboxNetworkOpts
	Lifecycle    *SandboxInfoLifecycle
	VolumeMounts []VolumeMountInfo
}

type SandboxNetworkOpts struct {
	AllowOut           []string
	DenyOut            []string
	AllowPublicTraffic bool
	MaskRequestHost    bool
}

type SandboxLifecycle struct {
	OnTimeout  string // "kill" or "pause"
	AutoResume bool
}

type SandboxInfoLifecycle struct {
	OnTimeout  string
	AutoResume bool
}

type VolumeMountInfo struct {
	VolumeID  string
	MountPath string
}

type SandboxMetrics struct {
	Timestamp    time.Time
	CpuUsedPct   float64
	CpuCount     int
	MemUsedMiB   int64
	MemTotalMiB  int64
	DiskUsedMiB  int64
	DiskTotalMiB int64
}

type SnapshotInfo struct {
	SnapshotID string
}

type SandboxOpts struct {
	ConnectionOpts
	Template            string
	Metadata            map[string]string
	Envs                map[string]string
	TimeoutMs           int
	Secure              bool
	AllowInternetAccess *bool
	Network             *SandboxNetworkOpts
	Lifecycle           *SandboxLifecycle
	VolumeMounts        []VolumeMountInfo
}

type SandboxConnectOpts struct {
	ConnectionOpts
	TimeoutMs int
}

type SandboxListOpts struct {
	ConnectionOpts
	State     SandboxState
	Metadata  map[string]string
	Limit     int
	NextToken string
}

type SandboxMetricsOpts struct {
	ConnectionOpts
	Start *time.Time
	End   *time.Time
}

type SnapshotListOpts struct {
	ConnectionOpts
	SandboxID string
	Limit     int
	NextToken string
}

// SandboxConnectionInfo holds connection details returned when creating/connecting a sandbox.
type SandboxConnectionInfo struct {
	SandboxID          string
	TemplateID         string
	Name               string
	Metadata           map[string]string
	StartedAt          time.Time
	EndAt              time.Time
	State              SandboxState
	CpuCount           int
	MemoryMB           int
	EnvdVersion        string
	SandboxDomain      string
	EnvdAccessToken    string
	TrafficAccessToken string
}

// SandboxApi provides static sandbox management methods.
type SandboxApi struct{}

func (a *SandboxApi) newClient(opts *ConnectionOpts) (*api.ApiClient, error) {
	config := NewConnectionConfig(opts)
	return api.NewApiClient(ToClientConfig(config), api.WithRequireApiKey())
}

// ToClientConfig converts a ConnectionConfig to an api.ClientConfig.
func ToClientConfig(c *ConnectionConfig) *api.ClientConfig {
	return &api.ClientConfig{
		ApiKey:           c.ApiKey,
		AccessToken:      c.AccessToken,
		Domain:           c.Domain,
		ApiUrl:           c.ApiUrl,
		RequestTimeoutMs: c.RequestTimeoutMs,
		Headers:          c.Headers,
		Logger:           c.Logger,
	}
}

func (a *SandboxApi) KillSandbox(ctx context.Context, sandboxId string, opts *ConnectionOpts) (bool, error) {
	if opts == nil {
		opts = &ConnectionOpts{}
	}
	client, err := a.newClient(opts)
	if err != nil {
		return false, err
	}

	_, err = client.Delete(ctx, fmt.Sprintf("/sandboxes/%s", sandboxId), nil)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *SandboxApi) GetInfo(ctx context.Context, sandboxId string, opts *ConnectionOpts) (*SandboxInfo, error) {
	if opts == nil {
		opts = &ConnectionOpts{}
	}
	client, err := a.newClient(opts)
	if err != nil {
		return nil, err
	}

	var resp api.SandboxResponse
	_, err = client.Get(ctx, fmt.Sprintf("/sandboxes/%s", sandboxId), &resp)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return nil, &SandboxNotFoundError{NotFoundError{SandboxError{Message: fmt.Sprintf("Sandbox %s not found", sandboxId)}}}
		}
		return nil, err
	}

	info := sandboxResponseToInfo(&resp)
	return &info, nil
}

func (a *SandboxApi) GetMetrics(ctx context.Context, sandboxId string, opts *SandboxMetricsOpts) ([]SandboxMetrics, error) {
	if opts == nil {
		opts = &SandboxMetricsOpts{}
	}
	client, err := a.newClient(&opts.ConnectionOpts)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	if opts.Start != nil {
		params.Set("start", strconv.FormatInt(opts.Start.Unix(), 10))
	}
	if opts.End != nil {
		params.Set("end", strconv.FormatInt(opts.End.Unix(), 10))
	}

	path := fmt.Sprintf("/sandboxes/%s/metrics", sandboxId)
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp []api.SandboxMetrics
	_, err = client.Get(ctx, path, &resp)
	if err != nil {
		return nil, err
	}

	metrics := make([]SandboxMetrics, len(resp))
	for i, m := range resp {
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

func (a *SandboxApi) SetTimeout(ctx context.Context, sandboxId string, timeoutMs int, opts *ConnectionOpts) error {
	if opts == nil {
		opts = &ConnectionOpts{}
	}
	client, err := a.newClient(opts)
	if err != nil {
		return err
	}

	timeoutSec := int(math.Ceil(float64(timeoutMs) / 1000.0))
	body := api.SetTimeoutRequest{Timeout: timeoutSec}

	_, err = client.Post(ctx, fmt.Sprintf("/sandboxes/%s/timeout", sandboxId), body, nil)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return &SandboxNotFoundError{NotFoundError{SandboxError{Message: fmt.Sprintf("Sandbox %s not found", sandboxId)}}}
		}
		return err
	}
	return nil
}

func (a *SandboxApi) Pause(ctx context.Context, sandboxId string, opts *ConnectionOpts) (bool, error) {
	if opts == nil {
		opts = &ConnectionOpts{}
	}
	client, err := a.newClient(opts)
	if err != nil {
		return false, err
	}

	resp, err := client.Post(ctx, fmt.Sprintf("/sandboxes/%s/pause", sandboxId), struct{}{}, nil)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return false, &SandboxNotFoundError{NotFoundError{SandboxError{Message: fmt.Sprintf("Sandbox %s not found", sandboxId)}}}
		}
		// Check for 409 Conflict (already paused)
		if resp != nil && resp.StatusCode == http.StatusConflict {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *SandboxApi) CreateSnapshot(ctx context.Context, sandboxId string, opts *ConnectionOpts) (*SnapshotInfo, error) {
	if opts == nil {
		opts = &ConnectionOpts{}
	}
	client, err := a.newClient(opts)
	if err != nil {
		return nil, err
	}

	var resp api.SnapshotInfo
	_, err = client.Post(ctx, fmt.Sprintf("/sandboxes/%s/snapshots", sandboxId), struct{}{}, &resp)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return nil, &SandboxNotFoundError{NotFoundError{SandboxError{Message: fmt.Sprintf("Sandbox %s not found", sandboxId)}}}
		}
		return nil, err
	}

	return &SnapshotInfo{SnapshotID: resp.SnapshotID}, nil
}

func (a *SandboxApi) ListSnapshots(ctx context.Context, opts *SnapshotListOpts) *Paginator[SnapshotInfo] {
	if opts == nil {
		opts = &SnapshotListOpts{}
	}
	connOpts := opts.ConnectionOpts
	sandboxID := opts.SandboxID
	limit := opts.Limit
	initialToken := opts.NextToken

	return NewPaginatorWithInitialToken(func(ctx context.Context, nextToken string) ([]SnapshotInfo, string, error) {
		client, err := a.newClient(&connOpts)
		if err != nil {
			return nil, "", err
		}

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

		path := "/snapshots"
		if len(params) > 0 {
			path += "?" + params.Encode()
		}

		var items []api.SnapshotInfo
		resp, err := client.Get(ctx, path, &items)
		if err != nil {
			return nil, "", err
		}

		token := ""
		if resp != nil {
			token = resp.Header.Get("x-next-token")
		}

		result := make([]SnapshotInfo, len(items))
		for i, s := range items {
			result[i] = SnapshotInfo{SnapshotID: s.SnapshotID}
		}
		return result, token, nil
	}, initialToken)
}

func (a *SandboxApi) DeleteSnapshot(ctx context.Context, snapshotId string, opts *ConnectionOpts) (bool, error) {
	if opts == nil {
		opts = &ConnectionOpts{}
	}
	client, err := a.newClient(opts)
	if err != nil {
		return false, err
	}

	_, err = client.Delete(ctx, fmt.Sprintf("/templates/%s", snapshotId), nil)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *SandboxApi) CreateSandbox(ctx context.Context, opts *SandboxOpts) (*SandboxConnectionInfo, error) {
	if opts == nil {
		opts = &SandboxOpts{}
	}
	client, err := a.newClient(&opts.ConnectionOpts)
	if err != nil {
		return nil, err
	}

	timeoutSec := 300 // default 5 minutes
	if opts.TimeoutMs > 0 {
		timeoutSec = int(math.Ceil(float64(opts.TimeoutMs) / 1000.0))
	}

	reqBody := api.CreateSandboxRequest{
		TemplateID:          opts.Template,
		Timeout:             timeoutSec,
		Metadata:            opts.Metadata,
		EnvVars:             opts.Envs,
		Secure:              opts.Secure,
		AllowInternetAccess: opts.AllowInternetAccess,
	}
	if opts.Network != nil {
		reqBody.Network = &api.NetworkOpts{
			AllowOut:           opts.Network.AllowOut,
			DenyOut:            opts.Network.DenyOut,
			AllowPublicTraffic: opts.Network.AllowPublicTraffic,
			MaskRequestHost:    opts.Network.MaskRequestHost,
		}
	}
	if opts.Lifecycle != nil {
		reqBody.Lifecycle = &api.LifecycleOpts{
			OnTimeout:  opts.Lifecycle.OnTimeout,
			AutoResume: opts.Lifecycle.AutoResume,
		}
	}
	if len(opts.VolumeMounts) > 0 {
		reqBody.VolumeMounts = make([]api.VolumeMount, len(opts.VolumeMounts))
		for i, v := range opts.VolumeMounts {
			reqBody.VolumeMounts[i] = api.VolumeMount{VolumeID: v.VolumeID, MountPath: v.MountPath}
		}
	}

	var resp api.SandboxResponse
	_, err = client.Post(ctx, "/sandboxes", reqBody, &resp)
	if err != nil {
		return nil, err
	}

	return sandboxRespToConnectionInfo(&resp), nil
}

func (a *SandboxApi) ConnectSandbox(ctx context.Context, sandboxId string, opts *SandboxConnectOpts) (*SandboxConnectionInfo, error) {
	if opts == nil {
		opts = &SandboxConnectOpts{}
	}
	client, err := a.newClient(&opts.ConnectionOpts)
	if err != nil {
		return nil, err
	}

	body := struct {
		Timeout int `json:"timeout,omitempty"`
	}{}
	timeoutMs := opts.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = DefaultSandboxTimeoutMs
	}
	body.Timeout = int(math.Ceil(float64(timeoutMs) / 1000.0))

	var resp api.SandboxResponse
	_, err = client.Post(ctx, fmt.Sprintf("/sandboxes/%s/connect", sandboxId), body, &resp)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return nil, &SandboxNotFoundError{NotFoundError{SandboxError{Message: fmt.Sprintf("Sandbox %s not found", sandboxId)}}}
		}
		return nil, err
	}

	return sandboxRespToConnectionInfo(&resp), nil
}

func (a *SandboxApi) ListSandboxes(ctx context.Context, opts *SandboxListOpts) *Paginator[SandboxInfo] {
	if opts == nil {
		opts = &SandboxListOpts{}
	}
	connOpts := opts.ConnectionOpts
	state := opts.State
	metadata := opts.Metadata
	limit := opts.Limit
	initialToken := opts.NextToken

	return NewPaginatorWithInitialToken(func(ctx context.Context, nextToken string) ([]SandboxInfo, string, error) {
		client, err := a.newClient(&connOpts)
		if err != nil {
			return nil, "", err
		}

		params := url.Values{}
		for k, v := range metadata {
			params.Add("metadata", fmt.Sprintf("%s=%s", k, v))
		}
		if state != "" {
			params.Set("state", string(state))
		}
		if limit > 0 {
			params.Set("limit", strconv.Itoa(limit))
		}
		if nextToken != "" {
			params.Set("nextToken", nextToken)
		}

		path := "/v2/sandboxes"
		if len(params) > 0 {
			path += "?" + params.Encode()
		}

		var items []api.SandboxResponse
		resp, err := client.Get(ctx, path, &items)
		if err != nil {
			return nil, "", err
		}

		token := ""
		if resp != nil {
			token = resp.Header.Get("x-next-token")
		}

		result := make([]SandboxInfo, len(items))
		for i, s := range items {
			result[i] = sandboxResponseToInfo(&s)
		}
		return result, token, nil
	}, initialToken)
}

// sandboxResponseToConnectionInfo converts an API response to connection info.
func sandboxRespToConnectionInfo(r *api.SandboxResponse) *SandboxConnectionInfo {
	return &SandboxConnectionInfo{
		SandboxID:          r.SandboxID,
		TemplateID:         r.TemplateID,
		Name:               r.Name,
		Metadata:           r.Metadata,
		StartedAt:          r.StartedAt,
		EndAt:              r.EndAt,
		State:              SandboxState(r.State),
		CpuCount:           r.CpuCount,
		MemoryMB:           r.MemoryMB,
		EnvdVersion:        r.EnvdVersion,
		SandboxDomain:      r.SandboxDomain,
		EnvdAccessToken:    r.EnvdAccessToken,
		TrafficAccessToken: r.TrafficAccessToken,
	}
}
