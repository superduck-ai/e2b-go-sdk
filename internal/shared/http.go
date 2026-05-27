package shared

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type HTTPMiddleware func(http.RoundTripper) http.RoundTripper

func CreateApiLogger(logger Logger) HTTPMiddleware {
	return createHTTPLogger(logger)
}

func CreateRpcLogger(logger Logger) HTTPMiddleware {
	return createHTTPLogger(logger)
}

func NewHTTPClient(timeout time.Duration, proxy string, logger Logger) *http.Client {
	var transport http.RoundTripper
	if proxy != "" {
		transport = proxyTransport(proxy)
	}
	if logger != nil {
		transport = CreateApiLogger(logger)(transport)
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
		return loggingRoundTripper{
			next:   next,
			logger: logger,
		}
	}
}

func proxyTransport(proxy string) http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	proxyURL, err := url.Parse(proxy)
	if err != nil || proxyURL.Scheme == "" || proxyURL.Host == "" {
		transport.Proxy = func(*http.Request) (*url.URL, error) {
			if err != nil {
				return nil, fmt.Errorf("invalid proxy URL %q: %w", proxy, err)
			}
			return nil, fmt.Errorf("invalid proxy URL %q", proxy)
		}
		return transport
	}
	transport.Proxy = http.ProxyURL(proxyURL)
	return transport
}

type loggingRoundTripper struct {
	next   http.RoundTripper
	logger Logger
}

func (t loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
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
