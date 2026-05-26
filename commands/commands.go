package commands

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/e2b-dev/e2b-go-sdk/envd"
	"github.com/e2b-dev/e2b-go-sdk/envd/process"
)

const (
	defaultProcessConnectionTimeoutMs = 60000
	keepalivePingIntervalSec          = 50
	keepalivePingHeader               = "Keepalive-Ping-Interval"
)

type CommandRequestOpts struct {
	RequestTimeoutMs *int
}

type CommandStartOpts struct {
	CommandRequestOpts
	Background bool
	Cwd        string
	User       string
	Envs       map[string]string
	OnStdout   func(data Stdout)
	OnStderr   func(data Stderr)
	Stdin      bool
	StdinOpt   *bool
	TimeoutMs  *int
}

type CommandConnectOpts struct {
	CommandRequestOpts
	OnStdout  func(data Stdout)
	OnStderr  func(data Stderr)
	TimeoutMs *int
}

type ProcessInfo struct {
	Pid  uint32
	Tag  string
	Cmd  string
	Args []string
	Envs map[string]string
	Cwd  string
}

type connectionConfig struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            bool
	RequestTimeoutMs int
	Headers          map[string]string
}

type Commands struct {
	connectionConfig *connectionConfig
	envdVersion      string
	httpClient       *http.Client
}

type streamEnvelope struct {
	payload json.RawMessage
	err     error
}

func NewCommands(cfg *struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            bool
	RequestTimeoutMs int
	Headers          map[string]string
}, envdVersion string) *Commands {
	var resolved *connectionConfig
	if cfg != nil {
		resolved = &connectionConfig{
			ApiKey:           cfg.ApiKey,
			AccessToken:      cfg.AccessToken,
			Domain:           cfg.Domain,
			ApiUrl:           cfg.ApiUrl,
			SandboxUrl:       cfg.SandboxUrl,
			Debug:            cfg.Debug,
			RequestTimeoutMs: cfg.RequestTimeoutMs,
			Headers:          cfg.Headers,
		}
	}
	return &Commands{
		connectionConfig: resolved,
		envdVersion:      envdVersion,
		httpClient:       &http.Client{},
	}
}

func (c *Commands) baseUrl() string {
	return c.connectionConfig.SandboxUrl
}

func (c *Commands) headers(user string) map[string]string {
	h := make(map[string]string)
	for k, v := range c.connectionConfig.Headers {
		h[k] = v
	}
	for k, v := range envd.AuthenticationHeader(c.envdVersion, user) {
		h[k] = v
	}
	return h
}

func requestContext(ctx context.Context, timeoutMs *int) (context.Context, context.CancelFunc) {
	if timeoutMs == nil {
		return ctx, func() {}
	}
	if *timeoutMs == 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, time.Duration(*timeoutMs)*time.Millisecond)
}

func (c *Commands) connectUnary(ctx context.Context, path string, reqBody interface{}, respBody interface{}, user string) error {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	url := c.baseUrl() + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers(user) {
		req.Header.Set(k, v)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		// Try to parse Connect error
		var connectErr struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &connectErr) == nil && connectErr.Code != "" {
			return envd.HandleRpcError(connectErr.Code, connectErr.Message)
		}
		return fmt.Errorf("connect RPC error: %d %s", resp.StatusCode, string(body))
	}
	if respBody != nil {
		return json.Unmarshal(body, respBody)
	}
	return nil
}

// connectServerStream opens a server-streaming Connect RPC call and returns the response body for reading envelopes.
func (c *Commands) connectServerStream(ctx context.Context, path string, reqBody interface{}, user string) (io.ReadCloser, error) {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	url := c.baseUrl() + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/connect+json")
	req.Header.Set(keepalivePingHeader, fmt.Sprintf("%d", keepalivePingIntervalSec))
	for k, v := range c.headers(user) {
		req.Header.Set(k, v)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var connectErr struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &connectErr) == nil && connectErr.Code != "" {
			return nil, envd.HandleRpcError(connectErr.Code, connectErr.Message)
		}
		return nil, fmt.Errorf("connect RPC error: %d %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

// readStreamEnvelopes reads Connect protocol envelopes (5-byte header: 1 flag + 4 length, then payload).
func readStreamEnvelopes(reader io.Reader, ch chan<- streamEnvelope) {
	defer close(ch)
	header := make([]byte, 5)
	for {
		_, err := io.ReadFull(reader, header)
		if err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				ch <- streamEnvelope{err: err}
			}
			return
		}
		flags := header[0]
		length := binary.BigEndian.Uint32(header[1:5])
		payload := make([]byte, length)
		_, err = io.ReadFull(reader, payload)
		if err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				ch <- streamEnvelope{err: err}
			}
			return
		}
		// flags & 0x02 means end-of-stream / trailers
		if flags&0x02 != 0 {
			if err := envd.ParseConnectEndStreamError(payload); err != nil {
				ch <- streamEnvelope{err: err}
			}
			return
		}
		ch <- streamEnvelope{payload: json.RawMessage(payload)}
	}
}

func (c *Commands) List(ctx context.Context, opts *CommandRequestOpts) ([]ProcessInfo, error) {
	reqCtx, cancel := requestContext(ctx, c.requestTimeout(opts))
	defer cancel()

	var resp process.ListResponse
	if err := c.connectUnary(reqCtx, "/process.Process/List", &process.ListRequest{}, &resp, ""); err != nil {
		return nil, err
	}
	result := make([]ProcessInfo, 0, len(resp.Processes))
	for _, p := range resp.Processes {
		info := ProcessInfo{
			Pid: p.Pid,
			Tag: p.Tag,
		}
		if p.Config != nil {
			info.Cmd = p.Config.Cmd
			info.Args = p.Config.Args
			info.Envs = p.Config.Envs
			info.Cwd = p.Config.Cwd
		}
		result = append(result, info)
	}
	return result, nil
}

func (c *Commands) SendStdin(ctx context.Context, pid uint32, data []byte, opts *CommandRequestOpts) error {
	reqCtx, cancel := requestContext(ctx, c.requestTimeout(opts))
	defer cancel()

	req := &process.SendInputRequest{
		Pid:   pid,
		Stdin: data,
	}
	return c.connectUnary(reqCtx, "/process.Process/SendInput", req, nil, "")
}

func (c *Commands) closeStdin(ctx context.Context, pid uint32, opts *CommandRequestOpts) error {
	if !versionGTE(c.envdVersion, envd.EnvdClose) {
		return fmt.Errorf("Sandbox envd version %s doesn't support closeStdin. Please rebuild your template to pick up the latest sandbox version.", c.envdVersion)
	}
	reqCtx, cancel := requestContext(ctx, c.requestTimeout(opts))
	defer cancel()
	req := &process.CloseStdinRequest{Pid: pid}
	return c.connectUnary(reqCtx, "/process.Process/CloseStdin", req, nil, "")
}

func (c *Commands) Kill(ctx context.Context, pid uint32, opts *CommandRequestOpts) (bool, error) {
	reqCtx, cancel := requestContext(ctx, c.requestTimeout(opts))
	defer cancel()

	req := &process.SendSignalRequest{
		Pid:    pid,
		Signal: process.SignalSIGKILL,
	}
	err := c.connectUnary(reqCtx, "/process.Process/SendSignal", req, nil, "")
	if err != nil {
		if rpcErr, ok := err.(*envd.RpcError); ok {
			if rpcErr.Code == "not_found" || strings.Contains(rpcErr.Message, "not found") {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

func (c *Commands) Connect(ctx context.Context, pid uint32, opts *CommandConnectOpts) (*CommandHandle, error) {
	if opts == nil {
		opts = &CommandConnectOpts{}
	}
	user := ""
	req := &process.ConnectRequest{Pid: pid}
	requestCtx, clearRequestTimeout, cancelRequestTimeout := requestTimeoutStreamContext(ctx, c.requestTimeoutFromConnectOpts(opts))
	streamCtx, streamCancel := streamContext(requestCtx, opts.TimeoutMs, defaultProcessConnectionTimeoutMs)
	body, err := c.connectServerStream(streamCtx, "/process.Process/Connect", req, user)
	if err != nil {
		streamCancel()
		cancelRequestTimeout()
		return nil, err
	}

	ch := make(chan streamEnvelope, 16)
	go readStreamEnvelopes(body, ch)

	firstMsg, ok, err := waitForFirstEvent(ch, c.requestTimeoutFromConnectOpts(opts))
	if err != nil {
		streamCancel()
		cancelRequestTimeout()
		body.Close()
		return nil, err
	}
	if !ok {
		streamCancel()
		cancelRequestTimeout()
		body.Close()
		return nil, fmt.Errorf("Expected start event")
	}
	clearRequestTimeout()

	var firstEvent process.ProcessEvent
	if err := json.Unmarshal(firstMsg, &firstEvent); err != nil {
		streamCancel()
		body.Close()
		return nil, fmt.Errorf("failed to parse connect start event: %w", err)
	}
	if firstEvent.Start == nil {
		streamCancel()
		body.Close()
		return nil, fmt.Errorf("Expected start event")
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	handle := newCommandHandle(pid, func() {
		cancel()
		streamCancel()
		body.Close()
	}, func() (bool, error) {
		return c.Kill(ctx, pid, nil)
	}, opts.OnStdout, opts.OnStderr)

	go func() {
		defer cancelRequestTimeout()
		defer streamCancel()
		defer body.Close()
		for {
			select {
			case <-cancelCtx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					if err := streamCtx.Err(); err != nil {
						handle.setWaitError(envd.HandleStreamContextError(err))
						return
					}
					handle.setWaitError(errProcessExitedWithoutResult)
					return
				}
				if msg.err != nil {
					handle.setWaitError(msg.err)
					return
				}
				c.handleProcessEvent(msg.payload, handle)
			}
		}
	}()

	return handle, nil
}

func (c *Commands) Run(ctx context.Context, cmd string, opts *CommandStartOpts) (*CommandResult, error) {
	if opts != nil && opts.Background {
		return nil, fmt.Errorf("Commands.Run does not support background execution; use RunBackground instead")
	}
	handle, err := c.start(ctx, cmd, opts)
	if err != nil {
		return nil, err
	}
	return handle.Wait()
}

func (c *Commands) RunBackground(ctx context.Context, cmd string, opts *CommandStartOpts) (*CommandHandle, error) {
	return c.start(ctx, cmd, opts)
}

func (c *Commands) start(ctx context.Context, cmd string, opts *CommandStartOpts) (*CommandHandle, error) {
	if opts == nil {
		opts = &CommandStartOpts{}
	}
	stdinEnabled, stdinExplicit := resolveCommandStdin(opts)
	if stdinExplicit && !stdinEnabled && !versionGTE(c.envdVersion, envd.EnvdCommandsStdin) {
		return nil, fmt.Errorf("Sandbox envd version %s can't specify stdin, it's always turned on. Please rebuild your template if you need this feature.", c.envdVersion)
	}

	user := opts.User
	envs := opts.Envs

	startReq := &process.StartRequest{
		Process: &process.ProcessConfig{
			Cmd:  "/bin/bash",
			Args: []string{"-l", "-c", cmd},
			Envs: envs,
			Cwd:  opts.Cwd,
		},
		Stdin: stdinEnabled,
	}

	requestCtx, clearRequestTimeout, cancelRequestTimeout := requestTimeoutStreamContext(ctx, c.requestTimeoutFromStartOpts(opts))
	streamCtx, streamCancel := streamContext(requestCtx, opts.TimeoutMs, defaultProcessConnectionTimeoutMs)
	body, err := c.connectServerStream(streamCtx, "/process.Process/Start", startReq, user)
	if err != nil {
		streamCancel()
		cancelRequestTimeout()
		return nil, err
	}

	// Read the first envelope to get the PID
	ch := make(chan streamEnvelope, 16)
	go readStreamEnvelopes(body, ch)

	firstMsg, ok, err := waitForFirstEvent(ch, c.requestTimeoutFromStartOpts(opts))
	if err != nil {
		streamCancel()
		cancelRequestTimeout()
		body.Close()
		return nil, err
	}
	if !ok {
		streamCancel()
		cancelRequestTimeout()
		body.Close()
		return nil, fmt.Errorf("Expected start event")
	}
	clearRequestTimeout()

	var event process.ProcessEvent
	if err := json.Unmarshal(firstMsg, &event); err != nil {
		streamCancel()
		body.Close()
		return nil, fmt.Errorf("failed to parse start event: %w", err)
	}
	if event.Start == nil {
		streamCancel()
		body.Close()
		return nil, fmt.Errorf("Expected start event")
	}

	pid := event.Start.Pid
	cancelCtx, cancel := context.WithCancel(ctx)

	handle := newCommandHandle(pid, func() {
		cancel()
		streamCancel()
		body.Close()
	}, func() (bool, error) {
		return c.Kill(ctx, pid, nil)
	}, opts.OnStdout, opts.OnStderr)

	go func() {
		defer cancelRequestTimeout()
		defer streamCancel()
		defer body.Close()
		for {
			select {
			case <-cancelCtx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					if err := streamCtx.Err(); err != nil {
						handle.setWaitError(envd.HandleStreamContextError(err))
						return
					}
					handle.setWaitError(errProcessExitedWithoutResult)
					return
				}
				if msg.err != nil {
					handle.setWaitError(msg.err)
					return
				}
				c.handleProcessEvent(msg.payload, handle)
				if handle.GetExitCode() != nil {
					return
				}
			}
		}
	}()

	return handle, nil
}

func (c *Commands) requestTimeout(opts *CommandRequestOpts) *int {
	if opts != nil && opts.RequestTimeoutMs != nil {
		return opts.RequestTimeoutMs
	}
	if c.connectionConfig.RequestTimeoutMs <= 0 {
		return nil
	}
	timeout := c.connectionConfig.RequestTimeoutMs
	return &timeout
}

func (c *Commands) requestTimeoutFromStartOpts(opts *CommandStartOpts) *int {
	if opts != nil && opts.RequestTimeoutMs != nil {
		return opts.RequestTimeoutMs
	}
	if c.connectionConfig.RequestTimeoutMs <= 0 {
		return nil
	}
	timeout := c.connectionConfig.RequestTimeoutMs
	return &timeout
}

func (c *Commands) requestTimeoutFromConnectOpts(opts *CommandConnectOpts) *int {
	if opts != nil && opts.RequestTimeoutMs != nil {
		return opts.RequestTimeoutMs
	}
	if c.connectionConfig.RequestTimeoutMs <= 0 {
		return nil
	}
	timeout := c.connectionConfig.RequestTimeoutMs
	return &timeout
}

func streamContext(ctx context.Context, timeoutMs *int, defaultTimeoutMs int) (context.Context, context.CancelFunc) {
	if timeoutMs == nil {
		return context.WithTimeout(ctx, time.Duration(defaultTimeoutMs)*time.Millisecond)
	}
	if *timeoutMs == 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, time.Duration(*timeoutMs)*time.Millisecond)
}

func requestTimeoutStreamContext(ctx context.Context, timeoutMs *int) (context.Context, func(), context.CancelFunc) {
	if timeoutMs == nil || *timeoutMs == 0 {
		return ctx, func() {}, func() {}
	}

	requestCtx, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(time.Duration(*timeoutMs)*time.Millisecond, cancel)

	return requestCtx, func() {
			timer.Stop()
		}, func() {
			timer.Stop()
			cancel()
		}
}

func waitForFirstEvent(ch <-chan streamEnvelope, timeoutMs *int) (json.RawMessage, bool, error) {
	if timeoutMs == nil || *timeoutMs == 0 {
		msg, ok := <-ch
		if !ok {
			return nil, false, nil
		}
		if msg.err != nil {
			return nil, false, msg.err
		}
		return msg.payload, true, nil
	}

	select {
	case msg, ok := <-ch:
		if !ok {
			return nil, false, nil
		}
		if msg.err != nil {
			return nil, false, msg.err
		}
		return msg.payload, true, nil
	case <-time.After(time.Duration(*timeoutMs) * time.Millisecond):
		return nil, false, envd.HandleRequestTimeoutError()
	}
}

func (c *Commands) handleProcessEvent(msg json.RawMessage, handle *CommandHandle) {
	var event process.ProcessEvent
	if err := json.Unmarshal(msg, &event); err != nil {
		return
	}
	if event.Data != nil {
		if len(event.Data.Stdout) > 0 {
			handle.appendStdout(string(event.Data.Stdout))
		}
		if len(event.Data.Stderr) > 0 {
			handle.appendStderr(string(event.Data.Stderr))
		}
	}
	if event.End != nil {
		handle.setEnd(int(event.End.ExitCode), event.End.Error)
	}
}

// versionGTE returns true if version >= minVersion (semver comparison).
func versionGTE(version, minVersion string) bool {
	if version == "" {
		return true // assume latest if unknown
	}
	parseSemver := func(v string) (int, int, int) {
		var major, minor, patch int
		fmt.Sscanf(v, "%d.%d.%d", &major, &minor, &patch)
		return major, minor, patch
	}
	maj1, min1, pat1 := parseSemver(version)
	maj2, min2, pat2 := parseSemver(minVersion)
	if maj1 != maj2 {
		return maj1 > maj2
	}
	if min1 != min2 {
		return min1 > min2
	}
	return pat1 >= pat2
}

func resolveCommandStdin(opts *CommandStartOpts) (enabled bool, explicit bool) {
	if opts == nil {
		return false, false
	}
	if opts.StdinOpt != nil {
		return *opts.StdinOpt, true
	}
	if opts.Stdin {
		return true, true
	}
	return false, false
}
