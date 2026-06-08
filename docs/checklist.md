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

## v1.4 Roadmap

v1.4 targets live connection and session visibility: a read-only Live Connection Inspector showing active TCP connections and UDP sessions with duration, bytes, and client address. See `docs/roadmap.md` for API direction, data model, UI direction, slices, and non-goals.

Checklist items to add per slice as work proceeds:

- [ ] **Slice 1** — Connection/session API strategy: response shape finalized, `GET /api/connections` vs. `/api/forwards/:id/connections` decided, shared types added to `@portier/shared`, `docs/api-contract.md` updated draft.
- [ ] **Slice 2** — TCP active connection tracking: per-rule connection registry in both runtimes, client/target address, start time, bytes in/out, clean removal on socket close, no double-count on error.
- [ ] **Slice 3** — UDP session visibility polish: active session registry for `bidirectional-multi-client`, first seen/last seen, packets/bytes, idle seconds; document limitations for `one-way` and `bidirectional-last-client`.
- [ ] **Slice 4** — `GET /api/connections` in TypeScript server and Go service: same response shape, generatedAt, 200 even when no connections.
- [ ] **Slice 5** — Contract validation and API Docs: `validate:contract` updated, `docs/api-contract.md` finalized, client in-app API Docs view updated, `ApiDocsView.test.tsx` updated.
- [ ] **Slice 6** — Live Connections UI: dedicated view or Activity subtab; table-based (protocol, rule, client, target, duration, bytes, packets, status); rule/protocol filters; auto-refresh toggle.
- [ ] **Slice 7** — Rule row live traffic summary: compact active connections/sessions count and last-traffic age per rule row, using `GET /api/connections` data.
- [ ] **Slice 8** — CLI commands for live connections, if v1.3 CLI exists: `portier connections`, `portier sessions`, `--rule` filter, `--json`.
- [ ] **Slice 9** — Diagnostics export integration: decide whether to include live session snapshot in the support bundle.
- [ ] **Slice 10** — v1.4 readiness audit, version bump, changelog finalized, tag created.

---

## v1.3 Roadmap

v1.3 targets native CLI and automation: a Go-based `portier` CLI that talks to the existing management API. See `docs/roadmap.md` for principles, command set, implementation structure, packaging direction, slices, and non-goals.

Checklist items to add per slice as work proceeds:

- [x] **Slice 1** — CLI strategy and command design: command set, rule lookup behavior, output modes, exit code contract, and module layout confirmed. Documented in `docs/roadmap.md`.
- [x] **Slice 2** — Go CLI skeleton and API client: `tools/cli/` module scaffolded, HTTP client (`ConnectionError`/`APIError` types), `--url`/`--host`/`--port`/`PORTIER_URL` connection options, `--json` flag, `runtime` command (human + JSON output), structured error output, 22+ tests using `httptest`, `build:cli`/`test:cli`/`validate:cli` npm scripts.
- [x] **Slice 3** — Read-only commands: `portier list`, `portier status`, `portier activity`; human table and `--json` output; `--limit`/`--rule`/`--type`/`--severity` filters for activity; output helpers (`FormatBool`, `FormatBytes`, `FormatTimestamp`, `PrintTable`); 59 CLI tests total.
- [x] **Slice 4** — Lifecycle commands: `portier start <id|name>`, `portier stop <id|name>`, `portier diagnose <id|name>`; safe rule resolver (exact ID wins, unique name match, ambiguous-name exit 2, not-found exit 1); `DiagnosticCheck`/`DiagnosticSummary`/`RuleDiagnosticsResult` types; `StartForward`/`StopForward`/`DiagnoseForward` client methods; stable JSON objects for start/stop; raw `RuleDiagnosticsResult` for diagnose; 89 CLI tests total.
- [ ] **Slice 5** — Config commands: `portier config export --out <file>`, `portier config import <file> --mode merge|replace [--yes]`; tests.
- [ ] **Slice 6** — Diagnostics export: `portier diagnostics export --out <file>`; tests.
- [ ] **Slice 7** — CLI packaging: CLI binary included in `build/portier/`, portable archives, and optionally Windows installer; binary naming (`portier`/`portier.exe`) confirmed alongside `service`/`service.exe`.
- [ ] **Slice 8** — v1.3 readiness audit, version bump, changelog finalized, tag created.

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
