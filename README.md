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
- `E2B_APPLE_CONTAINER_ENVD_BINARY`
- `E2B_APPLE_CONTAINER_BASE_IMAGE`

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

## Apple Container runtime

On Apple Silicon macOS, `e2b.Create` can create a local Apple Container sandbox directly through Apple Container XPC services instead of calling the E2B HTTP API or an `e2b-local` gateway.

Requirements:

- macOS with Apple Container installed and running
- `CGO_ENABLED=1`
- a locally pulled Linux image, for example `docker.io/library/debian:bookworm-slim`
- a Linux `envd` binary path in `RuntimeOptions.EnvdBinary` or `E2B_APPLE_CONTAINER_ENVD_BINARY`, unless every template sets `PrebakedEnvdPath`

Use `e2b.WithAppleContainerRuntime(...)` to create sandbox options configured for the native runtime, then fill any additional sandbox options on the returned struct.

```go
package main

import (
	"context"
	"log"

	e2b "github.com/superduck-ai/e2b-go-sdk"
	"github.com/superduck-ai/e2b-go-sdk/runtime/applecontainer"
)

func main() {
	ctx := context.Background()

	opts := e2b.WithAppleContainerRuntime(&applecontainer.RuntimeOptions{
		EnvdBinary: "/absolute/path/to/envd-linux-arm64",
		Templates: map[string]applecontainer.TemplateOptions{
			"base": {
				Image: "docker.io/library/debian:bookworm-slim",
			},
		},
	})
	sandbox, err := e2b.Create(ctx, "", opts)
	if err != nil {
		log.Fatal(err)
	}
	defer sandbox.Kill(context.Background(), nil)

	log.Println(sandbox.SandboxID)
}
```

If the image already includes `envd`, set `TemplateOptions.PrebakedEnvdPath` to the executable path inside the image. The runtime then skips copying `RuntimeOptions.EnvdBinary` on each cold create.

Capability notes:

| Capability | Native Apple Container runtime |
| ---------- | ------------------------------ |
| Create/connect/pause/kill/info/metrics | Supported |
| Commands, filesystem, PTY, Git | Supported through `envd` |
| Volume mounts | Supported for existing Apple Container volumes |
| Volume create/list/delete | Use `e2b-local` gateway or Apple Container CLI |
| Network update | Not supported |
| Snapshots | Not supported |

The runtime retries host-port allocation when Apple Container reports a port conflict, but Apple Container still requires explicit localhost published ports. This runtime is intended as an experimental local macOS backend, not as a drop-in replacement for the remote E2B control plane.

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
