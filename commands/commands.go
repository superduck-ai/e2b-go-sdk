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

type CommandRequestOpts struct {
	RequestTimeoutMs *int
}

type CommandStartOpts struct {
	CommandRequestOpts
	Background bool
	Cwd        string
	User       string
	Envs       map[string]string
	OnStdout   func(data string)
	OnStderr   func(data string)
	Stdin      bool
	TimeoutMs  *int
}

type CommandConnectOpts struct {
	CommandRequestOpts
	OnStdout  func(data string)
	OnStderr  func(data string)
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

// ConnectionConfig holds the connection configuration passed from the parent package.
type ConnectionConfig struct {
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
	connectionConfig *ConnectionConfig
	envdVersion      string
	httpClient       *http.Client
}

func NewCommands(connectionConfig *ConnectionConfig, envdVersion string) *Commands {
	timeout := time.Duration(connectionConfig.RequestTimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &Commands{
		connectionConfig: connectionConfig,
		envdVersion:      envdVersion,
		httpClient:       &http.Client{Timeout: timeout},
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
	if user == "" {
		user = "user"
	}
	for k, v := range envd.AuthenticationHeader(c.envdVersion, user) {
		h[k] = v
	}
	return h
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
func readStreamEnvelopes(reader io.Reader, ch chan<- json.RawMessage) {
	defer close(ch)
	header := make([]byte, 5)
	for {
		_, err := io.ReadFull(reader, header)
		if err != nil {
			return
		}
		flags := header[0]
		length := binary.BigEndian.Uint32(header[1:5])
		payload := make([]byte, length)
		_, err = io.ReadFull(reader, payload)
		if err != nil {
			return
		}
		// flags & 0x02 means end-of-stream / trailers
		if flags&0x02 != 0 {
			return
		}
		ch <- json.RawMessage(payload)
	}
}

func (c *Commands) List(ctx context.Context, opts *CommandRequestOpts) ([]ProcessInfo, error) {
	var resp process.ListResponse
	if err := c.connectUnary(ctx, "/process.Process/List", &process.ListRequest{}, &resp, ""); err != nil {
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
	req := &process.SendInputRequest{
		Pid:   pid,
		Stdin: data,
	}
	return c.connectUnary(ctx, "/process.Process/SendInput", req, nil, "")
}

func (c *Commands) CloseStdin(ctx context.Context, pid uint32, opts *CommandRequestOpts) error {
	if !versionGTE(c.envdVersion, envd.EnvdClose) {
		return fmt.Errorf("CloseStdin requires envd version >= %s, got %s", envd.EnvdClose, c.envdVersion)
	}
	req := &process.CloseStdinRequest{Pid: pid}
	return c.connectUnary(ctx, "/process.Process/CloseStdin", req, nil, "")
}

func (c *Commands) Kill(ctx context.Context, pid uint32, opts *CommandRequestOpts) (bool, error) {
	req := &process.SendSignalRequest{
		Pid:    pid,
		Signal: process.SignalSIGKILL,
	}
	err := c.connectUnary(ctx, "/process.Process/SendSignal", req, nil, "")
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
	body, err := c.connectServerStream(ctx, "/process.Process/Connect", req, user)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	handle := NewCommandHandle(pid, func() {
		cancel()
		body.Close()
	}, func() (bool, error) {
		return c.Kill(ctx, pid, nil)
	}, opts.OnStdout, opts.OnStderr)

	go func() {
		ch := make(chan json.RawMessage, 16)
		go readStreamEnvelopes(body, ch)
		for {
			select {
			case <-cancelCtx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				c.handleProcessEvent(msg, handle)
			}
		}
	}()

	return handle, nil
}

func (c *Commands) Run(ctx context.Context, cmd string, opts *CommandStartOpts) (*CommandResult, error) {
	handle, err := c.start(ctx, cmd, opts)
	if err != nil {
		return nil, err
	}
	if opts != nil && opts.Background {
		return nil, nil
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

	user := opts.User
	envs := opts.Envs

	startReq := &process.StartRequest{
		Process: &process.ProcessConfig{
			Cmd:  "/bin/bash",
			Args: []string{"-l", "-c", cmd},
			Envs: envs,
			Cwd:  opts.Cwd,
		},
	}

	body, err := c.connectServerStream(ctx, "/process.Process/Start", startReq, user)
	if err != nil {
		return nil, err
	}

	// Read the first envelope to get the PID
	ch := make(chan json.RawMessage, 16)
	go readStreamEnvelopes(body, ch)

	firstMsg, ok := <-ch
	if !ok {
		body.Close()
		return nil, fmt.Errorf("stream closed before receiving start event")
	}

	var event process.ProcessEvent
	if err := json.Unmarshal(firstMsg, &event); err != nil {
		body.Close()
		return nil, fmt.Errorf("failed to parse start event: %w", err)
	}
	if event.Start == nil {
		body.Close()
		return nil, fmt.Errorf("first event is not a start event")
	}

	pid := event.Start.Pid
	cancelCtx, cancel := context.WithCancel(ctx)

	handle := NewCommandHandle(pid, func() {
		cancel()
		body.Close()
	}, func() (bool, error) {
		return c.Kill(ctx, pid, nil)
	}, opts.OnStdout, opts.OnStderr)

	go func() {
		defer body.Close()
		for {
			select {
			case <-cancelCtx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					// Stream ended without explicit end event
					handle.SetEnd(0, "")
					return
				}
				c.handleProcessEvent(msg, handle)
				if handle.GetExitCode() != nil {
					return
				}
			}
		}
	}()

	return handle, nil
}

func (c *Commands) handleProcessEvent(msg json.RawMessage, handle *CommandHandle) {
	var event process.ProcessEvent
	if err := json.Unmarshal(msg, &event); err != nil {
		return
	}
	if event.Data != nil {
		if len(event.Data.Stdout) > 0 {
			handle.AppendStdout(string(event.Data.Stdout))
		}
		if len(event.Data.Stderr) > 0 {
			handle.AppendStderr(string(event.Data.Stderr))
		}
	}
	if event.End != nil {
		handle.SetEnd(int(event.End.ExitCode), event.End.Error)
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
