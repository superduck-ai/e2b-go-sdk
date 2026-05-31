package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsCodeInterpretingSupportedLanguagesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/supported-languages.mdx"); err != nil {
		t.Fatalf("code-interpreting supported-languages doc is missing: %v", err)
	}
}

// This overview is intentionally prose-only because the linked pages document
// the concrete runtime setup and command patterns for each language.
func TestDocsCodeInterpretingSupportedLanguagesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 supported-languages doc snippets, got %d", got)
	}
}

func TestDocsCodeInterpretingSupportedLanguagesBashDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/supported-languages/bash.mdx"); err != nil {
		t.Fatalf("code-interpreting bash doc is missing: %v", err)
	}
}

// This test keeps docs/code-interpreting/supported-languages/bash.mdx aligned
// with the exported Go SDK sandbox and command surface. The closures are
// compile-only examples and are intentionally never executed.
func TestDocsCodeInterpretingSupportedLanguagesBashExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "run-bash-command",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "base", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, `bash -lc 'echo "Hello, world!"'`, nil)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 bash doc snippet, got %d", got)
	}
}

func TestDocsCodeInterpretingSupportedLanguagesJavaDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/supported-languages/java.mdx"); err != nil {
		t.Fatalf("code-interpreting java doc is missing: %v", err)
	}
}

// This test keeps docs/code-interpreting/supported-languages/java.mdx aligned
// with the exported Go SDK template, sandbox, filesystem, and command surface.
// The closures are compile-only examples and are intentionally never executed.
func TestDocsCodeInterpretingSupportedLanguagesJavaExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "run-java-code",
			fn: func() {
				template := e2b.Template(nil).
					FromUbuntuImage("24.04").
					AptInstall([]string{"openjdk-21-jdk"}, &struct{ User string }{User: "root"})

				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "java-runtime", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, writeErr := sandbox.Files.Write(ctx, "/home/user/Main.java", `public class Main {
  public static void main(String[] args) {
    System.out.println("Hello, world!");
  }
}`, nil)
				execution, runErr := sandbox.Commands.Run(ctx, "cd /home/user && javac Main.java && java Main", nil)
				result := execution.(*e2b.CommandResult)

				_ = template
				_ = result.Stdout
				_ = writeErr
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 java doc snippet, got %d", got)
	}
}

func TestDocsCodeInterpretingSupportedLanguagesJavaScriptDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/supported-languages/javascript.mdx"); err != nil {
		t.Fatalf("code-interpreting javascript doc is missing: %v", err)
	}
}

// This test keeps docs/code-interpreting/supported-languages/javascript.mdx
// aligned with the exported Go SDK template, sandbox, filesystem, and command
// surface. The closures are compile-only examples and are intentionally never
// executed.
func TestDocsCodeInterpretingSupportedLanguagesJavaScriptExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "run-javascript",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "node-runtime", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, `node -e "console.log('Hello, world!')"`, nil)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
		{
			name: "run-typescript",
			fn: func() {
				template := e2b.Template(nil).
					FromNodeImage("24").
					RunCmd("npm install -g tsx", &struct{ User string }{User: "root"})

				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "ts-runtime", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, writeErr := sandbox.Files.Write(ctx, "/home/user/script.ts", `const message: string = "Hello, world!"
console.log(message)
`, nil)
				execution, runErr := sandbox.Commands.Run(ctx, "cd /home/user && npx tsx script.ts", nil)
				result := execution.(*e2b.CommandResult)

				_ = template
				_ = result.Stdout
				_ = writeErr
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 javascript doc snippets, got %d", got)
	}
}

func TestDocsCodeInterpretingSupportedLanguagesPythonDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/supported-languages/python.mdx"); err != nil {
		t.Fatalf("code-interpreting python doc is missing: %v", err)
	}
}

// This test keeps docs/code-interpreting/supported-languages/python.mdx
// aligned with the exported Go SDK template, sandbox, and command surface. The
// closures are compile-only examples and are intentionally never executed.
func TestDocsCodeInterpretingSupportedLanguagesPythonExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "run-python",
			fn: func() {
				template := e2b.Template(nil).FromPythonImage("3.12")

				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "python-runtime", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, `python3 -c 'print("Hello, world!")'`, nil)
				result := execution.(*e2b.CommandResult)

				_ = template
				_ = result.Stdout
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 python doc snippet, got %d", got)
	}
}

func TestDocsCodeInterpretingSupportedLanguagesRDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/code-interpreting/supported-languages/r.mdx"); err != nil {
		t.Fatalf("code-interpreting r doc is missing: %v", err)
	}
}

// This test keeps docs/code-interpreting/supported-languages/r.mdx aligned
// with the exported Go SDK template, sandbox, and command surface. The
// closures are compile-only examples and are intentionally never executed.
func TestDocsCodeInterpretingSupportedLanguagesRExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "run-r",
			fn: func() {
				template := e2b.Template(nil).
					FromUbuntuImage("24.04").
					AptInstall([]string{"r-base"}, &struct{ User string }{User: "root"})

				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "r-runtime", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, `Rscript -e 'print("Hello, world!")'`, nil)
				result := execution.(*e2b.CommandResult)

				_ = template
				_ = result.Stdout
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 r doc snippet, got %d", got)
	}
}
