package applecontainer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/superduck-ai/e2b-go-sdk/api"
	"github.com/superduck-ai/e2b-go-sdk/envd"
)

type CreateSandboxRequest struct {
	SandboxID           string
	TemplateID          string
	Metadata            map[string]string
	EnvVars             map[string]string
	TimeoutSeconds      int
	AllowInternetAccess *bool
	CreatedAt           time.Time
	EndAt               time.Time
	VolumeMounts        []api.VolumeMount
}

type nativeClient interface {
	Ping(ctx context.Context) error
	ResolveImage(ctx context.Context, ref string) (ImageDescription, error)
	ContainerCreate(ctx context.Context, config ContainerConfiguration) error
	ContainerBootstrap(ctx context.Context, id string) error
	ContainerStartProcess(ctx context.Context, containerID, processID string) error
	ContainerCopyIn(ctx context.Context, id, srcPath, dstPath string, mode uint32) error
	ContainerCreateProcess(ctx context.Context, containerID, processID string, config ProcessConfiguration) error
	ContainerStop(ctx context.Context, id string) error
	ContainerDelete(ctx context.Context, id string, force bool) error
	ContainerList(ctx context.Context, filters ContainerListFilters) ([]ContainerSnapshot, error)
	VolumeInspect(ctx context.Context, name string) (VolumeConfig, error)
	VolumeList(ctx context.Context) ([]VolumeConfig, error)
	DefaultNetworkAttachment(ctx context.Context, containerID string) ([]AttachmentConfig, error)
	Close()
}

type Runtime struct {
	cfg          RuntimeOptions
	client       nativeClient
	logger       *log.Logger
	httpClient   *http.Client
	findFreePort func() (int, error)
	now          func() time.Time
}

const (
	envdPath                       = "/usr/local/bin/envd"
	envdProcessID                  = "envd"
	envdHost                       = "127.0.0.1"
	initSleepTime                  = "2147483647"
	localManagedLabel              = "e2b.local.managed"
	localImageLabel                = "e2b.local.image"
	localSandboxIDLabel            = "e2b.local.sandbox_id"
	localSandboxTemplateIDLabel    = "e2b.local.template_id"
	localSandboxCreatedAtLabel     = "e2b.local.created_at"
	localSandboxEndAtLabel         = "e2b.local.end_at"
	localSandboxMetadataLabel      = "e2b.local.metadata"
	localSandboxAllowNetLabel      = "e2b.local.allow_internet_access"
	localSandboxMountsLabel        = "e2b.local.volume_mounts"
	localSandboxEnvVarsLabel       = "e2b.local.env_vars"
	localVolumeIDLabel             = "e2b.local.volume_id"
	localVolumeNameLabel           = "e2b.local.name"
	resourceRoleLabel              = "com.apple.container.resource.role"
	resourceRoleBuiltin            = "builtin"
	runtimeHandler                 = "container-runtime-linux"
	volumeNamePrefix               = "e2b-vol-"
	defaultSandboxTimeout          = 300
	maxEnvdPortAttempts            = 3
	transientNotFoundRetryAttempts = 20
	transientNotFoundInitialDelay  = 50 * time.Millisecond
	transientNotFoundMaxDelay      = 500 * time.Millisecond
)

func NewRuntime(opts *RuntimeOptions) (*Runtime, error) {
	cfg := DefaultRuntimeOptions()
	if opts != nil {
		cfg = *opts
	}
	cfg = cfg.withDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.requiresHostEnvdBinary() {
		if err := validateEnvdBinary(cfg.EnvdBinary); err != nil {
			return nil, err
		}
	}

	client, err := newNativeClient()
	if err != nil {
		return nil, fmt.Errorf("connect to Apple Container XPC services; ensure Apple Container is installed and running: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	return &Runtime{
		cfg:        cfg,
		client:     client,
		logger:     logger,
		httpClient: &http.Client{Timeout: 3 * time.Second},
		findFreePort: func() (int, error) {
			return findFreePort()
		},
		now: time.Now,
	}, nil
}

func (r *Runtime) Close() {
	if r == nil || r.client == nil {
		return
	}
	r.client.Close()
}

func (r *Runtime) CreateSandbox(ctx context.Context, req CreateSandboxRequest) (*api.SandboxResponse, error) {
	releaseOperationLock, err := acquireOperationLock(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire apple container operation lock: %w", err)
	}
	defer releaseOperationLock()

	req = r.normalizeCreateRequest(req)
	template, ok := r.cfg.Templates[req.TemplateID]
	if !ok {
		return nil, fmt.Errorf("applecontainer template %q not found", req.TemplateID)
	}

	imageRef := strings.TrimSpace(template.Image)
	image, err := r.client.ResolveImage(ctx, imageRef)
	if err != nil {
		return nil, fmt.Errorf("resolve apple container image %q: %w", imageRef, err)
	}

	volumeMounts, filesystems, err := r.resolveVolumeMounts(ctx, req.VolumeMounts)
	if err != nil {
		return nil, err
	}

	containerID := sandboxContainerID(r.cfg, req.SandboxID)
	networks, err := r.networkAttachments(ctx, containerID, req.AllowInternetAccess)
	if err != nil {
		return nil, err
	}
	labels, err := sandboxLabels(req, imageRef, volumeMounts)
	if err != nil {
		return nil, err
	}

	cpus := template.CPUs
	if cpus <= 0 {
		cpus = r.cfg.DefaultCPUs
	}
	memoryMB := template.MemoryMB
	if memoryMB <= 0 {
		memoryMB = r.cfg.DefaultMemoryMB
	}

	var lastErr error
	for attempt := 1; attempt <= maxEnvdPortAttempts; attempt++ {
		hostPort, err := r.nextEnvdHostPort()
		if err != nil {
			return nil, fmt.Errorf("find free envd host port: %w", err)
		}

		config := ContainerConfiguration{
			ID:       containerID,
			Image:    image,
			Mounts:   filesystems,
			Labels:   labels,
			Networks: networks,
			DNS:      dnsConfiguration(req.AllowInternetAccess),
			InitProcess: ProcessConfiguration{
				Executable:         "/bin/sleep",
				Arguments:          []string{initSleepTime},
				Environment:        processEnvironment(nil),
				WorkingDirectory:   "/",
				Terminal:           false,
				User:               ProcessUserRoot(),
				SupplementalGroups: []uint32{},
				Rlimits:            []any{},
			},
			PublishedPorts: []PublishPort{{
				HostAddress:   envdHost,
				HostPort:      uint16(hostPort),
				ContainerPort: uint16(r.cfg.EnvdPort),
				Proto:         "tcp",
				Count:         1,
			}},
			Resources: Resources{
				CPUs:        cpus,
				MemoryBytes: uint64(memoryMB) * 1024 * 1024,
				CPUOverhead: 1,
			},
			RuntimeHandler: runtimeHandler,
		}

		if err := r.client.ContainerCreate(ctx, config); err != nil {
			lastErr = fmt.Errorf("create apple container %s: %w", containerID, err)
			if isPortAllocationError(err) {
				if attempt < maxEnvdPortAttempts {
					r.cleanupContainer(containerID)
					r.logf("applecontainer retrying sandbox create after host port conflict sandbox_id=%s container_id=%s attempt=%d", req.SandboxID, containerID, attempt)
					continue
				}
				return nil, fmt.Errorf("create apple container %s after %d host port attempts: %w", containerID, maxEnvdPortAttempts, err)
			}
			return nil, lastErr
		}
		cleanup := func() {
			r.cleanupContainer(containerID)
		}
		if err := r.client.ContainerBootstrap(ctx, containerID); err != nil {
			cleanup()
			return nil, fmt.Errorf("bootstrap apple container %s: %w", containerID, err)
		}
		if err := r.startEnvd(ctx, containerID, req.EnvVars, template); err != nil {
			cleanup()
			lastErr = err
			if isAppleNotFound(err) && attempt < maxEnvdPortAttempts {
				r.logf("applecontainer retrying sandbox create after transient not_found sandbox_id=%s container_id=%s attempt=%d", req.SandboxID, containerID, attempt)
				continue
			}
			return nil, err
		}

		envdURL := appleEnvdURL(strconv.Itoa(hostPort))
		envdVersion, err := r.healthCheck(ctx, envdURL)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("envd health check: %w", err)
		}

		return &api.SandboxResponse{
			SandboxID:           req.SandboxID,
			TemplateID:          req.TemplateID,
			Name:                req.SandboxID,
			Alias:               req.SandboxID,
			Metadata:            copyStringMap(req.Metadata),
			StartedAt:           req.CreatedAt,
			EndAt:               req.EndAt,
			State:               "running",
			CpuCount:            cpus,
			MemoryMB:            memoryMB,
			EnvdURL:             envdURL,
			EnvdVersion:         envdVersion,
			AllowInternetAccess: req.AllowInternetAccess,
			VolumeMounts:        volumeMounts,
		}, nil
	}

	return nil, lastErr
}

func (r *Runtime) ConnectSandbox(ctx context.Context, sandboxID string) (*api.SandboxResponse, error) {
	snapshot, err := r.containerSnapshot(ctx, sandboxContainerID(r.cfg, sandboxID))
	if err != nil {
		return nil, err
	}
	hostPort := envdHostPort(snapshot.Configuration.PublishedPorts, r.cfg.EnvdPort)
	if hostPort == "" {
		return nil, fmt.Errorf("missing persisted envd host port for apple container %s", snapshot.Configuration.ID)
	}

	version := ""
	resumed := false
	if e2bStateFromAppleStatus(snapshot.Status) != "running" {
		template := r.templateForSnapshot(snapshot)
		envVars := stringMapFromLabel(snapshot.Configuration.Labels[localSandboxEnvVarsLabel])
		releaseOperationLock, err := acquireOperationLock(ctx)
		if err != nil {
			return nil, fmt.Errorf("acquire apple container operation lock: %w", err)
		}
		defer releaseOperationLock()

		if err := r.client.ContainerBootstrap(ctx, snapshot.Configuration.ID); err != nil {
			return nil, fmt.Errorf("bootstrap apple container %s: %w", snapshot.Configuration.ID, err)
		}
		if err := r.startEnvd(ctx, snapshot.Configuration.ID, envVars, template); err != nil {
			return nil, err
		}
		resumed = true
	}
	envdURL := appleEnvdURL(hostPort)
	if envdURL != "" {
		var err error
		version, err = r.healthCheck(ctx, envdURL)
		if err != nil {
			return nil, fmt.Errorf("envd health check: %w", err)
		}
	}
	if resumed {
		containerID := snapshot.Configuration.ID
		snapshot, err = r.containerSnapshot(ctx, containerID)
		if err != nil {
			return nil, fmt.Errorf("refresh apple container %s: %w", containerID, err)
		}
		if e2bStateFromAppleStatus(snapshot.Status) != "running" {
			snapshot.Status = "running"
		}
	}

	return r.responseFromSnapshot(snapshot, version), nil
}

func (r *Runtime) GetSandbox(ctx context.Context, sandboxID string) (*api.SandboxResponse, error) {
	snapshot, err := r.containerSnapshot(ctx, sandboxContainerID(r.cfg, sandboxID))
	if err != nil {
		return nil, err
	}
	return r.responseFromSnapshot(snapshot, ""), nil
}

func (r *Runtime) KillSandbox(ctx context.Context, sandboxID string) (bool, error) {
	containerID := sandboxContainerID(r.cfg, sandboxID)
	if err := r.client.ContainerDelete(ctx, containerID, true); err != nil {
		if isAppleNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Runtime) PauseSandbox(ctx context.Context, sandboxID string) (bool, error) {
	containerID := sandboxContainerID(r.cfg, sandboxID)
	if err := r.client.ContainerStop(ctx, containerID); err != nil {
		if isAppleAlreadyStopped(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Runtime) GetMetrics(ctx context.Context, sandboxID string) ([]api.SandboxMetrics, error) {
	resp, err := r.GetSandbox(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.EnvdURL) == "" {
		return nil, fmt.Errorf("applecontainer sandbox %s has no envd URL", sandboxID)
	}

	client := envd.NewEnvdApiClient(strings.TrimRight(resp.EnvdURL, "/"), "", nil, 3000)
	metrics, err := client.Metrics(ctx)
	if err != nil {
		return nil, err
	}
	timestamp, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(metrics.Timestamp))
	if timestamp.IsZero() {
		timestamp = r.now().UTC()
	}
	return []api.SandboxMetrics{{
		Timestamp:    timestamp,
		CpuUsedPct:   metrics.CpuUsedPct,
		CpuCount:     metrics.CpuCount,
		MemUsedMiB:   metrics.MemUsedMiB,
		MemTotalMiB:  metrics.MemTotalMiB,
		DiskUsedMiB:  metrics.DiskUsedMiB,
		DiskTotalMiB: metrics.DiskTotalMiB,
	}}, nil
}

func (r *Runtime) normalizeCreateRequest(req CreateSandboxRequest) CreateSandboxRequest {
	req.TemplateID = strings.TrimSpace(req.TemplateID)
	if req.TemplateID == "" {
		req.TemplateID = "base"
	}
	req.SandboxID = strings.TrimSpace(req.SandboxID)
	if req.SandboxID == "" {
		req.SandboxID = newSandboxID()
	}
	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = defaultSandboxTimeout
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = r.now().UTC()
	}
	if req.EndAt.IsZero() {
		req.EndAt = req.CreatedAt.Add(time.Duration(req.TimeoutSeconds) * time.Second)
	}
	return req
}

func (r *Runtime) startEnvd(ctx context.Context, containerID string, envVars map[string]string, template TemplateOptions) error {
	if err := r.withTransientNotFoundRetry(ctx, "start init process", func(ctx context.Context) error {
		return r.client.ContainerStartProcess(ctx, containerID, containerID)
	}); err != nil {
		return fmt.Errorf("start apple container init process: %w", err)
	}
	executable := strings.TrimSpace(template.PrebakedEnvdPath)
	if executable == "" {
		executable = envdPath
		if err := r.withTransientNotFoundRetry(ctx, "copy envd binary", func(ctx context.Context) error {
			return r.client.ContainerCopyIn(ctx, containerID, r.cfg.EnvdBinary, envdPath, 0o755)
		}); err != nil {
			return fmt.Errorf("copy envd binary: %w", err)
		}
	}

	config := ProcessConfiguration{
		Executable:         executable,
		Arguments:          envdArguments(r.cfg.EnvdPort, template.StartCmd),
		Environment:        processEnvironment(envVars),
		WorkingDirectory:   "/",
		Terminal:           false,
		User:               ProcessUserRoot(),
		SupplementalGroups: []uint32{},
		Rlimits:            []any{},
	}
	if err := r.withTransientNotFoundRetry(ctx, "create envd process", func(ctx context.Context) error {
		return r.client.ContainerCreateProcess(ctx, containerID, envdProcessID, config)
	}); err != nil {
		return fmt.Errorf("create envd process: %w", err)
	}
	if err := r.withTransientNotFoundRetry(ctx, "start envd process", func(ctx context.Context) error {
		return r.client.ContainerStartProcess(ctx, containerID, envdProcessID)
	}); err != nil {
		return fmt.Errorf("start envd process: %w", err)
	}
	return nil
}

func (r *Runtime) withTransientNotFoundRetry(ctx context.Context, operation string, execute func(context.Context) error) error {
	var lastErr error
	delay := transientNotFoundInitialDelay
	for attempt := 1; attempt <= transientNotFoundRetryAttempts; attempt++ {
		if err := execute(ctx); err != nil {
			if !isAppleNotFound(err) {
				return err
			}
			lastErr = err
		} else {
			if attempt > 1 {
				r.logf("applecontainer recovered after transient not_found operation=%s attempts=%d", operation, attempt)
			}
			return nil
		}

		if attempt == transientNotFoundRetryAttempts {
			break
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}

		delay *= 2
		if delay > transientNotFoundMaxDelay {
			delay = transientNotFoundMaxDelay
		}
	}
	return lastErr
}

func (r *Runtime) nextEnvdHostPort() (int, error) {
	find := r.findFreePort
	if find == nil {
		find = findFreePort
	}
	hostPort, err := find()
	if err != nil {
		return 0, err
	}
	if hostPort <= 0 || hostPort > 65535 {
		return 0, fmt.Errorf("invalid envd host port %d", hostPort)
	}
	return hostPort, nil
}

func (r *Runtime) cleanupContainer(containerID string) {
	if err := r.client.ContainerDelete(context.Background(), containerID, true); err != nil {
		if isAppleNotFound(err) {
			return
		}
		r.logf("applecontainer cleanup failed container_id=%s error=%v", containerID, err)
	}
}

func (r *Runtime) logf(format string, args ...any) {
	if r != nil && r.logger != nil {
		r.logger.Printf(format, args...)
	}
}

func isPortAllocationError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	portConflictSignals := []string{
		"address already in use",
		"bind: address",
		"host port",
		"port already allocated",
		"port is already allocated",
		"port is already in use",
		"port is unavailable",
	}
	for _, signal := range portConflictSignals {
		if strings.Contains(message, signal) {
			return true
		}
	}
	return false
}

func (r *Runtime) healthCheck(ctx context.Context, envdURL string) (string, error) {
	deadline := time.Now().UTC().Add(time.Duration(r.cfg.HealthTimeoutSeconds) * time.Second)
	healthURL := strings.TrimRight(strings.TrimSpace(envdURL), "/") + "/health"
	var lastErr error
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			if lastErr != nil {
				return "", fmt.Errorf("wait for envd health %s: %w", envdURL, lastErr)
			}
			return "", fmt.Errorf("wait for envd health %s timed out", envdURL)
		}
		requestCtx, cancel := context.WithTimeout(ctx, remaining)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, healthURL, nil)
		if err != nil {
			cancel()
			return "", fmt.Errorf("build envd health request: %w", err)
		}
		resp, err := r.httpClient.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var health envd.HealthResponse
				if len(body) > 0 {
					_ = json.Unmarshal(body, &health)
				}
				if strings.TrimSpace(health.Version) != "" {
					cancel()
					return health.Version, nil
				}
				cancel()
				return envd.EnvdDebugFallback, nil
			}
			if readErr != nil {
				lastErr = readErr
			} else {
				lastErr = fmt.Errorf("unexpected status %d", resp.StatusCode)
			}
		} else {
			lastErr = err
		}
		cancel()
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
}

func (r *Runtime) containerSnapshot(ctx context.Context, containerID string) (ContainerSnapshot, error) {
	snapshots, err := r.client.ContainerList(ctx, ContainerListFilters{IDs: []string{containerID}})
	if err != nil {
		return ContainerSnapshot{}, err
	}
	if len(snapshots) == 0 {
		return ContainerSnapshot{}, fmt.Errorf("apple container %s not found", containerID)
	}
	return snapshots[0], nil
}

func (r *Runtime) responseFromSnapshot(snapshot ContainerSnapshot, envdVersion string) *api.SandboxResponse {
	labels := snapshot.Configuration.Labels
	createdAt := timeFromLabel(labels[localSandboxCreatedAtLabel])
	if createdAt.IsZero() && snapshot.Configuration.CreationDate != nil {
		createdAt = time.Time(*snapshot.Configuration.CreationDate).UTC()
	}
	if createdAt.IsZero() {
		createdAt = r.now().UTC()
	}
	startedAt := createdAt
	if snapshot.StartedDate != nil {
		startedAt = time.Time(*snapshot.StartedDate).UTC()
	}
	endAt := timeFromLabel(labels[localSandboxEndAtLabel])
	if endAt.IsZero() {
		endAt = createdAt.Add(defaultSandboxTimeout * time.Second)
	}

	return &api.SandboxResponse{
		SandboxID:           firstNonEmpty(labels[localSandboxIDLabel], strings.TrimPrefix(snapshot.Configuration.ID, r.cfg.ContainerNamePrefix)),
		TemplateID:          strings.TrimSpace(labels[localSandboxTemplateIDLabel]),
		Name:                firstNonEmpty(labels[localSandboxIDLabel], snapshot.Configuration.ID),
		Alias:               firstNonEmpty(labels[localSandboxIDLabel], snapshot.Configuration.ID),
		Metadata:            stringMapFromLabel(labels[localSandboxMetadataLabel]),
		StartedAt:           startedAt,
		EndAt:               endAt,
		State:               e2bStateFromAppleStatus(snapshot.Status),
		CpuCount:            snapshot.Configuration.Resources.CPUs,
		MemoryMB:            int(snapshot.Configuration.Resources.MemoryBytes / 1024 / 1024),
		EnvdURL:             appleEnvdURL(envdHostPort(snapshot.Configuration.PublishedPorts, r.cfg.EnvdPort)),
		EnvdVersion:         envdVersion,
		AllowInternetAccess: boolPtrFromLabel(labels[localSandboxAllowNetLabel]),
		VolumeMounts:        volumeMountsFromLabels(labels),
	}
}

func (r *Runtime) templateForSnapshot(snapshot ContainerSnapshot) TemplateOptions {
	templateID := strings.TrimSpace(snapshot.Configuration.Labels[localSandboxTemplateIDLabel])
	if templateID == "" {
		return TemplateOptions{}
	}
	return r.cfg.Templates[templateID]
}

func (r *Runtime) networkAttachments(ctx context.Context, containerID string, allowInternet *bool) ([]AttachmentConfig, error) {
	if allowInternet != nil && !*allowInternet {
		return nil, nil
	}
	attachments, err := r.client.DefaultNetworkAttachment(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("resolve apple container default network: %w", err)
	}
	return attachments, nil
}

func (r *Runtime) resolveVolumeMounts(ctx context.Context, mounts []api.VolumeMount) ([]api.VolumeMount, []Filesystem, error) {
	normalized := normalizeVolumeMounts(mounts)
	filesystems := make([]Filesystem, 0, len(normalized))
	for index, mount := range normalized {
		if strings.TrimSpace(mount.VolumeID) == "" && strings.TrimSpace(mount.Name) == "" {
			return nil, nil, fmt.Errorf("volume mount %d requires volumeID or name", index)
		}
		if strings.TrimSpace(mount.MountPath) == "" {
			return nil, nil, fmt.Errorf("volume mount %d requires mount path", index)
		}
		if !strings.HasPrefix(strings.TrimSpace(mount.MountPath), "/") {
			return nil, nil, fmt.Errorf("volume mount path must be absolute: %s", mount.MountPath)
		}
		volume, err := r.resolveVolumeForMount(ctx, mount)
		if err != nil {
			return nil, nil, err
		}
		runtimeVolume := runtimeVolumeFromAppleVolume(volume)
		normalized[index].VolumeID = runtimeVolume.VolumeID
		normalized[index].Name = runtimeVolume.Name
		filesystems = append(filesystems, VolumeFilesystem(volume.Name, volume.Format, volume.Source, normalized[index].MountPath, false))
	}
	return normalized, filesystems, nil
}

func (r *Runtime) resolveVolumeForMount(ctx context.Context, mount api.VolumeMount) (VolumeConfig, error) {
	candidates := []string{strings.TrimSpace(mount.VolumeID), strings.TrimSpace(mount.Name)}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		volume, err := r.client.VolumeInspect(ctx, appleVolumeName(candidate))
		if err == nil && volume.Name != "" {
			return volume, nil
		}
		if err != nil && !isAppleNotFound(err) {
			return VolumeConfig{}, fmt.Errorf("inspect apple container volume %s: %w", appleVolumeName(candidate), err)
		}
	}
	volumes, err := r.client.VolumeList(ctx)
	if err != nil {
		return VolumeConfig{}, fmt.Errorf("list apple container volumes: %w", err)
	}
	for _, volume := range volumes {
		if volume.Labels[localManagedLabel] != "true" {
			continue
		}
		runtimeVolume := runtimeVolumeFromAppleVolume(volume)
		for _, candidate := range candidates {
			if volumeMatches(runtimeVolume, candidate) {
				return volume, nil
			}
		}
	}
	return VolumeConfig{}, fmt.Errorf("%w: %s", ErrVolumeNotFound, firstNonEmpty(mount.VolumeID, mount.Name))
}

func validateEnvdBinary(path string) error {
	stat, err := os.Stat(strings.TrimSpace(path))
	if err != nil {
		return fmt.Errorf("applecontainer envd binary is not accessible: %w", err)
	}
	if stat.IsDir() {
		return fmt.Errorf("applecontainer envd binary is a directory: %s", path)
	}
	if stat.Mode()&0o111 == 0 {
		return fmt.Errorf("applecontainer envd binary is not executable: %s", path)
	}
	return nil
}

func findFreePort() (int, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(envdHost, "0"))
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address %T", listener.Addr())
	}
	return addr.Port, nil
}

func newSandboxID() string {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "sbx" + strings.ReplaceAll(strconv.FormatInt(time.Now().UnixNano(), 16), "-", "")
	}
	return "sbx" + hex.EncodeToString(bytes[:])
}

func sandboxContainerID(cfg RuntimeOptions, sandboxID string) string {
	return strings.TrimSpace(cfg.ContainerNamePrefix) + strings.TrimSpace(sandboxID)
}

func dnsConfiguration(allowInternet *bool) *DNSConfiguration {
	if allowInternet != nil && !*allowInternet {
		return nil
	}
	return &DNSConfiguration{
		Nameservers:   []string{"1.1.1.1"},
		SearchDomains: []string{},
		Options:       []string{},
	}
}

func envdArguments(port int, startCmd string) []string {
	args := []string{"-isnotfc", "-port", strconv.Itoa(port)}
	if strings.TrimSpace(startCmd) != "" {
		args = append(args, "-cmd", strings.TrimSpace(startCmd))
	}
	return args
}

func processEnvironment(values map[string]string) []string {
	env := map[string]string{
		"HOME": "/root",
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key != "" {
			env[key] = value
		}
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+env[key])
	}
	return result
}

func sandboxLabels(req CreateSandboxRequest, imageRef string, mounts []api.VolumeMount) (map[string]string, error) {
	labels := map[string]string{
		localManagedLabel:           "true",
		localImageLabel:             imageRef,
		localSandboxIDLabel:         strings.TrimSpace(req.SandboxID),
		localSandboxTemplateIDLabel: strings.TrimSpace(req.TemplateID),
	}
	if !req.CreatedAt.IsZero() {
		labels[localSandboxCreatedAtLabel] = req.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	if !req.EndAt.IsZero() {
		labels[localSandboxEndAtLabel] = req.EndAt.UTC().Format(time.RFC3339Nano)
	}
	if len(req.Metadata) > 0 {
		data, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, fmt.Errorf("encode sandbox metadata label: %w", err)
		}
		labels[localSandboxMetadataLabel] = string(data)
	}
	if req.AllowInternetAccess != nil {
		labels[localSandboxAllowNetLabel] = strconv.FormatBool(*req.AllowInternetAccess)
	}
	if len(mounts) > 0 {
		data, err := json.Marshal(mounts)
		if err != nil {
			return nil, fmt.Errorf("encode sandbox volume mounts label: %w", err)
		}
		labels[localSandboxMountsLabel] = string(data)
	}
	if len(req.EnvVars) > 0 {
		data, err := json.Marshal(req.EnvVars)
		if err != nil {
			return nil, fmt.Errorf("encode sandbox env vars label: %w", err)
		}
		labels[localSandboxEnvVarsLabel] = string(data)
	}
	for key, value := range labels {
		if strings.TrimSpace(value) == "" {
			delete(labels, key)
		}
	}
	return labels, nil
}

func normalizeVolumeMounts(mounts []api.VolumeMount) []api.VolumeMount {
	result := make([]api.VolumeMount, 0, len(mounts))
	for _, mount := range mounts {
		name := strings.TrimSpace(mount.Name)
		volumeID := strings.TrimSpace(mount.VolumeID)
		pathValue := strings.TrimSpace(mount.Path)
		mountPath := strings.TrimSpace(mount.MountPath)
		if volumeID == "" {
			volumeID = name
		}
		if name == "" {
			name = volumeID
		}
		if pathValue == "" {
			pathValue = mountPath
		}
		if mountPath == "" {
			mountPath = pathValue
		}
		if volumeID == "" && mountPath == "" {
			continue
		}
		result = append(result, api.VolumeMount{Name: name, Path: pathValue, VolumeID: volumeID, MountPath: mountPath})
	}
	return result
}

func appleVolumeName(volumeID string) string {
	volumeID = strings.TrimSpace(volumeID)
	if volumeID == "" {
		return ""
	}
	if strings.HasPrefix(volumeID, volumeNamePrefix) {
		return volumeID
	}
	return volumeNamePrefix + volumeID
}

func runtimeVolumeFromAppleVolume(volume VolumeConfig) api.VolumeMount {
	volumeID := strings.TrimSpace(volume.Labels[localVolumeIDLabel])
	if volumeID == "" {
		volumeID = strings.TrimPrefix(strings.TrimSpace(volume.Name), volumeNamePrefix)
	}
	name := strings.TrimSpace(volume.Labels[localVolumeNameLabel])
	if name == "" {
		name = volumeID
	}
	return api.VolumeMount{VolumeID: volumeID, Name: name}
}

func volumeMatches(volume api.VolumeMount, value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && (value == strings.TrimSpace(volume.VolumeID) || value == strings.TrimSpace(volume.Name))
}

func envdHostPort(ports []PublishPort, envdPort int) string {
	for _, port := range ports {
		if int(port.ContainerPort) == envdPort && strings.EqualFold(strings.TrimSpace(port.Proto), "tcp") {
			return strconv.Itoa(int(port.HostPort))
		}
	}
	return ""
}

func appleEnvdURL(hostPort string) string {
	hostPort = strings.TrimSpace(hostPort)
	if hostPort == "" {
		return ""
	}
	return "http://" + net.JoinHostPort(envdHost, hostPort)
}

func e2bStateFromAppleStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return "running"
	case "stopped":
		return "paused"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func stringMapFromLabel(value string) map[string]string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return nil
	}
	return copyStringMap(decoded)
}

func volumeMountsFromLabels(labels map[string]string) []api.VolumeMount {
	value := strings.TrimSpace(labels[localSandboxMountsLabel])
	if value == "" {
		return nil
	}
	var decoded []api.VolumeMount
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return nil
	}
	return normalizeVolumeMounts(decoded)
}

func timeFromLabel(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func boolPtrFromLabel(value string) *bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
