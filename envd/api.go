package envd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// EnvdApiError holds the status code and message from an envd API error response.
type EnvdApiError struct {
	StatusCode int
	Body       string
}

func (e *EnvdApiError) Error() string {
	return fmt.Sprintf("%d: %s", e.StatusCode, e.Body)
}

type SandboxError struct {
	Message string
}

func (e *SandboxError) Error() string { return e.Message }

type TimeoutError struct{ SandboxError }
type InvalidArgumentError struct{ SandboxError }
type NotEnoughSpaceError struct{ SandboxError }
type NotFoundError struct{ SandboxError }

type AuthenticationError struct {
	Message string
}

func (e *AuthenticationError) Error() string { return e.Message }

// EnvdApiClient wraps HTTP client for envd API running inside the sandbox.
type EnvdApiClient struct {
	BaseUrl    string
	Version    string
	HttpClient *http.Client
	Headers    map[string]string
}

func NewEnvdApiClient(baseUrl string, accessToken string, headers map[string]string, requestTimeoutMs int) *EnvdApiClient {
	allHeaders := make(map[string]string)
	if accessToken != "" {
		allHeaders["X-Access-Token"] = accessToken
	}
	for k, v := range headers {
		allHeaders[k] = v
	}

	return &EnvdApiClient{
		BaseUrl:    baseUrl,
		HttpClient: &http.Client{Timeout: time.Duration(requestTimeoutMs) * time.Millisecond},
		Headers:    allHeaders,
	}
}

func (c *EnvdApiClient) doRequest(req *http.Request) (*http.Response, error) {
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	return c.HttpClient.Do(req)
}

// Health checks envd health and caches the version.
func (c *EnvdApiClient) Health(ctx context.Context) (*HealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseUrl+"/health", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if apiErr := HandleEnvdApiError(resp.StatusCode, body); apiErr != nil {
		return nil, apiErr
	}
	var result HealthResponse
	if len(body) > 0 {
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}
	}
	if result.Version != "" {
		c.Version = result.Version
	}
	return &result, nil
}

// Init initializes the sandbox environment with the given environment variables.
func (c *EnvdApiClient) Init(ctx context.Context, initReq *InitRequest) error {
	data, err := json.Marshal(initReq)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseUrl+"/init", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return HandleEnvdApiError(resp.StatusCode, body)
}

// Metrics retrieves sandbox resource metrics.
func (c *EnvdApiClient) Metrics(ctx context.Context) (*MetricsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseUrl+"/metrics", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if apiErr := HandleEnvdApiError(resp.StatusCode, body); apiErr != nil {
		return nil, apiErr
	}
	var result MetricsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DownloadFile downloads a file from the sandbox filesystem. Returns a ReadCloser with raw bytes.
func (c *EnvdApiClient) DownloadFile(ctx context.Context, path string, user string) (io.ReadCloser, error) {
	u := fmt.Sprintf("%s/files?path=%s&username=%s", c.BaseUrl, url.QueryEscape(path), url.QueryEscape(user))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, HandleEnvdApiError(resp.StatusCode, body)
	}
	return resp.Body, nil
}

// UploadFile uploads a file to the sandbox filesystem.
func (c *EnvdApiClient) UploadFile(ctx context.Context, path string, user string, data io.Reader, contentType string) error {
	u := fmt.Sprintf("%s/files?path=%s&username=%s", c.BaseUrl, url.QueryEscape(path), url.QueryEscape(user))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, data)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return HandleEnvdApiError(resp.StatusCode, body)
}

// HandleEnvdApiError maps HTTP status codes to an EnvdApiError.
// Returns nil for successful status codes (< 400).
func HandleEnvdApiError(statusCode int, body []byte) error {
	if statusCode < 400 {
		return nil
	}

	message := extractEnvdErrorMessage(body)

	switch statusCode {
	case http.StatusBadRequest:
		return &InvalidArgumentError{SandboxError{Message: message}}
	case http.StatusUnauthorized:
		return &AuthenticationError{Message: message}
	case http.StatusNotFound:
		return &NotFoundError{SandboxError{Message: message}}
	case http.StatusTooManyRequests:
		return &SandboxError{Message: message + ": The requests are being rate limited."}
	case http.StatusBadGateway:
		return &TimeoutError{SandboxError{Message: formatSandboxTimeoutMessage(message)}}
	case http.StatusInsufficientStorage:
		return &NotEnoughSpaceError{SandboxError{Message: message}}
	default:
		return &EnvdApiError{StatusCode: statusCode, Body: message}
	}
}

func extractEnvdErrorMessage(body []byte) string {
	message := strings.TrimSpace(string(body))
	if message == "" {
		return ""
	}

	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.Message != "" {
		return payload.Message
	}

	return message
}

func formatSandboxTimeoutMessage(message string) string {
	if message == "" {
		message = "Sandbox timed out"
	}
	return fmt.Sprintf("%s: This error is likely due to sandbox timeout. You can modify the sandbox timeout by passing 'timeoutMs' when starting the sandbox or calling '.setTimeout' on the sandbox with the desired timeout.", message)
}
