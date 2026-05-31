package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateReferenceDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sdk-reference/go-sdk/template.mdx"); err != nil {
		t.Fatalf("template reference doc is missing: %v", err)
	}
}

// This test keeps docs/sdk-reference/go-sdk/template.mdx aligned with the
// exported Go SDK template surface. The closures are compile-only examples and
// are intentionally never executed.
func TestDocsTemplateReferenceExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "choose-a-base",
			fn: func() {
				fromBase := e2b.Template(nil).FromBaseImage()
				fromDebian := e2b.Template(nil).FromDebianImage("bookworm")
				fromUbuntu := e2b.Template(nil).FromUbuntuImage("24.04")
				fromPython := e2b.Template(nil).FromPythonImage("3.12")
				fromNode := e2b.Template(nil).FromNodeImage("24")
				fromBun := e2b.Template(nil).FromBunImage("1.3")
				fromDockerfile := e2b.Template(&e2b.TemplateOptions{
					FileContextPath:    ".",
					FileIgnorePatterns: []string{"node_modules", "*.tmp"},
				}).FromDockerfile("Dockerfile")
				fromImage := e2b.Template(nil).FromImage("ghcr.io/acme/private:latest", &e2b.RegistryCredentials{
					Username: "user",
					Password: "token",
				})
				fromAWS := e2b.Template(nil).FromAWSRegistry(
					"123456789.dkr.ecr.us-west-2.amazonaws.com/app:latest",
					&e2b.AWSRegistryCredentials{
						AccessKeyID:     "AKIA...",
						SecretAccessKey: "...",
						Region:          "us-west-2",
					},
				)
				fromGCP := e2b.Template(&e2b.TemplateOptions{FileContextPath: "."}).FromGCPRegistry(
					"gcr.io/myproject/app:latest",
					&e2b.GCPRegistryCredentials{ServiceAccountJSON: "service-account.json"},
				)
				fromTemplate := e2b.Template(nil).FromTemplate("base-template")

				_ = fromBase
				_ = fromDebian
				_ = fromUbuntu
				_ = fromPython
				_ = fromNode
				_ = fromBun
				_ = fromDockerfile
				_ = fromImage
				_ = fromAWS
				_ = fromGCP
				_ = fromTemplate
			},
		},
		{
			name: "builder-methods",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					SkipCache().
					Copy("app.ts", "/app/", map[string]any{
						"forceUpload": true,
						"mode":        0o755,
					}).
					CopyItems([]e2b.CopyItem{
						{Src: "config.yaml", Dest: "/app/"},
						{Src: []string{"one.txt", "two.txt"}, Dest: "/assets", Mode: 0o644},
					}).
					Remove("/tmp/cache", &struct {
						Recursive bool
						Force     bool
					}{Recursive: true, Force: true}).
					Rename("/app/config.yaml", "/app/config.example.yaml", nil).
					MakeDir("/app/logs", &struct{ Mode int }{Mode: 0o755}).
					MakeSymlink("/usr/bin/python3", "/usr/bin/python", &struct{ Force bool }{Force: true}).
					RunCmd([]string{"go mod download", "go build ./..."}, &struct {
						User  string
						Force bool
					}{User: "root", Force: true}).
					SetWorkdir("/app").
					SetUser("user").
					SetEnvs(map[string]string{
						"APP_ENV": "production",
						"PORT":    "8000",
					}).
					PipInstall("requests", nil).
					PipInstall([]string{"pandas"}, &struct{ G bool }{G: false}).
					NpmInstall("tsx", &struct{ G bool }{G: true}).
					BunInstall("typescript", &struct{ Dev bool }{Dev: true}).
					AptInstall([]string{"git", "curl"}, &struct {
						NoInstallRecommends bool
						FixMissing          bool
					}{NoInstallRecommends: true, FixMissing: true}).
					GitClone("https://github.com/e2b-dev/e2b.git", "/src", &struct {
						Branch string
						Depth  int
						User   string
					}{Branch: "main", Depth: 1, User: "root"})

				_ = template
			},
		},
		{
			name: "start-ready-and-advanced-helpers",
			fn: func() {
				template := e2b.Template(nil).
					FromUbuntuImage("22.04").
					SetStartCmd("python -m http.server 8000", e2b.WaitForPort(8000)).
					SetReadyCmd("curl -fsS http://127.0.0.1:8000/health")

				portReady := e2b.WaitForPort(8000)
				urlReady := e2b.WaitForURL("http://127.0.0.1:8000/health", 200)
				processReady := e2b.WaitForProcess("nginx")
				fileReady := e2b.WaitForFile("/tmp/ready")
				timeoutReady := e2b.WaitForTimeout(2500)

				mcp := e2b.Template(nil).
					FromTemplate("mcp-gateway").
					AddMcpServer("exa", []string{"brave", "duckduckgo"})

				devcontainer := e2b.Template(nil).
					FromTemplate("devcontainer").
					BetaDevContainerPrebuild("/workspace").
					BetaSetDevContainerStart("/workspace")

				_ = template
				_ = portReady.GetCmd()
				_ = urlReady.GetCmd()
				_ = processReady.GetCmd()
				_ = fileReady.GetCmd()
				_ = timeoutReady.GetCmd()
				_ = mcp
				_ = devcontainer
			},
		},
		{
			name: "template-specific-helpers",
			fn: func() {
				mcp := e2b.Template(nil).
					FromTemplate("mcp-gateway").
					AddMcpServer("exa", []string{"brave", "duckduckgo"})

				devcontainer := e2b.Template(nil).
					FromTemplate("devcontainer").
					BetaDevContainerPrebuild("/workspace").
					BetaSetDevContainerStart("/workspace")

				_ = mcp
				_ = devcontainer
			},
		},
		{
			name: "build-status-and-tags",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 120000

				template := e2b.Template(nil).
					FromBaseImage().
					RunCmd(`echo "hello"`).
					SetStartCmd(`python -m http.server 8000`, e2b.WaitForPort(8000))

				buildOpts := &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						Alias:       "legacy-alias",
						Tags:        []string{"stable"},
						CpuCount:    2,
						MemoryMB:    1024,
						SkipCache:   true,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
					Signal:           ctx,
					RequestTimeoutMs: &timeoutMs,
					Headers: map[string]string{
						"X-Request-ID": "docs-template",
					},
				}

				info, buildErr := e2b.Build(ctx, template, "docs-template", buildOpts)
				backgroundInfo, backgroundErr := e2b.BuildInBackground(ctx, template, "", buildOpts)
				status, statusErr := e2b.GetBuildStatus(ctx, &e2b.BuildInfo{
					TemplateID: "tmpl_123",
					BuildID:    "bld_123",
				}, &e2b.GetBuildStatusOptions{
					LogsOffset:       5,
					Signal:           ctx,
					RequestTimeoutMs: &timeoutMs,
					Headers: map[string]string{
						"X-Request-ID": "docs-template-status",
					},
				})

				exists, existsErr := e2b.Exists(ctx, "docs-template", &e2b.ConnectionOpts{
					Signal: ctx,
					Headers: map[string]string{
						"X-Trace-ID": "docs-template",
					},
				})
				tagInfo, assignErr := e2b.AssignTags(ctx, "docs-template:stable", []string{"production"}, nil)
				removeErr := e2b.RemoveTags(ctx, "docs-template", "production", nil)
				tags, tagsErr := e2b.GetTags(ctx, "tmpl_123", nil)

				if info != nil {
					_ = info.Alias
					_ = info.Name
					_ = info.Tags
					_ = info.TemplateID
					_ = info.BuildID
				}
				if backgroundInfo != nil {
					_ = backgroundInfo.TemplateID
					_ = backgroundInfo.BuildID
				}
				if status != nil {
					_ = status.BuildID
					_ = status.TemplateID
					_ = status.Status
					_ = status.LogEntries
					_ = status.Logs
					_ = status.Reason
				}
				if tagInfo != nil {
					_ = tagInfo.BuildID
					_ = tagInfo.Tags
				}
				if len(tags) > 0 {
					_ = tags[0].Tag
					_ = tags[0].BuildID
					_ = tags[0].CreatedAt
				}

				buildingStatus := e2b.TemplateBuildStatus("building")
				waitingStatus := e2b.TemplateBuildStatus("waiting")
				readyStatus := e2b.TemplateBuildStatus("ready")
				errorStatus := e2b.TemplateBuildStatus("error")

				_ = buildingStatus
				_ = waitingStatus
				_ = readyStatus
				_ = errorStatus
				_ = exists
				_ = buildErr
				_ = backgroundErr
				_ = statusErr
				_ = existsErr
				_ = assignErr
				_ = removeErr
				_ = tagsErr
			},
		},
		{
			name: "existence-and-tags",
			fn: func() {
				ctx := context.Background()
				info := &e2b.BuildInfo{TemplateID: "tmpl_123"}

				exists, existsErr := e2b.Exists(ctx, "docs-template", nil)
				tagInfo, assignErr := e2b.AssignTags(ctx, "docs-template:stable", []string{"production"}, nil)
				removeErr := e2b.RemoveTags(ctx, "docs-template", "production", nil)
				tags, tagsErr := e2b.GetTags(ctx, info.TemplateID, nil)

				_ = exists
				_ = tagInfo
				_ = tags
				_ = existsErr
				_ = assignErr
				_ = removeErr
				_ = tagsErr
			},
		},
		{
			name: "serialization-and-log-helpers",
			fn: func() {
				template := e2b.Template(nil).
					FromPythonImage("3.12").
					SetEnvs(map[string]string{"PORT": "8000"}).
					SetStartCmd("python main.py", e2b.WaitForPort(8000))

				jsonText, jsonErr := e2b.ToJSON(template, true)
				dockerfile, dockerfileErr := e2b.ToDockerfile(template)

				var logger e2b.BuildLogger = func(entry *e2b.LogEntry) {
					_ = entry.Timestamp
					_ = entry.Level
					_ = entry.Message
					_ = entry.String()
				}

				start := e2b.NewLogEntryStart("uploading files")
				end := e2b.NewLogEntryEnd("build finished")
				defaultLogger := e2b.DefaultBuildLogger()

				_ = jsonText
				_ = dockerfile
				_ = jsonErr
				_ = dockerfileErr
				_ = logger
				_ = start.Timestamp
				_ = start.Level
				_ = start.Message
				_ = end.Timestamp
				_ = end.Level
				_ = end.Message
				_ = defaultLogger
			},
		},
	}

	if got := len(snippets); got != 7 {
		t.Fatalf("expected 7 template doc snippets, got %d", got)
	}
}
