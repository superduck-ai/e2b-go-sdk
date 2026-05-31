package doctest

import (
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateDefiningTemplateDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/defining-template.mdx"); err != nil {
		t.Fatalf("template defining-template doc is missing: %v", err)
	}
}

// This test keeps docs/template/defining-template.mdx aligned with the
// exported Go SDK template-builder surface. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsTemplateDefiningTemplateExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "method-chaining",
			fn: func() {
				template := e2b.Template(nil).
					FromUbuntuImage("22.04").
					AptInstall([]string{"curl"}, nil).
					SetWorkdir("/app").
					Copy("package.json", "/app/package.json", nil).
					RunCmd("npm install").
					SetStartCmd("npm start", e2b.WaitForPort(3000))

				_ = template
			},
		},
		{
			name: "user-and-workdir",
			fn: func() {
				template := e2b.Template(nil).
					SetWorkdir("/app").
					SetUser("node").
					SetUser("1000:1000")

				_ = template
			},
		},
		{
			name: "copy-files",
			fn: func() {
				template := e2b.Template(nil).
					Copy("package.json", "/app/package.json", nil).
					Copy([]string{"file1", "file2"}, "/app/file", nil).
					CopyItems([]e2b.CopyItem{
						{Src: "src/", Dest: "/app/src/"},
						{Src: "package.json", Dest: "/app/package.json"},
					}).
					Copy("config.json", "/app/config.json", &struct {
						User string
						Mode int
					}{
						User: "appuser",
						Mode: 0o644,
					})

				_ = template
			},
		},
		{
			name: "file-operations",
			fn: func() {
				template := e2b.Template(nil).
					Remove("/tmp/temp-file.txt", nil).
					Remove("/old-directory", &struct{ Recursive bool }{Recursive: true}).
					Remove("/file.txt", &struct{ Force bool }{Force: true}).
					Rename("/old-name.txt", "/new-name.txt", nil).
					Rename("/old-dir", "/new-dir", &struct{ Force bool }{Force: true}).
					MakeDir("/app/logs", nil).
					MakeDir("/app/data", &struct{ Mode int }{Mode: 0o755}).
					MakeSymlink("/app/data", "/app/logs/data", nil)

				_ = template
			},
		},
		{
			name: "install-packages",
			fn: func() {
				template := e2b.Template(nil).
					PipInstall([]string{"requests", "pandas", "numpy"}, nil).
					PipInstall([]string{"requests", "pandas", "numpy"}, &struct{ G bool }{G: false}).
					NpmInstall([]string{"express", "lodash"}, nil).
					NpmInstall([]string{"express", "lodash"}, &struct{ G bool }{G: true}).
					BunInstall([]string{"express", "lodash"}, nil).
					BunInstall([]string{"express", "lodash"}, &struct{ G bool }{G: true}).
					AptInstall([]string{"curl", "wget", "git"}, nil)

				_ = template
			},
		},
		{
			name: "git-operations",
			fn: func() {
				template := e2b.Template(nil).
					GitClone("https://github.com/user/repo.git").
					GitClone("https://github.com/user/repo.git", "/app/repo").
					GitClone("https://github.com/user/repo.git", "/app/repo", &struct{ Branch string }{Branch: "main"}).
					GitClone("https://github.com/user/repo.git", "/app/repo", &struct{ Depth int }{Depth: 1})

				_ = template
			},
		},
		{
			name: "environment-variables",
			fn: func() {
				template := e2b.Template(nil).SetEnvs(map[string]string{
					"NODE_ENV": "production",
					"API_KEY":  "your-api-key",
					"DEBUG":    "true",
				})

				_ = template
			},
		},
		{
			name: "envs-and-run-cmd",
			fn: func() {
				template := e2b.Template(nil).
					SetEnvs(map[string]string{
						"NODE_ENV": "production",
						"API_KEY":  "your-api-key",
						"DEBUG":    "true",
					}).
					RunCmd("apt-get update && apt-get install -y curl").
					RunCmd([]string{"apt-get update", "apt-get install -y curl", "curl --version"}).
					RunCmd("npm install", &struct{ User string }{User: "node"})

				_ = template
			},
		},
	}

	if got := len(snippets); got != 8 {
		t.Fatalf("expected 8 template defining-template doc snippets, got %d", got)
	}
}
