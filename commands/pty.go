package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/superduck-ai/e2b-go-sdk/envd"
	"github.com/superduck-ai/e2b-go-sdk/envd/process"
	"github.com/superduck-ai/e2b-go-sdk/internal/shared"
)

type PtyCreateOpts struct {
	Cols             uint32
	Rows             uint32
	OnData           func(data PtyOutput)
	TimeoutMs        *int
	User             string
	Envs             map[string]string
	Cwd              string
	Signal           context.Context
	RequestTimeoutMs *int
}

type PtySize struct {
	Cols uint32
	Rows uint32
}

type PtyConnectOpts struct {
	OnData           func(data PtyOutput)
	TimeoutMs        *int
	Signal           context.Context
	RequestTimeoutMs *int
}

type Pty struct {
	connectionConfig *connectionConfig
	envdVersion      string
	httpClient       *http.Client
}

func NewPty(cfg any, envdVersion string) *Pty {
	resolved := newConnectionConfig(cfg)
	var logger shared.Logger
	var proxy string
	if resolved != nil {
		logger = resolved.Logger
		proxy = resolved.Proxy
	}
	return &Pty{
		connectionConfig: resolved,
		envdVersion:      envdVersion,
		httpClient:       shared.NewEnvdRPCHTTPClient(0, proxy, logger),
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
	req, err := newRetryableRequest(ctx, http.MethodPost, url, data)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.headers(user) {
		req.Header.Set(k, v)
	}
	var resp *http.Response
	err = envd.RetryRPCTransportErrorWithBeforeAttempt(req.Context(), func() error {
		if req.GetBody == nil {
			return nil
		}
		body, err := req.GetBody()
		if err != nil {
			return err
		}
		req.Body = body
		return nil
	}, func() error {
		var innerErr error
		resp, innerErr = p.httpClient.Do(req)
		return innerErr
	})
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
			return wrapProcessError(envd.HandleRpcError(connectErr.Code, connectErr.Message))
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
	req, err := newRetryableRequest(ctx, http.MethodPost, url, envd.EncodeConnectEnvelope(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/connect+json")
	req.Header.Set(keepalivePingHeader, fmt.Sprintf("%d", keepalivePingIntervalSec))
	for k, v := range p.headers(user) {
		req.Header.Set(k, v)
	}
	var resp *http.Response
	err = envd.RetryRPCTransportErrorWithBeforeAttempt(req.Context(), func() error {
		if req.GetBody == nil {
			return nil
		}
		body, err := req.GetBody()
		if err != nil {
			return err
		}
		req.Body = body
		return nil
	}, func() error {
		var innerErr error
		resp, innerErr = p.httpClient.Do(req)
		return innerErr
	})
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
			return nil, wrapProcessError(envd.HandleRpcError(connectErr.Code, connectErr.Message))
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
	envs["LANG"] = "C.UTF-8"
	envs["LC_ALL"] = "C.UTF-8"
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
			Size: &process.PTYSize{
				Cols: cols,
				Rows: rows,
			},
		},
	}

	requestCtx, clearRequestTimeout, cancelRequestTimeout := requestTimeoutStreamContextWithSignal(ctx, opts.Signal, p.requestTimeoutFromCreateOpts(opts))
	streamCtx, streamCancel := streamContext(requestCtx, opts.TimeoutMs, defaultProcessConnectionTimeoutMs)
	body, err := p.connectServerStream(streamCtx, "/process.Process/Start", startReq, user)
	if err != nil {
		streamCancel()
		cancelRequestTimeout()
		return nil, err
	}

	ch := make(chan streamEnvelope, 16)
	go readStreamEnvelopesWithLogger(body, ch, p.connectionConfig.Logger)

	firstMsg, ok, err := waitForFirstEvent(ch, p.requestTimeoutFromCreateOpts(opts))
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
		return p.Kill(ctx, pid, nil)
	}, nil, nil)
	onData := opts.OnData

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
						handle.setWaitError(wrapProcessError(envd.HandleStreamContextError(err)))
						return
					}
					handle.setWaitError(errProcessExitedWithoutResult)
					return
				}
				if msg.err != nil {
					handle.setWaitError(wrapProcessError(msg.err))
					return
				}
				var ev process.ProcessEvent
				if json.Unmarshal(msg.payload, &ev) != nil {
					continue
				}
				if ev.Data != nil && len(ev.Data.Pty) > 0 {
					appendPtyOutput(handle, ev.Data.Pty, onData)
				}
				if ev.End != nil {
					handle.setEnd(int(ev.End.ExitCode), ev.End.Error)
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

	req := &process.ConnectRequest{Process: process.PidSelector(pid)}
	requestCtx, clearRequestTimeout, cancelRequestTimeout := requestTimeoutStreamContextWithSignal(ctx, opts.Signal, p.requestTimeoutFromConnectOpts(opts))
	streamCtx, streamCancel := streamContext(requestCtx, opts.TimeoutMs, defaultProcessConnectionTimeoutMs)
	body, err := p.connectServerStream(streamCtx, "/process.Process/Connect", req, "")
	if err != nil {
		streamCancel()
		cancelRequestTimeout()
		return nil, err
	}

	ch := make(chan streamEnvelope, 16)
	go readStreamEnvelopesWithLogger(body, ch, p.connectionConfig.Logger)

	firstMsg, ok, err := waitForFirstEvent(ch, p.requestTimeoutFromConnectOpts(opts))
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
		return p.Kill(ctx, pid, nil)
	}, nil, nil)
	onData := opts.OnData

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
						handle.setWaitError(wrapProcessError(envd.HandleStreamContextError(err)))
						return
					}
					handle.setWaitError(errProcessExitedWithoutResult)
					return
				}
				if msg.err != nil {
					handle.setWaitError(wrapProcessError(msg.err))
					return
				}
				var ev process.ProcessEvent
				if json.Unmarshal(msg.payload, &ev) != nil {
					continue
				}
				if ev.Data != nil && len(ev.Data.Pty) > 0 {
					appendPtyOutput(handle, ev.Data.Pty, onData)
				}
				if ev.End != nil {
					handle.setEnd(int(ev.End.ExitCode), ev.End.Error)
					return
				}
			}
		}
	}()

	return handle, nil
}

func appendPtyOutput(handle *CommandHandle, data []byte, onData func(PtyOutput)) {
	handle.appendStdout(bytesToValidString(data))
	if onData == nil {
		return
	}
	raw := make([]byte, len(data))
	copy(raw, data)
	onData(PtyOutput(raw))
}

func (p *Pty) SendInput(ctx context.Context, pid uint32, data []byte, opts *CommandRequestOpts) error {
	var signal context.Context
	if opts != nil {
		signal = opts.Signal
	}
	reqCtx, cancel := requestContextWithSignal(ctx, signal, p.requestTimeout(opts))
	defer cancel()

	req := &process.SendInputRequest{
		Process: process.PidSelector(pid),
		Input:   &process.ProcessInput{Pty: data},
	}
	return p.connectUnary(reqCtx, "/process.Process/SendInput", req, nil, "")
}

func (p *Pty) Resize(ctx context.Context, pid uint32, size PtySize, opts *CommandRequestOpts) error {
	var signal context.Context
	if opts != nil {
		signal = opts.Signal
	}
	reqCtx, cancel := requestContextWithSignal(ctx, signal, p.requestTimeout(opts))
	defer cancel()

	req := &process.UpdateRequest{
		Process: process.PidSelector(pid),
		Pty: &process.PTY{
			Size: &process.PTYSize{
				Cols: size.Cols,
				Rows: size.Rows,
			},
		},
	}
	return p.connectUnary(reqCtx, "/process.Process/Update", req, nil, "")
}

func (p *Pty) Kill(ctx context.Context, pid uint32, opts *CommandRequestOpts) (bool, error) {
	var signal context.Context
	if opts != nil {
		signal = opts.Signal
	}
	reqCtx, cancel := requestContextWithSignal(ctx, signal, p.requestTimeout(opts))
	defer cancel()

	req := &process.SendSignalRequest{
		Process: process.PidSelector(pid),
		Signal:  process.SignalSIGKILL,
	}
	err := p.connectUnary(reqCtx, "/process.Process/SendSignal", req, nil, "")
	if err != nil {
		if isProcessNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (p *Pty) requestTimeout(opts *CommandRequestOpts) *int {
	if opts != nil && opts.RequestTimeoutMs != nil {
		return opts.RequestTimeoutMs
	}
	if p.connectionConfig.RequestTimeoutMs <= 0 {
		return nil
	}
	timeout := p.connectionConfig.RequestTimeoutMs
	return &timeout
}

func (p *Pty) requestTimeoutFromCreateOpts(opts *PtyCreateOpts) *int {
	if opts != nil && opts.RequestTimeoutMs != nil {
		return opts.RequestTimeoutMs
	}
	if p.connectionConfig.RequestTimeoutMs <= 0 {
		return nil
	}
	timeout := p.connectionConfig.RequestTimeoutMs
	return &timeout
}

func (p *Pty) requestTimeoutFromConnectOpts(opts *PtyConnectOpts) *int {
	if opts != nil && opts.RequestTimeoutMs != nil {
		return opts.RequestTimeoutMs
	}
	if p.connectionConfig.RequestTimeoutMs <= 0 {
		return nil
	}
	timeout := p.connectionConfig.RequestTimeoutMs
	return &timeout
}
