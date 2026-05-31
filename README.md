# e2b-go-sdk

Go SDK for creating and controlling [E2B](./docs.mdx) sandboxes from Go.

This repository provides the root `e2b` package plus lower-level packages for commands, filesystem access, Git, templates, volumes, and API clients. It also contains the documentation source under [`docs/`](./docs/) and compile-checked doc examples in [`internal/doctest/`](./internal/doctest/).

## What you can do

- Create or connect to Linux sandboxes
- Run foreground or background commands
- Read, write, upload, download, and watch files
- Open PTY sessions
- Use Git inside a sandbox
- Create and mount persistent volumes
- Build reusable E2B templates from Go

## Requirements

- Go `1.22+`
- An E2B API key in `E2B_API_KEY`

Optional environment variables:

- `E2B_ACCESS_TOKEN`
- `E2B_DOMAIN`
- `E2B_API_URL`
- `E2B_SANDBOX_URL`
- `E2B_DEBUG`

## Install

```bash
go get github.com/superduck-ai/e2b-go-sdk
```

## Quickstart

Set your API key first:

```bash
export E2B_API_KEY=e2b_***
```

Create a sandbox, run a command, and inspect the filesystem:

```go
package main

import (
	"context"
	"log"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func main() {
	ctx := context.Background()

	sandbox, err := e2b.Create(ctx, "", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer sandbox.Kill(context.Background(), nil)

	execution, err := sandbox.Commands.Run(ctx, `python3 -c "print('hello world')"`, nil)
	if err != nil {
		log.Fatal(err)
	}
	result := execution.(*e2b.CommandResult)

	entries, err := sandbox.Files.List(ctx, "/", nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(result.Stdout)
	log.Println(entries)
}
```

## Main entry points

- `e2b.Create(ctx, template, opts)` creates a sandbox
- `e2b.Connect(ctx, sandboxID, opts)` reconnects to an existing sandbox
- `sandbox.Commands.Run(ctx, cmd, opts)` runs commands
- `sandbox.Files.Read/Write/List(...)` works with files inside the sandbox
- `sandbox.Pty.Create(ctx, opts)` starts an interactive PTY session
- `sandbox.Git` exposes Git operations inside the sandbox
- `e2b.CreateVolume(...)` and related helpers manage persistent volumes
- `e2b.Template(...)` and `e2b.Build(...)` define and build templates

The root package re-exports most commonly used command, filesystem, Git, template, and volume types so application code can stay in the `e2b` package for the common path.

## Development

Run the default test suite:

```bash
go test ./...
```

Run the documentation validation suite only:

```bash
go test ./internal/doctest -run '^TestDocs' -count=1
```

Run integration tests against a live E2B account:

```bash
E2B_API_KEY=e2b_*** go test -tags=integration ./...
```

Some expensive stress cases are skipped unless you opt in with additional environment variables such as `E2B_RUN_STRESS=1`.

## Repository layout

- [`sandbox.go`](./sandbox.go): sandbox lifecycle and runtime entry points
- [`commands/`](./commands/): command execution and PTY support
- [`filesystem/`](./filesystem/): sandbox filesystem APIs
- [`git/`](./git/): Git helpers for sandbox workflows
- [`template/`](./template/): template authoring and build APIs
- [`volume/`](./volume/): persistent volume APIs
- [`api/`](./api/): lower-level control plane client
- [`docs/`](./docs/): documentation pages
- [`internal/doctest/`](./internal/doctest/): compile-checked documentation examples plus link/asset/snippet audits

## Documentation

The repository documentation lives in:

- [`docs.mdx`](./docs.mdx)
- [`docs/quickstart.mdx`](./docs/quickstart.mdx)
- [`docs/sdk-reference/go-sdk/sandbox.mdx`](./docs/sdk-reference/go-sdk/sandbox.mdx)

If you change public behavior, update the corresponding doc page and matching coverage in [`internal/doctest/`](./internal/doctest/) in the same change.
