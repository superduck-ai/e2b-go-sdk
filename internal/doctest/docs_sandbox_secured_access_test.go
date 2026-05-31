package doctest

import (
	"context"
	"net/http"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxSecuredAccessDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/secured-access.mdx"); err != nil {
		t.Fatalf("sandbox secured access doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/secured-access.mdx aligned with the exported Go
// SDK secure sandbox and signed URL surface. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsSandboxSecuredAccessExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "default-secure-and-disable",
			fn: func() {
				ctx := context.Background()

				secureSandbox, secureErr := e2b.Create(ctx, "", nil)
				if secureSandbox != nil {
					defer secureSandbox.Kill(context.Background(), nil)
				}

				secure := false
				legacySandbox, legacyErr := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Secure: &secure,
				})
				if legacySandbox != nil {
					defer legacySandbox.Kill(context.Background(), nil)
				}

				_ = secureSandbox
				_ = legacySandbox
				_ = secureErr
				_ = legacyErr
			},
		},
		{
			name: "traffic-token",
			fn: func() {
				ctx := context.Background()
				secure := true
				allowPublicTraffic := false

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Secure: &secure,
					Network: &e2b.SandboxNetworkOpts{
						AllowPublicTraffic: &allowPublicTraffic,
					},
				})
				if err != nil {
					return
				}

				req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+sandbox.GetHost(3000), nil)
				if req != nil && sandbox.TrafficAccessToken != "" {
					req.Header.Set("e2b-traffic-access-token", sandbox.TrafficAccessToken)
				}

				_ = req
				_ = reqErr
			},
		},
		{
			name: "signed-file-urls",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				expirationSeconds := 60

				downloadURL, downloadErr := sandbox.DownloadUrl("hello.txt", &struct {
					UseSignatureExpiration *int
					User                   string
				}{
					UseSignatureExpiration: &expirationSeconds,
				})
				uploadURL, uploadErr := sandbox.UploadUrl("uploaded.txt", &struct {
					UseSignatureExpiration *int
					User                   string
				}{
					UseSignatureExpiration: &expirationSeconds,
				})
				rootDownloadURL, rootDownloadErr := sandbox.DownloadUrl("root-only.txt", &struct {
					UseSignatureExpiration *int
					User                   string
				}{
					UseSignatureExpiration: &expirationSeconds,
					User:                   "root",
				})

				_ = downloadURL
				_ = uploadURL
				_ = rootDownloadURL
				_ = downloadErr
				_ = uploadErr
				_ = rootDownloadErr
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 sandbox secured access doc snippets, got %d", got)
	}
}
