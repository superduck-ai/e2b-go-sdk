package volume

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/api"
	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

const fileTimeoutMs = 3_600_000

type VolumeApiOpts struct {
	Token            string
	Domain           string
	Debug            *bool
	ApiUrl           string
	RequestTimeoutMs *int
	Signal           context.Context
	Logger           api.Logger
	Headers          map[string]string
	Proxy            string
}

type VolumeReadOpts struct {
	VolumeApiOpts
	Format ReadFileFormat
}

type VolumeListOpts struct {
	VolumeApiOpts
	Depth *int
}

type VolumeConnectionConfig struct {
	Domain           string
	Debug            bool
	ApiUrl           string
	Token            string
	RequestTimeoutMs *int
	Signal           context.Context
	Logger           api.Logger
	Headers          map[string]string
	Proxy            string
}

func NewVolumeConnectionConfig(opts *VolumeApiOpts) *VolumeConnectionConfig {
	if opts == nil {
		opts = &VolumeApiOpts{}
	}

	domain := opts.Domain
	if domain == "" {
		domain = os.Getenv("E2B_DOMAIN")
		if domain == "" {
			domain = "e2b.app"
		}
	}
	var debug bool
	if opts.Debug != nil {
		debug = *opts.Debug
	} else if v, err := strconv.ParseBool(os.Getenv("E2B_DEBUG")); err == nil {
		debug = v
	}
	apiUrl := opts.ApiUrl
	if apiUrl == "" {
		apiUrl = os.Getenv("E2B_VOLUME_API_URL")
		if apiUrl == "" {
			apiUrl = os.Getenv("E2B_API_URL")
		}
		if apiUrl == "" {
			if debug {
				apiUrl = "http://localhost:8080"
			} else {
				apiUrl = fmt.Sprintf("https://api.%s", domain)
			}
		}
	}
	headers := map[string]string{}
	for k, v := range api.DefaultHeaders {
		headers[k] = v
	}
	for k, v := range opts.Headers {
		headers[k] = v
	}

	return &VolumeConnectionConfig{
		Domain:           domain,
		Debug:            debug,
		ApiUrl:           apiUrl,
		Token:            opts.Token,
		RequestTimeoutMs: opts.RequestTimeoutMs,
		Signal:           opts.Signal,
		Logger:           opts.Logger,
		Headers:          headers,
		Proxy:            opts.Proxy,
	}
}

type volumeApiClient struct {
	config     *VolumeConnectionConfig
	httpClient *http.Client
}

type volumeApiError struct {
	StatusCode int
	Message    string
}

func (e *volumeApiError) Error() string {
	return fmt.Sprintf("volume API error: %d - %s", e.StatusCode, e.Message)
}

func newVolumeApiClientWithConfig(config *VolumeConnectionConfig) *volumeApiClient {
	return &volumeApiClient{
		config:     config,
		httpClient: shared.NewHTTPClient(0, config.Proxy, config.Logger),
	}
}

func (c *volumeApiClient) Do(ctx context.Context, method, path string, body io.Reader, result interface{}, requestTimeoutMs *int) error {
	url := c.config.ApiUrl + path
	reqCtx, cancel := requestContextWithSignal(ctx, c.config.Signal, requestTimeoutMs)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
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
		return &volumeApiError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}
	if result != nil {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if len(respBody) == 0 {
			return nil
		}
		return json.Unmarshal(respBody, result)
	}
	return nil
}

func requestContext(ctx context.Context, timeoutMs *int) (context.Context, context.CancelFunc) {
	if timeoutMs == nil {
		return ctx, func() {}
	}
	if *timeoutMs == 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, time.Duration(*timeoutMs)*time.Millisecond)
}

func requestContextWithSignal(ctx context.Context, signal context.Context, timeoutMs *int) (context.Context, context.CancelFunc) {
	merged, cancelSignal := shared.MergeContexts(ctx, signal)
	reqCtx, cancelTimeout := requestContext(merged, timeoutMs)
	return reqCtx, func() {
		cancelTimeout()
		cancelSignal()
	}
}
