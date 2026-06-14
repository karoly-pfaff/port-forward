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
- **v1.7** — Post-v1.6 cleanup and maintainability release: reduce remaining TS/Go asymmetry, simplify high-complexity modules flagged by the v1.6 audits, polish CLI/UI/operator clarity, and harden release/CI hygiene — all while keeping the v1.6 contract stable and avoiding speculative rewrites.
- **v1.8** — Operator Power Tools release: features that make Portier more useful in daily operation — rule groups/profiles, rule health/last-start diagnostics, rule duplication/templates, safer config-import UI, and a bounded recent-connection history — while keeping the API backward-compatible and avoiding scope creep into a full reverse-proxy platform.
- **v1.9** — Doctor & Config Toolkit release: a companion tool layer inside the existing CLI — `portier doctor` (whole-setup diagnostics), offline `portier config doctor`, and deterministic `portier explain` / `portier migrate` ideas — composed from existing endpoints and validation, with stable diagnostic codes and CI-friendly exit codes. No new daemon, no new binary, minimal/no API change.
- **v1.10** — Automation & Policy Guardrails release: controlled automation that makes everyday forwarding safer and less repetitive — optional rule policies, best-effort preflight before start/apply, runtime-only temporary starts, CLI watch mode, optional profile switching, and automation-safe API polish — all backward-compatible, with new fields optional and migration-safe. Not "Kubernetes for ports."
- **v1.11** — Local Intelligence & Workflow Automation release: stays local-first and helps users reason about forwarding setups — explainable config diff/plan, a rule dependency/conflict analyzer, saved views/filters, local snapshots/restore points, a richer activity timeline, local automation recipes, and a local backup policy. Deterministic (not AI-generated) explanations; no remote/auth/team/cloud threat-model expansion.
- **v1.12** — Local History & Observability release: the observability/data foundation that later powers `tools/replay` — persistent (bounded, corruption-safe) activity history, bounded TCP/UDP connection-session history, a timeline/history query API, a timeline UI, a diagnostics-bundle v2, and bounded metrics/stats. Ships only a **skeleton** `tools/replay` offline analysis tool that defines and validates the exported-artifact input contract. Not the full replay tool — that is v1.13. Local-first; no remote/auth/team/cloud model.
- **v1.13** — Replay Tool & Incident Toolkit release: builds the full `tools/replay` offline analysis tool on top of the v1.12 data foundation and its versioned input-artifact contract — load exported timeline/history/diagnostics-v2 bundles, reconstruct a time window, correlate rule failures with connection/session activity and config changes, and produce deterministic human/JSON incident reports. Works entirely offline (no live runtime, never modifies runtime/config); deterministic explanations from known codes — no AI. `tools/replay` is a separate offline tool, not a `tools/cli` command.
- **v1.14** — NestJS Server Migration release: an architecture-migration release (not a feature release) that moves the TypeScript Node fallback server to a NestJS modules/controllers/services structure while preserving the existing REST API contract, error taxonomy, static client serving, config/persistence invariants, activity parity, and Go-service parity. The Go service stays the preferred runtime; the CLI and web UI work unchanged; `validate:contract` remains the parity source of truth. Not an excuse to redesign the API.
- **v1.15** — Go Service Modular Router release: the Go-service counterpart to v1.14 — reorganize the native service's HTTP/API layer into focused, `net/http`-compatible route modules with an explicit App/Dependencies struct (optionally adopting the lightweight `chi` router), splitting the monolithic `api.go` while preserving the REST contract, error envelopes, static serving, config/import/apply invariants, activity parity, and startup/shutdown semantics. Idiomatic Go (explicit wiring, small interfaces, no magic DI) — not a NestJS clone, not Fiber/fasthttp. `validate:contract` stays the TS↔Go parity source of truth.
- **v1.16** — Post-Migration Architecture & Reliability Audit release: a dedicated audit/hardening release (not a feature release) that re-runs the v1.6 audit discipline, expanded for everything added in v1.8–v1.15 — ten read-only audits (contract/runtime parity, architecture boundaries, resilience/durability, security/local-safety, observability/replay, automation/policy, testing/coverage, complexity/maintainability, docs/UX, release readiness), a synthesis + classified fix plan, and only the MUST/SHOULD fixes. Verifies NestJS↔Go parity, CLI/replay boundaries, history/replay durability, and that coverage stays meaningful — without broad rewrites or scope expansion.
- **v1.17** — Migration & Recovery release: make config, history, diagnostics, and tool artifacts safe to migrate and recover — explicit config schema versioning, a local migration command (preferred in `tools/cli`), backup-before-migration with rollback-on-failure, snapshot-restore hardening (plan/apply + dry-run, never bypassing safety checks), and versioned history/diagnostics/replay artifact schemas that reject unsupported versions. First of the Road-to-2.0 sequence.
- **v1.18** — Install, Service & Upgrade Experience release: the "great install" release — polished Windows installer + portable package, clear service install/uninstall/update flow, a validated and documented v1.x→2.0 upgrade path (config preserved + backed up), consistent version reporting everywhere, macOS/Linux install guidance, strengthened release-artifact validation, and generated checksums.
- **v1.19** — 2.0 RC Hardening release: no new features, no architecture churn — release-candidate stabilization only. Full validation matrix, a local-safety audit, a complete docs pass, the `docs/upgrade-v2.md` upgrade guide, a final deferred-items classification, and the final 2.0 readiness decision.
- **v2.0** — Stable Local-First Portier: the first version users can rely on for stable local runtime behavior and stable compatibility policies (REST API, config schema/migration, CLI/exit-codes, diagnostics/history/replay artifacts), with a robust install/upgrade process and validated release artifacts. **Explicitly local-first** — remote management, team/user/role systems, authentication, cloud sync, OAuth, and plugin frameworks are deferred beyond 2.0.

> **Status:** v1.0–v1.6 are shipped/tagged. v1.7–v1.19 and v2.0 are **planned, not yet started**.

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
1. ~~**Live Connection Inspector contract and coverage strategy**~~ — ✓ Complete. `GET /api/connections` response shape finalized; shared type names recorded; `docs/api-contract.md` updated; `docs/checklist.md` updated.
2. ~~**Shared types and API contract validation**~~ — ✓ Complete. `LiveConnectionsResponse`, `TcpConnectionInfo`, `UdpSessionInfo`, `RuleLiveSummary`, `LiveConnectionStatus`, `UdpSessionStatus` added to `@portier/shared` (`shared/sources/connections.ts`). `validate:contract` updated with skip note for planned `GET /api/connections`. `docs/api-contract.md` finalized: field directions (`bytesIn`=client→target, `bytesOut`=target→client, `packetsIn`/`packetsOut`), `lastTrafficAt` documented as `null` (not absent), Shared Shapes section updated to note types are defined but implementation pending. Client in-app API Docs updated with planned endpoint and `Planned — v1.4` badge. `ApiDocsView.test.tsx` updated with 5 new tests. `connections.test.ts` added with 14 shape tests covering all types, statuses, modes, and the fully populated response. Runtime implementation still pending (Slices 3–7).
3. ~~**TypeScript server TCP live tracking**~~ — ✓ Complete. `TcpConnectionRegistry` added (`server/sources/connections/tcp-connection-registry.ts`): runtime-local UUIDs, `openConnection`/`addBytesIn`/`addBytesOut`/`closeConnection`/`closeConnectionsForRule`/`snapshot`/`snapshotForRule` API, immutable plain-object snapshots with `durationMs` at snapshot time. Wired into `TcpForwarder`: entry opened on accept, `bytesIn` incremented client→target, `bytesOut` incremented target→client, entry removed in `onClosed` (countedClosed guard) and eagerly in `closeBoth` on error path, `closeConnectionsForRule` called in `stop()` for deterministic cleanup. `ForwardManager` owns the shared registry, injects it into each `TcpForwarder`, and exposes `getLiveTcpConnections()` for internal use. No public `GET /api/connections` endpoint added yet. Registry: 100% stmts/branch/funcs. `tcp-forwarder.ts`: 100% stmts/funcs, 90% branch (optional-registry branches). Server overall: 79.6% → 80.55% stmts. 28 registry unit tests + 7 forwarder integration tests (real sockets).
4. ~~**TypeScript server UDP session tracking**~~ — ✓ Complete. `UdpSessionRegistry` added (`server/sources/connections/udp-session-registry.ts`): runtime-local UUID IDs, composite session keys (`ruleId:mode:clientAddress:clientPort`), `openOrTouchSession`/`recordInbound`/`recordOutbound`/`closeSession`/`closeSessionsForRule`/`pruneExpired`/`snapshot`/`snapshotForRule` API. Constants: `UDP_SESSION_IDLE_MS = 30_000`, `UDP_SESSION_EXPIRE_MS = 300_000`. Snapshot filters expired sessions without pruning; `pruneExpired` removes them explicitly. Wired into `UdpForwarder` via optional 4th constructor parameter: all three UDP modes tracked; last-client mode closes old session on client-endpoint change; multi-client mode stores `registryId` on session struct for timeout cleanup; `closeSessionsForRule` in `stop()`. `ForwardManager` owns registry, exposes `getLiveUdpSessions()`. Registry: 100% stmts/branch/funcs; 49 unit tests. `udp-forwarder.ts`: 86.3% stmts, 84% branch, 100% funcs; 9 integration tests. Server overall: 80.55% → 82.21% stmts.
5. ~~**Go service TCP live tracking**~~ — ✓ Complete. `TcpConnectionRegistry` added (`service/sources/connections/tcp_connection_registry.go`): runtime-local UUID IDs, `OpenConnection`/`AddBytesIn`/`AddBytesOut`/`CloseConnection`/`CloseConnectionsForRule`/`Snapshot`/`SnapshotForRule` API, concurrency-safe (mutex for map ops, atomic ops for byte counters). `TcpConnectionInfo` snapshots with `durationMs` at snapshot time. `NewTCPForwarderWithRegistry` constructor added; `countingWriter` extended with `onBytes func(int64)` callback; `Stop()` calls `CloseConnectionsForRule` after `wg.Wait()`. `Manager` owns registry, passes to each `TCPForwarder`, exposes `GetLiveTCPConnections()`. Registry: 98.1% stmts (100% public methods, 1 untestable rand.Read path); 26 unit tests. Forwarder: 8 integration tests. Service overall: 79.7% → 80.6% stmts.
6. ~~**Go service UDP session tracking**~~ — ✓ Complete. `UdpSessionRegistry` added (`service/sources/connections/udp_session_registry.go`): composite session keys (`ruleId:mode:clientAddress:clientPort`), `OpenOrTouchSession`/`RecordInbound`/`RecordOutbound`/`CloseSession`/`CloseSessionsForRule`/`PruneExpired`/`Snapshot`/`SnapshotForRule` API. Constants: `UDPSessionIdleDuration = 30s`, `UDPSessionExpireDuration = 5min`. Status `active`/`idle` and `idleMs` calculated at snapshot time; expired sessions filtered from snapshot without explicit prune. Wired into all three UDP modes in `UDPForwarder` (`newUDPForwarderWithRegistryAndTimeout` for test isolation, `NewUDPForwarderWithRegistry` for production); last-client mode closes previous session on client-endpoint change; multi-client mode stores `registryID` on `udpSession` struct, closes in `expireSession`, `CloseSessionsForRule` in `Stop()` as belt-and-suspenders. `Manager` owns shared `UdpSessionRegistry`, injects it into each `UDPForwarder` via `NewUDPForwarderWithRegistry`, exposes `GetLiveUDPSessions()`. Registry: 98.4% stmts (100% public methods, 2 untestable defensive paths); 33 unit tests. Forwarder: 11 integration tests (all three modes). Service overall: 80.6% → 82.1% stmts.
7. ~~**`GET /api/connections` parity across runtimes**~~ — ✓ Complete. `GET /api/connections` implemented in both TypeScript server (`server/sources/api.ts`) and Go service (`service/sources/api/api.go`); `serveConnections` builds `LiveConnectionsResponse` with `tcpConnections`, `udpSessions`, and `ruleSummaries` (all configured rules included, active or idle); `buildRuleLiveSummary` aggregates bytes/packets/lastTrafficAt per rule; `fetchLiveConnections()` added to `client/sources/api/portierApi.ts`; `validate:contract` fully checks response shape and parity; ApiDocsView Planned badge removed; 6 TypeScript server tests + 5 Go service tests added.
8. ~~**Client API and Live Connections UI**~~ — ✓ Complete. `LiveConnectionsView` added (`client/sources/features/connections/LiveConnectionsView.tsx`); "Connections" nav item with `Network` icon; three tabs: TCP Connections, UDP Sessions, Rule Summary; summary strip (TCP count, UDP sessions, active rules, total traffic); client-side filters (protocol, status, rule dropdown); Rule Summary sorted active-first then by name; `lastTrafficAt null` → "Never"; auto-refresh (default on, 2s) and manual refresh; loading/error/empty states. Format helpers added: `formatDurationMs`, `formatEndpoint`, `formatTimestampOrNever`. CSS additions: connections-summary strip, connections-tab bar, conn-status-badge. 50 new LiveConnectionsView tests + format helper tests.
9. **Rule row live activity summary** — compact active connections/sessions count and last-traffic age per rule row, using `GET /api/connections` data; subtle display; tests added. *Deferred to v1.5.*
10. **CLI `portier connections`** — calls `GET /api/connections`; human aligned table; `--rule`, `--protocol`, `--json` flags; safe rule resolver reused for `--rule`; 100% meaningful coverage; `validate:coverage` threshold maintained or raised. *Deferred to v1.5.*
11. **Diagnostics export integration** — decide whether to include live session snapshot in the CLI support bundle and UI Download Diagnostics JSON; implement if promoted; update relevant tests and `validate:contract`. *Deferred to v1.5.*
12. ~~**Coverage gates and readiness audit**~~ — ✓ Complete. All v1.4 coverage targets verified; all new/changed modules at 100% meaningful coverage; coverage gates added for all 5 components (cli ≥92%, client ≥90/89/76, server ≥82/86/97, service ≥82%, shared ≥82/54/90); vitest config corrected for Windows path-case duplication; `validate:coverage` passes all gates; per-component `validate:coverage:*` scripts added.
13. ~~**v1.4 version bump, changelog, release/tag**~~ — ✓ Complete. Version bumped to 1.4.0; changelog finalized; full validation suite passed (`lint`, `typecheck`, `test`, `build`, `validate:cli`, `validate:contract`, `validate:runtime:smoke`, `validate:release:current`).
14. ~~**Update docs**~~ — ✓ Complete. `docs/roadmap.md`, `docs/checklist.md`, `docs/api-contract.md`, `docs/changelog.md`, `README.md`, `AGENTS.md`, `CLAUDE.md` all updated to reflect delivered v1.4 behavior.

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

**Coverage gates (set at v1.4.0):**
- All five components now have explicit release gates: cli ≥92%, client ≥90/89/76, server ≥82/86/97, service ≥82%, shared ≥82/54/90.
- Gates are enforced by `npm run validate:coverage`; per-component via `npm run validate:coverage:<component>`.
- Gates are release regression guards, not final targets. Raise them as coverage improves in v1.5 and beyond.
- Do not lower gates without explicit rationale. Do not remove gates to make a release pass.

v1.4 raised coverage in all components (server: 71.9%→82.88%, service: 79.7%→82.5%, client: 89.2%→90.56%) and extended the gating discipline to all five runtimes. The vitest coverage config was also corrected to avoid a Windows path-case duplication bug.

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

**Primary endpoints:**

```
POST /api/config/plan
POST /api/config/apply
```

`POST /api/config/plan` returns a structured diff: adds, updates, removes, and unchanged rules. Read-only — does not modify state.

`POST /api/config/apply` applies the desired config with explicit confirmation (`yes: true`). Supports `backup: true` to snapshot the pre-apply config.

Both TypeScript server and Go service must expose the same response shape.

### Config Comparison Semantics

Rules are matched using the following priority:

1. Match by stable rule `id` when present in both current and desired config.
2. If the desired rule has no `id`, match by identity key: `protocol + listenHost + listenPort`.
3. If no match, the desired rule is an **add** operation.
4. If a current rule has no desired match, it is a **remove** operation.
5. If matched and material fields differ, the operation is **update**.
6. If matched and all material fields are equal, the operation is **unchanged**.

**Material fields** (trigger update or destructive flag):
- `name`, `protocol`, `listenHost`, `listenPort`, `targetHost`, `targetPort`, `enabled`, `udpMode`

**Non-material fields** (never trigger update):
- Runtime status, lastError, live connections, activity events, diagnostics results, transient state

**Destructive operations:**
- `remove` is always destructive.
- `update` is destructive when any forwarding-affecting field changes: `protocol`, `listenHost`, `listenPort`, `targetHost`, `targetPort`, `udpMode`.

**Ambiguity policy:**
- Do not match by fuzzy name.
- Do not silently merge ambiguous rules.
- If identity matching is ambiguous (two desired rules share the same identity key), the plan reports an error and refuses apply.
- Plans are deterministic.

### Plan Operation Model

```
ConfigPlanOperation {
  type: "add" | "update" | "remove" | "unchanged"
  ruleId?: string
  ruleName: string
  protocol: "tcp" | "udp"
  current?: ConfigPlanRuleSnapshot
  desired?: ConfigPlanRuleSnapshot
  changes?: ConfigPlanChange[]
  destructive: boolean
}

ConfigPlanChange {
  field: string
  before: unknown
  after: unknown
}
```

### Plan Summary Model

```
ConfigPlanSummary {
  add: number
  update: number
  remove: number
  unchanged: number
  destructive: number
  hasDrift: boolean
  hasErrors: boolean
}

ConfigPlanResponse {
  generatedAt: string
  mode: "plan"
  summary: ConfigPlanSummary
  operations: ConfigPlanOperation[]
  errors: ConfigPlanError[]
  warnings: ConfigPlanWarning[]
}
```

Error examples: invalid desired config, duplicate desired rule identity, ambiguous match, invalid UDP mode, invalid port, missing required fields.

Warning examples: apply would remove rules, LAN exposure, privileged or common port advisories.

### UI Direction

**Settings / Config Import Preview:**

- Show Add / Update / Remove / Unchanged counts before import/apply.
- Make replace/apply behavior reviewable before confirmation.
- Reuse existing import flow; extend the preview step.

### Suggested Implementation Slices

1. ~~**Config diff/plan strategy and contract**~~ — ✓ Complete. Matching semantics, operation model, plan/summary types, and API contract documented. Shared TypeScript types added to `@portier/shared` (`shared/sources/plan.ts`). `POST /api/config/plan` and `POST /api/config/apply` added to `docs/api-contract.md` as Planned. Client in-app API Docs updated with planned badges. `validate:contract` updated with skip notes. `docs/e2e-coverage.md` updated with planned workflows. `tools/cli/readme.md` updated with planned CLI commands.
2. ~~**Backend plan endpoint**~~ — ✓ Complete. Pure plan engine (`server/sources/config-plan.ts`) with id-first matching, identity key fallback, ambiguous match error, 8 material field diff, destructive flag, `REMOVE_EXISTING`/`LAN_EXPOSURE` warnings. `POST /api/config/plan` endpoint added to TypeScript server; 400 for missing desired, 200 with structured errors for invalid desired. 65 engine unit tests + 11 API integration tests. `validate:contract` TS skip removed (11 real assertions). API Docs parity badge added. `docs/api-contract.md` updated.
3. ~~**Go service plan parity**~~ — ✓ Complete. `POST /api/config/plan` in Go service; parity with TypeScript plan engine behavior; `validate:contract` Go skip replaced with 11 real plan assertions (138 passed, 2 skipped across both runtimes). Pure plan engine at `service/sources/configplan/plan.go` (`BuildConfigPlan`, `ExtractRulesRaw`): id-first matching, identity key (protocol+listenHost+listenPort) fallback, 8 material field diff, destructive flag (remove always destructive; update destructive when forwarding fields change), `REMOVE_EXISTING`/`LAN_EXPOSURE` warnings, all five error codes. Route + handler in `service/sources/api/api.go`: 400 for missing `desired`, 200 with structured plan errors for invalid desired. 49 Go engine unit tests + 10 Go API integration tests. API Docs parity badge removed (both runtimes implement). Service coverage: 84.8% → 85.8% stmts.
4. ~~**CLI `config plan` and `config diff` commands**~~ — ✓ Complete. `portier config plan <file>` and `portier config diff <file>` added to the Go CLI. Both commands validate the file locally (reusing `parseLocalConfig`/`validateLocalConfig`), then call `POST /api/config/plan`. Plan output: structured summary (add/update/remove/unchanged/destructive) + per-operation listing with field-level change detail. Diff output: `+`/`~`/`-`/`=` prefixed lines with indented field changes; `--show-unchanged` flag to include unchanged rules. Both commands support `--fail-on-drift` (exit 4 when `hasDrift` true and no plan errors). Exit code priority: invalid usage (2) > connection failure (3) > plan errors (1) > drift (4) > success (0). `--json` prints raw `ConfigPlanResponse`. `configcmd.go` updated with `RunConfigPlan`, `RunConfigDiff`, `planExitCode`, `printPlanHuman`, `printDiffHuman`, `buildPlanRequest`, `opEndpoint`, `formatChangeValue`, `printPlanWarnings`, `printPlanErrors`. `client.go` extended with `ConfigPlanRuleSnapshot`, `ConfigPlanChange`, `ConfigPlanOperation`, `ConfigPlanSummary`, `ConfigPlanError`, `ConfigPlanWarning`, `ConfigPlanResponse`, `ConfigPlanDesired`, `ConfigPlanRequest`, and `PlanConfig`. New test file `configplancmd_test.go` with 35 tests. Client test additions: 5 `PlanConfig` tests. CLI coverage: 92.7% → 93.2% (gate 92%). All 5 coverage gates pass.
5. ~~**CLI `config apply` with `--yes`, `--dry-run`, `--backup-out`**~~ — ✓ Complete. `portier config apply <file>` added to the Go CLI. `POST /api/config/apply` implemented in both TypeScript server and Go service. Handler logic: plan errors → 200 ok:false; dryRun → 200 ok:true (no yes required); destructive without yes → 400; drift → replace import with ID injection for key-matched rules. `configcmd.go` extended with `RunConfigApply`, `applyExitCode`, `printApplyHuman`. `client.go` extended with `ConfigAppliedCounts`, `ConfigApplyRequest`, `ConfigApplyResponse`, `ApplyConfig`. `configapplycmd_test.go` added with 25 tests. `client_test.go` extended with 7 `ApplyConfig` tests. `api_test.go` (Go service) extended with 11 apply API tests. `scripts/validate-contract.js` updated with 9 apply assertions (replace skip); contract passes 156/156. API Docs updated (planned badge removed, response shape updated). `docs/api-contract.md` updated with implemented entry, actual request/response shapes, and behavior notes. All 5 coverage gates pass.
6. ~~**Settings UI Plan & Apply**~~ — ✓ Complete. `planHelpers.ts` (4 helpers, 17 tests); `planConfig`/`applyConfig` API helpers with tests; Plan & Apply section in SettingsView: file picker, plan preview (summary counts, errors, warnings, operation list, destructive confirmation checkbox), apply with `yes:true`, `ok:false` error path, form clear on success; 23 new unit tests in `SettingsView.test.tsx`; E2E test in `settings.spec.ts`; `portier.spec.ts`/`settings.spec.ts` E2E label exact-match fix. Client branch 90.1% (gate 90%). All 5 coverage gates pass. 32/32 E2E pass.
7. ~~**Contract/config validation and coverage gate hardening**~~ — ✓ Complete. `validate:contract` 156/156 assertions pass against both runtimes for all plan and apply scenarios. Coverage gates ratcheted to v1.5.0 values: cli ≥93%, server ≥88/90/99, client ≥94/90/79, service ≥85%, shared 100%. All new v1.5 code verified at 100% meaningful coverage.
8. ~~**v1.5 readiness audit, version bump to 1.5.0, changelog finalized, tag**~~ — ✓ Complete. Full validation suite passed: lint, typecheck, 756 unit tests, all 5 coverage gates, 156/156 contract assertions, 32/32 E2E across 9 spec files. `validate:runtime:smoke` passed. `build:release:current` produced release artifacts. Tagged 1.5.0.

### Quality Target for v1.5

All newly added or materially changed implementation areas should reach 100% meaningful test coverage, with explicit coverage gates where practical. This includes CLI, Go service, TypeScript server, shared config/diff/plan logic, contract validators, and client-side logic introduced by the release. Coverage should reflect real behavior and edge cases, not mechanical execution-only tests.

Prefer meaningful behavioral tests over superficial line execution. Do not exclude files just to game coverage. If a tiny branch is genuinely untestable, document the exception explicitly rather than excluding the file.

All five component coverage gates are now enforced via `npm run validate:coverage`. Raise cli, service, and TypeScript component gates as new v1.5 coverage is added. Do not lower gates without explicit rationale.

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

## Portier v1.6-pre — Coverage Ratchet & Quality Hardening

**Core theme: Raise the test safety net toward ~95% coverage across measurable components before starting the v1.6 architecture and quality audit.**

v1.5 shipped all product features for declarative config and drift control. Before beginning the structured multi-angle v1.6 audit, this pre-release hardening pass closes the remaining coverage gaps that exist across the CLI, TypeScript server, and Go service — gaps left over from incremental feature work in v1.4 and v1.5. The goal is to push meaningful coverage toward ~95% so that v1.6 refactoring and hardening work can be performed with confidence.

### Goals

- Raise CLI coverage from 93.2% toward 95%+.
- Raise server statement coverage from 88.7% toward 95% where meaningful.
- Raise service statement coverage from 85.8% toward 95% where meaningful.
- Preserve client statement coverage above 95% and improve branch/function coverage where meaningful.
- Preserve shared coverage at 100%.
- Do not add new product features.
- Do not change API contracts unless a real tested bug is found.
- Do not redesign the UI.

### Scope

- Coverage gap analysis across all five components.
- Meaningful behavioral tests for uncovered paths — not mechanical line-hit tests.
- Coverage gate ratcheting after stable coverage is achieved.
- Validation stability (EADDRINUSE flakiness watch).
- No new product features, no API contract changes, no forwarding behavior changes.

### Coverage Targets

| Component | v1.5.0 actual | v1.6-pre target | Notes |
|-----------|-------------|-----------------|-------|
| cli       | 93.2%       | ≥95%            | exit codes, JSON paths, edge cases |
| client    | 95.1%/90.1%/79.9% | preserve/improve | branch/function only where meaningful |
| server    | 88.7%/91.0%/99.1% | ≥95% stmts | diagnose.ts, udp-forwarder.ts, api.ts gaps |
| service   | 85.8%       | ≥90%            | api, config, manager, validation, forwarders |
| shared    | 100%        | 100%            | preserve only |

### Suggested Implementation Slices

1. **v1.6-pre tracking setup** — update docs, record baseline, identify gaps. *This slice.*
2. **Service coverage uplift** — meaningful Go service tests for api, config, manager, validation, forwarder gaps.
3. **Server coverage uplift** — meaningful TypeScript server tests for api.ts, diagnose.ts, udp-forwarder.ts gaps.
4. **CLI coverage uplift** — meaningful CLI tests for remaining exit code, JSON, and edge-case gaps.
5. **Client branch/function uplift** — meaningful React/client tests for uncovered branches in App.tsx, ActivityLogView, ForwardRuleList, etc.
6. **Gate ratchet and docs** — ratchet all gates to stable achieved values; update docs/coverage-baseline.md, changelog, checklist.
7. **v1.6-pre readiness check** — full validation suite, report coverage delta, confirm gates pass.

### Non-Goals for v1.6-pre

- New user-facing features.
- API contract changes.
- Forwarding behavior changes.
- UI redesign.
- Superficial line-hit coverage tests.
- Excluding files to inflate reported coverage.
- Lowering any existing gate.

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

---

## Portier v1.7 — Cleanup & Maintainability

**Core theme: A post-v1.6 cleanup and maintainability release.**

v1.6 focused on audit-driven correctness, parity, durability, coverage, and release readiness. v1.7 builds on that foundation by reducing remaining structural duplication, improving long-term maintainability, and polishing operator-facing workflows without destabilizing the v1.6 contract.

Primary goals:

- Reduce remaining TS/Go asymmetry where it increases maintenance cost.
- Simplify high-complexity modules identified by the v1.6 audits.
- Improve CLI/UI/operator clarity.
- Keep the v1.6 contract stable.
- Avoid speculative rewrites.
- Preserve meaningful coverage discipline.

Non-goals (release-level):

- No public API field renames for cosmetic consistency.
- No contract code generation unless separately justified.
- No socket factory abstraction just to chase unreachable branches.
- No large UI redesign.
- No release-process churn unless directly useful.

### Priority 1 — Runtime maintainability cleanup

#### 1. Go Forwarder interface + StartRule dedupe

**Why:** The v1.6 audits repeatedly flagged the Go manager's TCP/UDP branching in `StartRule`, `stopRuntime`, and status handling. TypeScript already has a forwarder strategy shape; Go can move closer to that without changing behavior.

**Scope:**
- Introduce a small Go `Forwarder` interface if it reduces duplicated TCP/UDP handling.
- Deduplicate `StartRule` TCP/UDP arms.
- Keep protocol-specific construction explicit.
- Preserve existing manager behavior, activity events, and status output.
- Add regression tests for TCP/UDP start, stop, status, and failure paths.

**Likely files:** `service/sources/manager/manager.go`, `service/sources/forwarders/tcp.go`, `service/sources/forwarders/udp.go`, `service/sources/manager/manager_test.go`

**Validation:** Go service tests, `npm run validate:coverage`, `npm run validate:contract`

#### 2. TS UDP forwarder emit facade

**Why:** The TS UDP forwarder still has repeated send/error callback blocks. Go already has clearer helper-style emission paths. v1.6 protected event values; v1.7 can safely simplify the code.

**Scope:**
- Extract small helpers for UDP packet/activity emission.
- Preserve `udp.packet.forwarded`, `udp.packet.returned`, and `udp.packet.error`.
- Preserve `lastError` behavior.
- Avoid changing UDP runtime behavior.
- Add/keep full-payload tests.

**Likely files:** `server/sources/forwarders/udp-forwarder.ts`, `server/sources/forwarders/udp-forwarder.test.ts`

**Validation:** server tests, UDP forwarder tests, `npm run validate:contract`, `npm run validate:coverage`

#### 3. Diagnose check-phase helpers

**Why:** `diagnoseRule` is readable but still a multi-phase function. v1.7 can split it into named phases without changing output.

**Scope:**
- Extract helper functions for: listen host checks, privileged/common port checks, bind checks, target checks, UDP mode checks, summary building.
- Preserve check IDs, labels, statuses, and contract output.
- Do not build a plugin framework.

**Likely files:** `server/sources/diagnose.ts`, Go diagnose equivalent if present, diagnose tests, `scripts/validate-contract.js`

**Validation:** diagnose unit tests, `npm run validate:contract`

### Priority 2 — CLI and operator polish

#### 4. CLI command-file naming cleanup

**Why:** The naming audit found `tools/cli/sources/commands/config.go` misleading: it contains URL/default-host resolution, not config commands.

**Scope:**
- Rename `config.go` to `url.go` or `connection.go`.
- Normalize command file naming convention where practical.
- Update `CLAUDE.md` / `AGENTS.md` with CLI file convention.
- No CLI behavior change.

**Likely files:** `tools/cli/sources/commands/config.go`, `tools/cli/sources/commands/configcmd.go`, `tools/cli/sources/commands/diagnosticscmd.go`, `CLAUDE.md`, `AGENTS.md`

**Validation:** `npm run validate:cli`, full tests

#### 5. CLI config loader / mapper cleanup

**Why:** The duplication audit flagged repeated config-load and mapping prelude code in CLI config commands.

**Scope:**
- Extract shared CLI helpers for: loading desired config files, validating local config input, mapping config rules into API DTOs.
- Preserve exit-code behavior.
- Do not change command names or public CLI flags.

**Likely files:** `tools/cli/sources/commands/configcmd.go`, CLI command tests

**Validation:** CLI tests, `npm run validate:cli`

#### 6. CLI exit-code normalization review

**Why:** The v1.6 audits noted minor inconsistencies around local config/read/validation failures. Not a blocker, but v1.7 can make CLI operator behavior clearer.

**Scope:**
- Review current exit-code ladder.
- Decide whether local input/config errors should consistently exit 2.
- Preserve API errors as exit 1, connection errors as exit 3.
- Update tests and CLI docs.

**Likely files:** `tools/cli/sources/commands/*`, `tools/cli/readme.md`, CLI tests

**Validation:** CLI tests, `npm run validate:cli`

### Priority 3 — UI and docs polish

#### 7. SettingsView decomposition

**Why:** The audits repeatedly flagged `SettingsView` as the largest client component with multiple concerns. Not a v1.6 blocker, but the main client maintainability target.

**Scope:** Split into focused subcomponents/hooks (API docs panel, import/export panel, config plan/apply panel, diagnostics export panel, status/runtime panel, shared settings helpers). Preserve current UI behavior, labels, accessibility, E2E selectors, and existing tests.

**Likely files:** `client/sources/views/SettingsView.tsx`, extracted components/hooks under the relevant client folder, client tests, E2E if selectors move

**Validation:** client tests, `npm run test:e2e`, `npm run validate:coverage`

#### 8. UI wording consistency pass

**Why:** The naming audit found small wording mismatches, especially "Connections" vs "Live Connections" and diagnose host/address labels.

**Scope:**
- Decide whether the sidebar should say "Connections" or "Live Connections".
- Review diagnose label consistency.
- Update docs/tests if any exact text changes.
- Avoid API field renames.

**Likely files:** client nav/view files, diagnose files only if label changes, E2E tests, `docs/glossary.md` if terminology expands

**Validation:** client tests, E2E, `npm run validate:contract` if diagnose output changes

### Priority 4 — Validation and script hardening

#### 9. validate-contract outer child-cleanup guard

**Why:** Resilience-F was deferred from v1.6. Current cleanup is good, but the script can be hardened so child runtimes are cleaned up even if an exception escapes between runtime sections.

**Scope:**
- Add an outer cleanup guard.
- Keep existing per-runtime cleanup.
- Preserve the current contract count.
- Do not change scenario behavior.

**Likely files:** `scripts/validate-contract.js`

**Validation:** `npm run validate:contract` twice, script validation

#### 10. HTTP `/start` EADDRINUSE residual follow-up

**Why:** Slice 3 (v1.6 Fix) intentionally left HTTP `/start` API tests as accepted residual unless a flake is observed. v1.7 can either leave it documented or close it with a focused helper.

**Scope:**
- Only act if a flake is observed or the helper remains simple.
- Avoid product-side retry; retry test setup only, only on EADDRINUSE.
- Preserve intentional EADDRINUSE tests.

**Likely files:** `server/sources/api.test.ts`, `service/sources/api/api_test.go`, test helper files

**Validation:** affected tests repeated, full test suite

### Priority 5 — Optional deeper seams

#### 11. Go Store interface / manager injection seam

**Why:** The SOLID/resilience audits noted that some Go manager persistence branches are harder to test because the manager holds a concrete store.

**Scope (only if it clearly improves testability without abstracting for its own sake):**
- Introduce a tiny store interface used by manager.
- Preserve concrete config store behavior.
- Add tests for currently hard-to-reach persistence/error paths.
- Avoid a dependency-injection framework.

**Likely files:** `service/sources/manager/manager.go`, `service/sources/config/config.go`, Go manager tests

**Validation:** Go service tests, coverage, contract

**Default recommendation:** Defer unless a specific v1.7 testability goal depends on it.

### Suggested Implementation Slices

**Milestone 1 — Low-risk cleanup foundation**
1. validate-contract outer child-cleanup guard
2. CLI command-file naming cleanup
3. UI/docs wording consistency pass

**Milestone 2 — Runtime symmetry**
4. Go Forwarder interface + StartRule dedupe
5. TS UDP emit facade
6. Diagnose check-phase helpers

**Milestone 3 — CLI operator polish**
7. CLI config loader / mapper cleanup
8. CLI exit-code normalization review

**Milestone 4 — Client maintainability**
9. SettingsView decomposition

**Milestone 5 — Optional testability seam**
10. Go Store interface / manager injection seam, only if still justified

The recommended first slice is **validate-contract outer child-cleanup guard** (script-only, low risk, improves release/CI hygiene, builds on the v1.6 scenario registry), then **CLI command-file naming cleanup**. The first product-code slice should be **Go Forwarder interface + StartRule dedupe**, because it discharges the largest runtime-symmetry cluster from the v1.6 synthesis.

### Acceptance Criteria for v1.7

Portier v1.7 is ready when:

- All v1.6 contract behavior remains compatible.
- `validate:contract` passes with the current or intentionally increased count.
- All coverage gates pass with no lowered gates.
- No known EADDRINUSE test flakes remain, or remaining residuals are explicitly documented.
- Go/TS runtime asymmetry is reduced in manager and UDP-forwarder code.
- CLI config-command internals are easier to maintain.
- `SettingsView` is either decomposed or explicitly deferred again with justification.
- `docs/glossary.md` remains the canonical terminology source.
- Deferred/accepted gaps are updated in durable docs.

### Items Explicitly Deferred or Avoided in v1.7

Unless a new reason appears:

- Public API field renames (`/api/forwards`, `listenHost`/`targetHost`/`clientAddress`/`targetAddress` cosmetic renames).
- Contract code generation.
- Diagnose plugin framework.
- Broad socket factory abstraction.
- Brittle tests for crypto/rand fallback or OS-level socket internals.
- Chasing impossible `main()` / `os.Exit` wrapper branches.
- Coverage gate ratcheting without meaningful behavior gain.

---

## Portier v1.8 — Groups, Health, and Faster Rule Management

**Core theme: Operator Power Tools — features that make Portier more useful in real daily operation.**

> Manage rules in groups, understand failures faster, and create related forwarding rules without repetitive setup.

v1.6 made Portier safer and more correct. v1.7 cleans up maintainability and runtime symmetry. v1.8 finally adds features that make Portier more useful in real daily operation.

Primary goals:

- Make Portier better at managing many rules.
- Improve observability and troubleshooting.
- Make config workflows more practical.
- Improve CLI automation.
- Keep the API stable.
- Avoid turning Portier into a full reverse-proxy platform.

### Priority 1 — Rule groups / profiles

**Goal:** Allow users to group rules into named profiles or logical groups (e.g. `work`, `local-dev`, `tailscale`, `tor`, `games`, `temporary`, `lab`).

**Why:** Once a user has more than a handful of rules, a flat list becomes annoying. Groups make the UI, CLI, config export/import, and plan/apply workflows much more usable.

**Scope:** Add optional group/profile metadata to rules. Possible model `group?: string` + `tags?: string[]`; recommended conservative start with just `group?: string`.

**Features:**
- Filter rules by group in UI.
- Filter rules by group in CLI.
- Start/stop all rules in a group.
- Export/import keeps group metadata.
- Config plan/apply shows group changes.
- Dashboard summarizes groups.

**API ideas:** `GET /api/forwards?group=work`, `POST /api/forwards/groups/:group/start`, `POST /api/forwards/groups/:group/stop`. Or keep the API smaller (client/CLI fetches all rules and filters locally; add group bulk start/stop later).

**Risk:** Medium — touches the shared `ForwardRule` schema and config persistence.

**Recommendation:** Good v1.8 flagship feature.

### Priority 2 — Rule health / last-start diagnostics

**Goal:** Make failed or flaky rules easier to understand.

**Why:** Portier already has status, activity, diagnose, and `lastError`; v1.8 can turn that into a clearer operator workflow. Builds naturally on v1.6 error-flow work.

**Features (per rule):** last started at, last stopped at, last error, last successful bind, last diagnose result summary, failure count since service start, and whether failure was bind, target, config, or unknown.

**UI:** A compact "Health" column or expandable rule details — e.g. `Healthy` / `Stopped` / `Failed: address already in use` / `Warning: target refused connection`.

**CLI:** `portier status --verbose`, `portier diagnose <rule> --json`, `portier health`.

**Risk:** Low to medium — most data already exists; the main risk is schema creep.

**Recommendation:** Very valuable.

### Priority 3 — Rule templates / duplicate rule

**Goal:** Make it easier to create similar forwarding rules.

**Why:** A user often creates several rules with the same listen host, protocol, UDP mode, or target host. Re-entering everything is annoying.

**Features:** duplicate an existing rule in UI; duplicate via CLI; optionally increment port; preserve group/tags if v1.8 includes groups.

**CLI ideas:** `portier duplicate <rule-id> --name "New Rule" --listen-port 48002` or `portier create --from <rule-id> --listen-port 48002`.

**UI:** "Duplicate" action in the rule row menu.

**Risk:** Low — basically a create helper with existing validation.

**Recommendation:** Good small feature; likely cheap and user-visible.

### Priority 4 — Config preview / safer import UX

**Goal:** Make import/apply safer and clearer in the UI.

**Why:** v1.5/v1.6 already added plan/apply and hardened import behavior; v1.8 can make the UI flow nicer (especially after v1.7 SettingsView decomposition).

**Features:** import file → preview changes before applying; show add/update/remove/unchanged counts; show duplicate-binding errors inline; show destructive warning clearly; allow "download backup before apply"; show exact affected rules.

**CLI:** Already mostly covered by plan/diff/apply; v1.8 brings the UI closer to CLI quality.

**Risk:** Medium on UI complexity, low on backend behavior.

**Recommendation:** Good v1.8 UI feature.

### Priority 5 — Connection history snapshot

**Goal:** Keep a short rolling history of connections/sessions, not just currently active ones.

**Why:** Live connections are useful, but many connections are too brief to catch in the UI. A short in-memory history makes troubleshooting easier.

**Scope:** In-memory only for v1.8. Keep last N events (e.g. last 100 TCP connection open/close records, last 100 UDP session open/close records). Do not persist by default.

**UI:** "Recent connections"; filter by rule; filter TCP/UDP; show duration/bytes if available later.

**API:** Possible endpoint `GET /api/connections/recent`.

**Risk:** Medium — needs bounded memory and clear cleanup.

**Recommendation:** Good, but bounded and in-memory only at first.

### Priority 6 — Import/export bundles

**Goal:** Make support/debug export more complete.

**Why:** Diagnostics export exists, but v1.8 can produce a more useful support bundle.

**Bundle could include:** runtime info, current rules, status, recent activity, recent connections (if implemented), config plan summary (optionally), environment summary, version/build info.

**CLI:** `portier diagnostics export --include-config`, `portier diagnostics export --redact`.

**Risk:** Medium — config export may contain sensitive host/port info.

**Recommendation:** Good if paired with redaction rules; otherwise defer.

### Priority 7 — Better Windows service / startup UX

**Goal:** Make the install/run/update experience smoother.

**Features:** clearer Windows service docs; a service status command; validate service install config; maybe a CLI wrapper command that prints install instructions; maybe a "run as service" smoke validation.

**Risk:** Low to medium depending on automation depth.

**Recommendation:** Useful, but not as exciting as groups/health.

### Suggested v1.8 Scope

**Must-have:**
1. Rule groups / profiles
2. Rule health / last-start diagnostics
3. Duplicate rule / rule templates

**Should-have:**
4. UI config preview / safer import flow
5. Recent connection history snapshot

**Nice-to-have:**
6. Better diagnostics export bundle
7. Windows service / startup UX polish

### Suggested Implementation Slices

**Milestone 1 — Group metadata foundation**
- Add optional `group` to the shared rule schema.
- Migrate TS + Go validation.
- Update import/export/plan/apply.
- Add contract parity tests.
- Update CLI/UI display.

**Milestone 2 — Group operations**
- UI group filter.
- CLI group filter.
- Start/stop group.
- Dashboard group summary.

**Milestone 3 — Rule health**
- Store last start/stop/error metadata.
- Expose through status or rule details.
- UI health badges.
- CLI verbose status.

**Milestone 4 — Rule duplication**
- Duplicate rule API or client-side create helper.
- UI duplicate action.
- CLI duplicate/create-from command.

**Milestone 5 — Safer config UI**
- Import preview.
- Destructive warning.
- Backup-before-apply affordance.
- Clearer error rendering.

**Milestone 6 — Recent connection history**
- Bounded in-memory history.
- Endpoint.
- UI recent tab/table.
- Tests for memory cap and cleanup.

### Acceptance Criteria for v1.8

Portier v1.8 is ready when:

- v1.7 cleanup remains stable.
- The API contract remains backward-compatible.
- New rule metadata is optional and migration-safe.
- TS and Go runtimes behave identically.
- CLI and UI both understand groups.
- Config import/export/plan/apply preserve group metadata.
- Group start/stop has rollback/error behavior documented and tested.
- Health/status fields are meaningful and bounded.
- Recent connection history is bounded and does not leak memory.
- All existing coverage gates pass without lowering.
- `validate:contract` covers all new public API behavior.

### Non-Goals for v1.8

- Turning Portier into a full reverse-proxy platform.
- Persisting connection history to disk by default.
- Authentication, remote management, or cloud sync.
- Public API field renames for cosmetic consistency.

---

## Portier v1.9 — Doctor & Config Toolkit

**Core theme: A companion tool layer around the runtime — inspect, diagnose, and explain.**

> Diagnose your Portier setup, validate configs before applying them, and get actionable explanations for common forwarding problems.

The runtime stays focused (run TCP/UDP rules, expose API, serve UI, persist config safely). v1.9 adds tools that help users and maintainers inspect configs before running them, diagnose the environment, explain port conflicts, migrate configs between versions, generate support/debug reports, and validate automation workflows. This keeps Portier simple while making it feel much more professional. All new tooling lives inside the existing CLI first — no new binary, no new daemon.

### Primary tool — `portier doctor`

**Goal:** A dedicated diagnostic command that checks the local machine and the whole Portier environment. This is broader than `diagnose <rule>` (which checks one forwarding rule); `doctor` checks the whole setup.

**Examples:** `portier doctor`, `portier doctor --json`, `portier doctor --config ./rules.json`, `portier doctor --include-runtime`, `portier doctor --strict`.

**What it checks:** API reachable; runtime version matches CLI expectation; config file readable; config schema valid; duplicate listen bindings; privileged ports; common risky ports; LAN exposure warnings; conflicting local listeners; service executable available; web UI static files present; current runtime is Node fallback or Go service; stale/corrupt rules file; release/build metadata present; Windows service likely installed/running (if safely detectable).

**Output:** Grouped human report (Runtime / Config / Ports / Recommendation) with ✓/!/✕ markers; `--json` stable enough for automation.

**Why it fits v1.9:** It reuses systems already built in v1.5–v1.8 — config validation, advisory rules, diagnose checks, runtime info, CLI/API client, health/status, config plan/apply — so it feels like a new tool without a new daemon or big architecture change.

### Secondary tool — `portier config doctor`

**Goal:** A config-only validator and explainer that works offline, without a running runtime.

**Examples:** `portier config doctor ./rules.json`, `... --json`, `... --explain`.

**Checks:** valid JSON; valid schema; duplicate IDs; duplicate listen bindings; invalid protocol; invalid UDP mode; invalid host/port; risky listen host; privileged/common ports; replace/import compatibility; v1.x migration notes.

**Why separate:** `portier doctor` checks environment + runtime; `portier config doctor` checks only a config file — useful for CI, backups, config review, preflight before import/apply, and sharing configs between machines.

### Third idea — `portier explain`

**Goal:** A human-readable explanation tool. Deterministic, tested, and based on known advisory/diagnostic codes — not an AI feature.

**Examples:** `portier explain rule <rule-id>`, `portier explain config ./rules.json`, `portier explain error EADDRINUSE`, `portier explain advisory LAN_EXPOSURE`.

**Why interesting:** Portier already collects enough structured information; `explain` turns that into operator-friendly guidance.

### Fourth idea — `portier migrate`

**Goal:** An explicit config migration tool.

**Examples:** `portier migrate ./rules-v1.5.json --to 1.9.0`, `portier migrate ./rules.json --write`, `portier migrate ./rules.json --backup-out ./rules.backup.json`.

**Checks:** detect config version; apply safe migration; preserve unknown future-safe fields if supported; write backup; produce diff; fail closed on invalid config.

**Recommendation:** Useful, but only if the config schema evolves in v1.8/v1.9 (e.g. groups/metadata create a real migration need). Do not build it too early.

### Suggested v1.9 Scope

**Must-have:**
1. `portier doctor`
2. `portier doctor --json`
3. Offline `portier config doctor <file>`

**Should-have:**
4. Stable diagnostic codes
5. Environment checks
6. Config explanations
7. CI-friendly exit codes

**Nice-to-have:**
8. `portier explain`
9. `portier migrate`
10. Support bundle integration

### Tool Architecture

**Keep it inside the existing CLI first — do not create a separate binary yet.** Subcommands: `portier doctor`, `portier config doctor`, `portier explain`, `portier migrate`. One install path, one version, the existing CLI API client, lower packaging complexity, simpler docs, no new release artifact family.

**Suggested internals:**

```text
tools/cli/sources/commands/doctor.go
tools/cli/sources/commands/config_doctor.go
tools/cli/sources/commands/explain.go
tools/cli/sources/doctor/        (checks.go, report.go, runtime.go, environment.go)
tools/cli/sources/configlint/    (lint.go, rules.go, report.go)
tools/cli/sources/explain/       (advisory.go, errors.go, rule.go)
```

**Check engine — keep it simple:**

```go
type CheckResult struct {
    Code     string
    Severity string
    Title    string
    Message  string
    Details  map[string]string
}
```

**Avoid:** plugin system, dynamic scripting, AI-generated advice, hidden network scans, OS-specific deep magic in v1.9.

### Exit-Code Policy

Align with the existing CLI exit-code ladder rather than inventing a new one:

```text
0 = all checks passed
1 = checks completed with errors
2 = invalid command/input/config file
3 = runtime unreachable
4 = doctor could not complete due to unexpected local failure
```

### API Impact

Start with minimal or no API changes. `portier doctor` composes existing endpoints: `/api/runtime`, `/api/forwards`, `/api/status`, `/api/activity`, `/api/connections`, `/api/ports/advisory`, `/api/config/plan`, `/api/forwards/:id/diagnose`. A `GET /api/doctor` endpoint is possible later but should not be added first — keep the first version CLI-composed.

### UI Impact

Optional for v1.9 and deferred until the check model stabilizes: a "Doctor" button in Settings, a doctor report in the web UI, and doctor-report inclusion in the diagnostics bundle. CLI first.

### Risks

- **Too broad** — a "doctor" tool can become a trash pile of checks. Mitigation: fixed check list, stable check codes, clear severity, short report, JSON output, no plugin system.
- **OS-specific weirdness** — services/processes/firewalls/occupied ports get platform-specific fast. Mitigation: v1.9 checks only safe, portable basics; OS-specific checks are optional and best-effort; unknown is `warning`/`info`, never a hard failure.
- **Duplicated validation logic** — config validation already exists in shared/server/service. Mitigation: reuse CLI DTOs and existing validation; do not invent a third schema engine; if duplication grows, carefully extract a shared CLI validation package.

### Suggested Implementation Slices

**Milestone 1 — Check model**
- Define `CheckResult`, severity values, and report structure; human + JSON output; tests for formatting and exit codes.

**Milestone 2 — Config doctor**
- Offline config file load; schema validation; duplicate-binding checks; advisory checks; JSON/human output; CI-friendly exit codes.

**Milestone 3 — Runtime doctor**
- Runtime reachable check; version check; rules/status summary; activity summary; live-connection summary; optionally run diagnose on enabled/running rules.

**Milestone 4 — Environment checks**
- Safe local port-conflict checks; privileged/common port warnings; static client presence if detectable; service binary presence if detectable; OS-specific checks only where safe.

**Milestone 5 — Support integration**
- Include the doctor report in diagnostics export; add docs; optionally add a UI entry point.

### Acceptance Criteria for v1.9

Portier v1.9 is ready when:

- `portier doctor` runs against a live runtime.
- `portier doctor --json` produces stable machine-readable output.
- `portier config doctor <file>` works offline.
- Duplicate listen bindings and risky listen hosts are caught before import/apply.
- Doctor exit codes are documented and tested.
- No product-side port-retry behavior is introduced.
- No API contract is broken; all existing v1.8 behavior remains compatible.
- `validate:contract`, `validate:cli`, and `validate:coverage` pass with no lowered gates.

### Non-Goals for v1.9

- A separate doctor binary or new daemon.
- A `GET /api/doctor` endpoint in the first version (CLI-composed first).
- A plugin system, dynamic scripting, or AI-generated advice.
- Hidden network scans or deep OS-specific magic.
- A third config-schema engine duplicating shared/server/service validation.
- Product-side port-retry behavior.

---

## Portier v1.10 — Automation & Policy Guardrails

**Core theme: Turn Portier from a manually operated forwarding manager into a safer, semi-automated operator tool.**

> Automate safer forwarding workflows with policies, preflight checks, temporary starts, and operator-friendly watch commands — without hiding errors.

v1.6 hardened correctness and parity; v1.7 cleaned up maintainability; v1.8 added rule groups, health, and faster rule management; v1.9 added doctor/config tooling. v1.10 adds controlled automation: policies, scheduled/conditional rule behavior, safer bulk operations, preflight enforcement, profile switching, and operator guardrails. The goal is not to become Kubernetes for ports — it is to make everyday forwarding workflows safer and less repetitive.

### Priority 1 — Rule policies

**Goal:** Add optional policy metadata that constrains what a rule is allowed to do — e.g. listen only on localhost, never expose LAN, no privileged ports, require confirmation for `0.0.0.0`, require localhost target, auto-disable after failure, or be temporary.

**Why:** Portier already has advisories and config validation; v1.10 can turn some of that into enforceable guardrails.

**Possible model** (conservative start in bold):

```ts
policy?: {
  allowLanExposure?: boolean      // conservative start
  allowPrivilegedPorts?: boolean  // conservative start
  requireHealthyTarget?: boolean
  maxRuntimeMinutes?: number
  temporary?: boolean
}
```

**Behavior:** validation warns or blocks based on policy; config plan/apply shows policy violations; UI shows "policy blocked" / "requires confirmation"; CLI `--strict-policy`; doctor reports policy violations.

**Risk:** Medium-high — policy means semantics, not just metadata.

**Recommendation:** Start with **warning + strict mode**, not hard blocking everywhere.

### Priority 2 — Profiles / environment switching

**Goal:** Let users switch between named active rule sets or group states (`portier profile use work|gaming|local-dev`).

**Difference from groups:** Groups *organize* rules; profiles *define which groups/rules should be active together* (e.g. profile `work` enables groups `work`,`vpn` and disables `gaming`,`lab`).

**Features:** create profile from current enabled/running state; preview switch; apply switch; rollback if persist/start fails; UI profile dropdown; CLI `profile plan/use/list`.

**API ideas:** `GET /api/profiles`, `POST /api/profiles/plan`, `POST /api/profiles/apply`.

**Risk:** High-ish — touches config model, enabled/running semantics, plan/apply behavior, and UX.

**Recommendation:** Good v1.10 flagship **if v1.8 groups landed cleanly**.

### Priority 3 — Scheduled temporary rules

**Goal:** Support rules that automatically stop or disable after a time limit (`portier start my-rule --for 30m`, `portier enable my-rule --until 18:00`).

**Use cases:** temporary tunnel, short debugging session, LAN exposure that should not stay open, one-off port bridge.

**Behavior:** start now; schedule automatic stop; activity event when auto-stopped; status shows remaining time; config does not silently lose user intent; restart behavior is explicit.

**Minimal model:** runtime-only first (`temporaryUntil?: string`); persisted scheduling comes later if needed.

**Risk:** Medium — time-based behavior needs careful tests.

**Recommendation:** Very useful, but keep v1.10 scope small: **runtime-only temporary starts first**.

### Priority 4 — Preflight before start/apply

**Goal:** Before starting rules or applying configs, run a lightweight preflight and show what will fail. Doctor finds issues when asked; preflight prevents obvious bad operations at the moment they happen.

**Checks before `start`:** listen port already occupied; privileged port likely requires elevation; LAN exposure advisory; target unreachable (optionally); duplicate listen binding; invalid policy.

**Checks before `config apply`:** duplicate desired bindings; destructive remove count; policy violations; risky exposure changes; disabled-to-enabled bulk changes.

**CLI:** `portier start <rule> --preflight`, `portier config apply rules.json --preflight`, `... --strict-policy`.

**UI:** show preflight warnings before starting/applying; "Start anyway" only if the warning is non-fatal; fatal blocks when policy says so.

**Risk:** Medium — must avoid false confidence: preflight can race.

**Recommendation:** Frame as **best-effort preflight**, not a guarantee.

### Priority 5 — Automation-safe API mode

**Goal:** Make Portier safer to use from scripts/CI.

**Features:** idempotency keys for mutating operations; broader dry-run support; stable machine-readable error codes; consistent operation result envelopes; CLI `--strict` mode; better non-interactive behavior.

**API additions:** possibly an `Idempotency-Key: <key>` header or explicit request fields.

**Risk:** Medium-high — idempotency is easy to do badly.

**Recommendation:** Start with **stable error codes and dry-run expansion**; defer full idempotency unless there is a concrete need.

### Priority 6 — Watch mode

**Goal:** CLI can watch runtime status, activity, or live connections (`portier watch status|activity|connections|health`).

**First implementation:** polling, not WebSocket — refresh every 1–2s, clean Ctrl+C exit, JSONL mode for automation.

**Risk:** Low-medium — mostly CLI UX.

**Recommendation:** Great small v1.10 feature, especially after v1.9 doctor.

### Priority 7 — Rule change audit trail improvements

**Goal:** Make activity events more useful as a lightweight audit trail.

**Features:** actor/source field (`ui`/`cli`/`api`/`system`); operation correlation id; config-apply grouped events; profile-switch grouped events; policy-blocked events; temporary auto-stop events.

**Risk:** Medium — touches activity schema and contract.

**Recommendation:** Only add if v1.10 introduces policy/profile/temporary behavior that benefits from it.

### Suggested v1.10 Scope

**Must-have:**
1. Rule policy metadata (conservative version)
2. Preflight checks before start/apply
3. Runtime-only temporary starts
4. CLI watch mode

**Should-have:**
5. Profile switching (if v1.8 groups are solid)
6. Stable error/advisory codes for automation
7. Activity audit-trail improvements

**Nice-to-have:**
8. Idempotency keys
9. Persisted schedules
10. UI profile dashboard

### API Design Principles for v1.10

- Keep existing v1.x behavior backward-compatible.
- New fields must be optional.
- Policy violations should be explicit and machine-readable.
- Preflight must not promise race-free success.
- Temporary runtime state must be clearly separated from persisted config.
- Contract tests must cover both runtimes before UI/CLI relies on new behavior.
- Do not add WebSockets unless polling becomes painful.
- Do not add background scheduler complexity for v1.10 unless temporary runtime starts prove valuable.

### Suggested Implementation Slices

**Milestone 1 — Policy foundation**
- Define the minimal policy model; validate policy metadata; preserve import/export/plan/apply; add contract parity tests; update `docs/glossary.md` if needed.

**Milestone 2 — Preflight engine**
- Shared preflight result model; start-rule preflight; config-apply preflight; CLI `--preflight`; UI warnings.

**Milestone 3 — Temporary runtime starts**
- `start --for <duration>`; runtime timer; auto-stop activity event; status countdown; no persisted schedule yet.

**Milestone 4 — Watch mode**
- CLI polling watch for status/activity/connections; JSONL output option; clean Ctrl+C handling.

**Milestone 5 — Profiles (if groups are mature)**
- Profile data model; profile plan; profile apply; UI selector; CLI `profile use`.

**Milestone 6 — Automation polish**
- Stable operation codes; stricter JSON outputs; grouped activity events; optional idempotency investigation.

### Acceptance Criteria for v1.10

Portier v1.10 is ready when:

- Policy metadata is optional and migration-safe.
- TS and Go runtimes enforce policy/preflight consistently.
- Config import/export/plan/apply preserve policy metadata.
- Temporary starts stop reliably and emit activity events.
- CLI watch mode works without leaking processes or hanging on Ctrl+C.
- UI clearly distinguishes warning vs blocked operations.
- doctor/config doctor understand policy violations.
- All existing v1.9 behavior remains compatible.
- `validate:contract` passes with all new API behavior covered.
- `validate:coverage` passes with no lowered gates.
- No product-side retry hides real bind conflicts.

### Non-Goals for v1.10

- Becoming "Kubernetes for ports."
- WebSockets (polling-first) unless polling becomes painful.
- A persistent background scheduler unless runtime-only temporary starts prove valuable.
- Full idempotency before a concrete need exists.
- Hard-blocking policy enforcement everywhere (warning + strict mode first).
- Product-side port retry that hides real bind conflicts.

---

## Portier v1.11 — Local Intelligence & Workflow Automation

**Core theme: Stay local-first; make Portier better at understanding, explaining, and automating local forwarding workflows.**

> Understand complex local forwarding setups, preview risky changes, save restore points, and automate safe local workflows.

Instead of remote/team/admin features, v1.11 makes Portier better at reasoning about local forwarding setups — comparing configs, simulating changes, detecting conflicts, explaining failures, and automating safe local workflows. Human-readable explanations are deterministic, not AI-generated, and the analyzer's warnings are advisory, not absolute truth.

### Priority 1 — Config diff / visual plan improvements

**Goal:** Make config plan/apply output much more understandable, building on the v1.5/v1.6 plan/apply foundation without changing the threat model.

**Features:** richer CLI diff display; better UI plan visualization; grouped changes if v1.8 groups exist; before/after per rule; explain why a rule is `add`/`update`/`remove`/`unchanged`; highlight risky changes (new LAN exposure, privileged port, target host changed, protocol changed, enabled-state changed, UDP mode changed).

**CLI:** `portier config diff rules.json --explain`, `portier config plan rules.json --risk`, `portier config apply rules.json --preview`.

### Priority 2 — Rule dependency / conflict analyzer

**Goal:** A local analyzer that explains conflicts and risky topology — "doctor", but deeper and topology-focused.

**Checks:** duplicate listen bindings; overlapping groups/profiles (if implemented); rules targeting ports used by other rules; localhost-to-localhost chains; circular-looking forwarding chains; LAN-exposed rules targeting localhost services; disabled rules that conflict with enabled rules; enabled rules that cannot start because another rule owns the binding.

**CLI:** `portier analyze`, `portier analyze --json`, `portier analyze --rule tor-control`. Output explains chained forwarding and other topology notes ("This may be intentional, but changes to Rule B affect Rule A").

### Priority 3 — Saved views / filters

**Goal:** Make the UI/CLI better for many rules without introducing remote/team complexity. After groups/profiles, list management becomes the next usability bottleneck.

**Features:** saved UI filters; saved CLI query presets; filter by group/protocol/status/enabled/failed/listen host/target host/LAN-exposed; quick views (Failed, LAN-exposed, Running, Disabled, Recently changed).

**CLI:** `portier list --failed`, `portier list --lan-exposed`, `portier list --group work`, `portier view save failed-local --failed --listen-host 127.0.0.1`, `portier view use failed-local`.

### Priority 4 — Local automation recipes

**Goal:** Let users define simple local workflows without remote/team features.

**Model (conservative v1.11 form):** a named recipe with an ordered `actions` list (e.g. `startGroup`/`diagnoseGroup`). Start with read-only/preview + start/stop group actions only.

**CLI:** `portier recipe list`, `portier recipe run dev-stack`, `portier recipe plan dev-stack`. **UI:** Run / Preview / Stop recipe; activity event per recipe run.

**Risk:** Medium — needs careful plan/preview and rollback semantics.

### Priority 5 — Rule snapshots / local restore points

**Goal:** Save named local snapshots of rule config before experimenting. A local-first convenience layer around the existing config export/import.

**CLI:** `portier snapshot create before-vpn-test`, `portier snapshot list`, `portier snapshot restore before-vpn-test --plan`, `... --yes`. **UI:** create/restore/compare-to-current.

**Scope:** stored locally; bounded snapshot count; no cloud, no remote sync, no team sharing. Restore uses plan/apply rules, not a bypass.

**Risk:** Low-medium.

### Priority 6 — Better activity timeline

**Goal:** Turn activity from a "log list" into a useful local timeline. After v1.10 automation, activity becomes more important.

**Features:** group events by operation (config apply, recipe run, group start, profile switch); filter by rule/group/type/severity; show related events; export filtered activity; optional in-memory correlation id.

**API impact (optional, local-only, no auth/team identity):**

```ts
operationId?: string
operationType?: string
source?: "ui" | "cli" | "api" | "system"
```

### Priority 7 — Local backup policy

**Goal:** Make config backups more automatic and safer. v1.6 made config writes safe; v1.11 makes operator mistakes easier to recover from.

**Features:** auto-backup before destructive config apply; auto-backup before snapshot restore; configurable retention; `portier backup list` / `portier backup restore`; backup integrity check; redaction option for export.

**Risk:** Medium — retention and storage rules need to be clear.

### Suggested v1.11 Scope

**Must-have:**
1. Config diff / plan explanation improvements
2. Rule dependency / conflict analyzer
3. Saved views / filters

**Should-have:**
4. Local snapshots / restore points
5. Better activity timeline

**Nice-to-have:**
6. Local automation recipes
7. Local backup policy

### API Design Principles for v1.11

- Stay local-first; no auth/remote/team threat-model expansion; no cloud sync.
- New fields must be optional.
- Plan/diff/analyze output should be machine-readable.
- Human explanations must be deterministic, not AI-generated.
- Analyzer warnings must not pretend to be absolute truth.
- Snapshot restore should use plan/apply rules, not bypass them.
- Recipes must support preview before execution.

### Suggested Implementation Slices

**Milestone 1 — Explainable config plan**
- Add richer diff metadata if needed; improve CLI `config diff --explain`; improve UI plan/apply display; add contract tests if the API changes.

**Milestone 2 — Local analyzer**
- Implement `portier analyze`; detect duplicate/chained/risky local setups; JSON + human output; tests for analyzer rules.

**Milestone 3 — Saved filters/views**
- CLI list filters; UI saved filters; `docs/glossary.md` update if needed.

**Milestone 4 — Snapshots**
- Snapshot create/list/restore/plan reusing existing config plan/apply machinery; auto-backup before restore.

**Milestone 5 — Activity timeline**
- Operation grouping; filters; export; optional `operationId`/`source` fields.

**Milestone 6 — Recipes**
- Recipe plan/run; start/stop groups only; rollback/error reporting; activity grouping.

### Acceptance Criteria for v1.11

Portier v1.11 is ready when:

- Users can understand config changes before applying them.
- `portier analyze` catches common local conflicts and topology risks.
- UI and CLI can filter large rule sets effectively.
- Snapshots can be created and restored safely.
- Destructive operations can be previewed.
- The activity timeline is easier to navigate.
- All new behavior is covered by unit/contract tests.
- `validate:contract` passes.
- `validate:coverage` passes with no lowered gates.
- No remote/auth/team security model is introduced.

### Non-Goals for v1.11

- Any remote/auth/team security-model expansion.
- Cloud sync, remote snapshot storage, or team sharing.
- AI-generated explanations (must be deterministic).
- Analyzer output presented as absolute truth rather than advisory.
- Snapshot restore that bypasses plan/apply safety rules.
- Recipe execution without a preview path.

---

## Portier v1.12 — Local History & Observability

**Core theme: Build the observability/data foundation that later powers `tools/replay`.**

> Keep useful local history, inspect connection timelines, and export better diagnostics without adding remote complexity.

v1.12 is the data/observability foundation, **not** the replay tool. The runtime/app components collect, retain, expose, and export local history and diagnostics; v1.12 ships only a skeleton of the future offline replay tool that pins down the input-artifact contract. The full offline incident-reconstruction tool is reserved for v1.13.

### Tooling Responsibility Split

This release formalizes a clear boundary between the live-runtime CLI and the future offline analysis tool. Both live under `tools/`, but they are distinct programs with distinct responsibilities:

| Path | Responsibility |
|---|---|
| `tools/cli/` | Talks to and **modifies a live Portier runtime** (the existing `portier` CLI — API client). |
| `tools/replay/` | Analyzes **exported/offline** timeline, diagnostics, and history artifacts. Works offline; must not require a live runtime. |

Runtime/app components (server, service, client) collect, retain, expose, and export the data. `tools/replay` explains exported data after the fact. **Replay is not a `portier replay` subcommand inside `tools/cli`** — it is a separate offline tool under `tools/replay/`.

### Priority 1 — Persistent activity history

- Activity history survives restart.
- Bounded retention: max count / max age.
- Clear/export support.
- History persistence must not brick runtime startup.
- Corrupt history is ignored/rotated/recovered, not fatal.

### Priority 2 — Connection/session history

- Bounded TCP connection history and bounded UDP session history.
- Records: timestamps, rule id/name, client/target addresses, duration where available, close reason where practical.
- No packet payload persistence.

### Priority 3 — Timeline/history API

- Query activity/history by time range, rule, protocol, severity, and limit.
- Optional dedicated timeline endpoint if client-side composition is too awkward.
- All new API shapes must be contract-tested across the TypeScript server and Go service.

### Priority 4 — Timeline UI

- View local activity + connection/session history together.
- Filters by rule/time/severity/protocol.
- Empty/error states; link events back to rules.
- No remote/team/auth model.

### Priority 5 — Diagnostics bundle v2

- Include runtime info; rules/status; activity history; connection/session history; doctor/analyzer reports if available; a timeline slice export.
- Redaction modes if sensitive fields are included.

### Priority 6 — Metrics / stats

- JSON metrics endpoint or equivalent.
- Rule-level counters if bounded and cheap.
- Avoid high-cardinality monitoring creep.
- Prometheus text output is optional/deferred.

### Priority 7 — `tools/replay` skeleton only

Create the skeleton for the future offline replay tool:

```text
tools/replay/
  readme.md
  package.json or go.mod   (depending on chosen implementation)
  sources/
  tests/
```

For v1.12, the replay tool only:

- Defines/validates the input artifact format.
- Loads a timeline/history/diagnostics export.
- Prints basic metadata or lists events.
- Fails cleanly on invalid input.
- Documents the planned v1.13 behavior.

Do **not** implement full incident reconstruction in v1.12.

### Design Principles for v1.12

- Stay local-first; no remote/auth/team threat model; no cloud sync; no AI-generated explanations.
- History must be bounded.
- History must be separate from rules config.
- History persistence failure must not prevent forwarding.
- History corruption must not prevent runtime startup.
- Exports must be deterministic.
- The timeline/replay input schema should be stable and documented.
- `tools/replay` must work offline and must not require a live runtime.

### Suggested Implementation Slices

**Milestone 1 — Persistent activity history**
- Bounded, corruption-safe persistence (max count/age, rotate/recover on corrupt, never fatal to startup); clear/export; both runtimes.

**Milestone 2 — Connection/session history**
- Bounded TCP/UDP history with timestamps, rule id/name, addresses, duration, close reason; no payloads; both runtimes.

**Milestone 3 — Timeline/history API**
- Query by time range/rule/protocol/severity/limit; optional timeline endpoint; `validate:contract` parity across TS and Go.

**Milestone 4 — Timeline UI**
- Combined activity + connection/session view; filters; empty/error states; event→rule links.

**Milestone 5 — Diagnostics bundle v2 + metrics**
- Bundle v2 with history/timeline slice and redaction modes; bounded JSON metrics/stats.

**Milestone 6 — `tools/replay` skeleton**
- Scaffold `tools/replay/`; define + validate the input artifact format; load export, print metadata/list events, fail cleanly; document planned v1.13 behavior.

### Acceptance Criteria for v1.12

Portier v1.12 is ready when:

- Activity history can persist across restart within retention limits.
- Connection/session history is bounded and queryable.
- The timeline UI can show recent local forwarding activity.
- Diagnostics export can include a timeline/history slice.
- Metrics/stats are bounded and documented.
- Corrupt history cannot brick runtime startup.
- The `tools/replay` skeleton exists and validates the exported input format.
- `validate:contract` covers the new API shapes.
- `validate:coverage` passes with no lowered gates.
- No remote/auth/team model is introduced.

### Non-Goals for v1.12

- The full offline replay/incident-reconstruction tool (reserved for v1.13).
- A `portier replay` subcommand inside `tools/cli` (replay lives in `tools/replay/`, offline).
- `tools/replay` requiring a live runtime.
- Persisting packet payloads.
- Remote/auth/team models, cloud sync, or AI-generated explanations.
- Unbounded history or high-cardinality metrics creep.
- History stored inside or coupled to the rules config.

---

## Portier v1.13 — Replay Tool & Incident Toolkit

**Core theme: Turn the v1.12 skeleton into the full `tools/replay` offline analysis tool.**

> Reconstruct what happened from exported Portier history, correlate rule failures with connection activity, and generate deterministic local incident reports.

v1.13 implements the real offline replay tool that reconstructs and explains exported history/diagnostics/timeline artifacts after the fact, working entirely offline against the versioned input-artifact contract defined in v1.12.

### Tooling Responsibility Split (carried forward from v1.12)

- `tools/cli/` talks to a **live** Portier runtime and performs live operations.
- `tools/replay/` reads **exported timeline/history/diagnostics artifacts offline** and reconstructs what happened — it must not require a running Portier service and must never modify a live runtime or config.
- Runtime/app components collect, retain, query, and export the data.
- The replay tool analyzes exported data **deterministically** after the fact.

Replay is a separate tool under `tools/replay/`, **not** a `portier replay` subcommand inside `tools/cli`.

### Slice 1 — Standalone replay tool scaffold (delivered)

Scaffolded `tools/replay/` as a separate Go module (`portier/replay`, stdlib-only) with the `replay` binary — a separate tool beside the CLI, not a `portier` subcommand, with no dependency on `tools/cli` or any runtime client. First command: `replay [--json] plan --from <file-or-dir> [--out <file>]` detects an existing workflow artifact (run report, plan report, history export, or support-report bundle directory) by JSON/manifest shape and prints a deterministic **replay plan** describing the offline analyses the artifact can support (available) plus the four it never performs (execution, runtime probing, referenced-file reread, mutation/enforcement). Strictly offline and read-only; exit codes `0`/`1`/`2` (no `3`). `validate:contract` stays 234/234. Scripts: `build:replay`/`test:replay`/`validate:replay`. This is the v1.13 foundation; later slices build the actual analyses on top.

### Slices 2–8 — Replay toolkit (delivered, released as 1.13.0)

The replay tool provides five offline commands — `plan` (Slice 1), `analyze` (Slice 2), `timeline` (Slice 4), `compare` (Slice 5), and `explain` (Slice 6, with optional `--code`/`--list`) — plus a naming/structure polish (Slice 3: binary `replay`, packages `core`/`commands`), a consistency audit (Slice 7), and a machine-enforced coverage gate (Slice 8). All commands are strictly offline and read-only, use local tool schemas (`schemaVersion: 1`, never the REST contract), share the `0`/`1`/`2` exit-code convention (no `3`), and support `--json` + `--out` with byte-identical parity. The `tools/replay` module is held to a **hard ≥95% module-wide coverage gate** (Slice 8: the `replay` component in `scripts/validate-coverage.js`, fed by `scripts/coverage-tools-replay.js`, run via `npm run validate:coverage:replay`; currently 96.5%). **Slice 7 (consistency audit): PASS.** Shipped in **v1.13.0**; `validate:contract` stayed 234/234 throughout (replay is wholly independent of the runtime) and the replay binary is a standalone analysis tool not bundled in the runtime release artifacts. The original incident-reconstruction/correlation ideas below remain a possible future direction but are not part of the shipped v1.13 toolkit.

### Priority 1 — `tools/replay` full offline analysis

Implement the real replay tool under `tools/replay/`. It should:

- Load exported timeline/history/diagnostics bundles.
- Validate the input schema.
- Reconstruct a time window.
- Correlate activity events, rule state changes, connection/session history, diagnostics, and doctor/analyzer outputs if available.
- Produce a deterministic incident report.
- Support human and JSON output.
- Work without a running Portier runtime.

### Priority 2 — Input formats

Support v1.12 exports: timeline export, diagnostics bundle v2, and history/activity export. The input contract must be documented and **versioned**. Example commands depend on the implementation choice:

```bash
# If TypeScript/npm:
npm run replay -- --input ./diagnostics/timeline.json --since 30m
npm run replay -- --input ./diagnostics/bundle.json --rule api --json

# If Go:
go run ./tools/replay --input ./diagnostics/timeline.json --since 30m
go run ./tools/replay --input ./diagnostics/bundle.json --rule api --json
```

### Priority 3 — Report modes

**Human report:** summary; likely incident window; related events; rule failures; connection/session activity; config changes; diagnostics findings; recommendations based on deterministic known codes.

**JSON report:** machine-readable summary; event slices; correlated entities; detected incident candidates; warnings/errors; input metadata.

### Priority 4 — Filters

`--since`, `--from`, `--to`, `--rule`, `--severity`, `--type`, `--protocol`, `--operation-id` (if v1.11/v1.12 added operation IDs), `--json`.

### Priority 5 — Deterministic explanation engine

The replay tool must **not** use AI-generated advice. Explanations are derived from known activity types, diagnostic check codes, advisory codes, error codes, connection/session events, and config apply/plan events. Examples:

- Rule failed to start because the listen port was already in use.
- Target refused connection after the rule started.
- LAN exposure warning appeared before connection history.
- Config apply removed or changed a rule before failures began.
- Temporary rule auto-stopped as expected.

### Priority 6 — Correlation rules

Add simple, testable correlation rules: group events by rule id; group events by operation id where available; correlate `rule.error` with nearby diagnose results; correlate connection close/error patterns with target failures; correlate config changes with later start failures; identify likely root-cause candidates without overclaiming certainty.

Use hedged language ("likely", "possibly related", "nearby event", "same rule", "same operation") — avoid false certainty.

### Priority 7 — Redaction awareness

If diagnostics bundle v2 supports redaction: replay preserves redacted values; replay must not infer or reconstruct redacted secrets; replay reports clearly mark redacted fields.

### Priority 8 — Tests

Cover: input schema validation; invalid/corrupt input; empty timeline; simple successful rule lifecycle; failed-start incident; config apply followed by rule failure; connection burst summary; JSON output stability; redacted input handling; filter behavior.

### Priority 9 — Docs

Add `tools/replay/readme.md`, input artifact format docs, example reports, the relationship to `tools/cli`, troubleshooting examples, and limitations. Clearly state that `tools/replay` is offline, does not modify a live runtime, does not require a running Portier service, and produces deterministic analysis from exported artifacts.

### Suggested Implementation Slices

**Milestone 1 — Loader + schema validation + event listing** *(recommended first)*
- Implement the `tools/replay` input loader, input-schema validation, and basic event listing/metadata; fail cleanly on invalid/corrupt input.

**Milestone 2 — Time-window reconstruction + filters**
- Reconstruct a time window; implement `--since`/`--from`/`--to`/`--rule`/`--severity`/`--type`/`--protocol`/`--operation-id`/`--json`.

**Milestone 3 — Deterministic explanation engine**
- Map known activity/diagnostic/advisory/error codes to deterministic explanations; human + JSON output.

**Milestone 4 — Correlation rules + incident candidates**
- Add testable correlation rules and likely-root-cause candidates with hedged language; detected-incident output in both report modes.

**Milestone 5 — Redaction awareness + docs**
- Preserve/mark redacted fields; finalize `tools/replay/readme.md`, artifact-format docs, example reports, and limitations.

Then add deterministic incident reconstruction in small slices on top of the loader foundation.

### Acceptance Criteria for v1.13

Portier v1.13 is ready when:

- `tools/replay` can read a v1.12 diagnostics/timeline export.
- The human report works.
- The JSON report works.
- Time/rule/severity filters work.
- Invalid input fails cleanly.
- Incident reconstruction is deterministic and tested.
- Report output cites source events or event IDs where possible.
- Replay does not require a live runtime.
- Replay does not modify runtime/config.
- Validate/test commands for the tool are integrated into the repo.
- Existing Portier runtime/API behavior remains unchanged.

### Non-Goals for v1.13

- Remote runtime management.
- Authentication/team features.
- Cloud uploads.
- AI-generated incident explanations.
- Live streaming replay.
- Packet payload inspection.
- Unlimited history processing.
- A plugin framework.
- A `portier replay` subcommand in `tools/cli` (replay stays a separate offline tool under `tools/replay/`).

---

## Portier v1.14 — NestJS Server Migration

**Core theme: Migrate the TypeScript Node fallback server to NestJS while preserving the existing API contract, runtime parity, and behavior.**

> Replace the TypeScript Node fallback server with a NestJS architecture while preserving the existing Portier API contract, runtime parity, and behavior.

This is an **architecture migration release, not a feature release**. The Go service remains the preferred/native runtime; the NestJS server remains the Node fallback/runtime implementation. Public REST API behavior must stay compatible, the CLI and web UI must keep working unchanged, and `validate:contract` remains the source of truth for API parity. The migration must not be used as an excuse to redesign the API.

### Primary Goals

- Move the TypeScript server to NestJS modules/controllers/services.
- Preserve all existing endpoints and response shapes.
- Preserve the error taxonomy and status mapping.
- Preserve static client serving behavior.
- Preserve config persistence behavior from v1.6.
- Preserve activity event behavior and value parity.
- Preserve diagnose/config/apply/import behavior.
- Preserve runtime smoke and contract parity against Go.
- Improve maintainability, test structure, and future extensibility.

### Proposed Architecture

Adjust names to the repo's existing `sources/` convention and the Portier glossary.

```text
server/
  sources/
    main.ts
    app.module.ts
    common/
      errors/
      filters/
      validation/
    runtime/      (runtime.controller.ts, runtime.service.ts)
    forwards/     (forwards.controller.ts, forwards.service.ts)
    config/       (config.controller.ts, config.service.ts, config-plan.service.ts)
    activity/     (activity.controller.ts, activity.service.ts)
    diagnostics/  (diagnostics.controller.ts, diagnostics.service.ts)
    connections/  (connections.controller.ts, connections.service.ts)
    ports/        (ports.controller.ts, ports.service.ts)
    static/       (static.module.ts, static.service.ts)
```

### Migration Principles

1. **Keep domain logic reusable.** Move routing/controller code to NestJS; do not rewrite stable domain logic unnecessarily. Preserve the existing `ForwardManager`, forwarders, config store, config plan, diagnose helpers, and activity store unless a small adapter is needed.
2. **Controllers for HTTP only.** Controllers translate requests/responses; services own orchestration; existing pure helpers remain pure.
3. **Preserve error behavior.** `ValidationError → 400`, `ConflictError → 409`, `NotFoundError → 404`, unknown → 500; malformed-JSON behavior matches the current documented contract; error body stays `{ errors: [] }`.
4. **Preserve contract parity.** `npm run validate:contract` must pass with the same or an intentionally increased count; the Go service is the parity reference; any count change must be documented.
5. **Keep static client serving.** The NestJS server still serves the built client when available; a missing static client still leaves the API usable; runtime smoke still verifies `/api/health` and web UI serving as before.

### Suggested Implementation Slices

**Milestone 1 — NestJS skeleton beside current server**
- Add NestJS dependencies; create the NestJS app bootstrap; wire basic health/runtime endpoints; preserve current build-output conventions; keep the existing Express server until parity is proven, if practical; add minimal smoke tests.
- *Acceptance:* NestJS server starts; `/api/health` and `/api/runtime` work; build/typecheck pass.
- **Slice 1 (done):** NestJS scaffold added under `server/sources/nest/` (modules/controllers/services), beside the unchanged Express server which remains the active runtime. Surface kept deliberately minimal and **contract-safe**: a `GET /health` liveness probe (`{ ok, server: "node", name: "Portier" }`) served **outside the frozen `/api` contract** plus a global `ApiNotFoundFilter` returning the `{ errors: ["API route was not found."] }` envelope for unmatched `/api/*`. Implementing `/api/health` and `/api/runtime` is **deferred** to the endpoint-migration slices (Milestone 2+) so they land behind `validate:contract` rather than silently changing the documented "TypeScript server: not implemented" status of `/api/health`. Nest 11 (aligned with Express 5); additive scripts `build:nest`/`start:nest`/`test:nest`; build output under `build/`; scaffold 100% covered; `validate:contract` 234/234; no gate lowered; no release/tag.

**Milestone 2 — Controllers and error filter**
- Implement a global exception filter matching the existing error envelope; implement request-validation behavior; migrate simple endpoints (health, runtime, ports/advisory, activity read); add status/body parity tests.
- *Acceptance:* `validate:contract` partial/local scenarios pass for migrated endpoints; no response-shape drift.
- **Slice 2 (done):** added an **API parity harness** (`server/sources/nest/testing/api-parity.ts` — boots Express/Nest on an ephemeral port, captures `{status,body}`, deterministic `diffApiResponses`/`stableStringify`) and migrated the first read-only endpoint, **`GET /api/ports/advisory`**, into Nest (`api/ports/` controller→service→module). Chosen as the smallest safe candidate: read-only, deterministic, no runtime-manager dependency, pure `getPortAdvisories`, clear `400` branches. The Nest route is byte-identical to Express (Express↔Nest parity-tested). It is served only under `start:nest`; the Express server stays the default active runtime and is unchanged; `validate:contract` 234/234; all new Nest code 100% covered; no gate lowered.
- **Slice 3 (done):** added the **shared `/api` error-envelope layer** (`server/sources/nest/common/`): pure `toApiError` mapping (`api-error-envelope.ts`), a global catch-all `ApiErrorEnvelopeFilter` registered via `APP_FILTER` (envelopes `/api/*`, leaves non-API on NestJS defaults; maps unknown `/api` errors to `500 { errors:["Internal server error."] }` with no leak), and `ApiBadRequestException(errors: string[])` (`api-errors.ts`). It **replaced** the Slice-1 NotFound-only filter (404 envelope + non-API no-leak preserved) and cleaned `GET /api/ports/advisory` to throw `ApiBadRequestException` instead of an inline `{ errors }` literal. Parity unchanged, Express untouched, `validate:contract` 234/234, all new/changed Nest code 100% covered, no gate lowered.
- **Slice 4 (done):** migrated **`GET /api/activity`** (`server/sources/nest/api/activity/`) — controller→service over an injected `ActivityReader`/`ACTIVITY_STORE` token (default = a fresh domain `ActivityStore`). Read-only, always `200 { events }` (no error branches); only the read migrated (`DELETE /api/activity` deferred). Chosen over `GET /api/runtime` because activity gives true byte-for-byte parity (no inherently time/process-varying fields) and establishes the inject-dependency-behind-a-narrow-token/fake pattern for the heavier manager-dependent endpoints. Parity proven across 10 query variations by seeding the Nest app's own store (`app.get(ACTIVITY_STORE)`) and sharing it with Express (no `@nestjs/testing` needed). Express untouched, shadow-only under `start:nest`, `validate:contract` 234/234, all new/changed Nest code 100% covered, no gate lowered.
- **Slice 5 (done):** migrated the first **manager-dependent** read endpoint, **`GET /api/status`** (`server/sources/nest/api/status/`) — controller→service over a narrow `StatusReader { listStatus(): ForwardStatus[] }` behind a `STATUS_READER` token (default = a trivial `emptyStatusReader`; production status code holds no manager/store dependency). Established the **runtime/manager read-provider seam** the remaining read endpoints (`/api/forwards`, `/api/connections`) will reuse. Chosen because status has **no volatile fields** for *stopped* rules (`startedAt` only appears when running; counters are 0/absent), so seeding stopped rules gives exact byte-for-byte parity with no normalization. Parity proven (empty + seeded stopped tcp/udp rules) by sharing the same `ForwardManager` instance between Express and Nest via `@nestjs/testing` `overrideProvider` (added as a server devDependency — a `ForwardManager` can't be cleanly `app.get`-seeded like the mutable `ActivityStore`; seeded manager stays in test code). No real sockets/forwarders. Express untouched, shadow-only under `start:nest`, `validate:contract` 234/234, all new/changed Nest code 100% covered, no gate lowered.
- **Slice 6 (done):** migrated **`GET /api/forwards`** (`server/sources/nest/api/forwards/`) — controller→service over a narrow `ForwardsReader { listRules(): ForwardRule[] }` behind a `FORWARDS_READER` token (default = trivial `emptyForwardsReader`), reusing the Slice-5 read-provider pattern. The service maps each rule to `ForwardRuleResponse` via the shared `getPortAdvisories` (exactly as Express's `toRuleResponse`); read-only, always `200`. Only the list read migrated — write/lifecycle routes under `/api/forwards/...` stay with Express. Exact-parity-safe (no volatile fields); parity proven byte-for-byte (empty + seeded stopped rules incl. a `0.0.0.0` LAN_EXPOSURE rule) by sharing the manager instance via `overrideProvider`. Express untouched, shadow-only, `validate:contract` 234/234, all new/changed Nest code 100% covered, no gate lowered.
- **Slice 7 (done):** added the **DTO-based request-validation foundation** — `class-validator` + `class-transformer` + a shared `ApiValidationPipe` (`server/sources/nest/common/api-validation.pipe.ts`). `PortsAdvisoryQueryDto` declares Express-faithful coercion/validation; the pipe runs `plainToInstance`+`validate` (`whitelist`, `stopAtFirstError`) and throws `ApiBadRequestException` → the shared envelope. The pipe takes the DTO class explicitly (esbuild doesn't emit `design:paramtypes`), applied per-route (non-API routes unaffected). `GET /api/ports/advisory` migrated to the DTO/pipe (parity byte-for-byte incl. extra-param). `GET /api/activity` kept endpoint-local (no validation errors — pure coercion-with-fallback; documented exception). Parity unchanged, Express untouched, `validate:contract` 234/234, all new/changed Nest code 100% covered, no gate lowered. JSON-body DTO validation is the foundation for the write slices. This completes the simple read endpoints + the validation foundation for Milestone 2. **Still pending:** `GET /api/runtime` (volatile uptime/pid), the volatile `/api/connections`/`/api/config/export` (need an exact-parity strategy or deferral).
- **Slice 8 (done):** migrated the **first write endpoint, `DELETE /api/activity`** (`server/sources/nest/api/activity/`) — clears the in-memory activity log, returns `204` empty, matching Express. Lowest-risk write (paired mutation for `GET /api/activity`, reuses `ACTIVITY_STORE`, no forwarder/socket/rule-persistence/import-export/manager). Added a narrow `ActivityClearer { clear(): void }` (the `ACTIVITY_STORE` value satisfies `ActivityReader & ActivityClearer`); `ActivityController.clear()` is `@Delete() @HttpCode(204)` (no request DTO — no input; `204` no-body → no response DTO). **Also corrected the DTO rule:** every migrated endpoint now has explicit **request DTOs (where input exists) + response DTOs (always)** — added `*ResponseDto` + pure mappers (the domain→HTTP boundary) for ports-advisory/activity-list/status-list/forwards-list; controllers map the service result through the mapper, preserving Express JSON byte-for-byte. Parity byte-for-byte (DELETE populated + empty; subsequent GET empty in both; all read endpoints after the mapper change) via the shared `ActivityStore` instance. Express untouched, shadow-only, `validate:contract` 234/234, all new/changed Nest code 100% covered, no gate lowered. **Rule create/update/delete/start/stop/reorder/import and config import/export remain deferred to Milestone 3.**
- **Slice 9 (done):** migrated the first **volatile** read endpoint, `GET /api/runtime` (`server/sources/nest/api/runtime/`) — establishing the time/process provider strategy before `/api/connections` (`generatedAt`) and `/api/config/export` (`exportedAt`). Extracted a **shared pure builder** `buildRuntimeInfo` (+ `normalizePlatform`/`normalizeArch` + `RuntimeInfoOptions`) into `server/sources/runtime-info.ts`; both the Express route and the Nest `RuntimeService` call it, so the runtime-info shape cannot drift (Express change behavior-preserving — `api.ts` re-exports the moved symbols). **Volatile provider/token pattern:** three narrow readers (`ClockReader`/`CLOCK_READER` — generic `now()`, reusable for the deferred timestamp endpoints; `ProcessReader`/`PROCESS_READER` — `pid`/`platform`/`arch`; `RuntimeInfoReader`/`RUNTIME_INFO_READER` — static launch metadata + a once-at-construction start time mirroring Express). Response DTO + mapper; **no request DTO** (no input — documented exception). **Exact parity, no normalization:** a minimal optional production-invisible clock seam on Express `AppOptions` (`now?: () => Date`, defaults to the real clock) lets the parity test boot the real Express app and the Nest app with the **same fixed clock + runtime info** (real `PROCESS_READER` so `pid`/`platform`/`arch` match in-process), making `uptimeSeconds` deterministic and the booted-Express-vs-booted-Nest diff empty byte-for-byte. Express untouched in behavior, shadow-only, `validate:contract` 234/234, all new/changed code 100% covered, no gate lowered. **The volatile `/api/connections` and `/api/config/export` now have a proven provider/parity pattern to reuse.**

**Milestone 3 — Forward rule API migration**
- Migrate `/api/forwards` create/update/delete/start/stop/reorder; preserve `ForwardManager` behavior, activity events, duplicate-binding behavior, and start-failure behavior.
- *Acceptance:* server tests and forward-manager tests pass; `validate:contract` forward/lifecycle groups pass.

**Milestone 4 — Config API migration**
- Migrate config export/import and config plan/apply; preserve v1.6/v1.7 invariants — duplicate-binding import parity, atomic config persistence, apply never `ok:true` when import reports errors, dry-run behavior, destructive-confirmation behavior.
- *Acceptance:* `validate:config` passes; `validate:contract` config groups pass; CLI config commands unchanged.

**Milestone 5 — Diagnose, connections, and diagnostics migration**
- Migrate diagnose, connections, and diagnostics/export endpoints (if present); preserve labels/check IDs unless intentionally changed and contract-tested; preserve live connection/session shapes.
- *Acceptance:* `validate:contract` diagnose/connections groups pass; E2E live connections still pass.

**Milestone 6 — Static client serving and runtime smoke**
- Migrate static serving to a NestJS-compatible implementation; preserve API-only behavior when the static client is missing; preserve built-client serving; update runtime smoke if needed.
- *Acceptance:* `npm run validate:runtime:smoke` passes; web UI loads through the NestJS server.

**Milestone 7 — Remove old Express composition**
- Remove old Express app wiring after full parity; keep reusable domain modules; update docs and scripts; move server tests to the NestJS test harness where appropriate; remove obsolete dependencies.
- *Acceptance:* no old Express-only routing remains unless deliberately kept; no duplicate server paths; lint/typecheck/test clean.

### Validation Requirements

`npm run lint`, `npm run typecheck`, `npm run test -w server`, `npm run test`, `npm run validate:coverage`, `npm run validate:contract`, `npm run validate:config`, `npm run validate:runtime:smoke`, `npm run test:e2e`, `npm run build`, `npm run build:runtime`.

### Coverage Strategy

Do not lower gates; do not chase NestJS framework internals. Focus on controller behavior, error mapping, contract parity, and service orchestration. If coverage fluctuates due to framework wrapper lines, document structural exclusions rather than lowering meaningful coverage.

### Documentation Updates

`docs/changelog.md`, `docs/checklist.md`, `docs/api-contract.md` (if server implementation notes change), `docs/glossary.md` (if terminology changes), `CLAUDE.md`, `AGENTS.md`, and `README.md` (if server startup/build commands change).

### Risks

- **Contract drift** — migrate behind `validate:contract` groups, run frequently.
- **Error-handling drift** — a global NestJS exception filter matching the current envelope.
- **Static-serving drift** — runtime smoke + E2E.
- **Over-NestJS-ification** — keep domain logic simple, controllers thin, no unnecessary decorators/services.
- **Test churn** — migrate endpoint tests incrementally; preserve domain tests.

### Acceptance Criteria for v1.14

Portier v1.14 is ready when:

- The TypeScript server runs on NestJS.
- All existing REST endpoints behave compatibly.
- The CLI works unchanged.
- The UI works unchanged.
- Go service parity remains guarded.
- `validate:contract` passes.
- `validate:runtime:smoke` passes.
- `test:e2e` passes.
- Coverage gates pass with no lowering.
- The old Express routing layer is removed or explicitly documented as gone.
- Docs clearly state the Node fallback server is now NestJS-based.

**Recommended first slice:** create the NestJS skeleton and migrate only health/runtime endpoints. Do not touch config/forwarding behavior until the error filter and contract baseline are proven.

### Non-Goals for v1.14

- Public API path renames; DTO field renames; `/api/forwards` rename.
- Auth/remote/team model.
- Contract codegen.
- A broad domain rewrite.
- A Go service rewrite.
- Feature additions unless required to preserve behavior.
- Dependency-injection overengineering beyond normal NestJS structure.

---

## Portier v1.15 — Go Service Modular Router

**Core theme: Modularize the Go native service's HTTP/API layer without forcing a NestJS-style framework onto Go.**

> Reorganize the Go native service around explicit app dependencies, focused route modules, and `net/http`-compatible routing while preserving the Portier API contract and runtime behavior.

This is the Go counterpart to v1.14 — an **architecture/migration release, not a feature release**. The Go service remains the preferred/native runtime; the TypeScript NestJS server remains the Node fallback. Public REST API behavior must stay compatible, the CLI and web UI must keep working unchanged, and `validate:contract` remains the source of truth for TS↔Go parity. The migration must not be used as an excuse to redesign the API.

### Preferred Technical Direction

- Use `chi` (or a similarly lightweight `net/http`-compatible router) **only if** it clearly improves route grouping and middleware; otherwise keep standard `net/http`.
- Add an explicit App/Dependencies struct.
- Split the current large API handler file into focused route modules.
- Keep domain logic unchanged unless a small extraction is clearly justified.
- Keep the Go service idiomatic rather than trying to clone NestJS.

### Primary Goals

- Reduce Go API file size and responsibility concentration; make route ownership obvious.
- Make startup/composition dependencies explicit.
- Make future API additions easier and safer.
- Preserve existing error handling and response envelopes.
- Preserve static client serving behavior.
- Preserve config/import/apply behavior.
- Preserve activity event behavior and value parity.
- Preserve diagnostics/connections/status behavior.
- Preserve runtime smoke and contract parity.
- Keep Go idioms: explicit wiring, small interfaces, no magic DI.

### Proposed Architecture

Adjust names to existing repo conventions and `docs/glossary.md`.

```text
service/sources/
  app/      (app.go, dependencies.go)
  api/
    router.go
    errors.go
    middleware.go
    runtime_routes.go
    forwards_routes.go
    config_routes.go
    activity_routes.go
    connections_routes.go
    diagnose_routes.go
    ports_routes.go
    static_routes.go
  manager/  forwarders/  config/  configplan/  activity/  advisory/  diagnose/  static/
```

**Suggested dependency shape** (keep it explicit — do not introduce a general service locator):

```go
type App struct {
    Manager  *manager.Manager
    Store    *config.Store
    Activity *activity.Store
    Static   *static.Server
    Version  string
    Logger   *logger.Logger
}
```

**Suggested router shape if using chi:**

```go
func NewRouter(app *app.App) http.Handler {
    r := chi.NewRouter()
    r.Route("/api", func(r chi.Router) {
        MountRuntimeRoutes(r, app)
        MountForwardRoutes(r, app)
        MountConfigRoutes(r, app)
        MountActivityRoutes(r, app)
        MountConnectionRoutes(r, app)
        MountDiagnoseRoutes(r, app)
        MountPortRoutes(r, app)
    })
    MountStaticRoutes(r, app)
    return r
}
```

If staying with `net/http`: use an explicit `http.ServeMux` and still split route registration into focused functions.

### Migration Principles

1. **Keep domain logic reusable.** Move route registration and HTTP handler functions into focused modules; do not rewrite `manager`, `configplan`, `forwarders`, `activity`, or `diagnose` unnecessarily; keep pure helpers pure.
2. **Keep handlers thin.** Handlers parse requests, call services/manager/helpers, and write responses; business behavior stays in manager/configplan/diagnose/etc.
3. **Preserve error behavior.** validation → 400, conflict → 409, not found → 404, unknown → 500; do **not** introduce auth in this release; malformed-JSON behavior stays contract-compatible; error body stays `{ errors: [] }`.
4. **Preserve contract parity.** `npm run validate:contract` must pass with the same count unless an increase is intentional and documented; TS NestJS and Go behavior stay aligned; the CLI DTO live-decode guard must keep passing.
5. **Preserve static client serving.** Still serve the built client when available; a missing static client still leaves the API usable; runtime smoke still verifies `/api/health` and web UI serving.
6. **Preserve startup/shutdown semantics.** Existing service startup flags/options stay compatible; graceful shutdown stays bounded; `StopAll` behavior unchanged; no new background lifecycle framework unless necessary.

### Suggested Implementation Slices

**Milestone 1 — App dependencies and router skeleton**
- Create `app.App` (or equivalent dependency struct); add an `api.NewRouter(app)` entry point; keep current handlers initially or delegate to the existing handler; optionally introduce chi; preserve current `main.go` startup behavior; add smoke tests if needed.
- *Acceptance:* Go service builds; `/api/health` and `/api/runtime` work; `validate:runtime:smoke` passes.

**Milestone 2 — Error helpers and shared response utilities**
- Move/write JSON helpers into `api/errors.go` or `api/responses.go`; preserve the error envelope, status mapping, and body read/decode limits; keep tests for malformed JSON and error mapping.
- *Acceptance:* Go API tests pass; `validate:contract` error-envelope scenarios pass.

**Milestone 3 — Runtime, activity, ports route modules**
- Migrate the simple read-only endpoints first (health/runtime, activity, ports/advisory); preserve activity value parity and advisory behavior.
- *Acceptance:* `validate:contract` runtime/activity/advisory groups pass.

**Milestone 4 — Forward rule route module**
- Migrate `/api/forwards` create/update/delete/start/stop/reorder; preserve manager behavior, duplicate-binding behavior, activity events, and start-failure status/error behavior.
- *Acceptance:* Go API tests pass; `validate:contract` forwards/lifecycle/delete groups pass.

**Milestone 5 — Config route module**
- Migrate config export/import and config plan/apply; preserve v1.6/v1.7 invariants — duplicate-binding import parity, apply never `ok:true` when import reports errors, dry-run behavior, destructive-confirmation behavior, config plan/apply response shape.
- *Acceptance:* `validate:config` passes; `validate:contract` config groups pass; CLI config commands unchanged.

**Milestone 6 — Connections and diagnose route modules**
- Migrate connections and diagnose endpoints; preserve labels/check IDs unless intentionally changed and contract-tested; preserve live TCP connection / UDP session shapes.
- *Acceptance:* `validate:contract` connections/diagnose groups pass; E2E live connections still pass.

**Milestone 7 — Static serving module**
- Migrate static serving behind a route module; preserve API-only behavior when the static client is missing; preserve built-client serving and SPA fallback (if present).
- *Acceptance:* `validate:runtime:smoke` passes; web UI served through the Go service.

**Milestone 8 — Remove old monolithic API composition**
- Remove or shrink the old `api.go` monolith, leaving only focused route modules; update docs/tests; remove obsolete helper duplication.
- *Acceptance:* no duplicate routing paths; no dead handler code; lint/typecheck/test clean.

### Validation Requirements

`go test ./service/...`, `npm run lint`, `npm run typecheck`, `npm run test`, `npm run validate:coverage`, `npm run validate:contract`, `npm run validate:config`, `npm run validate:runtime:smoke`, `npm run test:e2e`, `npm run build`, `npm run build:runtime`.

### Coverage Strategy

Do not lower gates; do not chase router-framework internals. Focus on route behavior, error mapping, contract parity, and service orchestration. If chi or another router introduces uninteresting wrapper lines, document structural exclusions rather than weakening meaningful coverage.

### Documentation Updates

`docs/changelog.md`, `docs/checklist.md`, `docs/api-contract.md` (if implementation notes change), `docs/glossary.md` (if terminology changes), `CLAUDE.md`, `AGENTS.md`, and `README.md` (if service startup/build commands change).

### Risks

- **Contract drift** — migrate endpoint groups behind `validate:contract`, run frequently.
- **Error-handling drift** — central response/error helpers, contract tests.
- **Static-serving drift** — runtime smoke + E2E.
- **Over-frameworking** — chi/`net/http` only; explicit app struct; no DI container by default.
- **Route duplication** — remove old monolithic routes after module migration.
- **Test churn** — preserve manager/domain tests; only update API tests where routing structure changed.

### Acceptance Criteria for v1.15

Portier v1.15 is ready when:

- The Go service API layer is modularized; route modules are focused and easy to navigate.
- All existing REST endpoints behave compatibly.
- The CLI works unchanged.
- The UI works unchanged.
- TS NestJS server parity remains guarded.
- `validate:contract` passes.
- `validate:runtime:smoke` passes.
- `test:e2e` passes.
- Coverage gates pass with no lowering.
- The old monolithic Go API routing layer is removed or reduced to a small router entry point.
- Docs clearly state the Go native service uses a modular `net/http`-compatible router architecture.

**Recommended first slice:** introduce the App/Dependencies struct and router skeleton while preserving current behavior; migrate only health/runtime first; do not touch config/forwarding behavior until error helpers and the contract baseline are proven.

**Explicit recommendation:** use `chi` only if it stays a thin routing/middleware layer; do not use Fiber/fasthttp; do not add Uber Fx or Google Wire in v1.15 unless manual wiring proves painful. Prefer explicit Go wiring and `validate:contract` over framework magic.

### Non-Goals for v1.15

- Public API path renames; DTO field renames; `/api/forwards` rename.
- Auth/remote/team model.
- Contract codegen.
- A broad domain rewrite; a NestJS clone in Go.
- Fiber/fasthttp migration unless separately justified.
- A runtime DI container unless the dependency graph genuinely needs it.
- Feature additions unless required to preserve behavior.
- Weakening `validate:contract`, coverage, or runtime smoke.

---

## Portier v1.16 — Post-Migration Architecture & Reliability Audit

**Core theme: Re-audit Portier after the large architecture changes to verify parity, resilience, coverage, security, and maintainability still hold.**

> Re-audit Portier after the NestJS server migration, Go modular router migration, observability work, replay tooling, and automation features to verify that contract parity, resilience, coverage, security posture, and maintainability still hold.

This is an **audit/hardening release, not a feature release**. Do not add large features in v1.16 unless the audit finds a blocker. The goal is to verify and harden everything added from v1.8 through v1.15. It reuses the v1.6 audit methodology, updated for the expanded system, and must explicitly cover the new architecture (NestJS server, Go modular router, observability/history, replay tooling, automation/policy) — not only repeat v1.6 checks.

### Primary Audit Goals

- Verify TS NestJS server parity with the Go service, and Go modular-router parity with the TS server.
- Verify `tools/cli` and `tools/replay` boundaries.
- Verify observability/history persistence safety.
- Verify automation/policy/preflight behavior.
- Verify config/group/profile/snapshot behavior.
- Verify diagnostics/export/redaction behavior.
- Verify coverage gates still represent meaningful coverage.
- Verify no public contract drift; verify docs and glossary still match reality.
- Identify v1.17 fix/hardening slices.

### Audit Set

Each audit is read-only analysis producing the listed output.

1. **Contract and Runtime Parity Audit** — TS NestJS vs Go modular service: public REST shape, status codes, error envelopes, activity values, config plan/apply/import/export, diagnostics/history/observability/policy/preflight endpoints, CLI DTO compatibility. *Output:* contract gap list, parity risk list, recommended contract scenario additions. **Run this first** — after the NestJS and Go router migrations, contract parity is the highest-risk surface; confirming it first prevents chasing architecture/doc issues before knowing whether behavior drift exists.
2. **Architecture and Module Boundary Audit** — NestJS module/controller/service boundaries, Go app/router/route-module boundaries, shared domain logic, `tools/cli` vs `tools/replay`, history/observability and config/policy/profile/snapshot boundaries. Are controllers/handlers thin, business logic outside HTTP adapters, TS/Go organized similarly, replay truly offline, modules cohesive, dependencies explicit? *Output:* module-boundary findings, architecture diagrams, v1.17 refactor candidates.
3. **Resilience and Data Durability Audit** — rules config persistence, history/activity persistence, snapshots/backups, diagnostics export, replay input artifacts, policy/preflight failure handling, temporary starts/schedules, corrupt-file recovery, bounded retention, shutdown. Can corrupt history brick startup? Can snapshot restore corrupt config? Are history writes isolated from rules config, backup/restore atomic enough, failures visible/recoverable, retention enforced, replay artifacts versioned/validated? *Output:* durability risk table, corruption/recovery matrix, accepted gaps vs must-fix.
4. **Security and Local-Safety Audit** — local management API assumptions, LAN-exposure warnings, policy/preflight guardrails, diagnostics redaction, sensitive data in history/activity/replay bundles, any introduced tokens/secrets, file-path exposure, browser/local storage. Are bundles safe to share? Does replay preserve redaction? Are host/IP/rule names redacted consistently? Do policy/preflight features create false security claims? Are secrets ever logged/exported? *Output:* local security posture, redaction gaps, policy/preflight wording risks.
5. **Observability and Replay Audit** — persistent activity history, connection/session history, timeline UI/API, metrics/stats, diagnostics bundle v2, `tools/replay`. Is history bounded, are timeline events correlated correctly, metrics low-cardinality/safe, replay offline-only and deterministic/source-cited, failing cleanly on invalid input, exports versioned/backward-compatible? *Output:* observability coverage map, replay correctness findings, export/input schema risks.
6. **Automation, Policy, and Workflow Audit** — groups/profiles, policy metadata, preflight, temporary starts, recipes, snapshots/restore, saved views/filters, local analyzer. Are policy violations warn/block consistent, preflight explicitly best-effort/race-aware, temporary starts reliable, recipes previewable/rollback-aware, snapshots using plan/apply, automation events visible, group/profile ops parity-tested? *Output:* workflow safety matrix, policy/preflight invariants, rollback gaps.
7. **Testing and Coverage Audit** — unit/integration/E2E balance, contract/CLI/replay/history coverage, framework-migration coverage, EADDRINUSE/determinism, gate strategy. Are gates still meaningful, did NestJS/chi add wrapper noise, are new tools in validation, important error paths covered or documented, E2E still user-meaningful, impossible branches documented not chased? *Output:* coverage posture, gate recommendations, flake risks, meaningful coverage gaps.
8. **Complexity, Duplication, and Maintainability Audit** — NestJS modules, Go route modules, config/policy/history/replay logic, repeated DTO mapping/validation, duplicate TS/Go contract semantics, CLI vs replay duplication, large UI components. Are abstractions paying for themselves, modules right-sized, did v1.14/v1.15 reduce complexity, any accidental CLI/replay duplication, any `runScenarios`-like high-complexity functions? *Output:* top complexity hotspots, duplication map, refactor candidates.
9. **Documentation, Glossary, and Operator UX Audit** — `README.md`, `docs/api-contract.md`, `docs/glossary.md`, `docs/checklist.md`, `tools/cli/readme.md`, `tools/replay/readme.md`, terminology, CLI/UI wording. Does the glossary still match the product, are new terms documented, CLI/replay responsibilities clear, operator warnings actionable, docs current (not old v1.6 assumptions), deferred/accepted gaps documented? *Output:* doc drift list, terminology update list, UX wording risks.
10. **Release Readiness and Packaging Audit** — build/release scripts, runtime smoke, release artifacts, version injection, CLI and replay packaging, web UI static serving, platform scripts. Do all scripts still work, does packaging include needed tools, is `tools/replay` packaged or intentionally separate, are versions injected consistently, does smoke cover static UI + API, are install docs accurate? *Output:* release readiness matrix, packaging gaps, final v1.16 decision.

### Output Artifacts

```text
audits/v1.16-contract-parity-audit-1.md
audits/v1.16-architecture-boundary-audit-1.md
audits/v1.16-resilience-durability-audit-1.md
audits/v1.16-security-local-safety-audit-1.md
audits/v1.16-observability-replay-audit-1.md
audits/v1.16-automation-policy-audit-1.md
audits/v1.16-testing-coverage-audit-1.md
audits/v1.16-complexity-maintainability-audit-1.md
audits/v1.16-docs-ux-audit-1.md
audits/v1.16-release-readiness-audit-1.md
audits/v1.16-audit-synthesis-and-fix-plan.md
```

### Process

1. Run all ten audits as read-only analysis.
2. Produce a synthesis and fix plan.
3. Classify findings: **MUST / SHOULD / NICE / DEFER / DO NOT DO**.
4. Execute only the MUST/SHOULD fixes needed for v1.16.
5. Run the release-readiness checkpoint.
6. Prepare the v1.16 release only after full validation passes.

### Audit Principles

- Reuse the v1.6 discipline; cite exact files/functions/modules.
- Prefer small fix slices; update durable docs after each fix.
- Keep coverage meaningful, not theatrical.
- Do not let framework migrations hide behavior drift.
- Validate across both runtimes; validate tools separately from the runtime.
- Do not expand scope into remote/team/security unless intentionally scheduled.

### Acceptance Criteria for v1.16

Portier v1.16 is ready when:

- All audits and the synthesis are complete; no MUST findings unresolved.
- TS NestJS server and Go modular service contract parity is verified.
- CLI and replay tool responsibilities are clear.
- Observability/history/replay durability is verified.
- Policy/preflight/automation behavior is verified.
- `docs/` and the glossary are current.
- All coverage gates pass with no lowering.
- `validate:contract`, `validate:coverage`, `validate:config`, `validate:cli`, replay-tool validation, runtime smoke, and E2E all pass.
- The release-readiness checkpoint is PASS, or conditional only on release mechanics.

**Recommended first task:** run the Contract and Runtime Parity Audit first (highest-risk surface after the NestJS and Go router migrations).

### Non-Goals for v1.16

- A broad rewrite.
- A new remote/team/auth model (unless intentionally introduced in an earlier release).
- Public API renames.
- Contract codegen (unless a prior release explicitly chose it).
- Chasing structurally unreachable branches with brittle tests.
- Lowering coverage gates to accommodate framework migrations.
- Speculative abstractions.

---

## Road to 2.0 — Stable Local-First Edition

**Product decision: Portier 2.0 remains local-first.** Remote/team/auth management is explicitly deferred beyond 2.0. The goal is not to become a multi-user remote administration platform — it is to become an extremely reliable local port-forwarding manager with excellent installation, upgrade, recovery, diagnostics, and release quality.

> Keeping 2.0 focused and achievable — an excellent local tool with safe config, great install, strong recovery, validated artifacts, and a clear upgrade path — is a stronger 2.0 than smuggling in a half-built remote administration model.

The final path to 2.0:

```text
v1.17 — Migration & Recovery
v1.18 — Install, Service & Upgrade Experience
v1.19 — 2.0 RC Hardening
v2.0  — Stable Local-First Portier
```

**Portier 2.0 is ready when:** local install is reliable on supported platforms; upgrade from v1.x is safe and documented; config migration and recovery are tested; release artifacts are reproducible and validated; the Windows service install/uninstall/update flow is clear; macOS/Linux install guidance is clear; CLI, UI, Go service, TypeScript fallback, and tools have clear responsibilities; diagnostics/replay/history artifacts are versioned; all release validations pass; no coverage gates are lowered; no unresolved MUST findings remain; and remote/team/auth is explicitly documented as out of scope for 2.0.

---

## Portier v1.17 — Migration & Recovery

**Core theme: Make config, history, diagnostics, and tool artifacts safe to migrate and recover.**

### Goals

Schema versioning; migration safety; backup-before-migration; rollback on migration failure; snapshot/restore hardening; artifact format versioning; corrupt-file recovery policy.

### Scope

**Config schema versioning** — add explicit config schema versioning if not already present: the current rules config has a version marker (or equivalent migration metadata); unknown/future versions fail safely; missing legacy versions migrate safely; migration is tested in both runtimes where applicable.

**Migration tool** — add a local migration command. Recommended default: keep config migration in `tools/cli` (config-oriented, operator-facing), e.g. `portier config migrate ./rules.json [--write] [--backup-out ./rules.backup.json]`. A separate `tools/migrate/` is the alternative if it should live outside the live CLI.

**Backup and rollback** — before any destructive migration: write backup → validate backup → migrate to temp output → validate migrated output → replace atomically where possible → roll back (or leave the original untouched) on failure.

**Snapshot restore hardening** — if snapshots exist by this point: restore must use plan/apply safety, support dry-run, create a backup, and never bypass duplicate-binding/policy checks.

**History / diagnostics / replay artifact versioning** — if v1.12/v1.13 introduced history and replay artifacts: version the exported history/timeline schemas, the diagnostics bundle schema, and the replay input schema; tools must reject unsupported versions clearly.

### Acceptance Criteria for v1.17

- Config migration is tested.
- Corrupt config recovery behavior is documented.
- Backup-before-migration works; a failed migration leaves the original config intact.
- Snapshot restore is dry-runnable and safe.
- Replay/diagnostics artifacts are versioned.
- `validate:config` covers migration fixtures.
- `validate:contract` remains green; all coverage gates pass.

---

## Portier v1.18 — Install, Service & Upgrade Experience

**Core theme: Make Portier pleasant and reliable to install, run, stop, update, and uninstall — the "great install" release.**

### Goals

Polished Windows installer and portable package; clear service install/uninstall/update flow; validated upgrade path; version reporting everywhere; platform-specific smoke validation; installer documentation; artifact checksums; first-run sanity checks.

### Scope

**Windows installer polish** — install/uninstall/update the service; preserve config; back up config before update; validate the service can start after install; validate the web UI is served; validate the CLI can reach the runtime; clear error messages when permissions are missing.

**Windows portable package** — portable zip contains service, CLI, web UI, docs; portable startup works without the installer; config path is documented; version command works; runtime smoke passes from the portable layout.

**macOS install guidance** — not necessarily a signed app/pkg yet, but good docs: recommended install location, binary placement, config path, launchd example if supported, start/stop and uninstall instructions.

**Linux install guidance** — binary placement, config path, systemd unit example if supported, start/stop/status, uninstall, upgrade.

**Version reporting** — consistent across CLI `--version`, the Go service runtime endpoint, the TypeScript fallback runtime endpoint, the UI footer/about/API docs (if applicable), the diagnostics bundle, release artifacts, and installer metadata where practical.

**Release artifact validation** — strengthen `npm run build:release:current` / `npm run validate:release:current` to verify: artifact exists and contains expected files; service starts; `/api/health` works; web UI served; CLI can connect; version injected correctly; sane config-path behavior; no missing static assets; checksums generated.

**Upgrade path** — document and test: v1.x config survives upgrade; service restart after upgrade works; old config migrates if needed; backup before upgrade; rollback notes if upgrade fails.

### Acceptance Criteria for v1.18

- Windows installer and portable package validate; current-platform release build validates.
- Version injection verified.
- Service install/uninstall docs are accurate; macOS/Linux docs are usable.
- Upgrade from the previous stable version is documented; checksums generated.
- No release artifact contains unexpected junk; runtime smoke passes from the release artifact.

---

## Portier v1.19 — 2.0 RC Hardening

**Core theme: No new features. No architecture churn. Release-candidate stabilization only.**

### Goals

Final full validation; packaging verification; docs pass; upgrade guide; local-safety audit; release checklist; final deferred list; final 2.0 readiness decision.

### Scope

**Full validation** — run the complete release matrix: lint, typecheck, unit tests, Go tests, CLI validation, replay/tool validation, config validation, contract validation, coverage validation, runtime smoke, E2E, release build, release artifact validation.

**Local-safety audit** — LAN exposure warnings; privileged-port warnings; diagnostics redaction; history/replay privacy; config import safety; static-serving safety; installer/service permissions; no secrets in logs; no dangerous default bind behavior.

**Docs pass** — README, install docs, upgrade guide, CLI docs, API contract docs, glossary, troubleshooting, release notes, known limitations, and the deferred-beyond-2.0 list.

**Upgrade guide** — create `docs/upgrade-v2.md`: supported upgrade path; backup recommendation; config migration behavior; install/update steps; rollback guidance; breaking changes (ideally none, or clearly listed); remote/team/auth explicitly out of scope.

**Final deferred list** — classify remaining items as v2.1 / v2.x later / explicitly not planned / accepted local-first limitation.

### Acceptance Criteria for v1.19

- No unresolved release blockers; no unresolved MUST findings.
- All release validations pass; release artifacts validate.
- Install/upgrade docs and the upgrade guide are complete.
- The local-first scope is clearly documented; remote/team/auth deferred beyond 2.0.
- The final release-readiness checkpoint is PASS, or conditional only on mechanical release actions.

---

## Portier v2.0 — Stable Local-First Portier

**Core theme: Stable, polished, local-first Portier.**

### Release Promise

Portier 2.0 is the first version where users can rely on: stable local runtime behavior; a stable REST API compatibility policy; a stable config schema/migration policy; stable CLI behavior/exit-code policy; a stable diagnostics/history/replay artifact policy; a robust install and upgrade process; validated release artifacts; and a documented recovery path.

### What v2.0 Includes

Mature rule management; doctor/config tooling; policy/preflight/automation features (if already landed); observability/history; the replay offline tool; the NestJS TypeScript fallback server; the modular Go service router; post-migration audit fixes; migration/recovery; polished install/upgrade; and release-candidate hardening.

### What v2.0 Does Not Include

Remote management; a team/user/role system; an authentication model; cloud sync; OAuth; a plugin framework; public API renames for cosmetic reasons.

### Acceptance Criteria for v2.0

Portier 2.0 is ready when:

- The v1.19 release-readiness checkpoint passes.
- Release artifacts are built and validated.
- The upgrade path from the previous stable v1.x is documented and tested.
- Version injection is correct everywhere; checksums are generated; install docs are complete.
- No coverage gates are lowered.
- `validate:contract`, `validate:coverage`, `validate:config`, `validate:cli`, replay/tool validations, runtime smoke, and E2E all pass.
- Release notes are complete.
- The tag/publish process is explicit and manual.
