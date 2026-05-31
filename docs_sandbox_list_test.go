package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxListDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/list.mdx"); err != nil {
		t.Fatalf("sandbox list doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/list.mdx aligned with the exported Go SDK list
// paginator surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsSandboxListExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "basic-listing",
			fn: func() {
				paginator := e2b.List(nil)

				firstPage, err := paginator.NextItems()
				var sandbox e2b.SandboxInfo
				if len(firstPage) > 0 {
					sandbox = firstPage[0]
				}

				_ = sandbox.SandboxID
				_ = sandbox.TemplateID
				_ = sandbox.State
				_ = sandbox.Metadata
				_ = sandbox.StartedAt
				_ = sandbox.EndAt
				_ = err
			},
		},
		{
			name: "filter-by-state",
			fn: func() {
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

				sandboxes, err := paginator.NextItems()

				_ = sandboxes
				_ = err
			},
		},
		{
			name: "filter-by-metadata",
			fn: func() {
				paginator := e2b.List(&e2b.SandboxListOpts{
					Query: &struct {
						Metadata map[string]string
						State    []e2b.SandboxState
					}{
						Metadata: map[string]string{
							"userID": "123",
							"env":    "dev",
						},
					},
				})

				sandboxes, err := paginator.NextItems()

				_ = sandboxes
				_ = err
			},
		},
		{
			name: "pagination",
			fn: func() {
				paginator := e2b.List(&e2b.SandboxListOpts{
					Limit:     100,
					NextToken: "<base64-encoded-token>",
				})

				page, err := paginator.NextItems()
				allSandboxes := append([]e2b.SandboxInfo{}, page...)

				for paginator.HasNext {
					items, pageErr := paginator.NextItemsContext(context.Background())
					allSandboxes = append(allSandboxes, items...)
					_ = pageErr
				}

				_ = paginator.NextToken
				_ = page
				_ = allSandboxes
				_ = err
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 sandbox list doc snippets, got %d", got)
	}
}
