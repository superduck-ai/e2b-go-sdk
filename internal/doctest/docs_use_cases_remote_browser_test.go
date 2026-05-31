package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsUseCasesRemoteBrowserDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/use-cases/remote-browser.mdx"); err != nil {
		t.Fatalf("use-cases remote-browser doc is missing: %v", err)
	}
}

// This test keeps docs/use-cases/remote-browser.mdx aligned with the exported
// Go SDK workflow for combining sandbox orchestration with a cloud browser. The
// closures are compile-only examples and are intentionally never executed.
func TestDocsUseCasesRemoteBrowserExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "screenshot-app-endpoints",
			fn: func() {
				const fastAPIApp = `from fastapi import FastAPI
from fastapi.responses import HTMLResponse

app = FastAPI()

@app.get("/")
def home():
    return HTMLResponse("<h1>Home</h1><p>Welcome to the app.</p>")

@app.get("/about")
def about():
    return HTMLResponse("<h1>About</h1><p>About this app.</p>")

@app.get("/dashboard")
def dashboard():
    return HTMLResponse("<h1>Dashboard</h1><p>Your dashboard.</p>")
`

				ctx := context.Background()
				timeoutMs := 300_000

				sandbox, err := e2b.Create(ctx, "kernel-browser", &e2b.SandboxOpts{
					Envs: map[string]string{
						"KERNEL_API_KEY": "kernel-api-key",
					},
					TimeoutMs: &timeoutMs,
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				writeAppInfo, writeAppErr := sandbox.Files.Write(ctx, "/home/user/app.py", fastAPIApp, nil)

				runTimeoutMs := 60_000
				installExecution, installErr := sandbox.Commands.Run(ctx, "pip install fastapi uvicorn", &e2b.CommandStartOpts{
					TimeoutMs: &runTimeoutMs,
				})

				appExecution, appErr := sandbox.Commands.Run(ctx, "uvicorn app:app --host 0.0.0.0 --port 8000", &e2b.CommandStartOpts{
					Background: true,
					Cwd:        "/home/user",
				})
				appHandle := appExecution.(*e2b.CommandHandle)

				appURL := "https://" + sandbox.GetHost(8000)
				browseScript := `from kernel import Kernel
from playwright.sync_api import sync_playwright

app_url = "` + appURL + `"
routes = ["/", "/about", "/dashboard"]

kernel = Kernel()
kb = kernel.browsers.create()

with sync_playwright() as pw:
    browser = pw.chromium.connect_over_cdp(kb.cdp_ws_url)
    page = browser.new_page()

    for route in routes:
        page.goto(f"{app_url}{route}", wait_until="networkidle")
        name = "home" if route == "/" else route.strip("/")
        page.screenshot(path=f"/home/user/{name}.png")

    browser.close()
`

				writeScriptInfo, writeScriptErr := sandbox.Files.Write(ctx, "/home/user/browse.py", browseScript, nil)
				browseExecution, browseErr := sandbox.Commands.Run(ctx, "python3 /home/user/browse.py", &e2b.CommandStartOpts{
					TimeoutMs: &runTimeoutMs,
				})
				browseResult := browseExecution.(*e2b.CommandResult)

				_ = writeAppInfo
				_ = writeScriptInfo
				_ = installExecution
				_ = appHandle
				_ = browseResult
				_ = writeAppErr
				_ = installErr
				_ = appErr
				_ = writeScriptErr
				_ = browseErr
			},
		},
		{
			name: "agent-data-extraction",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 300_000
				runTimeoutMs := 180_000

				sandbox, err := e2b.Create(ctx, "kernel-browser", &e2b.SandboxOpts{
					Envs: map[string]string{
						"KERNEL_API_KEY":    "kernel-api-key",
						"ANTHROPIC_API_KEY": "anthropic-api-key",
					},
					TimeoutMs: &timeoutMs,
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				agentScript := `import asyncio
from kernel import Kernel
from browser_use import Agent, Browser, ChatAnthropic

async def main():
    kernel = Kernel()
    kb = kernel.browsers.create()
    browser = Browser(cdp_url=kb.cdp_ws_url)

    agent = Agent(
        task="Go to https://news.ycombinator.com and return the top 5 story titles as JSON",
        llm=ChatAnthropic(model="claude-sonnet-4"),
        browser=browser,
    )
    result = await agent.run()
    print(result)

asyncio.run(main())
`

				writeInfo, writeErr := sandbox.Files.Write(ctx, "/home/user/agent_task.py", agentScript, nil)
				execution, runErr := sandbox.Commands.Run(ctx, "python3 /home/user/agent_task.py", &e2b.CommandStartOpts{
					TimeoutMs: &runTimeoutMs,
				})
				result := execution.(*e2b.CommandResult)

				_ = writeInfo
				_ = result.Stdout
				_ = writeErr
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 use-cases remote-browser doc snippets, got %d", got)
	}
}
