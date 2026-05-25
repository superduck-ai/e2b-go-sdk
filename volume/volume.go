package volume

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/e2b-dev/e2b-go-sdk/api"
)

type VolumeOpts struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	Debug            bool
	RequestTimeoutMs int
	Headers          map[string]string
}

type Volume struct {
	VolumeID string
	Name     string
	token    string
	client   *VolumeApiClient
}

func buildApiClientConfig(opts *VolumeOpts) *api.ClientConfig {
	if opts == nil {
		opts = &VolumeOpts{}
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

	timeout := opts.RequestTimeoutMs
	if timeout == 0 {
		timeout = 60000
	}

	return &api.ClientConfig{
		ApiKey:           apiKey,
		AccessToken:      accessToken,
		Domain:           domain,
		ApiUrl:           apiUrl,
		RequestTimeoutMs: timeout,
		Headers:          opts.Headers,
	}
}

func newVolumeApiClient(volumeID string, token string, opts *VolumeOpts) *VolumeApiClient {
	if opts == nil {
		opts = &VolumeOpts{}
	}
	apiOpts := &VolumeApiOpts{
		Token:            token,
		Domain:           opts.Domain,
		ApiUrl:           "",
		RequestTimeoutMs: opts.RequestTimeoutMs,
		Headers:          opts.Headers,
	}
	config := NewVolumeConnectionConfig(apiOpts)
	return NewVolumeApiClient(config)
}

// Create creates a new volume with the given name.
func Create(ctx context.Context, name string, opts *VolumeOpts) (*Volume, error) {
	if opts == nil {
		opts = &VolumeOpts{}
	}

	clientConfig := buildApiClientConfig(opts)
	apiClient, err := api.NewApiClient(clientConfig, api.WithRequireApiKey())
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
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
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}

	v := &Volume{
		VolumeID: resp.VolumeID,
		Name:     resp.Name,
		token:    resp.Token,
	}
	v.client = newVolumeApiClient(v.VolumeID, v.token, opts)
	return v, nil
}

// Connect connects to an existing volume by its ID.
func Connect(ctx context.Context, volumeId string, opts *VolumeOpts) (*Volume, error) {
	info, err := GetVolumeInfo(ctx, volumeId, opts)
	if err != nil {
		return nil, err
	}

	v := &Volume{
		VolumeID: info.VolumeID,
		Name:     info.Name,
		token:    info.Token,
	}
	v.client = newVolumeApiClient(v.VolumeID, v.token, opts)
	return v, nil
}

// GetVolumeInfo retrieves information about a volume including its access token.
func GetVolumeInfo(ctx context.Context, volumeId string, opts *VolumeOpts) (*VolumeAndToken, error) {
	if opts == nil {
		opts = &VolumeOpts{}
	}

	clientConfig := buildApiClientConfig(opts)
	apiClient, err := api.NewApiClient(clientConfig, api.WithRequireApiKey())
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	type infoResponse struct {
		VolumeID string `json:"volumeID"`
		Name     string `json:"name"`
		Token    string `json:"token"`
	}

	var resp infoResponse
	_, err = apiClient.Get(ctx, "/volumes/"+url.PathEscape(volumeId), &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to get volume info: %w", err)
	}

	return &VolumeAndToken{
		VolumeInfo: VolumeInfo{
			VolumeID: resp.VolumeID,
			Name:     resp.Name,
		},
		Token: resp.Token,
	}, nil
}

// List returns all volumes.
func List(ctx context.Context, opts *VolumeOpts) ([]VolumeInfo, error) {
	if opts == nil {
		opts = &VolumeOpts{}
	}

	clientConfig := buildApiClientConfig(opts)
	apiClient, err := api.NewApiClient(clientConfig, api.WithRequireApiKey())
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	type listItem struct {
		VolumeID string `json:"volumeID"`
		Name     string `json:"name"`
	}

	var resp []listItem
	_, err = apiClient.Get(ctx, "/volumes", &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
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
func Destroy(ctx context.Context, volumeId string, opts *VolumeOpts) (bool, error) {
	if opts == nil {
		opts = &VolumeOpts{}
	}

	clientConfig := buildApiClientConfig(opts)
	apiClient, err := api.NewApiClient(clientConfig, api.WithRequireApiKey())
	if err != nil {
		return false, fmt.Errorf("failed to create API client: %w", err)
	}

	_, err = apiClient.Delete(ctx, "/volumes/"+url.PathEscape(volumeId), nil)
	if err != nil {
		if _, ok := err.(*api.NotFoundError); ok {
			return false, nil
		}
		return false, fmt.Errorf("failed to destroy volume: %w", err)
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

// ListDir lists the contents of a directory at the given path.
func (v *Volume) ListDir(ctx context.Context, path string, opts *VolumeApiOpts) ([]VolumeEntryStat, error) {
	query := url.Values{}
	// Default depth 1
	query.Set("depth", "1")

	var result []VolumeEntryStat
	err := v.client.Do(ctx, http.MethodGet, v.volumeContentPath("dir", path, query), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}
	return result, nil
}

// MakeDir creates a directory at the given path.
func (v *Volume) MakeDir(ctx context.Context, path string, opts *VolumeApiOpts) (*VolumeEntryStat, error) {
	query := url.Values{}

	var result VolumeEntryStat
	err := v.client.Do(ctx, http.MethodPost, v.volumeContentPath("dir", path, query), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to make directory: %w", err)
	}
	return &result, nil
}

// GetInfo returns metadata about the entry at the given path.
func (v *Volume) GetInfo(ctx context.Context, path string, opts *VolumeApiOpts) (*VolumeEntryStat, error) {
	query := url.Values{}

	var result VolumeEntryStat
	err := v.client.Do(ctx, http.MethodGet, v.volumeContentPath("path", path, query), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get info: %w", err)
	}
	return &result, nil
}

// Exists checks whether a path exists in the volume. Returns false on 404.
func (v *Volume) Exists(ctx context.Context, path string, opts *VolumeApiOpts) (bool, error) {
	_, err := v.GetInfo(ctx, path, opts)
	if err != nil {
		// Check if the underlying error is a 404
		if isNotFound(err) {
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
	err = v.client.Do(ctx, http.MethodPatch, v.volumeContentPath("path", path, query), bytes.NewReader(bodyBytes), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to update metadata: %w", err)
	}
	return &result, nil
}

// ReadFile reads the raw bytes of a file at the given path.
func (v *Volume) ReadFile(ctx context.Context, path string, opts *VolumeApiOpts) ([]byte, error) {
	query := url.Values{}
	reqPath := v.volumeContentPath("file", path, query)

	reqUrl := v.client.config.ApiUrl + reqPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+v.client.config.Token)
	for k, val := range v.client.config.Headers {
		req.Header.Set(k, val)
	}

	resp, err := v.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("volume API error: %d - %s", resp.StatusCode, string(respBody))
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

	reqPath := v.volumeContentPath("file", path, query)
	reqUrl := v.client.config.ApiUrl + reqPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqUrl, data)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+v.client.config.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	for k, val := range v.client.config.Headers {
		req.Header.Set(k, val)
	}

	resp, err := v.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("volume API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var result VolumeEntryStat
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode write response: %w", err)
	}
	return &result, nil
}

// Remove deletes the entry at the given path.
func (v *Volume) Remove(ctx context.Context, path string, opts *VolumeApiOpts) error {
	query := url.Values{}
	err := v.client.Do(ctx, http.MethodDelete, v.volumeContentPath("path", path, query), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to remove: %w", err)
	}
	return nil
}

// isNotFound checks if an error message indicates a 404 response.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	// The VolumeApiClient returns errors like "volume API error: 404 - ..."
	msg := err.Error()
	return len(msg) >= 20 && contains404(msg)
}

func contains404(s string) bool {
	return len(s) > 0 && (stringContains(s, "404") || stringContains(s, "not found"))
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
