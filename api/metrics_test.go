package api

import (
	"encoding/json"
	"testing"
)

func TestSandboxMetricsListAcceptsSandboxInfoObjectAsEmpty(t *testing.T) {
	var metrics SandboxMetricsList
	err := json.Unmarshal([]byte(`{"templateID":"code-interpreter","sandboxID":"default--code-interpreter"}`), &metrics)
	if err != nil {
		t.Fatalf("failed to unmarshal sandbox info fallback: %v", err)
	}
	if len(metrics) != 0 {
		t.Fatalf("expected empty metrics fallback, got %#v", metrics)
	}
}

func TestSandboxMetricsListAcceptsCurrentArrayShape(t *testing.T) {
	var metrics SandboxMetricsList
	err := json.Unmarshal([]byte(`[{"timestampUnix":1779780000,"cpuUsedPct":1.5,"cpuCount":1,"memUsed":10,"memTotal":20,"diskUsed":30,"diskTotal":40}]`), &metrics)
	if err != nil {
		t.Fatalf("failed to unmarshal metrics: %v", err)
	}
	if len(metrics) != 1 || metrics[0].TimestampUnix != 1779780000 {
		t.Fatalf("unexpected metrics: %#v", metrics)
	}
}
