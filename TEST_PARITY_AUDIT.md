# JS/Python to Go Test Parity Audit

Date: 2026-05-27

Scope:
- Compared current Go worktree against:
  - `/Users/arthur/WebstormProjects/e2b/E2B/packages/js-sdk/tests`
  - `/Users/arthur/WebstormProjects/e2b/E2B/packages/python-sdk/tests`
- Volume remains listed but excluded from the current non-volume migration scope.
- Python async tests are listed as source files, but Go has no async API surface; their behavior is only comparable through the sync/live behavior tests.

Legend:
- `covered`: Go has a unit or integration test for the same behavior.
- `partial`: Go has related coverage but one or more source assertions are absent or not proven.
- `env-skip`: Go has an integration test but the current `.env` backend/environment skips it.
- `n/a`: source file is a harness, language/runtime-specific test, or intentionally excluded.
- `gap`: no current Go equivalent found.

## Summary Differences

1. JS runtime tests have no Go equivalent: `runtimes/browser/run.test.tsx`, `runtimes/bun/run.test.ts`, `runtimes/deno/run.test.ts`.
2. JS stress/randomness integration coverage is now present in Go: randomness is a live test that skips when the template lacks `python3`/`numpy`; stress is an opt-in live test gated by `E2B_RUN_STRESS=1`.
3. Python-only connector retry tests have no Go equivalent: `e2b_connect/test_client.py`.
4. Python async files duplicate sync behavior, but async method/coroutine parity is not applicable to Go.
5. `template/utils/getCallerDirectory` has no Go equivalent; Go uses explicit `TemplateOptions.FileContextPath` and current working directory.
6. `template/utils/normalizeBuildArguments` semantics are aligned through Go's typed `Build` API: name wins over alias, legacy alias is normalized into the request `name`, empty/missing names raise `TemplateError`, and other build options are preserved without sending an `alias` field.
7. Template tags public overloads are now aligned: Go accepts a single string or `[]string` for assign/remove and covers mocked payload behavior. Live invalid tag-format/build assertions remain blocked by template build API availability in the current `.env`.
8. Template build/stacktrace/runCmd/makeSymlink behavior is migrated structurally, but current `.env` returns `404 page not found` for template build API, so real build failures/stack traces are not proven.
9. Sandbox connect timeout behavior is aligned: Go now covers "connect does not shorten timeout" and "connect extends timeout" against a real sandbox.
10. Sandbox lifecycle auto-pause/auto-resume is partial/env-limited: Go now covers auto-pause requiring connect; current `.env` returns 502 instead of waking a paused sandbox on HTTP request, so that subcase is present but skipped.
11. JS sandbox pause/resume snapshot resource-retention cases are aligned: Go now covers env var, file, ongoing process, completed process, and HTTP server after resume.
12. Filesystem watch is partial only for Python's iterator-style watch API, which has no direct Go API equivalent. Callback watch, recursive/error cases, and secured-envd watch are covered.
13. Network/public traffic are environment-limited: Go has payload tests and live tests, but current backend skips or cannot enforce outbound network, `allowInternetAccess=false`, `allowPublicTraffic=false` traffic token, and `maskRequestHost`.

## JS File Matrix

| JS file | Go evidence | Status | Difference |
|---|---|---:|---|
| `api/handleApiError.test.ts` | `api/client_test.go` | covered | None found. |
| `api/info.test.ts` | `live_integration_test.go:TestLiveSandboxLifecycle`, `sandbox_test.go` | covered | None found. |
| `api/kill.test.ts` | `live_integration_test.go:TestLiveSandboxLifecycle`, `sandbox_test.go` | covered | None found. |
| `api/list.test.ts` | `live_integration_test.go:TestLiveSandboxLifecycle`, `paginator_test.go` | partial | Pagination rows are present but `env-skip` when backend does not expose a token. |
| `api/snapshot.test.ts` | `live_integration_test.go:TestLiveSandboxLifecycle` | covered | Pause/resume covered. |
| `cmdHelper.ts` | none | n/a | Test helper only. |
| `connectionConfig.test.ts` | `connection_config_test.go` | covered | None found. |
| `integration/randomness.test.ts` | `live_integration_test.go:TestLiveRandomness` | env-skip | Go equivalent exists; current inferred template lacks `python3`/`numpy`. |
| `integration/stress.test.ts` | `live_integration_test.go:TestLiveStress` | env-skip | Go equivalent exists but is opt-in with `E2B_RUN_STRESS=1`; app-host stress also requires `E2B_STRESS_TEMPLATE`. |
| `integration/template/*` | none | n/a | JS integration fixtures. |
| `runtimes/browser/run.test.tsx` | none | n/a | Browser runtime-specific. |
| `runtimes/bun/run.test.ts` | none | n/a | Bun runtime-specific. |
| `runtimes/deno/run.test.ts` | none | n/a | Deno runtime-specific. |
| `sandbox/commands/connect.test.ts` | `live_integration_test.go:TestLiveCommands`, `commands/commands_test.go` | covered | None found. |
| `sandbox/commands/envVars.test.ts` | `live_integration_test.go:TestLiveCommandOptions` | covered | None found. |
| `sandbox/commands/kill.test.ts` | `live_integration_test.go:TestLiveCommands` | covered | None found. |
| `sandbox/commands/list.test.ts` | `live_integration_test.go:TestLiveCommands` | covered | None found. |
| `sandbox/commands/run.test.ts` | `live_integration_test.go:TestLiveCommands`, `commands/commands_test.go` | covered | JS does not include Python broken UTF-8 iterator case; Go covers broken UTF-8 output. |
| `sandbox/commands/sendStdin.test.ts` | `live_integration_test.go:TestLiveCommands`, `envd/process/process_test.go` | covered | None found. |
| `sandbox/configPropagation.test.ts` | `sandbox_test.go`, `connection_config_test.go` | covered | None found. |
| `sandbox/connect.test.ts` | `live_integration_test.go:TestLiveSandboxLifecycle`, `sandbox_test.go` | covered | Connect/resume/not-found and connect timeout no-shorten/extend covered. |
| `sandbox/create.test.ts` | `live_integration_test.go:TestLiveSandboxLifecycle`, `TestLiveSandboxLifecycleAutoPause`, `sandbox_test.go` | partial/env-skip | Start/metadata and auto-pause requiring connect covered; HTTP auto-resume is skipped in current env with 502. |
| `sandbox/files/contentEncoding.test.ts` | `live_integration_test.go:TestLiveFilesystem` | covered | None found. |
| `sandbox/files/exists.test.ts` | `live_integration_test.go:TestLiveFilesystem` | covered | None found. |
| `sandbox/files/info.test.ts` | `live_integration_test.go:TestLiveFilesystem`, `filesystem/filesystem_test.go` | covered | None found. |
| `sandbox/files/list.test.ts` | `live_integration_test.go:TestLiveFilesystem` | covered | None found. |
| `sandbox/files/makeDir.test.ts` | `live_integration_test.go:TestLiveFilesystem` | covered | None found. |
| `sandbox/files/read.test.ts` | `live_integration_test.go:TestLiveFilesystem`, `filesystem/filesystem_test.go` | covered | None found. |
| `sandbox/files/remove.test.ts` | `live_integration_test.go:TestLiveFilesystem`, `filesystem/filesystem_test.go` | covered | None found. |
| `sandbox/files/rename.test.ts` | `live_integration_test.go:TestLiveFilesystem`, `filesystem/filesystem_test.go` | covered | None found. |
| `sandbox/files/signing.test.ts` | `live_integration_test.go:TestLiveFileSigning`, `signature_test.go` | covered | None found. |
| `sandbox/files/watch.test.ts` | `live_integration_test.go:TestLiveFilesystem`, `filesystem/filesystem_test.go` | covered | Callback, recursive, and error cases covered. |
| `sandbox/files/write.test.ts` | `live_integration_test.go:TestLiveFilesystem`, `filesystem/filesystem_test.go` | covered | Main write/writeFiles cases and empty writeFiles no-op covered. |
| `sandbox/git/*.test.ts` | `live_integration_test.go:TestLiveGit`, `git/*_test.go` | covered | None found for non-volume scope. |
| `sandbox/host.test.ts` | `live_integration_test.go:TestLiveSandboxHost` | covered | None found. |
| `sandbox/internetAccess.test.ts` | `live_integration_test.go:TestLiveSandboxInternetAccess`, `sandbox_test.go` | env-skip | Current backend does not enforce `allowInternetAccess=false`. |
| `sandbox/kill.test.ts` | `live_integration_test.go:TestLiveSandboxLifecycle`, `sandbox_test.go` | covered | None found. |
| `sandbox/lifecyclePayload.test.ts` | `sandbox_test.go` | covered | Payload mapping covered. |
| `sandbox/metrics.test.ts` | `live_integration_test.go:TestLiveSandboxLifecycle`, `api/metrics_test.go` | env-skip | Metrics endpoint returned no points in current env. |
| `sandbox/network.test.ts` | `live_integration_test.go:TestLiveSandboxNetwork`, `sandbox_test.go`, `network_test.go` | partial/env-skip | Payload covered; outbound matrix is not fully proven and current env cannot reach allowed route. |
| `sandbox/pty/*.test.ts` | `live_integration_test.go:TestLivePty`, `commands/pty_test.go` | covered | None found. |
| `sandbox/secure.test.ts` | `live_integration_test.go:TestLiveFileSigning`, `sandbox_test.go`, `signature_test.go` | covered | Secure signing and connect-to-secure covered. |
| `sandbox/snapshot-api.test.ts` | `live_integration_test.go:TestLiveSnapshots`, `sandbox_test.go` | covered | Second delete currently logs idempotent backend behavior instead of requiring `false`. |
| `sandbox/snapshot.test.ts` | `live_integration_test.go:TestLiveSandboxLifecycle`, `TestLiveSandboxPauseResumeStateRetention` | covered | Basic pause/resume and JS env/file/process/http-server retention covered. |
| `sandbox/timeout.test.ts` | `live_integration_test.go:TestLiveSandboxLifecycle` | covered | Shorten, shorten-then-lengthen, and `endAt` covered. |
| `setup.ts`, `template.ts` | none | n/a | Test harness/default template setup. |
| `template/backgroundBuild.test.ts` | `live_integration_test.go:TestLiveTemplateBuildUploadAndTags` | env-skip | Current template build API returns 404. |
| `template/build.test.ts` | `live_integration_test.go:TestLiveTemplateBuildUploadAndTags` | env-skip | Build, symlink, resolve symlink, base template flows cannot be proven in current env. |
| `template/exists.test.ts` | `live_integration_test.go:TestLiveTemplateBuildUploadAndTags` | env-skip | Existing/non-existing template existence flow depends on build API/current template availability. |
| `template/methods/fromDockerfile.test.ts` | `template/template_alignment_test.go` | covered | None found after parser alignment. |
| `template/methods/makeSymlink.test.ts` | `template/template_alignment_test.go`, live build test | partial/env-skip | Instruction shape covered; real build behavior skipped by current API. |
| `template/methods/runCmd.test.ts` | `template/template_alignment_test.go`, live build test | partial/env-skip | Instruction shape/user covered; invalid build user error skipped by current API. |
| `template/methods/toDockerfile.test.ts` | `template/template_alignment_test.go` | covered | None found. |
| `template/stacktrace.test.ts` | `live_integration_test.go:TestLiveTemplateBuildStacktrace` | env-skip | Current build API unavailable; stacktrace parity unproven. |
| `template/tags.test.ts` | `template/template_alignment_test.go`, live build test | partial/env-skip | Single/multiple tag payloads covered; live invalid tag-format cases remain unproven because build API returns 404. |
| `template/uploadFile.test.ts` | `template/template_alignment_test.go` | covered | None found. |
| `template/utils/getAllFilesInPath.test.ts` | `template/template_utils_alignment_test.go` | covered | None found. |
| `template/utils/getCallerDirectory.test.ts` | none | n/a/gap | Go has no caller-directory helper; uses explicit file context. |
| `template/utils/normalizeBuildArguments.test.ts` | `template/template_alignment_test.go` | covered | Name/alias precedence, missing-name errors, and request-level alias removal/options preservation covered. |
| `template/utils/tarFileStream.test.ts` | `template/template_utils_alignment_test.go` | covered | None found. |
| `template/utils/validateRelativePath.test.ts` | `template/template_utils_alignment_test.go` | covered | None found. |
| `volume/*.test.ts` | `volume/*_test.go` | excluded | Prior scope said volume is not migrated now. |

## Python File Matrix

| Python files | Go evidence | Status | Difference |
|---|---|---:|---|
| `test_connection_config.py` | `connection_config_test.go` | covered | None found. |
| `test_volume_connection_config.py` | `volume/*_test.go` | excluded | Volume scope excluded. |
| `sync/api_sync/test_sbx_info.py` | `live_integration_test.go:TestLiveSandboxLifecycle` | covered | None found. |
| `sync/api_sync/test_sbx_kill.py` | `live_integration_test.go:TestLiveSandboxLifecycle` | covered | None found. |
| `sync/api_sync/test_sbx_list.py` | `live_integration_test.go:TestLiveSandboxLifecycle`, `paginator_test.go` | partial/env-skip | Pagination token unavailable in current env. |
| `sync/api_sync/test_sbx_snapshot.py` | `live_integration_test.go:TestLiveSandboxLifecycle` | covered | None found. |
| `sync/sandbox_sync/commands/*.py` | `live_integration_test.go:TestLiveCommands`, `TestLiveCommandOptions`, `commands/*_test.go` | covered | Python "too short timeout iterating" has no direct Go iterator API equivalent. |
| `sync/sandbox_sync/files/test_content_encoding.py` | `live_integration_test.go:TestLiveFilesystem` | covered | None found. |
| `sync/sandbox_sync/files/test_exists.py` | `live_integration_test.go:TestLiveFilesystem` | covered | None found. |
| `sync/sandbox_sync/files/test_files_list.py` | `live_integration_test.go:TestLiveFilesystem` | covered | None found. |
| `sync/sandbox_sync/files/test_info.py` | `live_integration_test.go:TestLiveFilesystem` | covered | None found. |
| `sync/sandbox_sync/files/test_make_dir.py` | `live_integration_test.go:TestLiveFilesystem` | covered | None found. |
| `sync/sandbox_sync/files/test_read.py` | `live_integration_test.go:TestLiveFilesystem` | covered | None found. |
| `sync/sandbox_sync/files/test_remove.py` | `live_integration_test.go:TestLiveFilesystem` | covered | None found. |
| `sync/sandbox_sync/files/test_rename.py` | `live_integration_test.go:TestLiveFilesystem` | covered | None found. |
| `sync/sandbox_sync/files/test_secured.py` | `live_integration_test.go:TestLiveFileSigning` | covered | None found. |
| `sync/sandbox_sync/files/test_watch.py` | `live_integration_test.go:TestLiveFilesystem`, `TestLiveFileSigning` | partial | Secured watch covered; iterator-style `get_new_events` has no direct Go API equivalent. |
| `sync/sandbox_sync/files/test_write.py` | `live_integration_test.go:TestLiveFilesystem`, `filesystem/filesystem_test.go` | covered | Main cases and empty writeFiles no-op covered. |
| `sync/sandbox_sync/pty/*.py` | `live_integration_test.go:TestLivePty`, `commands/pty_test.go` | covered | None found. |
| `sync/sandbox_sync/test_config_propagation.py` | `sandbox_test.go`, `connection_config_test.go` | covered | None found. |
| `sync/sandbox_sync/test_connect.py` | `live_integration_test.go:TestLiveSandboxLifecycle`, `TestLiveFileSigning` | covered | Connect, paused resume, non-running error, timeout no-shorten/extend, and secure connect covered. |
| `sync/sandbox_sync/test_create.py` | `live_integration_test.go:TestLiveSandboxLifecycle`, `TestLiveSandboxLifecycleAutoPause`, `sandbox_test.go` | partial/env-skip | Start/metadata/lifecycle payload and auto-pause covered; HTTP auto-resume skipped in current env with 502. |
| `sync/sandbox_sync/test_host.py` | `live_integration_test.go:TestLiveSandboxHost` | covered | None found. |
| `sync/sandbox_sync/test_internet_access.py` | `live_integration_test.go:TestLiveSandboxInternetAccess` | env-skip | Disabled internet is not enforced in current env. |
| `sync/sandbox_sync/test_kill.py` | `live_integration_test.go:TestLiveSandboxLifecycle` | covered | None found. |
| `sync/sandbox_sync/test_metrics.py` | `live_integration_test.go:TestLiveSandboxLifecycle`, `api/metrics_test.go` | env-skip | No metric points in current env. |
| `sync/sandbox_sync/test_network.py` | `live_integration_test.go:TestLiveSandboxNetwork`, `TestLiveSandboxPublicTraffic`, `sandbox_test.go` | partial/env-skip | Payload covered; backend currently skips outbound/token/mask assertions. |
| `sync/sandbox_sync/test_secure.py` | `live_integration_test.go:TestLiveFileSigning` | covered | Secure creation/signing and connect-to-secure covered. |
| `sync/sandbox_sync/test_snapshot.py` | `live_integration_test.go:TestLiveSandboxLifecycle` | covered | Python simple pause/resume covered; JS has extra resource-retention cases not covered. |
| `sync/sandbox_sync/test_snapshot_api.py` | `live_integration_test.go:TestLiveSnapshots` | covered | Backend currently treats second delete idempotently. |
| `sync/sandbox_sync/test_timeout.py` | `live_integration_test.go:TestLiveSandboxLifecycle` | covered | None found. |
| `sync/template_sync/methods/test_from_dockerfile.py` | `template/template_alignment_test.go` | covered | None found. |
| `sync/template_sync/methods/test_make_symlink.py` | live template build test | env-skip | Build API unavailable. |
| `sync/template_sync/methods/test_run_cmd.py` | live template build test | env-skip | Build API unavailable; invalid user error unproven. |
| `sync/template_sync/methods/test_to_dockerfile.py` | `template/template_alignment_test.go` | covered | None found. |
| `sync/template_sync/test_background_build.py` | live template build test | env-skip | Build API unavailable. |
| `sync/template_sync/test_build.py` | live template build test | env-skip | Build API unavailable. |
| `sync/template_sync/test_exists.py` | live template build test | env-skip | Build API unavailable/template availability dependent. |
| `sync/template_sync/test_stacktrace.py` | `TestLiveTemplateBuildStacktrace` | env-skip | Build API unavailable; stacktrace parity unproven. |
| `sync/template_sync/test_tags.py` | `template/template_alignment_test.go`, live template build test | partial/env-skip | Single/multiple tag overloads covered; live invalid tag-format cases remain unproven because build API returns 404. |
| `sync/template_sync/test_upload_file.py` | `template/template_alignment_test.go` | covered | None found. |
| `shared/git/*.py` | `live_integration_test.go:TestLiveGit`, `git/*_test.go` | covered | Python `test_parity.py` method-signature/coroutine checks are Python-specific. |
| `shared/template/utils/get_all_files_in_path.py` | `template/template_utils_alignment_test.go` | covered | None found. |
| `shared/template/utils/test_get_caller_directory.py` | none | n/a/gap | No Go equivalent. |
| `shared/template/utils/test_normalize_build_arguments.py` | `template/template_alignment_test.go` | covered | Name/alias precedence and missing/empty name errors covered. |
| `shared/template/utils/test_tar_file_stream.py` | `template/template_utils_alignment_test.go` | covered | None found. |
| `shared/template/utils/test_validate_relative_path.py` | `template/template_utils_alignment_test.go` | covered | None found. |
| `async/**` | corresponding sync/live rows above | n/a | Go has no async API; behavior duplicates sync tests but async surface parity is not applicable. |
| `bugs/test_envelope_decode.py` | `commands/commands_test.go`, `envd/process/process_test.go` | partial | Go covers envelope/event decode and invalid UTF-8; skipped Desktop/pyautogui reproduction is Python-specific and not mirrored. |
| `e2b_connect/test_client.py` | none | n/a/gap | Python connector retry helper has no Go equivalent. |
| `conftest.py`, `sync/template_sync/conftest.py`, `shared/git/conftest.py` | none | n/a | Test harness fixtures. |
| `sync/volume_sync/*.py`, `async/volume_async/*.py` | `volume/*_test.go` | excluded | Prior scope said volume is not migrated now. |

## Go-Only Extra Coverage

These Go tests have no direct JS/Python source file but support the parity work:
- `surface_audit_test.go`, `template_aliases_test.go`, `api_aliases_test.go`, `export_aliases_test.go`: public API surface alignment guards.
- `envd/*_test.go`: current envd JSON/envelope compatibility.
- `commands/*_test.go`, `filesystem/*_test.go`: Connect envelope, deadline/error mapping, old-envd compatibility, and invalid UTF-8 guards.
- `internal/shared/sdk.go` error `Unwrap` tests through public error assignability.
- `volume/*_test.go`: outside current scope but present.

## Verification Snapshot

Latest commands run successfully:

```bash
go test ./... -count=1
go vet ./...
go test -tags=integration -run 'TestLiveSandboxLifecycle/shorten' -count=1 -v .
go test -tags=integration -run 'TestLiveSandboxLifecycle/(connect does not shorten timeout|connect extends timeout)$' -count=1 -v .
go test -tags=integration -run 'TestLiveSandboxLifecycleAutoPause' -count=1 -v .
go test -tags=integration -run 'TestLiveSandboxPauseResumeStateRetention' -count=1 -v .
go test -tags=integration -run 'TestLiveFileSigning' -count=1 -v .
go test -tags=integration -run 'TestLiveRandomness' -count=1 -v .
go test -tags=integration -run 'TestLiveStress' -count=1 -v .
go test -tags=integration -run 'TestLiveSandboxPublicTraffic' -count=1 -v .
go test -tags=integration -run 'TestLiveSandboxNetwork' -count=1 -v .
```

Earlier full live run also passed with environment skips:

```bash
go test -tags=integration -run 'TestLive' -count=1 -v .
```

Current environment skips observed:
- sandbox list pagination token unavailable
- metrics endpoint returned no points
- outbound allowed network route unreachable
- `allowInternetAccess=false` not enforced
- template build API unavailable: `404 page not found`
- `allowPublicTraffic=false` does not return traffic access token
- `maskRequestHost` not enforced
- HTTP auto-resume on paused sandbox returned 502 and did not wake the sandbox
- randomness template lacks `python3`/`numpy`
- stress tests are opt-in and skipped unless `E2B_RUN_STRESS=1`

## Grouped Row Expansion

Grouped rows above were used only to keep the table readable. Their file-level expansion is:

### JS grouped rows

| Group row | Expanded files | Status |
|---|---|---:|
| `sandbox/git/*.test.ts` | `sandbox/git/add.test.ts`, `sandbox/git/branches.test.ts`, `sandbox/git/clone.test.ts`, `sandbox/git/commit.test.ts`, `sandbox/git/config.test.ts`, `sandbox/git/dangerouslyAuthenticate.test.ts`, `sandbox/git/init.test.ts`, `sandbox/git/remote.test.ts`, `sandbox/git/reset.test.ts`, `sandbox/git/restore.test.ts`, `sandbox/git/status.test.ts`, `sandbox/git/sync.test.ts` | covered |
| `sandbox/pty/*.test.ts` | `sandbox/pty/ptyConnect.test.ts`, `sandbox/pty/ptyCreate.test.ts`, `sandbox/pty/resize.test.ts`, `sandbox/pty/sendInput.test.ts` | covered |
| `volume/*.test.ts` | `volume/file.test.ts`, `volume/volume.test.ts` | excluded |

### Python grouped rows

| Group row | Expanded files | Status |
|---|---|---:|
| `sync/sandbox_sync/commands/*.py` | `test_cmd_connect.py`, `test_cmd_kill.py`, `test_cmd_list.py`, `test_env_vars.py`, `test_run.py`, `test_send_stdin.py` | covered, except iterator-style timeout has no direct Go API |
| `sync/sandbox_sync/pty/*.py` | `test_pty.py`, `test_pty_connect.py`, `test_resize.py`, `test_send_input.py` | covered |
| `shared/git/*.py` | `test_add.py`, `test_branches.py`, `test_clone.py`, `test_commit.py`, `test_config.py`, `test_dangerously_authenticate.py`, `test_init.py`, `test_remote.py`, `test_reset.py`, `test_restore.py`, `test_status.py`, `test_sync.py` | covered |
| `shared/git/test_parity.py` | `test_identical_method_signatures`, `test_async_methods_are_coroutines` | Python-specific/n/a |
| `async/api_async/*.py` | `test_sbx_info.py`, `test_sbx_kill.py`, `test_sbx_list.py`, `test_sbx_snapshot.py` | n/a, behavior mirrors sync API files |
| `async/sandbox_async/commands/*.py` | `test_cmd_connect.py`, `test_cmd_kill.py`, `test_cmd_list.py`, `test_env_vars.py`, `test_run.py`, `test_send_stdin.py` | n/a, behavior mirrors sync command files |
| `async/sandbox_async/files/*.py` | `test_content_encoding.py`, `test_exists.py`, `test_files_list.py`, `test_info.py`, `test_make_dir.py`, `test_read.py`, `test_remove.py`, `test_rename.py`, `test_secured.py`, `test_watch.py`, `test_write.py` | n/a, behavior mirrors sync filesystem files |
| `async/sandbox_async/pty/*.py` | `test_pty_connect.py`, `test_pty_create.py`, `test_resize.py`, `test_send_input.py` | n/a, behavior mirrors sync PTY files |
| `async/sandbox_async/*.py` | `test_config_propagation.py`, `test_connect.py`, `test_create.py`, `test_host.py`, `test_internet_access.py`, `test_kill.py`, `test_metrics.py`, `test_network.py`, `test_secure.py`, `test_snapshot.py`, `test_snapshot_api.py`, `test_timeout.py` | n/a, behavior mirrors sync sandbox files |
| `async/template_async/methods/*.py` | `test_from_dockerfile.py`, `test_make_symlink.py`, `test_run_cmd.py`, `test_to_dockerfile.py` | n/a, behavior mirrors sync template method files |
| `async/template_async/*.py` | `conftest.py`, `test_background_build.py`, `test_build.py`, `test_exists.py`, `test_stacktrace.py`, `test_tags.py`, `test_upload_file.py` | n/a, behavior mirrors sync template files |
| `sync/volume_sync/*.py` | `test_file.py`, `test_volume.py` | excluded |
| `async/volume_async/*.py` | `test_file.py`, `test_volume.py` | excluded |

## Full One-File Index

This section is the authoritative no-wildcard index. It covers every file under the JS test tree and every Python file under the Python test tree.

### JS full index: 81 files

| # | JS file | Status | Go evidence / difference |
|---:|---|---:|---|
| 1 | `api/handleApiError.test.ts` | covered | `api/client_test.go`; aligned status/body mapping. |
| 2 | `api/info.test.ts` | covered | `live_integration_test.go:TestLiveSandboxLifecycle`; `sandbox_test.go`. |
| 3 | `api/kill.test.ts` | covered | `live_integration_test.go:TestLiveSandboxLifecycle`; `sandbox_test.go`. |
| 4 | `api/list.test.ts` | partial/env-skip | `live_integration_test.go:TestLiveSandboxLifecycle`; pagination token unavailable in current env. |
| 5 | `api/snapshot.test.ts` | covered | `live_integration_test.go:TestLiveSandboxLifecycle`. |
| 6 | `cmdHelper.ts` | n/a | Test helper only. |
| 7 | `connectionConfig.test.ts` | covered | `connection_config_test.go`. |
| 8 | `integration/randomness.test.ts` | env-skip | `live_integration_test.go:TestLiveRandomness`; current inferred template lacks `python3`/`numpy`. |
| 9 | `integration/stress.test.ts` | env-skip | `live_integration_test.go:TestLiveStress`; opt-in with `E2B_RUN_STRESS=1`, app-host stress requires `E2B_STRESS_TEMPLATE`. |
| 10 | `integration/template/README.md` | n/a | JS integration fixture. |
| 11 | `integration/template/e2b.Dockerfile` | n/a | JS integration fixture. |
| 12 | `integration/template/e2b.toml` | n/a | JS integration fixture. |
| 13 | `runtimes/browser/run.test.tsx` | n/a | Browser runtime-specific. |
| 14 | `runtimes/bun/run.test.ts` | n/a | Bun runtime-specific. |
| 15 | `runtimes/deno/run.test.ts` | n/a | Deno runtime-specific. |
| 16 | `sandbox/commands/connect.test.ts` | covered | `live_integration_test.go:TestLiveCommands`; `commands/commands_test.go`. |
| 17 | `sandbox/commands/envVars.test.ts` | covered | `live_integration_test.go:TestLiveCommandOptions`. |
| 18 | `sandbox/commands/kill.test.ts` | covered | `live_integration_test.go:TestLiveCommands`. |
| 19 | `sandbox/commands/list.test.ts` | covered | `live_integration_test.go:TestLiveCommands`. |
| 20 | `sandbox/commands/run.test.ts` | covered | `live_integration_test.go:TestLiveCommands`; `commands/commands_test.go`. |
| 21 | `sandbox/commands/sendStdin.test.ts` | covered | `live_integration_test.go:TestLiveCommands`; `envd/process/process_test.go`. |
| 22 | `sandbox/configPropagation.test.ts` | covered | `sandbox_test.go`; `connection_config_test.go`. |
| 23 | `sandbox/connect.test.ts` | covered | Basic connect/resume/not-found and connect timeout no-shorten/extend covered. |
| 24 | `sandbox/create.test.ts` | partial/env-skip | Start/metadata/payload and auto-pause requiring connect covered; HTTP auto-resume skipped in current env with 502. |
| 25 | `sandbox/files/contentEncoding.test.ts` | covered | `live_integration_test.go:TestLiveFilesystem`. |
| 26 | `sandbox/files/exists.test.ts` | covered | `live_integration_test.go:TestLiveFilesystem`. |
| 27 | `sandbox/files/info.test.ts` | covered | `live_integration_test.go:TestLiveFilesystem`; `filesystem/filesystem_test.go`. |
| 28 | `sandbox/files/list.test.ts` | covered | `live_integration_test.go:TestLiveFilesystem`. |
| 29 | `sandbox/files/makeDir.test.ts` | covered | `live_integration_test.go:TestLiveFilesystem`. |
| 30 | `sandbox/files/read.test.ts` | covered | `live_integration_test.go:TestLiveFilesystem`; `filesystem/filesystem_test.go`. |
| 31 | `sandbox/files/remove.test.ts` | covered | `live_integration_test.go:TestLiveFilesystem`; `filesystem/filesystem_test.go`. |
| 32 | `sandbox/files/rename.test.ts` | covered | `live_integration_test.go:TestLiveFilesystem`; `filesystem/filesystem_test.go`. |
| 33 | `sandbox/files/signing.test.ts` | covered | `live_integration_test.go:TestLiveFileSigning`; `signature_test.go`. |
| 34 | `sandbox/files/watch.test.ts` | covered | Callback/recursive/error cases covered. |
| 35 | `sandbox/files/write.test.ts` | covered | Main write/writeFiles cases and empty writeFiles no-op covered. |
| 36 | `sandbox/git/add.test.ts` | covered | `live_integration_test.go:TestLiveGit`. |
| 37 | `sandbox/git/branches.test.ts` | covered | `live_integration_test.go:TestLiveGit`; `git/utils_test.go`. |
| 38 | `sandbox/git/clone.test.ts` | covered | `live_integration_test.go:TestLiveGit`. |
| 39 | `sandbox/git/commit.test.ts` | covered | `live_integration_test.go:TestLiveGit`; `git/git_test.go`. |
| 40 | `sandbox/git/config.test.ts` | covered | `live_integration_test.go:TestLiveGit`; `git/git_test.go`. |
| 41 | `sandbox/git/dangerouslyAuthenticate.test.ts` | covered | `live_integration_test.go:TestLiveGit`; `git/git_test.go`. |
| 42 | `sandbox/git/helpers.ts` | n/a | Test helper only. |
| 43 | `sandbox/git/init.test.ts` | covered | `live_integration_test.go:TestLiveGit`; `git/git_test.go`. |
| 44 | `sandbox/git/remote.test.ts` | covered | `live_integration_test.go:TestLiveGit`; `git/git_test.go`. |
| 45 | `sandbox/git/reset.test.ts` | covered | `live_integration_test.go:TestLiveGit`. |
| 46 | `sandbox/git/restore.test.ts` | covered | `live_integration_test.go:TestLiveGit`. |
| 47 | `sandbox/git/status.test.ts` | covered | `live_integration_test.go:TestLiveGit`; `git/utils_test.go`. |
| 48 | `sandbox/git/sync.test.ts` | covered | `live_integration_test.go:TestLiveGit`; `git/git_test.go`. |
| 49 | `sandbox/host.test.ts` | covered | `live_integration_test.go:TestLiveSandboxHost`. |
| 50 | `sandbox/internetAccess.test.ts` | env-skip | Go test exists; current env does not enforce disabled internet. |
| 51 | `sandbox/kill.test.ts` | covered | `live_integration_test.go:TestLiveSandboxLifecycle`; `sandbox_test.go`. |
| 52 | `sandbox/lifecyclePayload.test.ts` | covered | `sandbox_test.go` payload tests. |
| 53 | `sandbox/metrics.test.ts` | env-skip | Go test exists; current env returned no metric points. |
| 54 | `sandbox/network.test.ts` | partial/env-skip | Payload covered; outbound/token/mask assertions limited by current env. |
| 55 | `sandbox/pty/ptyConnect.test.ts` | covered | `live_integration_test.go:TestLivePty`; `commands/pty_test.go`. |
| 56 | `sandbox/pty/ptyCreate.test.ts` | covered | `live_integration_test.go:TestLivePty`; `commands/pty_test.go`. |
| 57 | `sandbox/pty/resize.test.ts` | covered | `live_integration_test.go:TestLivePty`. |
| 58 | `sandbox/pty/sendInput.test.ts` | covered | `live_integration_test.go:TestLivePty`. |
| 59 | `sandbox/secure.test.ts` | covered | Signing and connect-to-secure covered. |
| 60 | `sandbox/snapshot-api.test.ts` | covered | `live_integration_test.go:TestLiveSnapshots`; second delete differs due current idempotent backend. |
| 61 | `sandbox/snapshot.test.ts` | covered | Basic pause/resume and env/file/process/http-server retention covered. |
| 62 | `sandbox/timeout.test.ts` | covered | `live_integration_test.go:TestLiveSandboxLifecycle`. |
| 63 | `setup.ts` | n/a | Test harness. |
| 64 | `template.ts` | n/a | Test harness/default template. |
| 65 | `template/backgroundBuild.test.ts` | env-skip | Go live test exists; template build API returns 404 in current env. |
| 66 | `template/build.test.ts` | env-skip | Go live test exists; template build API returns 404 in current env. |
| 67 | `template/exists.test.ts` | env-skip | Go live test exists; template build/template availability dependent. |
| 68 | `template/methods/fromDockerfile.test.ts` | covered | `template/template_alignment_test.go`. |
| 69 | `template/methods/makeSymlink.test.ts` | partial/env-skip | Instruction shape covered; real build behavior skipped. |
| 70 | `template/methods/runCmd.test.ts` | partial/env-skip | Instruction shape/user covered; invalid build user error skipped. |
| 71 | `template/methods/toDockerfile.test.ts` | covered | `template/template_alignment_test.go`. |
| 72 | `template/stacktrace.test.ts` | env-skip | Go test exists; build API unavailable. |
| 73 | `template/tags.test.ts` | partial/env-skip | Single/multiple tag payloads covered; live invalid tag-format tests remain unproven because build API returns 404. |
| 74 | `template/uploadFile.test.ts` | covered | `template/template_alignment_test.go`. |
| 75 | `template/utils/getAllFilesInPath.test.ts` | covered | `template/template_utils_alignment_test.go`. |
| 76 | `template/utils/getCallerDirectory.test.ts` | n/a/gap | No Go equivalent helper. |
| 77 | `template/utils/normalizeBuildArguments.test.ts` | covered | Name/alias precedence, missing-name errors, and request-level alias removal/options preservation covered. |
| 78 | `template/utils/tarFileStream.test.ts` | covered | `template/template_utils_alignment_test.go`. |
| 79 | `template/utils/validateRelativePath.test.ts` | covered | `template/template_utils_alignment_test.go`. |
| 80 | `volume/file.test.ts` | excluded | Volume migration excluded by prior scope. |
| 81 | `volume/volume.test.ts` | excluded | Volume migration excluded by prior scope. |

### Python full index: 125 files

| # | Python file | Status | Go evidence / difference |
|---:|---|---:|---|
| 1 | `async/api_async/test_sbx_info.py` | n/a | Async duplicate of sync API info behavior. |
| 2 | `async/api_async/test_sbx_kill.py` | n/a | Async duplicate of sync API kill behavior. |
| 3 | `async/api_async/test_sbx_list.py` | n/a | Async duplicate; sync pagination remains env-skip. |
| 4 | `async/api_async/test_sbx_snapshot.py` | n/a | Async duplicate of sync pause/resume API behavior. |
| 5 | `async/sandbox_async/commands/test_cmd_connect.py` | n/a | Async duplicate of sync command connect behavior. |
| 6 | `async/sandbox_async/commands/test_cmd_kill.py` | n/a | Async duplicate of sync command kill behavior. |
| 7 | `async/sandbox_async/commands/test_cmd_list.py` | n/a | Async duplicate of sync command list behavior. |
| 8 | `async/sandbox_async/commands/test_env_vars.py` | n/a | Async duplicate of sync env-var behavior. |
| 9 | `async/sandbox_async/commands/test_run.py` | n/a | Async duplicate of sync run behavior. |
| 10 | `async/sandbox_async/commands/test_send_stdin.py` | n/a | Async duplicate of sync stdin behavior. |
| 11 | `async/sandbox_async/files/test_content_encoding.py` | n/a | Async duplicate of sync filesystem gzip behavior. |
| 12 | `async/sandbox_async/files/test_exists.py` | n/a | Async duplicate of sync exists behavior. |
| 13 | `async/sandbox_async/files/test_files_list.py` | n/a | Async duplicate of sync list behavior. |
| 14 | `async/sandbox_async/files/test_info.py` | n/a | Async duplicate of sync info behavior. |
| 15 | `async/sandbox_async/files/test_make_dir.py` | n/a | Async duplicate of sync mkdir behavior. |
| 16 | `async/sandbox_async/files/test_read.py` | n/a | Async duplicate of sync read behavior. |
| 17 | `async/sandbox_async/files/test_remove.py` | n/a | Async duplicate of sync remove behavior. |
| 18 | `async/sandbox_async/files/test_rename.py` | n/a | Async duplicate of sync rename behavior. |
| 19 | `async/sandbox_async/files/test_secured.py` | n/a | Async duplicate of sync signed URL behavior. |
| 20 | `async/sandbox_async/files/test_watch.py` | n/a | Async duplicate; secured watch covered through sync/live, iterator-style API has no direct Go equivalent. |
| 21 | `async/sandbox_async/files/test_write.py` | n/a | Async duplicate of sync write behavior. |
| 22 | `async/sandbox_async/pty/test_pty_connect.py` | n/a | Async duplicate of sync PTY connect behavior. |
| 23 | `async/sandbox_async/pty/test_pty_create.py` | n/a | Async duplicate of sync PTY create behavior. |
| 24 | `async/sandbox_async/pty/test_resize.py` | n/a | Async duplicate of sync PTY resize behavior. |
| 25 | `async/sandbox_async/pty/test_send_input.py` | n/a | Async duplicate of sync PTY input behavior. |
| 26 | `async/sandbox_async/test_config_propagation.py` | n/a | Async duplicate of sync config propagation behavior. |
| 27 | `async/sandbox_async/test_connect.py` | n/a | Async duplicate; sync connect behavior is covered. |
| 28 | `async/sandbox_async/test_create.py` | n/a | Async duplicate; HTTP auto-resume env-skip mirrors sync. |
| 29 | `async/sandbox_async/test_host.py` | n/a | Async duplicate of sync host behavior. |
| 30 | `async/sandbox_async/test_internet_access.py` | n/a | Async duplicate; current env cannot enforce disabled internet. |
| 31 | `async/sandbox_async/test_kill.py` | n/a | Async duplicate of sync kill behavior. |
| 32 | `async/sandbox_async/test_metrics.py` | n/a | Async duplicate; current env has no metric points. |
| 33 | `async/sandbox_async/test_network.py` | n/a | Async duplicate; network env-skip mirrors sync. |
| 34 | `async/sandbox_async/test_secure.py` | n/a | Async duplicate; sync secure behavior is covered. |
| 35 | `async/sandbox_async/test_snapshot_api.py` | n/a | Async duplicate of sync snapshot API behavior. |
| 36 | `async/sandbox_async/test_snapshot.py` | n/a | Async duplicate of sync simple pause/resume behavior. |
| 37 | `async/sandbox_async/test_timeout.py` | n/a | Async duplicate of sync timeout behavior. |
| 38 | `async/template_async/conftest.py` | n/a | Test harness. |
| 39 | `async/template_async/methods/test_from_dockerfile.py` | n/a | Async duplicate; Go local fromDockerfile covered. |
| 40 | `async/template_async/methods/test_make_symlink.py` | n/a | Async duplicate; real build env-skip mirrors sync. |
| 41 | `async/template_async/methods/test_run_cmd.py` | n/a | Async duplicate; real build env-skip mirrors sync. |
| 42 | `async/template_async/methods/test_to_dockerfile.py` | n/a | Async duplicate; Go local toDockerfile covered. |
| 43 | `async/template_async/test_background_build.py` | n/a | Async duplicate; build API env-skip mirrors sync. |
| 44 | `async/template_async/test_build.py` | n/a | Async duplicate; build API env-skip mirrors sync. |
| 45 | `async/template_async/test_exists.py` | n/a | Async duplicate; build/template availability env-skip mirrors sync. |
| 46 | `async/template_async/test_stacktrace.py` | n/a | Async duplicate; build API env-skip mirrors sync. |
| 47 | `async/template_async/test_tags.py` | n/a | Async duplicate; tag overload covered, invalid tag build behavior remains env-skip. |
| 48 | `async/template_async/test_upload_file.py` | n/a | Async duplicate; Go upload regression covered. |
| 49 | `async/volume_async/test_file.py` | excluded | Volume migration excluded by prior scope. |
| 50 | `async/volume_async/test_volume.py` | excluded | Volume migration excluded by prior scope. |
| 51 | `bugs/test_envelope_decode.py` | partial | Go covers envelope/event decode and invalid UTF-8; skipped Desktop reproduction not mirrored. |
| 52 | `conftest.py` | n/a | Test harness. |
| 53 | `e2b_connect/__init__.py` | n/a | Package marker. |
| 54 | `e2b_connect/test_client.py` | gap | Python connector retry helper has no Go equivalent. |
| 55 | `shared/git/conftest.py` | n/a | Test harness. |
| 56 | `shared/git/test_add.py` | covered | `live_integration_test.go:TestLiveGit`. |
| 57 | `shared/git/test_branches.py` | covered | `live_integration_test.go:TestLiveGit`; `git/utils_test.go`. |
| 58 | `shared/git/test_clone.py` | covered | `live_integration_test.go:TestLiveGit`. |
| 59 | `shared/git/test_commit.py` | covered | `live_integration_test.go:TestLiveGit`; `git/git_test.go`. |
| 60 | `shared/git/test_config.py` | covered | `live_integration_test.go:TestLiveGit`; `git/git_test.go`. |
| 61 | `shared/git/test_dangerously_authenticate.py` | covered | `live_integration_test.go:TestLiveGit`; `git/git_test.go`. |
| 62 | `shared/git/test_init.py` | covered | `live_integration_test.go:TestLiveGit`; `git/git_test.go`. |
| 63 | `shared/git/test_parity.py` | n/a | Python method-signature/coroutine parity only. |
| 64 | `shared/git/test_remote.py` | covered | `live_integration_test.go:TestLiveGit`; `git/git_test.go`. |
| 65 | `shared/git/test_reset.py` | covered | `live_integration_test.go:TestLiveGit`. |
| 66 | `shared/git/test_restore.py` | covered | `live_integration_test.go:TestLiveGit`. |
| 67 | `shared/git/test_status.py` | covered | `live_integration_test.go:TestLiveGit`; `git/utils_test.go`. |
| 68 | `shared/git/test_sync.py` | covered | `live_integration_test.go:TestLiveGit`; `git/git_test.go`. |
| 69 | `shared/template/utils/get_all_files_in_path.py` | covered | `template/template_utils_alignment_test.go`. |
| 70 | `shared/template/utils/test_get_caller_directory.py` | n/a/gap | No Go equivalent helper. |
| 71 | `shared/template/utils/test_normalize_build_arguments.py` | covered | Name/alias precedence and missing/empty name errors covered. |
| 72 | `shared/template/utils/test_tar_file_stream.py` | covered | `template/template_utils_alignment_test.go`. |
| 73 | `shared/template/utils/test_validate_relative_path.py` | covered | `template/template_utils_alignment_test.go`. |
| 74 | `sync/api_sync/test_sbx_info.py` | covered | `live_integration_test.go:TestLiveSandboxLifecycle`. |
| 75 | `sync/api_sync/test_sbx_kill.py` | covered | `live_integration_test.go:TestLiveSandboxLifecycle`; `sandbox_test.go`. |
| 76 | `sync/api_sync/test_sbx_list.py` | partial/env-skip | Go pagination test exists; token unavailable in current env. |
| 77 | `sync/api_sync/test_sbx_snapshot.py` | covered | `live_integration_test.go:TestLiveSandboxLifecycle`. |
| 78 | `sync/sandbox_sync/commands/test_cmd_connect.py` | covered | `live_integration_test.go:TestLiveCommands`; `commands/commands_test.go`. |
| 79 | `sync/sandbox_sync/commands/test_cmd_kill.py` | covered | `live_integration_test.go:TestLiveCommands`. |
| 80 | `sync/sandbox_sync/commands/test_cmd_list.py` | covered | `live_integration_test.go:TestLiveCommands`. |
| 81 | `sync/sandbox_sync/commands/test_env_vars.py` | covered | `live_integration_test.go:TestLiveCommandOptions`. |
| 82 | `sync/sandbox_sync/commands/test_run.py` | covered | `live_integration_test.go:TestLiveCommands`; iterator-style timeout has no direct Go API. |
| 83 | `sync/sandbox_sync/commands/test_send_stdin.py` | covered | `live_integration_test.go:TestLiveCommands`; empty stdin marshal covered. |
| 84 | `sync/sandbox_sync/files/test_content_encoding.py` | covered | `live_integration_test.go:TestLiveFilesystem`. |
| 85 | `sync/sandbox_sync/files/test_exists.py` | covered | `live_integration_test.go:TestLiveFilesystem`. |
| 86 | `sync/sandbox_sync/files/test_files_list.py` | covered | `live_integration_test.go:TestLiveFilesystem`. |
| 87 | `sync/sandbox_sync/files/test_info.py` | covered | `live_integration_test.go:TestLiveFilesystem`; `filesystem/filesystem_test.go`. |
| 88 | `sync/sandbox_sync/files/test_make_dir.py` | covered | `live_integration_test.go:TestLiveFilesystem`. |
| 89 | `sync/sandbox_sync/files/test_read.py` | covered | `live_integration_test.go:TestLiveFilesystem`; `filesystem/filesystem_test.go`. |
| 90 | `sync/sandbox_sync/files/test_remove.py` | covered | `live_integration_test.go:TestLiveFilesystem`; `filesystem/filesystem_test.go`. |
| 91 | `sync/sandbox_sync/files/test_rename.py` | covered | `live_integration_test.go:TestLiveFilesystem`; `filesystem/filesystem_test.go`. |
| 92 | `sync/sandbox_sync/files/test_secured.py` | covered | `live_integration_test.go:TestLiveFileSigning`; `signature_test.go`. |
| 93 | `sync/sandbox_sync/files/test_watch.py` | partial | Callback/recursive/error and secured-envd watch covered; iterator-style API has no direct Go equivalent. |
| 94 | `sync/sandbox_sync/files/test_write.py` | covered | Main write cases and empty writeFiles no-op covered. |
| 95 | `sync/sandbox_sync/pty/test_pty_connect.py` | covered | `live_integration_test.go:TestLivePty`. |
| 96 | `sync/sandbox_sync/pty/test_pty.py` | covered | `live_integration_test.go:TestLivePty`. |
| 97 | `sync/sandbox_sync/pty/test_resize.py` | covered | `live_integration_test.go:TestLivePty`. |
| 98 | `sync/sandbox_sync/pty/test_send_input.py` | covered | `live_integration_test.go:TestLivePty`. |
| 99 | `sync/sandbox_sync/test_config_propagation.py` | covered | `sandbox_test.go`; `connection_config_test.go`. |
| 100 | `sync/sandbox_sync/test_connect.py` | covered | Basic connect, timeout extend/no-shorten, paused resume, non-running error, and secure connect covered. |
| 101 | `sync/sandbox_sync/test_create.py` | partial/env-skip | Start/metadata/lifecycle payload and auto-pause covered; HTTP auto-resume skipped in current env with 502. |
| 102 | `sync/sandbox_sync/test_host.py` | covered | `live_integration_test.go:TestLiveSandboxHost`. |
| 103 | `sync/sandbox_sync/test_internet_access.py` | env-skip | Current env does not enforce disabled internet. |
| 104 | `sync/sandbox_sync/test_kill.py` | covered | `live_integration_test.go:TestLiveSandboxLifecycle`. |
| 105 | `sync/sandbox_sync/test_metrics.py` | env-skip | Current env returned no metric points. |
| 106 | `sync/sandbox_sync/test_network.py` | partial/env-skip | Payload covered; outbound/token/mask assertions limited by current env. |
| 107 | `sync/sandbox_sync/test_secure.py` | covered | Secure signing/creation and connect-to-secure covered. |
| 108 | `sync/sandbox_sync/test_snapshot_api.py` | covered | `live_integration_test.go:TestLiveSnapshots`; second delete backend behavior differs. |
| 109 | `sync/sandbox_sync/test_snapshot.py` | covered | Python simple pause/resume covered. |
| 110 | `sync/sandbox_sync/test_timeout.py` | covered | `live_integration_test.go:TestLiveSandboxLifecycle`. |
| 111 | `sync/template_sync/conftest.py` | n/a | Test harness. |
| 112 | `sync/template_sync/methods/test_from_dockerfile.py` | covered | `template/template_alignment_test.go`. |
| 113 | `sync/template_sync/methods/test_make_symlink.py` | env-skip | Real build behavior skipped because build API unavailable. |
| 114 | `sync/template_sync/methods/test_run_cmd.py` | env-skip | Real build behavior and invalid user error skipped because build API unavailable. |
| 115 | `sync/template_sync/methods/test_to_dockerfile.py` | covered | `template/template_alignment_test.go`. |
| 116 | `sync/template_sync/test_background_build.py` | env-skip | Build API unavailable. |
| 117 | `sync/template_sync/test_build.py` | env-skip | Build API unavailable. |
| 118 | `sync/template_sync/test_exists.py` | env-skip | Build API/template availability dependent. |
| 119 | `sync/template_sync/test_stacktrace.py` | env-skip | Build API unavailable; stacktrace parity unproven. |
| 120 | `sync/template_sync/test_tags.py` | partial/env-skip | Single/multiple tag overloads covered; live invalid tag-format tests remain unproven because build API returns 404. |
| 121 | `sync/template_sync/test_upload_file.py` | covered | `template/template_alignment_test.go`. |
| 122 | `sync/volume_sync/test_file.py` | excluded | Volume migration excluded by prior scope. |
| 123 | `sync/volume_sync/test_volume.py` | excluded | Volume migration excluded by prior scope. |
| 124 | `test_connection_config.py` | covered | `connection_config_test.go`. |
| 125 | `test_volume_connection_config.py` | excluded | Volume migration excluded by prior scope. |
