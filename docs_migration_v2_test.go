package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsMigrationV2DocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/migration/v2.mdx"); err != nil {
		t.Fatalf("migration v2 doc is missing: %v", err)
	}
}

// This test keeps docs/migration/v2.mdx aligned with the exported Go SDK
// surface used when rewriting older JS or Python E2B examples to Go. The
// closures are compile-only examples and are intentionally never executed.
func TestDocsMigrationV2ExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "replace-run-code-with-commands",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "base", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(
					ctx,
					`python3 - <<'PY'
print("hello from sandbox")
PY`,
					nil,
				)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
		{
			name: "move-file-operations-under-files",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "base", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				writeInfo, writeErr := sandbox.Files.Write(ctx, "/home/user/app/main.go", "package main\n", nil)
				batchInfos, batchErr := sandbox.Files.WriteFiles(ctx, []e2b.WriteEntry{
					{
						Path: "/home/user/app/go.mod",
						Data: "module example.com/app\n",
					},
					{
						Path: "/home/user/app/README.md",
						Data: "hello from Go",
					},
				}, nil)
				value, readErr := sandbox.Files.Read(ctx, "/home/user/app/README.md", nil)
				text := value.(string)

				_ = writeInfo
				_ = batchInfos
				_ = text
				_ = writeErr
				_ = batchErr
				_ = readErr
			},
		},
		{
			name: "paginator-and-connect",
			fn: func() {
				ctx := context.Background()

				paginator := e2b.List(&e2b.SandboxListOpts{
					Query: &struct {
						Metadata map[string]string
						State    []e2b.SandboxState
					}{
						State: []e2b.SandboxState{
							e2b.SandboxState("running"),
							e2b.SandboxState("paused"),
						},
					},
				})

				page, listErr := paginator.NextItems()
				var sandboxID string
				if len(page) > 0 {
					sandboxID = page[0].SandboxID
				}

				connected, connectErr := e2b.Connect(ctx, sandboxID, nil)

				_ = connected
				_ = listErr
				_ = connectErr
			},
		},
		{
			name: "temporary-legacy-secure-override",
			fn: func() {
				ctx := context.Background()
				secure := false

				sandbox, err := e2b.Create(ctx, "legacy-template", &e2b.SandboxOpts{
					Secure: &secure,
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_ = sandbox.SandboxID
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 migration v2 doc snippets, got %d", got)
	}
}
