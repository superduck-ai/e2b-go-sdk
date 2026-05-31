package doctest

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFilesystemUploadDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/filesystem/upload.mdx"); err != nil {
		t.Fatalf("filesystem upload doc is missing: %v", err)
	}
}

// This test keeps docs/filesystem/upload.mdx aligned with the exported Go SDK
// upload surface. The closures are compile-only examples and are intentionally
// never executed.
func TestDocsFilesystemUploadExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "upload-single-file",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				content, readErr := os.ReadFile("/local/path")
				_, writeErr := sandbox.Files.Write(ctx, "/path/in/sandbox", content, nil)

				_ = readErr
				_ = writeErr
			},
		},
		{
			name: "upload-with-signed-url",
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
				publicUploadURL, urlErr := sandbox.UploadUrl("demo.txt", &struct {
					UseSignatureExpiration *int
					User                   string
				}{
					UseSignatureExpiration: &expirationMs,
				})

				var body bytes.Buffer
				writer := multipart.NewWriter(&body)
				part, _ := writer.CreateFormFile("file", "demo.txt")
				_, _ = part.Write([]byte("file content"))
				_ = writer.Close()

				req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, publicUploadURL, &body)
				if req != nil {
					req.Header.Set("Content-Type", writer.FormDataContentType())
				}

				resp, doErr := http.DefaultClient.Do(req)
				if resp != nil {
					resp.Body.Close()
				}

				_ = urlErr
				_ = reqErr
				_ = doErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 filesystem upload doc snippets, got %d", got)
	}
}
