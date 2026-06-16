package e2b

import (
	"context"
	"testing"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/api"
	"github.com/superduck-ai/e2b-go-sdk/runtime/applecontainer"
)

func TestCreateUsesAppleContainerRuntimeWithoutAPIKey(t *testing.T) {
	t.Setenv("E2B_API_KEY", "")

	fake := &fakeLocalSandboxRuntime{
		createResp: &api.SandboxResponse{
			SandboxID:   "sbxlocal",
			TemplateID:  "base",
			Name:        "sbxlocal",
			State:       "running",
			CpuCount:    1,
			MemoryMB:    1024,
			EnvdURL:     "http://127.0.0.1:49983",
			EnvdVersion: "99.99.99",
		},
	}
	restore := stubAppleContainerRuntime(fake)
	defer restore()

	opts := WithAppleContainerRuntime(&applecontainer.RuntimeOptions{
		EnvdBinary: "/tmp/envd-linux-arm64",
	})
	opts.Metadata = map[string]string{"mode": "local"}
	opts.Envs = map[string]string{"HELLO": "world"}

	sandbox, err := Create(context.Background(), "", opts)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if sandbox.SandboxID != "sbxlocal" {
		t.Fatalf("expected local sandbox id, got %q", sandbox.SandboxID)
	}
	if sandbox.localRuntime != fake {
		t.Fatal("expected sandbox to retain local runtime")
	}
	if fake.createReq.TemplateID != "base" {
		t.Fatalf("expected default base template, got %q", fake.createReq.TemplateID)
	}
	if fake.createReq.Metadata["mode"] != "local" || fake.createReq.EnvVars["HELLO"] != "world" {
		t.Fatalf("expected create request to carry metadata/envs, got %#v", fake.createReq)
	}
	if fake.createReq.SandboxID == "" {
		t.Fatal("expected SDK to generate a local sandbox id")
	}
}

func TestCreateAcceptsWithAppleContainerRuntimeOption(t *testing.T) {
	fake := &fakeLocalSandboxRuntime{
		createResp: &api.SandboxResponse{
			SandboxID:   "sbxwith",
			TemplateID:  "base",
			State:       "running",
			EnvdURL:     "http://127.0.0.1:49983",
			EnvdVersion: "99.99.99",
		},
	}
	restore := stubAppleContainerRuntime(fake)
	defer restore()

	runtimeOpts := &applecontainer.RuntimeOptions{EnvdBinary: "/tmp/envd-linux-arm64"}
	opts := WithAppleContainerRuntime(runtimeOpts)
	opts.Metadata = map[string]string{"runtime": "apple"}

	sandbox, err := Create(context.Background(), "", opts)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if sandbox.SandboxID != "sbxwith" {
		t.Fatalf("expected helper-created sandbox id, got %q", sandbox.SandboxID)
	}
	if opts.runtime != applecontainer.RuntimeName || opts.appleContainer != runtimeOpts {
		t.Fatalf("unexpected helper options: %#v", opts)
	}
	if fake.createReq.Metadata["runtime"] != "apple" {
		t.Fatalf("expected metadata to pass through helper options, got %#v", fake.createReq.Metadata)
	}
}

func TestConnectUsesAppleContainerRuntime(t *testing.T) {
	fake := &fakeLocalSandboxRuntime{
		connectResp: &api.SandboxResponse{
			SandboxID:   "sbxpaused",
			TemplateID:  "base",
			State:       "running",
			EnvdURL:     "http://127.0.0.1:49984",
			EnvdVersion: "99.99.99",
		},
	}
	restore := stubAppleContainerRuntime(fake)
	defer restore()

	sandbox, err := Connect(context.Background(), "sbxpaused", WithAppleContainerConnectRuntime())
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	if sandbox.SandboxID != "sbxpaused" {
		t.Fatalf("expected connected sandbox id, got %q", sandbox.SandboxID)
	}
	if fake.connectedID != "sbxpaused" {
		t.Fatalf("expected runtime connect for sbxpaused, got %q", fake.connectedID)
	}
}

func TestAppleContainerSandboxLifecycleUsesLocalRuntime(t *testing.T) {
	now := time.Now().UTC()
	fake := &fakeLocalSandboxRuntime{
		getResp: &api.SandboxResponse{
			SandboxID:   "sbxlocal",
			TemplateID:  "base",
			State:       "running",
			StartedAt:   now,
			EndAt:       now.Add(time.Minute),
			CpuCount:    2,
			MemoryMB:    2048,
			EnvdURL:     "http://127.0.0.1:49983",
			EnvdVersion: "99.99.99",
		},
		metricsResp: []api.SandboxMetrics{{
			Timestamp:   now,
			CpuUsedPct:  12.5,
			CpuCount:    2,
			MemUsedMiB:  64,
			MemTotalMiB: 2048,
		}},
	}
	sandbox := newSandboxFromResponse(fake.getResp, NewConnectionConfig(nil))
	sandbox.localRuntime = fake

	paused, err := sandbox.Pause(context.Background(), nil)
	if err != nil || !paused {
		t.Fatalf("Pause = %v, %v", paused, err)
	}
	info, err := sandbox.GetInfo(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetInfo returned error: %v", err)
	}
	if info.CpuCount != 2 || info.MemoryMB != 2048 {
		t.Fatalf("expected runtime info, got %#v", info)
	}
	metrics, err := sandbox.GetMetrics(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetMetrics returned error: %v", err)
	}
	if len(metrics) != 1 || metrics[0].MemTotal != 2048 {
		t.Fatalf("expected local metrics conversion, got %#v", metrics)
	}
	if err := sandbox.Kill(context.Background(), nil); err != nil {
		t.Fatalf("Kill returned error: %v", err)
	}
	if fake.killedID != "sbxlocal" || !fake.closed {
		t.Fatalf("expected local kill and close, got killed=%q closed=%v", fake.killedID, fake.closed)
	}
}

func TestAppleContainerPackageLifecycleUsesLocalRuntime(t *testing.T) {
	now := time.Now().UTC()
	fake := &fakeLocalSandboxRuntime{
		getResp: &api.SandboxResponse{
			SandboxID:   "sbxapi",
			TemplateID:  "base",
			State:       "running",
			StartedAt:   now,
			EndAt:       now.Add(time.Minute),
			CpuCount:    2,
			MemoryMB:    2048,
			EnvdURL:     "http://127.0.0.1:49983",
			EnvdVersion: "99.99.99",
		},
		metricsResp: []api.SandboxMetrics{{
			Timestamp:   now,
			CpuUsedPct:  12.5,
			CpuCount:    2,
			MemUsedMiB:  64,
			MemTotalMiB: 2048,
		}},
	}
	restore := stubAppleContainerRuntime(fake)
	defer restore()

	apiOpts := WithAppleContainerApiRuntime()
	paused, err := Pause(context.Background(), "sbxapi", apiOpts)
	if err != nil || !paused {
		t.Fatalf("Pause = %v, %v", paused, err)
	}
	info, err := GetInfo(context.Background(), "sbxapi", apiOpts)
	if err != nil {
		t.Fatalf("GetInfo returned error: %v", err)
	}
	if info.SandboxID != "sbxapi" || info.MemoryMB != 2048 {
		t.Fatalf("expected local info, got %#v", info)
	}
	metrics, err := GetMetrics(context.Background(), "sbxapi", &SandboxMetricsOpts{
		SandboxApiOpts: *WithAppleContainerApiRuntime(),
	})
	if err != nil {
		t.Fatalf("GetMetrics returned error: %v", err)
	}
	if len(metrics) != 1 || metrics[0].MemTotal != 2048 {
		t.Fatalf("expected local metrics conversion, got %#v", metrics)
	}
	killed, err := Kill(context.Background(), "sbxapi", apiOpts)
	if err != nil || !killed {
		t.Fatalf("Kill = %v, %v", killed, err)
	}
	if fake.pausedID != "sbxapi" || fake.getID != "sbxapi" || fake.metricsID != "sbxapi" || fake.killedID != "sbxapi" {
		t.Fatalf("expected package APIs to use local runtime, got paused=%q get=%q metrics=%q killed=%q", fake.pausedID, fake.getID, fake.metricsID, fake.killedID)
	}
}

type fakeLocalSandboxRuntime struct {
	createReq   applecontainer.CreateSandboxRequest
	createResp  *api.SandboxResponse
	connectResp *api.SandboxResponse
	getResp     *api.SandboxResponse
	metricsResp []api.SandboxMetrics
	connectedID string
	getID       string
	metricsID   string
	pausedID    string
	killedID    string
	closed      bool
}

func (fake *fakeLocalSandboxRuntime) CreateSandbox(ctx context.Context, req applecontainer.CreateSandboxRequest) (*api.SandboxResponse, error) {
	fake.createReq = req
	return fake.createResp, nil
}

func (fake *fakeLocalSandboxRuntime) ConnectSandbox(ctx context.Context, sandboxID string) (*api.SandboxResponse, error) {
	fake.connectedID = sandboxID
	return fake.connectResp, nil
}

func (fake *fakeLocalSandboxRuntime) GetSandbox(ctx context.Context, sandboxID string) (*api.SandboxResponse, error) {
	fake.getID = sandboxID
	return fake.getResp, nil
}

func (fake *fakeLocalSandboxRuntime) KillSandbox(ctx context.Context, sandboxID string) (bool, error) {
	fake.killedID = sandboxID
	return true, nil
}

func (fake *fakeLocalSandboxRuntime) PauseSandbox(ctx context.Context, sandboxID string) (bool, error) {
	fake.pausedID = sandboxID
	return true, nil
}

func (fake *fakeLocalSandboxRuntime) GetMetrics(ctx context.Context, sandboxID string) ([]api.SandboxMetrics, error) {
	fake.metricsID = sandboxID
	return fake.metricsResp, nil
}

func (fake *fakeLocalSandboxRuntime) Close() {
	fake.closed = true
}

func stubAppleContainerRuntime(fake localSandboxRuntime) func() {
	previous := newAppleContainerSandboxRuntime
	newAppleContainerSandboxRuntime = func(opts *applecontainer.RuntimeOptions) (localSandboxRuntime, error) {
		return fake, nil
	}
	return func() {
		newAppleContainerSandboxRuntime = previous
	}
}
