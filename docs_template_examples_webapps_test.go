package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateExamplesExpoDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/examples/expo.mdx"); err != nil {
		t.Fatalf("template example expo doc is missing: %v", err)
	}
}

// This test keeps docs/template/examples/expo.mdx aligned with the exported Go
// SDK template, build, sandbox, and host surface. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsTemplateExamplesExpoExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "template-definition",
			fn: func() {
				template := e2b.Template(nil).
					FromNodeImage().
					SetWorkdir("/home/user/expo-app").
					RunCmd("npx create-expo-app@latest . --yes").
					RunCmd("mv /home/user/expo-app/* /home/user/ && rm -rf /home/user/expo-app").
					SetWorkdir("/home/user").
					SetStartCmd("npx expo start --web --port 8081", e2b.WaitForURL("http://127.0.0.1:8081"))

				_ = template
			},
		},
		{
			name: "build-template",
			fn: func() {
				template := e2b.Template(nil).
					FromNodeImage().
					SetWorkdir("/home/user/expo-app").
					RunCmd("npx create-expo-app@latest . --yes").
					RunCmd("mv /home/user/expo-app/* /home/user/ && rm -rf /home/user/expo-app").
					SetWorkdir("/home/user").
					SetStartCmd("npx expo start --web --port 8081", e2b.WaitForURL("http://127.0.0.1:8081"))

				buildInfo, buildErr := e2b.Build(context.Background(), template, "expo-app", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    4,
						MemoryMB:    8192,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})

				_ = buildInfo
				_ = buildErr
			},
		},
		{
			name: "create-sandbox-and-open",
			fn: func() {
				sandbox, err := e2b.Create(context.Background(), "expo-app", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				url := "https://" + sandbox.GetHost(8081)

				_ = url
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 template example expo doc snippets, got %d", got)
	}
}

func TestDocsTemplateExamplesNextjsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/examples/nextjs.mdx"); err != nil {
		t.Fatalf("template example nextjs doc is missing: %v", err)
	}
}

// This test keeps docs/template/examples/nextjs.mdx aligned with the exported
// Go SDK template, build, sandbox, and host surface. The closures are
// compile-only examples and are intentionally never executed.
func TestDocsTemplateExamplesNextjsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "template-definition",
			fn: func() {
				template := e2b.Template(nil).
					FromNodeImage("21-slim").
					SetWorkdir("/home/user/nextjs-app").
					RunCmd(`npx create-next-app@14.2.30 . --ts --tailwind --no-eslint --import-alias "@/*" --use-npm --no-app --no-src-dir`).
					RunCmd("npx shadcn@2.1.7 init -d").
					RunCmd("npx shadcn@2.1.7 add --all").
					RunCmd("mv /home/user/nextjs-app/* /home/user/ && rm -rf /home/user/nextjs-app").
					SetWorkdir("/home/user").
					SetStartCmd("npm run dev -- --hostname 0.0.0.0 --port 3000", e2b.WaitForURL("http://127.0.0.1:3000"))

				_ = template
			},
		},
		{
			name: "build-template",
			fn: func() {
				template := e2b.Template(nil).
					FromNodeImage("21-slim").
					SetWorkdir("/home/user/nextjs-app").
					RunCmd(`npx create-next-app@14.2.30 . --ts --tailwind --no-eslint --import-alias "@/*" --use-npm --no-app --no-src-dir`).
					RunCmd("npx shadcn@2.1.7 init -d").
					RunCmd("npx shadcn@2.1.7 add --all").
					RunCmd("mv /home/user/nextjs-app/* /home/user/ && rm -rf /home/user/nextjs-app").
					SetWorkdir("/home/user").
					SetStartCmd("npm run dev -- --hostname 0.0.0.0 --port 3000", e2b.WaitForURL("http://127.0.0.1:3000"))

				buildInfo, buildErr := e2b.Build(context.Background(), template, "nextjs-app", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    4,
						MemoryMB:    4096,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})

				_ = buildInfo
				_ = buildErr
			},
		},
		{
			name: "create-sandbox-and-open",
			fn: func() {
				sandbox, err := e2b.Create(context.Background(), "nextjs-app", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				url := "https://" + sandbox.GetHost(3000)

				_ = url
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 template example nextjs doc snippets, got %d", got)
	}
}

func TestDocsTemplateExamplesNextjsBunDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/examples/nextjs-bun.mdx"); err != nil {
		t.Fatalf("template example nextjs-bun doc is missing: %v", err)
	}
}

// This test keeps docs/template/examples/nextjs-bun.mdx aligned with the
// exported Go SDK template, build, sandbox, and host surface. The closures are
// compile-only examples and are intentionally never executed.
func TestDocsTemplateExamplesNextjsBunExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "template-definition",
			fn: func() {
				template := e2b.Template(nil).
					FromBunImage("1.3").
					SetWorkdir("/home/user/nextjs-app").
					RunCmd("bun create next-app --app --ts --tailwind --turbopack --yes --use-bun .").
					RunCmd("bunx --bun shadcn@latest init -d").
					RunCmd("bunx --bun shadcn@latest add --all").
					RunCmd("mv /home/user/nextjs-app/* /home/user/ && rm -rf /home/user/nextjs-app").
					SetWorkdir("/home/user").
					SetStartCmd("bun run dev -- --hostname 0.0.0.0 --port 3000", e2b.WaitForURL("http://127.0.0.1:3000"))

				_ = template
			},
		},
		{
			name: "build-template",
			fn: func() {
				template := e2b.Template(nil).
					FromBunImage("1.3").
					SetWorkdir("/home/user/nextjs-app").
					RunCmd("bun create next-app --app --ts --tailwind --turbopack --yes --use-bun .").
					RunCmd("bunx --bun shadcn@latest init -d").
					RunCmd("bunx --bun shadcn@latest add --all").
					RunCmd("mv /home/user/nextjs-app/* /home/user/ && rm -rf /home/user/nextjs-app").
					SetWorkdir("/home/user").
					SetStartCmd("bun run dev -- --hostname 0.0.0.0 --port 3000", e2b.WaitForURL("http://127.0.0.1:3000"))

				buildInfo, buildErr := e2b.Build(context.Background(), template, "nextjs-app-bun", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    4,
						MemoryMB:    4096,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})

				_ = buildInfo
				_ = buildErr
			},
		},
		{
			name: "create-sandbox-and-open",
			fn: func() {
				sandbox, err := e2b.Create(context.Background(), "nextjs-app-bun", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				url := "https://" + sandbox.GetHost(3000)

				_ = url
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 template example nextjs-bun doc snippets, got %d", got)
	}
}
