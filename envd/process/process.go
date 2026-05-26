package process

// Signal enum
type Signal int32

const (
	SignalUnspecified Signal = 0
	SignalSIGTERM     Signal = 15
	SignalSIGKILL     Signal = 9
)

// PTY size
type PTY struct {
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

// ProcessConfig for starting a process
type ProcessConfig struct {
	Cmd  string            `json:"cmd"`
	Args []string          `json:"args,omitempty"`
	Envs map[string]string `json:"envs,omitempty"`
	Cwd  string            `json:"cwd,omitempty"`
}

// ProcessInfo describes a running process
type ProcessInfo struct {
	Config *ProcessConfig `json:"config"`
	Pid    uint32         `json:"pid"`
	Tag    string         `json:"tag,omitempty"`
}

// ProcessEvent from a running process stream
type ProcessEvent struct {
	Start     *ProcessStartEvent `json:"start,omitempty"`
	Data      *ProcessDataEvent  `json:"data,omitempty"`
	End       *ProcessEndEvent   `json:"end,omitempty"`
	Keepalive bool               `json:"keepalive,omitempty"`
}

type ProcessStartEvent struct {
	Pid uint32 `json:"pid"`
}

type ProcessDataEvent struct {
	Stdout []byte `json:"stdout,omitempty"`
	Stderr []byte `json:"stderr,omitempty"`
	Pty    []byte `json:"pty,omitempty"`
}

type ProcessEndEvent struct {
	ExitCode int32  `json:"exitCode"`
	Exited   bool   `json:"exited"`
	Status   string `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Request types

type StartRequest struct {
	Process *ProcessConfig    `json:"process"`
	Pty     *PTY              `json:"pty,omitempty"`
	Stdin   bool              `json:"stdin,omitempty"`
	Tag     string            `json:"tag,omitempty"`
	Envs    map[string]string `json:"envs,omitempty"`
}

type ConnectRequest struct {
	Pid uint32 `json:"pid"`
}

type ListRequest struct{}

type ListResponse struct {
	Processes []*ProcessInfo `json:"processes"`
}

type SendInputRequest struct {
	Pid   uint32 `json:"pid"`
	Stdin []byte `json:"stdin,omitempty"`
	Pty   []byte `json:"pty,omitempty"`
}

type SendSignalRequest struct {
	Pid    uint32 `json:"pid"`
	Signal Signal `json:"signal"`
}

type UpdateRequest struct {
	Pid  uint32 `json:"pid"`
	Size *PTY   `json:"size,omitempty"`
}

type CloseStdinRequest struct {
	Pid uint32 `json:"pid"`
}
