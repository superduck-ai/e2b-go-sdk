package shared

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"
)

type HTTPMiddleware func(http.RoundTripper) http.RoundTripper

type transportProfile string

const (
	transportProfileGeneric  transportProfile = "generic"
	transportProfileAPI      transportProfile = "api"
	transportProfileEnvdREST transportProfile = "envd_rest"
	transportProfileEnvdRPC  transportProfile = "envd_rpc"
)

const (
	defaultAPIConnectionLimit      = 100
	defaultAPIInflightLimit        = 1000
	defaultEnvdRESTConnectionLimit = 10
	defaultEnvdRESTInflightLimit   = 2000
	defaultEnvdRPCConnectionLimit  = 200
	defaultEnvdRPCInflightLimit    = 2000
)

type transportOptions struct {
	profile         transportProfile
	proxy           string
	http2           bool
	connectionLimit int
	inflightLimit   int
}

type transportCacheKey struct {
	profile         transportProfile
	proxy           string
	http2           bool
	connectionLimit int
	inflightLimit   int
}

type cachedTransport struct {
	base         *http.Transport
	roundTripper http.RoundTripper
}

var (
	transportCacheMu sync.Mutex
	transportCache   = map[transportCacheKey]*cachedTransport{}
)

func CreateApiLogger(logger Logger) HTTPMiddleware {
	return createHTTPLogger(logger)
}

func CreateRpcLogger(logger Logger) HTTPMiddleware {
	return createHTTPLogger(logger)
}

func NewHTTPClient(timeout time.Duration, proxy string, logger Logger) *http.Client {
	return newConfiguredHTTPClient(
		timeout,
		logger,
		CreateApiLogger,
		transportOptions{
			profile: transportProfileGeneric,
			proxy:   proxy,
			http2:   true,
		},
	)
}

func NewAPIHTTPClient(timeout time.Duration, proxy string, logger Logger) (*http.Client, error) {
	connectionLimit, err := getAPIConnectionLimit()
	if err != nil {
		return nil, err
	}
	inflightLimit, err := getAPIInflightLimit()
	if err != nil {
		return nil, err
	}

	return newConfiguredHTTPClient(
		timeout,
		logger,
		CreateApiLogger,
		transportOptions{
			profile:         transportProfileAPI,
			proxy:           proxy,
			http2:           true,
			connectionLimit: connectionLimit,
			inflightLimit:   inflightLimit,
		},
	), nil
}

func NewEnvdRESTHTTPClient(timeout time.Duration, proxy string, logger Logger) *http.Client {
	inflightLimit, err := getEnvdInflightLimit()
	if err != nil {
		return newErrorHTTPClient(timeout, logger, CreateRpcLogger, err)
	}

	return newConfiguredHTTPClient(
		timeout,
		logger,
		CreateRpcLogger,
		transportOptions{
			profile:         transportProfileEnvdREST,
			proxy:           proxy,
			http2:           true,
			connectionLimit: defaultEnvdRESTConnectionLimit,
			inflightLimit:   inflightLimit,
		},
	)
}

func NewEnvdRPCHTTPClient(timeout time.Duration, proxy string, logger Logger) *http.Client {
	connectionLimit, err := getEnvdRPCConnectionLimit()
	if err != nil {
		return newErrorHTTPClient(timeout, logger, CreateRpcLogger, err)
	}
	inflightLimit, err := getEnvdRPCInflightLimit()
	if err != nil {
		return newErrorHTTPClient(timeout, logger, CreateRpcLogger, err)
	}

	return newConfiguredHTTPClient(
		timeout,
		logger,
		CreateRpcLogger,
		transportOptions{
			profile:         transportProfileEnvdRPC,
			proxy:           proxy,
			http2:           true,
			connectionLimit: connectionLimit,
			inflightLimit:   inflightLimit,
		},
	)
}

func newConfiguredHTTPClient(
	timeout time.Duration,
	logger Logger,
	loggerFactory func(Logger) HTTPMiddleware,
	options transportOptions,
) *http.Client {
	transport, _ := getOrCreateTransport(options)
	if logger != nil {
		transport = loggerFactory(logger)(transport)
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func newErrorHTTPClient(
	timeout time.Duration,
	logger Logger,
	loggerFactory func(Logger) HTTPMiddleware,
	err error,
) *http.Client {
	var transport http.RoundTripper = errorRoundTripper{err: err}
	if logger != nil {
		transport = loggerFactory(logger)(transport)
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func createHTTPLogger(logger Logger) HTTPMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		if next == nil {
			next = http.DefaultTransport
		}
		return &loggingRoundTripper{
			next:   next,
			logger: logger,
		}
	}
}

func getOrCreateTransport(options transportOptions) (http.RoundTripper, *http.Transport) {
	key := transportCacheKey{
		profile:         options.profile,
		proxy:           options.proxy,
		http2:           options.http2,
		connectionLimit: options.connectionLimit,
		inflightLimit:   options.inflightLimit,
	}

	transportCacheMu.Lock()
	if cached := transportCache[key]; cached != nil {
		transportCacheMu.Unlock()
		return cached.roundTripper, cached.base
	}
	transportCacheMu.Unlock()

	base := baseTransport(options.proxy, options.connectionLimit, options.http2)

	var transport http.RoundTripper = base
	if options.inflightLimit > 0 {
		transport = &limitingRoundTripper{
			next:    base,
			limiter: newInflightLimiter(options.inflightLimit),
		}
	}

	cached := &cachedTransport{
		base:         base,
		roundTripper: transport,
	}

	transportCacheMu.Lock()
	defer transportCacheMu.Unlock()
	if existing := transportCache[key]; existing != nil {
		return existing.roundTripper, existing.base
	}
	transportCache[key] = cached
	return cached.roundTripper, cached.base
}

func baseTransport(proxy string, connectionLimit int, http2 bool) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if connectionLimit > 0 {
		transport.MaxConnsPerHost = connectionLimit
		transport.MaxIdleConnsPerHost = connectionLimit
		if transport.MaxIdleConns < connectionLimit {
			transport.MaxIdleConns = connectionLimit
		}
	}

	if !http2 {
		transport.ForceAttemptHTTP2 = false
		transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	}

	if proxy == "" {
		return transport
	}

	proxyURL, err := url.Parse(proxy)
	if err != nil || proxyURL.Scheme == "" || proxyURL.Host == "" {
		transport.Proxy = invalidProxyFunc(proxy, err)
		return transport
	}

	transport.Proxy = http.ProxyURL(proxyURL)
	return transport
}

func invalidProxyFunc(proxy string, parseErr error) func(*http.Request) (*url.URL, error) {
	return func(*http.Request) (*url.URL, error) {
		if parseErr != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", proxy, parseErr)
		}
		return nil, fmt.Errorf("invalid proxy URL %q", proxy)
	}
}

type loggingRoundTripper struct {
	next   http.RoundTripper
	logger Logger
}

func (t *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.logger != nil {
		t.logger.Info("Request:", req.Method, req.URL.String())
	}

	resp, err := t.next.RoundTrip(req)
	if err != nil {
		if t.logger != nil {
			t.logger.Error("Request error:", err)
		}
		return nil, err
	}

	if t.logger != nil {
		if resp.StatusCode >= http.StatusBadRequest {
			t.logger.Error("Response:", resp.StatusCode, resp.Status)
		} else {
			t.logger.Info("Response:", resp.StatusCode, resp.Status)
		}
	}

	return resp, nil
}

func (t *loggingRoundTripper) CloseIdleConnections() {
	closeIdleConnections(t.next)
}

type limitingRoundTripper struct {
	next    http.RoundTripper
	limiter *inflightLimiter
}

func (t *limitingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.limiter.Acquire(req.Context()); err != nil {
		return nil, err
	}
	defer t.limiter.Release()

	return t.next.RoundTrip(req)
}

func (t *limitingRoundTripper) CloseIdleConnections() {
	closeIdleConnections(t.next)
}

type errorRoundTripper struct {
	err error
}

func (t errorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, t.err
}

type closeIdler interface {
	CloseIdleConnections()
}

func closeIdleConnections(next http.RoundTripper) {
	if closeIdler, ok := next.(closeIdler); ok {
		closeIdler.CloseIdleConnections()
	}
}

type inflightLimiter struct {
	max    int
	mu     sync.Mutex
	active int
	queue  []*inflightWaiter
}

type inflightWaiter struct {
	ready    chan struct{}
	granted  bool
	canceled bool
}

func newInflightLimiter(max int) *inflightLimiter {
	return &inflightLimiter{max: max}
}

func (l *inflightLimiter) Acquire(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	l.mu.Lock()
	if l.active < l.max {
		l.active++
		l.mu.Unlock()
		return nil
	}

	waiter := &inflightWaiter{ready: make(chan struct{})}
	l.queue = append(l.queue, waiter)
	l.mu.Unlock()

	select {
	case <-waiter.ready:
		return nil
	case <-ctx.Done():
		l.mu.Lock()
		defer l.mu.Unlock()
		if waiter.granted {
			return nil
		}
		waiter.canceled = true
		return ctx.Err()
	}
}

func (l *inflightLimiter) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.active > 0 {
		l.active--
	}

	for len(l.queue) > 0 {
		waiter := l.queue[0]
		l.queue = l.queue[1:]
		if waiter.canceled {
			continue
		}

		waiter.granted = true
		l.active++
		close(waiter.ready)
		return
	}
}

func parseIntEnv(name string, defaultValue int) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: expected an integer", name, raw)
	}
	return parsed, nil
}

func parsePositiveIntEnv(name string, defaultValue int) (int, error) {
	parsed, err := parseIntEnv(name, defaultValue)
	if err != nil {
		return 0, err
	}
	if parsed < 1 {
		return 0, fmt.Errorf("invalid %s=%d: expected a positive integer", name, parsed)
	}
	return parsed, nil
}

func parseInflightLimitEnv(name string, defaultValue int) (int, error) {
	parsed, err := parseIntEnv(name, defaultValue)
	if err != nil {
		return 0, err
	}
	if parsed < 0 {
		return 0, fmt.Errorf("invalid %s=%d: expected a non-negative integer (use 0 to disable the cap)", name, parsed)
	}
	return parsed, nil
}

func getAPIConnectionLimit() (int, error) {
	return parsePositiveIntEnv("E2B_API_CONNECTIONS", defaultAPIConnectionLimit)
}

func getAPIInflightLimit() (int, error) {
	return parseInflightLimitEnv("E2B_API_INFLIGHT_REQUESTS", defaultAPIInflightLimit)
}

func getEnvdRPCConnectionLimit() (int, error) {
	return parsePositiveIntEnv("E2B_ENVD_RPC_CONNECTIONS", defaultEnvdRPCConnectionLimit)
}

func getEnvdInflightLimit() (int, error) {
	return parseInflightLimitEnv("E2B_ENVD_INFLIGHT_REQUESTS", defaultEnvdRESTInflightLimit)
}

func getEnvdRPCInflightLimit() (int, error) {
	return parseInflightLimitEnv("E2B_ENVD_RPC_INFLIGHT_REQUESTS", defaultEnvdRPCInflightLimit)
}

func resetTransportCacheForTests() {
	transportCacheMu.Lock()
	defer transportCacheMu.Unlock()
	transportCache = map[transportCacheKey]*cachedTransport{}
}

var _ http.RoundTripper = (*loggingRoundTripper)(nil)
var _ http.RoundTripper = (*limitingRoundTripper)(nil)
