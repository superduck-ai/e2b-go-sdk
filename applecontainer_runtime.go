package e2b

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/superduck-ai/e2b-go-sdk/api"
	"github.com/superduck-ai/e2b-go-sdk/commands"
	"github.com/superduck-ai/e2b-go-sdk/runtime/applecontainer"
)

type localSandboxRuntime interface {
	CreateSandbox(ctx context.Context, req applecontainer.CreateSandboxRequest) (*api.SandboxResponse, error)
	ConnectSandbox(ctx context.Context, sandboxID string) (*api.SandboxResponse, error)
	GetSandbox(ctx context.Context, sandboxID string) (*api.SandboxResponse, error)
	KillSandbox(ctx context.Context, sandboxID string) (bool, error)
	PauseSandbox(ctx context.Context, sandboxID string) (bool, error)
	GetMetrics(ctx context.Context, sandboxID string) ([]api.SandboxMetrics, error)
	Close()
}

var newAppleContainerSandboxRuntime = func(opts *applecontainer.RuntimeOptions) (localSandboxRuntime, error) {
	return applecontainer.NewRuntime(opts)
}

func isAppleContainerRuntime(runtime string) bool {
	return strings.EqualFold(strings.TrimSpace(runtime), applecontainer.RuntimeName)
}

// WithAppleContainerRuntime returns sandbox options configured for the native Apple Container runtime.
func WithAppleContainerRuntime(opts ...*applecontainer.RuntimeOptions) *SandboxOpts {
	var runtimeOpts *applecontainer.RuntimeOptions
	if len(opts) > 0 {
		runtimeOpts = opts[0]
	}
	return &SandboxOpts{
		runtime:        applecontainer.RuntimeName,
		appleContainer: runtimeOpts,
	}
}

// WithAppleContainerConnectRuntime returns connect options configured for the native Apple Container runtime.
func WithAppleContainerConnectRuntime(opts ...*applecontainer.RuntimeOptions) *SandboxConnectOpts {
	var runtimeOpts *applecontainer.RuntimeOptions
	if len(opts) > 0 {
		runtimeOpts = opts[0]
	}
	return &SandboxConnectOpts{
		runtime:        applecontainer.RuntimeName,
		appleContainer: runtimeOpts,
	}
}

// WithAppleContainerApiRuntime returns package-level API options configured for the native Apple Container runtime.
func WithAppleContainerApiRuntime(opts ...*applecontainer.RuntimeOptions) *SandboxApiOpts {
	var runtimeOpts *applecontainer.RuntimeOptions
	if len(opts) > 0 {
		runtimeOpts = opts[0]
	}
	return &SandboxApiOpts{
		runtime:        applecontainer.RuntimeName,
		appleContainer: runtimeOpts,
	}
}

func createAppleContainerSandbox(ctx context.Context, template string, opts *SandboxOpts, autoPause bool) (*Sandbox, error) {
	runtime, err := newAppleContainerSandboxRuntime(opts.appleContainer)
	if err != nil {
		return nil, err
	}

	timeoutMs := defaultSandboxTimeoutMs
	if opts.TimeoutMs != nil {
		timeoutMs = *opts.TimeoutMs
	}
	createReq, err := buildCreateSandboxRequest(template, opts, autoPause, timeoutMs)
	if err != nil {
		runtime.Close()
		return nil, err
	}

	timeoutSec := int(math.Ceil(float64(timeoutMs) / 1000.0))
	createdAt := time.Now().UTC()
	resp, err := runtime.CreateSandbox(ctx, applecontainer.CreateSandboxRequest{
		SandboxID:           newLocalSandboxID(),
		TemplateID:          createReq.TemplateID,
		Metadata:            createReq.Metadata,
		EnvVars:             createReq.EnvVars,
		TimeoutSeconds:      timeoutSec,
		AllowInternetAccess: createReq.AllowInternetAccess,
		CreatedAt:           createdAt,
		EndAt:               createdAt.Add(time.Duration(timeoutSec) * time.Second),
		VolumeMounts:        createReq.VolumeMounts,
	})
	if err != nil {
		runtime.Close()
		return nil, err
	}
	if err := ensureSandboxConnectionResponseData(resp); err != nil {
		runtime.Close()
		return nil, err
	}

	connConfig := NewConnectionConfig(&opts.ConnectionOpts)
	sbx := newSandboxFromResponse(resp, connConfig)
	sbx.localRuntime = runtime
	if err := startMcpGateway(ctx, sbx, opts.Mcp); err != nil {
		runtime.Close()
		return nil, err
	}
	return sbx, nil
}

func connectAppleContainerSandbox(ctx context.Context, sandboxID string, opts *SandboxConnectOpts) (*Sandbox, error) {
	runtime, err := newAppleContainerSandboxRuntime(opts.appleContainer)
	if err != nil {
		return nil, err
	}
	resp, err := runtime.ConnectSandbox(ctx, sandboxID)
	if err != nil {
		runtime.Close()
		return nil, err
	}
	if err := ensureSandboxConnectionResponseData(resp); err != nil {
		runtime.Close()
		return nil, err
	}
	connConfig := NewConnectionConfig(&opts.ConnectionOpts)
	sbx := newSandboxFromResponse(resp, connConfig)
	sbx.localRuntime = runtime
	return sbx, nil
}

func startMcpGateway(ctx context.Context, sbx *Sandbox, mcp McpServer) error {
	if mcp == nil {
		return nil
	}
	configJSON, err := json.Marshal(mcp)
	if err != nil {
		return fmt.Errorf("failed to marshal MCP config: %w", err)
	}
	sbx.mcpToken = uuid.NewString()
	execution, err := sbx.Commands.Run(ctx, "mcp-gateway --config "+shellQuote(string(configJSON)), &commands.CommandStartOpts{
		User: "root",
		Envs: map[string]string{
			"GATEWAY_ACCESS_TOKEN": sbx.mcpToken,
		},
	})
	if err != nil {
		var exitErr *commands.CommandExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("Failed to start MCP gateway: %s", exitErr.Stderr)
		}
		return fmt.Errorf("Failed to start MCP gateway: %w", err)
	}
	res, ok := execution.(*commands.CommandResult)
	if !ok {
		return fmt.Errorf("Failed to start MCP gateway: expected foreground command result, got %T", execution)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("Failed to start MCP gateway: %s", res.Stderr)
	}
	return nil
}

func sandboxMetricsFromAPI(metricsResp []api.SandboxMetrics) []SandboxMetrics {
	metrics := make([]SandboxMetrics, len(metricsResp))
	for i, m := range metricsResp {
		timestamp := m.Timestamp
		if timestamp.IsZero() && m.TimestampUnix != 0 {
			timestamp = time.Unix(m.TimestampUnix, 0)
		}
		metrics[i] = SandboxMetrics{
			Timestamp:  timestamp,
			CpuUsedPct: m.CpuUsedPct,
			CpuCount:   m.CpuCount,
			MemUsed:    resolveMetricValue(m.MemUsed, m.MemUsedMiB),
			MemTotal:   resolveMetricValue(m.MemTotal, m.MemTotalMiB),
			MemCache:   m.MemCache,
			DiskUsed:   resolveMetricValue(m.DiskUsed, m.DiskUsedMiB),
			DiskTotal:  resolveMetricValue(m.DiskTotal, m.DiskTotalMiB),
		}
	}
	return metrics
}

func newLocalSandboxID() string {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "sbx" + strings.TrimPrefix(strconv.FormatInt(time.Now().UnixNano(), 16), "-")
	}
	return "sbx" + hex.EncodeToString(bytes[:])
}
