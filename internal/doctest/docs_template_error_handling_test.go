package doctest

import (
	"context"
	"errors"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateErrorHandlingDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/error-handling.mdx"); err != nil {
		t.Fatalf("template error-handling doc is missing: %v", err)
	}
}

// This test keeps docs/template/error-handling.mdx aligned with the exported
// Go SDK template error surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsTemplateErrorHandlingExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "match-build-errors",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).FromBaseImage()

				_, err := e2b.Build(ctx, template, "my-template", nil)
				if err == nil {
					return
				}

				var authErr *e2b.AuthenticationError
				var uploadErr *e2b.FileUploadError
				var buildErr *e2b.BuildError

				_ = errors.As(err, &authErr)
				_ = errors.As(err, &uploadErr)
				_ = errors.As(err, &buildErr)
			},
		},
		{
			name: "inspect-caller-trace",
			fn: func() {
				ctx := context.Background()

				_, err := e2b.Build(ctx, e2b.Template(nil).FromTemplate("this-template-does-not-exist"), "my-template", nil)
				if err == nil {
					return
				}

				var buildErr *e2b.BuildError
				if errors.As(err, &buildErr) {
					_ = buildErr.Message
					_ = buildErr.CallerTrace
				}
			},
		},
		{
			name: "template-helper-validation",
			fn: func() {
				ctx := context.Background()

				_, err := e2b.AssignTags(ctx, "tmpl:latest", 123, nil)
				if err == nil {
					return
				}

				var templateErr *e2b.TemplateError
				_ = errors.As(err, &templateErr)

				_, _ = e2b.GetBuildStatus(ctx, nil, nil)
			},
		},
		{
			name: "recover-builder-validation-panic",
			fn: func() {
				defer func() {
					recovered := recover()
					if recovered == nil {
						return
					}

					err, ok := recovered.(error)
					if !ok {
						panic(recovered)
					}

					var buildErr *e2b.BuildError
					if errors.As(err, &buildErr) {
						_ = buildErr.Message
						return
					}

					panic(recovered)
				}()

				e2b.Template(nil).FromBaseImage().Copy("/absolute/path", "/app", nil)
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 template error-handling doc snippets, got %d", got)
	}
}
