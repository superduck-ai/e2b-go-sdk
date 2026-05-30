package shared

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTransportCacheReturnsDistinctTransportForHTTP2OptOut(t *testing.T) {
	resetTransportCacheForTests()

	rt1, base1 := getOrCreateTransport(transportOptions{
		profile:         transportProfileAPI,
		http2:           true,
		connectionLimit: 100,
		inflightLimit:   1000,
	})
	rt2, base2 := getOrCreateTransport(transportOptions{
		profile:         transportProfileAPI,
		http2:           false,
		connectionLimit: 100,
		inflightLimit:   1000,
	})
	rt3, base3 := getOrCreateTransport(transportOptions{
		profile:         transportProfileAPI,
		http2:           true,
		connectionLimit: 100,
		inflightLimit:   1000,
	})

	if rt1 == nil || rt2 == nil || rt3 == nil {
		t.Fatal("expected cached round trippers")
	}
	if base1 == nil || base2 == nil || base3 == nil {
		t.Fatal("expected cached base transports")
	}
	if base1 != base3 {
		t.Fatal("expected identical transport options to reuse the same base transport")
	}
	if base1 == base2 {
		t.Fatal("expected HTTP/2 opt-out to use a distinct base transport")
	}
	if !base1.ForceAttemptHTTP2 {
		t.Fatal("expected default transport to attempt HTTP/2")
	}
	if base2.ForceAttemptHTTP2 {
		t.Fatal("expected HTTP/2 opt-out transport to disable ForceAttemptHTTP2")
	}
	if base2.TLSNextProto == nil {
		t.Fatal("expected HTTP/2 opt-out transport to override TLSNextProto")
	}
}

func TestTransportConnectionLimitAppliesToBaseTransport(t *testing.T) {
	resetTransportCacheForTests()

	_, base := getOrCreateTransport(transportOptions{
		profile:         transportProfileEnvdRPC,
		http2:           true,
		connectionLimit: 200,
	})

	if base.MaxConnsPerHost != 200 {
		t.Fatalf("expected MaxConnsPerHost=200, got %d", base.MaxConnsPerHost)
	}
	if base.MaxIdleConnsPerHost != 200 {
		t.Fatalf("expected MaxIdleConnsPerHost=200, got %d", base.MaxIdleConnsPerHost)
	}
	if base.MaxIdleConns < 200 {
		t.Fatalf("expected MaxIdleConns >= 200, got %d", base.MaxIdleConns)
	}
}

func TestAPITransportEnvParsingMatchesJSRules(t *testing.T) {
	t.Setenv("E2B_API_CONNECTIONS", "bogus")
	if _, err := getAPIConnectionLimit(); err == nil || !strings.Contains(err.Error(), "E2B_API_CONNECTIONS") {
		t.Fatalf("expected malformed E2B_API_CONNECTIONS error, got %v", err)
	}

	t.Setenv("E2B_API_CONNECTIONS", "")
	t.Setenv("E2B_API_INFLIGHT_REQUESTS", "0")
	inflight, err := getAPIInflightLimit()
	if err != nil {
		t.Fatalf("expected disabled inflight env to parse, got %v", err)
	}
	if inflight != 0 {
		t.Fatalf("expected inflight limit 0, got %d", inflight)
	}

	t.Setenv("E2B_API_INFLIGHT_REQUESTS", "-5")
	if _, err := getAPIInflightLimit(); err == nil || !strings.Contains(err.Error(), "E2B_API_INFLIGHT_REQUESTS=-5") {
		t.Fatalf("expected negative inflight env error, got %v", err)
	}
}

func TestEnvdTransportEnvParsingMatchesJSRules(t *testing.T) {
	t.Setenv("E2B_ENVD_RPC_CONNECTIONS", "200")
	limit, err := getEnvdRPCConnectionLimit()
	if err != nil {
		t.Fatalf("expected rpc connection env to parse, got %v", err)
	}
	if limit != 200 {
		t.Fatalf("expected rpc connection limit 200, got %d", limit)
	}

	t.Setenv("E2B_ENVD_INFLIGHT_REQUESTS", "0")
	t.Setenv("E2B_ENVD_RPC_INFLIGHT_REQUESTS", "0")

	restLimit, err := getEnvdInflightLimit()
	if err != nil {
		t.Fatalf("expected envd inflight env to parse, got %v", err)
	}
	rpcLimit, err := getEnvdRPCInflightLimit()
	if err != nil {
		t.Fatalf("expected envd rpc inflight env to parse, got %v", err)
	}
	if restLimit != 0 || rpcLimit != 0 {
		t.Fatalf("expected disabled envd inflight limits, got rest=%d rpc=%d", restLimit, rpcLimit)
	}

	t.Setenv("E2B_ENVD_INFLIGHT_REQUESTS", "-1")
	if _, err := getEnvdInflightLimit(); err == nil || !strings.Contains(err.Error(), "E2B_ENVD_INFLIGHT_REQUESTS=-1") {
		t.Fatalf("expected negative envd inflight error, got %v", err)
	}

	t.Setenv("E2B_ENVD_RPC_INFLIGHT_REQUESTS", "-5")
	if _, err := getEnvdRPCInflightLimit(); err == nil || !strings.Contains(err.Error(), "E2B_ENVD_RPC_INFLIGHT_REQUESTS=-5") {
		t.Fatalf("expected negative envd rpc inflight error, got %v", err)
	}
}

func TestLimitingRoundTripperQueuesRequestsOverCapAndReleasesOnResponse(t *testing.T) {
	gate := make(chan struct{})
	firstStarted := make(chan struct{})
	startedSecond := make(chan struct{}, 1)

	rt := &limitingRoundTripper{
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if strings.HasSuffix(req.URL.Path, "/first") {
				close(firstStarted)
				<-gate
				return testResponse(req), nil
			}
			startedSecond <- struct{}{}
			return testResponse(req), nil
		}),
		limiter: newInflightLimiter(1),
	}

	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)

	go func() {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/first", nil)
		_, err := rt.RoundTrip(req)
		firstDone <- err
	}()

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first request to acquire the slot")
	}

	go func() {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/second", nil)
		_, err := rt.RoundTrip(req)
		secondDone <- err
	}()

	select {
	case <-startedSecond:
		t.Fatal("expected second request to remain queued while the first request is active")
	case <-time.After(50 * time.Millisecond):
	}

	close(gate)

	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("unexpected first request error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first request")
	}

	select {
	case <-startedSecond:
	case <-time.After(time.Second):
		t.Fatal("expected second request to start after the first request released its slot")
	}

	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("unexpected second request error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for second request")
	}
}

func TestLimitingRoundTripperReleasesWhenUnderlyingTransportReturnsError(t *testing.T) {
	var mu sync.Mutex
	calls := 0

	rt := &limitingRoundTripper{
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if calls == 1 {
				return nil, errors.New("boom")
			}
			return testResponse(req), nil
		}),
		limiter: newInflightLimiter(1),
	}

	req1, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/a", nil)
	if _, err := rt.RoundTrip(req1); err == nil || err.Error() != "boom" {
		t.Fatalf("expected first request to fail with boom, got %v", err)
	}

	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/b", nil)
	if _, err := rt.RoundTrip(req2); err != nil {
		t.Fatalf("expected slot release after error, got %v", err)
	}
}

func TestLimitingRoundTripperCancelsQueuedRequestsWhenContextCanceled(t *testing.T) {
	gate := make(chan struct{})
	firstStarted := make(chan struct{})

	rt := &limitingRoundTripper{
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			close(firstStarted)
			<-gate
			return testResponse(req), nil
		}),
		limiter: newInflightLimiter(1),
	}

	firstDone := make(chan error, 1)
	go func() {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/first", nil)
		_, err := rt.RoundTrip(req)
		firstDone <- err
	}()

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first request to acquire the slot")
	}

	queuedCtx, cancel := context.WithCancel(context.Background())
	queuedDone := make(chan error, 1)
	go func() {
		req, _ := http.NewRequestWithContext(queuedCtx, http.MethodGet, "https://example.com/queued", nil)
		_, err := rt.RoundTrip(req)
		queuedDone <- err
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-queuedDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected queued request cancellation, got %T %v", err, err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for queued request cancellation")
	}

	close(gate)

	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("unexpected first request error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first request completion")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testResponse(req *http.Request) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("ok")),
		Request:    req,
		Header:     make(http.Header),
	}
}
