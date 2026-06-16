package applecontainer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRuntimeConnectSandboxResumesStoppedContainerAndRefreshesState(t *testing.T) {
	now := time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)
	labels := map[string]string{
		localSandboxIDLabel:         "sbx-restored",
		localSandboxTemplateIDLabel: "base",
		localSandboxEnvVarsLabel:    `{"TOKEN":"secret"}`,
		localSandboxCreatedAtLabel:  now.Format(time.RFC3339Nano),
	}
	stoppedSnapshot := testContainerSnapshot("stopped", labels)
	runningSnapshot := testContainerSnapshot("running", labels)
	startedDate := AppleDate(now)
	runningSnapshot.StartedDate = &startedDate

	client := &fakeNativeClient{
		snapshots: []ContainerSnapshot{stoppedSnapshot, runningSnapshot},
	}
	runtime := &Runtime{
		cfg: RuntimeOptions{
			ContainerNamePrefix:  "e2b-sandbox-",
			EnvdBinary:           "/tmp/envd-linux-arm64",
			EnvdPort:             49983,
			HealthTimeoutSeconds: 1,
			DefaultCPUs:          1,
			DefaultMemoryMB:      1024,
			Templates: map[string]TemplateOptions{
				"base": {Image: "docker.io/library/debian:bookworm-slim", StartCmd: "python main.py"},
			},
		},
		client: client,
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/health" {
				t.Fatalf("expected health request, got %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"status":"ok","version":"99.99.99"}`)),
				Request:    req,
			}, nil
		})},
		now: func() time.Time { return now },
	}

	resp, err := runtime.ConnectSandbox(context.Background(), "sbx-restored")
	if err != nil {
		t.Fatalf("ConnectSandbox returned error: %v", err)
	}
	if resp.State != "running" {
		t.Fatalf("expected refreshed running state, got %q", resp.State)
	}
	if resp.EnvdURL != "http://127.0.0.1:56565" || resp.EnvdVersion != "99.99.99" {
		t.Fatalf("unexpected envd connection details: url=%q version=%q", resp.EnvdURL, resp.EnvdVersion)
	}
	if client.listCalls != 2 {
		t.Fatalf("expected connect to refresh snapshot after resume, got %d list calls", client.listCalls)
	}
	if len(client.processes) != 1 || !containsString(client.processes[0].Environment, "TOKEN=secret") {
		t.Fatalf("expected persisted env vars in envd process, got %#v", client.processes)
	}
	if !containsString(client.events, "bootstrap:e2b-sandbox-sbx-restored") ||
		!containsString(client.events, "start:e2b-sandbox-sbx-restored:envd") {
		t.Fatalf("expected resume to bootstrap and start envd, events=%#v", client.events)
	}
}

func TestAppleDateZeroValueMarshalsAsNull(t *testing.T) {
	var date AppleDate
	data, err := json.Marshal(date)
	if err != nil {
		t.Fatalf("marshal zero AppleDate: %v", err)
	}
	if string(data) != "null" {
		t.Fatalf("expected zero AppleDate to marshal as null, got %s", data)
	}

	var decoded AppleDate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal zero AppleDate: %v", err)
	}
	if !time.Time(decoded).IsZero() {
		t.Fatalf("expected zero AppleDate round-trip, got %s", time.Time(decoded))
	}
}

func TestRuntimeCreateSandboxRetriesHostPortConflicts(t *testing.T) {
	client := &fakeNativeClient{
		createErrs: []error{errors.New("bind: address already in use")},
	}
	runtime := testCreateRuntime(client)
	ports := []int{55001, 55002}
	runtime.findFreePort = func() (int, error) {
		if len(ports) == 0 {
			t.Fatal("unexpected extra port allocation")
		}
		port := ports[0]
		ports = ports[1:]
		return port, nil
	}

	resp, err := runtime.CreateSandbox(context.Background(), CreateSandboxRequest{
		SandboxID:  "sbx-retry",
		TemplateID: "base",
	})
	if err != nil {
		t.Fatalf("CreateSandbox returned error: %v", err)
	}
	if resp.EnvdURL != "http://127.0.0.1:55002" {
		t.Fatalf("expected retried envd URL, got %q", resp.EnvdURL)
	}
	if len(client.created) != 2 {
		t.Fatalf("expected two create attempts, got %d", len(client.created))
	}
	if client.created[0].PublishedPorts[0].HostPort != 55001 || client.created[1].PublishedPorts[0].HostPort != 55002 {
		t.Fatalf("unexpected create ports: %#v", client.created)
	}
	if !containsString(client.events, "delete:e2b-sandbox-sbx-retry") {
		t.Fatalf("expected cleanup after failed port allocation, events=%#v", client.events)
	}
}

func TestRuntimeCreateSandboxRetriesTransientStartNotFound(t *testing.T) {
	client := &fakeNativeClient{
		startErrs: []error{
			&xpcProtocolError{Code: appleErrorCodeNotFound, Message: "container with ID e2b-sandbox-sbx-retry not found"},
		},
	}
	runtime := testCreateRuntime(client)

	_, err := runtime.CreateSandbox(context.Background(), CreateSandboxRequest{
		SandboxID:  "sbx-retry",
		TemplateID: "base",
	})
	if err != nil {
		t.Fatalf("CreateSandbox returned error: %v", err)
	}

	wantStartEvents := []string{
		"start:e2b-sandbox-sbx-retry:e2b-sandbox-sbx-retry",
		"start:e2b-sandbox-sbx-retry:e2b-sandbox-sbx-retry",
		"start:e2b-sandbox-sbx-retry:envd",
	}
	startEvents := []string{}
	for _, event := range client.events {
		if strings.HasPrefix(event, "start:") {
			startEvents = append(startEvents, event)
		}
	}
	if strings.Join(startEvents, "\n") != strings.Join(wantStartEvents, "\n") {
		t.Fatalf("unexpected start events:\nwant %#v\ngot  %#v", wantStartEvents, startEvents)
	}
}

func TestRuntimeCreateSandboxUsesPrebakedEnvdAndSkipsNetworkWhenDisabled(t *testing.T) {
	opts := RuntimeOptions{
		ContainerNamePrefix:  "e2b-sandbox-",
		EnvdPort:             49983,
		HealthTimeoutSeconds: 1,
		DefaultCPUs:          1,
		DefaultMemoryMB:      1024,
		Templates: map[string]TemplateOptions{
			"base": {
				Image:            "docker.io/library/debian:bookworm-slim",
				StartCmd:         "python main.py",
				PrebakedEnvdPath: "/opt/e2b/envd",
			},
		},
	}
	if err := opts.Validate(); err != nil {
		t.Fatalf("prebaked envd config should not require host envd binary: %v", err)
	}

	client := &fakeNativeClient{}
	runtime := testCreateRuntime(client)
	runtime.cfg = opts
	runtime.findFreePort = func() (int, error) { return 55003, nil }
	allowInternet := false

	resp, err := runtime.CreateSandbox(context.Background(), CreateSandboxRequest{
		SandboxID:           "sbx-prebaked",
		TemplateID:          "base",
		AllowInternetAccess: &allowInternet,
	})
	if err != nil {
		t.Fatalf("CreateSandbox returned error: %v", err)
	}
	if resp.EnvdURL != "http://127.0.0.1:55003" {
		t.Fatalf("unexpected envd URL: %q", resp.EnvdURL)
	}
	if containsEventPrefix(client.events, "copy:") {
		t.Fatalf("expected prebaked envd path to skip copy, events=%#v", client.events)
	}
	if containsEventPrefix(client.events, "default-network:") {
		t.Fatalf("expected disabled internet to skip default network, events=%#v", client.events)
	}
	if len(client.created) != 1 {
		t.Fatalf("expected one create config, got %d", len(client.created))
	}
	if client.created[0].DNS != nil || len(client.created[0].Networks) != 0 {
		t.Fatalf("expected no DNS or network attachments, got dns=%#v networks=%#v", client.created[0].DNS, client.created[0].Networks)
	}
	if len(client.processes) != 1 {
		t.Fatalf("expected envd process config, got %d", len(client.processes))
	}
	process := client.processes[0]
	if process.Executable != "/opt/e2b/envd" {
		t.Fatalf("expected prebaked envd executable, got %q", process.Executable)
	}
	if len(process.Arguments) != 5 || process.Arguments[4] != "python main.py" {
		t.Fatalf("expected start command in envd args, got %#v", process.Arguments)
	}
}

func testCreateRuntime(client *fakeNativeClient) *Runtime {
	return &Runtime{
		cfg: RuntimeOptions{
			ContainerNamePrefix:  "e2b-sandbox-",
			EnvdBinary:           "/tmp/envd-linux-arm64",
			EnvdPort:             49983,
			HealthTimeoutSeconds: 1,
			DefaultCPUs:          1,
			DefaultMemoryMB:      1024,
			Templates: map[string]TemplateOptions{
				"base": {Image: "docker.io/library/debian:bookworm-slim"},
			},
		},
		client: client,
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"status":"ok","version":"99.99.99"}`)),
				Request:    req,
			}, nil
		})},
		findFreePort: func() (int, error) { return 55000, nil },
		now:          time.Now,
	}
}

func testContainerSnapshot(status string, labels map[string]string) ContainerSnapshot {
	return ContainerSnapshot{
		Status: status,
		Configuration: ContainerConfiguration{
			ID:     "e2b-sandbox-sbx-restored",
			Labels: labels,
			PublishedPorts: []PublishPort{{
				HostAddress:   "127.0.0.1",
				HostPort:      56565,
				ContainerPort: 49983,
				Proto:         "tcp",
				Count:         1,
			}},
			Resources: Resources{
				CPUs:        2,
				MemoryBytes: 2048 * 1024 * 1024,
			},
		},
	}
}

type fakeNativeClient struct {
	snapshots  []ContainerSnapshot
	events     []string
	processes  []ProcessConfiguration
	created    []ContainerConfiguration
	createErrs []error
	startErrs  []error
	listCalls  int
}

func (client *fakeNativeClient) Ping(ctx context.Context) error {
	client.events = append(client.events, "ping")
	return nil
}

func (client *fakeNativeClient) ResolveImage(ctx context.Context, ref string) (ImageDescription, error) {
	client.events = append(client.events, "resolve-image:"+ref)
	return ImageDescription{Reference: ref}, nil
}

func (client *fakeNativeClient) ContainerCreate(ctx context.Context, config ContainerConfiguration) error {
	client.events = append(client.events, "create:"+config.ID)
	client.created = append(client.created, config)
	if len(client.createErrs) > 0 {
		err := client.createErrs[0]
		client.createErrs = client.createErrs[1:]
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *fakeNativeClient) ContainerBootstrap(ctx context.Context, id string) error {
	client.events = append(client.events, "bootstrap:"+id)
	return nil
}

func (client *fakeNativeClient) ContainerStartProcess(ctx context.Context, containerID, processID string) error {
	client.events = append(client.events, "start:"+containerID+":"+processID)
	if len(client.startErrs) > 0 {
		err := client.startErrs[0]
		client.startErrs = client.startErrs[1:]
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *fakeNativeClient) ContainerCopyIn(ctx context.Context, id, srcPath, dstPath string, mode uint32) error {
	client.events = append(client.events, "copy:"+id+":"+dstPath)
	return nil
}

func (client *fakeNativeClient) ContainerCreateProcess(ctx context.Context, containerID, processID string, config ProcessConfiguration) error {
	client.events = append(client.events, "process:"+containerID+":"+processID)
	client.processes = append(client.processes, config)
	return nil
}

func (client *fakeNativeClient) ContainerStop(ctx context.Context, id string) error {
	client.events = append(client.events, "stop:"+id)
	return nil
}

func (client *fakeNativeClient) ContainerDelete(ctx context.Context, id string, force bool) error {
	client.events = append(client.events, "delete:"+id)
	return nil
}

func (client *fakeNativeClient) ContainerList(ctx context.Context, filters ContainerListFilters) ([]ContainerSnapshot, error) {
	client.events = append(client.events, "list:"+strings.Join(filters.IDs, ","))
	if len(client.snapshots) == 0 {
		client.listCalls++
		return nil, nil
	}
	index := client.listCalls
	client.listCalls++
	if index >= len(client.snapshots) {
		index = len(client.snapshots) - 1
	}
	snapshot := client.snapshots[index]
	if len(filters.IDs) == 0 {
		return []ContainerSnapshot{snapshot}, nil
	}
	for _, id := range filters.IDs {
		if snapshot.Configuration.ID == id {
			return []ContainerSnapshot{snapshot}, nil
		}
	}
	return nil, nil
}

func (client *fakeNativeClient) VolumeInspect(ctx context.Context, name string) (VolumeConfig, error) {
	client.events = append(client.events, "volume-inspect:"+name)
	return VolumeConfig{}, ErrVolumeNotFound
}

func (client *fakeNativeClient) VolumeList(ctx context.Context) ([]VolumeConfig, error) {
	client.events = append(client.events, "volume-list")
	return nil, nil
}

func (client *fakeNativeClient) DefaultNetworkAttachment(ctx context.Context, containerID string) ([]AttachmentConfig, error) {
	client.events = append(client.events, "default-network:"+containerID)
	return nil, nil
}

func (client *fakeNativeClient) Close() {
	client.events = append(client.events, "close")
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsEventPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
