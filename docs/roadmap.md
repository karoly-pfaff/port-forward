# Portier Roadmap

## Release Progression

- **v1.0** — Proved the core app and runtime behavior: TCP/UDP forwarding, both runtimes, Playwright E2E, package layout.
- **v1.1** — Made Portier easier to install and distribute: native OS service installers, release artifacts, service and package validation, cross-platform polish.
- **post-v1.1** — Config compatibility fixtures (`tests/fixtures/config/`), `validate:config` runner, and Settings import/export E2E (`settings.spec.ts`) added as pre-v1.2 groundwork.
- **v1.2** — Improve operational confidence: diagnostics, visibility, safer networking UX, and runtime transparency.
- **v1.3** — Make Portier automatable: native Go CLI for terminal and script workflows, talking to the existing management API.
- **v1.4** — Make live forwarding traffic visible: read-only TCP connection and UDP session inspector.

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
- If the CLI binary is named `portier`, keep the service binary named `service` or `portier-service` consistently to avoid naming confusion.
- PATH integration is decided later.
- Windows installer could eventually offer "Add Portier CLI to PATH".
- Portable archives should include the CLI once implemented.
- Installer behavior (PATH, shell completion, separate download vs. bundled) is decided during Slice 7.

### Suggested Implementation Slices

1. ~~**CLI strategy and command design**~~ — ✓ Complete. Command set, rule lookup behavior, output modes, exit code contract, module layout, and tools/ boundary confirmed. Documented in `docs/roadmap.md`.
2. ~~**Go CLI skeleton and API client**~~ — ✓ Complete. `tools/cli/` module scaffolded; HTTP client with `ConnectionError`/`APIError` types; `--url`/`--host`/`--port`/`PORTIER_URL` connection options; `--json` flag; `runtime` command (human + JSON output); structured error output; 22+ tests using `httptest`; `build:cli`, `test:cli`, `validate:cli` npm scripts.
3. ~~**Read-only commands: `list`, `status`, `activity`**~~ — ✓ Complete. `portier list` (calls `GET /api/forwards`, human table + JSON), `portier status` (calls `GET /api/status`, joins rule names for human output), `portier activity` (calls `GET /api/activity`, `--limit`/`--rule`/`--type`/`--severity` filters, raw event array JSON); output helpers (`FormatBool`, `FormatBytes`, `FormatTimestamp`, `PrintTable`); 59 CLI tests total.
4. ~~**Lifecycle commands: `start`, `stop`, `diagnose`**~~ — ✓ Complete. `portier start <id|name>` (resolves rule, calls `POST /api/forwards/:id/start`), `portier stop <id|name>` (calls `POST /api/forwards/:id/stop`), `portier diagnose <id|name>` (calls `POST /api/forwards/:id/diagnose`, human summary + checks table); safe rule resolver (exact ID wins, exact name match, ambiguous-name error with ID list, not-found); stable `{"ok", "action", "ruleId"}` JSON for start/stop; raw `RuleDiagnosticsResult` JSON for diagnose; 89 CLI tests total.
5. ~~**Config commands: `config validate`, `config export`, `config import`**~~ — ✓ Complete. `portier config validate <file>` (local validation, no API, all three config shapes); `portier config export --out <file>` (calls `GET /api/config/export`, writes file, stdout JSON mode); `portier config import --mode merge|replace [--yes] <file>` (local validate then `POST /api/config/import`, replace requires `--yes`); `doWithBody` helper; `ConfigRule`/`ConfigExportResponse`/`ConfigImportRequest`/`ImportResult`/`ConfigImportResponse` types; 132 CLI tests total.
6. Diagnostics export command
7. CLI packaging into runtime/release artifacts
8. CLI validation, documentation, readiness audit, version bump, tag

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

v1.0 proved the core forwarding runtime. v1.1 made Portier installable. v1.2 added diagnostics and operational visibility. v1.3 is planned to make Portier automatable from the CLI. v1.4 should make live forwarding traffic visible in real time — answering who is connected, which rules are carrying traffic, and how much data has passed.

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
  "connections": [
    {
      "id": "string",
      "ruleId": "string",
      "ruleName": "string",
      "protocol": "tcp",
      "clientAddress": "127.0.0.1",
      "clientPort": 52344,
      "targetAddress": "127.0.0.1",
      "targetPort": 5432,
      "startedAt": "ISO string",
      "durationSeconds": 42,
      "bytesIn": 1024,
      "bytesOut": 4096,
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
      "clientAddress": "192.168.1.25",
      "clientPort": 51000,
      "targetAddress": "1.1.1.1",
      "targetPort": 53,
      "startedAt": "ISO string",
      "lastSeenAt": "ISO string",
      "idleSeconds": 12,
      "packetsIn": 5,
      "packetsOut": 5,
      "bytesIn": 300,
      "bytesOut": 420,
      "status": "active | idle"
    }
  ],
  "generatedAt": "ISO string"
}
```

Exact shape should be finalized before implementation. Field names should align with existing status/stat naming where practical. Connection IDs need to be stable for display during a session but do not need to persist across restarts.

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

**Privacy and safety:**
- This is local management visibility only.
- Client IP and port information is operationally necessary but must not be uploaded or sent anywhere.
- Diagnostics export may eventually include a live session snapshot only if user-triggered.

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

If the v1.3 CLI exists, v1.4 may add:

```
portier connections
portier connections --rule <id|name>
portier connections --json
portier sessions
portier sessions --rule <id|name>
```

Do not expand v1.3 CLI scope prematurely. These are recorded here for planning only.

### Diagnostics Export Integration

The diagnostics export/support bundle introduced in v1.2 may eventually include:
- Current live TCP connections
- Current UDP sessions
- A `generatedAt` timestamp for the session snapshot
- A note that this is a user-triggered local export

Decide during Slice 9 whether to include this in v1.4 or defer to a later release.

### Suggested Implementation Slices

1. Connection/session API strategy and shared contract
2. TCP active connection tracking
3. UDP session visibility polish
4. `GET /api/connections` implementation in TypeScript server and Go service
5. Contract validation and API Docs update
6. Live Connections UI
7. Rule row live traffic summary
8. CLI commands for live connections, if v1.3 CLI exists
9. Diagnostics export integration, if desired
10. v1.4 readiness audit, version bump, changelog, tag

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
