package volume

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	e2b "github.com/e2b-dev/e2b-go-sdk"
)

const FileTimeoutMs = 3_600_000

type VolumeApiOpts struct {
	Token            string
	Domain           string
	Debug            bool
	ApiUrl           string
	RequestTimeoutMs int
	Logger           e2b.Logger
	Headers          map[string]string
}

type VolumeConnectionConfig struct {
	ApiUrl           string
	Token            string
	RequestTimeoutMs int
	Logger           e2b.Logger
	Headers          map[string]string
}

func NewVolumeConnectionConfig(opts *VolumeApiOpts) *VolumeConnectionConfig {
	domain := opts.Domain
	if domain == "" {
		domain = os.Getenv("E2B_DOMAIN")
		if domain == "" {
			domain = "e2b.app"
		}
	}
	apiUrl := opts.ApiUrl
	if apiUrl == "" {
		apiUrl = os.Getenv("E2B_VOLUME_API_URL")
		if apiUrl == "" {
			apiUrl = fmt.Sprintf("https://volumes.%s", domain)
		}
	}
	timeout := opts.RequestTimeoutMs
	if timeout == 0 {
		timeout = FileTimeoutMs
	}
	return &VolumeConnectionConfig{ApiUrl: apiUrl, Token: opts.Token, RequestTimeoutMs: timeout, Logger: opts.Logger, Headers: opts.Headers}
}

func (c *VolumeConnectionConfig) GetTimeout() time.Duration {
	return time.Duration(c.RequestTimeoutMs) * time.Millisecond
}

type VolumeApiClient struct {
	config     *VolumeConnectionConfig
	httpClient *http.Client
}

func NewVolumeApiClient(config *VolumeConnectionConfig) *VolumeApiClient {
	return &VolumeApiClient{
		config:     config,
		httpClient: &http.Client{Timeout: config.GetTimeout()},
	}
}

func (c *VolumeApiClient) Do(ctx context.Context, method, path string, body io.Reader, result interface{}) error {
	url := c.config.ApiUrl + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.config.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("volume API error: %d - %s", resp.StatusCode, string(respBody))
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}
