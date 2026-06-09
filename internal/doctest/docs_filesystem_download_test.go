package doctest

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFilesystemDownloadDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/filesystem/download.mdx"); err != nil {
		t.Fatalf("filesystem download doc is missing: %v", err)
	}
}

func TestDocsFilesystemDownloadExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "download-directly",
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				sandboxPath := "/home/user/download-source.txt"
				_, prepErr := sandbox.Files.Write(ctx, sandboxPath, "file content", nil)
				if !assert.NoError(t, prepErr, "failed to prepare sandbox file") {
					return
				}

				contentValue, readErr := sandbox.Files.Read(ctx, sandboxPath, nil)
				if !assert.NoError(t, readErr, "failed to read sandbox file") {
					return
				}
				content := contentValue.(string)

				localPath := filepath.Join(t.TempDir(), "download.txt")
				writeErr := os.WriteFile(localPath, []byte(content), 0o644)
				assert.NoError(t, writeErr, "failed to write local file")
			},
		},
		{
			name: "download-with-signed-url",
			fn: func(t *testing.T) {
				t.Skip("requires Secure sandbox + reachable signed URL endpoint")

				ctx := context.Background()
				secure := true
				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Secure: &secure,
				})
				if !assert.NoError(t, err, "failed to create secure sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				expirationMs := 10_000
				publicURL, urlErr := sandbox.DownloadUrl("demo.txt", &struct {
					UseSignatureExpiration *int
					User                   string
				}{
					UseSignatureExpiration: &expirationMs,
				})
				if !assert.NoError(t, urlErr, "failed to obtain signed URL") {
					return
				}

				res, getErr := http.Get(publicURL)
				if !assert.NoError(t, getErr, "failed to GET signed URL") {
					return
				}
				defer res.Body.Close()
				_, _ = io.ReadAll(res.Body)
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 filesystem download doc snippets, got %d", got)
	}

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
