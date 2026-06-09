# API Contract

This document covers the REST API currently used by the Portier client. The TypeScript server source in `server/sources/api.ts` remains the reference implementation; shared shapes live in `shared/sources/index.ts`. The native Go service under `service/` implements the same contract and is the preferred production runtime.

Errors are returned as JSON with an `errors: string[]` property for validation, conflict, not found, unknown API route, and unexpected server errors.

## `GET /api/forwards`

Purpose: list configured forwarding rules.

Request body: none.

Response: `ForwardRuleResponse[]`.

`ForwardRuleResponse` is a `ForwardRule` plus `advisories: PortAdvisory[]`.

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

## `GET /api/status`

Purpose: list runtime status for configured rules.

Request body: none.

Response: `ForwardStatus[]`.

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
- `merge`: adds imported rules. IDs that clash with existing rules are regenerated. Listen binding conflicts reject the entire import.

Response: `{ result: ImportResult, rules: ForwardRuleResponse[] }`.

Errors:

- `400` when `mode` is not `replace` or `merge`.
- `400` when `config` is missing or not a valid Portier v1 config.
- `422` when any rule fails validation; no rules are imported.

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

**Limitations:**
- Activity is stored in memory only. Restarting the server clears all activity.
- The store is bounded to the latest 500 events.
- UDP packet events are throttled to at most one log entry per second per rule to avoid flooding.

## `DELETE /api/activity`

Purpose: clear the in-memory activity log. Added in v1.2.

Both runtimes implement this endpoint. Does not affect forwarding rules or runtime state.

Response: `204 No Content`

## `GET /api/runtime`

Purpose: expose runtime environment details for the local management UI. Added in v1.2.

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

Purpose: run diagnostic checks against an existing forward rule without changing rule state. Added in v1.2.

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

Implemented in v1.4 Slice 7. Both runtimes expose the same response shape.

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

## `POST /api/config/plan` — Implemented (TypeScript server); Go service parity pending (v1.5 Slice 3)

Purpose: compare a desired config against the currently running Portier configuration and return a structured plan showing adds, updates, removes, and unchanged rules. Read-only — does not modify state.

Implemented in v1.5 Slice 2 in the TypeScript server. Go service parity is pending v1.5 Slice 3.

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
- `name`, `protocol`, `listenHost`, `listenPort`, `targetHost`, `targetPort`, `enabled`, `udpMode`

Destructive operations:
- `remove` is always destructive.
- `update` is destructive when any of `protocol`, `listenHost`, `listenPort`, `targetHost`, `targetPort`, or `udpMode` changes.

Error conditions:
- `400` when `desired.rules` is not a valid array.
- `400` when any rule fails field validation.
- `400` when two desired rules share the same identity key (ambiguous match).
- `400` when a desired rule id matches multiple current rules.

Does not mutate running config. Does not start or stop rules.

## `POST /api/config/apply` — Planned (v1.5)

Purpose: apply a desired config to the running configuration after explicit confirmation.

Planned for v1.5. Not yet implemented. `POST /api/config/plan` must be implemented first.

Request body:

```json
{
  "desired": { "rules": [...] },
  "yes": true,
  "backup": true
}
```

The `yes: true` field is required for destructive operations. The `backup` field is optional; when true, the response includes the pre-apply config snapshot.

Response: `ConfigApplyResponse`.

```json
{
  "appliedAt": "ISO timestamp",
  "applied": 3,
  "errors": [],
  "warnings": []
}
```

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

### Added in v1.4 Slice 2, implemented in v1.4 Slice 7

The following types are defined in `@portier/shared` as of v1.4 Slice 2. The `GET /api/connections` endpoint is implemented in both runtimes as of v1.4 Slice 7.

- `LiveConnectionsResponse` — top-level response for `GET /api/connections`
- `TcpConnectionInfo` — individual TCP connection record; `status: LiveConnectionStatus`
- `UdpSessionInfo` — individual UDP session record; `status: UdpSessionStatus`; `mode` matches existing `UdpMode` values
- `RuleLiveSummary` — per-rule aggregated live traffic summary; `lastTrafficAt` is `string | null`
- `LiveConnectionStatus` — `"active"` (TCP connections are either active or gone)
- `UdpSessionStatus` — `"active" | "idle"`

### Added in v1.5 — Plan/diff/apply types

The following types are defined in `@portier/shared` (`shared/sources/plan.ts`). `POST /api/config/plan` is implemented in the TypeScript server (v1.5 Slice 2); Go parity is pending Slice 3. `POST /api/config/apply` is pending Slices 3+.

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
- `ConfigApplyRequest` — request body for `POST /api/config/apply`: `{ desired: DesiredConfig, yes: boolean, backup?: boolean }`
- `ConfigApplyResponse` — response for `POST /api/config/apply`: `{ appliedAt, applied, errors, warnings }`
