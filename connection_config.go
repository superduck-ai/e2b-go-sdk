package e2b

import (
	"fmt"
	"os"
	"strconv"
)

const (
	RequestTimeoutMs         = 60000
	DefaultSandboxTimeoutMs  = 300000
	KeepalivePingIntervalSec = 50
	KeepalivePingHeader      = "Keepalive-Ping-Interval"
	EnvdPort                 = 49983
)

const DefaultUsername = "user"

type ConnectionOpts struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            bool
	RequestTimeoutMs int
	Logger           Logger
	Headers          map[string]string
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

	apiUrl := opts.ApiUrl
	if apiUrl == "" {
		apiUrl = os.Getenv("E2B_API_URL")
	}

	sandboxUrl := opts.SandboxUrl
	if sandboxUrl == "" {
		sandboxUrl = os.Getenv("E2B_SANDBOX_URL")
	}

	debug := opts.Debug
	if !debug {
		if v, err := strconv.ParseBool(os.Getenv("E2B_DEBUG")); err == nil {
			debug = v
		}
	}

	requestTimeoutMs := opts.RequestTimeoutMs
	if requestTimeoutMs == 0 {
		requestTimeoutMs = RequestTimeoutMs
	}

	return &ConnectionConfig{
		Debug:            debug,
		Domain:           domain,
		ApiUrl:           apiUrl,
		SandboxUrl:       sandboxUrl,
		Logger:           opts.Logger,
		RequestTimeoutMs: requestTimeoutMs,
		ApiKey:           apiKey,
		AccessToken:      accessToken,
		Headers:          opts.Headers,
	}
}

func (c *ConnectionConfig) GetSandboxUrl(sandboxId string, sandboxDomain string, envdPort int) string {
	if c.SandboxUrl != "" {
		return c.SandboxUrl
	}
	return fmt.Sprintf("https://%d-%s.%s", envdPort, sandboxId, sandboxDomain)
}

func (c *ConnectionConfig) GetHost(sandboxId string, port int, sandboxDomain string) string {
	if port == 0 {
		return fmt.Sprintf("%s.%s", sandboxId, sandboxDomain)
	}
	return fmt.Sprintf("%d-%s.%s", port, sandboxId, sandboxDomain)
}
