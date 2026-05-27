package process

import "encoding/json"

// Signal enum
type Signal int32

const (
	SignalUnspecified Signal = 0
	SignalSIGTERM     Signal = 15
	SignalSIGKILL     Signal = 9
)

// PTY size
type PTY struct {
	Size *PTYSize `json:"size,omitempty"`
}

type PTYSize struct {
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

type ProcessSelector struct {
	Pid uint32 `json:"pid,omitempty"`
	Tag string `json:"tag,omitempty"`
}

func PidSelector(pid uint32) *ProcessSelector {
	return &ProcessSelector{Pid: pid}
}

// ProcessEvent from a running process stream
type ProcessEvent struct {
	Start     *ProcessStartEvent `json:"start,omitempty"`
	Data      *ProcessDataEvent  `json:"data,omitempty"`
	End       *ProcessEndEvent   `json:"end,omitempty"`
	Keepalive bool               `json:"keepalive,omitempty"`
}

func (e *ProcessEvent) UnmarshalJSON(data []byte) error {
	type eventJSON struct {
		Start     *ProcessStartEvent `json:"start,omitempty"`
		Data      *ProcessDataEvent  `json:"data,omitempty"`
		End       *ProcessEndEvent   `json:"end,omitempty"`
		Keepalive json.RawMessage    `json:"keepalive,omitempty"`
	}

	var wrapped struct {
		Event *eventJSON `json:"event"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return err
	}
	if wrapped.Event != nil {
		applyProcessEventJSON(e, *wrapped.Event)
		return nil
	}

	var direct eventJSON
	if err := json.Unmarshal(data, &direct); err != nil {
		return err
	}
	applyProcessEventJSON(e, direct)
	return nil
}

func applyProcessEventJSON(event *ProcessEvent, data struct {
	Start     *ProcessStartEvent `json:"start,omitempty"`
	Data      *ProcessDataEvent  `json:"data,omitempty"`
	End       *ProcessEndEvent   `json:"end,omitempty"`
	Keepalive json.RawMessage    `json:"keepalive,omitempty"`
}) {
	event.Start = data.Start
	event.Data = data.Data
	event.End = data.End
	event.Keepalive = parseKeepalive(data.Keepalive)
}

func parseKeepalive(data json.RawMessage) bool {
	if len(data) == 0 {
		return false
	}

	var value bool
	if err := json.Unmarshal(data, &value); err == nil {
		return value
	}

	var object map[string]any
	return json.Unmarshal(data, &object) == nil
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
	Process *ProcessSelector `json:"process,omitempty"`
}

type ProcessInput struct {
	Stdin []byte `json:"stdin,omitempty"`
	Pty   []byte `json:"pty,omitempty"`
}

func (i ProcessInput) MarshalJSON() ([]byte, error) {
	data := map[string][]byte{}
	if i.Stdin != nil {
		data["stdin"] = i.Stdin
	}
	if i.Pty != nil {
		data["pty"] = i.Pty
	}
	return json.Marshal(data)
}

type ListRequest struct{}

type ListResponse struct {
	Processes []*ProcessInfo `json:"processes"`
}

type SendInputRequest struct {
	Process *ProcessSelector `json:"process,omitempty"`
	Input   *ProcessInput    `json:"input,omitempty"`
}

type SendSignalRequest struct {
	Process *ProcessSelector `json:"process,omitempty"`
	Signal  Signal           `json:"signal"`
}

type UpdateRequest struct {
	Process *ProcessSelector `json:"process,omitempty"`
	Pty     *PTY             `json:"pty,omitempty"`
}

type CloseStdinRequest struct {
	Process *ProcessSelector `json:"process,omitempty"`
}
