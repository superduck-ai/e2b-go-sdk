package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateUserWorkdirDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/user-and-workdir.mdx"); err != nil {
		t.Fatalf("template user-and-workdir doc is missing: %v", err)
	}
}

// This test keeps docs/template/user-and-workdir.mdx aligned with the
// exported Go SDK user/workdir surface. The closures are compile-only examples
// and are intentionally never executed.
func TestDocsTemplateUserWorkdirExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "default-sandbox-user-and-workdir",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				whoamiExec, whoamiErr := sandbox.Commands.Run(ctx, "whoami", nil)
				pwdExec, pwdErr := sandbox.Commands.Run(ctx, "pwd", nil)

				whoami := whoamiExec.(*e2b.CommandResult)
				pwd := pwdExec.(*e2b.CommandResult)

				_ = whoami.Stdout
				_ = pwd.Stdout
				_ = whoamiErr
				_ = pwdErr
			},
		},
		{
			name: "custom-template-user-and-workdir",
			fn: func() {
				ctx := context.Background()

				template := e2b.Template(nil).
					FromBaseImage().
					RunCmd("whoami").
					RunCmd("pwd").
					SetUser("guest").
					SetWorkdir("/home/guest").
					RunCmd("whoami").
					RunCmd("pwd")

				buildInfo, buildErr := e2b.Build(ctx, template, "custom-user-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})

				sandbox, sandboxErr := e2b.Create(ctx, "custom-user-template", nil)
				if sandbox == nil {
					_ = buildInfo
					_ = buildErr
					_ = sandboxErr
					return
				}

				whoamiExec, whoamiErr := sandbox.Commands.Run(ctx, "whoami", nil)
				pwdExec, pwdErr := sandbox.Commands.Run(ctx, "pwd", nil)

				whoami := whoamiExec.(*e2b.CommandResult)
				pwd := pwdExec.(*e2b.CommandResult)

				_ = whoami.Stdout
				_ = pwd.Stdout
				_ = buildInfo
				_ = buildErr
				_ = sandboxErr
				_ = whoamiErr
				_ = pwdErr
			},
		},
		{
			name: "build-step-user-override",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					RunCmd("apt-get update", &struct{ User string }{User: "root"}).
					RunCmd("apt-get install -y curl", &struct{ User string }{User: "root"}).
					SetUser("user").
					SetWorkdir("/home/user")

				_ = template
			},
		},
		{
			name: "runtime-user-and-workdir-override",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				asRootExec, asRootErr := sandbox.Commands.Run(ctx, "whoami", &e2b.CommandStartOpts{
					User: "root",
				})
				inTmpExec, inTmpErr := sandbox.Commands.Run(ctx, "pwd", &e2b.CommandStartOpts{
					Cwd: "/tmp",
				})

				asRoot := asRootExec.(*e2b.CommandResult)
				inTmp := inTmpExec.(*e2b.CommandResult)

				_ = asRoot.Stdout
				_ = inTmp.Stdout
				_ = asRootErr
				_ = inTmpErr
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 template user/workdir doc snippets, got %d", got)
	}
}
