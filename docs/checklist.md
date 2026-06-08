# QA Checklist

## Automated Release Validation

Run these before tagging:

- [ ] `npm run lint`
- [ ] `npm run typecheck`
- [ ] `npm run test`
- [ ] `npm run build`
- [ ] `npm run build:client`
- [ ] `npm run test:e2e`
- [ ] `npm run check`
- [ ] `go test ./...` from `service/`
- [ ] `go build ./...` from `service/`
- [ ] `npm run build:service`
- [ ] `npm run validate:runtime:smoke` — builds `build/portier/`, validates layout, runs smoke test

## Additional Validation Suites

Run explicitly — not part of `npm run check`. Slower or platform-sensitive.

- [ ] `npm run validate:config` — Config compatibility: loads every fixture from `tests/fixtures/config/`, verifies valid fixtures load and import correctly, invalid fixtures are rejected with appropriate errors, duplicate bindings are caught, UDP mode defaults are applied, and the export shape is stable. TypeScript runtime always checked; Go runtime checked when binary is available. (`scripts/validate-config.js`)
- [ ] `npm run validate:contract` — API contract parity: runs all API scenarios against TypeScript server; if Go binary available, runs same scenarios against Go service and compares response shapes, status codes, field names, and error shapes. (`scripts/validate-contract.js`)
- [ ] `npm run validate:binary` (or `validate:runtime:behavior`) — Runtime binary behavior: starts `build/portier/service[.exe]`, verifies health, static serving, missing-static-dir handling, invalid-config failure, and clean shutdown. Runs `build:runtime` first unless `--no-build` is passed. (`scripts/validate-binary.js`)
- [ ] `npm run validate:scripts` — Installer script analysis: static analysis of all platform install and validation scripts (no firewall commands, test-specific names in validate scripts, production path defaults in install scripts, quoting correctness); plus dry-run execution on the current platform. (`scripts/validate-scripts.js`)

## Explicit OS Service Install Validation

These commands must be run explicitly on the target platform before distribution. They are not run by `npm run check`.

Each script uses test-specific service names, ports, and temp directories. Production Portier installs and config are never touched.

- [ ] `npm run validate:service:windows:user` — Windows scheduled task (no Administrator required)
- [ ] `npm run validate:service:windows:machine` — Windows Service (Administrator required)
- [ ] `npm run validate:service:macos` — macOS LaunchAgent (no sudo required)
- [ ] `npm run validate:service:linux` — Linux systemd unit (root/sudo required)

Each script validates:
1. Package copy to isolated temp install dir
2. Service/task/agent registration with test-specific name
3. Service start
4. `/api/health` responds 200
5. Web UI HTML served at `/`
6. Service stop
7. Service/task/agent unregistered and verified removed
8. Temp files cleaned up

Flags supported by all scripts:
- `--no-build` / `-NoBuild` — skip `npm run build:runtime`, use existing `build/portier/`
- `--keep-files` / `-KeepFiles` — preserve temp directories on failure for debugging
- `--port` / `-Port` — override the management port (default: auto-detect free port)

## Automated Coverage Confirmed

Shared and TypeScript coverage:

- [ ] Shared validation and port advisories
- [ ] TypeScript server CRUD HTTP layer
- [ ] TypeScript server import/export HTTP layer
- [ ] TypeScript server status, start/stop, reorder, duplicate binding, and static serving behavior
- [ ] Client App integration flows
- [ ] Settings import/FileReader flow
- [ ] Dashboard, Activity, Settings, API Docs, Forward Rules, and drawer flows
- [ ] Rule diagnostics UI: Diagnose button, loading state, pass/warn/fail/skip panel, error display, duplicate prevention, clear on close/delete
- [ ] Diagnostics export: Download Diagnostics JSON button in Settings, bundle structure (schemaVersion, runtime, rules, statuses, activity, diagnostics, metadata), partial-failure errors array, empty-diagnostics note, filename pattern, disabled-while-generating
- [ ] **API documentation rule:** when an API endpoint is added, removed, or changed — both `docs/api-contract.md` and the client in-app API Docs view (`client/sources/features/apidocs/ApiDocsView.tsx`) must be updated, along with `ApiDocsView.test.tsx`.
- [x] **TypeScript server forwarder coverage hardening (v1.4 Slice 1)** — tcp-forwarder.ts raised to 100% statements/functions; udp-forwarder.ts raised to 84.3% statements/100% functions. 7 new TCP tests + 13 new UDP tests. Remaining UDP gaps (multi-client send/return error callbacks, race guard) documented in `docs/coverage-baseline.md`. Server overall: 71.9% → 79.6% stmts.

Playwright E2E coverage:

- [ ] App load
- [ ] Add/edit/delete rule flows
- [ ] Start/stop rule flow
- [ ] Settings config import (merge — 1 TCP rule)
- [ ] Settings config import (replace — v1-mixed fixture, 4 rules; verify Forward Rules view)
- [ ] Settings config import (invalid JSON — parse error, state preserved)
- [ ] Settings config export (download shape: version, exportedAt, rules array, rule present)
- [ ] Mobile sidebar behavior
- [ ] TCP real forwarding
- [ ] UDP one-way real forwarding
- [ ] UDP bidirectional-last-client real forwarding
- [ ] UDP bidirectional-multi-client real forwarding
- [ ] TCP and UDP activity assertions

Config compatibility coverage (`validate:config` — not manual QA):

- [ ] Valid fixture config load (raw array — all protocols and UDP modes)
- [ ] Valid fixture config load (Go wrapper shape)
- [ ] Valid fixture import via HTTP API (all 8 valid fixtures)
- [ ] UDP default mode normalization (no udpMode → one-way)
- [ ] Export shape stability (version, exportedAt, rules[])
- [ ] Duplicate binding rejection (409)
- [ ] Invalid field rejection — port out of range, missing name, empty host, bad protocol, bad udpMode (400)
- [ ] Malformed JSON rejection (server exit)

Go service coverage:

- [ ] Config load/save and import/export
- [ ] Manager lifecycle, duplicate binding rejection, update/restart behavior, and reorder
- [ ] API routes and error shapes
- [ ] TCP real forwarding and activity
- [ ] UDP one-way, bidirectional-last-client, bidirectional-multi-client, stats, sessions, stop, and activity
- [ ] Port advisories, validation, options, static serving, and health endpoint

## Manual Platform QA Required Before Distribution

Manual QA is now limited to firewall behavior and production install paths. Core TCP/UDP protocol correctness and OS service install/uninstall flows are automated.

### Package Build and Smoke Test (Automated)

- [ ] `npm run validate:runtime:smoke` passes — builds `build/portier/`, validates layout and content, runs smoke test.
  - Validates: `service`/`service.exe`, `server.js`, `web/index.html`, `web/assets/`, `readme.txt`.
  - Validates: `readme.txt` mentions management URL and config path.
  - Validates: `node_modules`, `rules.json`, `sources/`, `client/`, `server/` are absent from the package.
  - Smoke test: starts the packaged binary, polls `/api/health`, GETs `/`, verifies HTML is served, stops cleanly.

### OS Service Install Validation (Automated — Run Explicitly)

Service install, start, health check, stop, and uninstall are validated by explicit commands on each platform:

- [ ] `npm run validate:service:windows:user` — Windows scheduled task, no Administrator required.
- [ ] `npm run validate:service:windows:machine` — Windows Service, Administrator required.
- [ ] `npm run validate:service:macos` — macOS LaunchAgent, no sudo required.
- [ ] `npm run validate:service:linux` — Linux systemd unit, root/sudo required.

Each script uses test-specific names and temp directories. Production Portier installs are not touched.

Pass `--no-build` to reuse an existing `build/portier/` and skip the package build step.

### Windows Firewall (Manual — On Real Production Install)

- [ ] Windows Firewall prompts or required inbound rules documented and observed for LAN-visible forwarded ports.
- [ ] Production install to `%ProgramFiles%\Portier` (Machine) or `%LOCALAPPDATA%\Portier` (User) verified.
- [ ] Config preserved at `%ProgramData%\Portier\rules.json` (Machine) or `%APPDATA%\Portier\rules.json` (User) after uninstall.

### macOS Firewall (Manual — On Real Production Install)

- [ ] macOS Firewall prompts or required settings documented and observed for LAN-visible forwarded ports.
- [ ] Production install to `~/Applications/Portier` with config at `~/Library/Application Support/Portier/rules.json` verified.
- [ ] Config preserved after `scripts/macos/service/uninstall-launch-agent.sh`.

### Linux Firewall (Manual — On Real Production Install)

- [ ] Firewall rules for LAN-visible forwarded ports documented and observed.
- [ ] Production install to `/opt/portier` with config at `/etc/portier/rules.json` verified.
- [ ] Config preserved after `scripts/linux/service/uninstall-service.sh` without `--remove-config`.

## Post-v1.0 Follow-Ups

- Drag-and-drop rule reorder testing, if drag-and-drop UI replaces the current Move Up/Down controls.
- macOS `.app` bundle or Homebrew formula.
- Linux hardening beyond the example systemd unit.

## Coverage Baseline (pre-v1.4)

Measured at v1.3.0. See `docs/coverage-baseline.md` for full per-file breakdown and ratchet plan.

| Component  | Statements | Branch | Functions |    Gate | Run command                      |
| ---------- | ---------: | -----: | --------: | ------: | -------------------------------- |
| tools/cli  |      92.7% |      — |         — |     92% | `npm run coverage:cli`           |
| client     |      89.2% |  87.6% |     74.0% |    none | `npm run coverage:client`        |
| service    |      79.7% |      — |         — |    none | `npm run coverage:service`       |
| shared     |      82.1% |  53.6% |     88.9% |    none | `npm run coverage:shared`        |
| server     |      71.9% |  80.1% |     93.3% |    none | `npm run coverage:server`        |
| scripts    |        N/M |      — |         — |    none | not yet measured                 |

Run all (reporting): `npm run coverage:baseline`  
Run all with gates: `npm run validate:coverage` (exits 1 on gate failure)

Key gaps before v1.4:
- `server/sources/forwarders/tcp-forwarder.ts`: 68.5% — v1.4 live tracking lands here
- `server/sources/forwarders/udp-forwarder.ts`: 57.7% — same
- `service/sources/api` (update/delete/reorder handlers): 50–54% — real gap
- `shared/sources/index.ts` branch coverage: 52.9% — validation edge cases
- `server/sources/logger.ts`: 0% — genuine gap, low-risk

Coverage policy for v1.4 and v1.5: 100% meaningful coverage for all newly added or materially changed files. Existing baselines ratcheted incrementally; do not block unrelated work on legacy gaps.

---

## v1.6 Planning

v1.6 is a dedicated audit and hardening release targeting a structured multi-angle inspection of the codebase after v1.4 and v1.5 have raised test coverage. See `docs/roadmap.md` for audit dimensions, slice plan, and non-goals.

Planning checklist:

- [x] v1.6 roadmap added to `docs/roadmap.md` as Architecture, Quality & Maintainability Audit
- [x] Audit dimensions recorded (architecture boundaries, runtime parity, forwarding correctness, API contract, CLI quality, client/UI quality, test quality, complexity/maintainability, security/safety, packaging, documentation consistency)
- [x] Coverage prerequisite explained: v1.4/v1.5 100% meaningful coverage target is the safety net required before the v1.6 audit and hardening work
- [x] Tentative audit slice plan recorded (12 slices)
- [x] Non-goals recorded (no new large features, no server/ removal, no major architecture rewrite without audit-backed plan)
- [x] v1.4/v1.5 coverage target linked to v1.6 audit safety net in `docs/roadmap.md`

---

## v1.5 Planning

v1.5 targets declarative config and drift control: plan/diff/apply workflows for comparing desired config files with the running configuration and applying changes safely. See `docs/roadmap.md` for goals, CLI commands, UI direction, slices, and non-goals.

Planning checklist:

- [x] v1.5 roadmap added to `docs/roadmap.md`
- [x] Diff/plan/apply scope recorded
- [x] CLI commands recorded (`config diff`, `config plan`, `config apply --yes/--dry-run/--backup-out`, `--fail-on-drift`, `--json`)
- [x] Proposed API direction recorded (`POST /api/config/plan`)
- [x] UI import preview direction recorded (Settings / Config Import Preview: Add/Update/Remove/Unchanged counts before confirm)
- [x] Non-goals recorded (auth, remote management, cloud sync, team sharing, scheduled rules, firewall management, service install from CLI, full rollback/history, TUI, traffic graphs)
- [x] 100% meaningful coverage target recorded for v1.4 and v1.5

---

## v1.4 Roadmap

v1.4 targets live connection and session visibility: a read-only Live Connection Inspector showing active TCP connections and UDP sessions with duration, bytes, and client address. See `docs/roadmap.md` for API direction, data model, UI direction, slices, and non-goals.

Quality target: all newly added or materially changed code in v1.4 should reach 100% meaningful test coverage, with explicit coverage gates where practical. This covers CLI additions, Go service changes, TypeScript server changes, shared types, contract validators, and client-side logic.

- [ ] 100% meaningful coverage target/gate tracked for all new v1.4 Live Connection Inspector code (TCP tracking, UDP tracking, `GET /api/connections` handler, contract validator, UI helpers, CLI connections command — both runtimes)

Checklist items to add per slice as work proceeds:

- [ ] **Slice 1** — Live Connection Inspector contract and coverage strategy: `GET /api/connections` response shape finalized (`tcpConnections`, `udpSessions`, `ruleSummaries`, `generatedAt`); coverage gate plan recorded; `docs/api-contract.md` draft updated; `docs/checklist.md` updated.
- [x] **Slice 2** — Shared types and API contract validation: `LiveConnectionsResponse`, `TcpConnectionInfo`, `UdpSessionInfo`, `RuleLiveSummary`, `LiveConnectionStatus`, `UdpSessionStatus` added to `@portier/shared` (`shared/sources/connections.ts`); `validate:contract` updated with skip note for planned endpoint; `docs/api-contract.md` finalized (field directions, `lastTrafficAt` null, Shared Shapes updated); client in-app API Docs view updated with planned badge; `ApiDocsView.test.tsx` updated with 5 new tests; `connections.test.ts` added with 14 shape tests. Implementation still pending (Slices 3–7).
- [ ] **Slice 3** — TypeScript server TCP live tracking: per-rule connection registry; create entry on accept, remove on full close; bytes in/out; no double-counting on error/close race; no payload capture; 100% meaningful coverage of the tracking module.
- [ ] **Slice 4** — TypeScript server UDP session tracking: active session registry for all UDP modes; startedAt, lastSeenAt, packets/bytes; named idle/expiry constants (30s idle, 5min expire); 100% meaningful coverage including idle/expiry edge cases.
- [ ] **Slice 5** — Go service TCP live tracking: per-rule connection registry; create on accept, remove on full close; bytes in/out; no double-counting on close/error race; no goroutine leaks; 100% meaningful coverage.
- [ ] **Slice 6** — Go service UDP session tracking: active session registry for all UDP modes; named idle/expiry constants; idle detection and expiry with clean goroutine/ticker lifecycle; 100% meaningful coverage.
- [ ] **Slice 7** — `GET /api/connections` parity: both runtimes return identical response shape; 200 with empty arrays when no connections active; `ruleSummaries` covers all running rules; `validate:contract` checks parity.
- [ ] **Slice 8** — Client API and Live Connections UI: `fetchLiveConnections()` API helper; Live Connections view (table with protocol, rule, client, target, duration/idle, bytes in/out, packets, status); rule/protocol/status filters; manual refresh and auto-refresh toggle; empty state; loading/error handling; 100% meaningful coverage of helpers and display logic.
- [ ] **Slice 9** — Rule row live activity summary: compact active connections/sessions count and last-traffic age per rule row, using `GET /api/connections` data; subtle display; tests added.
- [ ] **Slice 10** — CLI `portier connections`: calls `GET /api/connections`; human aligned table; `--rule`, `--protocol`, `--json` flags; safe rule resolver reused; 100% meaningful coverage; `validate:cli:coverage` threshold maintained or raised.
- [ ] **Slice 11** — Diagnostics export integration: decide whether to include live session snapshot in CLI and UI support bundle; implement if promoted; update tests and contract if changed.
- [ ] **Slice 12** — Coverage gates and readiness audit: all coverage targets verified; TypeScript server coverage gate for new live tracking modules finalized; Go service coverage gate added or extended; no known lifecycle edge-case gaps.
- [ ] **Slice 13** — v1.4 version bump, changelog finalized, tag created, full validation suite passed (`lint`, `typecheck`, `test`, `build`, `test:e2e`, `validate:cli`, `validate:contract`, `validate:runtime:smoke`).
- [ ] **Slice 14** — Docs update: `docs/roadmap.md`, `docs/api-contract.md`, `docs/changelog.md`, `README.md`, `tools/cli/readme.md` all reflect delivered v1.4 behavior.

---

## v1.3 Roadmap

v1.3 targets native CLI and automation: a Go-based `portier` CLI that talks to the existing management API. See `docs/roadmap.md` for principles, command set, implementation structure, packaging direction, slices, and non-goals.

Checklist items to add per slice as work proceeds:

- [x] **Slice 1** — CLI strategy and command design: command set, rule lookup behavior, output modes, exit code contract, and module layout confirmed. Documented in `docs/roadmap.md`.
- [x] **Slice 2** — Go CLI skeleton and API client: `tools/cli/` module scaffolded, HTTP client (`ConnectionError`/`APIError` types), `--url`/`--host`/`--port`/`PORTIER_URL` connection options, `--json` flag, `runtime` command (human + JSON output), structured error output, 22+ tests using `httptest`, `build:cli`/`test:cli`/`validate:cli` npm scripts.
- [x] **Slice 3** — Read-only commands: `portier list`, `portier status`, `portier activity`; human table and `--json` output; `--limit`/`--rule`/`--type`/`--severity` filters for activity; output helpers (`FormatBool`, `FormatBytes`, `FormatTimestamp`, `PrintTable`); 59 CLI tests total.
- [x] **Slice 4** — Lifecycle commands: `portier start <id|name>`, `portier stop <id|name>`, `portier diagnose <id|name>`; safe rule resolver (exact ID wins, unique name match, ambiguous-name exit 2, not-found exit 1); `DiagnosticCheck`/`DiagnosticSummary`/`RuleDiagnosticsResult` types; `StartForward`/`StopForward`/`DiagnoseForward` client methods; stable JSON objects for start/stop; raw `RuleDiagnosticsResult` for diagnose; 89 CLI tests total.
- [x] **Slice 5** — Config commands: `portier config validate <file>` (local-only, all three shapes, field/binding validation); `portier config export --out <file>` (calls `GET /api/config/export`, stdout JSON mode, no partial writes); `portier config import --mode merge|replace [--yes] <file>` (local validate before API, replace requires `--yes`); `ConfigRule`/`ConfigExportResponse`/`ConfigImportRequest`/`ImportResult`/`ConfigImportResponse` types; `doWithBody` helper; 132 CLI tests total.
- [x] **Slice 6** — Diagnostics export: `portier diagnostics export --out <file>` (bundle schema, partial failure, `--run-diagnostics`, `--activity-limit`, 153 tests total).
- [x] **Slice 7** — CLI packaging: `portier`/`portier.exe` built into `build/portier/` by all platform build scripts; runtime and release validation require CLI binary; Windows installer includes `portier.exe`; `readme.txt` documents CLI usage; no PATH integration in v1.3.
- [x] **Slice 8** — v1.3 readiness audit, version bump, changelog finalized, tag created. Coverage gate added (`validate:cli:coverage`, 88% threshold, 90.1% actual); version bumped to 1.3.0; all validation suites passed.
- [x] **Post-v1.3 coverage ratchet** — CLI coverage gate raised to 92% (actual: 92.7%); 10 targeted tests added; first ratchet step toward v1.4/v1.5 100% meaningful coverage target.

---

## v1.2 Roadmap

v1.2 focuses on diagnostics and operational polish. See `docs/roadmap.md` for goals, slices, and non-goals.

Checklist items to add per slice as work proceeds:

- [x] **Slice 1** — Runtime info endpoint and UI display: `GET /api/runtime` in both runtimes, Settings Runtime/Environment section, shared `RuntimeInfo` type, contract validator updated.
- [x] **Slice 2** — Rule diagnostics API: `POST /api/forwards/:id/diagnose` in both runtimes; `docs/api-contract.md` updated; client in-app API Docs updated; API Docs tests updated.
- [x] **Slice 3** — Rule diagnostics UI: Diagnose button per rule row, inline diagnostics panel, all check states, clear on close/delete; tests added; API Docs updated.
- [x] **Slice 4** — Activity Log polish: View Activity button per rule row, filter banner, type filter, clear filters, `DELETE /api/activity` on both runtimes, Export JSON, Clear Log, packet throttle note; `docs/api-contract.md` updated; client in-app API Docs updated; contract validator updated; tests added.
- [x] **Slice 5** — Settings / runtime / config polish: copy buttons (config path, static dir, management URL), datetime export filename, export note excludes Activity Log, export success/error feedback, import mode with descriptions above file picker, replace confirm backup export button, `PORTIER_APP_VERSION` constant, version in sidebar footer; 24 new tests.
- [x] **Slice 6** — Safer networking UX: listen host presets (Local only / LAN exposed), inline LAN warning in form, platform-aware firewall note, friendly conflict error copy, improved LAN_EXPOSURE advisory message; 17 new/updated ForwardRuleForm tests.
- [x] **Slice 7** — Diagnostics export: Download Diagnostics JSON button in Settings; client-side bundle (runtime, rules, statuses, activity, UI-session diagnostics); partial-failure errors array; no backend endpoint; 216 client tests passing.
- [x] **Slice 8** — v1.2 readiness audit, version bump, changelog, tag: all validation passed; version bumped to 1.2.0; changelog finalized; tag ready.

---

## v1.1 Installer Readiness

v1.1 focuses on distribution and native OS service installers. See `docs/installer-strategy.md` for scope and implementation slices.

Checklist items to add per slice as work proceeds:

- [x] **Slice 2** — Windows Inno Setup installer: machine-wide install (`%ProgramFiles%\Portier`), optional Windows Service task, uninstall preserves `rules.json`. Scripts in `scripts/windows/release/`. Build via `npm run release:current`. Requires Inno Setup 6.
- [x] **Slice 3** — macOS LaunchAgent polish: auto-copy from `build/portier/`, `--source-dir`/`--no-start`/`--runtime` options, label bug fixed, `--purge` on uninstall, `scripts/macos/release/build-release.sh` for portable tar.gz, signing/notarization docs. Service lifecycle scripts in `scripts/macos/service/`. Build via `npm run release:current` (on macOS). Validate via `npm run validate:service:macos`.
- [x] **Slice 4** — Linux install/uninstall/start/stop/status scripts complete; `--source-dir`/`--no-enable` added to `install-service.sh`; `scripts/linux/release/build-release.sh` for portable tar.gz; service lifecycle scripts in `scripts/linux/service/`; systemd unit examples and docs updated.
- [x] **Slice 5** — `validate:service:*` scripts unified: all support `--no-build`, `--keep-files`, `--port`; test-specific names/paths/ports on all platforms; `validate:service:current` dispatches by OS with unsupported-platform error. Windows user-scope validated on Windows host. macOS/Linux validation requires the respective OS.
- [x] **Slice 6** — `release:current` and `release:portable` produce `build/releases/<platform>/` with versioned portable archives. `validate:release:portable` validates layout, required files, forbidden files, and readme.txt content. Windows zip via `Compress-Archive`; macOS/Linux tar.gz via `scripts/macos/release/build-release.sh` and `scripts/linux/release/build-release.sh`. Windows installer non-fatal if Inno Setup absent. macOS .pkg and Linux .deb/.rpm deferred.
- [x] **Slice 7** — v1.1 readiness audit passed; version bumped to `1.1.0`; changelog entry finalized; tag created.
