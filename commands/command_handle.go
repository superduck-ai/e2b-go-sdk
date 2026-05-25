package commands

import (
	"sync"
)

type CommandResult struct {
	ExitCode int
	Error    string
	Stdout   string
	Stderr   string
}

type CommandExitError struct {
	CommandResult
	Message string
}

func (e *CommandExitError) Error() string { return e.Message }

type CommandHandle struct {
	Pid        uint32
	mu         sync.Mutex
	stdout     string
	stderr     string
	exitCode   *int
	error      string
	done       chan struct{}
	disconnect func()
	killFn     func() (bool, error)
	onStdout   func(data string)
	onStderr   func(data string)
}

func NewCommandHandle(pid uint32, disconnect func(), killFn func() (bool, error), onStdout, onStderr func(string)) *CommandHandle {
	return &CommandHandle{
		Pid: pid, disconnect: disconnect, killFn: killFn,
		onStdout: onStdout, onStderr: onStderr, done: make(chan struct{}),
	}
}

func (h *CommandHandle) GetStdout() string  { h.mu.Lock(); defer h.mu.Unlock(); return h.stdout }
func (h *CommandHandle) GetStderr() string  { h.mu.Lock(); defer h.mu.Unlock(); return h.stderr }
func (h *CommandHandle) GetExitCode() *int  { h.mu.Lock(); defer h.mu.Unlock(); return h.exitCode }
func (h *CommandHandle) GetError() string   { h.mu.Lock(); defer h.mu.Unlock(); return h.error }

func (h *CommandHandle) Wait() (*CommandResult, error) {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	result := &CommandResult{Stdout: h.stdout, Stderr: h.stderr}
	if h.exitCode != nil {
		result.ExitCode = *h.exitCode
	}
	result.Error = h.error
	if result.ExitCode != 0 {
		return nil, &CommandExitError{CommandResult: *result, Message: "command exited with non-zero exit code"}
	}
	return result, nil
}

func (h *CommandHandle) Disconnect() { h.disconnect() }
func (h *CommandHandle) Kill() (bool, error) { return h.killFn() }

// AppendStdout is an internal method for streaming data
func (h *CommandHandle) AppendStdout(data string) {
	h.mu.Lock()
	h.stdout += data
	h.mu.Unlock()
	if h.onStdout != nil {
		h.onStdout(data)
	}
}

// AppendStderr is an internal method for streaming data
func (h *CommandHandle) AppendStderr(data string) {
	h.mu.Lock()
	h.stderr += data
	h.mu.Unlock()
	if h.onStderr != nil {
		h.onStderr(data)
	}
}

// SetEnd marks the command as finished
func (h *CommandHandle) SetEnd(exitCode int, errStr string) {
	h.mu.Lock()
	h.exitCode = &exitCode
	h.error = errStr
	h.mu.Unlock()
	close(h.done)
}
