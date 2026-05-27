package volume

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/superduck-ai/e2b-go-sdk/api"
	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

type ConnectionOpts struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            bool
	RequestTimeoutMs *int
	Logger           api.Logger
	Headers          map[string]string
}

type Volume struct {
	VolumeID string
	Name     string
	Token    string
	Domain   string
	Debug    bool
	client   *volumeApiClient
}

func buildApiClientConfig(opts *ConnectionOpts) *api.ClientConfig {
	if opts == nil {
		opts = &ConnectionOpts{}
	}

	apiKey := opts.ApiKey
	if apiKey == "" {
		apiKey = os.Getenv("E2B_API_KEY")
	}

	accessToken := opts.AccessToken
	if accessToken == "" {
		accessToken = os.Getenv("E2B_ACCESS_TOKEN")
	}

	domain := opts.Domain
	if domain == "" {
		domain = os.Getenv("E2B_DOMAIN")
		if domain == "" {
			domain = "e2b.app"
		}
	}

	apiUrl := opts.ApiUrl
	if apiUrl == "" {
		apiUrl = os.Getenv("E2B_API_URL")
	}
	if apiUrl == "" && opts.Debug {
		apiUrl = "http://localhost:3000"
	}

	timeout := 60000
	if opts.RequestTimeoutMs != nil {
		timeout = *opts.RequestTimeoutMs
	}

	return &api.ClientConfig{
		ApiKey:           apiKey,
		AccessToken:      accessToken,
		Domain:           domain,
		ApiUrl:           apiUrl,
		RequestTimeoutMs: timeout,
		Logger:           opts.Logger,
		Headers:          opts.Headers,
	}
}

func newVolumeApiClient(volumeID string, token string, opts *ConnectionOpts) *volumeApiClient {
	if opts == nil {
		opts = &ConnectionOpts{}
	}
	apiOpts := &VolumeApiOpts{
		Token:  token,
		Domain: opts.Domain,
		Debug:  opts.Debug,
	}
	config := NewVolumeConnectionConfig(apiOpts)
	return newVolumeApiClientWithConfig(config)
}

// Create creates a new volume with the given name.
func Create(ctx context.Context, name string, opts *ConnectionOpts) (*Volume, error) {
	if opts == nil {
		opts = &ConnectionOpts{}
	}

	clientConfig := buildApiClientConfig(opts)
	apiClient, err := api.NewApiClient(clientConfig, api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	type createRequest struct {
		Name string `json:"name"`
	}
	type createResponse struct {
		VolumeID string `json:"volumeID"`
		Name     string `json:"name"`
		Token    string `json:"token"`
	}

	var resp createResponse
	_, err = apiClient.Post(ctx, "/volumes", &createRequest{Name: name}, &resp)
	if err != nil {
		return nil, wrapControlPlaneVolumeError(err, "")
	}

	v := &Volume{
		VolumeID: resp.VolumeID,
		Name:     resp.Name,
		Token:    resp.Token,
	}
	if v.VolumeID == "" && v.Name == "" && v.Token == "" {
		return nil, fmt.Errorf("Response data is missing")
	}
	v.client = newVolumeApiClient(v.VolumeID, v.Token, opts)
	v.Domain = v.client.config.Domain
	v.Debug = v.client.config.Debug
	return v, nil
}

// Connect connects to an existing volume by its ID.
func Connect(ctx context.Context, volumeId string, opts *ConnectionOpts) (*Volume, error) {
	info, err := GetInfo(ctx, volumeId, opts)
	if err != nil {
		return nil, err
	}

	v := &Volume{
		VolumeID: info.VolumeID,
		Name:     info.Name,
		Token:    info.Token,
	}
	if v.VolumeID == "" && v.Name == "" && v.Token == "" {
		return nil, fmt.Errorf("Response data is missing")
	}
	v.client = newVolumeApiClient(v.VolumeID, v.Token, opts)
	v.Domain = v.client.config.Domain
	v.Debug = v.client.config.Debug
	return v, nil
}

// GetInfo retrieves information about a volume including its access token.
func GetInfo(ctx context.Context, volumeId string, opts *ConnectionOpts) (*VolumeAndToken, error) {
	if opts == nil {
		opts = &ConnectionOpts{}
	}

	clientConfig := buildApiClientConfig(opts)
	apiClient, err := api.NewApiClient(clientConfig, api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	type infoResponse struct {
		VolumeID string `json:"volumeID"`
		Name     string `json:"name"`
		Token    string `json:"token"`
	}

	var resp infoResponse
	_, err = apiClient.Get(ctx, "/volumes/"+url.PathEscape(volumeId), &resp)
	if err != nil {
		return nil, wrapControlPlaneVolumeError(err, fmt.Sprintf("Volume %s not found", volumeId))
	}

	return &VolumeAndToken{
		VolumeInfo: VolumeInfo{
			VolumeID: resp.VolumeID,
			Name:     resp.Name,
		},
		Token: resp.Token,
	}, ensureVolumeAndTokenData(resp)
}

// List returns all volumes.
func List(ctx context.Context, opts *ConnectionOpts) ([]VolumeInfo, error) {
	if opts == nil {
		opts = &ConnectionOpts{}
	}

	clientConfig := buildApiClientConfig(opts)
	apiClient, err := api.NewApiClient(clientConfig, api.WithRequireApiKey())
	if err != nil {
		return nil, err
	}

	type listItem struct {
		VolumeID string `json:"volumeID"`
		Name     string `json:"name"`
	}

	var resp []listItem
	_, err = apiClient.Get(ctx, "/volumes", &resp)
	if err != nil {
		return nil, wrapControlPlaneVolumeError(err, "")
	}

	result := make([]VolumeInfo, len(resp))
	for i, item := range resp {
		result[i] = VolumeInfo{
			VolumeID: item.VolumeID,
			Name:     item.Name,
		}
	}
	return result, nil
}

// Destroy deletes a volume by its ID. Returns false if the volume was not found (404).
func Destroy(ctx context.Context, volumeId string, opts *ConnectionOpts) (bool, error) {
	if opts == nil {
		opts = &ConnectionOpts{}
	}

	clientConfig := buildApiClientConfig(opts)
	apiClient, err := api.NewApiClient(clientConfig, api.WithRequireApiKey())
	if err != nil {
		return false, err
	}

	_, err = apiClient.Delete(ctx, "/volumes/"+url.PathEscape(volumeId), nil)
	if err != nil {
		if _, ok := err.(*api.NotFoundError); ok {
			return false, nil
		}
		return false, wrapControlPlaneVolumeError(err, "")
	}
	return true, nil
}

// --- Instance methods ---

func (v *Volume) volumeContentPath(endpoint string, path string, query url.Values) string {
	if query == nil {
		query = url.Values{}
	}
	query.Set("path", path)
	return fmt.Sprintf("/volumecontent/%s/%s?%s", url.PathEscape(v.VolumeID), endpoint, query.Encode())
}

// List lists the contents of a directory at the given path.
func (v *Volume) List(ctx context.Context, path string, opts *struct {
	Token            string
	Domain           string
	Debug            bool
	ApiUrl           string
	RequestTimeoutMs *int
	Logger           api.Logger
	Headers          map[string]string
	Depth            *int
}) ([]VolumeEntryStat, error) {
	query := url.Values{}
	// Default depth 1
	depth := 1
	if opts != nil && opts.Depth != nil {
		depth = *opts.Depth
	}
	query.Set("depth", strconv.Itoa(depth))

	var clientOpts *VolumeApiOpts
	if opts != nil {
		clientOpts = &VolumeApiOpts{
			Token:            opts.Token,
			Domain:           opts.Domain,
			Debug:            opts.Debug,
			ApiUrl:           opts.ApiUrl,
			RequestTimeoutMs: opts.RequestTimeoutMs,
			Logger:           opts.Logger,
			Headers:          opts.Headers,
		}
	}

	var result []VolumeEntryStat
	client := v.resolveClient(clientOpts)
	err := client.Do(ctx, http.MethodGet, v.volumeContentPath("dir", path, query), nil, &result, client.config.RequestTimeoutMs)
	if err != nil {
		return nil, wrapContentVolumeError(err, fmt.Sprintf("Path %s not found", path))
	}
	if result == nil {
		return []VolumeEntryStat{}, nil
	}
	return result, nil
}

// MakeDir creates a directory at the given path.
func (v *Volume) MakeDir(ctx context.Context, path string, opts *VolumeWriteOptions) (*VolumeEntryStat, error) {
	query := queryFromVolumeWriteOpts(opts)

	var result VolumeEntryStat
	client := v.resolveClient(volumeWriteOptsToApiOpts(opts))
	err := client.Do(ctx, http.MethodPost, v.volumeContentPath("dir", path, query), nil, &result, client.config.RequestTimeoutMs)
	if err != nil {
		return nil, wrapContentVolumeError(err, fmt.Sprintf("Path %s not found", path))
	}
	if isZeroVolumeEntryStat(result) {
		return nil, fmt.Errorf("Response data is missing")
	}
	return &result, nil
}

// GetInfo returns metadata about the entry at the given path.
func (v *Volume) GetInfo(ctx context.Context, path string, opts *VolumeApiOpts) (*VolumeEntryStat, error) {
	query := url.Values{}

	var result VolumeEntryStat
	client := v.resolveClient(opts)
	err := client.Do(ctx, http.MethodGet, v.volumeContentPath("path", path, query), nil, &result, client.config.RequestTimeoutMs)
	if err != nil {
		return nil, wrapContentVolumeError(err, fmt.Sprintf("Path %s not found", path))
	}
	if isZeroVolumeEntryStat(result) {
		return nil, fmt.Errorf("Response data is missing")
	}
	return &result, nil
}

// Exists checks whether a path exists in the volume. Returns false on 404.
func (v *Volume) Exists(ctx context.Context, path string, opts *VolumeApiOpts) (bool, error) {
	_, err := v.GetInfo(ctx, path, opts)
	if err != nil {
		var notFoundErr *shared.NotFoundError
		if errors.As(err, &notFoundErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// UpdateMetadata updates the uid, gid, and/or mode of a path.
func (v *Volume) UpdateMetadata(ctx context.Context, path string, metadata *VolumeMetadataOptions, opts *VolumeApiOpts) (*VolumeEntryStat, error) {
	query := url.Values{}

	type metadataBody struct {
		UID  *int `json:"uid,omitempty"`
		GID  *int `json:"gid,omitempty"`
		Mode *int `json:"mode,omitempty"`
	}

	body := &metadataBody{}
	if metadata != nil {
		body.UID = metadata.UID
		body.GID = metadata.GID
		body.Mode = metadata.Mode
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var result VolumeEntryStat
	client := v.resolveClient(opts)
	err = client.Do(ctx, http.MethodPatch, v.volumeContentPath("path", path, query), bytes.NewReader(bodyBytes), &result, client.config.RequestTimeoutMs)
	if err != nil {
		return nil, wrapContentVolumeError(err, fmt.Sprintf("Path %s not found", path))
	}
	if isZeroVolumeEntryStat(result) {
		return nil, fmt.Errorf("Response data is missing")
	}
	return &result, nil
}

// ReadFile reads the raw bytes of a file at the given path.
func (v *Volume) ReadFile(ctx context.Context, path string, opts *VolumeApiOpts) ([]byte, error) {
	query := url.Values{}
	client := v.resolveClient(opts)
	reqPath := v.volumeContentPath("file", path, query)
	reqUrl := client.config.ApiUrl + reqPath
	reqCtx, cancel := requestContext(ctx, fileRequestTimeout(opts))
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.config.Token)
	for k, val := range client.config.Headers {
		req.Header.Set(k, val)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, wrapContentVolumeError(&volumeApiError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}, fmt.Sprintf("Path %s not found", path))
	}

	return io.ReadAll(resp.Body)
}

// ReadFileText reads the contents of a file as a string.
func (v *Volume) ReadFileText(ctx context.Context, path string, opts *VolumeApiOpts) (string, error) {
	data, err := v.ReadFile(ctx, path, opts)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile writes data to a file at the given path.
func (v *Volume) WriteFile(ctx context.Context, path string, data io.Reader, opts *VolumeWriteOptions) (*VolumeEntryStat, error) {
	query := url.Values{}
	if opts != nil {
		if opts.UID != nil {
			query.Set("uid", strconv.Itoa(*opts.UID))
		}
		if opts.GID != nil {
			query.Set("gid", strconv.Itoa(*opts.GID))
		}
		if opts.Mode != nil {
			query.Set("mode", strconv.Itoa(*opts.Mode))
		}
		if opts.Force {
			query.Set("force", "true")
		}
	}

	client := v.resolveClient(volumeWriteOptsToApiOpts(opts))
	reqPath := v.volumeContentPath("file", path, query)
	reqUrl := client.config.ApiUrl + reqPath
	reqCtx, cancel := requestContext(ctx, fileRequestTimeout(volumeWriteOptsToApiOpts(opts)))
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPut, reqUrl, data)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.config.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	for k, val := range client.config.Headers {
		req.Header.Set(k, val)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, wrapContentVolumeError(&volumeApiError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}, fmt.Sprintf("Path %s not found", path))
	}

	var result VolumeEntryStat
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("Response data is missing")
		}
		return nil, fmt.Errorf("failed to decode write response: %w", err)
	}
	if isZeroVolumeEntryStat(result) {
		return nil, fmt.Errorf("Response data is missing")
	}
	return &result, nil
}

// Remove deletes the entry at the given path.
func (v *Volume) Remove(ctx context.Context, path string, opts *VolumeApiOpts) error {
	query := url.Values{}
	client := v.resolveClient(opts)
	err := client.Do(ctx, http.MethodDelete, v.volumeContentPath("path", path, query), nil, nil, client.config.RequestTimeoutMs)
	if err != nil {
		return wrapContentVolumeError(err, fmt.Sprintf("Path %s not found", path))
	}
	return nil
}

func (v *Volume) resolveClient(opts *VolumeApiOpts) *volumeApiClient {
	if v.client == nil {
		return newVolumeApiClientWithConfig(NewVolumeConnectionConfig(opts))
	}

	if opts == nil {
		return v.client
	}

	merged := &VolumeApiOpts{
		Token:  v.client.config.Token,
		Domain: v.client.config.Domain,
		Debug:  v.client.config.Debug,
		ApiUrl: v.client.config.ApiUrl,
	}

	if opts.Token != "" {
		merged.Token = opts.Token
	}
	if opts.Domain != "" {
		merged.Domain = opts.Domain
	}
	if opts.Debug {
		merged.Debug = true
	}
	if opts.ApiUrl != "" {
		merged.ApiUrl = opts.ApiUrl
	}
	if opts.RequestTimeoutMs != nil {
		merged.RequestTimeoutMs = opts.RequestTimeoutMs
	}
	if opts.Headers != nil {
		merged.Headers = opts.Headers
	}

	return newVolumeApiClientWithConfig(NewVolumeConnectionConfig(merged))
}

func wrapControlPlaneVolumeError(err error, notFoundMessage string) error {
	if err == nil {
		return nil
	}

	var notFoundErr *api.NotFoundError
	if errors.As(err, &notFoundErr) && notFoundMessage != "" {
		return &shared.NotFoundError{SandboxError: shared.SandboxError{Message: notFoundMessage}}
	}

	var apiErr *api.ApiError
	if errors.As(err, &apiErr) {
		return &shared.VolumeError{Message: apiErr.Message}
	}

	return err
}

func wrapContentVolumeError(err error, notFoundMessage string) error {
	if err == nil {
		return nil
	}

	var apiErr *volumeApiError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusNotFound {
			return &shared.NotFoundError{SandboxError: shared.SandboxError{Message: notFoundMessage}}
		}
		return &shared.VolumeError{Message: apiErr.Message}
	}

	return err
}

func queryFromVolumeWriteOpts(opts *VolumeWriteOptions) url.Values {
	query := url.Values{}
	if opts == nil {
		return query
	}
	if opts.UID != nil {
		query.Set("uid", strconv.Itoa(*opts.UID))
	}
	if opts.GID != nil {
		query.Set("gid", strconv.Itoa(*opts.GID))
	}
	if opts.Mode != nil {
		query.Set("mode", strconv.Itoa(*opts.Mode))
	}
	if opts.Force {
		query.Set("force", "true")
	}
	return query
}

func fileRequestTimeout(opts *VolumeApiOpts) *int {
	if opts != nil && opts.RequestTimeoutMs != nil {
		return opts.RequestTimeoutMs
	}
	timeout := fileTimeoutMs
	return &timeout
}

func volumeWriteOptsToApiOpts(opts *VolumeWriteOptions) *VolumeApiOpts {
	if opts == nil {
		return nil
	}
	return &VolumeApiOpts{
		Token:            opts.Token,
		Domain:           opts.Domain,
		Debug:            opts.Debug,
		ApiUrl:           opts.ApiUrl,
		RequestTimeoutMs: opts.RequestTimeoutMs,
		Headers:          opts.Headers,
	}
}

func ensureVolumeAndTokenData(resp struct {
	VolumeID string `json:"volumeID"`
	Name     string `json:"name"`
	Token    string `json:"token"`
}) error {
	if resp.VolumeID == "" && resp.Name == "" && resp.Token == "" {
		return fmt.Errorf("Response data is missing")
	}
	return nil
}

func isZeroVolumeEntryStat(entry VolumeEntryStat) bool {
	return entry.Atime.IsZero() &&
		entry.Mtime.IsZero() &&
		entry.Ctime.IsZero() &&
		entry.Type == "" &&
		entry.Name == "" &&
		entry.Path == "" &&
		entry.Size == 0 &&
		entry.UID == 0 &&
		entry.GID == 0 &&
		entry.Mode == 0 &&
		entry.Target == ""
}
