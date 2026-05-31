package doctest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsQuickstartConnectLLMsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/quickstart/connect-llms.mdx"); err != nil {
		t.Fatalf("quickstart connect-llms doc is missing: %v", err)
	}
}

// This test keeps docs/quickstart/connect-llms.mdx aligned with the exported
// Go SDK surface used to expose sandbox operations as LLM tools. The closures
// are compile-only examples and are intentionally never executed.
func TestDocsQuickstartConnectLLMsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "execute-command-tool",
			fn: func() {
				type ToolCall struct {
					Name      string
					Arguments json.RawMessage
				}

				type ExecuteCommandArgs struct {
					Command string `json:"command"`
					Cwd     string `json:"cwd,omitempty"`
				}

				dispatchTool := func(ctx context.Context, sandbox *e2b.Sandbox, call ToolCall) (string, error) {
					switch call.Name {
					case "execute_command":
						var args ExecuteCommandArgs
						if err := json.Unmarshal(call.Arguments, &args); err != nil {
							return "", err
						}

						execution, err := sandbox.Commands.Run(ctx, args.Command, &e2b.CommandStartOpts{
							Cwd: args.Cwd,
						})
						if err != nil {
							return "", err
						}
						result := execution.(*e2b.CommandResult)
						return result.Stdout, nil
					default:
						return "", fmt.Errorf("unknown tool %q", call.Name)
					}
				}

				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "base", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				output, toolErr := dispatchTool(ctx, sandbox, ToolCall{
					Name:      "execute_command",
					Arguments: json.RawMessage(`{"command":"uname -a","cwd":"/home/user"}`),
				})

				_ = output
				_ = toolErr
			},
		},
		{
			name: "file-tools",
			fn: func() {
				type ToolCall struct {
					Name      string
					Arguments json.RawMessage
				}

				type WriteFileArgs struct {
					Path    string `json:"path"`
					Content string `json:"content"`
				}

				type ReadFileArgs struct {
					Path string `json:"path"`
				}

				dispatchFileTool := func(ctx context.Context, sandbox *e2b.Sandbox, call ToolCall) (string, error) {
					switch call.Name {
					case "write_file":
						var args WriteFileArgs
						if err := json.Unmarshal(call.Arguments, &args); err != nil {
							return "", err
						}

						if _, err := sandbox.Files.Write(ctx, args.Path, args.Content, nil); err != nil {
							return "", err
						}
						return "ok", nil
					case "read_file":
						var args ReadFileArgs
						if err := json.Unmarshal(call.Arguments, &args); err != nil {
							return "", err
						}

						value, err := sandbox.Files.Read(ctx, args.Path, nil)
						if err != nil {
							return "", err
						}
						return value.(string), nil
					default:
						return "", fmt.Errorf("unknown tool %q", call.Name)
					}
				}

				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "base", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				writeResult, writeErr := dispatchFileTool(ctx, sandbox, ToolCall{
					Name:      "write_file",
					Arguments: json.RawMessage(`{"path":"/home/user/task.txt","content":"Summarize the repository"}`),
				})
				content, readErr := dispatchFileTool(ctx, sandbox, ToolCall{
					Name:      "read_file",
					Arguments: json.RawMessage(`{"path":"/home/user/task.txt"}`),
				})

				_ = writeResult
				_ = content
				_ = writeErr
				_ = readErr
			},
		},
		{
			name: "reuse-sandbox-per-conversation",
			fn: func() {
				type Conversation struct {
					SandboxID string
				}

				ctx := context.Background()
				timeoutMs := 600_000

				sandbox, err := e2b.Create(ctx, "base", &e2b.SandboxOpts{
					TimeoutMs: &timeoutMs,
					Lifecycle: &e2b.SandboxLifecycle{
						OnTimeout: "pause",
					},
				})
				if err != nil {
					return
				}

				conversation := Conversation{
					SandboxID: sandbox.SandboxID,
				}

				paused, pauseErr := sandbox.Pause(ctx, nil)
				resumed, connectErr := e2b.Connect(ctx, conversation.SandboxID, nil)
				execution, runErr := resumed.Commands.Run(ctx, "pwd", nil)
				result := execution.(*e2b.CommandResult)

				_ = paused
				_ = result.Stdout
				_ = pauseErr
				_ = connectErr
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 quickstart connect-llms doc snippets, got %d", got)
	}
}
