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
	"reflect"
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
	Debug            *bool
	RequestTimeoutMs *int
	Signal           context.Context
	Logger           api.Logger
	Headers          map[string]string
	Proxy            string
}

type Volume struct {
	VolumeID string
	Name     string
	Token    string
	Domain   string
	Debug    *bool
}

type Blob = shared.Blob

func boolPtr(value bool) *bool {
	return &value
}

func volumeApiDebugFromConnectionOpts(value *bool) *bool {
	if value == nil || !*value {
		return nil
	}
	return boolPtr(true)
}

type ReadFileFormat string

const (
	ReadFileFormatText   ReadFileFormat = "text"
	ReadFileFormatBytes  ReadFileFormat = "bytes"
	ReadFileFormatStream ReadFileFormat = "stream"
	ReadFileFormatBlob   ReadFileFormat = "blob"
)

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r cancelReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.cancel()
	return err
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
	var debug bool
	if opts.Debug != nil && *opts.Debug {
		debug = true
	} else {
		if v, err := strconv.ParseBool(os.Getenv("E2B_DEBUG")); err == nil {
			debug = v
		}
	}
	if apiUrl == "" && debug {
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
		Proxy:            opts.Proxy,
	}
}

func newVolumeApiClient(volumeID string, token string, opts *ConnectionOpts) *volumeApiClient {
	if opts == nil {
		opts = &ConnectionOpts{}
	}
	apiOpts := &VolumeApiOpts{
		Token:            token,
		Domain:           opts.Domain,
		Debug:            volumeApiDebugFromConnectionOpts(opts.Debug),
		RequestTimeoutMs: opts.RequestTimeoutMs,
		Logger:           opts.Logger,
		Headers:          opts.Headers,
		Proxy:            opts.Proxy,
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
	reqCtx, cancel := requestContextWithSignal(ctx, opts.Signal, nil)
	defer cancel()

	_, err = apiClient.Post(reqCtx, "/volumes", &createRequest{Name: name}, &resp)
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
	config := NewVolumeConnectionConfig(&VolumeApiOpts{
		Token:  v.Token,
		Domain: opts.Domain,
		Debug:  volumeApiDebugFromConnectionOpts(opts.Debug),
	})
	v.Domain = config.Domain
	v.Debug = boolPtr(config.Debug)
	return v, nil
}

// Connect connects to an existing volume by its ID.
func Connect(ctx context.Context, volumeId string, opts *ConnectionOpts) (*Volume, error) {
	if opts == nil {
		opts = &ConnectionOpts{}
	}

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
	config := NewVolumeConnectionConfig(&VolumeApiOpts{
		Token:  v.Token,
		Domain: opts.Domain,
		Debug:  volumeApiDebugFromConnectionOpts(opts.Debug),
	})
	v.Domain = config.Domain
	v.Debug = boolPtr(config.Debug)
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
	reqCtx, cancel := requestContextWithSignal(ctx, opts.Signal, nil)
	defer cancel()

	_, err = apiClient.Get(reqCtx, "/volumes/"+url.PathEscape(volumeId), &resp)
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
	reqCtx, cancel := requestContextWithSignal(ctx, opts.Signal, nil)
	defer cancel()

	_, err = apiClient.Get(reqCtx, "/volumes", &resp)
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

	reqCtx, cancel := requestContextWithSignal(ctx, opts.Signal, nil)
	defer cancel()

	_, err = apiClient.Delete(reqCtx, "/volumes/"+url.PathEscape(volumeId), nil)
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
func (v *Volume) List(ctx context.Context, path string, opts *VolumeListOpts) ([]VolumeEntryStat, error) {
	query := url.Values{}
	if opts != nil && opts.Depth != nil {
		query.Set("depth", strconv.Itoa(*opts.Depth))
	}

	var clientOpts *VolumeApiOpts
	if opts != nil {
		clientOpts = &opts.VolumeApiOpts
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

// ReadFile provides the JS/Python-style single-entry read surface. It returns
// text by default, and the return type changes when opts.Format is set.
func (v *Volume) ReadFile(ctx context.Context, path string, opts *VolumeReadOpts) (any, error) {
	format := ReadFileFormatText
	var apiOpts *VolumeApiOpts
	if opts != nil {
		apiOpts = &opts.VolumeApiOpts
		if opts.Format != "" {
			format = opts.Format
		}
	}

	switch format {
	case ReadFileFormatText:
		return v.readFileText(ctx, path, apiOpts)
	case ReadFileFormatBytes:
		return v.readFileBytes(ctx, path, apiOpts)
	case ReadFileFormatBlob:
		data, err := v.readFileBytes(ctx, path, apiOpts)
		if err != nil {
			return nil, err
		}
		return Blob(data), nil
	case ReadFileFormatStream:
		return v.readFileStream(ctx, path, apiOpts)
	default:
		return nil, &shared.InvalidArgumentError{SandboxError: shared.SandboxError{Message: fmt.Sprintf("Unsupported read format %s", format)}}
	}
}

func (v *Volume) readFileBytes(ctx context.Context, path string, opts *VolumeApiOpts) ([]byte, error) {
	body, err := v.readFileStream(ctx, path, opts)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return io.ReadAll(body)
}

func (v *Volume) readFileStream(ctx context.Context, path string, opts *VolumeApiOpts) (io.ReadCloser, error) {
	query := url.Values{}
	client := v.resolveClient(opts)
	reqPath := v.volumeContentPath("file", path, query)
	reqUrl := client.config.ApiUrl + reqPath
	reqCtx, cancel := requestContextWithSignal(ctx, client.config.Signal, fileRequestTimeout(opts))

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, reqUrl, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.config.Token)
	for k, val := range client.config.Headers {
		req.Header.Set(k, val)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		return nil, wrapContentVolumeError(&volumeApiError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}, fmt.Sprintf("Path %s not found", path))
	}

	return cancelReadCloser{ReadCloser: resp.Body, cancel: cancel}, nil
}

func (v *Volume) readFileText(ctx context.Context, path string, opts *VolumeApiOpts) (string, error) {
	data, err := v.readFileBytes(ctx, path, opts)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func normalizeVolumeWriteData(path string, data any) (io.Reader, error) {
	if data == nil {
		return nil, &shared.InvalidArgumentError{SandboxError: shared.SandboxError{Message: fmt.Sprintf("Unsupported data type for file %s", path)}}
	}

	switch v := data.(type) {
	case string:
		return bytes.NewBufferString(v), nil
	case []byte:
		return bytes.NewReader(v), nil
	case Blob:
		return v.Reader(), nil
	case io.Reader:
		if isNilVolumeWriteData(v) {
			return nil, &shared.InvalidArgumentError{SandboxError: shared.SandboxError{Message: fmt.Sprintf("Unsupported data type for file %s", path)}}
		}
		return v, nil
	default:
		return nil, &shared.InvalidArgumentError{SandboxError: shared.SandboxError{Message: fmt.Sprintf("Unsupported data type for file %s", path)}}
	}
}

func isNilVolumeWriteData(data any) bool {
	if data == nil {
		return true
	}

	value := reflect.ValueOf(data)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// WriteFile writes data to a file at the given path.
func (v *Volume) WriteFile(ctx context.Context, path string, data any, opts *VolumeWriteOptions) (*VolumeEntryStat, error) {
	query := queryFromVolumeWriteOpts(opts)
	apiOpts := volumeWriteOptsToApiOpts(opts)
	client := v.resolveClient(apiOpts)
	reqPath := v.volumeContentPath("file", path, query)
	reqUrl := client.config.ApiUrl + reqPath
	reqCtx, cancel := requestContextWithSignal(ctx, client.config.Signal, fileRequestTimeout(apiOpts))
	defer cancel()

	reader, err := normalizeVolumeWriteData(path, data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPut, reqUrl, reader)
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
	if opts == nil {
		opts = &VolumeApiOpts{}
	}

	merged := &VolumeApiOpts{
		Token:  v.Token,
		Domain: v.Domain,
		Debug:  v.Debug,
	}

	if opts.Token != "" {
		merged.Token = opts.Token
	}
	if opts.Domain != "" {
		merged.Domain = opts.Domain
	}
	if opts.Debug != nil {
		merged.Debug = opts.Debug
	}
	if opts.ApiUrl != "" {
		merged.ApiUrl = opts.ApiUrl
	}
	if opts.RequestTimeoutMs != nil {
		merged.RequestTimeoutMs = opts.RequestTimeoutMs
	}
	if opts.Signal != nil {
		merged.Signal = opts.Signal
	}
	if opts.Headers != nil {
		merged.Headers = opts.Headers
	}
	if opts.Logger != nil {
		merged.Logger = opts.Logger
	}
	if opts.Proxy != "" {
		merged.Proxy = opts.Proxy
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
	if opts.Force != nil {
		query.Set("force", strconv.FormatBool(*opts.Force))
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
		Signal:           opts.Signal,
		Logger:           opts.Logger,
		Headers:          opts.Headers,
		Proxy:            opts.Proxy,
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
