package e2b

import (
	"context"
	"fmt"
	"os"
	"strconv"
)

const (
	defaultRequestTimeoutMs  = 60000
	defaultSandboxTimeoutMs  = 300000
	keepalivePingIntervalSec = 50
	keepalivePingHeader      = "Keepalive-Ping-Interval"
	envdPort                 = 49983
)

var stableSandboxDomains = map[string]struct{}{
	"e2b.app":         {},
	"e2b.dev":         {},
	"e2b.pro":         {},
	"e2b-staging.dev": {},
}

const defaultUsername = "user"
const sdkVersion = "dev"

type Username = string

type ConnectionOpts struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            *bool
	Signal           context.Context
	RequestTimeoutMs *int
	Logger           Logger
	Headers          map[string]string
	Proxy            string
}

type ConnectionConfig struct {
	Debug            bool
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Logger           Logger
	RequestTimeoutMs int
	ApiKey           string
	AccessToken      string
	Headers          map[string]string
	Proxy            string
}

func NewConnectionConfig(opts *ConnectionOpts) *ConnectionConfig {
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
	}
	if domain == "" {
		domain = "e2b.app"
	}

	debug := boolValue(opts.Debug)
	if !debug {
		if v, err := strconv.ParseBool(os.Getenv("E2B_DEBUG")); err == nil {
			debug = v
		}
	}

	apiUrl := opts.ApiUrl
	if apiUrl == "" {
		apiUrl = os.Getenv("E2B_API_URL")
	}
	if apiUrl == "" && debug {
		apiUrl = "http://localhost:3000"
	}
	if apiUrl == "" {
		apiUrl = fmt.Sprintf("https://api.%s", domain)
	}

	sandboxUrl := opts.SandboxUrl
	if sandboxUrl == "" {
		sandboxUrl = os.Getenv("E2B_SANDBOX_URL")
	}

	requestTimeoutMs := defaultRequestTimeoutMs
	if opts.RequestTimeoutMs != nil {
		requestTimeoutMs = *opts.RequestTimeoutMs
	}

	headers := map[string]string{}
	for k, v := range opts.Headers {
		headers[k] = v
	}
	headers["User-Agent"] = "e2b-go-sdk/" + sdkVersion

	return &ConnectionConfig{
		Debug:            debug,
		Domain:           domain,
		ApiUrl:           apiUrl,
		SandboxUrl:       sandboxUrl,
		Logger:           opts.Logger,
		RequestTimeoutMs: requestTimeoutMs,
		ApiKey:           apiKey,
		AccessToken:      accessToken,
		Headers:          headers,
		Proxy:            opts.Proxy,
	}
}

func intPtr(value int) *int {
	return &value
}

func boolRef(value bool) *bool {
	return &value
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func trueBoolRef(value bool) *bool {
	if !value {
		return nil
	}
	return boolRef(true)
}

func (c *ConnectionConfig) GetSandboxUrl(sandboxId string, sandboxDomain string, envdPort int) string {
	if c.SandboxUrl != "" {
		return c.SandboxUrl
	}
	if c.Debug {
		return fmt.Sprintf("http://localhost:%d", envdPort)
	}
	if sandboxDomain == "" {
		sandboxDomain = c.Domain
	}
	if _, ok := stableSandboxDomains[sandboxDomain]; ok {
		return fmt.Sprintf("https://sandbox.%s", sandboxDomain)
	}
	return fmt.Sprintf("https://%s", c.GetHost(sandboxId, envdPort, sandboxDomain))
}

func (c *ConnectionConfig) GetSandboxDirectUrl(sandboxId string, sandboxDomain string, envdPort int) string {
	if c.SandboxUrl != "" {
		return c.SandboxUrl
	}
	if c.Debug {
		return fmt.Sprintf("http://localhost:%d", envdPort)
	}
	return fmt.Sprintf("https://%s", c.GetHost(sandboxId, envdPort, sandboxDomain))
}

func (c *ConnectionConfig) GetHost(sandboxId string, port int, sandboxDomain string) string {
	if c.Debug {
		return fmt.Sprintf("localhost:%d", port)
	}
	if sandboxDomain == "" {
		sandboxDomain = c.Domain
	}
	return fmt.Sprintf("%d-%s.%s", port, sandboxId, sandboxDomain)
}
