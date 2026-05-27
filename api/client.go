package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

// Logger is a simple logging interface.
type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
}

// ClientConfig holds the configuration needed to create an ApiClient.
type ClientConfig struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	RequestTimeoutMs int
	Headers          map[string]string
	Logger           Logger
	Proxy            string
}

type ApiClient struct {
	BaseUrl    string
	HttpClient *http.Client
	Headers    map[string]string
	Logger     Logger
}

type ApiClientOption func(*apiClientOptions)

type apiClientOptions struct {
	RequireAccessToken bool
	RequireApiKey      bool
}

func WithRequireApiKey() ApiClientOption {
	return func(o *apiClientOptions) {
		o.RequireApiKey = true
	}
}

func WithRequireAccessToken() ApiClientOption {
	return func(o *apiClientOptions) {
		o.RequireAccessToken = true
	}
}

func NewApiClient(config *ClientConfig, opts ...ApiClientOption) (*ApiClient, error) {
	options := &apiClientOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.RequireApiKey && config.ApiKey == "" {
		return nil, &AuthenticationError{Message: "API key is required, please visit the Team tab at https://e2b.dev/dashboard to get your API key. You can either set the environment variable `E2B_API_KEY` or you can pass it directly to the sandbox like Sandbox.create({ apiKey: 'e2b_...' })"}
	}

	if options.RequireAccessToken && config.AccessToken == "" {
		return nil, &AuthenticationError{Message: "Access token is required, please visit the Personal tab at https://e2b.dev/dashboard to get your access token. You can set the environment variable `E2B_ACCESS_TOKEN` or pass the `accessToken` in options."}
	}

	baseUrl := config.ApiUrl
	if baseUrl == "" {
		baseUrl = fmt.Sprintf("https://api.%s", config.Domain)
	}

	headers := make(map[string]string)
	for k, v := range DefaultHeaders {
		headers[k] = v
	}
	if config.ApiKey != "" {
		headers["X-API-Key"] = config.ApiKey
	}
	if config.AccessToken != "" {
		headers["Authorization"] = "Bearer " + config.AccessToken
	}
	for k, v := range config.Headers {
		headers[k] = v
	}

	client := shared.NewHTTPClient(time.Duration(config.RequestTimeoutMs)*time.Millisecond, config.Proxy, config.Logger)

	return &ApiClient{
		BaseUrl:    baseUrl,
		HttpClient: client,
		Headers:    headers,
		Logger:     config.Logger,
	}, nil
}

// Error types

type AuthenticationError struct {
	Message string
}

func (e *AuthenticationError) Error() string { return e.Message }

type RateLimitError struct {
	Message string
}

func (e *RateLimitError) Error() string { return e.Message }

type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string { return e.Message }

type ApiError struct {
	StatusCode int
	Message    string
}

func (e *ApiError) Error() string { return fmt.Sprintf("%d: %s", e.StatusCode, e.Message) }

// HandleApiError maps HTTP response status code to the appropriate error type.
func HandleApiError(statusCode int, body []byte) error {
	if statusCode < 400 {
		return nil
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		errResp.Message = string(body)
	}

	switch statusCode {
	case http.StatusUnauthorized:
		message := "Unauthorized, please check your credentials."
		if errResp.Message != "" {
			message += " - " + errResp.Message
		}
		return &AuthenticationError{Message: message}
	case http.StatusTooManyRequests:
		message := "Rate limit exceeded, please try again later"
		if errResp.Message != "" {
			message += " - " + errResp.Message
		}
		return &RateLimitError{Message: message}
	case http.StatusNotFound:
		return &NotFoundError{Message: fmt.Sprintf("%d: %s", statusCode, errResp.Message)}
	default:
		return &ApiError{StatusCode: statusCode, Message: errResp.Message}
	}
}

// Do performs an HTTP request and decodes the JSON response.
// It returns the raw *http.Response (with body already read/closed) so callers
// can inspect headers (e.g. x-next-token for pagination).
func (c *ApiClient) Do(ctx context.Context, method, path string, body interface{}, result interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := c.BaseUrl + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return resp, HandleApiError(resp.StatusCode, respBody)
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return resp, fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return resp, nil
}

// Get performs an HTTP GET request.
func (c *ApiClient) Get(ctx context.Context, path string, result interface{}) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, path, nil, result)
}

// Post performs an HTTP POST request.
func (c *ApiClient) Post(ctx context.Context, path string, body interface{}, result interface{}) (*http.Response, error) {
	return c.Do(ctx, http.MethodPost, path, body, result)
}

// Delete performs an HTTP DELETE request.
func (c *ApiClient) Delete(ctx context.Context, path string, result interface{}) (*http.Response, error) {
	return c.Do(ctx, http.MethodDelete, path, nil, result)
}
