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

## `GET /api/runtime`

Purpose: expose runtime environment details for the local management UI. Added in v1.2.

Both runtimes implement this endpoint with the same response shape.

This endpoint is local-admin oriented — it exposes file paths, process info, and runtime config, which is intentional for operational transparency in a local management tool.

Response:

```json
{
  "name": "Portier",
  "version": "1.1.0",
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
- port constants and advisory helpers
