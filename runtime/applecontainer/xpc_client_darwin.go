//go:build darwin && cgo

package applecontainer

/*
#cgo CFLAGS: -I${SRCDIR} -fblocks
#include <stdlib.h>
#include "xpc_bridge.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	goruntime "runtime"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const (
	apiServiceName   = "com.apple.container.apiserver"
	imageServiceName = "com.apple.container.core.container-core-images"

	xpcTimeoutDefault = 120 * time.Second
	xpcTimeoutList    = 30 * time.Second
	xpcTimeoutCopy    = 300 * time.Second

	keyContainerConfig   = "containerConfig"
	keyContainerOptions  = "containerOptions"
	keyContainers        = "containers"
	keyCreateParents     = "createParents"
	keyDestinationPath   = "destinationPath"
	keyDynamicEnv        = "dynamicEnv"
	keyFileMode          = "fileMode"
	keyForceDelete       = "forceDelete"
	keyID                = "id"
	keyImageDescriptions = "imageDescriptions"
	keyKernel            = "kernel"
	keyListFilters       = "listFilters"
	keyNetworkResources  = "networkResources"
	keyProcessConfig     = "processConfig"
	keyProcessIdentifier = "processIdentifier"
	keySourcePath        = "sourcePath"
	keyStderr            = "stderr"
	keyStdin             = "stdin"
	keyStdout            = "stdout"
	keyStopOptions       = "stopOptions"
	keySystemPlatform    = "systemPlatform"
	keyVolume            = "volume"
	keyVolumeName        = "volumeName"
	keyVolumes           = "volumes"

	routeContainerBootstrap     = "containerBootstrap"
	routeContainerCopyIn        = "containerCopyIn"
	routeContainerCreate        = "containerCreate"
	routeContainerCreateProcess = "containerCreateProcess"
	routeContainerDelete        = "containerDelete"
	routeContainerList          = "containerList"
	routeContainerStartProcess  = "containerStartProcess"
	routeContainerStop          = "containerStop"
	routeGetDefaultKernel       = "getDefaultKernel"
	routeImageList              = "imageList"
	routeNetworkList            = "networkList"
	routePing                   = "ping"
	routeVolumeInspect          = "volumeInspect"
	routeVolumeList             = "volumeList"
)

type xpcBridgeConn = C.xpc_bridge_conn_t
type xpcBridgeMsg = C.xpc_bridge_msg_t

type xpcService uint8

const (
	xpcAPIService xpcService = iota
	xpcImageService
)

type XPCClient struct {
	apiConn   xpcBridgeConn
	imageConn xpcBridgeConn

	mu         sync.Mutex
	closed     bool
	kernelData []byte
}

type containerCreateOptions struct {
	AutoRemove     bool        `json:"autoRemove"`
	RootFSOverride *Filesystem `json:"rootFsOverride,omitempty"`
}

type containerStopOptions struct {
	TimeoutInSeconds int32   `json:"timeoutInSeconds"`
	Signal           *string `json:"signal,omitempty"`
}

func newNativeClient() (nativeClient, error) {
	return NewXPCClient()
}

func NewXPCClient() (*XPCClient, error) {
	apiConn, err := xpcConnect(apiServiceName)
	if err != nil {
		return nil, err
	}
	imageConn, err := xpcConnect(imageServiceName)
	if err != nil {
		C.xpc_bridge_disconnect(apiConn)
		return nil, err
	}
	return &XPCClient{apiConn: apiConn, imageConn: imageConn}, nil
}

func (c *XPCClient) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	if c.apiConn != nil {
		C.xpc_bridge_disconnect(c.apiConn)
		c.apiConn = nil
	}
	if c.imageConn != nil {
		C.xpc_bridge_disconnect(c.imageConn)
		c.imageConn = nil
	}
}

func (c *XPCClient) Ping(ctx context.Context) error {
	msg, err := newXPCMessage(routePing)
	if err != nil {
		return err
	}
	reply, err := c.sendAPI(ctx, msg, xpcTimeoutDefault)
	if err != nil {
		return err
	}
	C.xpc_bridge_msg_release(reply)
	return nil
}

func (c *XPCClient) ResolveImage(ctx context.Context, ref string) (ImageDescription, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ImageDescription{}, fmt.Errorf("image reference is required")
	}
	msg, err := newXPCMessage(routeImageList)
	if err != nil {
		return ImageDescription{}, err
	}
	reply, err := c.sendImage(ctx, msg, xpcTimeoutList)
	if err != nil {
		return ImageDescription{}, err
	}
	defer C.xpc_bridge_msg_release(reply)

	data, err := replyData(reply, keyImageDescriptions)
	if err != nil {
		return ImageDescription{}, err
	}
	var images []ImageDescription
	if err := json.Unmarshal(data, &images); err != nil {
		return ImageDescription{}, fmt.Errorf("decode imageDescriptions: %w", err)
	}
	for _, image := range images {
		if imageReferenceMatches(ref, image.Reference) {
			return image, nil
		}
	}
	return ImageDescription{}, fmt.Errorf("apple container image %q is not available locally; pull it with container image pull first", ref)
}

func (c *XPCClient) ContainerCreate(ctx context.Context, config ContainerConfiguration) error {
	configData, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode containerConfig: %w", err)
	}
	kernelData, err := c.defaultKernelData(ctx)
	if err != nil {
		return err
	}
	optionsData, err := json.Marshal(containerCreateOptions{AutoRemove: false})
	if err != nil {
		return fmt.Errorf("encode containerOptions: %w", err)
	}

	msg, err := newXPCMessage(routeContainerCreate)
	if err != nil {
		return err
	}
	setData(msg, keyContainerConfig, configData)
	setData(msg, keyKernel, kernelData)
	setData(msg, keyContainerOptions, optionsData)
	return c.sendNoBodyAPI(ctx, msg, xpcTimeoutDefault)
}

func (c *XPCClient) ContainerBootstrap(ctx context.Context, id string) error {
	msg, err := newXPCMessage(routeContainerBootstrap)
	if err != nil {
		return err
	}
	setString(msg, keyID, id)
	dynamicEnv, err := json.Marshal(map[string]string{})
	if err != nil {
		return err
	}
	setData(msg, keyDynamicEnv, dynamicEnv)
	files, err := setDevNullStdio(msg)
	if err != nil {
		C.xpc_bridge_msg_release(msg)
		return err
	}
	defer closeFiles(files)
	return c.sendNoBodyAPI(ctx, msg, xpcTimeoutDefault)
}

func (c *XPCClient) ContainerStartProcess(ctx context.Context, containerID, processID string) error {
	msg, err := newXPCMessage(routeContainerStartProcess)
	if err != nil {
		return err
	}
	setString(msg, keyID, containerID)
	setString(msg, keyProcessIdentifier, processID)
	return c.sendNoBodyAPI(ctx, msg, xpcTimeoutDefault)
}

func (c *XPCClient) ContainerCopyIn(ctx context.Context, id, srcPath, dstPath string, mode uint32) error {
	msg, err := newXPCMessage(routeContainerCopyIn)
	if err != nil {
		return err
	}
	setString(msg, keyID, id)
	setString(msg, keySourcePath, srcPath)
	setString(msg, keyDestinationPath, dstPath)
	setUint64(msg, keyFileMode, uint64(mode))
	setBool(msg, keyCreateParents, true)
	return c.sendNoBodyAPI(ctx, msg, xpcTimeoutCopy)
}

func (c *XPCClient) ContainerCreateProcess(ctx context.Context, containerID, processID string, config ProcessConfiguration) error {
	configData, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode processConfig: %w", err)
	}
	msg, err := newXPCMessage(routeContainerCreateProcess)
	if err != nil {
		return err
	}
	setString(msg, keyID, containerID)
	setString(msg, keyProcessIdentifier, processID)
	setData(msg, keyProcessConfig, configData)
	files, err := setDevNullStdio(msg)
	if err != nil {
		C.xpc_bridge_msg_release(msg)
		return err
	}
	defer closeFiles(files)
	return c.sendNoBodyAPI(ctx, msg, xpcTimeoutDefault)
}

func (c *XPCClient) ContainerStop(ctx context.Context, id string) error {
	options, err := json.Marshal(containerStopOptions{TimeoutInSeconds: 5})
	if err != nil {
		return err
	}
	msg, err := newXPCMessage(routeContainerStop)
	if err != nil {
		return err
	}
	setString(msg, keyID, id)
	setData(msg, keyStopOptions, options)
	return c.sendNoBodyAPI(ctx, msg, xpcTimeoutDefault)
}

func (c *XPCClient) ContainerDelete(ctx context.Context, id string, force bool) error {
	msg, err := newXPCMessage(routeContainerDelete)
	if err != nil {
		return err
	}
	setString(msg, keyID, id)
	setBool(msg, keyForceDelete, force)
	return c.sendNoBodyAPI(ctx, msg, xpcTimeoutDefault)
}

func (c *XPCClient) ContainerList(ctx context.Context, filters ContainerListFilters) ([]ContainerSnapshot, error) {
	filterData, err := json.Marshal(filters)
	if err != nil {
		return nil, fmt.Errorf("encode listFilters: %w", err)
	}
	msg, err := newXPCMessage(routeContainerList)
	if err != nil {
		return nil, err
	}
	setData(msg, keyListFilters, filterData)
	reply, err := c.sendAPI(ctx, msg, xpcTimeoutList)
	if err != nil {
		return nil, err
	}
	defer C.xpc_bridge_msg_release(reply)

	data, err := replyData(reply, keyContainers)
	if err != nil {
		if errorsIsMissingXPCData(err) {
			return []ContainerSnapshot{}, nil
		}
		return nil, err
	}
	var containers []ContainerSnapshot
	if err := json.Unmarshal(data, &containers); err != nil {
		return nil, fmt.Errorf("decode containers: %w", err)
	}
	return containers, nil
}

func (c *XPCClient) VolumeInspect(ctx context.Context, name string) (VolumeConfig, error) {
	msg, err := newXPCMessage(routeVolumeInspect)
	if err != nil {
		return VolumeConfig{}, err
	}
	setString(msg, keyVolumeName, name)
	reply, err := c.sendAPI(ctx, msg, xpcTimeoutDefault)
	if err != nil {
		return VolumeConfig{}, err
	}
	defer C.xpc_bridge_msg_release(reply)

	data, err := replyData(reply, keyVolume)
	if err != nil {
		return VolumeConfig{}, err
	}
	var volume VolumeConfig
	if err := json.Unmarshal(data, &volume); err != nil {
		return VolumeConfig{}, fmt.Errorf("decode volume: %w", err)
	}
	return volume, nil
}

func (c *XPCClient) VolumeList(ctx context.Context) ([]VolumeConfig, error) {
	msg, err := newXPCMessage(routeVolumeList)
	if err != nil {
		return nil, err
	}
	reply, err := c.sendAPI(ctx, msg, xpcTimeoutDefault)
	if err != nil {
		return nil, err
	}
	defer C.xpc_bridge_msg_release(reply)

	data, err := replyData(reply, keyVolumes)
	if err != nil {
		if errorsIsMissingXPCData(err) {
			return []VolumeConfig{}, nil
		}
		return nil, err
	}
	var volumes []VolumeConfig
	if err := json.Unmarshal(data, &volumes); err != nil {
		return nil, fmt.Errorf("decode volumes: %w", err)
	}
	return volumes, nil
}

func (c *XPCClient) DefaultNetworkAttachment(ctx context.Context, containerID string) ([]AttachmentConfig, error) {
	msg, err := newXPCMessage(routeNetworkList)
	if err != nil {
		return nil, err
	}
	reply, err := c.sendAPI(ctx, msg, xpcTimeoutList)
	if err != nil {
		return nil, err
	}
	defer C.xpc_bridge_msg_release(reply)

	data, err := replyData(reply, keyNetworkResources)
	if err != nil {
		return nil, err
	}
	var networks []NetworkResource
	if err := json.Unmarshal(data, &networks); err != nil {
		return nil, fmt.Errorf("decode networkResources: %w", err)
	}
	for _, network := range networks {
		if network.Configuration.Labels[resourceRoleLabel] == resourceRoleBuiltin {
			return []AttachmentConfig{defaultNetworkAttachment(network.Configuration.Name, containerID)}, nil
		}
	}
	for _, network := range networks {
		if network.Configuration.Name == "default" {
			return []AttachmentConfig{defaultNetworkAttachment(network.Configuration.Name, containerID)}, nil
		}
	}
	return nil, fmt.Errorf("apple container builtin network is not available")
}

func (c *XPCClient) defaultKernelData(ctx context.Context) ([]byte, error) {
	c.mu.Lock()
	if len(c.kernelData) > 0 {
		data := append([]byte(nil), c.kernelData...)
		c.mu.Unlock()
		return data, nil
	}
	c.mu.Unlock()

	platformData, err := json.Marshal(defaultSystemPlatform())
	if err != nil {
		return nil, err
	}
	msg, err := newXPCMessage(routeGetDefaultKernel)
	if err != nil {
		return nil, err
	}
	setData(msg, keySystemPlatform, platformData)
	reply, err := c.sendAPI(ctx, msg, xpcTimeoutDefault)
	if err != nil {
		return nil, fmt.Errorf("get default apple container kernel; run 'container system kernel set --recommended' if no default kernel is configured: %w", err)
	}
	defer C.xpc_bridge_msg_release(reply)

	kernelData, err := replyData(reply, keyKernel)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if !c.closed {
		c.kernelData = append([]byte(nil), kernelData...)
	}
	c.mu.Unlock()
	return kernelData, nil
}

func (c *XPCClient) sendNoBodyAPI(ctx context.Context, msg xpcBridgeMsg, timeout time.Duration) error {
	reply, err := c.send(ctx, xpcAPIService, msg, timeout)
	if err != nil {
		return err
	}
	C.xpc_bridge_msg_release(reply)
	return nil
}

func (c *XPCClient) sendAPI(ctx context.Context, msg xpcBridgeMsg, timeout time.Duration) (xpcBridgeMsg, error) {
	return c.send(ctx, xpcAPIService, msg, timeout)
}

func (c *XPCClient) sendImage(ctx context.Context, msg xpcBridgeMsg, timeout time.Duration) (xpcBridgeMsg, error) {
	return c.send(ctx, xpcImageService, msg, timeout)
}

func (c *XPCClient) send(ctx context.Context, service xpcService, msg xpcBridgeMsg, timeout time.Duration) (xpcBridgeMsg, error) {
	defer C.xpc_bridge_msg_release(msg)

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timeoutMS := timeoutMilliseconds(ctx, timeout)
	errBuf := make([]C.char, 4096)
	var reply xpcBridgeMsg

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("apple container XPC client is closed")
	}
	conn, serviceName := c.connectionLocked(service)
	if conn == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("apple container XPC %s connection is closed", serviceName)
	}
	rc := C.xpc_bridge_send(conn, msg, C.uint64_t(timeoutMS), &reply, &errBuf[0], C.size_t(len(errBuf)))
	c.mu.Unlock()

	if rc != 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("xpc send: %s", cString(&errBuf[0]))
	}
	if reply == nil {
		return nil, fmt.Errorf("xpc reply is nil")
	}
	if err := checkReply(reply); err != nil {
		C.xpc_bridge_msg_release(reply)
		return nil, err
	}
	return reply, nil
}

func (c *XPCClient) connectionLocked(service xpcService) (xpcBridgeConn, string) {
	switch service {
	case xpcImageService:
		return c.imageConn, imageServiceName
	default:
		return c.apiConn, apiServiceName
	}
}

func xpcConnect(serviceName string) (xpcBridgeConn, error) {
	cService := C.CString(serviceName)
	defer C.free(unsafe.Pointer(cService))
	conn := C.xpc_bridge_connect(cService)
	if conn == nil {
		return nil, fmt.Errorf("connect to XPC service %s", serviceName)
	}
	return conn, nil
}

func newXPCMessage(route string) (xpcBridgeMsg, error) {
	cRoute := C.CString(route)
	defer C.free(unsafe.Pointer(cRoute))
	msg := C.xpc_bridge_msg_create(cRoute)
	if msg == nil {
		return nil, fmt.Errorf("create XPC message for route %s", route)
	}
	return msg, nil
}

func setString(msg xpcBridgeMsg, key string, value string) {
	cKey := C.CString(key)
	cValue := C.CString(value)
	defer C.free(unsafe.Pointer(cKey))
	defer C.free(unsafe.Pointer(cValue))
	C.xpc_bridge_msg_set_string(msg, cKey, cValue)
}

func setBool(msg xpcBridgeMsg, key string, value bool) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	if value {
		C.xpc_bridge_msg_set_bool(msg, cKey, 1)
		return
	}
	C.xpc_bridge_msg_set_bool(msg, cKey, 0)
}

func setUint64(msg xpcBridgeMsg, key string, value uint64) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	C.xpc_bridge_msg_set_uint64(msg, cKey, C.uint64_t(value))
}

func setData(msg xpcBridgeMsg, key string, data []byte) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	if len(data) == 0 {
		C.xpc_bridge_msg_set_data(msg, cKey, nil, 0)
		return
	}
	C.xpc_bridge_msg_set_data(msg, cKey, (*C.uint8_t)(unsafe.Pointer(&data[0])), C.size_t(len(data)))
}

func setFD(msg xpcBridgeMsg, key string, fd uintptr) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	C.xpc_bridge_msg_set_fd(msg, cKey, C.int(fd))
}

func setDevNullStdio(msg xpcBridgeMsg) ([]*os.File, error) {
	files := make([]*os.File, 0, 3)
	for _, key := range []string{keyStdin, keyStdout, keyStderr} {
		file, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			closeFiles(files)
			return nil, fmt.Errorf("open %s for stdio: %w", os.DevNull, err)
		}
		files = append(files, file)
		setFD(msg, key, file.Fd())
	}
	return files, nil
}

func closeFiles(files []*os.File) {
	for _, file := range files {
		_ = file.Close()
	}
}

func replyData(reply xpcBridgeMsg, key string) ([]byte, error) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var length C.size_t
	ptr := C.xpc_bridge_reply_get_data(reply, cKey, &length)
	if ptr == nil || length == 0 {
		return nil, missingXPCDataError{key: key}
	}
	return C.GoBytes(unsafe.Pointer(ptr), C.int(length)), nil
}

func checkReply(reply xpcBridgeMsg) error {
	if protocolErr, ok := replyProtocolError(reply); ok {
		return protocolErr
	}

	errBuf := make([]C.char, 4096)
	rc := C.xpc_bridge_reply_check_error(reply, &errBuf[0], C.size_t(len(errBuf)))
	if rc == 0 {
		return nil
	}
	return fmt.Errorf("apple container XPC error: %s", cString(&errBuf[0]))
}

func replyProtocolError(reply xpcBridgeMsg) (error, bool) {
	data, err := replyData(reply, xpcProtocolErrorKey)
	if err != nil {
		if errorsIsMissingXPCData(err) {
			return nil, false
		}
		return err, true
	}
	protocolErr, err := decodeXPCProtocolError(data)
	if err != nil {
		return err, true
	}
	return protocolErr, true
}

func timeoutMilliseconds(ctx context.Context, fallback time.Duration) uint64 {
	if fallback <= 0 {
		fallback = xpcTimeoutDefault
	}
	timeout := fallback
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}
	if timeout <= 0 {
		return 1
	}
	timeoutMS := timeout.Milliseconds()
	if timeoutMS <= 0 {
		return 1
	}
	return uint64(timeoutMS)
}

func cString(value *C.char) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(C.GoString(value))
}

type missingXPCDataError struct {
	key string
}

func (e missingXPCDataError) Error() string {
	return fmt.Sprintf("missing XPC data key %s", e.key)
}

func errorsIsMissingXPCData(err error) bool {
	_, ok := err.(missingXPCDataError)
	return ok
}

func defaultSystemPlatform() SystemPlatform {
	architecture := goruntime.GOARCH
	switch architecture {
	case "amd64":
		architecture = "amd64"
	case "arm64":
		architecture = "arm64"
	}
	return SystemPlatform{Architecture: architecture, OS: "linux"}
}

func defaultNetworkAttachment(networkName string, containerID string) AttachmentConfig {
	return AttachmentConfig{
		Network: networkName,
		Options: AttachmentOptions{
			Hostname: strings.TrimSpace(containerID),
			MTU:      1280,
		},
	}
}

func imageReferenceMatches(want string, got string) bool {
	wantCandidates := imageReferenceCandidates(want)
	gotCandidates := imageReferenceCandidates(got)
	for candidate := range wantCandidates {
		if _, ok := gotCandidates[candidate]; ok {
			return true
		}
	}
	return false
}

func imageReferenceCandidates(ref string) map[string]struct{} {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return map[string]struct{}{}
	}
	candidates := map[string]struct{}{ref: {}}
	short := shortImageReference(ref)
	if short != "" {
		candidates[short] = struct{}{}
	}
	if !imageReferenceHasTagOrDigest(ref) {
		candidates[ref+":latest"] = struct{}{}
		if short != "" {
			candidates[short+":latest"] = struct{}{}
		}
	}
	first := strings.Split(ref, "/")[0]
	if !strings.Contains(first, ".") && !strings.Contains(first, ":") {
		name := ref
		if !strings.Contains(name, "/") {
			name = "library/" + name
		}
		candidates["docker.io/"+name] = struct{}{}
		candidates["registry-1.docker.io/"+name] = struct{}{}
		if !imageReferenceHasTagOrDigest(name) {
			candidates["docker.io/"+name+":latest"] = struct{}{}
			candidates["registry-1.docker.io/"+name+":latest"] = struct{}{}
		}
	}
	return candidates
}

func shortImageReference(ref string) string {
	for _, prefix := range []string{"registry-1.docker.io/library/", "docker.io/library/", "registry-1.docker.io/", "docker.io/"} {
		if strings.HasPrefix(ref, prefix) {
			return strings.TrimPrefix(ref, prefix)
		}
	}
	return ""
}

func imageReferenceHasTagOrDigest(ref string) bool {
	if strings.Contains(ref, "@") {
		return true
	}
	lastSlash := strings.LastIndex(ref, "/")
	return strings.Contains(ref[lastSlash+1:], ":")
}
