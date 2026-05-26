package process

import (
	"encoding/json"
	"testing"
)

func TestProcessEventUnmarshalAcceptsStartResponseWrapper(t *testing.T) {
	var event ProcessEvent
	if err := json.Unmarshal([]byte(`{"event":{"start":{"pid":662}}}`), &event); err != nil {
		t.Fatalf("failed to unmarshal wrapped start event: %v", err)
	}
	if event.Start == nil || event.Start.Pid != 662 {
		t.Fatalf("unexpected start event: %#v", event.Start)
	}
}

func TestProcessEventUnmarshalAcceptsWrappedDataAndEndEvents(t *testing.T) {
	var dataEvent ProcessEvent
	if err := json.Unmarshal([]byte(`{"event":{"data":{"stdout":"SGVsbG8gZnJvbSBFMkIhCg=="}}}`), &dataEvent); err != nil {
		t.Fatalf("failed to unmarshal wrapped data event: %v", err)
	}
	if dataEvent.Data == nil || string(dataEvent.Data.Stdout) != "Hello from E2B!\n" {
		t.Fatalf("unexpected data event: %#v", dataEvent.Data)
	}

	var endEvent ProcessEvent
	if err := json.Unmarshal([]byte(`{"event":{"end":{"exitCode":7,"exited":true,"status":"exit status 7","error":"boom"}}}`), &endEvent); err != nil {
		t.Fatalf("failed to unmarshal wrapped end event: %v", err)
	}
	if endEvent.End == nil || endEvent.End.ExitCode != 7 || endEvent.End.Error != "boom" {
		t.Fatalf("unexpected end event: %#v", endEvent.End)
	}
}

func TestProcessEventUnmarshalKeepsLegacyDirectShape(t *testing.T) {
	var event ProcessEvent
	if err := json.Unmarshal([]byte(`{"start":{"pid":123}}`), &event); err != nil {
		t.Fatalf("failed to unmarshal direct start event: %v", err)
	}
	if event.Start == nil || event.Start.Pid != 123 {
		t.Fatalf("unexpected start event: %#v", event.Start)
	}
}
