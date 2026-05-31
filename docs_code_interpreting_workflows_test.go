package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsCodeInterpretingAnalyzeDataWithAIDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/analyze-data-with-ai.mdx"); err != nil {
		t.Fatalf("code-interpreting analyze-data-with-ai doc is missing: %v", err)
	}
}

// This test keeps docs/code-interpreting/analyze-data-with-ai.mdx aligned with
// the exported Go SDK template, sandbox, commands, and filesystem surface. The
// closures are compile-only examples and are intentionally never executed.
func TestDocsCodeInterpretingAnalyzeDataWithAIExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "build-python-analysis-template",
			fn: func() {
				template := e2b.Template(nil).
					FromPythonImage("3.12").
					RunCmd("pip install pandas matplotlib seaborn", &struct{ User string }{User: "root"})

				_ = template
			},
		},
		{
			name: "upload-dataset-run-script-read-artifacts",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "python-data", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, writeDatasetErr := sandbox.Files.Write(ctx, "/home/user/dataset.csv", "year,value\n2022,3.1\n2023,4.2\n", nil)

				script := `import json
import pandas as pd
import matplotlib.pyplot as plt

df = pd.read_csv("/home/user/dataset.csv")

summary = {
    "rows": int(len(df)),
    "average": float(df["value"].mean()),
}

with open("/home/user/result.json", "w", encoding="utf-8") as f:
    json.dump(summary, f)

plt.plot(df["year"], df["value"])
plt.savefig("/home/user/chart.png")
`

				_, writeScriptErr := sandbox.Files.Write(ctx, "/home/user/analyze.py", script, nil)
				execution, runErr := sandbox.Commands.Run(ctx, "python3 /home/user/analyze.py", nil)
				resultJSON, readJSONErr := sandbox.Files.Read(ctx, "/home/user/result.json", nil)
				chartBytes, readChartErr := sandbox.Files.Read(ctx, "/home/user/chart.png", &e2b.FilesystemReadOpts{
					Format: e2b.ReadFormatBytes,
				})

				result := execution.(*e2b.CommandResult)
				_ = result.Stdout
				_ = resultJSON.(string)
				_ = len(chartBytes.([]byte))
				_ = writeDatasetErr
				_ = writeScriptErr
				_ = runErr
				_ = readJSONErr
				_ = readChartErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 analyze-data-with-ai doc snippets, got %d", got)
	}
}

func TestDocsCodeInterpretingAnalyzeDataWithAIPreInstalledLibrariesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/analyze-data-with-ai/pre-installed-libraries.mdx"); err != nil {
		t.Fatalf("code-interpreting pre-installed-libraries doc is missing: %v", err)
	}
}

// This test keeps docs/code-interpreting/analyze-data-with-ai/pre-installed-libraries.mdx
// aligned with the exported Go SDK template surface. The closures are
// compile-only examples and are intentionally never executed.
func TestDocsCodeInterpretingAnalyzeDataWithAIPreInstalledLibrariesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "build-template-with-libraries",
			fn: func() {
				template := e2b.Template(nil).
					FromPythonImage("3.12").
					RunCmd("pip install pandas polars matplotlib seaborn scikit-learn", &struct{ User string }{User: "root"})

				_ = template
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 pre-installed-libraries doc snippet, got %d", got)
	}
}

func TestDocsCodeInterpretingContextsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/contexts.mdx"); err != nil {
		t.Fatalf("code-interpreting contexts doc is missing: %v", err)
	}
}

// This test keeps docs/code-interpreting/contexts.mdx aligned with the
// exported Go SDK sandbox, commands, filesystem, and PTY surface. The
// closures are compile-only examples and are intentionally never executed.
func TestDocsCodeInterpretingContextsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "separate-sandboxes",
			fn: func() {
				ctx := context.Background()

				analysisSandbox, err := e2b.Create(ctx, "python-runtime", nil)
				if err != nil {
					return
				}
				defer analysisSandbox.Kill(context.Background(), nil)

				reportSandbox, reportErr := e2b.Create(ctx, "python-runtime", nil)
				if reportErr != nil {
					return
				}
				defer reportSandbox.Kill(context.Background(), nil)

				analysisExec, analysisRunErr := analysisSandbox.Commands.Run(ctx, `python3 -c "print('analysis context')"`, nil)
				reportExec, reportRunErr := reportSandbox.Commands.Run(ctx, `python3 -c "print('report context')"`, nil)

				analysisResult := analysisExec.(*e2b.CommandResult)
				reportResult := reportExec.(*e2b.CommandResult)

				_ = analysisResult.Stdout
				_ = reportResult.Stdout
				_ = analysisRunErr
				_ = reportRunErr
			},
		},
		{
			name: "working-directories-or-envs",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "python-runtime", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, mkdirAErr := sandbox.Files.MakeDir(ctx, "/home/user/context-a", nil)
				_, mkdirBErr := sandbox.Files.MakeDir(ctx, "/home/user/context-b", nil)

				contextA, runAErr := sandbox.Commands.Run(ctx, "pwd && printenv CONTEXT_NAME", &e2b.CommandStartOpts{
					Cwd:  "/home/user/context-a",
					Envs: map[string]string{"CONTEXT_NAME": "analysis"},
				})
				contextB, runBErr := sandbox.Commands.Run(ctx, "pwd && printenv CONTEXT_NAME", &e2b.CommandStartOpts{
					Cwd:  "/home/user/context-b",
					Envs: map[string]string{"CONTEXT_NAME": "report"},
				})

				resultA := contextA.(*e2b.CommandResult)
				resultB := contextB.(*e2b.CommandResult)

				_ = resultA.Stdout
				_ = resultB.Stdout
				_ = mkdirAErr
				_ = mkdirBErr
				_ = runAErr
				_ = runBErr
			},
		},
		{
			name: "pty-sessions",
			fn: func() {
				ctx := context.Background()
				ptyTimeoutMs := 0

				sandbox, err := e2b.Create(ctx, "base", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				first, firstErr := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
					Cols:      100,
					Rows:      30,
					TimeoutMs: &ptyTimeoutMs,
				})
				second, secondErr := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
					Cols:      100,
					Rows:      30,
					TimeoutMs: &ptyTimeoutMs,
				})

				if first != nil {
					firstKilled, firstKillErr := first.Kill()
					_ = first.Pid
					_ = firstKilled
					_ = firstKillErr
				}
				if second != nil {
					secondKilled, secondKillErr := second.Kill()
					_ = second.Pid
					_ = secondKilled
					_ = secondKillErr
				}

				_ = firstErr
				_ = secondErr
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 contexts doc snippets, got %d", got)
	}
}

func TestDocsCodeInterpretingCreateChartsVisualizationsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/create-charts-visualizations.mdx"); err != nil {
		t.Fatalf("code-interpreting create-charts-visualizations doc is missing: %v", err)
	}
}

// This overview is intentionally prose-only because chart generation in the Go
// SDK is expressed through the concrete file and command APIs on the linked
// pages rather than a separate typed chart abstraction.
func TestDocsCodeInterpretingCreateChartsVisualizationsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 create-charts-visualizations doc snippets, got %d", got)
	}
}

func TestDocsCodeInterpretingStreamingDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/streaming.mdx"); err != nil {
		t.Fatalf("code-interpreting streaming doc is missing: %v", err)
	}
}

// This test keeps docs/code-interpreting/streaming.mdx aligned with the
// exported Go SDK command streaming surface. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsCodeInterpretingStreamingExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "stream-stdout-and-stderr",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "python-runtime", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, runErr := sandbox.Commands.Run(ctx, `python3 - <<'PY'
import sys
import time

print("This goes first to stdout")
time.sleep(1)
print("This goes later to stderr", file=sys.stderr)
time.sleep(1)
print("This goes last")
PY`, &e2b.CommandStartOpts{
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
					OnStderr: func(data e2b.Stderr) {
						_ = data
					},
				})

				_ = runErr
			},
		},
		{
			name: "background-state-and-wait",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "python-runtime", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, `python3 - <<'PY'
import time
for i in range(3):
    print(f"step {i}")
    time.sleep(1)
PY`, &e2b.CommandStartOpts{
					Background: true,
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
				})

				handle := execution.(*e2b.CommandHandle)
				state := handle.State()
				result, waitErr := handle.Wait()

				_ = state.Stdout
				_ = result.Stdout
				_ = runErr
				_ = waitErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 streaming doc snippets, got %d", got)
	}
}
