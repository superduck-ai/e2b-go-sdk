package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsStorageArchilDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/storage/archil.mdx"); err != nil {
		t.Fatalf("storage archil doc is missing: %v", err)
	}
}

// This test keeps docs/storage/archil.mdx aligned with the exported Go SDK
// template, sandbox, connect, and command surface used to mount Archil. The
// closures are compile-only examples and are intentionally never executed.
func TestDocsStorageArchilExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "build-template",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					RunCmd("curl -fsSL https://archil.com/install | sh", &struct{ User string }{User: "root"})

				_ = template
			},
		},
		{
			name: "mount-disk",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "<your-template-id>", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				mkdirResult, mkdirErr := sandbox.Commands.Run(ctx, "mkdir -p /home/user/archil", &e2b.CommandStartOpts{
					User: "root",
				})
				mountResult, mountErr := sandbox.Commands.Run(
					ctx,
					"archil mount <disk-name-or-id> /home/user/archil --region <region>",
					&e2b.CommandStartOpts{
						User: "root",
						Envs: map[string]string{
							"ARCHIL_MOUNT_TOKEN": "<disk-token>",
						},
					},
				)
				chownResult, chownErr := sandbox.Commands.Run(ctx, "chown user:user /home/user/archil", &e2b.CommandStartOpts{
					User: "root",
				})

				_ = mkdirResult
				_ = mountResult
				_ = chownResult
				_ = mkdirErr
				_ = mountErr
				_ = chownErr
			},
		},
		{
			name: "unmount",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				result, runErr := sandbox.Commands.Run(ctx, "archil unmount /home/user/archil", &e2b.CommandStartOpts{
					User: "root",
				})

				_ = result
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 storage archil doc snippets, got %d", got)
	}
}

func TestDocsStorageCloudBucketsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/storage/cloud-buckets.mdx"); err != nil {
		t.Fatalf("storage cloud-buckets doc is missing: %v", err)
	}
}

// This test keeps docs/storage/cloud-buckets.mdx aligned with the exported Go
// SDK template, filesystem, sandbox, and command surface used to mount cloud
// buckets with FUSE tools. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsStorageCloudBucketsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "gcs-template",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					AptInstall([]string{"gnupg", "lsb-release"}, &struct{ User string }{User: "root"}).
					RunCmd("lsb_release -c -s > /tmp/lsb_release", &struct{ User string }{User: "root"}).
					RunCmd(`GCSFUSE_REPO=$(cat /tmp/lsb_release) && echo "deb [signed-by=/usr/share/keyrings/cloud.google.asc] https://packages.cloud.google.com/apt gcsfuse-$GCSFUSE_REPO main" > /etc/apt/sources.list.d/gcsfuse.list`, &struct{ User string }{User: "root"}).
					RunCmd("curl https://packages.cloud.google.com/apt/doc/apt-key.gpg > /usr/share/keyrings/cloud.google.asc", &struct{ User string }{User: "root"}).
					AptInstall([]string{"gcsfuse"}, &struct{ User string }{User: "root"})

				_ = template
			},
		},
		{
			name: "gcs-mount",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "<your-template-id>", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				created, mkdirErr := sandbox.Files.MakeDir(ctx, "/home/user/bucket", nil)
				keyInfo, writeErr := sandbox.Files.Write(ctx, "/home/user/key.json", "<your service account key>", nil)
				result, mountErr := sandbox.Commands.Run(
					ctx,
					`gcsfuse -o allow_other --file-mode=777 --dir-mode=777 --key-file /home/user/key.json <bucket-name> /home/user/bucket`,
					&e2b.CommandStartOpts{
						User: "root",
					},
				)

				_ = created
				_ = keyInfo
				_ = result
				_ = mkdirErr
				_ = writeErr
				_ = mountErr
			},
		},
		{
			name: "s3-template",
			fn: func() {
				template := e2b.Template(nil).
					FromUbuntuImage("latest").
					AptInstall([]string{"s3fs"}, &struct{ User string }{User: "root"})

				_ = template
			},
		},
		{
			name: "s3-mount",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "<your-template-id>", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				created, mkdirErr := sandbox.Files.MakeDir(ctx, "/home/user/bucket", nil)
				credsInfo, writeErr := sandbox.Files.Write(ctx, "/home/user/.passwd-s3fs", "<AWS_ACCESS_KEY_ID>:<AWS_SECRET_ACCESS_KEY>", nil)
				chmodResult, chmodErr := sandbox.Commands.Run(ctx, "chmod 600 /home/user/.passwd-s3fs", nil)
				mountResult, mountErr := sandbox.Commands.Run(
					ctx,
					`sudo s3fs -o passwd_file=/home/user/.passwd-s3fs -o allow_other <bucket-name> /home/user/bucket`,
					nil,
				)

				_ = created
				_ = credsInfo
				_ = chmodResult
				_ = mountResult
				_ = mkdirErr
				_ = writeErr
				_ = chmodErr
				_ = mountErr
			},
		},
		{
			name: "r2-mount",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "<your-template-id>", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				created, mkdirErr := sandbox.Files.MakeDir(ctx, "/home/user/bucket", nil)
				credsInfo, writeErr := sandbox.Files.Write(ctx, "/home/user/.passwd-s3fs", "<R2_ACCESS_KEY_ID>:<R2_SECRET_ACCESS_KEY>", nil)
				chmodResult, chmodErr := sandbox.Commands.Run(ctx, "chmod 600 /home/user/.passwd-s3fs", nil)
				mountResult, mountErr := sandbox.Commands.Run(
					ctx,
					`sudo s3fs -o passwd_file=/home/user/.passwd-s3fs -o url=https://<ACCOUNT_ID>.r2.cloudflarestorage.com -o allow_other <bucket-name> /home/user/bucket`,
					nil,
				)

				_ = created
				_ = credsInfo
				_ = chmodResult
				_ = mountResult
				_ = mkdirErr
				_ = writeErr
				_ = chmodErr
				_ = mountErr
			},
		},
	}

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 storage cloud-buckets doc snippets, got %d", got)
	}
}
