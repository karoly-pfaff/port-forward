# Portier Roadmap

## Release Progression

- **v1.0** — Proved the core app and runtime behavior: TCP/UDP forwarding, both runtimes, Playwright E2E, package layout.
- **v1.1** — Made Portier easier to install and distribute: native OS service installers, release artifacts, service and package validation, cross-platform polish.
- **post-v1.1** — Config compatibility fixtures (`tests/fixtures/config/`), `validate:config` runner, and Settings import/export E2E (`settings.spec.ts`) added as pre-v1.2 groundwork.
- **v1.2** — Improve operational confidence: diagnostics, visibility, safer networking UX, and runtime transparency.
- **v1.3** — Make Portier automatable: native Go CLI for terminal and script workflows, talking to the existing management API.
- **v1.4** — Make live forwarding traffic visible: read-only TCP connection and UDP session inspector.
- **v1.5** — Make Portier configuration repeatable and reviewable: plan/diff/apply workflows for comparing desired config with the running state and applying changes safely.
- **v1.6** — Dedicated audit and hardening release: structured multi-angle inspection of architecture, parity, forwarding correctness, API contract, CLI, UI, test quality, security posture, packaging, and documentation. The 100% meaningful coverage push in v1.4 and v1.5 is the prerequisite safety net for this work.

---

## Portier v1.2 — Diagnostics & Operational Polish

**Core theme: Make Portier easier to trust while it is running.**

v1.0 and v1.1 established what Portier is and how to get it running. v1.2 focuses on what happens after installation — giving users the tools to understand what the service is doing, verify that rules are working, and act on problems quickly.

### Goals

#### Runtime Info and Transparency

Surface what is actually running, not just that it is running.

Possible fields:
- Runtime implementation: Go service or Node/TypeScript server
- Version
- Platform (OS and architecture)
- Uptime
- Config path
- Static dir
- Management host and port
- Package or install mode, if detectable

Possible API direction:
- Add `GET /api/runtime` with runtime info
- Or extend the existing `GET /api/status` endpoint if that is cleaner

UI: display in the Settings view alongside management endpoint info. Keep it read-only and compact.

#### Rule Diagnostics

Add a per-rule check flow that lets users verify a rule before or after starting it.

For TCP:
- Check whether the listen host/port can bind
- Check whether the target host resolves
- Check whether the target port is reachable
- Report connection timeout or failure clearly

For UDP:
- Check whether the listen host/port can bind
- Check whether the target host resolves
- Explain the selected UDP mode
- Show a warning for `bidirectional-last-client` limitations
- Show active session count for `bidirectional-multi-client` if relevant

UI direction:
- Add a **Diagnose** or **Test** button per rule in the rules table or edit drawer
- Show results as clear pass / warn / fail rows with short descriptions
- Do not block rule creation — this is informational, not a gatekeeper

API direction:
- `POST /api/forwards/:id/diagnose` — run checks and return structured results
- Results include: step name, status (pass/warn/fail), message

#### Safer Networking UX

Improve how the UI communicates network safety decisions.

Ideas:
- Stronger inline LAN exposure warning when `listenHost` is `0.0.0.0`, visible in the form and rule row
- Clearer common/privileged port warnings in the add/edit form
- **Local-only preset** — fills `listenHost` with `127.0.0.1` and labels it clearly
- **LAN-exposed preset** — fills `listenHost` with `0.0.0.0` and shows an explicit warning before the user saves
- Better duplicate binding/conflict error message (what is already bound, not just that it is bound)
- Windows-specific note in the UI where relevant: forwarded LAN ports may require a Windows Firewall rule

#### Activity Log Polish

Make the Activity Log more useful during real operation.

Ideas:
- Per-rule activity filter: "show events for this rule"
- "Last events for this rule" link from the rules table row
- Clear activity log button
- Export activity log as JSON or text
- Better event grouping or throttle display (e.g., "UDP: 1,240 packets in the last 10s" instead of raw entries)
- Keep packet stats accurate while avoiding activity event spam

#### Settings / Runtime / Config Polish

Improve the Settings view as an operational control room.

Ideas:
- Show config path (current, from runtime or env)
- Copy config path to clipboard
- Show runtime info (version, platform, uptime) — linked to `GET /api/runtime`
- Show service binary or static dir path if available
- Better import/export UX: clearer replace vs. merge warnings, validation preview improvements
- Link to platform-specific install docs from Settings
- Show current app version in the UI footer or Settings

#### Diagnostics Export

Add a simple diagnostics export for support and debugging.

Possible contents:
- Runtime info (version, platform, uptime, config path, static dir)
- Current rules (same as config export)
- Rule statuses and traffic stats
- Recent activity (last N events)
- No secrets expected, but avoid capturing sensitive local paths unless the user intentionally includes them

Possible API direction:
- `GET /api/diagnostics` — returns a structured JSON bundle
- Or a client-side "copy diagnostics" that assembles info from existing endpoints

---

### Suggested Implementation Slices

1. ~~**Runtime info endpoint and UI display**~~ — ✓ Complete. `GET /api/runtime` in both runtimes, Settings Runtime/Environment section, shared `RuntimeInfo` type, contract validator updated.
2. ~~**Rule diagnostics API**~~ — ✓ Complete. `POST /api/forwards/:id/diagnose` in both runtimes, shared `RuleDiagnosticsResult` type, structured pass/warn/fail/skip checks, contract validator updated, `docs/api-contract.md` updated, client in-app API Docs updated, API Docs tests added.
3. ~~**Rule diagnostics UI**~~ — ✓ Complete. Diagnose button (stethoscope icon) in every rule row; inline collapsible diagnostics panel with pass/warn/fail/skip display; loading, error, and clear states; duplicate-click prevention; delete clears diagnosis. 133 client tests passing.
4. ~~**Activity Log polish**~~ — ✓ Complete. View Activity button per rule row, rule filter banner, type filter, clear filters, `DELETE /api/activity` on both runtimes, Export JSON (client-side), Clear Log button, packet throttle note, `docs/api-contract.md` updated, client in-app API Docs updated, contract validator updated.
5. ~~**Settings / runtime / config polish**~~ — ✓ Complete. Copy buttons for config path, static dir, management URL; datetime export filename; export note excludes Activity Log; export success/error feedback; import mode with descriptions above file picker; replace confirm backup export button; `PORTIER_APP_VERSION` constant; version in sidebar footer. 24 new tests.
6. ~~**Safer networking UX pass**~~ — ✓ Complete. Listen host presets (Local only / LAN exposed), inline LAN warning in form, platform-aware firewall note, friendly conflict error copy, improved LAN_EXPOSURE advisory message. `ForwardRuleForm` accepts optional `runtimePlatform` prop. 17 new/updated tests.
7. ~~**Diagnostics export**~~ — ✓ Complete. Client-side bundle: runtime, rules, statuses, activity, UI-session diagnostics. Download Diagnostics JSON button in Settings. Partial-failure handling with `errors` array. No backend endpoint. 216 tests passing.
8. ~~**v1.2 readiness audit, version bump, changelog, tag**~~ — ✓ Complete. All slices shipped; version bumped to 1.2.0; changelog finalized; tag ready.

---

### Non-Goals for v1.2

The following are explicitly out of scope for v1.2:

- Auto-update or self-update behavior
- Tray app or desktop GUI wrapper
- Full desktop GUI redesign
- Firewall auto-rule management (creating or modifying OS firewall rules)
- Homebrew, winget, or Chocolatey package registry submissions
- `.deb` / `.rpm` packages unless explicitly promoted from deferred status
- Cloud sync of rules or config
- User accounts or authentication
- Remote management (managing Portier from another machine)
- Telemetry of any kind
- New forwarding protocols
- Large UI redesign or component framework swap

---

## Portier v1.3 — Native CLI & Automation

**Core theme: Make Portier automatable from the terminal and scripts.**

v1.0 proved the core forwarding runtime. v1.1 made Portier installable and distributable. v1.2 added diagnostics and operational visibility. v1.3 adds a Go-based `portier` CLI that gives script-friendly access to runtime info, rules, status, activity, diagnostics, and config operations without replacing the web UI.

### CLI Principles

- Implemented in Go; distributed as a small native binary alongside the Go service.
- An API client — talks to the existing local management API. Not another Portier runtime.
- Default management URL: `http://127.0.0.1:47831`
- Works with both runtimes because both expose the same API contract:
  - Go service (`service` / `service.exe`)
  - TypeScript server (`server.js`)
- Does not replace the web UI.
- Does not duplicate forwarding engine logic.
- Does not directly edit `rules.json` in the first release.
- Does not manage OS service install/uninstall in the first release.

### Initial Command Set

**Read-only:**

```
portier runtime
portier list
portier status
portier activity
```

**Lifecycle and diagnostics:**

```
portier start <id|name>
portier stop <id|name>
portier diagnose <id|name>
```

**Config:**

```
portier config export --out rules.json
portier config import rules.json --mode merge
portier config import rules.json --mode replace --yes
```

**Diagnostics export:**

```
portier diagnostics export --out portier-diagnostics.json
```

### Connection and Output

**Connection options:**

- `--url` — full management URL override
- `--host` — management host
- `--port` — management port
- `PORTIER_URL` — environment variable override

**Output modes:**

- Human-readable table/text by default
- `--json` for scripting

**Exit codes:**

- `0` — success
- Non-zero for: API error, invalid arguments, connection failure, ambiguous rule name, destructive command missing confirmation

**Rule lookup:**

- Exact ID match first, then exact name match
- Ambiguous name → fail and list matching rules
- No fuzzy matching in the first version

### Implementation Structure

Preferred Go module layout:

```text
tools/
  cli/
    go.mod
    readme.md
    sources/
      main.go
      client/
      commands/
      output/
```

- The CLI lives under `tools/cli/` — it is a user-facing tool, not the background runtime.
  - Not under `service/` — the CLI must remain an API client, not part of service internals.
  - Not under `scripts/` — `scripts/` is for repo build, release, and service automation, not user-facing programs.
  - Not top-level `cli/` — `tools/` leaves room for future tools without cluttering the repo root.
- A separate Go module (`tools/cli/go.mod`) is preferred initially for clear boundaries.
- Reuse shared concepts from the Go service where practical.
- Avoid tightly coupling the CLI to service internals.
- HTTP request/response handling must stay aligned with `docs/api-contract.md`.
- The CLI is an API client — it does not run a forwarding engine or expose its own API.

### Tools Directory

The `tools/` directory holds user-facing or developer-facing project tools — programs that interact with Portier or its artifacts but are not the background runtime itself.

**Current (v1.3):**

```text
tools/
  cli/    — the portier CLI (API client for the local management API)
```

**Possible future tools (not part of v1.3 unless explicitly promoted):**

```text
tools/bench/    — future lightweight forwarding benchmark / load-smoke helper
tools/replay/   — future scenario/activity replay helper
```

**`tools/bench/` — future benchmarking helper:**
- Lightweight forwarding benchmark or load-smoke helper.
- Possible uses: TCP connection smoke/load, UDP packet smoke/load, counter/stat sanity checks, release confidence testing.
- Should not be included in normal user installers by default.
- Not part of v1.3 unless explicitly promoted.

**`tools/replay/` — future scenario replay helper:**
- Scenario or activity replay helper.
- Possible uses: replay activity snapshots, run scripted forwarding scenarios, demo and debug reproducible scenarios.
- Should stay clearly separated from the main CLI until the use case is real.
- Not part of v1.3 unless explicitly promoted.

**Tools vs. scripts and runtimes — boundary table:**

| Path | Role |
|---|---|
| `tools/cli/` | User-facing CLI — talks to the management API |
| `tools/bench/` | Future benchmarking/load-smoke helper |
| `tools/replay/` | Future scenario replay helper |
| `scripts/build-runtime.js` | Repo build automation — not a user-facing tool |
| `scripts/windows/service/` | OS service helper scripts — not CLI commands |
| `service/` | Background runtime — not a tool, not a CLI |
| `server/` | TypeScript reference/fallback runtime — not a tool |
| `client/` | Web UI — not a CLI tool |

### Packaging Direction

Runtime and release artifacts may eventually include:

```text
<install-dir>/
  portier          (or portier.exe)    # CLI
  service          (or service.exe)    # background service
  server.js                            # Node fallback
  web/
  readme.txt
```

Notes:
- CLI binary is `portier`/`portier.exe`; service binary remains `service`/`service.exe`.
- PATH integration is not included in v1.3. Users invoke the CLI with a full path or add the install directory to PATH manually.
- Windows installer (`portier.iss`) includes `portier.exe` in the installed files.
- Shell completion deferred to a future release.
- This layout is implemented as of Slice 7.

### Suggested Implementation Slices

1. ~~**CLI strategy and command design**~~ — ✓ Complete. Command set, rule lookup behavior, output modes, exit code contract, module layout, and tools/ boundary confirmed. Documented in `docs/roadmap.md`.
2. ~~**Go CLI skeleton and API client**~~ — ✓ Complete. `tools/cli/` module scaffolded; HTTP client with `ConnectionError`/`APIError` types; `--url`/`--host`/`--port`/`PORTIER_URL` connection options; `--json` flag; `runtime` command (human + JSON output); structured error output; 22+ tests using `httptest`; `build:cli`, `test:cli`, `validate:cli` npm scripts.
3. ~~**Read-only commands: `list`, `status`, `activity`**~~ — ✓ Complete. `portier list` (calls `GET /api/forwards`, human table + JSON), `portier status` (calls `GET /api/status`, joins rule names for human output), `portier activity` (calls `GET /api/activity`, `--limit`/`--rule`/`--type`/`--severity` filters, raw event array JSON); output helpers (`FormatBool`, `FormatBytes`, `FormatTimestamp`, `PrintTable`); 59 CLI tests total.
4. ~~**Lifecycle commands: `start`, `stop`, `diagnose`**~~ — ✓ Complete. `portier start <id|name>` (resolves rule, calls `POST /api/forwards/:id/start`), `portier stop <id|name>` (calls `POST /api/forwards/:id/stop`), `portier diagnose <id|name>` (calls `POST /api/forwards/:id/diagnose`, human summary + checks table); safe rule resolver (exact ID wins, exact name match, ambiguous-name error with ID list, not-found); stable `{"ok", "action", "ruleId"}` JSON for start/stop; raw `RuleDiagnosticsResult` JSON for diagnose; 89 CLI tests total.
5. ~~**Config commands: `config validate`, `config export`, `config import`**~~ — ✓ Complete. `portier config validate <file>` (local validation, no API, all three config shapes); `portier config export --out <file>` (calls `GET /api/config/export`, writes file, stdout JSON mode); `portier config import --mode merge|replace [--yes] <file>` (local validate then `POST /api/config/import`, replace requires `--yes`); `doWithBody` helper; `ConfigRule`/`ConfigExportResponse`/`ConfigImportRequest`/`ImportResult`/`ConfigImportResponse` types; 132 CLI tests total.
6. ~~**Diagnostics export command**~~ — ✓ Complete. `portier diagnostics export --out <file>` (builds local JSON bundle from runtime/rules/statuses/activity; `--run-diagnostics` adds per-rule diagnostics; `--activity-limit` 1–500; partial failures recorded in `errors[]`, bundle still written; human output shows counts and "with warnings" on errors; JSON result object or raw bundle to stdout; `diagnosticsBundle` schema with `schemaVersion`/`metadata.source`/`errors`); 153 CLI tests total.
7. ~~**CLI packaging into runtime/release artifacts**~~ — ✓ Complete. `portier`/`portier.exe` built and included in `build/portier/` alongside `service`/`service.exe`; all three platform build scripts (`build-runtime.ps1`, `build-runtime.sh` macOS/Linux) build the CLI from `tools/cli/` directly into the output directory; runtime validation requires CLI binary (non-empty, separate from service); release archive validation requires CLI binary; Windows installer (`portier.iss`) includes `portier.exe`; `build:clean` removes `tools/cli/build/`; `readme.txt` in the runtime package documents CLI usage and all six key commands; no PATH integration in v1.3.
8. ~~**CLI validation, documentation, readiness audit, version bump, tag**~~ — ✓ Complete. Coverage gate (`validate:cli:coverage`, threshold 88%); coverage tests filling all meaningful gaps (90.1% total); version bumped to 1.3.0; changelog finalized; all validation suites passed.
9. ~~**CLI coverage gate ratchet (post-v1.3)**~~ — ✓ Complete. Gate raised from 88% to 92% (actual: 92.7%); 10 targeted tests added covering parse-error paths, write-failure paths, `validateURL` parse error, `validateLocalConfig` target field errors, `RunDiagnose` API error, and `buildDiagnosticsBundle` GetStatus failure. First ratchet step toward the v1.4/v1.5 100% meaningful coverage target.

### Non-Goals for v1.3

- TUI
- Tray app
- Remote authentication
- Multi-user management
- Cloud sync
- Auto-update
- Firewall management
- OS service installer control
- Direct config file editing without the API
- Replacing the web UI
- Shell completion unless explicitly added later

---

## Portier v1.4 — Live Connection Inspector

**Core theme: Make live forwarding traffic visible and understandable.**

v1.0 proved the core forwarding runtime. v1.1 made Portier installable. v1.2 added diagnostics and operational visibility. v1.3 added a native Go CLI for terminal and script workflows. v1.4 makes live forwarding traffic visible in real time — answering who is connected, which rules are carrying traffic, and how much data has passed.

### Product Value

Simple rule-set profiles would mostly reorganize existing rules. The Live Connection Inspector answers operational questions that have no answer today:

- Who is connected right now?
- Which rule is carrying traffic?
- How long has this connection or session been active?
- How many bytes and packets have passed?
- When was this UDP client last seen?
- Is a rule idle or actively being used?

This provides debugging, observability, and safety value. It complements v1.2 diagnostics: diagnostics explain whether a rule *can* work; the live inspector shows what is happening *right now*.

### Proposed API Direction

**Primary endpoint:**

```
GET /api/connections
```

**Optional rule-scoped endpoint:**

```
GET /api/forwards/:id/connections
```

Notes:
- Endpoint naming should be finalized during Slice 1.
- Both TypeScript server and Go service must expose the same response shape.
- Read-only first. Closing or killing active connections is explicitly out of scope for the first version unless promoted later.

**Suggested response shape:**

```json
{
  "generatedAt": "2026-06-08T12:00:00.000Z",
  "tcpConnections": [
    {
      "id": "string",
      "ruleId": "string",
      "ruleName": "string",
      "protocol": "tcp",
      "clientAddress": "127.0.0.1",
      "clientPort": 54321,
      "targetAddress": "127.0.0.1",
      "targetPort": 5432,
      "startedAt": "2026-06-08T12:00:00.000Z",
      "durationMs": 12000,
      "bytesIn": 1024,
      "bytesOut": 2048,
      "status": "active"
    }
  ],
  "udpSessions": [
    {
      "id": "string",
      "ruleId": "string",
      "ruleName": "string",
      "protocol": "udp",
      "mode": "one-way | bidirectional-last-client | bidirectional-multi-client",
      "clientAddress": "127.0.0.1",
      "clientPort": 53000,
      "targetAddress": "1.1.1.1",
      "targetPort": 53,
      "startedAt": "2026-06-08T12:00:00.000Z",
      "lastSeenAt": "2026-06-08T12:00:05.000Z",
      "idleMs": 5000,
      "packetsIn": 10,
      "packetsOut": 8,
      "bytesIn": 1200,
      "bytesOut": 900,
      "status": "active | idle"
    }
  ],
  "ruleSummaries": [
    {
      "ruleId": "string",
      "ruleName": "string",
      "protocol": "tcp | udp",
      "activeTcpConnections": 1,
      "activeUdpSessions": 0,
      "bytesIn": 1024,
      "bytesOut": 2048,
      "packetsIn": 0,
      "packetsOut": 0,
      "lastTrafficAt": "2026-06-08T12:00:05.000Z"
    }
  ]
}
```

Field names should align with existing status/stat naming where practical. Connection IDs need to be stable for display during the process lifetime but do not persist across restarts. All data is operational metadata; payload contents are never exposed. `ruleName` is included for display convenience; use empty string when a name cannot be resolved. The exact response shape is finalized during Slice 1 and must be identical across both runtimes.

### Data Model Considerations

**TCP connections:**
- Track active TCP connections per rule: client address/port, target address/port, start time, bytes in/out, status.
- Remove a connection from the active list when both sockets close.
- Avoid double-counting on close/error events.
- Do not store long-term connection history in this slice.

**UDP sessions:**
- Expose active UDP sessions, especially for `bidirectional-multi-client` mode.
- Include client address/port, target address/port, first seen/last seen, packets/bytes in/out, idle seconds.
- `one-way` UDP may have limited per-client session data; document limitations clearly.
- `bidirectional-last-client` may expose only the most recent client summary.
- Keep existing UDP forwarding behavior unchanged.

**Retention and expiry:**
- v1.4 keeps in-memory live state only. No connection or session data is persisted to disk.
- TCP connections are removed immediately when both sockets close.
- UDP sessions are retained briefly after becoming idle, so they remain visible in the UI for a short time after traffic stops.
- Proposed defaults: idle after 30 seconds of no traffic; expire/remove from memory after 5 minutes of idle.
- These thresholds must be named constants so they are testable and the tests make the policy explicit.

**Privacy and safety:**
- This is local management visibility only.
- Client IP and port information is operationally necessary but must not be uploaded or sent anywhere.
- Diagnostics export may eventually include a live session snapshot only if user-triggered.

### Shared TypeScript Types

The following types are planned for `@portier/shared` in v1.4 Slice 2. Naming is recorded here for planning only — do not implement until Slice 2.

- `LiveConnectionsResponse` — top-level response for `GET /api/connections`
- `TcpConnectionInfo` — individual TCP connection record
- `UdpSessionInfo` — individual UDP session record
- `RuleLiveSummary` — per-rule aggregated live traffic summary
- `LiveConnectionStatus` — `"active"`
- `UdpSessionStatus` — `"active" | "idle"`

Field names should align with existing `@portier/shared` conventions. CLI and contract validator types mirror these shapes using Go structs.

### UI Direction

**Possible locations:**
- Dedicated "Connections" or "Live" sidebar view
- Subtab within the Activity view
- Forward Rules row expansion or rule detail panel
- Dashboard widget

**Preferred first UI:**
A dedicated compact Live Connections view or Activity subtab. Table-based, no charts initially.

**Table columns:**
- Protocol, rule name, client address, target address, duration, bytes in/out, packets in/out (UDP), last seen, status

**Filters:** by rule, by protocol

**Refresh:** manual refresh button and optional auto-refresh toggle

Do not add charts or graphs in the first version.

### Rule Row Live Summary

A later or same-release enhancement: rule rows show a compact live traffic summary using the same connection/session data.

Possible fields per row:
- TCP active connections count
- UDP active sessions count
- Last traffic age
- Bytes transferred since start

Keep it subtle — the primary rule state remains start/stop status.

### CLI Direction (v1.4 additions for the v1.3 CLI)

v1.3 shipped the Go CLI. v1.4 adds a `portier connections` command:

```
portier connections
portier connections --rule <id|name>
portier connections --protocol tcp|udp
portier connections --json
```

Behavior:
- Read-only; calls `GET /api/connections`.
- `--rule` accepts an exact rule ID or unique name; reuses the existing safe rule resolver.
- `--protocol` filters to `tcp` or `udp` connections/sessions; filtering is client-side initially.
- Human output: aligned table with protocol, rule, client, target, duration/idle, bytes in, bytes out, packets (UDP), status.
- `--json`: prints raw `LiveConnectionsResponse` or a stable filtered subset.
- Empty state shows a friendly message.
- A separate `portier sessions` command is deferred; `portier connections --protocol udp` covers the UDP session use case.

Do not implement in this planning task.

### Diagnostics Export Integration

The diagnostics export/support bundle introduced in v1.2 may eventually include:
- Current live TCP connections
- Current UDP sessions
- A `generatedAt` timestamp for the session snapshot
- A note that this is a user-triggered local export

Decide during Slice 9 whether to include this in v1.4 or defer to a later release.

### Suggested Implementation Slices

0. ~~**Coverage baseline**~~ — ✓ Complete. Pre-v1.4 baselines measured and documented: tools/cli 92.7% (gate 92%), client 89.2%, service 79.7%, shared 82.1%, server 71.9%. Coverage tooling added to all TypeScript workspaces. `npm run coverage:baseline` aggregates all. See `docs/coverage-baseline.md` and `docs/checklist.md`.
0a. ~~**TypeScript server forwarder coverage hardening**~~ — ✓ Complete. `tcp-forwarder.ts` raised from 68.5% to 100% statements/functions; `udp-forwarder.ts` raised from 57.7% to 84.3% statements/100% functions. Server overall: 71.9% → 79.6% stmts. 7 TCP tests and 13 UDP tests added covering lifecycle (idempotent start/stop, bind failure), connection/session events (opened/closed via onEvent), error paths (target unreachable, post-bind server error, socket error handlers), and UDP-specific behavior (forwarded rate-limiting, last-client stats and returned event, multi-client timer reset, session events). Remaining UDP gaps (multi-client send/return error callbacks, race guard) documented in `docs/coverage-baseline.md`. No behavior changed.
1. **Live Connection Inspector contract and coverage strategy** — finalize `GET /api/connections` response shape (`tcpConnections`, `udpSessions`, `ruleSummaries`, `generatedAt`); decide whether to add `GET /api/forwards/:id/connections` now or later; record shared type names and coverage gates; update `docs/api-contract.md` draft and `docs/checklist.md`.
2. ~~**Shared types and API contract validation**~~ — ✓ Complete. `LiveConnectionsResponse`, `TcpConnectionInfo`, `UdpSessionInfo`, `RuleLiveSummary`, `LiveConnectionStatus`, `UdpSessionStatus` added to `@portier/shared` (`shared/sources/connections.ts`). `validate:contract` updated with skip note for planned `GET /api/connections`. `docs/api-contract.md` finalized: field directions (`bytesIn`=client→target, `bytesOut`=target→client, `packetsIn`/`packetsOut`), `lastTrafficAt` documented as `null` (not absent), Shared Shapes section updated to note types are defined but implementation pending. Client in-app API Docs updated with planned endpoint and `Planned — v1.4` badge. `ApiDocsView.test.tsx` updated with 5 new tests. `connections.test.ts` added with 14 shape tests covering all types, statuses, modes, and the fully populated response. Runtime implementation still pending (Slices 3–7).
3. ~~**TypeScript server TCP live tracking**~~ — ✓ Complete. `TcpConnectionRegistry` added (`server/sources/connections/tcp-connection-registry.ts`): runtime-local UUIDs, `openConnection`/`addBytesIn`/`addBytesOut`/`closeConnection`/`closeConnectionsForRule`/`snapshot`/`snapshotForRule` API, immutable plain-object snapshots with `durationMs` at snapshot time. Wired into `TcpForwarder`: entry opened on accept, `bytesIn` incremented client→target, `bytesOut` incremented target→client, entry removed in `onClosed` (countedClosed guard) and eagerly in `closeBoth` on error path, `closeConnectionsForRule` called in `stop()` for deterministic cleanup. `ForwardManager` owns the shared registry, injects it into each `TcpForwarder`, and exposes `getLiveTcpConnections()` for internal use. No public `GET /api/connections` endpoint added yet. Registry: 100% stmts/branch/funcs. `tcp-forwarder.ts`: 100% stmts/funcs, 90% branch (optional-registry branches). Server overall: 79.6% → 80.55% stmts. 28 registry unit tests + 7 forwarder integration tests (real sockets).
4. ~~**TypeScript server UDP session tracking**~~ — ✓ Complete. `UdpSessionRegistry` added (`server/sources/connections/udp-session-registry.ts`): runtime-local UUID IDs, composite session keys (`ruleId:mode:clientAddress:clientPort`), `openOrTouchSession`/`recordInbound`/`recordOutbound`/`closeSession`/`closeSessionsForRule`/`pruneExpired`/`snapshot`/`snapshotForRule` API. Constants: `UDP_SESSION_IDLE_MS = 30_000`, `UDP_SESSION_EXPIRE_MS = 300_000`. Snapshot filters expired sessions without pruning; `pruneExpired` removes them explicitly. Wired into `UdpForwarder` via optional 4th constructor parameter: all three UDP modes tracked; last-client mode closes old session on client-endpoint change; multi-client mode stores `registryId` on session struct for timeout cleanup; `closeSessionsForRule` in `stop()`. `ForwardManager` owns registry, exposes `getLiveUdpSessions()`. Registry: 100% stmts/branch/funcs; 49 unit tests. `udp-forwarder.ts`: 86.3% stmts, 84% branch, 100% funcs; 9 integration tests. Server overall: 80.55% → 82.21% stmts.
5. ~~**Go service TCP live tracking**~~ — ✓ Complete. `TcpConnectionRegistry` added (`service/sources/connections/tcp_connection_registry.go`): runtime-local UUID IDs, `OpenConnection`/`AddBytesIn`/`AddBytesOut`/`CloseConnection`/`CloseConnectionsForRule`/`Snapshot`/`SnapshotForRule` API, concurrency-safe (mutex for map ops, atomic ops for byte counters). `TcpConnectionInfo` snapshots with `durationMs` at snapshot time. `NewTCPForwarderWithRegistry` constructor added; `countingWriter` extended with `onBytes func(int64)` callback; `Stop()` calls `CloseConnectionsForRule` after `wg.Wait()`. `Manager` owns registry, passes to each `TCPForwarder`, exposes `GetLiveTCPConnections()`. Registry: 98.1% stmts (100% public methods, 1 untestable rand.Read path); 26 unit tests. Forwarder: 8 integration tests. Service overall: 79.7% → 80.6% stmts.
6. ~~**Go service UDP session tracking**~~ — ✓ Complete. `UdpSessionRegistry` added (`service/sources/connections/udp_session_registry.go`): composite session keys (`ruleId:mode:clientAddress:clientPort`), `OpenOrTouchSession`/`RecordInbound`/`RecordOutbound`/`CloseSession`/`CloseSessionsForRule`/`PruneExpired`/`Snapshot`/`SnapshotForRule` API. Constants: `UDPSessionIdleDuration = 30s`, `UDPSessionExpireDuration = 5min`. Status `active`/`idle` and `idleMs` calculated at snapshot time; expired sessions filtered from snapshot without explicit prune. Wired into all three UDP modes in `UDPForwarder` (`newUDPForwarderWithRegistryAndTimeout` for test isolation, `NewUDPForwarderWithRegistry` for production); last-client mode closes previous session on client-endpoint change; multi-client mode stores `registryID` on `udpSession` struct, closes in `expireSession`, `CloseSessionsForRule` in `Stop()` as belt-and-suspenders. `Manager` owns shared `UdpSessionRegistry`, injects it into each `UDPForwarder` via `NewUDPForwarderWithRegistry`, exposes `GetLiveUDPSessions()`. Registry: 98.4% stmts (100% public methods, 2 untestable defensive paths); 33 unit tests. Forwarder: 11 integration tests (all three modes). Service overall: 80.6% → 82.1% stmts.
7. ~~**`GET /api/connections` parity across runtimes**~~ — ✓ Complete. `GET /api/connections` implemented in both TypeScript server (`server/sources/api.ts`) and Go service (`service/sources/api/api.go`); `serveConnections` builds `LiveConnectionsResponse` with `tcpConnections`, `udpSessions`, and `ruleSummaries` (all configured rules included, active or idle); `buildRuleLiveSummary` aggregates bytes/packets/lastTrafficAt per rule; `fetchLiveConnections()` added to `client/sources/api/portierApi.ts`; `validate:contract` fully checks response shape and parity; ApiDocsView Planned badge removed; 6 TypeScript server tests + 5 Go service tests added.
8. **Client API and Live Connections UI** — `fetchLiveConnections()` API helper; dedicated Live Connections view (table with protocol, rule, client, target, duration/idle, bytes in/out, packets, status); rule/protocol/status filters; manual refresh and auto-refresh toggle; empty state and loading/error handling; 100% meaningful coverage of helpers and display logic.
9. **Rule row live activity summary** — compact active connections/sessions count and last-traffic age per rule row, using `GET /api/connections` data; subtle display; tests added.
10. **CLI `portier connections`** — calls `GET /api/connections`; human aligned table; `--rule`, `--protocol`, `--json` flags; safe rule resolver reused for `--rule`; 100% meaningful coverage; `validate:cli:coverage` threshold maintained or raised.
11. **Diagnostics export integration** — decide whether to include live session snapshot in the CLI support bundle and UI Download Diagnostics JSON; implement if promoted; update relevant tests and `validate:contract`.
12. **Coverage gates and readiness audit** — verify all coverage targets met; TypeScript server coverage gate for new live tracking modules finalized; Go service coverage gate added or extended; no known gaps in lifecycle edge cases (close race, idle expiry, empty state, both runtimes).
13. **v1.4 version bump, changelog, release/tag** — bump version to 1.4.0; finalize changelog entry; tag; run full validation suite (`lint`, `typecheck`, `test`, `build`, `test:e2e`, `validate:cli`, `validate:contract`, `validate:runtime:smoke`).
14. **Update docs** — `docs/roadmap.md`, `docs/checklist.md`, `docs/api-contract.md`, `docs/changelog.md`, `README.md`, `tools/cli/readme.md`; keep details in roadmap/checklist/contract; keep README and CLI readme focused on delivered behavior.

### Quality Target for v1.4

All newly added or materially changed implementation areas should reach 100% meaningful test coverage, with explicit coverage gates where practical. Coverage should reflect real behavior and edge cases, not mechanical execution-only tests.

**Areas requiring 100% meaningful coverage:**
- TCP live connection tracking model (both runtimes)
- UDP session tracking model, including idle/expiry behavior (both runtimes)
- `GET /api/connections` handler (both runtimes)
- API contract validation scenarios for the new endpoint
- UI data-mapping and display-logic helpers for the Live Connections view
- CLI `portier connections` command
- Diagnostics export integration if changed in v1.4

**Edge cases to cover explicitly:**
- TCP close/error race: double-close, partial close, failed target connection
- UDP idle detection and expiry constant boundaries
- Empty state: no running rules, no active connections
- Both runtimes independently
- Cleanup on rule stop, service shutdown, and error paths

**Coverage gates:**
- CLI: `validate:cli:coverage` threshold is now 92% (actual: 92.7%); continue raising as v1.4 coverage is added.
- Go service: add or extend a coverage gate for new live tracking modules; use `validate:cli:coverage` as the model for the gate runner.
- TypeScript server: coverage for new modules tracked by Vitest; explicit gate decision made in Slice 12.

The CLI coverage gate was raised to 92% (actual 92.7%) as the first ratchet step toward the v1.4/v1.5 100% meaningful coverage target. v1.4 should continue raising this gate and extend the same discipline to Go service changes and TypeScript server additions.

If a tiny branch is genuinely untestable (main entry point guard, platform-specific path never reachable in CI), document the exception explicitly rather than excluding the file.

**Why this matters for v1.6:** The v1.4 and v1.5 coverage goals are a deliberate prerequisite for the v1.6 audit. High meaningful coverage creates the safety net needed to refactor, harden, and simplify confidently during the v1.6 audit phase without silently breaking forwarding behavior, API parity, or CLI workflows.

### Non-Goals for v1.4

- Closing or killing active connections
- Traffic graphs or charts
- Packet capture
- Payload inspection
- Deep protocol analysis
- Long-term traffic history
- Remote monitoring
- Telemetry or cloud upload
- Authentication or multi-user management
- Firewall management
- IDS or security scanning
- Profiles or rule-set management as the primary theme

---

## Portier v1.5 — Declarative Config & Drift Control

**Core theme: Make Portier configuration repeatable, reviewable, and safely applicable.**

v1.3 made Portier automatable from the CLI. v1.4 made live traffic visible. v1.5 adds plan/diff/apply workflows so users can compare desired config files with the currently running Portier configuration, preview changes safely, and apply them from the CLI or UI with explicit confirmation. This release makes Portier easier to automate in repeatable local workflows without adding remote management or cloud sync.

### Goals

- Compare desired config files with the running configuration.
- Preview adds, updates, removes, and unchanged rules before applying changes.
- Apply desired configuration safely with explicit confirmation.
- Support JSON output for automation and CI-style workflows.
- Detect drift between desired and running config.
- Reuse/extend existing config validation.
- Keep the workflow local-only and API-driven.

### Planned CLI Commands

```
portier config diff desired.json
portier config plan desired.json
portier config apply desired.json --yes
portier config apply desired.json --dry-run
portier config apply desired.json --backup-out current-backup.json --yes
portier --json config plan desired.json
portier config plan desired.json --fail-on-drift
```

### Proposed Exit Code Addition

- `4` = drift detected (for `--fail-on-drift` style workflows)

### Proposed API Direction

**Primary endpoint:**

```
POST /api/config/plan
```

Returns a structured diff: adds, updates, removes, and unchanged rules. Read-only — does not modify state.

Notes:
- Exact request and response shape to be finalized during Slice 1.
- Both TypeScript server and Go service must expose the same response shape.
- Applying the plan uses the existing `POST /api/config/import` with replace mode or a new dedicated apply endpoint, to be decided during Slice 1.

### UI Direction

**Settings / Config Import Preview:**

- Show Add / Update / Remove / Unchanged counts before import/apply.
- Make replace/apply behavior reviewable before confirmation.
- Reuse existing import flow; extend the preview step.

### Suggested Implementation Slices

1. Config diff/plan strategy and contract
2. Backend plan endpoint: `POST /api/config/plan` in TypeScript server
3. Go service plan parity
4. CLI `config plan` and `config diff` commands
5. CLI `config apply` with `--yes`, `--dry-run`, `--backup-out`
6. Settings import preview UI
7. Contract/config validation and coverage gates
8. v1.5 readiness audit, version bump, changelog, tag

### Quality Target for v1.5

All newly added or materially changed implementation areas should reach 100% meaningful test coverage, with explicit coverage gates where practical. This includes CLI, Go service, TypeScript server, shared config/diff/plan logic, contract validators, and client-side logic introduced by the release. Coverage should reflect real behavior and edge cases, not mechanical execution-only tests.

Prefer meaningful behavioral tests over superficial line execution. Do not exclude files just to game coverage. If a tiny branch is genuinely untestable, document the exception explicitly rather than excluding the file.

The CLI coverage gate introduced in v1.3 (`validate:cli:coverage`) should be maintained and raised where practical. Extend equivalent coverage gates to new Go service and TypeScript server areas added in v1.5.

### Non-Goals for v1.5

- Authentication
- Remote management
- Cloud sync
- Team sharing
- Profiles/rule sets as the main feature
- Scheduled rules
- Firewall management
- Service install management from CLI
- Full rollback/history store
- TUI
- Traffic graphs

---

## Portier v1.6 — Architecture, Quality & Maintainability Audit

**Core theme: Inspect the whole codebase with fresh eyes after v1.4 and v1.5 have raised coverage.**

v1.6 is a dedicated audit and hardening release, not a feature release. It will perform a structured multi-angle inspection of architecture, runtime parity, forwarding correctness, API contract quality, CLI quality, client/UI quality, test quality, security posture, packaging, and documentation consistency. Any hardening or refactoring work identified by the audit will be done in this release where safe, or deferred to a tracked backlog with explicit rationale.

### Why Coverage Comes First

The v1.4 and v1.5 quality target of 100% meaningful test coverage for new and materially changed implementation areas is a deliberate prerequisite for v1.6. The audit may identify refactoring, simplification, and hardening work across multiple components. High meaningful coverage gives the project a safety net so those changes can be made confidently without silently breaking forwarding behavior, API parity, CLI workflows, config import/export, diagnostics, packaging, or UI behavior.

Starting the audit before that safety net is in place would make follow-up hardening riskier. v1.4 and v1.5 build the net; v1.6 uses it.

### Audit Dimensions

**Architecture boundaries**

- `client` vs `server` vs `service` vs `shared` vs `tools` vs `scripts` — boundaries are clean and respected
- CLI remains an API client, not a runtime
- `service/` remains the preferred native runtime
- `server/` remains a supported reference and fallback runtime
- `tools/cli/` does not contain service internals
- `scripts/` contains only repo build and OS service automation, not user-facing programs

**Runtime parity**

- TypeScript server and Go service API behavior consistency
- Contract validation coverage and correctness
- Config import/export behavior in both runtimes
- Diagnostics endpoint behavior in both runtimes
- Activity, status, and runtime endpoints

**Forwarding correctness**

- TCP lifecycle: open, bidirectional pipe, close, error cleanup, no double-close
- UDP modes: one-way, bidirectional-last-client, bidirectional-multi-client
- Cleanup behavior on rule stop, service shutdown, and error paths
- Edge cases: rapid start/stop, duplicate bindings, unreachable targets
- Resource leaks: socket handles, goroutines, Node streams

**API contract quality**

- `docs/api-contract.md` accuracy and completeness
- Client in-app API Docs (`ApiDocsView.tsx`) consistency with the contract
- `validate:contract` coverage and correctness
- Endpoint error shapes: consistent `errors[]` structure, appropriate status codes

**CLI quality**

- Command UX and flag naming consistency
- Exit code contract accuracy (`0/1/2/3/4`)
- JSON output stability and schema consistency
- Rule resolver safety: ID vs name precedence, ambiguity detection
- Config, diff, plan, and apply command workflows (v1.5)
- Diagnostics export data boundaries
- Coverage gate value (92% minimum as of post-v1.3 ratchet; continue raising through v1.4/v1.5)

**Client/UI quality**

- Component boundaries and state management
- Error, empty, and loading state coverage
- Accessibility basics: labels, keyboard navigation, focus management
- Import preview and live inspector UX (v1.5/v1.4 additions)

**Test quality**

- Meaningful coverage vs. mechanical line execution
- Edge-case coverage in forwarding, config, and API paths
- Contract test accuracy against real runtime behavior
- Fixture quality and completeness
- E2E coverage: golden-path and key edge cases
- No shallow coverage-gaming tests in the codebase

**Complexity and maintainability**

- Large files or functions without clear responsibility
- Duplicated logic between runtimes, commands, or components
- Unclear names or implicit coupling
- Dead code or unused exports
- Unnecessary abstractions
- Refactor candidates identified and prioritized

**Security and safety posture**

- Management API bind behavior: localhost-only by default
- LAN exposure warnings: present, accurate, and not suppressible
- Config import safety: validation before API, replace confirmation
- Diagnostics export data boundaries: no secrets, env vars, or OS-user info
- No telemetry or upload behavior present or inadvertently added
- No accidental secret/env/log inclusion in exports or bundles

**Packaging and release quality**

- Runtime layout correctness: `portier`, `service`, `server.js`, `web/`, `readme.txt`
- Portable archive contents and forbidden-file checks
- Installer contents and upgrade behavior
- CLI and service binary separation
- Validation script coverage and correctness
- Platform docs accuracy

**Documentation consistency**

- `README.md` matches current behavior and feature set
- `AGENTS.md` is accurate for agent workflows
- `CLAUDE.md` guidance is current
- `docs/roadmap.md` reflects delivered and planned work
- `docs/checklist.md` is actionable and current
- `docs/changelog.md` accurately reflects each release
- `docs/api-contract.md` matches the live API in both runtimes
- `tools/cli/readme.md` matches CLI behavior

### Suggested Implementation Slices

1. Audit plan and scoring rubric
2. Architecture boundary audit
3. Runtime/API parity audit
4. Forwarding lifecycle and resource cleanup audit
5. CLI and automation audit
6. Client/UI quality and accessibility audit
7. Test quality and coverage audit
8. Security/safety and diagnostics data-boundary audit
9. Packaging and release validation audit
10. Hardening backlog and prioritized fix plan
11. Small safe hardening fixes
12. v1.6 readiness audit, version bump, changelog, tag

Any major refactors identified by the audit that carry meaningful risk will be deferred to a separate version with a tracked rationale rather than folded into v1.6 under time pressure.

### Non-Goals for v1.6

- New large user-facing features
- Remote management
- Authentication
- Cloud sync
- Team sharing
- Traffic graphing as a primary feature
- TUI
- Major architecture rewrite without an audit-backed plan
- Removing or deprecating `server/`
- Replacing the web UI framework
- Changing the runtime/release layout unless the audit identifies a clear issue
