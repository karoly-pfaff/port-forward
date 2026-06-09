# Changelog

All notable changes to Portier are documented here.

---

## [Unreleased]

### Added

- **v1.4 Slice 6 — Go service UDP session tracking** — added `UdpSessionRegistry` (`service/sources/connections/udp_session_registry.go`): composite session keys (`ruleId:mode:clientAddress:clientPort`) for deterministic per-client identity, `OpenOrTouchSession`/`RecordInbound`/`RecordOutbound`/`CloseSession`/`CloseSessionsForRule`/`PruneExpired`/`Snapshot`/`SnapshotForRule` API, value-copy snapshots with `idleMs` and `status` (`active`/`idle`) calculated at snapshot time, expired sessions filtered from snapshot without explicit prune. Constants: `UDPSessionIdleDuration = 30s`, `UDPSessionExpireDuration = 5min`. Concurrency-safe: `sync.Mutex` protects all operations (full packet ops, not hot-path byte chunks). Wired into all three UDP modes in `UDPForwarder`: one-way and last-client call `OpenOrTouchSession` + `RecordInbound` on each inbound packet; last-client mode reads `prevSessionID` under mutex, closes it via `CloseSession` when client endpoint changes, writes new `lastSessionID` under mutex; bidirectional-last-client `targetReadLoop` reads `lastSessionID` under mutex and calls `RecordOutbound`; multi-client mode calls `OpenOrTouchSession` under forwarder mutex when creating session (lock order: forwarder → registry, no deadlock risk), stores `registryID` on `udpSession` struct, calls `RecordInbound` after if/else block, calls `RecordOutbound` in `sessionReadLoop`, calls `CloseSession` in `expireSession` after releasing forwarder mutex. `Stop()` clears `lastSessionID` under mutex and calls `CloseSessionsForRule` as belt-and-suspenders cleanup. `Manager` owns shared `UdpSessionRegistry`, uses `NewUDPForwarderWithRegistry` constructor for UDP rules, exposes `GetLiveUDPSessions() []UdpSessionInfo` for internal use. No public `GET /api/connections` endpoint added yet. No forwarding behavior or existing API behavior changed. Coverage: registry 98.4% stmts (100% all public methods; stale-key defensive path in `OpenOrTouchSession` and `rand.Read` fallback in `generateConnectionID` are accepted untestable paths); 33 registry unit tests (`connections_test` package). Forwarder: `newUDPForwarderWithRegistryAndTimeout` unexported constructor for test isolation; 11 integration tests (real sockets, `waitForUDPCondition` polling, all three modes). Service overall: 80.6% → 82.1% stmts.

- **v1.4 Slice 5 — Go service TCP live tracking** — added `TcpConnectionRegistry` (`service/sources/connections/tcp_connection_registry.go`): runtime-local UUID IDs, `OpenConnection`/`AddBytesIn`/`AddBytesOut`/`CloseConnection`/`CloseConnectionsForRule`/`Snapshot`/`SnapshotForRule` API, serializable `TcpConnectionInfo` snapshots (`id`, `ruleId`, `ruleName`, `protocol`, `clientAddress`, `clientPort`, `targetAddress`, `targetPort`, `startedAt`, `durationMs`, `bytesIn`, `bytesOut`, `status`). `durationMs` calculated at snapshot time. Concurrency-safe: mutex protects map operations; atomic ops handle byte counters in the IO copy hot path to avoid lock contention. `NewTCPForwarderWithRegistry` constructor added to `TCPForwarder`; `countingWriter` extended with `onBytes func(int64)` callback invoked per write chunk; `copyAndClose` accepts optional `onBytes` parameter; `Stop()` calls `CloseConnectionsForRule` after `wg.Wait()` as belt-and-suspenders cleanup for the shutdown timeout path. `Manager` owns shared `TcpConnectionRegistry`, passes it to each `TCPForwarder` via `NewTCPForwarderWithRegistry`, and exposes `GetLiveTCPConnections() []TcpConnectionInfo` for internal use. No public `GET /api/connections` endpoint added yet. No forwarding behavior or existing API behavior changed. Coverage: registry 98.1% stmts (100% all public methods; 1 untestable `rand.Read` error path in private `generateConnectionID`); 26 registry unit tests. Forwarder: 8 new integration tests (real sockets, `waitForTestCondition` polling). Service overall: 79.7% → 80.6% stmts.

- **v1.4 Slice 4 — TypeScript server UDP session tracking** — added `UdpSessionRegistry` (`server/sources/connections/udp-session-registry.ts`): runtime-local UUID IDs, composite session keys (`ruleId:mode:clientAddress:clientPort`) for deterministic identity, `openOrTouchSession`/`recordInbound`/`recordOutbound`/`closeSession`/`closeSessionsForRule`/`pruneExpired`/`snapshot`/`snapshotForRule` API, immutable plain-object snapshots with `idleMs` and `status` calculated at snapshot time. Constants: `UDP_SESSION_IDLE_MS = 30_000` (30s), `UDP_SESSION_EXPIRE_MS = 300_000` (5min). `snapshot` and `snapshotForRule` filter expired sessions from output without mutating state; `pruneExpired` removes them explicitly from memory. Wired into `UdpForwarder` (4th optional constructor param): one-way and last-client modes call `openOrTouchSession` on each inbound packet; last-client mode detects client-endpoint changes, closes the previous session, and opens a new one; multi-client mode calls `openOrTouchSession` per client in `handleMultiClientMessage` with `registryId` stored on the session struct; `recordOutbound` called on target responses in last-client and multi-client modes; `closeSession` called on multi-client timeout; `closeSessionsForRule` called in `stop()` as bulk cleanup. Existing constructor callers that omit the 4th parameter continue to work unchanged. `ForwardManager` owns shared `UdpSessionRegistry`, injects it into every `UdpForwarder`, and exposes `getLiveUdpSessions()` for internal use; no public `GET /api/connections` endpoint added yet. Coverage: registry 100% stmts/branch/funcs; 49 unit tests. `udp-forwarder.ts`: 86.3% stmts (up from 84.3%), 84% branch, 100% funcs; 9 new forwarder integration tests (real sockets, no arbitrary sleeps). Server overall: 80.55% → 82.21% stmts. No forwarding behavior, existing API behavior, or existing tests changed.

- **v1.4 Slice 3 — TypeScript server TCP live tracking** — added `TcpConnectionRegistry` (`server/sources/connections/tcp-connection-registry.ts`): runtime-local ID assignment, `openConnection`/`addBytesIn`/`addBytesOut`/`closeConnection`/`closeConnectionsForRule`/`snapshot`/`snapshotForRule` API, immutable plain-object snapshots, `durationMs` calculated at snapshot time. Wired into `TcpForwarder`: registry entry opened on client accept, `bytesIn` incremented client→target, `bytesOut` incremented target→client, entry removed on connection close (`onClosed` countedClosed guard) and immediately on error path (`closeBoth`), `closeConnectionsForRule` called in `stop()` for deterministic cleanup. `ForwardManager` owns shared `TcpConnectionRegistry`, injects it into every `TcpForwarder`, and exposes `getLiveTcpConnections()` for internal use; no public API endpoint added yet. Coverage: registry 100% stmts/branch/funcs; `tcp-forwarder.ts` 100% stmts/funcs, 90% branch (optional registry param branches when omitted); server overall 79.6% → 80.55% stmts. 28 new registry unit tests and 7 forwarder integration tests added (real sockets, no arbitrary sleeps). No forwarding behavior, existing API behavior, or existing tests changed.

- **v1.4 Slice 2 — Shared types and API contract finalization** — added `LiveConnectionsResponse`, `TcpConnectionInfo`, `UdpSessionInfo`, `RuleLiveSummary`, `LiveConnectionStatus`, `UdpSessionStatus` to `@portier/shared` (`shared/sources/connections.ts`). Finalized `GET /api/connections` contract in `docs/api-contract.md`: field directions documented (`bytesIn` = client-to-target, `bytesOut` = target-to-client, `packetsIn`/`packetsOut` for UDP), `lastTrafficAt` documented as `null` (not absent) when no traffic since rule start, `ruleName` documented as empty string `""` when unresolvable, Shared Shapes section updated to record types as defined with implementation pending. Client in-app API Docs updated with planned endpoint entry and `Planned — v1.4` badge; 5 new `ApiDocsView` tests added. `validate:contract` updated with a skip note for the unimplemented endpoint. `connections.test.ts` added with 14 shape tests covering all types, status unions, UDP modes, and the fully populated response. Runtime implementation remains pending (Slices 3–7).

- **v1.4 Slice 1 — TypeScript server forwarder coverage hardening** — raised `tcp-forwarder.ts` from 68.5% to 100% statements and 100% functions; raised `udp-forwarder.ts` from 57.7% to 84.3% statements and 100% functions. Server overall coverage: 71.9% → 79.6% statements. Added 7 new TCP forwarder tests (idempotent start/stop, bind failure, connection opened/closed events, target-unreachable error, post-bind server error, stop-with-active-connections) and 13 new UDP forwarder tests (lifecycle, one-way forwarded event, forwarded rate-limiting, last-client stats and returned event, multi-client timer reset, session opened/closed events, multi-client forwarded rate-limiting, listen/target/session socket error handlers). Remaining UDP gaps (multi-client send/return error callbacks and a race guard) require mocking to trigger deterministically; documented in `docs/coverage-baseline.md`. No forwarding behavior, API contracts, or runtime behavior changed.

- **Coverage baseline measured** — pre-v1.4 statement coverage: tools/cli 92.7% (gate 92%), client 89.2%, service 79.7%, shared 82.1%, server 71.9%. Scripts not yet measured. Full per-file breakdown and ratchet plan in `docs/coverage-baseline.md`.
- **Coverage tooling added** — `@vitest/coverage-v8` installed; vitest.config.ts files added/updated in all three TypeScript workspaces (client, server, shared) with coverage include/exclude settings; `coverage` script added to each workspace; `npm run coverage:shared/server/client/service/cli/baseline` scripts added to root `package.json`; `scripts/coverage-service.js` and `scripts/coverage-cli.js` added for Go combined coverage reporting; `scripts/validate-coverage.js` added as unified coverage validator with `--only <component>` support, replacing `scripts/validate-cli-coverage.js`; `validate:cli:coverage` now delegates to `validate-coverage.js --only cli`.
- **`waitForTestCondition` timeout raised** — `service/sources/forwarders/tcp_test.go` timeout increased from 2s to 5s to eliminate a flaky failure under cross-package coverage instrumentation with sequential (`-p 1`) test execution.

### Planning

- Planned v1.4 as Live Connection Inspector: read-only live TCP connection and UDP session visibility, per-rule live summaries, `GET /api/connections` endpoint in both runtimes, Live Connections UI view, and CLI `portier connections` command. API contract shape recorded in `docs/api-contract.md`. See `docs/roadmap.md` for goals, data model, UI direction, 14-slice plan, and non-goals.
- Recorded v1.4 coverage strategy: all newly added or materially changed implementation areas target 100% meaningful coverage with explicit gates. Covers TCP and UDP live tracking models (both runtimes), `GET /api/connections` handler, API contract validation, UI helpers, CLI connections command, and diagnostics export integration if changed.
- Planned v1.6 as Architecture, Quality & Maintainability Audit: a dedicated audit and hardening release with structured multi-angle inspection across architecture, runtime parity, forwarding correctness, API contract, CLI, UI, test quality, security posture, packaging, and documentation consistency. See `docs/roadmap.md`.
- Clarified that the v1.4/v1.5 100% meaningful coverage target is a deliberate prerequisite for the v1.6 audit: high meaningful coverage creates the safety net needed to refactor and harden confidently without silently breaking forwarding behavior, API parity, or CLI workflows.
- Planned v1.5 as Declarative Config & Drift Control: plan/diff/apply workflows for comparing desired config files with the running configuration and applying changes safely. See `docs/roadmap.md`.
- Recorded v1.4/v1.5 quality target: all newly added or materially changed implementation areas should reach 100% meaningful test coverage, with explicit coverage gates where practical. Covers CLI, Go service, TypeScript server, shared config/diff logic, contract validators, and client-side logic introduced by the release.
- **CLI coverage gate raised to 92%** — `validate:cli:coverage` threshold raised from 88% to 92% (actual: 92.7%) as the first ratchet step toward the v1.4/v1.5 100% meaningful coverage target. Added 10 targeted tests covering: `validateURL` parse-error path, invalid-flag parse-error paths for `RunConfigExport`/`RunConfigImport`/`RunConfigValidate`/`RunDiagnosticsExport`, file-write failure in `RunConfigExport`, empty `targetHost` and out-of-range `targetPort` in `validateLocalConfig`, `DiagnoseForward` API error in `RunDiagnose`, and `GetStatus` failure in `buildDiagnosticsBundle`. Future v1.4/v1.5 slices should continue raising the gate as meaningful behavioral coverage is added.

---

## [1.3.0] — 2026-06-08

### Added (v1.3 Slice 8 — Readiness audit, coverage gate, version bump)

- **CLI test coverage gate** — `npm run validate:cli:coverage` runs Go tests with cross-package coverage instrumentation and fails if total statement coverage falls below 88%. `scripts/validate-cli-coverage.js` documents genuinely untestable branches (main entry point, http.NewRequest errors, json.Marshal errors). Coverage gate included in `validate:cli`.
- **Coverage tests added** — new tests fill coverage gaps: `output/output_test.go` (FormatBytes MB/GB path, FormatTimestamp RFC3339Nano and invalid paths), `main_test.go` (run() full dispatch for all 9 commands + flag/help/version/unknown branches), and targeted additions to `list_test.go`, `activity_test.go`, `configcmd_test.go`, `status_test.go`, `diagnose_test.go`, `start_test.go`, `stop_test.go`, and `diagnosticscmd_test.go` covering formatProto all UDP modes, --help flags for all subcommands, RunConfig dispatch through all three subcommands, parseLocalConfig edge cases (empty file, invalid array, invalid object, missing rules field, rules not array, null rules), ActiveUDPSessions branch, empty Checks branch, non-connection API errors, per-rule diagnose errors, and JSON output with warningCount.
- **Total CLI test coverage: 90.1%** across sources, client, commands, and output packages.
- **Version bumped to 1.3.0** — `package.json` and `tools/cli/sources/version/version.go`.

### Added (v1.3 Slice 7 — CLI packaging into runtime and release artifacts)

- **CLI binary included in runtime package** — `portier`/`portier.exe` is now built and included in `build/portier/` alongside `service`/`service.exe`, `server.js`, `web/`, and `readme.txt`.
- **Platform build scripts updated** — `scripts/windows/build-runtime.ps1`, `scripts/macos/build-runtime.sh`, and `scripts/linux/build-runtime.sh` each build the CLI from `tools/cli/` directly into the output directory as part of the normal `npm run build:runtime` step.
- **Runtime validation updated** — `scripts/validate-runtime.js` now requires the CLI binary (`portier`/`portier.exe`), verifies it is non-empty, verifies CLI and service binaries have different names, and checks that `readme.txt` mentions `portier runtime`, `portier list`, and `portier diagnostics export`.
- **Release archive validation updated** — `scripts/validate-release.js` now requires the CLI binary in portable archives and validates `readme.txt` CLI command references.
- **Windows installer updated** — `scripts/windows/release/portier.iss` now includes `portier.exe` in the installed files under `{app}`.
- **`readme.txt` updated** — all three platform readme.txt templates now list the files in the package, document CLI usage (six key commands), and note that the CLI requires a running service and does not start the service by itself.
- **`build:clean` updated** — `scripts/clean-build.js` now removes `tools/cli/build/` alongside the other build output directories.
- **No PATH integration in v1.3** — the installer does not add `portier` to PATH; users invoke the CLI by full path or add the install directory to PATH manually.

### Added (v1.3 Slice 6 — Diagnostics export command)

- **`portier diagnostics export --out <file>`** — builds a local JSON diagnostics bundle from the running Portier service. Fetches runtime info, rules, statuses, and recent activity independently; partial failures are recorded in `errors[]` rather than aborting the export. Bundle is always written as long as the file write succeeds.
- **`--run-diagnostics` flag** — when present, runs `POST /api/forwards/:id/diagnose` for each rule sequentially and includes results in `diagnostics.<ruleId>`. If the rules list could not be fetched, per-rule diagnostics are skipped entirely. Per-rule failures are recorded in `errors[]` with source `diagnostics:<ruleId>`.
- **`--activity-limit <n>` flag** — controls how many activity events are included (1–500, default: 100). Exits code `2` if out of range.
- **Bundle schema** (`schemaVersion: "1"`): `app`, `runtime`, `rules`, `statuses`, `diagnostics`, `diagnosticsNote`, `activity` (with `included`/`events`/`note`), `metadata` (`managementUrl`, `source: "cli"`, `generatedBy`), `errors[]`. Does not include logs, environment variables, OS usernames, or raw disk files.
- **Human output**: `Exported diagnostics to <file>` with rule/status/activity counts; with partial failures: `Exported diagnostics with warnings to <file>`.
- **JSON output** (with `--out`): `{"ok":true,"path":"...","ruleCount":N,"statusCount":N,"activityCount":N,"diagnosticCount":N}`; with warnings adds `"warningCount":N`. Without `--out`: prints full bundle to stdout.
- **Human mode without `--out`**: exits code `2` with clear message.
- **Diagnostics subcommand routing** (`RunDiagnostics`): `portier diagnostics help` / `portier diagnostics <unknown>` handled cleanly.
- **`BaseURL()` method** added to API client for embedding the management URL in the bundle metadata.
- **Tests**: 153 CLI tests total (was 132); new tests cover dispatch, usage validation (missing `--out`, invalid `--activity-limit`), happy path (writes file, human/JSON output), partial failure (still writes bundle, "with warnings"), `--run-diagnostics` calls diagnose per rule, activity limit passed to API, rules failure skips diagnostics, write failure exits nonzero, bundle content (schemaVersion, metadata.source, diagnosticsNote, counts), no forbidden fields.
- **Help text updated**: `diagnostics` listed in `portier help`; `portier diagnostics help` shows subcommand list and options.

### Added (v1.3 Slice 5 — Config commands: validate, export, import)

- **`portier config validate <file>`** — validates a local config file without importing it or contacting the service; accepts raw JSON arrays, `{"rules":[...]}` wrapper objects, and full export objects `{"version":"1","exportedAt":"...","rules":[...]}`. Checks: non-empty name, valid protocol, non-empty hosts, ports in range, valid `udpMode`, no duplicate listen bindings. Human output: `Config is valid.` / `Config is invalid.` with errors listed. `--json` output: `{"valid":true,"ruleCount":N,"tcpCount":N,"udpCount":N,"errors":[]}`. Exit 0 valid, exit 1 invalid/unreadable.
- **`portier config export --out <file>`** — exports current rules from the running service to a JSON file via `GET /api/config/export`; file is written only after a successful API response (no partial writes). `--json` + `--out`: prints `{"ok":true,"path":"...","ruleCount":N}`. `--json` without `--out`: prints raw config JSON to stdout (useful for piping). Human mode without `--out`: exits with code 2 (requires `--out`).
- **`portier config import --mode merge|replace [--yes] <file>`** — reads and validates a local config file, then calls `POST /api/config/import`. Invalid files are rejected locally without contacting the service. Replace mode requires `--yes` — without it exits with code 2 and shows a clear warning. No interactive prompts. Human output: `Imported config using merge|replace mode.`; `--json`: `{"ok":true,"mode":"merge"|"replace"}`.
- **Config subcommand routing** (`RunConfig`): `portier config help` / `portier config <unknown>` handled cleanly.
- **New API client types**: `ConfigRule`, `ConfigExportResponse`, `ConfigImportRequest`, `ImportResult`, `ConfigImportResponse` mirroring `GET /api/config/export` and `POST /api/config/import` response shapes.
- **New API client methods**: `ExportConfig()`, `ImportConfig(req)` using a new `doWithBody` helper for POST with a JSON body.
- **Tests**: 132 CLI tests total (was 89); new tests cover `ExportConfig`/`ImportConfig` client methods, all validation cases (valid shapes, malformed JSON, invalid protocol/port/udpMode/host/name/duplicate binding), all export cases (writes file, JSON output, stdout mode, API error leaves no file, connection failure), all import cases (merge/replace modes, `--yes` requirement, local validation blocks API, bad mode, connection failure), and config dispatch.
- **Help text updated**: `config` listed in `portier help`; `portier config help` shows subcommand list; each subcommand shows usage and flags.

### Added (v1.3 Slice 4 — Lifecycle and diagnostics commands: start, stop, diagnose)

- **`portier start <id|name>`** — resolves rule by exact ID or exact name, calls `POST /api/forwards/:id/start`; human output shows `Started <name>  (<listen> → <target>)`; `--json` prints `{"ok": true, "action": "start", "ruleId": "..."}`.
- **`portier stop <id|name>`** — resolves rule, calls `POST /api/forwards/:id/stop`; human output shows `Stopped <name>`; `--json` prints `{"ok": true, "action": "stop", "ruleId": "..."}`.
- **`portier diagnose <id|name>`** — resolves rule, calls `POST /api/forwards/:id/diagnose`; human output shows summary status + message and a CHECK/STATUS/MESSAGE table; `--json` prints raw `RuleDiagnosticsResult`.
- **Safe rule resolver** (`commands.ResolveRule`): exact ID match wins unconditionally; exact name match succeeds if unique; duplicate names produce exit code 2 with an ID/NAME/PROTO/LISTEN disambiguation table on stderr; no match exits 1.
- **API client methods**: `StartForward(id)`, `StopForward(id)`, `DiagnoseForward(id)` added to the management API client; shared `do(method, path, out)` helper extracted to remove duplication between GET and POST paths.
- **New types**: `DiagnosticCheck`, `DiagnosticSummary`, `RuleDiagnosticsResult` mirroring the `POST /api/forwards/:id/diagnose` response shape.
- **Tests**: 89 CLI tests total (was 59); new tests cover `StartForward`/`StopForward`/`DiagnoseForward` client methods, all resolver cases (exact ID, exact name, ID wins over same-text name, duplicate ambiguity, not found, empty list), and all three command handlers (human output by ID and by name, JSON output, missing arg, ambiguity, connection failure, not found).
- **Help text updated**: `start`, `stop`, `diagnose` listed in `portier help`; rule identity note added.

### Added (v1.3 Slice 3 — Read-only commands: list, status, activity)

- **`portier list`** — calls `GET /api/forwards`; human-readable aligned table (NAME, PROTO, LISTEN, TARGET, ENABLED); `--json` prints raw `ForwardRuleResponse[]`. Empty state shows a friendly message. UDP mode shown compactly in PROTO column (`udp/1way`, `udp/lc`, `udp/mc`).
- **`portier status`** — calls `GET /api/status`; human output joins rule names and protocols via `GET /api/forwards` (falls back to ruleId if unavailable); shows NAME, PROTO, STATE, CONNS, BYTES IN, BYTES OUT, LAST ERROR; `--json` prints raw `ForwardStatus[]`.
- **`portier activity`** — calls `GET /api/activity`; human output shows TIME, SEV, TYPE, RULE, MESSAGE; `--json` prints raw `ActivityEvent[]` (not the `{"events":[...]}` API wrapper). Filters: `--limit` (1–500, default 50), `--rule` (rule ID), `--type` (event type), `--severity`; invalid `--limit` exits with code 2.
- **Output helpers** (`tools/cli/sources/output/`): `FormatBool`, `FormatBytes`, `FormatTimestamp`, `PrintTable` — used by all human-readable command output.
- **`exitWithError` helper** in `commands` package — shared connection/API error handling across commands.
- **Tests**: 59 Go CLI tests total (was 22+); new tests cover `GetForwards`, `GetStatus`, `GetActivity` client methods (success, error, query params, empty), plus all three command handlers (human output, JSON output, empty state, connection failure, API error, filter passing, limit validation, ruleId fallback).

### Added (v1.3 Slice 2 — Go CLI skeleton and API client)

- **`tools/cli/` module** — new Go module (`portier/cli`) under `tools/cli/` for the Portier CLI. Separate from the Go service (`portier/service`); the CLI is an API client, not a second runtime.
- **CLI binary name**: `portier` (Linux/macOS) / `portier.exe` (Windows). Background service binary remains `service` / `service.exe`.
- **`portier runtime` command** — calls `GET /api/runtime`; displays name, version, runtime, platform/arch, uptime, management URL, config path, static dir, service mode. Works with both the Go service and TypeScript server.
- **`--json` flag** — prints machine-readable JSON from the API response; no decorative text around JSON output.
- **Connection options** — `--url`, `--host`, `--port`, `PORTIER_URL` env var; precedence: `--url` > `--host`/`--port` > `PORTIER_URL` > default (`http://127.0.0.1:47831`).
- **`portier help` / `--help` / `-h`** — shows purpose, default URL, global flags, available commands, planned commands, exit codes.
- **`portier version` / `--version`** — shows `Portier CLI <version>`.
- **Exit code policy**: `0` success, `1` general/API error, `2` invalid arguments/usage, `3` connection failure.
- **`User-Agent` header**: `PortierCLI/<version>` on every API request.
- **Structured error handling**: `ConnectionError` (maps to exit 3 + hint) and `APIError` (non-2xx response with `errors[]` shape).
- **Tests**: 22+ Go tests using `httptest` covering URL resolution precedence, runtime human/JSON output, connection failure, API errors, User-Agent, invalid JSON, and error type unwrapping.
- **npm scripts**: `build:cli`, `test:cli`, `validate:cli`.
- **`tools/cli/readme.md`** — CLI usage, global flags, commands, exit codes, module structure, planned commands.

---

## [1.2.0] - 2026-06-07

### Goal

Portier v1.2 improves operational confidence with diagnostics, visibility, and safer networking UX. See `docs/roadmap.md` for goals, slices, and non-goals.

### Added (v1.2 Slice 7 — Diagnostics export / support bundle)

- **Download Diagnostics JSON** button in Settings, below Runtime / Environment. Assembles a local JSON support bundle and downloads it directly in the browser — no upload, no backend endpoint.
- **Bundle contents**: `schemaVersion`, `exportedAt`, `app` (name + version), `runtime` (from `GET /api/runtime`), `rules` (fresh fetch), `statuses` (fresh fetch), `diagnostics` (any results already run in the current UI session), `activity` (up to 100 recent events), and `metadata` (management URL, source, generatedBy).
- **Partial-failure handling**: each data source is fetched independently using `Promise.allSettled`. If any source fails, the rest are still included; an `errors` array is added to the bundle and a warning is shown in the UI. The download still proceeds.
- **Empty diagnostics note**: if no rule diagnostics have been run in the current session, the bundle includes `diagnosticsNote: "No rule diagnostics had been run in this UI session."` alongside an empty `diagnostics` object.
- **Excluded data**: raw config file contents from disk, logs, environment variables, OS user name, home directory beyond paths already in the runtime endpoint, node_modules, and build file listings. Nothing is uploaded or transmitted.
- **Filename format**: `portier-diagnostics-YYYYMMDD-HHMMSS.json` (local time; no characters invalid on Windows).
- **`diagnosticsExport.ts`** helper module added at `client/sources/features/settings/`: exports `buildDiagnosticsBundle()`, `buildDiagnosticsFilename()`, and `downloadJson()`.
- **`settings-warn` CSS class** added for the partial-data warning state.
- **New tests**: `diagnosticsExport.test.ts` (28 unit tests covering bundle structure, diagnostics inclusion, partial failure, filename, and download mechanics); 10 new `SettingsView` component tests covering render, fetch calls, success/partial/disabled states, bundle schema, and diagnostics from UI state. 216 tests total pass.

### Added (v1.2 Slice 6 — Safer networking UX)

- **Listen host presets** in the Add/Edit Rule drawer: "Local only" (127.0.0.1) and "LAN exposed" (0.0.0.0) quick-select buttons. Active preset is highlighted; manual host entry remains fully supported.
- **Inline LAN warning** in the rule form: when listenHost is `0.0.0.0`, a warning block appears directly below the listen host/port row stating that the rule listens on all interfaces and that Portier does not create firewall rules automatically.
- **Listen host hint text**: "Only this computer can connect" shown for 127.0.0.1; "Other devices on your network may be able to connect if firewall rules allow it" shown for 0.0.0.0.
- **Firewall note advisory card**: shown in the advisory section when listenHost is `0.0.0.0`. Platform-aware: shows a Windows-specific message if `runtimePlatform="windows"` is provided, otherwise a generic OS note. `ForwardRuleForm` now accepts an optional `runtimePlatform` prop.
- **Friendly conflict error copy**: when a save fails because the listen binding is already in use, the form shows "Another rule is already using this protocol, listen host, and listen port. Choose a different listen port, or stop/remove the conflicting rule." instead of the raw server error.
- **Improved LAN_EXPOSURE advisory message** (shared): updated to "Listening on 0.0.0.0 exposes this forwarded port on all interfaces. Other LAN devices may be able to connect if firewall settings allow it."
- **New tests** (ForwardRuleForm): Local only / LAN exposed presets set correct host; custom host entry still works; LAN warning visible for 0.0.0.0; LAN warning absent for 127.0.0.1; generic and Windows-specific firewall notes; save not blocked by LAN warning; friendly conflict error shown; privileged port advisory. 17 new/updated test cases.

### Added (v1.2 Slice 5 — Settings / runtime / config polish)

- **App version in sidebar footer**: Sidebar now shows `v{version}` below the management host. Sourced from new `PORTIER_APP_VERSION` constant in `@portier/shared`.
- **Copy buttons in Runtime / Environment**: Config path, static dir, and management URL each have a small "Copy" button. Uses `navigator.clipboard.writeText`; shows "Copied!" on success, "Failed" on error, resets after 2 seconds. Gracefully handles environments where clipboard is unavailable.
- **Export Config UX**:
  - Filename is now datetime-stamped: `portier-config-YYYYMMDD-HHMMSS.json` (local time).
  - Button renamed to "Download Config (JSON)" for clarity.
  - Helper text now states "The Activity Log is not included" so users know what the export covers.
  - Export shows a success confirmation message; errors are displayed inline instead of via `alert()`.
- **Import Config UX**:
  - Import mode (Merge / Replace) is now shown above the file picker with descriptive explanations, so users understand the difference before selecting a file.
  - Merge description: "adds rules from the file; skips rules with conflicting listen port bindings."
  - Replace description: "stops and deletes all current rules, then applies imported rules. Export a backup first if you want to keep your current rules."
  - Replace confirmation dialog now includes an "Export current config as backup" button for a one-click safety exit.
- **`PORTIER_APP_VERSION`** constant exported from `shared/sources/index.ts`.
- **New tests** (SettingsView): import mode descriptions visible before file selection; import button absent until valid file loaded; replace confirm shows backup export button; copy config path/static dir/management URL; clipboard failure shows "Failed" feedback; export success/error feedback. 18 new test cases.
- **New tests** (Sidebar): 6 tests covering render, nav click, open state, management host, and version footer.

### Added (v1.2 Slice 4 — Activity Log polish)

- **View activity** button (Activity icon) added to every rule row in the Forward Rules table. Clicking it navigates to the Activity view and pre-filters to that rule.
- Activity view **filter banner**: when navigating from a rule row, a status banner shows "Filtered to rule: [name]" with a Clear button. Handles deleted/missing rule names gracefully by displaying the raw rule ID.
- Activity view **type filter**: new dropdown lets users filter events by event type (rule lifecycle, connection, packet, session, config events). Type is passed as a query param to `GET /api/activity`.
- **Clear filters** button appears in the Activity header when any filter (rule, severity, type) is active.
- **Export JSON** button in the Activity footer exports currently loaded/filtered events as a JSON file. Filename format: `portier-activity-YYYYMMDD-HHMMSS.json`. Payload includes `exportedAt`, `filters`, and `events`. Client-side only; no backend endpoint.
- **Clear Log** button in the Activity footer calls `DELETE /api/activity` and empties the displayed list.
- Packet event **throttle note**: small explanatory text in the Activity body — "High-frequency packet events may be summarized or throttled; counters remain exact in rule status."
- `DELETE /api/activity` endpoint on both the TypeScript server and Go service: clears the in-memory activity log and returns `204 No Content`. Does not affect rules or forwarding state.
- `clearActivity()` API helper added to `client/sources/api/portierApi.ts`.
- `Store.Clear()` method added to `service/sources/activity/store.go` (Go).
- `Manager.ClearActivity()` method added to `service/sources/manager/manager.go` (Go).
- `docs/api-contract.md` updated with `DELETE /api/activity` section.
- Client in-app API Docs view updated with `DELETE /api/activity` entry.
- 18 new `ActivityLogView` component tests (21 total); 4 new `ForwardRuleList` activity navigation tests; 2 new `ApiDocsView` tests.
- 3 new TypeScript server API tests for `DELETE /api/activity`; 3 new Go API tests.
- `validate:contract` updated with `DELETE /api/activity` scenario and post-clear empty-array verification.

### Added (v1.2 Slice 3 — rule diagnostics UI)

- **Diagnose** action button (stethoscope icon) added to every rule row in the Forward Rules table.
- Clicking Diagnose calls `POST /api/forwards/:id/diagnose` and shows a collapsible diagnostics panel inline below the rule row — no navigation required.
- Diagnostics panel states: loading spinner while in flight, pass/warn/fail summary with timestamp, per-check rows with pass/warn/fail/skip status, and error display for API failures.
- Duplicate Diagnose clicks are prevented while a request is in flight for that rule; other rules remain clickable.
- Re-running Diagnose replaces the previous result immediately (pending → result).
- Closing the panel (✕ button) clears the result; deleting a rule also clears its diagnostic state.
- `diagnoseForwardRule(ruleId)` API helper added to `client/sources/api/portierApi.ts`.
- `DiagnosisEntry` union type exported from `ForwardRuleList.tsx` for App-level state management.
- `RuleDiagnosticsPanel` component added at `client/sources/features/forwards/RuleDiagnosticsPanel.tsx`.
- CSS for diagnostics panel added to `client/sources/styles/styles.css`.
- 11 new `RuleDiagnosticsPanel` unit tests; 9 new `ForwardRuleList` diagnostics tests; 2 new `App` integration tests (133 total client tests).

### Added (v1.2 Slice 2 — rule diagnostics API)

- `POST /api/forwards/:id/diagnose` endpoint on both the TypeScript server and Go service: runs diagnostic checks against an existing forward rule without changing rule state or opening long-lived sockets.
- Response shape: `RuleDiagnosticsResult` — ruleId, ruleName, protocol, summary (`pass`/`warn`/`fail`), checks array, diagnosedAt timestamp.
- Checks implemented: `listen-host`, `lan-exposure`, `privileged-port`, `common-port`, `listen-bind`, `target-host`, `target-connect`, `udp-mode` (UDP only).
- `listen-bind` skips the bind attempt when the rule is running and returns `pass` — Portier owns the socket.
- `target-connect` is always `skip` for UDP rules (UDP reachability cannot be verified without a protocol-specific response).
- `udp-mode` warns for `bidirectional-last-client`; passes for `one-way` and `bidirectional-multi-client`.
- `DiagnosticStatus`, `DiagnosticCheck`, `DiagnosticSummary`, `RuleDiagnosticsResult` types exported from `@portier/shared`.
- `getRule(id)` method added to `ForwardManager` (TypeScript server).
- `diagnoseRule()` helper added to `server/sources/diagnose.ts` (TypeScript) and `service/sources/api/diagnose.go` (Go).
- `DiagnosticCheck`, `DiagnosticSummary`, `RuleDiagnosticsResult` types added to `service/sources/domain/domain.go`.
- `validate:contract` updated with diagnose scenarios: unknown rule 404, TCP reachable target, UDP target-connect skip, LAN exposure warning, response shape verification.
- TypeScript tests and Go tests added for all check scenarios.

### Added (v1.2 Slice 1 — runtime info)

- `GET /api/runtime` endpoint on both the TypeScript server and Go service: returns name, version, runtime (`"node"`/`"go"`), platform, arch, uptimeSeconds, startedAt, managementHost/Port, configPath, staticDir, serviceMode, and pid.
- `RuntimeInfo` type exported from `@portier/shared`.
- `fetchRuntimeInfo()` API helper in the client.
- Settings view — **Runtime / Environment** section: shows live runtime info (runtime, version, platform/arch, uptime, management URL, config path, static dir, service mode, PID). Gracefully shows "Runtime information is unavailable from this backend." for older backends.
- Go service version package (`service/sources/version/version.go`): default `"dev"`, injectable at build time via `-ldflags`.
- `validate:contract` updated to verify GET /api/runtime shape and runtime field for both runtimes.

### Added (post-v1.1, pre-v1.2)

- Config compatibility fixtures (`tests/fixtures/config/`): 8 valid and 8 invalid `rules.json` fixtures covering TCP, all UDP modes, both config shapes (raw array and Go wrapper), and all field-level error categories.
- `npm run validate:config` — fixture-based compatibility runner (`scripts/validate-config.js`): validates config load, HTTP API import/export, UDP mode defaults, duplicate binding rejection, and invalid fixture rejection against the TypeScript runtime and the Go service when binary is available. Never reads or writes the user's real `rules.json`.

---

## [1.1.0] - 2026-06-06

### Goal

Portier v1.1 makes Portier easy and safe to install as a native background service on Windows, macOS, and Linux, with automated package/service validation and clean release artifacts.

See `docs/installer-strategy.md` for scope, platform decisions, and implementation slices.

### Added

- Windows Inno Setup installer (`scripts/windows/release/portier.iss`): installs to `%ProgramFiles%\Portier\`, optional Windows Service registration with auto-start at boot, config at `%ProgramData%\Portier\rules.json`. Upgrade support: stops running service before overwriting binaries. Uninstall removes service registration and logs; preserves `rules.json` by default.
- `build-release.ps1` (`scripts/windows/release/`): reads version from `package.json`, runs `build:runtime`, calls ISCC.exe, produces `build/releases/windows/Portier-Setup-<version>.exe`.
- macOS install scripts updated: `install-launch-agent.sh` now auto-copies `build/portier/` to `~/Applications/Portier/` by default; adds `--source-dir`, `--no-start`, and `--runtime service|node` options; fixes label consistency bug (`com.portier.port-forwarding` everywhere).
- `uninstall-launch-agent.sh` adds `--purge` flag for removing config and logs (config is preserved by default).
- `scripts/macos/release/build-release.sh` — builds `build/releases/macos/portier-portable-macos-<version>.tar.gz` from `build/portier/`.
- Signing and notarization documented in `scripts/macos/readme.md` (unsigned local builds supported; Developer ID signing documented as required for public distribution).
- Linux `install-service.sh` updated: auto-copies `build/portier/` to `/opt/portier/` by default; adds `--source-dir`, `--no-enable`, `--no-start`, and `--runtime service|node` options.
- `scripts/linux/release/build-release.sh` — builds `build/releases/linux/portier-<version>-linux.tar.gz` from `build/portier/`.
- `scripts/linux/readme.md` updated: install flags table, release archive section, firewall notes, journald commands, `--no-enable` documented.
- `scripts/windows/service/validate-user-install.ps1`: validates user-scope scheduled task flow with test name `PortierTestUser`, isolated temp dirs, auto-port detection, `-NoBuild`/`-KeepFiles`/`-Port` flags; never touches production.
- `scripts/windows/service/validate-machine-service.ps1`: validates machine-scope Windows Service flow with test name `PortierTestMachine`; requires Administrator; same flags; never touches production.
- `scripts/macos/service/validate-launch-agent.sh`: validates LaunchAgent flow with test label `com.portier.test`, temp plist at `~/Library/LaunchAgents/com.portier.test.plist`; no sudo required; `--no-build`/`--keep-files`/`--port` flags.
- `scripts/linux/service/validate-systemd-service.sh`: validates systemd flow with test unit `portier-test.service`, temp paths under `/tmp/portier-test-<pid>/`; requires root; fails clearly if not root or systemd unavailable.
- `scripts/validate-service.js`: cross-platform dispatcher — Windows runs user-scope, macOS runs LaunchAgent, Linux runs systemd; fails clearly on unsupported platforms.
- `npm run validate:service:current` / `validate:service:windows:user` / `validate:service:windows:machine` / `validate:service:macos` / `validate:service:linux` — explicit release validation commands.

- `scripts/build-release.js`: unified release packaging script for all platforms. Reads version from `package.json`, calls `build:runtime`, produces portable archives (Windows `.zip`, macOS/Linux `.tar.gz`) and Windows installer (non-fatal if Inno Setup absent). Service binaries are platform-native; run on each target OS.
- `scripts/validate-release.js`: validates `build/releases/<platform>/` layout, archive contents (required/forbidden files, readme.txt content), and optional installer artifact.
- Updated `readme.txt` in Windows/macOS/Linux build scripts: now includes portable archive notice ("does not install OS services"), `--config` / `--static-dir web` options, "not bundled in this archive" note for config.
- `npm run build:release` / `build:release:current` — full release packaging for current platform (portable + installer if available).
- `npm run build:release:portable` — portable archive only, skip installer.
- `npm run validate:release` / `validate:release:current` — validate release artifacts for current platform.
- `npm run validate:release:portable` — validate portable archive only.

### Deferred

- macOS `.pkg` installer via `pkgbuild`/`productbuild` (requires macOS tooling; deferred to future release).
- Linux `.deb` / `.rpm` packages (deferred beyond v1.1).

---

## [1.0.0] - 2026-06-06

### Added

- Native Go service runtime in `service/`, preferred for production packages while the TypeScript server in `server/` remains supported as the reference and fallback runtime.
- TCP forwarding in both runtimes with real listeners, bidirectional piping, status counters, and lifecycle cleanup.
- UDP forwarding in both runtimes across all supported modes: `one-way`, `bidirectional-last-client`, and `bidirectional-multi-client`.
- Rule CRUD, start/stop, update/restart behavior, duplicate listen binding rejection, persistent config, config import/export, and rule reorder.
- Activity Log with bounded in-memory events for rule lifecycle, TCP connections, UDP packets, UDP multi-client sessions, config export, and config import.
- Dark React web UI with Dashboard, Forward Rules, Activity, Settings, API Docs, mobile sidebar, add/edit/delete drawer flows, error banners, LAN exposure warnings, and common/recommended port advisories.
- Playwright E2E coverage for app load, add/edit/delete flows, start/stop, settings import, mobile navigation, and real TCP/UDP forwarding.
- Go service tests for config, manager behavior, API routes, TCP forwarding, UDP modes, activity events, validation, options, and advisories.
- Windows, macOS, and Linux packaging/install docs and scripts, including Go service mode and Node/TypeScript server fallback where applicable.

### Changed

- Repository convention standardized on `sources/` for source directories, `build/` for generated output, and `scripts/` for executable automation; platform docs and templates are co-located under `scripts/{platform}/`.
- Production package layout standardized to a flat install directory:

```text
<install-dir>/
  service          (or service.exe on Windows)
  server.js        (Node fallback, requires Node.js)
  web/
    index.html
    assets/
```

- Native Go service static directory defaults to `web` for packaged layout. Repository development commands may still pass `../client/build`.
- TypeScript server remains supported and can serve either repository `client/build` or packaged `web` via `--static-dir` / `PORTIER_STATIC_DIR`.
- Runtime config remains external in all modes; `rules.json` is not baked into binaries or packages.
- Manual QA scope narrowed to platform/package/service install behavior because core TCP/UDP correctness is automated.

### Validation

- Root validation scripts cover lint, typecheck, unit/integration tests, Go tests, build, Playwright E2E, package smoke checks, and repository hygiene checks.
- Real protocol coverage is automated for TCP, UDP one-way, UDP bidirectional-last-client, and UDP bidirectional-multi-client.

---

## [0.1.0] - 2026-06-04

First usable internal release of Portier, a local TCP/UDP port forwarding manager for development and LAN testing.

### Added

- TypeScript server with REST API on `127.0.0.1:47831` by default.
- TCP forwarding, UDP one-way forwarding, and UDP bidirectional-last-client forwarding.
- React management UI for listing, creating, editing, starting, stopping, and deleting forward rules.
- Shared TypeScript validation, common port advisories, recommended forward port range, and LAN exposure warnings.
- External JSON config path via `--config` / `PORTIER_CONFIG`.
- Static web serving via `--static-dir` / `PORTIER_STATIC_DIR`.
