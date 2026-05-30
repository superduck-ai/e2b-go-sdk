# JS/Python to Go Test Parity Audit

Date: 2026-05-30

Scope:
- Compared current Go worktree against:
  - `/data/E2B/packages/js-sdk/tests`
  - `/data/E2B/packages/python-sdk/tests`
- Verified current behavior with the active `.env` in `/data/e2b-go-sdk`.
- Included `volume` in the audit. The previous document excluded it; that is no longer accepted as the comparison scope.

Legend:
- `covered`: Go has equivalent unit or integration coverage for the behavior.
- `partial`: Go has related coverage, but one or more source assertions are still not directly proven.
- `env-skip`: Go has a matching test path, but the current backend/template/environment prevents proving the behavior.
- `n/a`: language/runtime-specific or intentionally not applicable to Go.
- `gap`: no current Go equivalent found.

## Current Conclusions

1. Go is not at “100% pixel-level migration” yet, but after rechecking the current worktree there are no remaining explicit `gap` rows in this audit.
   - the unresolved rows are now all documented as either backend/environment blockers or irreducible JS/Python/Go public-surface differences
2. API key format validation now matches JS/Python (`e2b_` prefix plus lowercase hex body), with aligned tests in `api/client_test.go`.
3. Shared transport behavior now covers env-var parsing, reusable cached transports, HTTP/2 opt-out semantics, process-wide inflight request caps, envd REST vs RPC client separation, and Connect/RPC transient transport retry semantics with direct unit coverage.
4. Template caller attribution now captures Go-native builder call sites for builder-time validation failures and backend-reported build-step failures without changing existing error messages.
5. Representative upstream template method cases that previously looked environment-blocked (`runCmd`, invalid build user, `makeSymlink`, `makeSymlink(force)`, symlink copy, and `resolveSymlinks`) are now directly proven live against `e2bdev/base`, so they no longer depend on the failing `ubuntu:22.04` provisioning path.
6. The root Go package now exposes volume static wrappers (`CreateVolume`, `ConnectVolume`, `GetVolumeInfo`, `ListVolumes`, `DestroyVolume`) so the top-level surface is closer to the JS/Python volume entrypoints instead of forcing callers into the subpackage for volume management.
7. Exported Go sandbox/snapshot paginators now accept per-call request options on `NextItems`, and paginator page fetches now also honor per-call `Signal` cancellation:
   - this closes a real Go-only gap where paginator page requests accepted per-call opts but ignored per-call `Signal`
   - the remaining `abortSignal` difference is now mostly public API shape, because cancellation itself still uses `context.Context`
8. Several conclusions in the previous audit were stale in the current `/data` environment:
   - a local `base` template alias was built in this account, so upstream JS/Python sandbox source tests now run unmodified
   - template build API is available now
   - template build/background-build/exists/tag/stacktrace live tests pass now
   - auto-resume on HTTP request passes now
9. A real Go parity gap was found and fixed this turn in template build options:
   - `BuildOptions.SkipCache` existed on the Go surface, but unlike JS/Python it did not actually force the whole template build to skip cache
   - Go `Build(...)` / `BuildInBackground(...)` now apply `BuildOptions.SkipCache` to the serialized build payload just like JS/Python
   - Go template control-plane helpers also now match the shared JS/Python connection defaults more closely:
     - `Build(...)`, `BuildInBackground(...)`, `Exists(...)`, `GetBuildStatus(...)`, `AssignTags(...)`, `RemoveTags(...)`, and `GetTags(...)` now read the same env-backed API config inputs instead of requiring explicit Go-only wiring
     - when template request options omit `RequestTimeoutMs`, Go now uses the same nominal 60s default request timeout that current JS and Python connection configs expose
     - current Python template polling is not the same raw request shape as JS, though: Python sync/async build-status calls go through the generated `get_templates_template_id_builds_build_id_status` wrapper, which sends `limit=100` on every poll; Go now mirrors that internal `limit=100` default, while JS still sends only `logsOffset`
     - a new same-environment local `template_api_payload` probe now captures the request-build, trigger-build, exists, build-status, and tag helper request sequence in Go, JS, and Python without depending on backend build stability
     - that probe exposed one real Go-only wire bug that is now fixed: Go previously omitted explicit `force:false` on both the top-level trigger-build payload and per-step instruction payloads because of `omitempty`
     - the remaining raw request-shape split on that local probe is still `getBuildStatus`: Go and Python send `logsOffset=...&limit=100`, while JS sends only `logsOffset=...`
     - explicit zero request timeout is still preserved
   - Go template build defaults now also match JS/Python for `memoryMB`: omitted template build memory now defaults to `1024` instead of the previous Go-only `512`
   - Go template build-status polling now also matches JS/Python more closely:
     - `waitForBuildFinish(...)` now repolls every `200ms` instead of the previous Go-only `2s`
     - `logsOffset` now advances even when no logger is attached, matching JS/Python and avoiding repeated full-log fetches during polling
     - Go build-status polling now also sends `limit=100` alongside `logsOffset`, which matches the current Python sync/async generated client request shape more closely and reduces one remaining internal polling difference
   - Go template build logger semantics now also match JS/Python more closely:
     - `Build(...)` no longer implicitly enables `DefaultBuildLogger()` when the caller did not pass `OnBuildLogs`
     - when a logger is provided, Go now emits the same control-plane lifecycle messages JS/Python emit for build/build-in-background flows (`Requesting build...`, `Template created...`, upload/skip-upload, `All file uploads completed`, `Starting building...`, and for synchronous build also `Build started`, `Waiting for logs...`, `Build finished`)
     - Go `DefaultBuildLogger()` now ignores `debug` entries by default like the JS/Python default logger path
     - Go now also strips ANSI escape sequences from mapped template build log entries and `LogEntry.String()` output like the upstream JS/Python logger paths
   - Go template build tag handling now matches JS/Python more closely:
     - `requestBuild(...)` now uses the `/v3/templates` response `tags`
     - `Build(...)` no longer sends a Go-only extra `/templates/tags` request after a successful build
   - Go template file-ignore helper semantics now also match JS/Python more closely:
     - slashless ignore patterns such as `.env` and `temp*` are now treated as root-relative, not as basename matches at any depth
     - nested ignore behavior still requires explicit path/glob patterns like `**/*.spec.*` or `**/ui/**`, matching the current JS/Python source behavior
     - `getAllFilesInPath('.')` is now also pinned directly so the current-directory pattern stays inside the provided context while still returning `.` plus nested entries, matching JS/Python
   - Go Dockerfile parsing now also matches JS/Python more closely for multi-source copy instructions:
     - `COPY file1 file2 /dest/` and `COPY --chown=user a b /dest/` now expand into one Go `COPY` instruction per source path instead of incorrectly keeping only the first source
     - Dockerfiles without any `FROM` instruction and multi-stage Dockerfiles are now rejected at builder time like JS/Python instead of being silently accepted
     - Dockerfile `ENV`/`ARG` parsing now follows the current JS/Python parser semantics more closely for multi-pair `ENV` lines and `ARG` instructions with or without defaults
   - focused Go tests now pin both public build paths plus the env/default-timeout/default-memory behavior so these regressions cannot silently return
10. Go now directly proves the same NumPy randomness semantics exercised by the upstream JS integration test through `TestLiveTemporaryPythonNumpyTemplateRandomness`, which builds a temporary Python+NumPy template and verifies NumPy random vectors differ both within one sandbox and across sandboxes created from the same template.
   - the hardcoded upstream JS template alias `en716jw99aj63v1k8ugh` now exists in this account, so the old `404` blocker is gone
   - direct upstream JS `tests/integration/randomness.test.ts` now reaches the template, but its same-sandbox case is intermittent in this environment: repeated unmodified runs can pass fully or fail with `SandboxError: 2: [unknown] terminated` on the second command in the same sandbox
   - same-template manual repros through JS, Go, and Python all succeed repeatedly with the same NumPy command shape, so the remaining JS-source gap is now a flaky direct-source proof path rather than a Go-only or template-missing issue
   - the current Python upstream test tree does not include a dedicated randomness source suite
11. `claude-code-interpreter` was verified directly in the current environment:
   - Go live test `TestLiveClaudeCodeInterpreterRandomness` reaches the template successfully
   - a direct same-environment sandbox probe shows the current alias already has `python`, `python3`, `pip`, `pip3`, and `apt-get`
   - the direct alias still does not have `numpy`, so the Go live test skips and JS/Python direct commands fail with `ModuleNotFoundError: No module named 'numpy'`
   - JS fails on the same template for the same reason (`ModuleNotFoundError: No module named 'numpy'`)
   - repo-local `bash scripts/live_parity_crosscheck.sh claude_derived` now proves Go, JS, and Python can all derive from `claude-code-interpreter`, install `numpy` with `python3 -m pip install --break-system-packages --no-cache-dir numpy`, and observe same-sandbox and cross-sandbox NumPy randomness
   - this is an environment/template issue, not a Go-only migration failure
12. `volume` control-plane lifecycle works in the current environment for Go.
   - Go volume package `ConnectionOpts.Debug` now also uses `*bool`, and the control-plane request config now honors `E2B_DEBUG=true` for the local control-plane API URL, matching the upstream JS static volume entrypoints and Python volume control-plane calls that go through the shared connection config
13. `volume` file-content APIs are not currently provable in this environment:
   - Go `MakeDir("/multi-file-dir")` returns `Path /multi-file-dir not found`
   - JS `Volume.makeDir('/multi-file-dir')` returns the same `NotFoundError`
   - this is an environment/service behavior issue, not a Go-only migration failure
14. Some `ubuntu:22.04` template-build method cases are currently blocked by the environment, not by a proven Go-only mismatch:
   - Go live build logs fail during provisioning with mirror/certificate/package-resolution errors
   - the upstream JS SDK reaches the same final build status `error` with reason `error waiting for provisioning sandbox: exit status: 1`
   - but the representative user-visible `runCmd`/`makeSymlink`/symlink-upload behaviors are now proven live against `e2bdev/base`, so the remaining block is narrower than before
15. Go now implements sandbox network update parity at the API/surface level:
   - root/static `UpdateNetwork(ctx, sandboxID, network, opts)` exists
   - instance `(*Sandbox).UpdateNetwork(ctx, network, opts)` exists
   - `PUT /sandboxes/{sandboxID}/network` request serialization is covered
   - `bash scripts/live_parity_crosscheck.sh network_update_payload` now captures that local `PUT /sandboxes/{sandboxID}/network` body in Go, JS, and Python without relying on backend rule enforcement
   - selector-based update payloads now match across all three SDKs, including resolved `allowOut`, resolved `denyOut`, transformed `rules`, and `allow_internet_access=false`
   - Go now also preserves explicitly empty `allowOut: []`, `denyOut: []`, and `rules: {}` on update payloads like JS/Python instead of collapsing them to omission
   - omitted update fields are now proven live to clear existing egress rules in Go
   - same-environment JS checks show the previously failing `after update, 1.1.1.1 stays reachable` assertion is not stable in this backend either, so that is not a Go-only failure
   - direct upstream JS `tests/sandbox/network.test.ts` and Python `tests/sync/sandbox_sync/test_network.py` now run against that same local `base` alias and fail the same allow/deny/update reachability cases in this backend
   - `bash scripts/live_parity_crosscheck.sh network_egress` now matches those case-by-case egress outcomes across Go, JS, and Python on the same template
   - Go's focused `TestLiveSandboxUpdateNetwork` still covers a narrower subset than the direct upstream source suites, so `network_egress` is the stronger current same-template parity proof path for those blocked cases
16. Go snapshot API parity is now directly proven live in the current environment:
   - `TestLiveSnapshots` covers create, global list, per-sandbox list, named snapshots, restore into multiple sandboxes, filesystem preservation, branch isolation, and delete behavior
   - first `DeleteSnapshot` returns `true`
   - second `DeleteSnapshot` returns `false`, matching the current JS and Python source semantics
17. Go now also carries the upstream network `rules` shape on create/info/update requests and responses, with unit coverage for request/response serialization.
   - focused Go request tests now also pin explicit-empty `allowOut` / `denyOut` / `rules` serialization on both create and update payloads so this wire-shape regression cannot silently return
18. Go now matches more of the upstream network and lifecycle API shape:
   - `SandboxNetworkOpts` / `SandboxNetworkUpdate` accept selector callbacks equivalent to JS/Python `allowOut` / `denyOut` functions
   - Go now exposes `SandboxNetworkSelector`, `SandboxNetworkSelectorContext`, and `SandboxNetworkSelectorFunc`
   - sandbox info/full-info now use an info-only `SandboxNetworkInfo` return type instead of reusing the input options shape
   - `SandboxNetworkOpts.AllowPublicTraffic` and `SandboxNetworkInfo.AllowPublicTraffic` now use `*bool`, so omitted vs explicit `false` is preserved like JS/Python on both create and info paths
   - `SandboxLifecycle.AutoResume` now uses `*bool`, while output `SandboxInfoLifecycle.AutoResume` remains a required `bool` like JS/Python
   - create-time payloads now omit `allowPublicTraffic` when unset, preserve explicit `false`, preserve explicit empty `allowOut: []` / `denyOut: []` / `rules: {}` when provided, always send `autoResume: { enabled: ... }` with default `false`, and reject `autoResume=true` unless the resolved `onTimeout` is `pause`
19. Go now directly proves the upstream internet/public-host traffic controls live in this environment:
   - `allowInternetAccess` default and explicit `true` both allow outbound `curl` connectivity and return `204`
   - `allowInternetAccess=false` blocks outbound `curl` with a `CommandExitError`
   - `allowPublicTraffic=false` gives `403` without the traffic access token and `200` with it
   - `allowPublicTraffic=true` gives `200` without a token
   - `maskRequestHost` rewrites the inbound `Host` header to the configured host plus port
   - same-environment JS checks pass for the same cases, so these are no longer just env-skip items in the current backend
20. Go metrics shape is now closer to the upstream JS/Python surface:
   - Go `SandboxMetrics` now includes `MemCache`
   - unit coverage verifies `memCache` is preserved from the current API response shape
   - same-environment cross-checks prove unfiltered metrics exist in Go, JS, and Python in the current backend
   - Go live integration now probes an inclusive filtered window around the returned metric timestamp and skips with a concrete environment reason when the backend does not honor it
   - the repo-local Go cross-check now also records the raw control-plane count for that same inclusive window, and the raw request returns zero rows too
   - the repo-local Python cross-check now records the same raw control-plane counts, and both the inclusive filtered window and the raw request return zero rows there too
   - after correcting the local JS metrics request to send `start`/`end` as query params, JS matches the same backend limitation too
   - direct upstream JS `tests/sandbox/metrics.test.ts` and Python `tests/sync/sandbox_sync/test_metrics.py` now run against the local `base` alias and fail the same filtered-window assertion
   - filtered-window behavior is therefore environment/backend-limited rather than a current Go-only mismatch: Go, JS, and Python all return zero items for real filtered windows in this backend
21. The old audit missed upstream files that must be considered:
   - JS: `api/http2.test.ts`, `api/inflight.test.ts`, `api/validateApiKey.test.ts`, `connectionConfig.browser.test.ts`, `envd/http2.test.ts`, `sandbox/abortSignal.test.ts`, `template/abortSignal.test.ts`
   - JS: `runtimes/browser/run.test.tsx`, `runtimes/bun/run.test.ts`, `runtimes/deno/run.test.ts`, `integration/stress.test.ts`
   - Python: `test_api_client_transport.py`, `test_validate_api_key.py`, `test_connection_config.py`, `test_volume_connection_config.py`, `bugs/test_envelope_decode.py`
22. The previously omitted config/helper files are now explicitly classified:
   - Python `test_connection_config.py` is covered by `connection_config_test.go`
   - Python `test_volume_connection_config.py` is now covered by `volume/volume_test.go`
   - JS runtime harness tests (`browser`/`bun`/`deno`) are runtime-specific and not a Go SDK surface requirement; equivalent live sandbox/command/filesystem semantics are already covered by Go integration tests
   - JS `integration/stress.test.ts` is an operational load test, not a stable API parity contract
   - Python `bugs/test_envelope_decode.py` is skipped upstream and desktop-template specific, so it is not a missing stable Go parity requirement in the current scope
23. The remaining environment-blocked cases now have executable cross-language repro helpers in this repo:
   - `scripts/live_parity_crosscheck.sh`
   - `scripts/live_parity_crosscheck/go_main.go`
   - `scripts/live_parity_crosscheck/js.ts`
   - `scripts/live_parity_crosscheck/py.py`
   These let us rerun the same blocker probes through Go, JS, and Python SDKs without reconstructing ad hoc one-liners.
   - the shell wrapper now also accepts `LIVE_PARITY_TIMEOUT_SEC` and converts runner wall-timeouts into structured `env_blocked` JSON results, so long Python/JS/Go hangs no longer disappear into an outer shell timeout
24. Go now matches the upstream JS stable-vs-direct sandbox URL split:
   - for supported hosted domains, envd transport uses the stable `https://sandbox.<domain>` host
   - signed file upload/download URLs still use the direct per-sandbox host
   - focused Go unit coverage now pins both behaviors and their fallback/override cases
   - direct upstream JS `tests/connectionConfig.test.ts` passes in the same workspace
   - current Python source still uses the direct per-sandbox host for both transport and file URLs, so this was a real JS parity gap in Go rather than a shared JS+Python contract
25. Go sandbox instance config propagation now preserves inherited headers when per-call headers are provided:
   - `resolveConnectionConfig` and `resolveSandboxApiConnectionConfig` now merge per-call headers over the inherited sandbox connection headers instead of replacing them
   - this prevents per-call overrides from dropping the Go SDK `User-Agent` and other inherited headers
   - direct Go `Sandbox.Pause(...)` coverage now also pins both “inherit base config without overrides” and “override wins” semantics on the real instance API path, including merged headers and request-timeout override behavior
   - direct Go `Sandbox.UpdateNetwork(..., opts)` coverage now also pins the same per-call `Signal` propagation that the upstream JS `configPropagation.test.ts` exercises on the instance method boundary
   - `newSandboxFromResponse(...)` coverage now also pins that sandbox-created `Commands`, `Filesystem`, and `Pty` clients all forward the derived sandbox transport headers (`E2b-Sandbox-Id`, `E2b-Sandbox-Port`, `X-Access-Token`) plus inherited custom headers on real request paths
   - inspection of `NewCommands(...)`, `NewFilesystem(...)`, and `NewPty(...)` did not reveal a clean narrowing path for their current `cfg any` constructors without introducing an additional exported Go-only config carrier or config-getter method surface, so that constructor bag remains an intentional compatibility compromise rather than an untracked parity regression
   - external-package tests now also prove those subpackage constructors accept a real root `*ConnectionConfig` from outside the SDK package boundary and still preserve inherited headers plus `User-Agent` on real request paths, so the current constructor bag remains externally usable without exporting extra Go-only config types
   - focused helper-level Go coverage still pins the lower-level merge and optional-field semantics directly
   - direct JS `tests/sandbox/configPropagation.test.ts` still passes in the same workspace
   - upstream Python `sandbox_sync/test_config_propagation.py` and `sandbox_async/test_config_propagation.py` both pass in the same workspace and prove the stricter Python header-merging behavior directly
26. Go volume instances now persist only the JS/Python-aligned instance fields:
   - `Volume` state is now limited to `volumeId`, `name`, `token`, `domain`, and `debug`
   - Go `Volume.Debug` now uses `*bool`, and volume content opts now also use `*bool` for `debug`, so omitted vs explicit `false` stays distinct on the volume content path like the current JS/Python volume sources
   - create/connect-time transport fields such as `apiUrl`, `headers`, `requestTimeoutMs`, `proxy`, `logger`, and `signal` no longer leak into later instance content calls
   - focused Go tests now pin create-time header/timeout non-persistence, nil-safe `Connect(..., nil)` / `resolveClient(nil)` behavior, and explicit `debug=false` override semantics against env/persisted `debug=true`
   - direct same-workspace JS proof shows `new Volume(...)` only carries `["debug","domain","name","token","volumeId"]` and `new VolumeConnectionConfig(volume, {})` falls back to default connection config instead of persisted create/connect transport overrides
   - current Python `Volume.__init__` / `_get_volume_config` in the same workspace also only persist `_token`, `_domain`, and `_debug`; `api_url`, `request_timeout`, `headers`, and `proxy` still come from per-call opts/defaults rather than stored instance state
27. Same-environment network-rule transform cross-checks now show the remaining live gap is backend-wide rather than Go-specific:
   - `bash scripts/live_parity_crosscheck.sh network_rules` now creates temporary base-image sandboxes through Go, JS, and Python and runs the same `curl https://httpbin.e2b.team/headers` probe with an injected `X-E2B-Test-Token`
   - all three SDKs create the sandbox successfully, but the reflected header is still empty
   - create-time `network.rules` header transforms are therefore not currently enforced by this backend for Go, JS, or Python
28. Go now directly covers the remaining JS host-path semantics that were previously only implied:
   - `TestLiveSandboxHost` now verifies both `getHost(port)` reachability while the sandbox is running and the JS-only negative case where fetching the same host after kill returns `502` with a JSON payload containing `message`, `code`, and a sandbox-id prefix
   - direct same-workspace JS `tests/sandbox/host.test.ts` now runs and passes against the locally built `base` alias
   - upstream Python `tests/sync/sandbox_sync/test_host.py` also passes against that same alias
   - the Go live assertion therefore matches directly runnable upstream-source evidence for the killed-host contract instead of being the only proof path
29. Go paginator page fetches now honor per-call `Signal` like the upstream JS abort-signal source test:
   - `mergeSandboxApiOpts` now preserves `Signal`, and sandbox/snapshot paginator fetchers merge the page-fetch context with paginator-level or per-call `Signal`
   - focused Go tests now pin in-flight per-call `Signal` cancellation on both `SandboxPaginator.NextItems(...)` and `SnapshotPaginator.NextItems(...)`
   - this closes a real Go-only mismatch with the upstream JS `Sandbox.list().nextItems({ signal })` behavior
30. Go now directly covers the upstream sandbox filesystem read/write/encoding semantics more closely:
   - `TestLiveFilesystem` now directly asserts relative-path single-file writes returning `/home/user/...`, nested relative-path auto-parent creation, relative `WriteFiles(...)` path normalization, exact multi-file metadata/order, gzip text and byte readback, missing-file typed errors plus deprecated `NotFoundError` compatibility, and empty-file reads
   - `TestLiveFileSigning` now also directly asserts secure-sandbox `Files.Write(...)` / `Files.Read(...)` metadata and readback on a regular relative path, which matches the upstream Python secure-filesystem expectation more directly
   - Go filesystem writes now also accept `string`, `[]byte`, Go-side `Blob`, and `io.Reader` inputs directly for both single-file `Write(...)` and multi-file `WriteFiles(...)`, which is closer to the upstream JS/Python text/bytes/blob/stream write families than the previous Go-only `io.Reader` restriction
   - Go `Read(...)` now also acts as the primary single-entry read surface: it defaults to text and switches return shape through `FilesystemReadOpts.Format` for `text` / `bytes` / `blob` / `stream`
   - Go no longer exports additive filesystem read helpers; the public surface now stays on the single-entry `Read(...)` API
   - Go `FilesystemListOpts.Depth` now also uses `*int` instead of `int`, which means omitted depth still defaults to `1` like JS/Python, but explicit `0` is now rejected instead of being silently treated like omission
   - the remaining difference here is now mostly literal API shape plus upstream-language divergence: Go is structurally closer to JS because JS `files.read(...)` also uses `opts.format` and exposes `text` / `bytes` / `blob` / `stream`, while Python sync/async `read(...)` use a named `format` parameter and only type `text` / `bytes` / `stream`; Go still represents the JS browser-native payload families through Go-native types (`Blob`, `[]byte`, `io.Reader`) instead of literal `Blob` / `ArrayBuffer` / `ReadableStream` runtime objects
31. Direct upstream sandbox filesystem suites now run unmodified against the local `base` alias:
   - JS `tests/sandbox/files/*.test.ts`: 11 files, 49 tests passed
   - Python `tests/sync/sandbox_sync/files`: 43 tests passed
32. A real Go public-surface parity gap was found and fixed this turn in `git restore` options:
   - upstream JS `GitRestoreOpts` and Python `Git.restore(...)` both use `paths` for restore targets
   - Go previously exposed only `GitRestoreOpts.Files`, which was a Go-only option-shape mismatch on a source-level API contract
   - Go `GitRestoreOpts` now exposes `Paths []string` like JS/Python
   - Go also keeps `Files []string` as a deprecated compatibility alias so existing Go callers still work
33. Go git clone depth shape now matches the upstream optional contract more closely:
   - upstream JS `GitCloneOpts.depth?: number` and Python `git.clone(..., depth: Optional[int] = None, ...)` both model clone depth as optional
   - Go previously exposed `GitCloneOpts.Depth int`, which made the public shape more rigid than the current JS/Python source even though `0` still behaved like omission at runtime
   - Go `GitCloneOpts.Depth` now uses `*int`, so omitted-vs-present depth is represented directly in the public surface while preserving the same practical command behavior
   - focused Go tests now pin both the JS/Python-aligned `Paths` field and the legacy alias fallback
33. Direct upstream `git` suites now run unmodified in the same workspace:
   - JS `tests/sandbox/git/*.test.ts`: 12 files, 25 tests passed
   - Python `tests/shared/git`: 27 tests passed
   - this confirms the current environment is stable for the representative upstream `git` surface and that the remaining `git` parity work is not being masked by backend instability
34. Go now matches the remaining deterministic upstream `git` option shapes more closely:
   - `GitCommitOpts` no longer exposes the Go-only `Author` field; it now matches the upstream JS/Python `authorName` / `authorEmail` / `allowEmpty` shape
   - `GitPushOpts` no longer exposes the Go-only `Force` field
   - `GitPullOpts` no longer exposes the Go-only `Rebase` field
   - `Commit(...)` now also preserves the upstream JS/Python `-c user.name=...` then `-c user.email=...` argument order
   - focused Go tests now pin those direct field sets and the commit command shape so these source-level mismatches cannot silently return
35. Go now matches the upstream sandbox paginator option shapes more closely:
   - upstream JS `SandboxListOpts` / `SnapshotListOpts` explicitly omit `signal`, and Python list/list-snapshots entrypoints likewise take pagination/query/api params without a dedicated list-level cancel field
   - Go `SandboxListOpts` and `SnapshotListOpts` no longer expose a Go-only `Signal` field
   - per-page cancellation still remains available where upstream has it: Go paginator `NextItems(...)` / `NextItemsContext(...)` still accept per-call `SandboxApiOpts{Signal: ...}` overrides just like JS paginator `nextItems(opts)`
   - focused Go tests now pin both no-`Signal` list-option shapes and the continuing per-page paginator cancellation behavior
36. Go now matches part of the remaining deterministic command-start shape more closely:
   - upstream JS and Python both still expose a public background-run entry plus a single optional `stdin` control, but they are not literally identical: JS carries `background` inside the `run(cmd, opts)` object, while Python exposes `background` as a named `run(...)` parameter
   - Go `CommandStartOpts` no longer exposes the extra `StdinOpt` compatibility field
   - Go now models `stdin` through the single `Stdin` option field only, which preserves the needed omitted/true/false distinction for old-envd compatibility
   - Go `CommandStartOpts` now also exposes `Background bool`, and `Run(...)` now acts as the primary single-entry command surface: it returns a foreground `*CommandResult` by default and a background `*CommandHandle` when `opts.Background` is true, through an explicit internal `commandExecution` interface instead of a bare `any` return
   - Go no longer exposes the extra public helpers `RunForeground(...)`, `RunBackground(...)`, and `RunWithMode(...)`
   - the remaining difference is now mostly literal type-shape: Go is structurally closer to JS on the option-carrier side, but JS/Python still express the return split through overloads/union typing on one `run(...)` API while Go still cannot literally express those overload signatures and instead narrows them through an internal interface
   - external-package tests now also pin that callers outside the SDK package can still type-assert `Run(...)` results to `*CommandResult` / `*CommandHandle` and read `handle.State()` fields even though the execution/state helper types remain unnamed
   - focused Go tests now pin the aligned `Background` field, the single-entry `Run(...)` foreground/background behavior, the absence of the legacy helper methods from the public surface, plus the old-envd explicit-`stdin=false` rejection path and omitted-stdin success
37. Go now matches the remaining deterministic filesystem read/write option shape and octet-stream behavior more closely:
   - upstream JS `FilesystemWriteOpts` and Python `write(...)` / `write_files(...)` only use `application/octet-stream` when the caller explicitly opts in with `useOctetStream` / `use_octet_stream`
   - Go previously diverged here in two ways:
     - it kept extra public filesystem read helpers on top of the primary `Read(...)` surface
     - on envd `>= 0.5.7`, Go always switched writes to `application/octet-stream` by default, while upstream JS/Python still defaulted to `multipart/form-data`
   - Go `FilesystemWriteOpts` now exposes `UseOctetStream bool` like the upstream write surface
   - Go `Read(...)` remains the primary public read entry, with `FilesystemReadOpts.Format` selecting `text` / `bytes` / `blob` / `stream`
   - the extra public filesystem read helpers were removed, so the exported surface is now closer to the upstream single-entry contract
   - Go `Write(...)` / `WriteFiles(...)` now default to multipart uploads on newer envd too, and only switch to octet-stream when `UseOctetStream` is explicitly set and the sandbox envd version supports it
   - focused Go tests now pin the direct filesystem opts field sets, default multipart-on-new-envd behavior, and the explicit per-file octet-stream upload path
38. Go now matches the upstream PTY resize surface more closely:
   - upstream JS `pty.resize(pid, { cols, rows }, opts)` and Python `pty.resize(pid, PtySize, request_timeout=...)` both use a dedicated size object instead of split positional `cols` / `rows` parameters
   - Go previously exposed `(*Pty).Resize(ctx, pid, cols, rows, opts)`, which was a Go-only public-surface mismatch
   - Go now exports `PtySize` and `(*Pty).Resize(ctx, pid, size, opts)` like the upstream size-object contract
   - focused Go tests now pin the `PtySize` field set, the `Resize(...)` method signature, the nested `pty.size` request payload, the root-package `PtySize` alias export, and the continuing live PTY resize path
39. Go now matches the upstream volume list option surface more closely:
   - upstream JS `volume.list(path, opts?: VolumeApiOpts & { depth?: number })` and Python `volume.list(path, depth=None, **opts)` both use an explicit list-specific depth input rather than an untyped reflection bag
   - Go previously exposed `(*Volume).List(ctx, path, opts any)` and used reflection to probe `Depth` plus connection fields, which was a Go-only public-surface mismatch
   - Go now exposes `VolumeListOpts` with embedded `VolumeApiOpts` plus `Depth *int`
   - `(*Volume).List(ctx, path, opts *VolumeListOpts)` now uses that typed surface directly, the root package re-exports `VolumeListOpts`, and focused Go tests now pin both the field shape and the `depth` query/default-omission behavior
40. Go now matches the upstream volume write-option surface more closely:
   - upstream JS `writeFile(...)` / `makeDir(...)` accept `VolumeWriteOptions & VolumeApiOpts`, so write-style calls can carry the same connection-level `logger` field as other volume API options
   - upstream JS also models `force?: boolean`, and Python `make_dir(..., force: Optional[bool] = None)` / `write_file(..., force: Optional[bool] = None)` use the same omission-vs-explicit-bool contract
   - Go previously exposed `VolumeWriteOptions` without `Logger`, and it modeled `Force` as a plain `bool`, so this was still a real Go-only surface mismatch in the write-option family
   - Go `VolumeWriteOptions` now also exposes `Logger api.Logger`, and `Force` now uses `*bool` so omission is distinct from explicit `false` / `true` like upstream
   - `volumeWriteOptsToApiOpts(...)` now preserves that logger into the resolved `VolumeApiOpts`, `queryFromVolumeWriteOpts(...)` now preserves explicit `force=false`, and focused Go tests pin both the field shapes and the mapping/query behavior
41. Go now matches the upstream template control-plane helper surface more closely:
   - upstream JS `Template.exists/assignTags/removeTags/getTags` and Python `Template.exists/assign_tags/remove_tags/get_tags` accept only shared connection/request options, not build-only fields
   - upstream JS `Template.getBuildStatus(...)` and Python `Template.get_build_status(...)` both consume build-info objects instead of split `templateID` / `buildID` positional parameters
   - Go previously exposed `BuildOptions` on `Exists(...)`, `AliasExists(...)`, `AssignTags(...)`, `RemoveTags(...)`, and `GetTags(...)`, and it exposed `GetBuildStatus(ctx, templateID, buildID, opts)`
   - the `template` package now exports connection-only `ConnectionOpts` for those helper APIs, the root package uses the shared root `ConnectionOpts` on its wrappers, and `GetBuildStatus(...)` now accepts `*BuildInfo`
   - `template.ConnectionOpts`, `BuildOptions`, and `GetBuildStatusOptions` now also use `*bool` for `Debug`, which is closer to the upstream optional shared-connection shape
   - focused Go tests now pin both the subpackage and root-package signatures so this public-surface mismatch cannot silently return
42. Direct upstream sandbox command suites now run unmodified in the same workspace:
   - JS `tests/sandbox/commands/*.test.ts`: 6 files, 17 tests passed
   - Python `tests/sync/sandbox_sync/commands`: 19 tests passed
   - Go already had equivalent run/connect/list/kill/send-stdin/env-var behavior covered through `live_integration_test.go` and `commands/*_test.go`, and focused Go tests now pin the single-entry `Run(...)` foreground/background split plus background-handle output/callback semantics explicitly
43. Go now also explicitly pins the filesystem single-entry read surface more closely:
   - upstream JS and Python keep a single `files.read(...)` entry and select the return type through the `format` argument or overload family
   - Go runtime behavior is covered, and Go `Read(...)` now also defaults to text and switches return shape through `FilesystemReadOpts.Format` for `text` / `bytes` / `blob` / `stream`
   - Go no longer exports additive filesystem read helpers, so the public surface is now narrower and closer to the upstream single-entry contract
   - Go filesystem writes now accept `string`, `[]byte`, Go-side `Blob`, and `io.Reader` directly on both `Write(...)` and `WriteEntry.Data`, which closes the previous Go-only `io.Reader` restriction and brings the write surface closer to JS/Python
   - focused Go tests now pin that broader write-input support, the primary single-entry `Read(...)` behavior, the absence of the legacy helper methods from the exported surface, and the remaining browser-type/format-shape differences so the audit does not overstate literal API parity
44. Direct upstream static sandbox API suites now also run unmodified in the same workspace:
   - JS `tests/api/*.test.ts`: 8 files, 46 tests passed
   - Python `tests/sync/api_sync`: 13 tests passed
   - this directly covers the current upstream `getInfo` / `list` / `kill` / `pause` / `resume` static entrypoints against the same environment
   - current upstream JS and Python still diverge on one source-level detail here: JS `Sandbox.pause(...)` returns `Promise<boolean>`, while Python `Sandbox.pause(...)` returns `None`, so Go's boolean `Pause(...)` result is not a shared JS+Python contract gap by itself
45. Go volume file writes now also align more closely with the upstream JS/Python data-input family:
   - upstream JS `volume.writeFile(...)` accepts text/bytes/blob/stream-style inputs, and Python `volume.write_file(...)` accepts `str` / `bytes` / `IO`
   - Go `WriteFile(...)` previously required `io.Reader` only, which was a real Go-only input-surface restriction
   - Go `WriteFile(...)` now accepts `string`, `[]byte`, Go-side `Blob`, and `io.Reader`, and focused volume parity tests now pin text, bytes, blob, IO/stream, empty-file, metadata/force, and unsupported-type behavior
   - Go `ReadFile(...)` now also acts as the primary single-entry read surface: it defaults to text and switches return shape through `VolumeReadOpts.Format` for `text` / `bytes` / `blob` / `stream`
   - Go no longer exports additive volume read helpers, so the public surface is now closer to the upstream single-entry `readFile(...)` family
   - the remaining volume file public-surface difference is now mostly literal API shape plus upstream-language divergence: Go is structurally closer to JS because JS `readFile(...)` also uses `opts.format` and exposes `text` / `bytes` / `blob` / `stream`, while Python sync/async `read_file(...)` use a named `format` parameter and only type `text` / `bytes` / `stream`; Go still represents the JS browser-native payload families through Go-native types (`Blob`, `[]byte`, `io.Reader`) instead of literal `Blob` / `ArrayBuffer` / `ReadableStream` runtime objects
46. Direct upstream volume suites now also run in the same workspace and split cleanly between control-plane success and shared file-content backend failure:
   - JS `tests/volume/volume.test.ts`: 8 tests passed
   - JS `tests/volume/file.test.ts`: 22 failed, 4 passed, 1 skipped
   - Python `tests/sync/volume_sync/test_volume.py`: 8 passed
   - Python `tests/sync/volume_sync/test_file.py`: 22 failed, 4 passed, 1 skipped
   - the failing JS/Python file-content cases all hit the same current-environment backend pattern (`Path ... not found`), so the remaining live volume-content block is not a Go-only miss
47. Go command handles no longer expose additive getter helpers that upstream JS/Python do not define:
   - upstream JS exposes live `stdout` / `stderr` / `exitCode` / `error` properties on the handle, Python sync mainly exposes `pid` plus iteration / `wait(...)`, and Python async exposes properties too
   - Go previously still exported Go-only `GetStdout` / `GetStderr` / `GetExitCode` / `GetError` helpers on `CommandHandle`
   - Go also previously exposed a named `CommandHandleState` export at both the `commands` package and root package layers, which upstream JS/Python do not define
   - Go `State()` snapshots also no longer duplicate `pid`; that stays on `CommandHandle`, which matches the upstream handle model more closely
   - Go now keeps the live `State()` snapshot plus `Wait()` / `Kill()` / `Disconnect()`, but the extra getter methods, named `CommandHandleState` export, and duplicate `pid` snapshot field are gone; focused surface/runtime tests now pin that tighter surface
48. Direct upstream template utility suites now also run in the same workspace:
   - JS `tests/template/utils/*.test.ts`: 50 passed, 3 skipped
   - Python `tests/shared/template/utils`: 33 passed, 1 skipped
   - these directly cover the current file-ignore, caller-directory, and helper semantics that Go now pins in `template/template_utils_alignment_test.go`
49. Repo-local randomness cross-checks now also run across all three SDKs in the same workspace:
   - `bash scripts/live_parity_crosscheck.sh randomness` returns `ok` for Go, JS, and Python
   - each language builds a temporary Python+NumPy template and verifies NumPy random vectors differ both within one sandbox and across sandboxes created from the same template
   - this closes the same-environment cross-language proof gap even though the direct upstream JS randomness file is still account-blocked by its hardcoded template ID
50. Direct upstream PTY suites now also run unmodified in the same workspace:
   - JS `tests/sandbox/pty/*.test.ts`: 4 tests passed
   - Python `tests/sync/sandbox_sync/pty`: 4 tests passed
   - this directly confirms the current upstream PTY create/connect/reconnect/resize/send-input behaviors against the same environment, in addition to Go's focused PTY unit and live tests

## Verified This Turn

Go commands run successfully:

```bash
go test ./... -count=1
go test ./... -run 'TestPaginator|TestSandboxPaginatorNextItems|TestListFactoriesDoNotExposeContextParameter' -count=1
go test ./... -run 'Test(SandboxPaginatorNextItemsHonorsPerCallSignalContext|SnapshotPaginatorNextItemsHonorsPerCallSignalContext)' -count=1
go test -tags=integration -run 'TestLiveTemplateBuildUploadAndTags|TestLiveTemplateBuildStacktrace|TestLiveSandboxLifecycleAutoPause' -count=1 -v .
go test -tags=integration -run 'TestLiveClaudeCodeInterpreterRandomness|TestLiveVolumeLifecycle' -count=1 -v .
go test -tags=integration -run 'TestLiveSandboxLifecycle|TestLiveSandboxLifecycleAutoPause|TestLiveCommands|TestLiveFilesystem' -count=1 -v .
go test -tags=integration -run '^TestLiveFilesystem$|^TestLiveFileSigning$' -count=1 -v .
go test -tags=integration -run 'TestLiveTemplateBuildRejectsInvalidTagFormats|TestLiveTemplateBuildInBackgroundStatus|TestLiveTemplateBuildFromExistingTemplate' -count=1 -v .
go test ./template -run 'Test(BuildInBackgroundUsesStructuredJsTemplatePayload|BuildInBackgroundPreservesExplicitFalseForceFieldsLikeJsAndPython)$'
go test -tags=integration -run 'TestLiveTemplateBuildMethodParityOnBaseImage' -count=1 -v .
go test -tags=integration -run 'TestLiveTemporaryPythonNumpyTemplateRandomness' -count=1 -v .
go test -tags=integration -run 'TestLiveClaudeCodeInterpreterDerivedNumpyTemplateRandomness' -count=1 -timeout 45m .
go test -tags=integration -run 'TestLiveSandboxNetwork|TestLiveSandboxNetworkRules|TestLiveSandboxUpdateNetwork' -count=1 -v .
go test -tags=integration -run '^TestLiveSnapshots$|^TestLiveSandboxInternetAccess$|^TestLiveSandboxPublicTraffic$' -count=1 -v .
go test -tags=integration -run '^TestLiveSandboxHost$' -count=1 -v .
go test ./... -run 'TestGetSignature' -count=1
go test -tags=integration -run '^TestLiveFileSigning$' -count=1 -v .
go test ./... -run 'TestDownloadURLMatchesJsDirectUrlSerialization|TestFileURLPreservesJsQueryParameterOrder|TestUploadURLUsesUserAndSignatureExpiration|TestDownloadURLUsesDefaultUserForOldEnvd' -count=1
go test ./volume -run 'TestVolume(FileParity|ListWrapsNotFound|UpdateMetadataWrapsNotFound|MakeDirWrapsNotFound|RemoveWrapsNotFound)' -count=1
go test ./volume -run 'Test(ResolveClientUsesPersistedVolumeFieldsWhenOptsNil|ConnectAllowsNilOptsAndExposesJsStyleVolumeMetadataFields|VolumeCreateDoesNotPersist(HeadersIntoFutureInstanceCalls|RequestTimeoutIntoFutureInstanceCalls))' -count=1
go test ./volume -count=1
go test ./... -run 'TestConnectionConfig|TestNewSandboxFromResponse|TestDownloadURLMatchesJsDirectUrlSerialization|TestFileURLPreservesJsQueryParameterOrder|TestFileURLFallsBackToEnvdApiUrlWhenDirectUrlMissing' -count=1
go test ./... -run 'TestResolveConnectionConfig|TestResolveSandboxApiConnectionConfig|TestConnectionConfig|TestNewSandboxFromResponse' -count=1
go test ./... -count=1
go test ./filesystem -run 'Test(FilesystemReadAndWriteOptsMatchJsAndPythonFieldShape|FilesystemUsesSingleEntryReadSurfaceAndBroadWriteInputs|ReadMatchesJsStyleSingleEntrySurface|ReadRejectsUnsupportedFormats|ListDefaultsDepthWhenOmittedAndRejectsExplicitZero|WriteUsesMultipartByDefaultOnEnvdThatSupportsOctetStream|WriteFilesUsesSingleMultipartRequestOnOldEnvd|WriteFilesUsesSingleMultipartRequestByDefaultOnEnvdThatSupportsOctetStream|WriteFilesUsesOctetStreamPerFileWhenExplicitlyRequested|WriteMultipartUsesPathQueryForSingleFileOnOldEnvd|WriteRejectsUnsupportedDataTypeWithInvalidArgumentError|WriteFilesAcceptsBlobInput)' -count=1
go test -tags=integration -run '^TestLiveFilesystem$' -count=1 -v .
go test ./volume -run 'Test(VolumeApiOptsOnlyExposeConnectionFields|VolumeReadOptsMatchJsAndPythonReadShape|VolumePackageExposesJsStyleStaticSurfaceNames|VolumeFileParityWriteAndReadText|VolumeFileParityWriteAndReadBytes|VolumeFileParityWriteAndReadStreamInput|VolumeFileParityWriteAndReadBlob|VolumeFileParityWriteAndReadEmptyFile|VolumeFileParityReadFileMatchesJsStyleSingleEntrySurface|VolumeFileParityReadFileRejectsUnsupportedFormats|VolumeFileParityWriteFileWithMetadataAndForce|VolumeFileParityWriteFileRejectsUnsupportedDataType|VolumeWriteFileUsesPerCallApiURLAndTimeout|VolumeReadFileWrapsNotFoundAsSdkNotFoundError|VolumeReadFileReturnsStreamResponseBody)' -count=1
go test ./template -run 'Test(SkipCacheMarksWholeTemplateWhenAppliedBeforeBaseLayer|BuildOptionSkipCacheMarksWholeTemplateForBuild|BuildOptionSkipCacheMarksWholeTemplateForBuildInBackground)' -count=1
go test ./template -run 'Test(TemplateApiClientsUseEnvFallbackAndDefaultTimeoutLikeJsAndPython|TemplateApiClientsPreserveExplicitZeroRequestTimeoutLikeJsAndPython|BuildInBackgroundUsesEnvFallbackAndDefaultsMemoryLikeJsAndPython)' -count=1
go test ./template -run 'Test(WaitForBuildFinishRepollsAtJsCadence|WaitForBuildFinishAdvancesLogsOffsetWithoutLoggerLikeJsAndPython|WaitForBuildFinishIncludesCallerTraceForFailedStep)' -count=1
go test ./template -run 'Test(BuildUsesRequestBuildResponseTagsAndEmitsJsLifecycleLogs|BuildInBackgroundWithLoggerEmitsJsUploadLifecycleLogs|BuildWithoutLoggerDoesNotUseDefaultBuildLoggerLikeJsAndPython|DefaultBuildLoggerIgnoresDebugByDefaultLikeJsAndPython)' -count=1
go test ./git -count=1
go test ./git -run 'Test(GitOptionStructsMatchJsAndPythonFieldShapes|CloneUsesOptionalDepthLikeJsAndPython)' -count=1
go test ./filesystem -run 'Test(FilesystemReadAndWriteOptsMatchJsAndPythonFieldShape|WriteErrorsWhenExplicitOctetStreamUploadReturnsNoInfo|WriteErrorsWhenMultipartUploadReturnsNoInfo|WriteUsesMultipartByDefaultOnEnvdThatSupportsOctetStream|WriteFilesUsesSingleMultipartRequest(OnOldEnvd|ByDefaultOnEnvdThatSupportsOctetStream)|WriteFilesUsesOctetStreamPerFileWhenExplicitlyRequested|WriteMultipartUsesPathQueryForSingleFileOnOldEnvd|WriteFilesEmptyArrayMatchesJsNoop)' -count=1
go test ./commands . -run 'Test(PtyResizeUsesJsAndPythonStyleSizeSurface|PtyResizeUsesNestedSizeRequestLikeJsAndPython|PtyCreateSendsConnectEnvelopeRequest|PtyOnDataReceivesRawBytes|PtyApisHonorSignalContext|GoDocSurfaceMatchesAlignedExportNames|RootAliasesExposeJsStyleFilesystemCommandAndGitTypes)' -count=1
go test ./volume . -run 'Test(VolumeListUsesDepthOption|VolumeApiOptsOnlyExposeConnectionFields|VolumeListOptsMatchJsAndPythonDepthShape|VolumeFileParityListDepthOption|VolumeFileParityListOmitsDepthQueryByDefaultLikeJsAndPython|VolumeListReturnsEmptySliceWhenResponseBodyMissing|VolumeListWrapsNotFoundAsSdkNotFoundError|RootAliasesExposeJsStyleVolumeTypes|GoDocSurfaceMatchesAlignedExportNames)' -count=1
go test ./volume -run 'Test(VolumeWriteOptionsExposeLoggerLikeJsVolumeApiOpts|VolumeWriteOptsToApiOptsPreservesLogger|VolumeListOptsMatchJsAndPythonDepthShape|VolumeListUsesDepthOption|VolumeApiOptsOnlyExposeConnectionFields)' -count=1
go test ./volume . -run 'Test(NewVolumeConnectionConfigPreservesExplicitFalseDebugOverEnv|ResolveClientPreservesPersistedExplicitFalseDebugOverEnv|ResolveClientAllowsExplicitFalseDebugOverride|VolumeApiOptsDebugMatchesJsAndPythonOptionalShape|VolumeWriteOptionsDebugMatchesJsAndPythonOptionalShape|VolumeWriteOptsToApiOptsPreservesExplicitFalseDebug|VolumePackageExposesJsStyleStaticSurfaceNames|RootAliasesExposeJsStyleVolumeTypes|GoDocSurfaceMatchesAlignedExportNames)' -count=1
go test ./volume -run 'Test(BuildApiClientConfigUsesDebugApiURL|BuildApiClientConfigUsesDebugApiURLFromEnv)' -count=1
go test . -run 'Test(NewConnectionConfigDefaultsApiUrlLikeJsConstructor|ConnectionOptsDebugMatchesJsAndPythonOptionalShape|PausePassesInheritedConnectionConfigWithoutOverrides|PauseLetsPerCallOverridesWinOverInheritedConnectionConfig|PauseMergesPerCallHeadersOverInheritedConnectionHeaders|ResolveConnectionConfigAllowsExplicitZeroRequestTimeout|ResolveConnectionConfigCarriesSandboxUrlAndLoggerOverrides|ResolveConnectionConfigMergesHeadersWithPerCallOverrides|ResolveConnectionConfigLetsPerCallHeadersOverrideBaseValues|ResolveConnectionConfigAllowsExplicitFalseDebugOverride|GoDocSurfaceMatchesAlignedExportNames)' -count=1
go test . -run 'Test(SandboxListOptsDoesNotExposeLegacyTopLevelFilters|SnapshotListOptsMatchJsAndPythonRequestFieldShape|SandboxApiOptsOnlyExposeRequestLevelFields|SandboxLifecycleAndNetworkOptionalBooleanShapesMatchJsAndPython|SandboxApiCreateSandboxOmitsAllowPublicTrafficWhenUnset|SandboxApiCreateSandboxRejectsAutoResumeWithoutPauseLifecycle|ResolveSandboxApiConnectionConfigMergesHeadersWithOverrides|ResolveSandboxApiConnectionConfigAllowsExplicitFalseDebugOverride|GoDocSurfaceMatchesAlignedExportNames)' -count=1
go test . -run 'Test(SandboxWrapperMethodsUseNarrowJsOptionShapes|IsRunningUsesPerCallRequestTimeoutOverride|IsRunningHonorsPreCanceledSignalContext|IsRunningHonorsSignalContext)' -count=1
go test ./template . ./scripts/live_parity_crosscheck -run 'Test(AssignTagsAcceptsSingleTagLikeJsAndPython|RemoveTagsAcceptsSingleTagLikeJsAndPython|TemplateApisHonorCanceledContext|TemplateApisHonorPreCanceledSignalContext|TemplateApisHonorInFlightCancellation|TemplateApisHonorSignalContext|TemplateControlPlaneHelperShapesMatchJsAndPython|TemplateBuildOptionShapesIncludeSharedConnectionFields|RootTemplateFunctionSignaturesAreAvailable|RootAliasesExposeJsStyleTemplateHelpers)' -count=1
go test ./commands -run 'Test(CommandHandleStateSnapshotsLiveOutputAndCopiesExitCode|RunBackgroundWaitAccumulatesOutputAndInvokesCallbacks|HandleProcessEventReplacesInvalidUTF8|CommandStartOptsMatchJsAndPythonBackgroundSurface|RunUsesForegroundSemanticsWithoutBackgroundFlag|RunUsesBackgroundFlagWhenRequested)' -count=1
go test ./scripts/live_parity_crosscheck -count=1
go test . -run 'Test(GoDocSurfaceMatchesAlignedExportNames|RootAliasesExposeJsStyleFilesystemCommandAndGitTypes|RootAliasesExposeJsStyleVolumeTypes)' -count=1
go test ./... -run '^$' -count=1
go test -tags=integration -run '^$' -count=1 .
go test ./... -count=1
```

Additional direct JS verification run this turn:

```bash
node --input-type=module ... Sandbox.create('claude-code-interpreter') ... commands.run(numpy script)
node --input-type=module ... Volume.create(...).makeDir('/multi-file-dir')
node --input-type=module ... Template.buildInBackground(Template().fromImage('ubuntu:22.04').skipCache() ...)
set -a && source /data/e2b-go-sdk/.env && set +a && bun -e "import { Template } from '/data/E2B/packages/js-sdk/src/index.ts'; const template = Template().fromBaseImage(); const info = await Template.build(template, 'base', { cpuCount: 1, memoryMB: 1024, requestTimeoutMs: 600000 }); console.log(JSON.stringify(info));"
bun -e ... Template.build(Template().fromBaseImage(), ..., { cpuCount: 1, memoryMB: 512, requestTimeoutMs: 600000 })
bun -e ... Sandbox.create(tempTemplate).createSnapshot() ... Sandbox.deleteSnapshot(snapshotId)
bun -e ... Sandbox.create(tempTemplate).updateNetwork({ denyOut: ['8.8.8.8'] })
bun -e ... Sandbox.create(tempTemplate, { network: { denyOut: [ALL_TRAFFIC], allowOut: ['1.1.1.1'] } }).updateNetwork({})
bun -e ... Sandbox.create(tempTemplate, { network: { allowOut: ['httpbin.e2b.team'], denyOut: [ALL_TRAFFIC], rules: { ... } } })
bun -e ... Sandbox.create(tempTemplate, { allowInternetAccess: false }) ... commands.run(curl connectivitycheck)
bun -e ... Sandbox.create(tempTemplate, { network: { allowPublicTraffic: false } }) ... fetch(host with/without token)
bun -e ... Sandbox.create(tempTemplate, { network: { allowPublicTraffic: true, maskRequestHost: 'custom-host.example.com:${PORT}' } }) ... fetch(host) ... cat headers
cd /data/E2B/packages/js-sdk && bun install
set -a && source /data/e2b-go-sdk/.env && ./node_modules/.bin/vitest run tests/sandbox/abortSignal.test.ts tests/template/abortSignal.test.ts
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/sandbox/host.test.ts tests/sandbox/metrics.test.ts
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/sandbox/files/*.test.ts
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/api/*.test.ts tests/sandbox/*.test.ts tests/sandbox/**/*.test.ts
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/template/*.test.ts
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/template/methods/*.test.ts
cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/template/utils/*.test.ts
set -a && source /data/e2b-go-sdk/.env && export E2B_INTEGRATION_TEST=1 && set +a && cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/integration/randomness.test.ts
bash /data/e2b-go-sdk/scripts/live_parity_crosscheck.sh randomness js
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/sandbox/pty/*.test.ts
set -a && source /data/e2b-go-sdk/.env && set +a && bun -e "... Template.build(Template({ fileContextPath: tmp }).fromBaseImage().copy('folder/*','folder',{forceUpload:true}).runCmd('cat folder/test.txt').setWorkdir('/app').setStartCmd('echo \\\"Hello, world!\\\"', waitForTimeout(10000)), ..., { cpuCount: 1, memoryMB: 1024, skipCache: true, requestTimeoutMs: 300000 })"
set -a && source /data/e2b-go-sdk/.env && set +a && bun -e "... Template.build(Template().fromBaseImage(), ..., { cpuCount: 1, memoryMB: 1024, requestTimeoutMs: 300000 })"
cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/connectionConfig.test.ts
cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/sandbox/configPropagation.test.ts
bun -e "import { Volume } from '/data/E2B/packages/js-sdk/src/volume/index.ts'; import { VolumeConnectionConfig } from '/data/E2B/packages/js-sdk/src/volume/client.ts'; const volume = new Volume('vol-1','name','token','example.test',false); const config = new VolumeConnectionConfig(volume, {}); console.log(JSON.stringify({keys:Object.keys(volume).sort(), configKeys:Object.keys(config).sort(), apiUrl:config.apiUrl, headers:config.headers, requestTimeout:config.requestTimeoutMs ?? null}))"
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/sandbox/git/*.test.ts
```

Additional direct Python verification run this turn:

```bash
/tmp/e2b-python-sdk-venv/bin/python ... Sandbox.create('claude-code-interpreter') ... commands.run(numpy script)
/tmp/e2b-python-sdk-venv/bin/python ... Volume.create(...).make_dir('/multi-file-dir')
/tmp/e2b-python-sdk-venv/bin/python ... Template.build_in_background(Template().from_image('ubuntu:22.04').skip_cache() ..., cpu_count=1, memory_mb=512)
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/sync/sandbox_sync/test_host.py tests/sync/sandbox_sync/test_metrics.py -q
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/sync/sandbox_sync/files -q
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/sync/api_sync tests/sync/sandbox_sync -q
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/api_async tests/async/sandbox_async -q
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/sandbox_async/test_metrics.py -q
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/sandbox_async/test_network.py -q
/tmp/e2b-python-sdk-venv/bin/python -m pip install pytest-timeout
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/sync/template_sync -q
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/sync/template_sync/methods -q
cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/shared/template/utils -q
bash /data/e2b-go-sdk/scripts/live_parity_crosscheck.sh randomness python
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/sync/sandbox_sync/pty -q
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/volume_async/test_volume.py -q
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/volume_async/test_file.py -q
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/template_async/test_background_build.py -vv -s
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. timeout --foreground 900s /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/template_async/test_build.py -vv -s
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. timeout --foreground 300s /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/template_async/test_build.py::test_build_template_from_base_template -vv -s
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. timeout --foreground 300s /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/template_async/methods/test_from_dockerfile.py tests/async/template_async/methods/test_to_dockerfile.py -q
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. timeout --foreground 300s /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/template_async/methods/test_run_cmd.py::test_run_command -vv -s
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. timeout --foreground 300s /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/template_async/methods/test_make_symlink.py::test_make_symlink -vv -s
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. timeout --foreground 600s /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/sync/template_sync/methods/test_run_cmd.py::test_run_command -vv -s
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/template_async/test_build.py::test_build_template -vv -s
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/template_async/methods/test_run_cmd.py::test_run_command -vv -s
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/template_async/methods/test_make_symlink.py::test_make_symlink -vv -s
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/sync/template_sync/methods/test_run_cmd.py::test_run_command -vv -s
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/sync/template_sync/methods/test_make_symlink.py::test_make_symlink -vv -s
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/template/methods/runCmd.test.ts --reporter=verbose
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/template/methods/makeSymlink.test.ts --reporter=verbose
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/js-sdk && ./node_modules/.bin/vitest run tests/template/build.test.ts -t '^build template$' --reporter=verbose
set -a && source /data/e2b-go-sdk/.env && set +a && PYTHONPATH=/data/E2B/packages/python-sdk /tmp/e2b-python-sdk-venv/bin/python - <<'PY'
... Template.build(Template(file_context_path=tmp).from_base_image().copy('folder/*','folder', force_upload=True).run_cmd('cat folder/test.txt').set_workdir('/app').set_start_cmd(\"echo 'Hello, world!'\", wait_for_timeout(10000)), ..., cpu_count=1, memory_mb=1024, skip_cache=True, request_timeout=300)
PY
set -a && source /data/e2b-go-sdk/.env && set +a && PYTHONPATH=/data/E2B/packages/python-sdk /tmp/e2b-python-sdk-venv/bin/python - <<'PY'
... Template.build(Template().from_base_image(), ..., cpu_count=1, memory_mb=1024, request_timeout=300)
PY
cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/test_connection_config.py -q --noconftest
cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/sync/sandbox_sync/test_config_propagation.py -q
cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/async/sandbox_async/test_config_propagation.py -q
PYTHONPATH=/data/E2B/packages/python-sdk /tmp/e2b-python-sdk-venv/bin/python - <<'PY'
from e2b.volume.volume_sync import Volume
v = Volume('vol-1', 'name', 'token', 'example.test', False)
config = v._get_volume_config()
print({
    'attrs': sorted(vars(v).keys()),
    'api_url': config.api_url,
    'request_timeout': config.request_timeout,
})
PY
set -a && source /data/e2b-go-sdk/.env && set +a && cd /data/E2B/packages/python-sdk && PYTHONPATH=. /tmp/e2b-python-sdk-venv/bin/python -m pytest tests/shared/git -q
```

Reusable repo-local blocker cross-check commands added this turn:

```bash
bash scripts/live_parity_crosscheck.sh claude
bash scripts/live_parity_crosscheck.sh claude_derived
bash scripts/live_parity_crosscheck.sh config_headers
bash scripts/live_parity_crosscheck.sh randomness
bash scripts/live_parity_crosscheck.sh randomness_alias
bash scripts/live_parity_crosscheck.sh metrics
bash scripts/live_parity_crosscheck.sh network_egress
bash scripts/live_parity_crosscheck.sh network_update_payload
bash scripts/live_parity_crosscheck.sh network_rules
bash scripts/live_parity_crosscheck.sh volume_api_payload
bash scripts/live_parity_crosscheck.sh template_api_payload
bash scripts/live_parity_crosscheck.sh template_methods
bash scripts/live_parity_crosscheck.sh template_timeout
bash scripts/live_parity_crosscheck.sh volume
bash scripts/live_parity_crosscheck.sh ubuntu
```

Current-turn live blocker revalidation on 2026-05-30:
- `bash scripts/live_parity_crosscheck.sh claude` still returns `env_blocked` in Go, JS, and Python with `ModuleNotFoundError: No module named 'numpy'`
- `bash scripts/live_parity_crosscheck.sh claude_derived` now returns `ok` in Go, JS, and Python by building a temporary template from `claude-code-interpreter`, installing `numpy` with `--break-system-packages`, and confirming same-sandbox and cross-sandbox NumPy vectors differ
- `bash scripts/live_parity_crosscheck.sh config_headers` now shows a real upstream-language divergence on sandbox API header propagation rather than a Go-only gap:
  - Go returns `ok` with `x_test=base`, `x_extra=1`, and preserved `User-Agent`
  - Python returns `ok` with the same merged header result
  - JS returns `partial` with `x_test=` and `x_extra=1`, meaning the current `pause(...)` API path replaces base headers with per-call headers instead of merging them
- `bash scripts/live_parity_crosscheck.sh randomness_alias` now returns `ok` in Go, JS, and Python against the exact upstream JS integration alias `en716jw99aj63v1k8ugh`, using the same `python -c "import numpy as np; ..."` command shape as `tests/integration/randomness.test.ts`
- `bash scripts/live_parity_crosscheck.sh volume` still returns the same `Path /multi-file-dir not found` blocker in Go, JS, and Python
- `bash scripts/live_parity_crosscheck.sh volume_api_payload` now returns `ok` in Go, JS, and Python:
  - `GET/POST /volumecontent/{volumeID}/dir`, `GET /volumecontent/{volumeID}/path`, `PATCH /volumecontent/{volumeID}/path`, `GET/PUT /volumecontent/{volumeID}/file`, and `DELETE /volumecontent/{volumeID}/path` request shapes align across all three SDKs on the local capture path
  - the same probe exposed a real Go-only wire bug that is now fixed: Go previously sent `Content-Type: application/json` on volume content requests that had no request body (`GET`, `DELETE`, and bodyless `POST /dir`), while JS and Python omitted that header
  - explicit `force=false`, metadata query params, `Authorization: Bearer <token>`, JSON metadata-patch bodies, and octet-stream file-upload bodies now also match across all three SDKs on that local probe
- `bash scripts/live_parity_crosscheck.sh metrics` still returns `partial` in all three SDKs: unfiltered metrics exist, inclusive filtered windows return zero items, and the recorded raw inclusive-window control-plane queries still return zero rows too
- `bash scripts/live_parity_crosscheck.sh network_update_payload` now returns `ok` in Go, JS, and Python:
  - selector-based update bodies match across all three SDKs, including `allowOut=["httpbin.e2b.team"]`, `denyOut=["0.0.0.0/0"]`, transformed `rules`, and `allow_internet_access=false`
  - explicit-empty update bodies now also match across all three SDKs as `{"allowOut":[],"denyOut":[],"rules":{}}`
- `bash scripts/live_parity_crosscheck.sh network_rules` still returns `env_blocked` in Go, JS, and Python because the reflected `X-E2B-Test-Token` header is still empty
- `bash scripts/live_parity_crosscheck.sh network_egress` still returns the same aligned case matrix in Go, JS, and Python:
  - `allow_precedence_8888`, `update_before_8888`, and `clear_after_8888` still return `ok:302`
  - `allow_only_1111`, `allow_only_8888`, `deny_specific_1111`, `deny_specific_8888`, `allow_precedence_1111`, `update_after_deny_1111`, `update_after_deny_8888`, `clear_before_8888`, and `clear_after_1111` still return `exit:35`
- `bash scripts/live_parity_crosscheck.sh template_api_payload` now returns `ok` in Go, JS, and Python:
  - `POST /v3/templates`, `POST /v2/templates/{templateID}/builds/{buildID}`, `GET /templates/aliases/{alias}`, `POST /templates/tags`, `DELETE /templates/tags`, and `GET /templates/{templateID}/tags` request shapes align across all three SDKs on the local capture path
  - the same probe exposed a real Go-only wire bug that is now fixed: Go previously omitted explicit `force:false` on the trigger-build payload and per-step instruction payloads
  - the remaining raw request-shape split is `GET /templates/{templateID}/builds/{buildID}/status`: Go and Python send `logsOffset=3&limit=100`, while JS sends only `logsOffset=3`
- `bash scripts/live_parity_crosscheck.sh template_methods` now returns `ok` in Go, JS, and Python by building one temporary `fromBaseImage()` / `from_base_image()` template per SDK, proving `runCmd(..., user=root)`, `makeSymlink`, and preserved-vs-resolved symlink copy semantics on the stable base-image path, then confirming the same runtime summary from the created sandbox
- `bash scripts/live_parity_crosscheck.sh template_timeout` still shows the remaining default-timeout base-image build instability is not Go-only, but the exact failure mode is highly unstable:
  - Go now polls build status with `logsOffset+limit=100`; on one same-day rerun it failed quickly after ~5.4s with a backend image-config fetch `connection reset by peer`, and on a second rerun it returned `env_blocked` after ~100.7s with `Client.Timeout exceeded while awaiting headers`
  - JS still returns `env_blocked` after ~73s with `TimeoutError: The operation timed out.` while polling with `logsOffset`
  - Python succeeded once earlier the same day after ~428s on that omitted-timeout shape, but a later rerun with `LIVE_PARITY_TIMEOUT_SEC=900` did not return an SDK result in time and is now recorded as `env_blocked` with `failure_kind=runner_timeout`
- `bash scripts/live_parity_crosscheck.sh ubuntu` still returns `env_blocked` in Go, JS, and Python with `error waiting for provisioning sandbox: exit status: 1` at step `base`

Observed results:
- Go paginator tests now cover:
  - per-call request-option overrides on `SandboxPaginator.NextItems` / `SnapshotPaginator.NextItems`
  - in-flight per-call `Signal` cancellation on sandbox and snapshot paginator page fetches
  - canceled page fetches do not poison later pagination calls
- Go `volume/volume_test.go` now directly covers the Python `test_volume_connection_config.py` cases for:
  - `E2B_VOLUME_API_URL` env override
  - explicit `apiUrl` priority over env
  - debug-mode local default from env
  - custom-domain default from env
- Go volume content request shapes now align more closely with JS/Python too:
  - `volume/client.go` now omits `Content-Type` on bodyless volume content requests instead of unconditionally sending `application/json`
  - `volume/volume_file_parity_test.go` now pins the current JS/Python content-type split directly: no `Content-Type` for bodyless `GET` / `DELETE` / `POST dir`, `application/json` for metadata patch, and `application/octet-stream` for file upload
  - the same-environment `volume_api_payload` cross-check now captures those aligned request shapes in Go, JS, and Python without depending on the currently blocked live volume-content backend
- Go now applies `BuildOptions.SkipCache` like JS/Python:
  - before this fix, Go exposed the field but did not propagate it into the serialized template build payload
  - `template/template.go` now sets the whole-template `force` flag for both `Build(...)` and `BuildInBackground(...)` when `BuildOptions.SkipCache` is true
  - `template/template_alignment_test.go` now pins both public build paths at the request-payload level
- Go template control-plane defaults now align more closely with JS/Python too:
  - `template/template.go` now resolves template API credentials/API URL from env when explicit `BuildOptions` / `GetBuildStatusOptions` fields are omitted
  - template API clients now default to `60000ms` request timeouts when those options omit `RequestTimeoutMs`, while still preserving explicit zero timeout
  - omitted template build `memoryMB` now defaults to `1024` instead of the previous Go-only `512`
  - `template/template_alignment_test.go` now pins env fallback, default timeout, explicit zero timeout, and default build memory
- Go template control-plane helper surface now aligns more closely with JS/Python too:
  - `template.ConnectionOpts` now carries only shared connection/request fields, without build-only fields such as `Alias`, `Tags`, `CpuCount`, `MemoryMB`, `SkipCache`, or `OnBuildLogs`
  - `template.Exists(...)`, `AliasExists(...)`, `AssignTags(...)`, `RemoveTags(...)`, and `GetTags(...)` now use that connection-only surface instead of `BuildOptions`
  - root `e2b.Exists(...)`, `AssignTags(...)`, `RemoveTags(...)`, and `GetTags(...)` now use the shared root `ConnectionOpts`
  - `GetBuildStatus(...)` now accepts `*BuildInfo` instead of separate template/build IDs, matching the upstream build-info contract more closely
  - `template.ConnectionOpts`, `BuildOptions`, and `GetBuildStatusOptions` now also use `*bool` for `Debug`, which is closer to the upstream optional shared-connection shape
  - `template/template_alignment_test.go` and `template_aliases_test.go` now pin both subpackage and root-package helper signatures
- Go template trigger-build payloads now align more closely with JS/Python too:
  - `template/build_api.go` now preserves explicit `force:false` on both the top-level trigger-build payload and per-step instruction payloads instead of omitting them via `omitempty`
  - `template/template_alignment_test.go` now pins that request shape directly, and the same-environment `template_api_payload` cross-check now captures the fixed trigger-build body in Go, JS, and Python
- Go template build-status polling now aligns more closely with JS/Python too:
  - `template/build_api.go` now repolls every `200ms` instead of the previous Go-only `2s`
  - `template/build_api.go` now advances `logsOffset` on every poll even when `logger == nil`, matching the current JS/Python source behavior
  - `template/template_alignment_test.go` now pins both the repoll cadence and the logger-free `logsOffset` advancement
- Go template logger/tag behavior now aligns more closely with JS/Python too:
  - `template/template.go` no longer silently falls back to `DefaultBuildLogger()` when the caller omitted `OnBuildLogs`
  - when a logger is provided, `template/template.go` now emits the same control-plane lifecycle messages the current JS/Python sources emit for build/build-in-background paths
  - `template/logger.go` now ignores `debug` entries by default in `DefaultBuildLogger()`, matching the current JS/Python default logger path more closely
  - `template/build_api.go` / `template/logger.go` now strip ANSI escape sequences from mapped build log entries and `LogEntry.String()` output, matching the shared upstream logger behavior more closely
  - `template/build_api.go` now reads `tags` from the `/v3/templates` response and `template/template.go` no longer sends a Go-only extra `/templates/tags` request after `Build(...)`
  - `template/template_alignment_test.go` now pins the lifecycle log sequence, the absence of implicit default logging, default-logger debug filtering, and the no-extra-assign-tags behavior
- Go template file-ignore helper semantics now align more closely with JS/Python too:
  - `template/utils.go` no longer treats slashless ignore patterns like `.env` or `temp*` as basename matches at any depth
  - slashless ignore patterns now stay root-relative, while nested matching still requires explicit path/glob segments such as `**/*.spec.*`
  - `template/template_utils_alignment_test.go` now pins nested-dotfile, nested-wildcard, and current-directory (`.`) cases so this does not regress
- Go Dockerfile parsing now aligns more closely with JS/Python too:
  - `template/dockerfile_parser.go` now expands multi-source `COPY`/`ADD` instructions into one `COPY` step per source path, matching the current JS/Python parser behavior
  - `template/dockerfile_parser.go` / `template/template.go` now also reject Dockerfiles with no `FROM` instruction and multi-stage Dockerfiles at builder time, matching current JS/Python parser semantics more closely
  - `template/dockerfile_parser.go` now also parses multi-pair `ENV` lines and `ARG` instructions into the same `ENV`-style builder steps JS/Python currently produce
  - `template/template_alignment_test.go` now pins plain multi-source `COPY`, multi-source `COPY --chown`, missing-`FROM`, multi-stage rejection, and representative `ENV`/`ARG` parsing
- Go sandbox command behavior is now directly proven against the current upstream suites too:
  - same-workspace JS `tests/sandbox/commands/*.test.ts` now pass directly
  - same-workspace Python `tests/sync/sandbox_sync/commands` now pass directly
  - Go already covers the same runtime behaviors through `TestLiveCommands`, `TestLiveCommandOptions`, `commands/commands_test.go`, and `commands/command_handle_test.go`
  - the remaining command difference is now public surface only: Go `Run(...)` now matches the upstream single-entry/background-on-main-call shape more closely, and it no longer uses a bare `any` return, but Go still cannot literally express JS/Python overload signatures and instead narrows the split through an internal interface; handle state is closer too because Go now exposes a live `State()` snapshot without the extra getter layer, a named exported state type, or a duplicate `pid` field, but it still does not literally mirror JS property access or Python sync iteration
- Go `Sandbox.IsRunning(...)` now also matches the current JS instance contract more closely:
  - the instance opts shape now exposes both `RequestTimeoutMs` and `Signal`, matching the JS `Pick<ConnectionOpts, 'requestTimeoutMs' | 'signal'>` shape more closely
  - focused Go tests now pin per-call timeout override plus pre-canceled and in-flight `Signal` cancellation for `IsRunning(...)`
- Go now covers sandbox network update parity with direct unit and live evidence:
  - `sandbox_test.go` covers `PUT /sandboxes/{sandboxID}/network` request shape, not-found wrapping, signal forwarding, and exported surface shape
  - `sandbox_test.go` also covers network `rules` request/response serialization on create/info/update paths
  - `sandbox_test.go` now also covers selector callback resolution and invalid selector-type rejection for create/update network payloads
  - Go now exposes the upstream-style `SandboxNetworkInfo` return type separately from input option types
  - focused sandbox create/info tests now also pin optional `allowPublicTraffic` / `autoResume` field shapes, omitted-vs-explicit-false request behavior, and the JS/Python `autoResume` validation/default-body contract
  - `TestLiveSandboxUpdateNetwork` now passes live for:
    - adding a deny rule to a running sandbox
    - clearing existing deny/allow rules with an empty update body
- Go `TestLiveSnapshots` now passes for the main upstream snapshot API behaviors:
  - create snapshot
  - global and per-sandbox list
  - named snapshot metadata
  - restore into multiple sandboxes
  - filesystem preservation and branch isolation
  - first `DeleteSnapshot` returns `true`, second `DeleteSnapshot` returns `false`
- Go `TestLiveSandboxInternetAccess` now passes live:
  - default sandbox outbound `curl` returns `204`
  - explicit `allowInternetAccess=true` outbound `curl` returns `204`
  - `allowInternetAccess=false` blocks outbound `curl` with `CommandExitError`
- Go `TestLiveSandboxHost` now directly covers the upstream host reachability semantics more completely:
  - a running sandbox host returns `200`
  - fetching `getHost(...)` after killing the sandbox returns `502` with JSON `{"message":"The sandbox was not found", "code":502, "sandboxId":...}`
- Go `TestLiveFilesystem` now mirrors the upstream JS/Python sandbox file tests more directly:
  - relative single-file writes return `/home/user/...`
  - nested relative writes auto-create parent directories
  - `WriteFiles(...)` with relative paths returns normalized `/home/user/...` metadata and preserves returned order/name/type
  - absolute/nested `WriteFiles(...)` paths now also assert exact returned metadata instead of only non-empty readback
  - gzip text and byte readback, missing-file typed errors plus deprecated `NotFoundError`, and empty-file reads remain covered live
- Go `TestLiveSandboxPublicTraffic` now passes live:
  - `allowPublicTraffic=false` gives `403` without token and `200` with token
  - `allowPublicTraffic=true` gives `200` without token
  - `maskRequestHost` is enforced and the captured `Host` header contains the configured host plus port
- Go `signature_test.go` now directly covers the upstream JS signing helper semantics:
  - exact static `GetSignature` golden output matches the JS source test
  - expiration-bearing signatures preserve the same raw-field ordering and base64 formatting as JS
  - returned expirations are still validated as real future timestamps rather than fixed literals
- Go sandbox file-URL serialization now matches the upstream JS string shape exactly:
  - direct sandbox file URLs preserve `username` before `path`
  - root tests now pin the exact `downloadUrl('/hello.txt')` string used by the JS config-propagation suite instead of only comparing parsed query values
- Go connection-config URL behavior now matches the upstream JS hosted-domain split:
  - `GetSandboxUrl` returns the stable `https://sandbox.<domain>` host for supported hosted domains
  - `GetSandboxDirectUrl` keeps the direct per-sandbox host for those same domains
  - custom/non-hosted domains still use the direct per-sandbox host for both
  - explicit `SandboxUrl` overrides and debug localhost behavior are pinned for both methods
  - `newSandboxFromResponse` now threads the stable host into envd transport while keeping direct file URLs on the per-sandbox host
  - direct upstream JS `tests/connectionConfig.test.ts` passes in the same workspace
  - current Python source and tests still keep the direct per-sandbox host only, so this closes a JS-specific parity gap rather than a shared JS/Python one
- Go sandbox config propagation now preserves inherited headers on per-call overrides:
  - instance sandbox API helpers merge per-call `Headers` over inherited connection headers instead of replacing them wholesale
  - this preserves inherited headers such as Go’s `User-Agent` and any caller-provided base headers while still letting per-call keys override same-name values
  - direct Go `Sandbox.Pause(...)` tests now pin inherited `apiKey`/headers without overrides plus per-call `apiKey`/`requestTimeoutMs`/header override precedence on the actual instance API path
  - direct Go `Sandbox.UpdateNetwork(..., opts)` tests now also pin instance-method `Signal` propagation at the same boundary covered by the upstream JS config-propagation suite
  - focused helper-level Go unit coverage still pins the lower-level merge and optional-field semantics
  - same-workspace JS `tests/sandbox/configPropagation.test.ts` passes
  - current Python sandbox config-propagation tests are stricter than JS here and now pass directly in the same workspace: Python preserves base headers and appends override headers, while JS currently only proves non-header overrides plus per-call header replacement semantics
- Go volume file/list semantics now match the upstream JS/Python query behavior more closely:
  - `Volume.list(path)` no longer injects `depth=1` by default; `depth` is now omitted unless the caller sets it, matching JS/Python request semantics
  - Go mock parity tests now pin empty-file reads, directory `getInfo`, and default-depth omission directly
- Go volume instance config persistence now matches the current JS/Python object/config contract more closely:
  - `Create` / `Connect` now persist only `volumeId`, `name`, `token`, `domain`, and `debug` on the Go `Volume`
  - later instance content calls no longer inherit create/connect-time `ApiUrl`, `Headers`, `RequestTimeoutMs`, `Proxy`, `Logger`, or `Signal`
  - Go `Volume.Debug` plus volume content `Debug` opts now also use `*bool`, so omitted vs explicit `false` is preserved on the content path like the current JS/Python volume sources
  - focused Go coverage now pins create-time header/timeout non-persistence, nil-safe `Connect(..., nil)` / `resolveClient(nil)`, and explicit `debug=false` override behavior against env/persisted debug state
  - same-workspace JS runtime output shows `new Volume(...)` exposes only `["debug","domain","name","token","volumeId"]`, while `new VolumeConnectionConfig(volume, {})` derives a default `apiUrl` and does not surface persisted `headers` or `requestTimeoutMs`
  - same-workspace Python runtime/source evidence shows `Volume(...)` stores only `_volume_id`, `_name`, `_token`, `_domain`, and `_debug`, and `_get_volume_config()` still takes `api_url`, `request_timeout`, `headers`, and `proxy` only from per-call opts/defaults
- Go `TestLiveFileSigning` now directly covers the upstream secure sandbox/file-signing cases more completely:
  - regular secure `Files.Write(...)` / `Files.Read(...)` now also assert `/home/user/hello.txt` metadata and readback before signed URL checks
  - secure sandbox reconnect still works
  - secure filesystem watch still works
  - command execution on a secure sandbox returns the expected stdout
  - signed downloads now pass both without expiration and with a valid expiration
  - expired signed downloads still return `401`
  - root-user signed downloads still return the expected content
  - signed uploads now pass for both the default user path and the root-user path
  - expired signed uploads still return `401`
- Go metrics coverage now includes the upstream `memCache` field:
  - `SandboxMetrics` now exposes `MemCache`
  - `sandbox_test.go` verifies `memCache` mapping from the current API response shape
- Go `TestLiveSandboxLifecycle/metrics` now checks more than “any metrics exist”:
  - unfiltered metrics must return data first
  - then an inclusive filtered window around the returned metric timestamp is queried
  - if that filtered window still returns zero items, the test skips with the exact metric timestamp and queried window so the backend limitation is explicit
- Go `internal/shared/sdk_context_test.go` now directly covers the shared context-merging semantics behind option-carried cancellation:
  - pre-canceled secondary context
  - in-flight secondary cancellation
  - explicit merged cancel function
  - earliest-deadline wins
- Go signal-cancellation tests now cover representative in-flight cancellation for:
  - commands / PTY request and stream setup paths
  - filesystem REST, RPC, and watch startup paths
  - git command dispatch through `commands.Run`
  - volume control-plane and content/file request paths
- Go public sandbox/template APIs now also cover pre-canceled option-carried cancellation explicitly:
  - sandbox `Create`, `Kill`, and `UpdateNetwork` with pre-canceled `Signal`
  - template `Build`, `BuildInBackground`, `Exists`, `GetBuildStatus`, `AssignTags`, `RemoveTags`, and `GetTags` with pre-canceled `Signal`
  - these mirror the upstream JS `AbortSignal` “already aborted” cases more directly than the earlier shared-helper coverage alone
- `bash scripts/live_parity_crosscheck.sh claude` now reproduces the same blocker across all three SDKs:
  - Go: `ModuleNotFoundError: No module named 'numpy'`
  - JS: `ModuleNotFoundError: No module named 'numpy'`
  - Python: `ModuleNotFoundError: No module named 'numpy'`
- `bash scripts/live_parity_crosscheck.sh randomness` now proves the same NumPy randomness semantics across all three SDKs:
  - Go: `ok`, same-sandbox and cross-sandbox NumPy vectors differ on a temporary Python+NumPy template
  - JS: `ok`, same-sandbox and cross-sandbox NumPy vectors differ on a temporary Python+NumPy template
  - Python: `ok`, same-sandbox and cross-sandbox NumPy vectors differ on a temporary Python+NumPy template
  - a later same-day rerun still returned `ok` in Go and JS; Python first hit a transient envd `i/o timeout` on one run and then returned `ok` on an immediate rerun, so the repo-local cross-check remains useful but is not completely immune to environment noise
- `bash scripts/live_parity_crosscheck.sh randomness_alias` now proves the exact upstream alias path is healthy across all three SDKs:
  - Go: `ok`, same-sandbox and cross-sandbox random vectors differ on `en716jw99aj63v1k8ugh`
  - JS: `ok`, same-sandbox and cross-sandbox random vectors differ on `en716jw99aj63v1k8ugh`
  - Python: `ok`, same-sandbox and cross-sandbox random vectors differ on `en716jw99aj63v1k8ugh`
  - this isolates the remaining issue to the intermittent unmodified upstream JS test path rather than to the alias contents or a cross-language runtime mismatch
- `bash scripts/live_parity_crosscheck.sh metrics` now shows:
  - Go: unfiltered metrics return data, but an inclusive filtered window around the returned metric timestamp still returns zero items
  - Go raw control-plane request for that same inclusive window also returns zero rows
  - JS: after correcting the local metrics request to use query params, unfiltered metrics still return data but real filtered windows now also return zero items
  - Python: unfiltered metrics return data, but an inclusive filtered window around the returned metric timestamp still returns zero items
  - Python raw control-plane request for that same inclusive window also returns zero rows
  - this shows the remaining metrics issue is a backend/environment limitation, not a current Go-only mismatch
- `bash scripts/live_parity_crosscheck.sh network_rules` now shows the same backend limitation across all three SDKs:
  - Go: sandbox creates successfully, but `httpbin.e2b.team/headers` reflects an empty `X-E2B-Test-Token`
  - JS: sandbox creates successfully, but the same reflected header is empty
  - Python: sandbox creates successfully, but the same reflected header is empty
  - this shows create-time `network.rules` header transforms are not currently enforced in this backend, rather than exposing a Go-only mismatch
- `bash scripts/live_parity_crosscheck.sh volume` now reproduces the same blocker across all three SDKs:
  - Go: `Path /multi-file-dir not found`
  - JS: `Path /multi-file-dir not found`
  - Python: `Path /multi-file-dir not found`
- `bash scripts/live_parity_crosscheck.sh ubuntu` now reproduces the same blocker across all three SDKs:
  - Go final build status: `env_blocked`, reason `error waiting for provisioning sandbox: exit status: 1`, step `base`
  - JS final build status: `env_blocked`, reason `error waiting for provisioning sandbox: exit status: 1`, step `base`
  - Python final build status: `env_blocked`, reason `error waiting for provisioning sandbox: exit status: 1`, step `base`
- `bash scripts/live_parity_crosscheck.sh template_timeout` now probes the same base-image copy/run/start build family that the upstream JS/Python build helpers exercise, with request-timeout overrides intentionally omitted:
  - Go now polls with `logsOffset+limit=100` like the current Python sync/async generated client path instead of `logsOffset` alone
  - Go: one same-day rerun failed quickly after ~5.4s with `build failed: error getting image config file ... read: connection reset by peer`, and a second rerun still returned `env_blocked` after ~100.7s with `Client.Timeout exceeded while awaiting headers`
  - JS: `env_blocked` after ~73s with `TimeoutError: The operation timed out.` while polling with `logsOffset`
  - Python: `ok` once earlier the same day after ~428s on that omitted-timeout build shape, then `env_blocked` on a later rerun when the shell harness hit `LIVE_PARITY_TIMEOUT_SEC=900`
  - `scripts/live_parity_crosscheck.sh` now emits structured `runner_timeout` results instead of hanging indefinitely when one language run exceeds the configured wall timeout
  - this keeps the core conclusion the same: the remaining default-timeout build instability is not Go-only, and the current backend/runtime mix is too unstable to treat any one of these results as a clean Go-specific signal
- `claude-code-interpreter` JS command fails with `ModuleNotFoundError: No module named 'numpy'`
- `claude-code-interpreter` Python command fails with `ModuleNotFoundError: No module named 'numpy'`
- Go `TestLiveClaudeCodeInterpreterRandomness` skips for the same reason
- Go `TestLiveTemporaryPythonNumpyTemplateRandomness` passes and proves NumPy random vectors differ within one sandbox and across sandboxes created from the same template in this environment
- direct upstream JS `tests/integration/randomness.test.ts` now reaches its hardcoded template alias `en716jw99aj63v1k8ugh`, but the same-sandbox case is intermittent here: repeated unmodified runs can either fully pass or fail with `SandboxError: 2: [unknown] terminated`
- same-template manual JS/Go/Python repros using `python -c "import numpy as np; print([...])"` now all succeed repeatedly, so the remaining source-level gap is flaky direct-source evidence rather than a missing template or a Go-only runtime failure
- current Python upstream tests do not include a dedicated randomness source suite
- direct upstream JS `tests/template/utils/*.test.ts`: 50 passed, 3 skipped
- direct upstream Python `tests/shared/template/utils`: 33 passed, 1 skipped
- direct upstream JS `tests/sandbox/pty/*.test.ts`: 4 passed
- direct upstream Python `tests/sync/sandbox_sync/pty`: 4 passed
- JS volume `makeDir('/multi-file-dir')` fails with `Path /multi-file-dir not found`
- Python volume `make_dir('/multi-file-dir')` fails with `Path /multi-file-dir not found`
- Go `TestLiveVolumeLifecycle/file_operations_lifecycle` skips for the same reason
- Go `TestLiveTemplateBuildMethodParityOnBaseImage` passes for representative upstream method cases on `e2bdev/base`:
  - `runCmd` as root
  - invalid build user rejection
  - `makeSymlink`
  - `makeSymlink(force)`
  - symlink copy preserving link target
  - symlink copy with `ResolveSymlinks`
- Go `volume/volume_file_parity_test.go` now covers representative upstream volume file semantics under a local mock content server:
  - text/bytes stream reads
  - empty-file reads
  - metadata and `force` query propagation for `writeFile` and `makeDir`
  - directory `getInfo`
  - default vs explicit `depth` query semantics for `list`
  - `updateMetadata` request/response semantics
  - `list(depth)`
  - `remove`
- Go volume error coverage now also explicitly mirrors the upstream negative file-operation cases:
  - `list('/non-existent')` -> `NotFoundError`
  - `updateMetadata('/non-existent.txt')` -> `NotFoundError`
  - `makeDir('/missing-parent/child')` backend `404` -> `NotFoundError`
  - `remove('/non-existent.txt')` -> `NotFoundError`
- upstream JS `Template.buildInBackground(... fromImage('ubuntu:22.04') ...)` reaches final build status `error` with reason `error waiting for provisioning sandbox: exit status: 1`
- Python `Template.build_in_background(... from_image('ubuntu:22.04') ..., cpu_count=1, memory_mb=512)` reaches final build status `error` with reason `error waiting for provisioning sandbox: exit status: 1`
- Go build logs for the same ubuntu-template family show the same provisioning-layer failure mode, including mirror/certificate/package-resolution errors
- same-environment JS temporary-template checks now show:
  - `Sandbox.deleteSnapshot(snapshotId)` returns `true` first and `false` on the second delete
  - `allowInternetAccess: true` returns `204`
- upstream JS source abort-signal suites now run directly in this environment after installing package dependencies:
  - `./node_modules/.bin/vitest run tests/sandbox/abortSignal.test.ts tests/template/abortSignal.test.ts`
  - all 12 tests pass
  - this provides direct upstream-source confirmation for the cancellation behaviors that Go now mirrors with `context.Context` plus option-carried `Signal`
  - the specific JS paginator case `Sandbox.list(...).nextItems({ signal })` is now also covered directly in Go unit tests via per-call `Signal` cancellation on `SandboxPaginator.NextItems(...)`
- a local `base` template alias was built successfully in this account from `Template().fromBaseImage()`, so the upstream JS/Python sandbox source tests now run unmodified in this environment
- upstream template source suites now show a mix of true parity and shared environment/time-budget limits:
  - JS `tests/api/info.test.ts`, `tests/api/list.test.ts`, `tests/api/kill.test.ts`, and `tests/api/snapshot.test.ts` all pass directly in the same workspace
  - Python `tests/sync/api_sync` passes directly in the same workspace
  - JS `tests/api/http2.test.ts`, `tests/api/inflight.test.ts`, `tests/api/validateApiKey.test.ts`, `tests/envd/http2.test.ts`, `tests/connectionConfig.test.ts`, and `tests/sandbox/configPropagation.test.ts` all pass directly in the same workspace
  - Python `tests/test_api_client_transport.py`, `tests/test_validate_api_key.py`, `tests/test_connection_config.py`, `tests/test_volume_connection_config.py`, and `tests/e2b_connect/test_client.py` all pass directly in the same workspace when run without the custom environment variables that would intentionally override their default-value assertions
  - JS `tests/template/abortSignal.test.ts`, `exists.test.ts`, `backgroundBuild.test.ts`, `stacktrace.test.ts`, and `uploadFile.test.ts` pass directly
  - Python `tests/sync/template_sync/test_tags.py` passes directly
  - JS `tests/template/build.test.ts` and the integration cases inside `tests/template/tags.test.ts` fail under the default 60s request timeout with `The operation was aborted due to timeout`; a fresh targeted rerun of `./node_modules/.bin/vitest run tests/template/build.test.ts -t '^build template$' --reporter=verbose` also timed out after ~72.5s while still uncompressing `e2bdev/base` layers
  - Python `tests/sync/template_sync/test_build.py` can fail on the same base-image build path with backend/internal build errors in this environment, and the full sync template suite times out or fails in the same family of cases
  - direct JS/Python template `methods` suites also fail in the same shared area: a fresh rerun of JS `tests/template/methods/runCmd.test.ts` timed out all three cases after ~73s, `tests/template/methods/makeSymlink.test.ts` timed out both cases after ~73s, fresh Python sync/async `tests/*/template_*/methods/test_run_cmd.py::test_run_command` reruns fail in ~22s with `error waiting for provisioning sandbox: exit status: 1`, Python sync `tests/sync/template_sync/methods/test_make_symlink.py::test_make_symlink` hits pytest-timeout at `180s` while polling build status, and Python async `tests/async/template_async/methods/test_make_symlink.py::test_make_symlink` also hits pytest-timeout at `180s`
  - these are not current Go-only implementation misses
- same-environment manual template probes narrow the cause further:
  - JS `Template.build(...)` for a base-image `skipCache` upload/runCmd template succeeds when `requestTimeoutMs` is raised to `300000`, but the same shape fails in the upstream source suite under the default `60000`
  - Python `Template.build(...)` for the same base-image `skip_cache` shape succeeds with `request_timeout=300`
  - the repo-local `template_methods` cross-check now also returns `ok` in Go, JS, and Python on the stable base-image path, proving representative `runCmd`, `makeSymlink`, and symlink-copy semantics in the same environment even though the upstream `ubuntu:22.04` source suites still fail
  - the repo-local `template_timeout` cross-check now also shows the same omitted-timeout base-image copy/run/start shape timing out in Go and JS in this environment, while Python succeeded once on that same shape after ~428s
  - Go now also shows the same `skipCache` behavior after the fix on longer-timeout probes: the same base-image `skipCache` build path remains expensive enough that timeout budget materially affects whether the synchronous path completes in this environment, proving the whole-build force path is now exercised instead of being silently ignored
- upstream JS source sandbox suites now run directly against that local `base` alias:
  - `tests/sandbox/files/*.test.ts`: 11 files passed, 49 tests passed
  - `tests/sandbox/host.test.ts`: passes
  - `tests/sandbox/metrics.test.ts`: fails the filtered-window assertion (`expected 0 to be greater than 0`)
  - `tests/sandbox/network.test.ts`: six failures limited to outbound allow/deny/update reachability and transform enforcement
  - combined `tests/api/*.test.ts tests/sandbox/*.test.ts tests/sandbox/**/*.test.ts`: 53/55 files passed, 190/197 tests passed; only `metrics.test.ts` and `network.test.ts` fail
- upstream Python sync sandbox suites now also run directly against that same local `base` alias:
  - `tests/sync/sandbox_sync/files`: 43 passed
  - `tests/sync/sandbox_sync/test_host.py`: passes
  - `tests/sync/sandbox_sync/test_metrics.py`: fails the same filtered-window assertion
  - `tests/sync/sandbox_sync/test_network.py`: fails the same outbound allow/deny/update reachability and transform cases
  - combined `tests/sync/api_sync tests/sync/sandbox_sync`: 116 passed, 7 failed; only `test_metrics.py` and `test_network.py` fail
- upstream Python async API/sandbox suites now also run directly in the same workspace:
  - `tests/async/sandbox_async/test_metrics.py`: fails the same filtered-window assertion
  - `tests/async/sandbox_async/test_network.py`: fails the same outbound allow/deny/update reachability and transform cases
  - combined `tests/async/api_async tests/async/sandbox_async`: 114 passed, 7 failed; only `test_metrics.py` and `test_network.py` fail
- upstream Python async template source sampling now also narrows the remaining async risk:
  - `pytest-timeout` is now installed in the current audit venv, so upstream `timeout` config/marks are active for these runs
  - `tests/async/template_async/test_background_build.py`: passes
  - `tests/async/template_async/test_build.py::test_build_template_from_base_template`: passes in ~29s
  - `tests/async/template_async/test_build.py::test_build_template`: now fails at pytest-timeout `180s` after the backend build starts and stalls while uncompressing `e2bdev/base` layers
  - `tests/async/template_async/methods/test_from_dockerfile.py` plus `tests/async/template_async/methods/test_to_dockerfile.py`: 9 passed
  - `tests/async/template_async/methods/test_run_cmd.py::test_run_command`: now fails in ~22s with `error waiting for provisioning sandbox: exit status: 1`
  - `tests/async/template_async/methods/test_make_symlink.py::test_make_symlink`: now hits pytest-timeout at `180s`
  - `tests/sync/template_sync/methods/test_run_cmd.py::test_run_command`: now fails in ~22s with that same `BuildException`, so the remaining direct-source block is still shared template-build instability rather than an async-only mismatch
- upstream volume source suites now also run directly in the same workspace:
  - JS `tests/volume/volume.test.ts`: 8 passed
  - JS `tests/volume/file.test.ts`: 22 failed, 4 passed, 1 skipped; the failing cases all report `Path ... not found`
  - Python `tests/sync/volume_sync/test_volume.py`: 8 passed
  - Python `tests/sync/volume_sync/test_file.py`: 22 failed, 4 passed, 1 skipped; the failing cases likewise report `Path ... not found`
  - Python `tests/async/volume_async/test_volume.py`: 8 passed
  - Python `tests/async/volume_async/test_file.py`: 22 failed, 4 passed, 1 skipped; the failing cases likewise report `Path ... not found`
  - this separates current volume control-plane parity from the shared live content/backend block with direct upstream-source evidence instead of only ad hoc repros
- same-environment JS temporary-template checks still show:
  - `allowInternetAccess: false` blocks outbound `curl` with `CommandExitError` / exit status `28`
  - `allowPublicTraffic: false` gives `403` without token and `200` with token
  - `allowPublicTraffic: true` gives `200` without token
  - `maskRequestHost` works and captures `Host: custom-host.example.com:8082`
  - `Sandbox.updateNetwork({ denyOut: ['8.8.8.8'] })` denies `8.8.8.8`
  - `Sandbox.updateNetwork({})` clears prior deny/allow rules and re-allows `8.8.8.8`
  - the stronger `after update, 1.1.1.1 stays reachable` assertion is not stable in this backend for JS either, so the earlier Go live failure on that assertion was not a Go-only mismatch
  - create-time `network.rules` header injection against `httpbin.e2b.team` is not enforced in this environment for JS either, and the repo-local Go/JS/Python `network_rules` cross-check now shows the reflected header is empty in all three SDKs
- `bash scripts/live_parity_crosscheck.sh network_egress` now reproduces the same same-template egress outcome across Go, JS, and Python:
  - all three return final status `env_blocked`
  - `allow_only_1111`, `allow_only_8888`, `deny_specific_8888`, `deny_specific_1111`, `allow_precedence_1111`, `update_after_deny_8888`, `update_after_deny_1111`, and `clear_before_8888` all fail in the same way (`exit:35`)
  - `allow_precedence_8888`, `update_before_8888`, and `clear_after_8888` all succeed in the same way (`ok:302`)
  - this shows the remaining direct-source `network` failures are backend/environment-wide rather than Go-only

## High-Signal Matrix

| Source area | Status | Go evidence | Notes |
|---|---:|---|---|
| JS/Python API key validation | covered | `api/client.go`, `api/client_test.go` | Fixed this turn; previously missing in Go. Direct upstream JS `tests/api/validateApiKey.test.ts` and Python `tests/test_validate_api_key.py` now also pass in the same workspace. |
| JS `api/handleApiError`, Python API error mapping | covered | `api/client_test.go` | Status/body handling aligned. |
| JS/Python sandbox create/connect/kill/info/timeout/config | covered | `live_integration_test.go`, `sandbox_test.go`, `connection_config_test.go` | Current env confirms core lifecycle paths. Go now also matches the current JS/Python create-lifecycle contract more closely: request-side `SandboxLifecycle.AutoResume` is optional, `create` always emits `autoResume.enabled` with default `false`, and `autoResume=true` is rejected unless timeout resolves to `pause`. Direct upstream JS `tests/api/info.test.ts`, `tests/api/list.test.ts`, `tests/api/kill.test.ts`, and `tests/api/snapshot.test.ts` now also pass directly in the same workspace, and Python `tests/sync/api_sync` passes directly too. One source-level detail is upstream-language-divergent rather than Go-only: JS currently returns `boolean` from `Sandbox.pause(...)`, while Python `Sandbox.pause(...)` returns `None`, so Go's boolean `Pause(...)` result is not itself a standalone JS+Python contract miss. |
| JS/Python sandbox filesystem read/write/encoding | covered / partial | `live_integration_test.go`, `filesystem/filesystem_test.go`, `errors_test.go` | Go now directly covers relative-path `/home/user` normalization, overwrite, nested-parent auto-creation, exact `WriteFiles(...)` metadata/order for relative and absolute paths, gzip text/bytes/blob reads, missing-file `FileNotFoundError` plus deprecated `NotFoundError` compatibility, empty-file reads, secure sandbox read/write, empty `WriteFiles(...)` noop, transport/error edge cases, the direct read/write opts field set, default multipart uploads on newer envd, explicit octet-stream uploads only when the caller opts in, direct `string` / `[]byte` / Go-side `Blob` / `io.Reader` write inputs for both single-file and multi-file paths, and a primary single-entry `Read(...)` surface that defaults to text and switches return shape through `FilesystemReadOpts.Format` for `text` / `bytes` / `blob` / `stream`. Direct upstream JS `tests/sandbox/files/*.test.ts` and Python `tests/sync/sandbox_sync/files` now also pass against the locally built `base` alias. The remaining `partial` is now mostly literal API shape plus upstream-language divergence: JS `files.read(...)` already uses `opts.format` and exposes `text` / `bytes` / `blob` / `stream`, while Python sync/async `read(...)` use a named `format` parameter and only type `text` / `bytes` / `stream`. Go therefore leans closer to JS on the read-entry shape, but it still represents the JS browser-native payload families through Go-native types (`Blob`, `[]byte`, `io.Reader`) instead of literal `Blob` / `ArrayBuffer` / `ReadableStream` runtime objects. |
| JS/Python snapshot API / snapshot restore semantics | covered | `live_integration_test.go`, `sandbox_test.go`, `surface_audit_test.go` | Go now directly covers create, named snapshot, global and per-sandbox listing, restore into one or multiple sandboxes, filesystem preservation/isolation, and delete semantics where the second `DeleteSnapshot` returns `false`, matching JS/Python. |
| JS/Python template build/background build/exists/tags | covered / partial | `live_integration_test.go`, `template/template_alignment_test.go`, `template/template_utils_alignment_test.go`, `scripts/live_parity_crosscheck/*` | Go now directly proves live build success, `BuildInBackground` in-progress status, positive `FromTemplate` builds, positive/negative `Exists`, positive tag flows, invalid tag-format rejection, applies `BuildOptions.SkipCache` to whole-template builds like JS/Python, resolves template API config from env like JS/Python when explicit options are omitted, defaults template request timeouts to 60s when omitted, preserves explicit zero timeout, defaults template build `memoryMB` to `1024` like JS/Python, uses connection-only control-plane opts for `Exists`/`AssignTags`/`RemoveTags`/`GetTags`, accepts `*BuildInfo` in `GetBuildStatus(...)` instead of split IDs, repolls build status at the same `200ms` cadence family as JS/Python, advances `logsOffset` even when no logger is attached like JS/Python, no longer silently enables a default build logger when the caller omitted one, emits the same control-plane lifecycle log messages JS/Python emit when a logger is provided, strips ANSI escape sequences from mapped build log entries like the upstream logger paths, consumes `tags` from the `/v3/templates` response like JS/Python, no longer sends a Go-only extra `/templates/tags` request after `Build(...)`, preserves explicit `force:false` on trigger-build payloads like JS/Python, and keeps slashless copy-source ignore patterns root-relative instead of over-matching nested basenames while also keeping `.`-pattern file listing inside the provided context. Direct upstream JS `exists`/`backgroundBuild`/`stacktrace`/`abortSignal`/`uploadFile` and Python `test_tags.py` pass in the same workspace. A same-environment `bash scripts/live_parity_crosscheck.sh template_api_payload` probe now also captures the local control-plane request sequence without backend involvement: Go, JS, and Python align on `POST /v3/templates`, trigger-build, exists, assignTags/removeTags/getTags request shapes, and that probe caught the fixed Go-only omission of explicit `force:false` at both the top-level trigger-build payload and per-step instruction payloads. The one remaining raw request-shape difference on that local probe is `getBuildStatus`: Go and Python send `logsOffset+limit=100`, while JS sends only `logsOffset`. Same-day live rechecks after that alignment still did not produce a stable success story: Go once failed quickly with a backend image-config fetch reset and once timed out after ~100.7s, JS still timed out after ~73s, and Python produced one success earlier the same day but later exceeded a 900s runner wall timeout. The remaining partial is therefore still environment/timing proof rather than a current Go-only implementation miss. |
| JS/Python template method parity (`runCmd`, `makeSymlink`, symlink copy/resolve) | covered / env-skip | `live_integration_test.go`, `template/template_alignment_test.go`, `template/template_utils_alignment_test.go`, `scripts/live_parity_crosscheck/*` | Representative upstream method behaviors are now proven live on `e2bdev/base` both in Go integration tests and in a same-environment repo-local `bash scripts/live_parity_crosscheck.sh template_methods` run that returns `ok` in Go, JS, and Python. That stable-path cross-check builds one temporary `fromBaseImage()` / `from_base_image()` template per SDK, verifies `runCmd(..., user=root)`, `makeSymlink`, and preserved-vs-resolved symlink copy semantics during build, then confirms the same runtime sandbox summary afterward. Go’s Dockerfile parser coverage now also matches the current JS/Python multi-source `COPY`/`COPY --chown` expansion semantics, builder-time rejection of missing-`FROM`/multi-stage Dockerfiles, and representative `ENV`/`ARG` parsing at the unit level. Fresh direct-source reruns still show the same shared block on the upstream ubuntu-image path: JS `tests/template/methods/runCmd.test.ts` timed out all three cases, JS `tests/template/methods/makeSymlink.test.ts` timed out both cases, Python sync/async `test_run_cmd.py::test_run_command` now fail with provisioning/build-status errors, and Python sync/async `test_make_symlink.py::test_make_symlink` now hit pytest-timeout at `180s`. The remaining direct-source block is therefore environmental rather than a current Go-only mismatch. |
| JS/Python template stacktrace / caller-trace semantics | covered | `live_integration_test.go`, `template/template.go`, `template/build_api.go`, `template/template_alignment_test.go`, `template/template_utils_alignment_test.go` | Go now captures Go-native caller attribution for logical template steps, including caller-directory default file-context resolution, builder-time invalid-copy rejection with `BuildError.CallerTrace`, and backend-reported build-step failures with mapped caller traces. It does not reproduce JS/Python raw runtime stack formats, but the originating builder-method attribution is now directly covered. Direct upstream JS `tests/template/stacktrace.test.ts` passes in the same workspace. |
| JS/Python template utils / file-ignore / caller-directory helpers | covered | `template/utils.go`, `template/template_utils_alignment_test.go` | Go now directly covers current file-ignore root-relative behavior, caller-directory default file-context resolution, and related template helper semantics. Direct upstream JS `tests/template/utils/*.test.ts` now pass in the same workspace (50 passed, 3 skipped), and Python `tests/shared/template/utils` now also pass directly (33 passed, 1 skipped). |
| JS randomness integration / NumPy randomness semantics | covered / partial | `TestLiveTemporaryPythonNumpyTemplateRandomness`, `TestLiveRandomness`, `scripts/live_parity_crosscheck/*` | Go directly proves same-sandbox and cross-sandbox NumPy randomness via a temporary Python+NumPy template built in the current environment, and the repo-local `bash scripts/live_parity_crosscheck.sh randomness` case generally returns `ok` for Go, JS, and Python with that same semantic check. A new repo-local `bash scripts/live_parity_crosscheck.sh randomness_alias` case now also returns `ok` in Go, JS, and Python against the exact upstream JS integration alias `en716jw99aj63v1k8ugh`, using the same `python -c "import numpy as np; ..."` command shape as `tests/integration/randomness.test.ts`. The upstream JS file itself is therefore the remaining weak point: repeated unmodified runs can pass fully, but the same-sandbox case is intermittent and can also fail with `SandboxError: 2: [unknown] terminated` on the second command. One later repo-local Python rerun on the temporary-template path also hit a transient envd `i/o timeout` before passing on immediate retry. The remaining `partial` is therefore still flaky direct-source or environment proof rather than a template-missing or Go-only runtime gap. The current Python upstream test tree still does not include a dedicated randomness suite. |
| JS/Python sandbox metrics | covered / partial | `live_integration_test.go`, `sandbox_test.go`, `api/schema.go`, `scripts/live_parity_crosscheck/*` | Go now exposes the upstream `MemCache` field, unit-tests current/legacy metric response mapping, and live-tests unfiltered metrics plus an inclusive filtered window around the returned metric timestamp. Same-environment cross-checks show unfiltered metrics are available in Go, JS, and Python. Go’s raw control-plane request for the inclusive window returns zero rows, Python’s raw control-plane request now does the same, and after correcting the local JS metrics request to use query params, JS matches the same zero-row filtered-window behavior. Direct upstream JS `tests/sandbox/metrics.test.ts` plus Python `tests/sync/sandbox_sync/test_metrics.py` and `tests/async/sandbox_async/test_metrics.py` now also run against the local `base` alias and fail the same filtered-window assertion. The remaining gap is environment proof: the backend does not currently honor filtered metrics windows in this environment. |
| JS/Python sandbox network update | covered | `sandbox.go`, `sandbox_api.go`, `sandbox_test.go`, `live_integration_test.go`, `scripts/live_parity_crosscheck/*` | Go now exposes root/static and instance `UpdateNetwork`, preserves upstream request semantics for omitted fields clearing existing rules, and passes focused live update-network coverage. A new same-environment `bash scripts/live_parity_crosscheck.sh network_update_payload` probe now also captures the local `PUT /sandboxes/{sandboxID}/network` wire body in Go, JS, and Python without backend involvement; selector-based bodies match across all three SDKs, and Go now also preserves explicit empty `allowOut: []`, `denyOut: []`, and `rules: {}` on update requests like JS/Python instead of omitting them. The broader upstream allow/deny/update reachability assertions are currently best evidenced by the same-template `network_egress` cross-check because Go’s focused live test covers a narrower subset than the direct upstream source suites. |
| JS/Python network rules / transform shape | covered / env-skip | `sandbox.go`, `sandbox_api.go`, `sandbox_test.go`, `live_integration_test.go`, `scripts/live_parity_crosscheck/*` | Go now supports `rules` on create/info/update payloads, selector callbacks for `allowOut` / `denyOut`, a separate `SandboxNetworkInfo` return shape, and optional `allowPublicTraffic` omission semantics on both create opts and info payloads with unit coverage. Focused Go request tests now also pin explicit-empty create/update `allowOut` / `denyOut` / `rules` serialization so those fields stay aligned with current JS/Python wire shape instead of being collapsed by Go-side omission rules. Direct upstream JS `tests/sandbox/network.test.ts` plus Python `tests/sync/sandbox_sync/test_network.py` and `tests/async/sandbox_async/test_network.py` now run against the local `base` alias and fail the same transform and egress/update reachability categories. Same-environment Go/JS/Python `network_rules`, `network_egress`, and `network_update_payload` cross-checks show the remaining live gap is backend/environment proof rather than a current Go-only implementation miss. |
| JS/Python internet/public traffic / host masking | covered | `live_integration_test.go`, `sandbox_test.go` | Go now directly proves `allowInternetAccess` default/true/false behavior, preserves omitted-vs-explicit-false `allowPublicTraffic` on create payloads, and covers `allowPublicTraffic` token gating plus `maskRequestHost` host-header rewriting. Same-environment JS checks pass for the same cases. |
| JS sandbox host reachability / killed-host payload | covered | `live_integration_test.go` | Go `TestLiveSandboxHost` now directly covers the running-host `200` behavior plus the JS-only negative case where fetching `getHost(...)` after kill returns `502` with JSON `message`, `code`, and a sandbox-id prefix. The upstream JS `tests/sandbox/host.test.ts` and Python `tests/sync/sandbox_sync/test_host.py` now also pass directly against the locally built `base` alias. |
| JS/Python secure sandbox + file signing semantics | covered | `live_integration_test.go`, `sandbox_test.go`, `signature_test.go` | Go now directly covers secure sandbox reconnect/watch/command-run behavior, signed download URLs with and without expiration, expired signing rejection, root-user signed download/upload paths, exact upload/download URL query serialization including JS-matching `username`/`path` ordering, and exact `GetSignature` golden output/expiration formatting aligned with the JS source tests. |
| JS/Python sandbox git surface | covered | `live_integration_test.go`, `git/git.go`, `git/git_test.go`, `git/utils_test.go`, `git/signal_test.go` | Go now directly covers the representative upstream git surface: init/add/status/commit/configureUser/getConfig/setConfig/branches/create-branch/checkout/delete-branch/remote-add/remote-get/clone/reset/restore/push/pull/dangerouslyAuthenticate, plus upstream-style auth/upstream error mapping and signal propagation. The remaining deterministic option-shape mismatches called out during audit were fixed this turn: `GitRestoreOpts` now exposes `Paths` like the upstream JS/Python APIs while keeping deprecated `Files` as a compatibility alias, `GitCloneOpts.Depth` now uses `*int` so the public clone-depth shape is optional like JS/Python, and Go no longer exposes the extra `GitCommitOpts.Author`, `GitPushOpts.Force`, or `GitPullOpts.Rebase` fields that upstream JS/Python do not provide. `Commit(...)` now also matches the upstream `authorName` / `authorEmail` flag order. Direct upstream JS `tests/sandbox/git/*.test.ts` and Python `tests/shared/git` now both pass in the same workspace. |
| `claude-code-interpreter` randomness | covered / env-skip | `TestLiveClaudeCodeInterpreterRandomness`, `TestLiveClaudeCodeInterpreterDerivedNumpyTemplateRandomness`, `scripts/live_parity_crosscheck/*` | The current alias exists and already includes `python` / `python3` / `pip`, but it does not currently include `numpy`, so the direct alias still yields the same `ModuleNotFoundError` in Go, JS, and Python. A new Go live test plus the repo-local `claude_derived` cross-check now also prove that Go, JS, and Python can all derive from `claude-code-interpreter`, install `numpy` with `python3 -m pip install --break-system-packages --no-cache-dir numpy`, and observe the same same-sandbox and cross-sandbox randomness semantics. The remaining env-skip is therefore the current alias contents, not a Go-only SDK gap. |
| JS/Python volume control-plane CRUD | covered | `volume/*_test.go`, `TestLiveVolumeLifecycle/control_plane_lifecycle` | Included in current audit. Go volume package `ConnectionOpts.Debug` now also uses `*bool`, and the control-plane config honors `E2B_DEBUG` for the local control-plane API URL like the upstream JS static volume entrypoints and Python volume control-plane calls that go through the shared connection config. Direct upstream JS `tests/volume/volume.test.ts` plus Python `tests/sync/volume_sync/test_volume.py` and `tests/async/volume_async/test_volume.py` now all pass directly in the same workspace. |
| JS/Python volume instance config persistence | covered | `volume/volume.go`, `volume/volume_test.go`, `volume/volume_opts_test.go` | Go `Volume` now persists only the JS/Python-aligned instance fields (`volumeId`, `name`, `token`, `domain`, `debug`). `Volume.Debug` now uses `*bool`, and the volume content debug opts now also use `*bool`, so omitted vs explicit `false` stays distinct on the content path like the current JS/Python volume sources. Create/connect-time transport fields (`apiUrl`, `headers`, `requestTimeoutMs`, `proxy`, `logger`, `signal`) no longer leak into later content calls. Same-workspace JS runtime and Python runtime/source checks confirm that contract. |
| JS/Python volume file-content semantics | covered / env-skip | `volume/*_test.go`, `TestLiveVolumeLifecycle/file_operations_lifecycle`, `scripts/live_parity_crosscheck/*` | Go now has direct unit parity coverage for representative upstream file semantics (`writeFile`, `readFile`, empty-file reads, metadata, `makeDir`, `updateMetadata`, directory `getInfo`, `list`, `list(depth)`, default list-depth omission, `remove`, and explicit not-found mappings). A same-environment `bash scripts/live_parity_crosscheck.sh volume_api_payload` probe now also captures the local volume content wire shapes without depending on the currently broken live backend; Go, JS, and Python align there on list/mkdir/info/update-metadata/read/write/remove request targets, queries, auth headers, and content-type/body behavior. That probe caught and verified the fix for one real Go-only wire bug: Go previously sent `Content-Type: application/json` on bodyless volume content requests while JS and Python omitted it. Direct upstream JS `tests/volume/file.test.ts` plus Python `tests/sync/volume_sync/test_file.py` and `tests/async/volume_async/test_file.py` now also run in the same workspace and fail the same live content cases with `Path ... not found`. The remaining env-skip is therefore only the live backend proof, not a current Go-only implementation miss. |
| JS/Python volume file read/write public surface | covered / partial | `volume/volume.go`, `volume/volume_surface_test.go`, `volume/volume_file_parity_test.go` | Go `WriteFile(...)` now accepts `string`, `[]byte`, Go-side `Blob`, and `io.Reader`, which closes the previous Go-only `io.Reader` restriction and matches the practical Python write-input family more closely while covering the text/bytes/blob/stream family exercised upstream. Go `ReadFile(...)` now also acts as a primary single-entry read surface that defaults to text and switches return shape through `VolumeReadOpts.Format` for `text` / `bytes` / `blob` / `stream`. The extra public volume read helpers were removed, so the exported surface is closer to the upstream single-entry `readFile(...)` family. The remaining `partial` is now mostly literal API shape plus upstream-language divergence: JS `readFile(...)` already uses `opts.format` and exposes `text` / `bytes` / `blob` / `stream`, while Python sync/async `read_file(...)` use a named `format` parameter and only type `text` / `bytes` / `stream`. Go therefore leans closer to JS on the read-entry shape, but it still represents the JS browser-native payload families through Go-native types (`Blob`, `[]byte`, `io.Reader`) instead of literal `Blob` / `ArrayBuffer` / `ReadableStream` runtime objects. |
| JS/Python volume list option surface | covered | `volume/client.go`, `volume/volume.go`, `volume/volume_opts_test.go`, `volume/volume_file_parity_test.go`, `volume_aliases.go`, `surface_audit_test.go` | Go no longer exposes the reflection-based `opts any` list surface. It now uses `VolumeListOpts` (embedded `VolumeApiOpts` plus `Depth`) so list-depth handling is typed and explicit like the upstream JS/Python list APIs, with root-alias and request-shape tests pinning the contract. |
| JS/Python volume write option surface | covered | `volume/types.go`, `volume/volume.go`, `volume/volume_opts_test.go` | Go `VolumeWriteOptions` now carries the same logger-capable connection surface as the upstream JS `VolumeWriteOptions & VolumeApiOpts` composition, and both `Force` and `Debug` now use `*bool` so omission vs explicit `false` / `true` matches the upstream JS optional and Python `Optional[...]` semantics more closely. Focused tests pin the exported field shapes plus the logger/debug/query mapping behavior. |
| Root SDK volume entry surface | covered | `volume_aliases.go`, `volume_aliases_test.go`, `surface_audit_test.go` | Go root package now exposes top-level volume wrappers analogous to the static entrypoints exposed by the JS/Python SDKs, instead of only re-exporting types. |
| JS/Python connection-config URL behavior | covered | `connection_config.go`, `connection_config_test.go`, `sandbox.go`, `sandbox_test.go` | Go now covers API URL defaults/env overrides/arg priority, explicit zero request timeout, direct host formatting, JS hosted-domain stable sandbox host behavior, direct-vs-stable URL split for file URLs vs envd transport, custom-domain fallback, explicit sandbox URL override, and debug localhost behavior. Root `ConnectionOpts.Debug` now also uses `*bool`, which is closer to the upstream optional debug field shape while preserving the current JS/Python root-connection runtime behavior. Same-workspace JS `tests/connectionConfig.test.ts` passes. Same-workspace Python `tests/test_connection_config.py` and `tests/test_volume_connection_config.py` also pass when run without the custom env overrides that intentionally replace their default-value expectations. Python’s current source/test surface is narrower here and still uses the direct per-sandbox host for sandbox transport/file URLs. |
| JS/Python sandbox list/snapshot-list option surface | covered | `sandbox.go`, `sandbox_api.go`, `sandbox_test.go`, `paginator_test.go` | Go `SandboxApiOpts`, `SandboxListOpts`, and `SnapshotListOpts` now use `*bool` for `Debug`, and `SandboxListOpts` / `SnapshotListOpts` no longer expose a Go-only list-level `Signal` field. This matches the upstream JS `Omit<SandboxApiOpts, 'signal'>` list option shapes more closely, and also matches the Python list/list-snapshots API style where cancellation is not a dedicated list-option field. Per-page cancellation still remains available through paginator `NextItems(...)` overrides with `SandboxApiOpts{Signal: ...}`, matching the upstream JS paginator contract more closely. |
| JS/Python sandbox command run/connect/list/kill/send-stdin/env-var semantics | covered | `live_integration_test.go`, `commands/commands_test.go`, `commands/command_handle_test.go`, `commands/signal_test.go` | Go directly covers the same user-visible command behaviors as the current upstream JS/Python suites: foreground run output, broken UTF-8 replacement, timeout errors, background process listing, background connect/kill/not-found handling, stdin streaming, scoped/global env vars, cwd/user overrides, and handle wait/exit semantics. Direct same-workspace JS `tests/sandbox/commands/*.test.ts` and Python `tests/sync/sandbox_sync/commands` now also pass unmodified. |
| JS/Python command-start / background-run public surface | covered / partial | `commands/commands.go`, `commands/commands_test.go`, `commands/command_handle_test.go`, `public_surface_external_test.go`, `live_integration_test.go` | Go `CommandStartOpts` now uses a single `Stdin` option field and no longer carries the extra `StdinOpt` compatibility knob. Go `CommandStartOpts` now also exposes `Background bool`, and Go `Run(...)` now acts as the primary single-entry command surface: it returns a foreground `*CommandResult` by default and a background `*CommandHandle` when `opts.Background` is true, through an explicit internal `commandExecution` interface instead of a bare `any` return. Go no longer exports the extra `RunForeground(...)`, `RunBackground(...)`, and `RunWithMode(...)` helpers, so the public surface is closer to the upstream single-entry shape. JS and Python are not literally identical here either: JS carries `background` inside the `run(cmd, opts)` object, while Python exposes `background` as a named `run(...)` parameter, so Go is structurally closer to JS on the option-carrier side. Go command handles now expose a live `State()` snapshot (`stdout` / `stderr` / `exitCode` / `error`) without the extra getter layer, a named exported `CommandHandleState` type, or a duplicate `pid` field, which is closer to the JS and async-Python property-reading style because `pid` already lives on `CommandHandle` itself. External-package tests now also prove that `Run(...)` remains usable across package boundaries despite the unnamed helper types: callers can still type-assert to `*CommandResult` / `*CommandHandle` and read `handle.State()` fields from outside the SDK package. The remaining `partial` is now mostly literal type-shape: JS and Python express the return split through overloads/union typing on one `run(...)` API, while Go still cannot literally express those overloads and Python sync still relies on iteration rather than a shared property contract. |
| JS/Python PTY create/connect/resize/input surface | covered | `commands/pty.go`, `commands/pty_test.go`, `live_integration_test.go`, `export_aliases.go`, `surface_audit_test.go` | Go now exposes `PtySize` and `(*Pty).Resize(ctx, pid, size, opts)` like the upstream JS/Python size-object API instead of the previous split `cols` / `rows` parameters, while still covering create/connect/reconnect/send-input semantics. Focused Go tests pin both the direct public surface and the nested `pty.size` request serialization, live PTY coverage still exercises resize end-to-end, and direct upstream JS `tests/sandbox/pty/*.test.ts` plus Python `tests/sync/sandbox_sync/pty` now both pass in the same workspace. |
| JS/Python sandbox config propagation | covered / partial | `sandbox.go`, `sandbox_test.go`, `scripts/live_parity_crosscheck/*` | Go now preserves inherited sandbox connection headers when per-call overrides are provided, so per-call `Headers` no longer drop inherited base headers or Go’s `User-Agent`. Root Go tests also pin the real `Sandbox.Pause(...)` instance path both without overrides and with per-call `apiKey`/`requestTimeoutMs`/header overrides, and they pin instance `Sandbox.UpdateNetwork(..., opts)` signal propagation at the same method boundary covered by the upstream JS config-propagation suite. Same-workspace JS `tests/sandbox/configPropagation.test.ts` passes for the fields it asserts, and same-workspace Python `tests/sync/sandbox_sync/test_config_propagation.py` and `tests/async/sandbox_async/test_config_propagation.py` also pass and explicitly require merged base+override headers. A new repo-local `bash scripts/live_parity_crosscheck.sh config_headers` probe makes the current runtime split explicit: Go and Python both send merged `X-Test=base` plus `X-Extra=1` on `pause(...)`, while JS currently sends only `X-Extra=1`, dropping the inherited `X-Test` header on that API path. The remaining `partial` is therefore an upstream-language divergence rather than a Go-only gap. |
| Python volume-connection-config URL precedence tests | covered | `volume/volume_test.go` | Go now covers default domain construction, env overrides, arg priority, debug-mode local URL selection, and env-domain derived volume API URLs. |
| JS/Python abort-signal semantics | covered / partial | `sandbox_test.go`, `template/template_alignment_test.go`, `commands/signal_test.go`, `filesystem/signal_test.go`, `git/signal_test.go`, `volume/signal_test.go`, `internal/shared/sdk_context_test.go`, `paginator.go`, `paginator_test.go` | Go now covers already-canceled and in-flight cancellation for sandbox create/kill/updateNetwork/paginator, template build/buildInBackground/exists/getBuildStatus/assignTags/removeTags/getTags, commands/PTY request paths, representative filesystem request/watch paths, git command dispatch, representative volume control-plane/content paths, and the shared merged-context helper semantics that back option-carried cancellation. Sandbox/snapshot paginators now also honor per-call `Signal` on `NextItems(...)`, which closes the concrete JS paginator abort gap. Direct same-workspace JS source verification now also passes for the upstream abort-signal suites: `bunx vitest run tests/sandbox/abortSignal.test.ts tests/template/abortSignal.test.ts`. Python does not currently expose a dedicated abort-signal source suite. The remaining difference is therefore no longer a missing behavior proof path so much as a public cancellation-form difference: Go uses `context.Context`/`NextItemsContext` rather than JS `AbortSignal` in option objects. |
| JS `api/http2`, `api/inflight`, `envd/http2`; Python transport cache/http2 tests | covered | `internal/shared/http.go`, `internal/shared/http_test.go`, `api/client_test.go`, `envd/rpc_test.go`, `commands/*_test.go`, `filesystem/filesystem_test.go` | Go now has direct parity tests for env var parsing, inflight queue release/cancel behavior, cached transport reuse, HTTP/2 opt-out, proxy wiring, envd REST vs RPC separation, and transient Connect/RPC transport retries. Direct upstream JS `tests/api/http2.test.ts`, `tests/api/inflight.test.ts`, and `tests/envd/http2.test.ts` now also pass in the same workspace. Direct upstream Python `tests/test_api_client_transport.py` and `tests/e2b_connect/test_client.py` pass too when run without the custom env overrides that would intentionally alter their default-value assertions. |
| JS browser runtime connectionConfig | n/a | `connection_config_test.go` | Browser-specific runtime branch is not applicable to Go. |
| JS browser/bun/deno runtime smoke tests | n/a | `live_integration_test.go` | These validate JS package/runtime compatibility in browser-like, Bun, and Deno environments. Go does not target those runtimes; equivalent live sandbox/command/filesystem semantics are already covered in Go integration tests. |
| JS stress/load tests | n/a | operational | `integration/stress.test.ts` exercises load behavior rather than a stable SDK API parity contract. |
| Python async suites | n/a / directly sampled | sync/live equivalents | Go has no async public API surface. High-signal async suites that could still change the parity conclusion have now been run directly in the same workspace: `tests/async/api_async tests/async/sandbox_async` finish with 114 passed and 7 failed, and those failures are limited to the same `metrics` / `network` categories already seen in JS and Python sync. More focused async checks also confirm `tests/async/sandbox_async/test_config_propagation.py` passes, `tests/async/volume_async/test_volume.py` passes, and `tests/async/volume_async/test_file.py` fails with the same `Path ... not found` backend pattern as the sync volume file suite. Async template sampling narrows the remaining async risk further: `tests/async/template_async/test_background_build.py` passes, `tests/async/template_async/test_build.py::test_build_template_from_base_template` passes in ~29s, and pure-construction `tests/async/template_async/methods/test_from_dockerfile.py` plus `test_to_dockerfile.py` pass (9 passed). After installing `pytest-timeout` into the audit venv, build-bearing async cases now fail with stronger upstream-native signals: `tests/async/template_async/methods/test_run_cmd.py::test_run_command` fails in ~22s with `error waiting for provisioning sandbox: exit status: 1`, and `tests/async/template_async/test_build.py::test_build_template` hits pytest-timeout at `180s` while base-image layer uncompress is still in progress. The sync Python `tests/sync/template_sync/methods/test_run_cmd.py::test_run_command` sample now fails with that same provisioning error too, so the remaining async template block still matches the shared template-build instability already seen in sync Python/JS/Go rather than introducing an async-only mismatch. |
| Python `e2b_connect/test_client.py` retry helper | covered | `envd/rpc.go`, `envd/rpc_test.go`, `commands/*_test.go`, `filesystem/filesystem_test.go` | Go now directly covers retry-on-expected-transport-error, no-retry-on-unexpected-error, success-after-retry, and request-path transient transport recovery. |
| Python skipped desktop envelope regression | n/a | skipped upstream | `bugs/test_envelope_decode.py` is currently skipped upstream and depends on the desktop template plus `pyautogui`/X11-specific behavior, so it is not a missing stable Go parity requirement in the current audit scope. |

## Remaining Non-Negligible Gaps

These are the items that still prevent claiming full 100% migration:

1. Abort/cancel parity is only partially proven
   - JS `sandbox/abortSignal.test.ts`
   - JS `template/abortSignal.test.ts`
   - direct same-workspace JS source verification now strengthens this substantially: `bunx vitest run tests/sandbox/abortSignal.test.ts tests/template/abortSignal.test.ts` passes in `/data/E2B/packages/js-sdk`
   - Go now has stable canceled-context coverage, explicit pre-canceled `Signal` coverage on the matching public sandbox/template APIs, representative in-flight cancellation coverage across sandbox/template/commands/PTY/filesystem/git/volume request paths, paginator context support, per-call paginator `Signal` cancellation, and per-call request-option overrides on exported paginators; focused Go sandbox/template signal suites also pass directly
   - Python does not currently carry a dedicated abort-signal source suite, so there is no third-language direct source file to mirror on this exact point
   - the remaining difference is now mostly public cancellation form: Go uses `context.Context`/`NextItemsContext` rather than JS `AbortSignal` inside option objects for create/kill/build-style calls

2. `volume` file-content parity cannot be fully proven live in the current environment
   - unit-level semantics are now broadly covered in Go against a mock content server
   - but the live backend proof is still blocked
   - this is not because Go is uniquely failing
   - direct upstream JS `tests/volume/file.test.ts` and Python `tests/sync/volume_sync/test_file.py` / `tests/async/volume_async/test_file.py` now also fail in the same workspace with the same `Path ... not found` pattern
   - JS and Python fail in the same way against the current backend
   - until an environment exists where these source tests pass live and Go passes too, the migration is not fully proven

3. The upstream `ubuntu:22.04` provisioning path is still environment-blocked
   - upstream JS, Python, and Go all hit provisioning failures for representative `fromImage('ubuntu:22.04')` build flows in this environment
   - representative method semantics themselves are already proven live on `e2bdev/base`, and the repo-local `bash scripts/live_parity_crosscheck.sh template_methods` case now returns `ok` across Go, JS, and Python on that stable path
   - what remains unproven end-to-end is that same build-method family on the upstream ubuntu-image path in the current backend

4. Some upstream template integration cases still depend on environment-specific build timing
   - JS `tests/template/build.test.ts` and the integration cases inside `tests/template/tags.test.ts` still use the default 60s request timeout and currently fail with timeout errors in this environment
   - a fresh targeted rerun of JS `tests/template/build.test.ts -t '^build template$'` timed out after ~72.5s while the build was still uncompressing `e2bdev/base` layers, a fresh rerun of JS `tests/template/methods/runCmd.test.ts` timed out all three cases after ~73s, and `tests/template/methods/makeSymlink.test.ts` timed out both cases after ~73s
   - the repo-local `template_timeout` cross-check now exercises the same base-image copy/run/start build family with request-timeout overrides omitted: Go still hits environment-blocked failures there after aligning its build-status polling to `logsOffset+limit=100`, JS still times out while polling with `logsOffset`, and Python remains variable across same-day runs
   - Python succeeded once earlier the same day after ~428s on that omitted-timeout shape, but a later rerun exceeded a 900s runner wall timeout and is now recorded as `env_blocked` instead of hanging indefinitely
   - direct async/template-method sampling still does not change that conclusion after installing `pytest-timeout` into the audit venv: `tests/async/template_async/test_background_build.py` and `tests/async/template_async/test_build.py::test_build_template_from_base_template` pass, `tests/async/template_async/methods/test_run_cmd.py::test_run_command` fails in ~22s with `error waiting for provisioning sandbox: exit status: 1`, `tests/sync/template_sync/methods/test_run_cmd.py::test_run_command` fails the same way, `tests/async/template_async/methods/test_make_symlink.py::test_make_symlink` and `tests/sync/template_sync/methods/test_make_symlink.py::test_make_symlink` hit pytest-timeout at `180s`, and `tests/async/template_async/test_build.py::test_build_template` also hits pytest-timeout at `180s` while base-image layer uncompress is still in progress
   - current internal polling is therefore no longer Go-vs-Python on this point: Go and Python now both send `limit=100`, while JS still polls with `logsOffset` only
   - manual JS/Python probes still show the same base-image build shapes can succeed when the request timeout is raised substantially
   - Go previously masked one part of this because `BuildOptions.SkipCache` was not actually applied; that real Go gap is now fixed
   - but until these upstream template integration cases pass unmodified in an environment with stable build timing, and ideally under one identical polling shape, full direct-source proof is still incomplete

5. Filtered metrics parity is still not fully explained
   - same-environment cross-checks now show unfiltered metrics are available in Go, JS, and Python
   - Go live integration and the repo-local cross-check both show Go returns zero items even for an inclusive filtered window around the reported metric timestamp
   - the repo-local Go cross-check also shows the raw control-plane request with those same query params returns zero rows
   - Python shows the same inclusive-window behavior as Go, and the repo-local Python raw control-plane request with those same query params also returns zero rows
   - after correcting the local JS metrics request to use query params, JS now matches the same zero-row filtered-window behavior
   - this means the remaining gap is backend/environment proof, not a current Go-only or Go-vs-JS implementation mismatch
   - until the backend honors filtered metrics windows in an environment where upstream tests can pass, metrics parity is not fully proven end-to-end

6. Some network-rule enforcement details are still environment-blocked
   - Go now matches the upstream `rules` payload shape, and same-environment cross-checks succeed in constructing the same rule-bearing sandboxes through Go, JS, and Python
   - but the current backend does not enforce the `httpbin.e2b.team` header-injection transform in any of the three SDKs
   - so create-time/update-time `rules` header transforms are not yet positively proven live end-to-end in this environment

7. Some outbound egress-rule source assertions are still environment-blocked
   - upstream JS `tests/sandbox/network.test.ts` and Python `tests/sync/sandbox_sync/test_network.py` now run directly against the local `base` alias, and both fail the same allow/deny/update reachability cases in this backend
   - the same-template `network_egress` cross-check matches those case-by-case results across Go, JS, and Python
   - Go's focused `TestLiveSandboxUpdateNetwork` covers a narrower subset than the upstream source suites, so the cross-check is currently the stronger proof path for these blocked cases

8. Sandbox filesystem public surface is still not literally identical
   - direct same-workspace JS `tests/sandbox/files/*.test.ts` and Python `tests/sync/sandbox_sync/files` now pass, and Go covers the same runtime filesystem behaviors through `TestLiveFilesystem`, `TestLiveFileSigning`, and `filesystem/*_test.go`
   - upstream JS and Python no longer present one identical read signature here: JS uses a single `files.read(path, opts?.format)` overload family with `text` / `bytes` / `blob` / `stream`, while Python sync/async use `read(path, format=..., user=..., request_timeout=..., gzip=...)` with `text` / `bytes` / `stream`
   - Go now also uses a single `Read(...)` entry for that role, with `FilesystemReadOpts.Format` selecting `text` / `bytes` / `blob` / `stream`, so the public shape leans closer to JS than Python
   - Go filesystem writes now accept `string`, `[]byte`, Go-side `Blob`, and `io.Reader`, and `Read(..., blob)` now returns that Go-side `Blob` helper, but Go still does not literally use JS browser-native `Blob` / `ArrayBuffer` / `ReadableStream` runtime objects and Python still keeps a different named-parameter/no-`blob` read surface
   - this is now an explicit public-surface divergence rather than an untested runtime behavior gap

9. Sandbox command public surface is still not literally identical
   - direct same-workspace JS `tests/sandbox/commands/*.test.ts` and Python `tests/sync/sandbox_sync/commands` now pass, and Go covers the same runtime behaviors through `Run(...)`, `Connect(...)`, `List(...)`, `Kill(...)`, `SendStdin(...)`, and handle wait semantics
   - JS `commands.run(cmd, opts?.background)` and Python `commands.run(cmd, background=...)` are already not literally identical upstream; Go `Run(...)` now also acts as the primary single command entry, with `CommandStartOpts.Background` selecting foreground vs background behavior, so the option-carrier shape leans closer to JS
   - the remaining difference is now mostly return-type and handle-state shape: Go no longer uses a bare `any`, but it still cannot literally express JS/Python overload/union typing on one `run(...)` API
   - Go command handles now expose a live `State()` snapshot and no longer carry the extra getter-method layer, a named exported state type, or a duplicate `pid` snapshot field, which narrows the gap for JS-style and async-Python-style state reads, but upstream is still not internally uniform there: JS exposes live result properties on the handle, Python sync mainly exposes `pid` plus iteration/`wait(...)`, and Python async adds result-property accessors
   - this is now an explicit public-surface divergence rather than an untested behavior gap

10. Volume file public surface is still not literally identical
   - direct same-workspace JS `tests/volume/file.test.ts` and Python `tests/sync/volume_sync/test_file.py` cover related but not identical read families: JS `readFile(path, { format })` exposes `text` / `bytes` / `blob` / `stream`, while Python sync/async `read_file(path, format=...)` type only `text` / `bytes` / `stream`
   - Go `WriteFile(...)` now accepts `string`, `[]byte`, Go-side `Blob`, and `io.Reader`, which removes the previous Go-only `io.Reader` restriction
   - Go `ReadFile(...)` now also acts as the primary single read entry, with `VolumeReadOpts.Format` selecting `text` / `bytes` / `blob` / `stream`
   - Go does not literally use JS browser-native `Blob` / `ArrayBuffer` / `ReadableStream` runtime objects, and Python still keeps a different named-parameter/no-`blob` read surface even though Go and JS both use a single read entry with format-selected return shapes
   - this is now an explicit public-surface divergence in addition to the current live-backend block on volume content operations

11. Direct upstream randomness source proof is only partial in this workspace
   - Go now proves the user-visible same-sandbox and cross-sandbox NumPy randomness behavior through `TestLiveTemporaryPythonNumpyTemplateRandomness`
   - repo-local `bash scripts/live_parity_crosscheck.sh randomness` now proves the same semantics across Go, JS, and Python using temporary Python+NumPy templates
   - repo-local `bash scripts/live_parity_crosscheck.sh randomness_alias` now also proves the exact upstream alias `en716jw99aj63v1k8ugh` behaves the same way across Go, JS, and Python with the same `python -c "import numpy as np; ..."` command shape the JS source test uses
   - a later same-day rerun still returned `ok` in Go and JS, while Python hit one transient envd `i/o timeout` before returning `ok` on an immediate rerun
   - upstream JS `tests/integration/randomness.test.ts` now reaches its hardcoded template alias `en716jw99aj63v1k8ugh`, but the same-sandbox case is intermittent and can fail with `SandboxError: 2: [unknown] terminated` on the second command even though the repo-local alias cross-check now returns `ok` in JS, Go, and Python against that same alias and command shape
   - the current Python upstream test tree does not include a dedicated randomness suite

## Current Environment Limits

Current backend/template limitations observed from direct evidence:

- `claude-code-interpreter` alias exists and already contains `python` / `python3` / `pip`, but its current template environment does not include `numpy`
- direct upstream JS `tests/integration/randomness.test.ts` is no longer blocked by a missing template alias here, but its same-sandbox case is intermittent and can fail with `SandboxError: 2: [unknown] terminated` even though repeated manual same-template repros in JS/Go/Python succeed
- volume content API returns `Path ... not found` for `makeDir` in Go, JS, and Python
- some `ubuntu:22.04` template builds fail during provisioning in Go, JS, and Python with final reason `error waiting for provisioning sandbox: exit status: 1`; Go build logs also show mirror/certificate/package-resolution failures under that provisioning layer
- some base-image template integration cases also exceed default request-timeout budgets in this environment: the repo-local `template_timeout` cross-check now shows Go still hitting environment-blocked failures on the omitted-timeout copy/run/start build shape even after its polling path was aligned to `logsOffset+limit=100`, JS still timing out while polling with `logsOffset`, and Python remaining variable across same-day runs (one success after ~428s, one later runner wall-timeout after 900s). Fresh upstream reruns match that shape: JS `tests/template/build.test.ts -t '^build template$'` times out while still uncompressing `e2bdev/base` layers, Python async `tests/async/template_async/test_build.py::test_build_template` now hits pytest-timeout at `180s` in that same phase, and Python sync/async `test_run_command` samples can also fail sooner with `error waiting for provisioning sandbox: exit status: 1`
- filtered sandbox metrics are currently a backend/environment limitation in this environment: Go, JS, and Python all return zero items for real filtered windows once JS sends `start`/`end` as query params
- outbound egress-rule allow/deny/update reachability assertions are currently backend-limited across Go, JS, and Python even on the shared local `base` alias
- create-time network `rules` header transforms against `httpbin.e2b.team` are currently not enforced in Go, JS, or Python in this environment

## Net Assessment

The Go SDK is materially closer to parity after this turn:
- the latest broad Go regression still passes after the latest filesystem/volume/commands surface tightening: `go test ./... -count=1`
- API key validation parity is now implemented
- transport-specialized API/envd parity is now directly covered in Go unit tests
- Connect/RPC transient transport retry parity now exists as a Go internal helper with request-path tests
- representative in-flight cancellation parity is now covered across sandbox/template/commands/PTY/filesystem/git/volume request paths
- exported sandbox/snapshot paginators now accept per-call request options and honor per-call `Signal`, which closes the concrete JS paginator abort gap and brings `nextItems(...)` closer to the upstream JS paginator shape
- snapshot API parity is now directly covered live, including named snapshots, restore/isolation behavior, and second-delete returning `false`
- template build/background-build/exists/tags parity is now directly covered live, including malformed tag rejection and positive `FromTemplate` builds
- representative upstream template method parity (`runCmd`, `makeSymlink`, symlink copy/resolve) is now directly covered live on a stable base-image path
- repo-local `template_methods` cross-checks now also pass across Go, JS, and Python on that stable base-image path
- caller-directory default file-context behavior now matches JS/Python when `FileContextPath` is omitted
- direct upstream template utility suites now also pass in the same workspace, which strengthens the file-ignore and caller-directory alignment evidence
- invalid template copy-path rejection now happens at the builder call site like JS/Python, not only later during build
- Go-native template caller attribution now points build-time and builder-time failures back to the originating builder call, which closes the practical remaining stacktrace parity target
- general NumPy randomness semantics are now directly proven live in Go via a temporary Python+NumPy template
- repo-local randomness cross-checks now also pass across Go, JS, and Python in the same workspace
- repo-local `claude_derived` cross-checks now also pass across Go, JS, and Python from a template derived from `claude-code-interpreter`
- a new Go live integration test, `TestLiveClaudeCodeInterpreterDerivedNumpyTemplateRandomness`, now also passes directly under `-tags=integration`
- `createLiveSandboxFromTemplate(...)` and `newLiveSandboxWithOpts(...)` now carry merged `liveConnectionOpts()` by default, so representative Go live tests no longer silently fall back to the shorter default request-timeout budget during sandbox creation; fresh reruns of `TestLiveSandboxLifecycle` and `TestLiveSandboxInternetAccess` still pass under `-tags=integration`
- the integration-tag compile still passes after the latest command surface tightening: `go test -tags=integration -run '^$' -count=1 .`
- external-package tests now also prove two previously awkward Go surfaces remain usable without widening the API: subpackage constructors accept a real root `*ConnectionConfig`, and `Run(...)`/`State()` stay consumable across package boundaries even though their helper types remain unnamed
- Go filesystem and volume now also use primary single-entry read surfaces (`Read(...)` / `ReadFile(...)`) with format-selected return shapes, the extra public read helpers were removed, and Go still uses a Go-side `Blob` helper type for the upstream JS `blob` family even though the current Python sync/async read surfaces do not type `blob`
- Go command handles now also expose a live `State()` snapshot without the extra getter-method layer, a named exported state type, or a duplicate `pid` snapshot field, which narrows the handle-state gap with JS/async-Python without pretending the JS property model and Python sync iteration model are literally identical
- Go commands now also use a primary single-entry `Run(...)` surface with `CommandStartOpts.Background` plus an explicit internal execution-interface return, and the extra public command-run helpers were removed so the surface is closer to the upstream single-entry contract
- sandbox metrics surface parity improved further: Go now exposes `MemCache` like JS/Python
- internet access, public traffic gating, and `maskRequestHost` are now directly proven live in the current environment
- sandbox filesystem read/write/encoding parity is now directly covered more closely, including `/home/user` relative-path normalization, exact multi-file metadata/order, gzip text/bytes reads, typed missing-file compatibility, empty-file reads, and secure sandbox read/write
- secure sandbox/file-signing parity is now directly covered more completely, including exact `GetSignature` golden output and root-user signed upload behavior
- sandbox direct file-URL serialization now matches the upstream JS exact-string expectation instead of only matching after query parsing
- connection-config sandbox URL behavior now matches the upstream JS hosted-domain split, with stable envd transport hosts and direct file-URL hosts separated internally in Go
- representative upstream `volume` file semantics are now directly covered in Go unit tests even though the current live backend still blocks content operations
- `Volume.list()` no longer sends an implicit `depth=1`, which removes a real request-shape mismatch with the upstream JS/Python SDKs
- `Volume` instance state now matches the upstream JS/Python contract more closely: Go persists only `volumeId`/`name` plus `token`/`domain`/`debug`, and no longer leaks create/connect-time transport overrides into later content calls
- root Go now exposes volume management wrappers at the SDK top level instead of forcing callers into the `volume` subpackage for the static entry surface
- `claude-code-interpreter` is directly covered by a Go live test
- `volume` is no longer incorrectly excluded from the audit
- current-environment false negatives were separated from real Go gaps by JS and Python cross-checks

But the migration still cannot honestly be called fully 100% proven yet, because:
- the cancellation API still is not literally identical across languages (`context.Context` vs `AbortSignal`)
- the filesystem API still is not literally identical across languages (JS uses `files.read(path, { format })` with `text` / `bytes` / `blob` / `stream`; Python sync/async use `read(path, format=...)` with `text` / `bytes` / `stream`; Go `Read(...)` with `FilesystemReadOpts.Format` leans closer to JS, but the literal overload/type unions and runtime object families still differ)
- the volume file API still is not literally identical across languages (JS uses `readFile(path, { format })` with `text` / `bytes` / `blob` / `stream`; Python sync/async use `read_file(path, format=...)` with `text` / `bytes` / `stream`; Go `ReadFile(...)` with `VolumeReadOpts.Format` leans closer to JS, but the literal overload/type unions and runtime object families still differ)
- some behavior is blocked by the current environment rather than positively proven
- filtered sandbox metrics still are not positively proven end-to-end because the backend returns zero rows for real filtered windows across Go, JS, and Python in this environment
- create-time/update-time network `rules` transforms are still not positively proven live because the current backend does not enforce them for Go, JS, or Python
- live `volume` content operations and some `ubuntu:22.04` template provisioning paths are still not positively proven end-to-end in this environment
