package e2b

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/api"
	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

const (
	defaultSandboxTemplate    = "base"
	defaultSandboxMcpTemplate = "mcp-gateway"
)

type SandboxState string

const (
	sandboxStateRunning SandboxState = "running"
	sandboxStatePaused  SandboxState = "paused"
)

type SandboxInfo struct {
	SandboxID           string
	TemplateID          string
	Name                string
	Metadata            map[string]string
	StartedAt           time.Time
	EndAt               time.Time
	State               SandboxState
	CpuCount            int
	MemoryMB            int
	EnvdVersion         string
	AllowInternetAccess *bool
	Network             *SandboxNetworkInfo
	Lifecycle           *SandboxInfoLifecycle
	VolumeMounts        []struct {
		Name string
		Path string
	}
}

type SandboxNetworkSelectorContext struct {
	AllTraffic string
	Rules      SandboxNetworkRules
}

type SandboxNetworkSelector interface{}

type SandboxNetworkSelectorFunc func(ctx SandboxNetworkSelectorContext) []string

type SandboxNetworkOpts struct {
	AllowOut           SandboxNetworkSelector
	DenyOut            SandboxNetworkSelector
	Rules              SandboxNetworkRules
	AllowPublicTraffic *bool
	MaskRequestHost    string
}

type SandboxNetworkInfo struct {
	AllowOut           []string
	DenyOut            []string
	Rules              SandboxNetworkRules
	AllowPublicTraffic *bool
	MaskRequestHost    string
}

type SandboxNetworkTransform struct {
	Headers map[string]string
}

type SandboxNetworkRule struct {
	Transform *SandboxNetworkTransform
}

type SandboxNetworkRules map[string][]SandboxNetworkRule

type SandboxNetworkUpdate struct {
	AllowOut            SandboxNetworkSelector
	DenyOut             SandboxNetworkSelector
	Rules               SandboxNetworkRules
	AllowInternetAccess *bool
}

type SandboxLifecycle struct {
	OnTimeout  string // "kill" or "pause"
	AutoResume *bool
}

type SandboxInfoLifecycle struct {
	OnTimeout  string
	AutoResume bool
}

type SandboxMetrics struct {
	Timestamp  time.Time
	CpuUsedPct float64
	CpuCount   int
	MemUsed    int64
	MemTotal   int64
	MemCache   int64
	DiskUsed   int64
	DiskTotal  int64
}

type SnapshotInfo struct {
	SnapshotID string
	Names      []string
}

func snapshotInfoFromAPI(info api.SnapshotInfo) SnapshotInfo {
	return SnapshotInfo{
		SnapshotID: info.SnapshotID,
		Names:      info.Names,
	}
}

type McpServer map[string]any

type SandboxOpts struct {
	ConnectionOpts
	Template            string
	Metadata            map[string]string
	Envs                map[string]string
	TimeoutMs           *int
	Secure              *bool
	AllowInternetAccess *bool
	Mcp                 McpServer
	Network             *SandboxNetworkOpts
	Lifecycle           *SandboxLifecycle
	VolumeMounts        map[string]any
}

type SandboxBetaCreateOpts struct {
	SandboxOpts
	AutoPause bool
}

type SandboxConnectOpts struct {
	ConnectionOpts
	TimeoutMs *int
}

type SandboxApiOpts struct {
	ApiKey           string
	Domain           string
	Debug            *bool
	Signal           context.Context
	RequestTimeoutMs *int
	Headers          map[string]string
	Proxy            string
	apiUrl           string
}

type SandboxListOpts struct {
	ApiKey           string
	Domain           string
	Debug            *bool
	RequestTimeoutMs *int
	Headers          map[string]string
	Proxy            string
	apiUrl           string
	Query            *struct {
		Metadata map[string]string
		State    []SandboxState
	}
	Limit     int
	NextToken string
}

type SandboxMetricsOpts struct {
	SandboxApiOpts
	Start *time.Time
	End   *time.Time
}

type SnapshotListOpts struct {
	ApiKey           string
	Domain           string
	Debug            *bool
	RequestTimeoutMs *int
	Headers          map[string]string
	Proxy            string
	apiUrl           string
	SandboxID        string
	Limit            int
	NextToken        string
}

type CreateSnapshotOpts struct {
	SandboxApiOpts
	Name string
}

// sandboxConnectionInfo holds connection details returned when creating/connecting a sandbox.
type sandboxConnectionInfo struct {
	SandboxID          string
	SandboxDomain      string
	EnvdVersion        string
	EnvdAccessToken    string
	TrafficAccessToken string
}

// SandboxFullInfo holds detailed sandbox info plus connection fields returned by GetFullInfo.
type SandboxFullInfo struct {
	SandboxID           string
	TemplateID          string
	Name                string
	Metadata            map[string]string
	StartedAt           time.Time
	EndAt               time.Time
	State               SandboxState
	CpuCount            int
	MemoryMB            int
	EnvdVersion         string
	AllowInternetAccess *bool
	Network             *SandboxNetworkInfo
	Lifecycle           *SandboxInfoLifecycle
	VolumeMounts        []struct {
		Name string
		Path string
	}
	SandboxDomain   string
	EnvdAccessToken string
}

type sandboxApi struct{}

type sandboxResponseEnvelope struct {
	api.SandboxResponse
	present bool
}

func (r *sandboxResponseEnvelope) UnmarshalJSON(data []byte) error {
	type sandboxResponseAlias api.SandboxResponse

	r.present = true
	return json.Unmarshal(data, (*sandboxResponseAlias)(&r.SandboxResponse))
}

func newConnectionConfigFromSandboxApiOpts(opts *SandboxApiOpts) *ConnectionConfig {
	if opts == nil {
		return NewConnectionConfig(nil)
	}

	return NewConnectionConfig(&ConnectionOpts{
		ApiKey:           opts.ApiKey,
		Domain:           opts.Domain,
		ApiUrl:           opts.apiUrl,
		Debug:            opts.Debug,
		RequestTimeoutMs: opts.RequestTimeoutMs,
		Headers:          opts.Headers,
		Proxy:            opts.Proxy,
	})
}

func sandboxApiOptsFromSandboxListOpts(opts *SandboxListOpts) SandboxApiOpts {
	if opts == nil {
		return SandboxApiOpts{}
	}
	return SandboxApiOpts{
		ApiKey:           opts.ApiKey,
		Domain:           opts.Domain,
		Debug:            opts.Debug,
		RequestTimeoutMs: opts.RequestTimeoutMs,
		Headers:          opts.Headers,
		Proxy:            opts.Proxy,
		apiUrl:           opts.apiUrl,
	}
}

func sandboxApiOptsFromSnapshotListOpts(opts *SnapshotListOpts) SandboxApiOpts {
	if opts == nil {
		return SandboxApiOpts{}
	}
	return SandboxApiOpts{
		ApiKey:           opts.ApiKey,
		Domain:           opts.Domain,
		Debug:            opts.Debug,
		RequestTimeoutMs: opts.RequestTimeoutMs,
		Headers:          opts.Headers,
		Proxy:            opts.Proxy,
		apiUrl:           opts.apiUrl,
	}
}

func mergeSandboxApiOpts(base, override SandboxApiOpts) SandboxApiOpts {
	merged := base
	if override.ApiKey != "" {
		merged.ApiKey = override.ApiKey
	}
	if override.Domain != "" {
		merged.Domain = override.Domain
	}
	if override.Debug != nil {
		merged.Debug = override.Debug
	}
	if override.RequestTimeoutMs != nil {
		merged.RequestTimeoutMs = override.RequestTimeoutMs
	}
	if override.Signal != nil {
		merged.Signal = override.Signal
	}
	if override.Headers != nil {
		headers := make(map[string]string, len(base.Headers)+len(override.Headers))
		for k, v := range base.Headers {
			headers[k] = v
		}
		for k, v := range override.Headers {
			headers[k] = v
		}
		merged.Headers = headers
	}
	if override.Proxy != "" {
		merged.Proxy = override.Proxy
	}
	if override.apiUrl != "" {
		merged.apiUrl = override.apiUrl
	}
	return merged
}

func mergeSandboxApiSignal(ctx context.Context, opts *SandboxApiOpts) (context.Context, context.CancelFunc) {
	if opts == nil {
		return ctx, func() {}
	}
	return shared.MergeContexts(ctx, opts.Signal)
}

func (a *sandboxApi) newClient(opts *SandboxApiOpts) (*api.ApiClient, error) {
	config := newConnectionConfigFromSandboxApiOpts(opts)
	return api.NewApiClient(toClientConfig(config), api.WithRequireApiKey())
}

// toClientConfig converts a ConnectionConfig to an api.ClientConfig.
func toClientConfig(c *ConnectionConfig) *api.ClientConfig {
	return &api.ClientConfig{
		ApiKey:           c.ApiKey,
		AccessToken:      c.AccessToken,
		Domain:           c.Domain,
		ApiUrl:           c.ApiUrl,
		RequestTimeoutMs: c.RequestTimeoutMs,
		Headers:          c.Headers,
		Logger:           c.Logger,
		Proxy:            c.Proxy,
	}
}

func Kill(ctx context.Context, sandboxId string, opts *SandboxApiOpts) (bool, error) {
	if opts != nil {
		var cancel context.CancelFunc
		ctx, cancel = shared.MergeContexts(ctx, opts.Signal)
		defer cancel()
	}
	return (&sandboxApi{}).Kill(ctx, sandboxId, opts)
}

func GetInfo(ctx context.Context, sandboxId string, opts *SandboxApiOpts) (*SandboxInfo, error) {
	if opts != nil {
		var cancel context.CancelFunc
		ctx, cancel = shared.MergeContexts(ctx, opts.Signal)
		defer cancel()
	}
	return (&sandboxApi{}).GetInfo(ctx, sandboxId, opts)
}

func GetFullInfo(ctx context.Context, sandboxId string, opts *SandboxApiOpts) (*SandboxFullInfo, error) {
	if opts != nil {
		var cancel context.CancelFunc
		ctx, cancel = shared.MergeContexts(ctx, opts.Signal)
		defer cancel()
	}
	return (&sandboxApi{}).getFullInfo(ctx, sandboxId, opts)
}

func GetMetrics(ctx context.Context, sandboxId string, opts *SandboxMetricsOpts) ([]SandboxMetrics, error) {
	if opts != nil {
		var cancel context.CancelFunc
		ctx, cancel = shared.MergeContexts(ctx, opts.Signal)
		defer cancel()
	}
	return (&sandboxApi{}).GetMetrics(ctx, sandboxId, opts)
}

func SetTimeout(ctx context.Context, sandboxId string, timeoutMs int, opts *SandboxApiOpts) error {
	if opts != nil {
		var cancel context.CancelFunc
		ctx, cancel = shared.MergeContexts(ctx, opts.Signal)
		defer cancel()
	}
	return (&sandboxApi{}).SetTimeout(ctx, sandboxId, timeoutMs, opts)
}

func UpdateNetwork(ctx context.Context, sandboxId string, network SandboxNetworkUpdate, opts *SandboxApiOpts) error {
	if opts != nil {
		var cancel context.CancelFunc
		ctx, cancel = shared.MergeContexts(ctx, opts.Signal)
		defer cancel()
	}
	return (&sandboxApi{}).UpdateNetwork(ctx, sandboxId, network, opts)
}

func Pause(ctx context.Context, sandboxId string, opts *SandboxApiOpts) (bool, error) {
	return (&sandboxApi{}).Pause(ctx, sandboxId, opts)
}

// BetaPause is deprecated. Use Pause instead.
func BetaPause(ctx context.Context, sandboxId string, opts *SandboxApiOpts) (bool, error) {
	return (&sandboxApi{}).BetaPause(ctx, sandboxId, opts)
}

func CreateSnapshot(ctx context.Context, sandboxId string, opts *CreateSnapshotOpts) (*SnapshotInfo, error) {
	return (&sandboxApi{}).CreateSnapshot(ctx, sandboxId, opts)
}

func ListSnapshots(opts *SnapshotListOpts) *SnapshotPaginator {
	return (&sandboxApi{}).ListSnapshots(opts)
}

func DeleteSnapshot(ctx context.Context, snapshotId string, opts *SandboxApiOpts) (bool, error) {
	return (&sandboxApi{}).DeleteSnapshot(ctx, snapshotId, opts)
}

func (a *sandboxApi) killSandbox(ctx context.Context, sandboxId string, opts *SandboxApiOpts) (bool, error) {
	if opts == nil {
		opts = &SandboxApiOpts{}
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

func (a *sandboxApi) Kill(ctx context.Context, sandboxId string, opts *SandboxApiOpts) (bool, error) {
	return a.killSandbox(ctx, sandboxId, opts)
}

func (a *sandboxApi) GetInfo(ctx context.Context, sandboxId string, opts *SandboxApiOpts) (*SandboxInfo, error) {
	if opts == nil {
		opts = &SandboxApiOpts{}
	}
	client, err := a.newClient(opts)
	if err != nil {
		return nil, err
	}

	resp, err := getSandboxResponse(ctx, client, sandboxId)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return nil, &SandboxNotFoundError{NotFoundError: NotFoundError{SandboxError: SandboxError{Message: fmt.Sprintf("Sandbox %s not found", sandboxId)}}}
		}
		return nil, err
	}

	info := sandboxResponseToInfo(resp)
	return &info, nil
}

func (a *sandboxApi) getFullInfo(ctx context.Context, sandboxId string, opts *SandboxApiOpts) (*SandboxFullInfo, error) {
	if opts == nil {
		opts = &SandboxApiOpts{}
	}
	client, err := a.newClient(opts)
	if err != nil {
		return nil, err
	}

	resp, err := getSandboxResponse(ctx, client, sandboxId)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return nil, &SandboxNotFoundError{NotFoundError: NotFoundError{SandboxError: SandboxError{Message: fmt.Sprintf("Sandbox %s not found", sandboxId)}}}
		}
		return nil, err
	}

	return sandboxRespToFullInfo(resp), nil
}

func (a *sandboxApi) GetMetrics(ctx context.Context, sandboxId string, opts *SandboxMetricsOpts) ([]SandboxMetrics, error) {
	if opts == nil {
		opts = &SandboxMetricsOpts{}
	}
	client, err := a.newClient(&opts.SandboxApiOpts)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	if opts.Start != nil {
		params.Set("start", strconv.FormatInt(roundUnixSeconds(*opts.Start), 10))
	}
	if opts.End != nil {
		params.Set("end", strconv.FormatInt(roundUnixSeconds(*opts.End), 10))
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
		memUsed := resolveMetricValue(m.MemUsed, m.MemUsedMiB)
		memTotal := resolveMetricValue(m.MemTotal, m.MemTotalMiB)
		diskUsed := resolveMetricValue(m.DiskUsed, m.DiskUsedMiB)
		diskTotal := resolveMetricValue(m.DiskTotal, m.DiskTotalMiB)
		metrics[i] = SandboxMetrics{
			Timestamp:  m.Timestamp,
			CpuUsedPct: m.CpuUsedPct,
			CpuCount:   m.CpuCount,
			MemUsed:    memUsed,
			MemTotal:   memTotal,
			MemCache:   m.MemCache,
			DiskUsed:   diskUsed,
			DiskTotal:  diskTotal,
		}
	}
	return metrics, nil
}

func getSandboxResponse(ctx context.Context, client *api.ApiClient, sandboxId string) (*api.SandboxResponse, error) {
	var resp sandboxResponseEnvelope
	_, err := client.Get(ctx, fmt.Sprintf("/sandboxes/%s", sandboxId), &resp)
	if err != nil {
		return nil, err
	}
	if !resp.present {
		return nil, errors.New("Sandbox not found")
	}

	return &resp.SandboxResponse, nil
}

func ensureSandboxConnectionResponseData(resp *api.SandboxResponse) error {
	if resp == nil || resp.SandboxID == "" {
		return errors.New("Response data is missing")
	}

	return nil
}

func ensureSnapshotResponseData(resp *api.SnapshotInfo) error {
	if resp == nil || resp.SnapshotID == "" {
		return errors.New("Response data is missing")
	}

	return nil
}

func (a *sandboxApi) SetTimeout(ctx context.Context, sandboxId string, timeoutMs int, opts *SandboxApiOpts) error {
	if opts == nil {
		opts = &SandboxApiOpts{}
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
			return &SandboxNotFoundError{NotFoundError: NotFoundError{SandboxError: SandboxError{Message: fmt.Sprintf("Sandbox %s not found", sandboxId)}}}
		}
		return err
	}
	return nil
}

func (a *sandboxApi) UpdateNetwork(ctx context.Context, sandboxId string, network SandboxNetworkUpdate, opts *SandboxApiOpts) error {
	if opts == nil {
		opts = &SandboxApiOpts{}
	}
	client, err := a.newClient(opts)
	if err != nil {
		return err
	}

	body, err := buildNetworkUpdateBody(network)
	if err != nil {
		return err
	}
	_, err = client.Put(ctx, fmt.Sprintf("/sandboxes/%s/network", sandboxId), body, nil)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return &SandboxNotFoundError{NotFoundError: NotFoundError{SandboxError: SandboxError{Message: fmt.Sprintf("Sandbox %s not found", sandboxId)}}}
		}
		return err
	}

	return nil
}

func (a *sandboxApi) Pause(ctx context.Context, sandboxId string, opts *SandboxApiOpts) (bool, error) {
	if opts == nil {
		opts = &SandboxApiOpts{}
	}
	client, err := a.newClient(opts)
	if err != nil {
		return false, err
	}

	resp, err := client.Post(ctx, fmt.Sprintf("/sandboxes/%s/pause", sandboxId), struct{}{}, nil)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return false, &SandboxNotFoundError{NotFoundError: NotFoundError{SandboxError: SandboxError{Message: fmt.Sprintf("Sandbox %s not found", sandboxId)}}}
		}
		// Check for 409 Conflict (already paused)
		if resp != nil && resp.StatusCode == http.StatusConflict {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// BetaPause is deprecated. Use Pause instead.
func (a *sandboxApi) BetaPause(ctx context.Context, sandboxId string, opts *SandboxApiOpts) (bool, error) {
	return a.Pause(ctx, sandboxId, opts)
}

func (a *sandboxApi) CreateSnapshot(ctx context.Context, sandboxId string, opts *CreateSnapshotOpts) (*SnapshotInfo, error) {
	if opts == nil {
		opts = &CreateSnapshotOpts{}
	}
	client, err := a.newClient(&opts.SandboxApiOpts)
	if err != nil {
		return nil, err
	}

	var resp api.SnapshotInfo
	body := struct {
		Name string `json:"name,omitempty"`
	}{Name: opts.Name}
	_, err = client.Post(ctx, fmt.Sprintf("/sandboxes/%s/snapshots", sandboxId), body, &resp)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return nil, &SandboxNotFoundError{NotFoundError: NotFoundError{SandboxError: SandboxError{Message: fmt.Sprintf("Sandbox %s not found", sandboxId)}}}
		}
		return nil, err
	}
	if err := ensureSnapshotResponseData(&resp); err != nil {
		return nil, err
	}

	info := snapshotInfoFromAPI(resp)
	return &info, nil
}

func (a *sandboxApi) ListSnapshots(opts *SnapshotListOpts) *SnapshotPaginator {
	if opts == nil {
		opts = &SnapshotListOpts{}
	}
	sandboxID := opts.SandboxID
	limit := opts.Limit
	initialToken := opts.NextToken

	fetchPage := func(ctx context.Context, nextToken string, override *SandboxApiOpts) ([]SnapshotInfo, string, error) {
		effectiveOpts := sandboxApiOptsFromSnapshotListOpts(opts)
		if override != nil {
			effectiveOpts = mergeSandboxApiOpts(effectiveOpts, *override)
		}
		ctx, cancel := mergeSandboxApiSignal(ctx, &effectiveOpts)
		defer cancel()
		client, err := a.newClient(&effectiveOpts)
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
			result[i] = snapshotInfoFromAPI(s)
		}
		return result, token, nil
	}

	return &SnapshotPaginator{
		paginator: newPaginatorWithInitialToken(func(ctx context.Context, nextToken string) ([]SnapshotInfo, string, error) {
			return fetchPage(ctx, nextToken, nil)
		}, initialToken),
		fetchWithOpts: fetchPage,
	}
}

func (a *sandboxApi) DeleteSnapshot(ctx context.Context, snapshotId string, opts *SandboxApiOpts) (bool, error) {
	if opts == nil {
		opts = &SandboxApiOpts{}
	}
	client, err := a.newClient(opts)
	if err != nil {
		return false, err
	}

	_, err = client.Delete(ctx, fmt.Sprintf("/templates/%s", url.PathEscape(snapshotId)), nil)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *sandboxApi) createSandbox(ctx context.Context, opts *SandboxOpts) (*sandboxConnectionInfo, error) {
	if opts == nil {
		opts = &SandboxOpts{}
	}
	client, err := api.NewApiClient(toClientConfig(NewConnectionConfig(&opts.ConnectionOpts)), api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	timeoutSec := 300 // default 5 minutes
	if opts.TimeoutMs != nil {
		timeoutSec = int(math.Ceil(float64(*opts.TimeoutMs) / 1000.0))
	}

	reqBody, err := buildCreateSandboxRequest(opts.Template, opts, false, timeoutSec*1000)
	if err != nil {
		return nil, err
	}

	var resp api.SandboxResponse
	_, err = client.Post(ctx, "/sandboxes", reqBody, &resp)
	if err != nil {
		return nil, err
	}
	if err := ensureSandboxConnectionResponseData(&resp); err != nil {
		return nil, err
	}
	if err := ensureSupportedTemplateEnvd(ctx, client, resp.SandboxID, resp.EnvdVersion); err != nil {
		return nil, err
	}

	return sandboxRespToConnectionInfo(&resp), nil
}

func (a *sandboxApi) connectSandbox(ctx context.Context, sandboxId string, opts *SandboxConnectOpts) (*sandboxConnectionInfo, error) {
	if opts == nil {
		opts = &SandboxConnectOpts{}
	}
	client, err := api.NewApiClient(toClientConfig(NewConnectionConfig(&opts.ConnectionOpts)), api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	body := struct {
		Timeout int `json:"timeout"`
	}{}
	timeoutMs := defaultSandboxTimeoutMs
	if opts.TimeoutMs != nil {
		timeoutMs = *opts.TimeoutMs
	}
	body.Timeout = int(math.Ceil(float64(timeoutMs) / 1000.0))

	var resp api.SandboxResponse
	_, err = client.Post(ctx, fmt.Sprintf("/sandboxes/%s/connect", sandboxId), body, &resp)
	if err != nil {
		var nfe *api.NotFoundError
		if errors.As(err, &nfe) {
			return nil, &SandboxNotFoundError{NotFoundError: NotFoundError{SandboxError: SandboxError{Message: fmt.Sprintf("Paused sandbox %s not found", sandboxId)}}}
		}
		return nil, err
	}
	if err := ensureSandboxConnectionResponseData(&resp); err != nil {
		return nil, err
	}

	return sandboxRespToConnectionInfo(&resp), nil
}

func (a *sandboxApi) ListSandboxes(opts *SandboxListOpts) *SandboxPaginator {
	if opts == nil {
		opts = &SandboxListOpts{}
	}
	metadata, states := resolveSandboxListQuery(opts)
	limit := opts.Limit
	initialToken := opts.NextToken

	fetchPage := func(ctx context.Context, nextToken string, override *SandboxApiOpts) ([]SandboxInfo, string, error) {
		effectiveOpts := sandboxApiOptsFromSandboxListOpts(opts)
		if override != nil {
			effectiveOpts = mergeSandboxApiOpts(effectiveOpts, *override)
		}
		ctx, cancel := mergeSandboxApiSignal(ctx, &effectiveOpts)
		defer cancel()
		client, err := a.newClient(&effectiveOpts)
		if err != nil {
			return nil, "", err
		}

		params := url.Values{}
		if len(metadata) > 0 {
			encodedMetadata := url.Values{}
			for k, v := range metadata {
				encodedMetadata.Set(k, v)
			}
			params.Set("metadata", encodedMetadata.Encode())
		}
		for _, state := range states {
			if state != "" {
				params.Add("state", string(state))
			}
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
	}

	return &SandboxPaginator{
		paginator: newPaginatorWithInitialToken(func(ctx context.Context, nextToken string) ([]SandboxInfo, string, error) {
			return fetchPage(ctx, nextToken, nil)
		}, initialToken),
		fetchWithOpts: fetchPage,
	}
}

func resolveSandboxListQuery(opts *SandboxListOpts) (map[string]string, []SandboxState) {
	if opts == nil || opts.Query == nil {
		return nil, nil
	}

	var metadata map[string]string
	if len(opts.Query.Metadata) > 0 {
		metadata = opts.Query.Metadata
	}

	var states []SandboxState
	if len(opts.Query.State) > 0 {
		states = append(states, opts.Query.State...)
	}

	return metadata, states
}

// sandboxResponseToConnectionInfo converts an API response to connection info.
func sandboxRespToConnectionInfo(r *api.SandboxResponse) *sandboxConnectionInfo {
	if r == nil {
		return &sandboxConnectionInfo{}
	}

	return &sandboxConnectionInfo{
		SandboxID:          r.SandboxID,
		SandboxDomain:      resolveSandboxDomain(r),
		EnvdVersion:        r.EnvdVersion,
		EnvdAccessToken:    r.EnvdAccessToken,
		TrafficAccessToken: r.TrafficAccessToken,
	}
}

func sandboxRespToFullInfo(r *api.SandboxResponse) *SandboxFullInfo {
	info := sandboxResponseToInfo(r)

	return &SandboxFullInfo{
		SandboxID:           info.SandboxID,
		TemplateID:          info.TemplateID,
		Name:                info.Name,
		Metadata:            info.Metadata,
		StartedAt:           info.StartedAt,
		EndAt:               info.EndAt,
		State:               info.State,
		CpuCount:            info.CpuCount,
		MemoryMB:            info.MemoryMB,
		EnvdVersion:         info.EnvdVersion,
		AllowInternetAccess: info.AllowInternetAccess,
		Network:             info.Network,
		Lifecycle:           info.Lifecycle,
		VolumeMounts:        info.VolumeMounts,
		SandboxDomain:       resolveSandboxDomain(r),
		EnvdAccessToken:     r.EnvdAccessToken,
	}
}

type createSandboxRequest struct {
	TemplateID          string                `json:"templateID"`
	Timeout             int                   `json:"timeout"`
	Metadata            map[string]string     `json:"metadata,omitempty"`
	Mcp                 map[string]any        `json:"mcp,omitempty"`
	EnvVars             map[string]string     `json:"envVars,omitempty"`
	Secure              *bool                 `json:"secure,omitempty"`
	AllowInternetAccess *bool                 `json:"allow_internet_access,omitempty"`
	AutoPause           *bool                 `json:"autoPause,omitempty"`
	AutoResume          *api.AutoResumeConfig `json:"autoResume,omitempty"`
	Network             *networkOptsRequest   `json:"network,omitempty"`
	VolumeMounts        []api.VolumeMount     `json:"volumeMounts,omitempty"`
}

type networkOptsRequest struct {
	AllowOut           []string
	allowOutSet        bool
	DenyOut            []string
	denyOutSet         bool
	AllowPublicTraffic *bool
	MaskRequestHost    string
	Rules              map[string][]api.NetworkRule
	rulesSet           bool
}

func (n networkOptsRequest) MarshalJSON() ([]byte, error) {
	body := map[string]any{}
	if len(n.AllowOut) > 0 || n.allowOutSet {
		body["allowOut"] = emptyStringList(n.AllowOut)
	}
	if len(n.DenyOut) > 0 || n.denyOutSet {
		body["denyOut"] = emptyStringList(n.DenyOut)
	}
	if n.AllowPublicTraffic != nil {
		body["allowPublicTraffic"] = n.AllowPublicTraffic
	}
	if n.MaskRequestHost != "" {
		body["maskRequestHost"] = n.MaskRequestHost
	}
	if len(n.Rules) > 0 || n.rulesSet {
		body["rules"] = emptyNetworkRuleMap(n.Rules)
	}
	return json.Marshal(body)
}

type sandboxNetworkUpdateRequest struct {
	AllowOut            []string
	allowOutSet         bool
	DenyOut             []string
	denyOutSet          bool
	Rules               map[string][]api.NetworkRule
	rulesSet            bool
	AllowInternetAccess *bool
}

func (n sandboxNetworkUpdateRequest) MarshalJSON() ([]byte, error) {
	body := map[string]any{}
	if len(n.AllowOut) > 0 || n.allowOutSet {
		body["allowOut"] = emptyStringList(n.AllowOut)
	}
	if len(n.DenyOut) > 0 || n.denyOutSet {
		body["denyOut"] = emptyStringList(n.DenyOut)
	}
	if len(n.Rules) > 0 || n.rulesSet {
		body["rules"] = emptyNetworkRuleMap(n.Rules)
	}
	if n.AllowInternetAccess != nil {
		body["allow_internet_access"] = n.AllowInternetAccess
	}
	return json.Marshal(body)
}

func emptyStringList(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func emptyNetworkRuleMap(rules map[string][]api.NetworkRule) map[string][]api.NetworkRule {
	if rules == nil {
		return map[string][]api.NetworkRule{}
	}
	return rules
}

func buildCreateSandboxRequest(template string, opts *SandboxOpts, autoPause bool, timeoutMs int) (createSandboxRequest, error) {
	timeoutSec := int(math.Ceil(float64(timeoutMs) / 1000.0))
	lifecycle := resolveSandboxLifecycle(opts.Lifecycle, autoPause)
	if err := validateSandboxLifecycle(lifecycle); err != nil {
		return createSandboxRequest{}, err
	}

	req := createSandboxRequest{
		TemplateID:          template,
		Timeout:             timeoutSec,
		Metadata:            opts.Metadata,
		Mcp:                 map[string]any(opts.Mcp),
		EnvVars:             opts.Envs,
		Secure:              resolveSecure(opts.Secure),
		AllowInternetAccess: resolveAllowInternetAccess(opts.AllowInternetAccess),
		AutoPause:           resolveAutoPause(lifecycle),
		AutoResume:          resolveAutoResume(lifecycle),
	}
	if opts.Network != nil {
		allowOut, err := resolveNetworkSelector(opts.Network.AllowOut, opts.Network.Rules)
		if err != nil {
			return createSandboxRequest{}, err
		}
		denyOut, err := resolveNetworkSelector(opts.Network.DenyOut, opts.Network.Rules)
		if err != nil {
			return createSandboxRequest{}, err
		}
		req.Network = &networkOptsRequest{
			AllowOut:           allowOut,
			allowOutSet:        opts.Network.AllowOut != nil,
			DenyOut:            denyOut,
			denyOutSet:         opts.Network.DenyOut != nil,
			Rules:              buildNetworkRulesForAPI(opts.Network.Rules),
			rulesSet:           opts.Network.Rules != nil,
			AllowPublicTraffic: opts.Network.AllowPublicTraffic,
			MaskRequestHost:    opts.Network.MaskRequestHost,
		}
	}
	if len(opts.VolumeMounts) > 0 {
		mounts, err := resolveCreateVolumeMounts(opts.VolumeMounts)
		if err != nil {
			return createSandboxRequest{}, err
		}
		req.VolumeMounts = mounts
	}
	return req, nil
}

func buildNetworkUpdateBody(network SandboxNetworkUpdate) (sandboxNetworkUpdateRequest, error) {
	allowOut, err := resolveNetworkSelector(network.AllowOut, network.Rules)
	if err != nil {
		return sandboxNetworkUpdateRequest{}, err
	}
	denyOut, err := resolveNetworkSelector(network.DenyOut, network.Rules)
	if err != nil {
		return sandboxNetworkUpdateRequest{}, err
	}

	return sandboxNetworkUpdateRequest{
		AllowOut:            allowOut,
		allowOutSet:         network.AllowOut != nil,
		DenyOut:             denyOut,
		denyOutSet:          network.DenyOut != nil,
		Rules:               buildNetworkRulesForAPI(network.Rules),
		rulesSet:            network.Rules != nil,
		AllowInternetAccess: network.AllowInternetAccess,
	}, nil
}

func buildNetworkRulesForAPI(rules SandboxNetworkRules) map[string][]api.NetworkRule {
	if len(rules) == 0 {
		return nil
	}

	out := make(map[string][]api.NetworkRule, len(rules))
	for host, hostRules := range rules {
		converted := make([]api.NetworkRule, len(hostRules))
		for i, rule := range hostRules {
			converted[i] = api.NetworkRule{
				Transform: buildNetworkTransformForAPI(rule.Transform),
			}
		}
		out[host] = converted
	}

	return out
}

func resolveNetworkSelector(selector SandboxNetworkSelector, rules SandboxNetworkRules) ([]string, error) {
	if selector == nil {
		return nil, nil
	}

	switch value := selector.(type) {
	case []string:
		return append([]string(nil), value...), nil
	case SandboxNetworkSelectorFunc:
		return append([]string(nil), value(SandboxNetworkSelectorContext{
			AllTraffic: ALL_TRAFFIC,
			Rules:      cloneNetworkRules(rules),
		})...), nil
	case func(SandboxNetworkSelectorContext) []string:
		return append([]string(nil), value(SandboxNetworkSelectorContext{
			AllTraffic: ALL_TRAFFIC,
			Rules:      cloneNetworkRules(rules),
		})...), nil
	default:
		return nil, &InvalidArgumentError{SandboxError: SandboxError{Message: "network selector must be []string or func(SandboxNetworkSelectorContext) []string"}}
	}
}

func cloneNetworkRules(rules SandboxNetworkRules) SandboxNetworkRules {
	if len(rules) == 0 {
		return nil
	}

	out := make(SandboxNetworkRules, len(rules))
	for host, hostRules := range rules {
		converted := make([]SandboxNetworkRule, len(hostRules))
		for i, rule := range hostRules {
			converted[i] = SandboxNetworkRule{
				Transform: cloneNetworkTransform(rule.Transform),
			}
		}
		out[host] = converted
	}

	return out
}

func cloneNetworkTransform(transform *SandboxNetworkTransform) *SandboxNetworkTransform {
	if transform == nil {
		return nil
	}

	return &SandboxNetworkTransform{
		Headers: cloneNetworkHeaders(transform.Headers),
	}
}

func buildNetworkTransformForAPI(transform *SandboxNetworkTransform) *api.NetworkTransform {
	if transform == nil {
		return nil
	}

	return &api.NetworkTransform{
		Headers: cloneNetworkHeaders(transform.Headers),
	}
}

func networkRulesFromAPI(rules map[string][]api.NetworkRule) SandboxNetworkRules {
	if len(rules) == 0 {
		return nil
	}

	out := make(SandboxNetworkRules, len(rules))
	for host, hostRules := range rules {
		converted := make([]SandboxNetworkRule, len(hostRules))
		for i, rule := range hostRules {
			converted[i] = SandboxNetworkRule{
				Transform: networkTransformFromAPI(rule.Transform),
			}
		}
		out[host] = converted
	}

	return out
}

func networkTransformFromAPI(transform *api.NetworkTransform) *SandboxNetworkTransform {
	if transform == nil {
		return nil
	}

	return &SandboxNetworkTransform{
		Headers: cloneNetworkHeaders(transform.Headers),
	}
}

func cloneNetworkHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[k] = v
	}

	return out
}

func resolveCreateVolumeMounts(volumeMounts map[string]any) ([]api.VolumeMount, error) {
	paths := make([]string, 0, len(volumeMounts))
	for path := range volumeMounts {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	result := make([]api.VolumeMount, 0, len(paths))
	for _, path := range paths {
		name, err := resolveCreateVolumeMountName(volumeMounts[path])
		if err != nil {
			return nil, fmt.Errorf("invalid volume mount for path %q: %w", path, err)
		}
		result = append(result, api.VolumeMount{
			Name: name,
			Path: path,
		})
	}

	return result, nil
}

func resolveCreateVolumeMountName(value any) (string, error) {
	if name, ok := value.(string); ok {
		if name == "" {
			return "", errors.New("volume name must not be empty")
		}
		return name, nil
	}
	if value == nil {
		return "", errors.New("volume mount must not be nil")
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "", errors.New("volume mount pointer must not be nil")
		}
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Struct {
		nameField := rv.FieldByName("Name")
		if nameField.IsValid() && nameField.Kind() == reflect.String {
			name := nameField.String()
			if name == "" {
				return "", errors.New("volume mount Name must not be empty")
			}
			return name, nil
		}
	}

	return "", errors.New("volume mount must be a string or a value with a Name field")
}

func ensureSupportedTemplateEnvd(ctx context.Context, client *api.ApiClient, sandboxID, envdVersion string) error {
	if sandboxVersionGTE(envdVersion, "0.1.0") {
		return nil
	}

	if sandboxID != "" {
		_, _ = client.Delete(ctx, fmt.Sprintf("/sandboxes/%s", sandboxID), nil)
	}

	return &TemplateError{SandboxError: SandboxError{Message: "You need to update the template to use the new SDK. You can do this by running `e2b template build` in the directory with the template."}}
}

func resolveAllowInternetAccess(value *bool) *bool {
	if value != nil {
		return value
	}

	defaultValue := true
	return &defaultValue
}

func resolveSecure(value *bool) *bool {
	if value != nil {
		return value
	}

	defaultValue := true
	return &defaultValue
}

func resolveSandboxDomain(r *api.SandboxResponse) string {
	if r == nil {
		return ""
	}
	if r.Domain != "" {
		return r.Domain
	}
	return r.SandboxDomain
}

func resolveMetricValue(current int64, legacy int64) int64 {
	if current != 0 {
		return current
	}
	return legacy
}

func roundUnixSeconds(value time.Time) int64 {
	return int64(math.Round(float64(value.UnixMilli()) / 1000.0))
}

func resolveSandboxLifecycle(lifecycle *SandboxLifecycle, autoPause bool) *SandboxLifecycle {
	if lifecycle != nil {
		return lifecycle
	}
	if autoPause {
		return &SandboxLifecycle{
			OnTimeout: "pause",
		}
	}
	return &SandboxLifecycle{
		OnTimeout: "kill",
	}
}

func resolveAutoPause(lifecycle *SandboxLifecycle) *bool {
	autoPause := false
	if lifecycle != nil && lifecycle.OnTimeout == "pause" {
		autoPause = true
	}
	return &autoPause
}

func resolveAutoResume(lifecycle *SandboxLifecycle) *api.AutoResumeConfig {
	if lifecycle == nil {
		return nil
	}

	return &api.AutoResumeConfig{
		Enabled: boolValue(lifecycle.AutoResume),
	}
}

func validateSandboxLifecycle(lifecycle *SandboxLifecycle) error {
	if lifecycle == nil {
		return nil
	}
	if boolValue(lifecycle.AutoResume) && lifecycle.OnTimeout != "pause" {
		return &InvalidArgumentError{
			SandboxError: SandboxError{
				Message: "autoResume can only be true when the resolved onTimeout is 'pause'.",
			},
		}
	}
	return nil
}
