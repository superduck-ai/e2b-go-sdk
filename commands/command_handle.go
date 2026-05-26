package commands

import (
	"errors"
	"sync"
)

type Stdout = string
type Stderr = string
type PtyOutput = []byte

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
	waitErr    error
	done       chan struct{}
	doneOnce   sync.Once
	disconnect func()
	killFn     func() (bool, error)
	onStdout   func(data Stdout)
	onStderr   func(data Stderr)
}

var errProcessExitedWithoutResult = errors.New("Process exited without a result")

func newCommandHandle(pid uint32, disconnect func(), killFn func() (bool, error), onStdout func(Stdout), onStderr func(Stderr)) *CommandHandle {
	return &CommandHandle{
		Pid: pid, disconnect: disconnect, killFn: killFn,
		onStdout: onStdout, onStderr: onStderr, done: make(chan struct{}),
	}
}

func (h *CommandHandle) GetStdout() string { h.mu.Lock(); defer h.mu.Unlock(); return h.stdout }
func (h *CommandHandle) GetStderr() string { h.mu.Lock(); defer h.mu.Unlock(); return h.stderr }
func (h *CommandHandle) GetExitCode() *int { h.mu.Lock(); defer h.mu.Unlock(); return h.exitCode }
func (h *CommandHandle) GetError() string  { h.mu.Lock(); defer h.mu.Unlock(); return h.error }

func (h *CommandHandle) Wait() (*CommandResult, error) {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.waitErr != nil {
		return nil, h.waitErr
	}
	result := &CommandResult{Stdout: h.stdout, Stderr: h.stderr}
	if h.exitCode != nil {
		result.ExitCode = *h.exitCode
	}
	result.Error = h.error
	if result.ExitCode != 0 {
		return nil, &CommandExitError{CommandResult: *result, Message: result.Error}
	}
	return result, nil
}

func (h *CommandHandle) Disconnect()         { h.disconnect() }
func (h *CommandHandle) Kill() (bool, error) { return h.killFn() }

// AppendStdout is an internal method for streaming data
func (h *CommandHandle) appendStdout(data Stdout) {
	h.mu.Lock()
	h.stdout += data
	h.mu.Unlock()
	if h.onStdout != nil {
		h.onStdout(data)
	}
}

// AppendStderr is an internal method for streaming data
func (h *CommandHandle) appendStderr(data Stderr) {
	h.mu.Lock()
	h.stderr += data
	h.mu.Unlock()
	if h.onStderr != nil {
		h.onStderr(data)
	}
}

// SetEnd marks the command as finished
func (h *CommandHandle) setEnd(exitCode int, errStr string) {
	h.mu.Lock()
	h.exitCode = &exitCode
	h.error = errStr
	h.mu.Unlock()
	h.doneOnce.Do(func() {
		close(h.done)
	})
}

func (h *CommandHandle) setWaitError(err error) {
	h.mu.Lock()
	h.waitErr = err
	h.mu.Unlock()
	h.doneOnce.Do(func() {
		close(h.done)
	})
}
