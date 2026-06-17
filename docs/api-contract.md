# API Contract

This document covers the REST API currently used by the Portier client. The TypeScript server (NestJS, under `server/sources/api/`) is the reference implementation; shared shapes live in `shared/sources/index.ts`. The native Go service under `service/` implements the same contract and is the preferred production runtime. `validate:contract` guards parity between the two.

Errors are returned as JSON with an `errors: string[]` property for validation, conflict, not found, unknown API route, and unexpected server errors.

For canonical terminology used in this document (forward rule, runtime `node`/`go`, config plan/apply/import, advisory vs warning, live connection vs UDP session, etc.), see `docs/glossary.md`.

## `GET /api/forwards`

Purpose: list configured forwarding rules.

Request body: none.

Response: `ForwardRuleResponse[]`.

`ForwardRuleResponse` is a `ForwardRule` plus `advisories: PortAdvisory[]`.

### Optional rule group metadata

A forward rule MAY carry an optional `group` label: operator-facing, **behavior-neutral**
metadata used to organize rules. It does **not** affect forwarding, lifecycle,
duplicate-binding (still `protocol + listenHost + listenPort` only), or status behavior.

- `group`: optional string. Omitted (not `null`, not empty) when the rule has no group — legacy rules without a `group` remain valid and are returned unchanged.
- Validation (identical in the TypeScript and Go runtimes, parity-tested via `validate:contract`):
  - Whitespace is trimmed. An empty or whitespace-only value normalizes to **absent** (the rule has no group; the field is omitted).
  - A present (non-empty, post-trim) value must be **≤ 64 characters** and contain **no control characters** (C0 range `U+0000`–`U+001F` or DEL `U+007F`); otherwise the rule is rejected with `group must be 64 characters or fewer.` / `group must not contain control characters.`
  - A non-string value is rejected with `group must be a string.`
- `PATCH /api/forwards/:id` semantics: `group: ""` (or whitespace) **clears** the group; a non-empty string **sets** it (trimmed); omitting the field, or sending `null`, leaves the existing group **unchanged**.
- Preserved across config export/import and config plan/apply. In a plan, `group` is a **material** field (a group-only change is reported as an `update`) but **not** a forwarding field, so the update is **non-destructive** (no socket restart).

## `POST /api/forwards`

Purpose: create a forwarding rule.

Request body: `ForwardRuleInput`.

Response: `201` with `ForwardRuleResponse`.

Errors:

- `400` for validation errors.
- `409` for duplicate `protocol + listenHost + listenPort` bindings.

## `PATCH /api/forwards/:id`

Purpose: update an existing forwarding rule.

Request body: partial forward rule input. Exact patch validation is implemented in `shared/sources/index.ts` through `validateForwardRulePatch`.

Response: `ForwardRuleResponse`.

Errors:

- `400` for validation errors.
- `404` when the rule is not found.
- `409` for duplicate `protocol + listenHost + listenPort` bindings.

## `DELETE /api/forwards/:id`

Purpose: delete an existing forwarding rule.

Request body: none.

Response: `204` with no body.

Errors:

- `404` when the rule is not found.

## `POST /api/forwards/:id/start`

Purpose: start a configured forwarding rule.

Request body: none.

Response: `ForwardStatus`.

Errors:

- `404` when the rule is not found.
- `500` for unexpected start failures.

Go service: TCP and UDP rules start real forwarding.

## `POST /api/forwards/:id/stop`

Purpose: stop a configured forwarding rule.

Request body: none.

Response: `ForwardStatus`.

Errors:

- `404` when the rule is not found.
- `500` for unexpected stop failures.

Go service: TCP and UDP rules close the listener and active sockets.

## `POST /api/forwards/groups/:group/start` / `POST /api/forwards/groups/:group/stop`

Purpose: start or stop **all** rules that share a `group` label. These are
**behavior over existing rule metadata**: they never change rule definitions, order,
`enabled`/autostart, or `group`, and duplicate-binding behavior is unchanged. The
per-rule lifecycle is identical to the single-rule `:id/start` and `:id/stop` endpoints.

Path: `:group` is the URL-encoded group label. It is validated like any group label and, additionally, must be **non-empty** (a group operation needs a target): trimmed; rejected (`group is required.`) when empty/whitespace; otherwise ≤ 64 characters with no control characters. Matching uses the normalized (trimmed) group value and is exact/case-sensitive.

Request body: none.

Behaviour:

- Rules are processed in **rule order** (the manager's stored order); `results` is deterministic.
- **start**: each matched rule that is not already running is started — `enabled`/autostart is **not** a precondition (matching single-rule start). An already-running rule is `skipped` with reason `already_running`. A start error yields `failed` (reason = error message) and does not stop the rest of the group.
- **stop**: each matched running rule is stopped (`stopped`); a rule that is not running is `skipped` with reason `not_running`.
- Per-rule activity events (`rule.started`/`rule.stopped`/`rule.error`) fire exactly as for single-rule operations; no new group-level event type is introduced.

Response: `200` with `GroupActionResponse`:

```
{
  "group": "web",
  "action": "start" | "stop",
  "total": 2,
  "succeeded": 2,   // started or stopped
  "skipped": 0,
  "failed": 0,
  "results": [
    { "ruleId": "...", "ruleName": "...", "status": "started" },
    { "ruleId": "...", "ruleName": "...", "status": "skipped", "reason": "already_running" }
  ]
}
```

Per-rule `status` is `started` | `skipped` | `failed` (start) or `stopped` | `skipped` | `failed` (stop). `reason` carries the skip token (`already_running` / `not_running`) or, for `failed`, the error message; it is omitted on success.

Errors:

- `400` for an invalid group label (`group is required.`, `group must be 64 characters or fewer.`, `group must not contain control characters.`).
- `404` (standard `{ errors: [...] }` envelope) when **no rule** has the requested group — group operations do not silently no-op on an absent target.

Ungrouped bulk actions are intentionally **not** provided by these endpoints. The
TypeScript server and Go service implement identical behavior and response shape;
`validate:contract` guards parity.

## `GET /api/status`

Purpose: list runtime status for configured rules.

Request body: none.

Response: `ForwardStatus[]`.

### Rule health

Each `ForwardStatus` carries a required `health: "healthy" | "warning" | "error"` field — an operator-facing classification **derived deterministically from existing runtime state**. It performs **no target probing and no background check**; it is purely a function of `enabled`, `running`, and `lastError`. Health is distinct from the lifecycle `running` state.

Derivation (identical in the TypeScript server and Go service, parity-tested via `validate:contract`; priority order):

- `error` — the rule has a current non-empty `lastError` (a failed start or a socket error), regardless of `running`/`enabled`.
- `warning` — the rule is `enabled` (autostart) but is **not** running, and has no error (it is expected to run but isn't).
- `healthy` — running cleanly, or intentionally stopped (not `enabled`, no error).

Notes: `health` is computed by the manager (which owns the rule's `enabled` flag); forwarders do not classify it. Health does **not** mutate the rule. The exact `lastError` lifecycle is unchanged from prior versions — in particular, whether a `lastError` survives a failed start follows the existing per-runtime behaviour.

## `GET /api/ports/advisory`

Purpose: get advisory messages for a port, host, and purpose.

Query parameters:

- `port`: integer from `1` to `65535`.
- `purpose`: `forward` or `management`.
- `listenHost`: optional string.

Response: `PortAdvisory[]`.

Errors:

- `400` when `port` is missing, not an integer, or outside `1-65535`.
- `400` when `purpose` is not `forward` or `management`.

## `POST /api/forwards/reorder`

Purpose: reorder forwarding rules by providing IDs in the desired order. Does not restart running rules.

Request body: `{ "ids": string[] }` — all existing rule IDs in the desired order.

Response: `ForwardRuleResponse[]` (updated order).

Errors:

- `400` when `ids` is not an array of strings.
- `404` when any ID is not found.

## `GET /api/config/export`

Purpose: export all forwarding rules as a portable JSON config.

Request body: none.

Response:

```json
{
  "version": "1",
  "exportedAt": "ISO timestamp",
  "rules": ForwardRule[]
}
```

Sensitivity: the exported config contains no secrets, but it reveals local network topology (listen/target hosts and ports, rule and group names). Treat an exported config as local diagnostic data and review it before sharing.

## `POST /api/config/import`

Purpose: import rules from a previously exported config. Validates all rules atomically — no partial import.

Request body:

```json
{
  "mode": "replace" | "merge",
  "config": { "version": "1", "exportedAt": "...", "rules": ForwardRule[] }
}
```

Behavior:
- `replace`: stops all running rules, removes all existing rules, applies imported rules, restarts enabled ones.
- `merge`: adds imported rules. IDs that clash with existing rules are regenerated. Listen binding conflicts with existing rules reject the entire import.
- Both modes: two rules **within the imported set** that share a listen binding (`protocol + listenHost + listenPort`) reject the entire import — even when their IDs differ. Both runtimes enforce this identically.

Response: `{ result: ImportResult, rules: ForwardRuleResponse[] }`.

Errors:

- `400` when `mode` is not `replace` or `merge`.
- `400` when `config` is missing or not a valid Portier v1 config.
- `422` when any rule fails validation; no rules are imported.
- `422` when the imported set contains a duplicate listen binding (`protocol + listenHost + listenPort`); no rules are imported. Body is `{ errors: [...], result }` with `result.imported === 0`.

## `GET /api/activity`

Purpose: list recent in-memory activity events (rule lifecycle, TCP connections, UDP packets).

Query parameters:

- `limit`: optional integer, default `100`, max `500`. Clamped to `[1, 500]`.
- `ruleId`: optional string. Filter to events for a specific rule.
- `type`: optional `ActivityEventType` string.
- `severity`: optional `"info" | "success" | "warning" | "error"`.

Response:

```json
{ "events": ActivityEvent[] }
```

Events are returned newest first.

**`ActivityEventType` values (17, contract values — not cosmetic labels):**
`rule.created`, `rule.updated`, `rule.deleted`, `rule.started`, `rule.stopped`, `rule.error`, `tcp.connection.opened`, `tcp.connection.closed`, `tcp.connection.error`, `udp.packet.forwarded`, `udp.packet.returned`, `udp.packet.error`, `udp.session.opened`, `udp.session.closed`, `config.exported`, `config.imported`, `config.import.failed`.

**`ActivitySeverity` values (4):** `info`, `success`, `warning`, `error`.

Both runtimes must declare and emit exactly this set. `validate:contract` guards value membership and cross-runtime parity (the declared TS union, the declared Go consts, and the runtime-emitted values), so a rename/typo/drift in either runtime — not just a shape mismatch — fails the contract. Adding a new event type or severity requires updating `@portier/shared` (`shared/sources/activity.ts`), the Go consts (`service/sources/activity/activity.go`), this list, and the `validate:contract` expected sets together.

**Limitations:**
- Activity is stored in memory only. Restarting the server clears all activity.
- The store is bounded to the latest 500 events.
- UDP packet events are throttled to at most one log entry per second per rule to avoid flooding.

## `DELETE /api/activity`

Purpose: clear the in-memory activity log.

Both runtimes implement this endpoint. Does not affect forwarding rules or runtime state.

Response: `204 No Content`

## `GET /api/runtime`

Purpose: expose runtime environment details for the local management UI.

Both runtimes implement this endpoint with the same response shape.

This endpoint is local-admin oriented — it exposes file paths, process info, and runtime config, which is intentional for operational transparency in a local management tool.

Response:

```json
{
  "name": "Portier",
  "version": "1.2.0",
  "runtime": "node" | "go",
  "platform": "windows" | "macos" | "linux" | "unknown",
  "arch": "x64" | "arm64" | "unknown",
  "uptimeSeconds": 123,
  "startedAt": "2026-01-01T00:00:00Z",
  "managementHost": "127.0.0.1",
  "managementPort": 47831,
  "configPath": "/path/to/rules.json",
  "staticDir": "/path/to/web",
  "serviceMode": false,
  "pid": 12345
}
```

Field notes:
- `version`: read from the server `package.json` (TypeScript) or injected at build time via `-ldflags` (Go; defaults to `"dev"` for development builds).
- `runtime`: `"node"` for the TypeScript server, `"go"` for the native Go service.
- `platform`: normalized from `process.platform` / `runtime.GOOS`.
- `arch`: normalized from `process.arch` / `runtime.GOARCH`; `"x64"` maps from `amd64`.
- `uptimeSeconds`: computed on each request from startup time.
- `startedAt`: ISO 8601 / RFC 3339 string of service startup time.
- `serviceMode`: reflects the `--service` flag / headless mode.

## `POST /api/forwards/:id/diagnose`

Purpose: run diagnostic checks against an existing forward rule without changing rule state.

Both runtimes implement this endpoint with the same response shape.

Request body: empty or `{}`. No body is required; the rule configuration is read from the server state.

Response: `RuleDiagnosticsResult`.

Errors:

- `404` when the rule is not found.

No state is mutated. The rule is not started, stopped, or modified. No sockets remain open after the response.

### Response shape

```json
{
  "ruleId": "string",
  "ruleName": "string",
  "protocol": "tcp" | "udp",
  "summary": {
    "status": "pass" | "warn" | "fail",
    "message": "string"
  },
  "checks": [
    {
      "id": "string",
      "label": "string",
      "status": "pass" | "warn" | "fail" | "skip",
      "message": "string",
      "details": { "key": "value" }
    }
  ],
  "diagnosedAt": "ISO timestamp"
}
```

### Check IDs

| ID | When it runs | Status |
|----|-------------|--------|
| `listen-host` | Always | `pass` for specific host; `warn` for `0.0.0.0` |
| `lan-exposure` | Always | `pass` unless listenHost is `0.0.0.0`; then `warn` |
| `privileged-port` | Always | `warn` for ports below 1024 |
| `common-port` | Always | `warn` if listenPort is a well-known service port |
| `listen-bind` | Always | `pass` if port is available to bind; `fail` if occupied or permission denied; `pass` if rule is running (Portier owns the socket) |
| `target-host` | Always | `pass` if targetHost resolves; `fail` if DNS lookup fails |
| `target-connect` | TCP: after target-host. UDP: always | TCP: `pass` if TCP connection succeeds; `fail` if refused/timeout. UDP: always `skip` (UDP reachability cannot be verified) |
| `udp-mode` | UDP only | `pass` for `one-way` and `bidirectional-multi-client`; `warn` for `bidirectional-last-client` |

### Running-rule listen-bind behavior

If the rule is currently running, its own listener already occupies the port. The diagnostic does not attempt a bind and returns `pass` with message: `"Rule is currently running; the listen port is already owned by Portier."` This prevents a false failure when Portier itself is the occupant.

### UDP target-connect limitation

UDP reachability cannot be proven without a protocol-specific response from the target. The `target-connect` check is always `skip` for UDP rules. This is by design, not an omission.

## `GET /api/connections`

Purpose: return a read-only snapshot of active TCP connections and UDP sessions for all running forwarding rules, along with per-rule live traffic summaries.

Both runtimes expose the same response shape.

Request body: none.

Response: `LiveConnectionsResponse`.

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

Field notes:
- `generatedAt`: ISO 8601 timestamp of when the snapshot was generated.
- `tcpConnections`: active TCP forwarding connections. Empty array when none are active.
- `udpSessions`: active and recently-idle UDP sessions. Empty array when none are tracked.
- `ruleSummaries`: per-rule aggregation of live traffic state. Includes all running rules, even those with no active connections.
- `id` values are stable for display during the process lifetime but do not persist across restarts.
- `ruleName`: included for display convenience; empty string (`""`) when the name cannot be resolved.
- `durationMs`: milliseconds since the TCP connection was accepted.
- `idleMs`: milliseconds since the last UDP packet was seen for the session.
- `bytesIn`: bytes transferred from client to target.
- `bytesOut`: bytes transferred from target to client.
- `packetsIn` (UDP): packets received from the client and forwarded to the target.
- `packetsOut` (UDP): packets received from the target and returned to the client.
- TCP `status`: `"active"` while both sockets are open.
- UDP `status`: `"active"` while traffic is being seen; `"idle"` after 30 seconds of no traffic. Sessions are retained up to 5 minutes after becoming idle.
- `lastTrafficAt`: most recent traffic timestamp for the rule across all connections/sessions. `null` when no traffic has been seen since the rule started.
- Data is operational metadata only. Payload contents are never exposed.

**Limitations:**
- Live state is in-memory only. Data resets on service restart.
- UDP `one-way` mode: per-client session metadata may be limited; see UDP mode notes.
- UDP `bidirectional-last-client` mode: only the most recent client session is available.

## `POST /api/config/plan`

Purpose: compare a desired config against the currently running Portier configuration and return a structured plan showing adds, updates, removes, and unchanged rules. Read-only — does not modify state.

Both runtimes implement this endpoint.

Request body:

```json
{
  "desired": {
    "rules": [...]
  }
}
```

The `desired.rules` array accepts the same rule input shape as `POST /api/config/import`. The `exportedAt` and `version` wrapper fields are not required.

Response: `ConfigPlanResponse`.

```json
{
  "generatedAt": "ISO timestamp",
  "mode": "plan",
  "summary": {
    "add": 1,
    "update": 0,
    "remove": 1,
    "unchanged": 2,
    "destructive": 1,
    "hasDrift": true,
    "hasErrors": false
  },
  "operations": [
    {
      "type": "add" | "update" | "remove" | "unchanged",
      "ruleId": "string (when matched)",
      "ruleName": "string",
      "protocol": "tcp" | "udp",
      "current": "ConfigPlanRuleSnapshot | undefined",
      "desired": "ConfigPlanRuleSnapshot | undefined",
      "changes": [{ "field": "string", "before": "any", "after": "any" }],
      "destructive": true | false
    }
  ],
  "errors": [{ "code": "string", "message": "string", "field": "string (optional)" }],
  "warnings": [{ "code": "string", "message": "string" }]
}
```

Matching semantics:
- Match by stable rule `id` if present in both current and desired config.
- If the desired rule has no `id`, match by identity key: `protocol + listenHost + listenPort`.
- If no match, the desired rule is an `add` operation.
- If a current rule has no desired match, it is a `remove` operation.
- If matched and material fields differ, the operation is `update`.
- If matched and all material fields are equal, the operation is `unchanged`.

Material fields (those that trigger `update` and potentially `destructive: true`):
- `name`, `protocol`, `listenHost`, `listenPort`, `targetHost`, `targetPort`, `enabled`, `udpMode`, `group`

Destructive operations:
- `remove` is always destructive.
- `update` is destructive when any of `protocol`, `listenHost`, `listenPort`, `targetHost`, `targetPort`, or `udpMode` changes. `name`, `enabled`, and `group` are non-destructive (metadata only — no socket restart).

Error conditions:
- `400` when `desired.rules` is not a valid array.
- `400` when any rule fails field validation.
- `400` when two desired rules share the same identity key (ambiguous match).
- `400` when a desired rule id matches multiple current rules.

Does not mutate running config. Does not start or stop rules.

## `POST /api/config/apply`

Purpose: apply a desired config to the running configuration after explicit confirmation. Supports dry-run mode.

Both runtimes implement this endpoint.

Request body:

```json
{
  "desired": { "rules": [...] },
  "yes": true,
  "dryRun": false
}
```

- `desired` (required) — desired config with `rules` array.
- `yes` — required to be `true` when destructive operations (remove or forwarding-affecting update) are present; without it returns 400.
- `dryRun` — optional; when `true`, previews plan counts without mutating config. Dry-run does **not** require `yes: true`.

Response: `ConfigApplyResponse`.

```json
{
  "ok": true,
  "dryRun": false,
  "appliedAt": "ISO timestamp",
  "plan": { ...ConfigPlanResponse... },
  "applied": { "add": 0, "update": 0, "remove": 0, "unchanged": 1 }
}
```

Behavior:

- Plan errors → `200 ok:false`, no mutation. Plan errors include duplicate desired listen bindings (`protocol + listenHost + listenPort`), which are caught by the plan engine **before** any import — apply never silently succeeds on a duplicate-binding desired config.
- `dryRun: true` → `200 ok:true`, no mutation (does not require `yes`).
- Destructive operations without `yes: true` → `400`.
- No drift → `200 ok:true`, no import called.
- Drift present → replace import using desired rules; key-matched rules (unchanged/update) have their current `ruleId` injected to preserve IDs.
- **Invariant:** apply never reports `ok:true` when the underlying import reports errors. Reachable import errors are pre-blocked as plan errors (above); as a defensive guard against future drift, if the import step itself returns errors, apply responds `200 ok:false` with the errors surfaced in `plan.errors` (code `IMPORT_ERROR`) and `plan.summary.hasErrors: true`, and zero applied counts. Both runtimes behave identically.

## `GET /api/health`

Purpose: lightweight health probe for the native Go service.

TypeScript server: not implemented.

Go service response:

```json
{
  "ok": true,
  "server": "go",
  "name": "Portier"
}
```

## Static File Serving

In production, the server serves the built client alongside the API on the same origin.

- API routes under `/api` are handled before static files.
- Unknown `/api/*` routes return `404` with `{ "errors": ["API route was not found."] }`.
- Non-API routes return `index.html` from the configured static directory, enabling SPA client-side routing.
- Static assets (JS, CSS, images) are served directly from the static directory.
- If the static directory is missing or does not contain `index.html`, the API continues to function and the server logs a warning. Static routes are not registered in that case.

The static directory is configured by:
1. `--static-dir <path>` CLI flag
2. `PORTIER_STATIC_DIR` environment variable
3. Default: `client/build` for the repository TypeScript server runtime; `web` for the native Go service and packaged runtime layout. Packaged `server.js` fallback commands should pass `--static-dir web` or set `PORTIER_STATIC_DIR=web`.

## Shared Shapes

The client should import these from `@portier/shared`:

- `ForwardProtocol`
- `UdpMode`
- `ForwardRule`
- `ForwardRuleInput`
- `ForwardRuleResponse`
- `ForwardStatus`
- `PortAdvisory`
- `ActivityEvent`
- `ActivityEventType`
- `ActivitySeverity`
- `ActivityEventInput`
- `ExportedConfig`
- `ImportMode`
- `ImportResult`
- `RuntimeInfo`
- `DiagnosticStatus`
- `DiagnosticCheck`
- `DiagnosticSummary`
- `RuleDiagnosticsResult`
- port constants and advisory helpers

### Live connection types

The following types are defined in `@portier/shared`. The `GET /api/connections`
endpoint is implemented in both runtimes.

- `LiveConnectionsResponse` — top-level response for `GET /api/connections`
- `TcpConnectionInfo` — individual TCP connection record; `status: LiveConnectionStatus`
- `UdpSessionInfo` — individual UDP session record; `status: UdpSessionStatus`; `mode` matches existing `UdpMode` values
- `RuleLiveSummary` — per-rule aggregated live traffic summary; `lastTrafficAt` is `string | null`
- `LiveConnectionStatus` — `"active"` (TCP connections are either active or gone)
- `UdpSessionStatus` — `"active" | "idle"`

### Plan/diff/apply types

The following types are defined in `@portier/shared` (`shared/sources/plan.ts`). Both `POST /api/config/plan` and `POST /api/config/apply` are implemented in both the TypeScript server and Go service.

- `ConfigPlanOperationType` — `"add" | "update" | "remove" | "unchanged"`
- `ConfigPlanChange` — a single field-level change: `{ field, before, after }`
- `ConfigPlanRuleSnapshot` — rule config fields used for comparison (mirrors `ForwardRule`; no runtime state or advisories)
- `ConfigPlanOperation` — a single plan operation: `{ type, ruleId?, ruleName, protocol, current?, desired?, changes?, destructive }`
- `ConfigPlanSummary` — counts and drift flags: `{ add, update, remove, unchanged, destructive, hasDrift, hasErrors }`
- `ConfigPlanError` — a plan validation error: `{ code, message, field? }`
- `ConfigPlanWarning` — a plan advisory warning: `{ code, message }`
- `ConfigPlanResponse` — top-level response for `POST /api/config/plan`: `{ generatedAt, mode: "plan", summary, operations, errors, warnings }`
- `DesiredConfig` — the desired config wrapper: `{ rules: ConfigPlanRuleSnapshot[] }`
- `ConfigPlanRequest` — request body for `POST /api/config/plan`: `{ desired: DesiredConfig }`
- `ConfigAppliedCounts` — per-operation counts in apply response: `{ add, update, remove, unchanged }`
- `ConfigApplyRequest` — request body for `POST /api/config/apply`: `{ desired: DesiredConfig, yes: boolean, dryRun?: boolean }`
- `ConfigApplyResponse` — response for `POST /api/config/apply`: `{ ok: boolean, dryRun: boolean, appliedAt: string, plan: ConfigPlanResponse, applied: ConfigAppliedCounts }`
