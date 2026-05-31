package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsQuickstartUploadDownloadFilesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/quickstart/upload-download-files.mdx"); err != nil {
		t.Fatalf("quickstart upload-download-files doc is missing: %v", err)
	}
}

// This test keeps docs/quickstart/upload-download-files.mdx aligned with the
// exported Go SDK filesystem transfer surface. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsQuickstartUploadDownloadFilesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "upload-one-file",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				content, readErr := os.ReadFile("local/file")
				info, writeErr := sandbox.Files.Write(ctx, "/home/user/my-file", content, nil)

				_ = info
				_ = readErr
				_ = writeErr
			},
		},
		{
			name: "upload-multiple-files",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				fileA, readAErr := os.ReadFile("local/file/a")
				fileB, readBErr := os.ReadFile("local/file/b")

				infoA, writeAErr := sandbox.Files.Write(ctx, "/home/user/my-file-a", fileA, nil)
				infoB, writeBErr := sandbox.Files.Write(ctx, "/home/user/my-file-b", fileB, nil)

				_ = infoA
				_ = infoB
				_ = readAErr
				_ = readBErr
				_ = writeAErr
				_ = writeBErr
			},
		},
		{
			name: "download-one-file",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				contentValue, readErr := sandbox.Files.Read(ctx, "/home/user/my-file", nil)
				content := contentValue.(string)
				writeErr := os.WriteFile("local/file", []byte(content), 0o644)

				_ = readErr
				_ = writeErr
			},
		},
		{
			name: "download-multiple-files",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				contentAValue, readAErr := sandbox.Files.Read(ctx, "/home/user/my-file-a", nil)
				contentA := contentAValue.(string)

				contentBValue, readBErr := sandbox.Files.Read(ctx, "/home/user/my-file-b", nil)
				contentB := contentBValue.(string)

				writeAErr := os.WriteFile("local/file/a", []byte(contentA), 0o644)
				writeBErr := os.WriteFile("local/file/b", []byte(contentB), 0o644)

				_ = readAErr
				_ = readBErr
				_ = writeAErr
				_ = writeBErr
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 quickstart upload-download-files doc snippets, got %d", got)
	}
}
