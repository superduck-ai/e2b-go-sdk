package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/e2b-dev/e2b-go-sdk/envd"
	"github.com/e2b-dev/e2b-go-sdk/envd/process"
)

type PtyCreateOpts struct {
	Cols             uint32
	Rows             uint32
	OnData           func(data []byte)
	TimeoutMs        *int
	User             string
	Envs             map[string]string
	Cwd              string
	RequestTimeoutMs *int
}

type PtyConnectOpts struct {
	OnData           func(data []byte)
	TimeoutMs        *int
	RequestTimeoutMs *int
}

type Pty struct {
	connectionConfig *ConnectionConfig
	envdVersion      string
	httpClient       *http.Client
}

func NewPty(connectionConfig *ConnectionConfig, envdVersion string) *Pty {
	return &Pty{
		connectionConfig: connectionConfig,
		envdVersion:      envdVersion,
		httpClient:       &http.Client{},
	}
}

func (p *Pty) baseUrl() string {
	return p.connectionConfig.SandboxUrl
}

func (p *Pty) headers(user string) map[string]string {
	h := make(map[string]string)
	for k, v := range p.connectionConfig.Headers {
		h[k] = v
	}
	if user == "" {
		user = "user"
	}
	for k, v := range envd.AuthenticationHeader(p.envdVersion, user) {
		h[k] = v
	}
	return h
}

func (p *Pty) connectUnary(ctx context.Context, path string, reqBody interface{}, respBody interface{}, user string) error {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	url := p.baseUrl() + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.headers(user) {
		req.Header.Set(k, v)
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
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

func (p *Pty) connectServerStream(ctx context.Context, path string, reqBody interface{}, user string) (io.ReadCloser, error) {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	url := p.baseUrl() + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/connect+json")
	for k, v := range p.headers(user) {
		req.Header.Set(k, v)
	}
	resp, err := p.httpClient.Do(req)
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

func (p *Pty) Create(ctx context.Context, opts *PtyCreateOpts) (*CommandHandle, error) {
	if opts == nil {
		opts = &PtyCreateOpts{}
	}
	cols := opts.Cols
	if cols == 0 {
		cols = 80
	}
	rows := opts.Rows
	if rows == 0 {
		rows = 24
	}

	user := opts.User
	envs := make(map[string]string)
	envs["TERM"] = "xterm-256color"
	envs["COLORTERM"] = "truecolor"
	for k, v := range opts.Envs {
		envs[k] = v
	}

	startReq := &process.StartRequest{
		Process: &process.ProcessConfig{
			Cmd:  "/bin/bash",
			Args: []string{"-i", "-l"},
			Envs: envs,
			Cwd:  opts.Cwd,
		},
		Pty: &process.PTY{
			Cols: cols,
			Rows: rows,
		},
	}

	body, err := p.connectServerStream(ctx, "/process.Process/Start", startReq, user)
	if err != nil {
		return nil, err
	}

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

	// For PTY, we route data events through onData
	var onStdout func(string)
	if opts.OnData != nil {
		onData := opts.OnData
		onStdout = func(data string) {
			onData([]byte(data))
		}
	}

	handle := NewCommandHandle(pid, func() {
		cancel()
		body.Close()
	}, func() (bool, error) {
		return p.Kill(ctx, pid, nil)
	}, onStdout, nil)

	go func() {
		defer body.Close()
		for {
			select {
			case <-cancelCtx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					handle.SetEnd(0, "")
					return
				}
				var ev process.ProcessEvent
				if json.Unmarshal(msg, &ev) != nil {
					continue
				}
				if ev.Data != nil && len(ev.Data.Pty) > 0 {
					handle.AppendStdout(string(ev.Data.Pty))
				}
				if ev.End != nil {
					handle.SetEnd(int(ev.End.ExitCode), ev.End.Error)
					return
				}
			}
		}
	}()

	return handle, nil
}

func (p *Pty) Connect(ctx context.Context, pid uint32, opts *PtyConnectOpts) (*CommandHandle, error) {
	if opts == nil {
		opts = &PtyConnectOpts{}
	}

	req := &process.ConnectRequest{Pid: pid}
	body, err := p.connectServerStream(ctx, "/process.Process/Connect", req, "")
	if err != nil {
		return nil, err
	}

	cancelCtx, cancel := context.WithCancel(ctx)

	var onStdout func(string)
	if opts.OnData != nil {
		onData := opts.OnData
		onStdout = func(data string) {
			onData([]byte(data))
		}
	}

	handle := NewCommandHandle(pid, func() {
		cancel()
		body.Close()
	}, func() (bool, error) {
		return p.Kill(ctx, pid, nil)
	}, onStdout, nil)

	go func() {
		ch := make(chan json.RawMessage, 16)
		go readStreamEnvelopes(body, ch)
		for {
			select {
			case <-cancelCtx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					handle.SetEnd(0, "")
					return
				}
				var ev process.ProcessEvent
				if json.Unmarshal(msg, &ev) != nil {
					continue
				}
				if ev.Data != nil && len(ev.Data.Pty) > 0 {
					handle.AppendStdout(string(ev.Data.Pty))
				}
				if ev.End != nil {
					handle.SetEnd(int(ev.End.ExitCode), ev.End.Error)
					return
				}
			}
		}
	}()

	return handle, nil
}

func (p *Pty) SendInput(ctx context.Context, pid uint32, data []byte, opts *CommandRequestOpts) error {
	req := &process.SendInputRequest{
		Pid: pid,
		Pty: data,
	}
	return p.connectUnary(ctx, "/process.Process/SendInput", req, nil, "")
}

func (p *Pty) Resize(ctx context.Context, pid uint32, cols, rows uint32, opts *CommandRequestOpts) error {
	req := &process.UpdateRequest{
		Pid: pid,
		Size: &process.PTY{
			Cols: cols,
			Rows: rows,
		},
	}
	return p.connectUnary(ctx, "/process.Process/Update", req, nil, "")
}

func (p *Pty) Kill(ctx context.Context, pid uint32, opts *CommandRequestOpts) (bool, error) {
	req := &process.SendSignalRequest{
		Pid:    pid,
		Signal: process.SignalSIGKILL,
	}
	err := p.connectUnary(ctx, "/process.Process/SendSignal", req, nil, "")
	if err != nil {
		if rpcErr, ok := err.(*envd.RpcError); ok {
			if rpcErr.Code == "not_found" || rpcErr.Message == "process not found" {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}
