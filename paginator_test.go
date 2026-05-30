package e2b

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPaginatorNextItemsErrorsWhenExhausted(t *testing.T) {
	paginator := newPaginator(func(ctx context.Context, nextToken string) ([]int, string, error) {
		return []int{1}, "", nil
	})

	items, err := paginator.NextItems()
	if err != nil {
		t.Fatalf("unexpected error on first page: %v", err)
	}
	if len(items) != 1 || items[0] != 1 {
		t.Fatalf("unexpected items: %#v", items)
	}
	if paginator.HasNext {
		t.Fatal("expected paginator to be exhausted after page without next token")
	}

	_, err = paginator.NextItems()
	if err == nil {
		t.Fatal("expected error when fetching past the end of paginator")
	}
	if err.Error() != "No more items to fetch" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPaginatorNextItemsDoesNotExposeContextParameter(t *testing.T) {
	paginatorType := reflect.TypeOf(&paginator[int]{})
	method, ok := paginatorType.MethodByName("NextItems")
	if !ok {
		t.Fatal("expected paginator.NextItems to exist")
	}
	if method.Type.NumIn() != 1 {
		t.Fatalf("expected paginator.NextItems to accept no arguments, got %d inputs", method.Type.NumIn()-1)
	}
}

func TestSandboxPaginatorNextItemsAcceptsOptionalPerCallOpts(t *testing.T) {
	method, ok := reflect.TypeOf(&SandboxPaginator{}).MethodByName("NextItems")
	if !ok {
		t.Fatal("expected SandboxPaginator.NextItems to exist")
	}
	if method.Type.NumIn() != 2 {
		t.Fatalf("expected SandboxPaginator.NextItems to accept optional opts, got %d inputs", method.Type.NumIn()-1)
	}
	if got := method.Type.In(1); got != reflect.TypeOf([]*SandboxApiOpts{}) {
		t.Fatalf("expected SandboxPaginator.NextItems variadic opts type []*SandboxApiOpts, got %v", got)
	}
}

func TestSnapshotPaginatorNextItemsAcceptsOptionalPerCallOpts(t *testing.T) {
	method, ok := reflect.TypeOf(&SnapshotPaginator{}).MethodByName("NextItems")
	if !ok {
		t.Fatal("expected SnapshotPaginator.NextItems to exist")
	}
	if method.Type.NumIn() != 2 {
		t.Fatalf("expected SnapshotPaginator.NextItems to accept optional opts, got %d inputs", method.Type.NumIn()-1)
	}
	if got := method.Type.In(1); got != reflect.TypeOf([]*SandboxApiOpts{}) {
		t.Fatalf("expected SnapshotPaginator.NextItems variadic opts type []*SandboxApiOpts, got %v", got)
	}
}

func TestPaginatorNextItemsContextHonorsCancellation(t *testing.T) {
	paginator := newPaginator(func(ctx context.Context, nextToken string) ([]int, string, error) {
		<-ctx.Done()
		return nil, "", ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := paginator.NextItemsContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %T %v", err, err)
	}
}

func TestPaginatorNextItemsContextHonorsInFlightCancellation(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	paginator := newPaginator(func(ctx context.Context, nextToken string) ([]int, string, error) {
		requestStarted <- struct{}{}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-release:
			return []int{1}, "", nil
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		_, err := paginator.NextItemsContext(ctx)
		done <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for paginator request to start")
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %T %v", err, err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for paginator cancellation")
	}

	close(release)
}

func TestSandboxPaginatorNextItemsPerCallOptsOverrideRequestConfig(t *testing.T) {
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		w.Header().Set("x-next-token", "")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	paginator := List(&SandboxListOpts{
		ApiKey:           "e2b_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		apiUrl:           server.URL,
		Domain:           "e2b.app",
		RequestTimeoutMs: intPtr(1000),
	})

	_, err := paginator.NextItems(&SandboxApiOpts{
		ApiKey: "e2b_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	})
	if err != nil {
		t.Fatalf("expected paginator.NextItems override to succeed, got %v", err)
	}
	if gotAPIKey != "e2b_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("expected per-call opts to override API key, got %q", gotAPIKey)
	}
}

func TestSandboxPaginatorNextItemsPerCallCancellationDoesNotPoisonFuturePages(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	allowSuccess := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requestStarted <- struct{}{}:
		default:
		}

		select {
		case <-allowSuccess:
			w.Header().Set("x-next-token", "")
			w.Write([]byte("[]"))
			return
		case <-release:
		case <-r.Context().Done():
			return
		}

		select {
		case <-allowSuccess:
			w.Header().Set("x-next-token", "")
			w.Write([]byte("[]"))
		case <-r.Context().Done():
		}
	}))
	defer server.Close()

	paginator := List(&SandboxListOpts{
		ApiKey:           "e2b_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		apiUrl:           server.URL,
		Domain:           "e2b.app",
		RequestTimeoutMs: intPtr(1000),
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := paginator.NextItemsContext(ctx, nil)
		done <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first page request to start")
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected first page to be canceled, got %T %v", err, err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first page cancellation")
	}

	close(release)
	close(allowSuccess)

	items, err := paginator.NextItems()
	if err != nil {
		t.Fatalf("expected paginator to remain usable after canceled page, got %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty items after successful retry, got %#v", items)
	}
	if paginator.HasNext {
		t.Fatal("expected paginator to be exhausted after successful retry")
	}
}

func TestSandboxPaginatorNextItemsHonorsPerCallSignalContext(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStarted <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()

	paginator := List(&SandboxListOpts{
		ApiKey:           "e2b_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		apiUrl:           server.URL,
		Domain:           "e2b.app",
		RequestTimeoutMs: intPtr(10000),
	})

	signal, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := paginator.NextItems(&SandboxApiOpts{Signal: signal})
		done <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for paginator request to start")
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected per-call signal cancellation, got %T %v", err, err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for paginator signal cancellation")
	}
}

func TestSnapshotPaginatorNextItemsHonorsPerCallSignalContext(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStarted <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()

	paginator := ListSnapshots(&SnapshotListOpts{
		ApiKey:           "e2b_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		apiUrl:           server.URL,
		Domain:           "e2b.app",
		RequestTimeoutMs: intPtr(10000),
	})

	signal, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := paginator.NextItems(&SandboxApiOpts{Signal: signal})
		done <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for snapshot paginator request to start")
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected snapshot paginator per-call signal cancellation, got %T %v", err, err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for snapshot paginator signal cancellation")
	}
}

func TestPaginatorConstructorsAreNotExported(t *testing.T) {
	source, err := os.ReadFile("paginator.go")
	if err != nil {
		t.Fatalf("failed to read paginator.go: %v", err)
	}

	text := string(source)
	if strings.Contains(text, "func NewPaginator(") {
		t.Fatal("did not expect NewPaginator to be exported")
	}
	if strings.Contains(text, "func NewPaginatorWithInitialToken(") {
		t.Fatal("did not expect NewPaginatorWithInitialToken to be exported")
	}
	if strings.Contains(text, "type Paginator[") {
		t.Fatal("did not expect Paginator to be exported")
	}
	if !strings.Contains(text, "type SandboxPaginator struct") {
		t.Fatal("expected SandboxPaginator to be exported")
	}
	if !strings.Contains(text, "type SnapshotPaginator struct") {
		t.Fatal("expected SnapshotPaginator to be exported")
	}
}
