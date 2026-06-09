package doctest

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFilesystemUploadDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/filesystem/upload.mdx"); err != nil {
		t.Fatalf("filesystem upload doc is missing: %v", err)
	}
}

func TestDocsFilesystemUploadExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "upload-single-file",
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				localPath := filepath.Join(t.TempDir(), "upload.txt")
				if !assert.NoError(t, os.WriteFile(localPath, []byte("file content"), 0o644), "failed to seed local file") {
					return
				}

				content, readErr := os.ReadFile(localPath)
				if !assert.NoError(t, readErr, "failed to read local file") {
					return
				}
				_, writeErr := sandbox.Files.Write(ctx, "/home/user/upload-target.txt", content, nil)
				assert.NoError(t, writeErr, "failed to upload to sandbox")
			},
		},
		{
			name: "upload-with-signed-url",
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
				publicUploadURL, urlErr := sandbox.UploadUrl("demo.txt", &struct {
					UseSignatureExpiration *int
					User                   string
				}{
					UseSignatureExpiration: &expirationMs,
				})
				if !assert.NoError(t, urlErr, "failed to obtain signed upload URL") {
					return
				}

				var body bytes.Buffer
				writer := multipart.NewWriter(&body)
				part, _ := writer.CreateFormFile("file", "demo.txt")
				_, _ = part.Write([]byte("file content"))
				_ = writer.Close()

				req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, publicUploadURL, &body)
				if !assert.NoError(t, reqErr, "failed to build upload request") {
					return
				}
				req.Header.Set("Content-Type", writer.FormDataContentType())

				resp, doErr := http.DefaultClient.Do(req)
				if !assert.NoError(t, doErr, "failed to POST to signed URL") {
					return
				}
				resp.Body.Close()
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 filesystem upload doc snippets, got %d", got)
	}

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
