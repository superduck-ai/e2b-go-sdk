package e2b_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsCodeInterpretingStaticChartsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/create-charts-visualizations/static-charts.mdx"); err != nil {
		t.Fatalf("code-interpreting static-charts doc is missing: %v", err)
	}
}

// This test keeps docs/code-interpreting/create-charts-visualizations/static-charts.mdx
// aligned with the exported Go SDK command and filesystem surface. The
// closures are compile-only examples and are intentionally never executed.
func TestDocsCodeInterpretingStaticChartsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "generate-png-and-read-bytes",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "python-data", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, `python3 - <<'PY'
import matplotlib.pyplot as plt

plt.plot([1, 2, 3, 4])
plt.ylabel("some numbers")
plt.savefig("/home/user/chart.png")
PY`, nil)
				value, readErr := sandbox.Files.Read(ctx, "/home/user/chart.png", &e2b.FilesystemReadOpts{
					Format: e2b.ReadFormatBytes,
				})
				writeErr := os.WriteFile("chart.png", value.([]byte), 0o644)

				result := execution.(*e2b.CommandResult)
				_ = result.Stdout
				_ = runErr
				_ = readErr
				_ = writeErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 static-charts doc snippet, got %d", got)
	}
}

func TestDocsCodeInterpretingInteractiveChartsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/create-charts-visualizations/interactive-charts.mdx"); err != nil {
		t.Fatalf("code-interpreting interactive-charts doc is missing: %v", err)
	}
}

// This test keeps docs/code-interpreting/create-charts-visualizations/interactive-charts.mdx
// aligned with the exported Go SDK filesystem surface. The closures are
// compile-only examples and are intentionally never executed.
func TestDocsCodeInterpretingInteractiveChartsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "persist-json-and-unmarshal",
			fn: func() {
				type ChartElement struct {
					Label string  `json:"label"`
					Value float64 `json:"value"`
					Group string  `json:"group"`
				}

				type ChartPayload struct {
					Type     string         `json:"type"`
					Title    string         `json:"title"`
					XLabel   string         `json:"x_label"`
					YLabel   string         `json:"y_label"`
					Elements []ChartElement `json:"elements"`
				}

				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "python-data", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, writeErr := sandbox.Files.Write(ctx, "/home/user/chart.json", `{
  "type": "bar",
  "title": "Book Sales by Authors",
  "x_label": "Authors",
  "y_label": "Books Sold",
  "elements": [
    {"label": "Author A", "value": 100, "group": "Books Sold"},
    {"label": "Author B", "value": 200, "group": "Books Sold"}
  ]
}`, nil)
				value, readErr := sandbox.Files.Read(ctx, "/home/user/chart.json", nil)

				var payload ChartPayload
				unmarshalErr := json.Unmarshal([]byte(value.(string)), &payload)

				_ = payload.Type
				_ = payload.Title
				_ = len(payload.Elements)
				_ = writeErr
				_ = readErr
				_ = unmarshalErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 interactive-charts doc snippet, got %d", got)
	}
}
