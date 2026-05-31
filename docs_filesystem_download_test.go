package e2b_test

import (
	"context"
	"io"
	"net/http"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFilesystemDownloadDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/filesystem/download.mdx"); err != nil {
		t.Fatalf("filesystem download doc is missing: %v", err)
	}
}

// This test keeps docs/filesystem/download.mdx aligned with the exported Go
// SDK download surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsFilesystemDownloadExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "download-directly",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				contentValue, readErr := sandbox.Files.Read(ctx, "/path/in/sandbox", nil)
				content := contentValue.(string)

				writeErr := os.WriteFile("/local/path", []byte(content), 0o644)
				_ = readErr
				_ = writeErr
			},
		},
		{
			name: "download-with-signed-url",
			fn: func() {
				ctx := context.Background()
				secure := true
				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Secure: &secure,
				})
				if err != nil {
					return
				}

				expirationMs := 10_000
				publicURL, urlErr := sandbox.DownloadUrl("demo.txt", &struct {
					UseSignatureExpiration *int
					User                   string
				}{
					UseSignatureExpiration: &expirationMs,
				})

				res, getErr := http.Get(publicURL)
				if res != nil {
					defer res.Body.Close()
					_, _ = io.ReadAll(res.Body)
				}

				_ = urlErr
				_ = getErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 filesystem download doc snippets, got %d", got)
	}
}
