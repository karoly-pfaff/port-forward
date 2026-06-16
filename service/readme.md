# Portier Go Service

This directory contains the native Go service implementation for Portier. It is the preferred runtime for production deployment: smaller binary, no Node.js dependency, no warm-up time. The TypeScript server in `../server/` remains supported as a reference implementation and fallback runtime. Both runtimes expose the same API contract.

The Go service is a full drop-in replacement for the TypeScript server. It covers option resolution, logging, HTTP API basics, an in-memory manager initialized from config, shared rule validation, port advisories, config load/save, rule management API routes, config import/export, real TCP forwarding, real UDP forwarding (one-way, bidirectional-last-client, bidirectional-multi-client), graceful shutdown, activity log, and built client static serving.

## Run

From `service/`:

```powershell
go run ./sources
```

To serve the existing built client from the repository root output:

```powershell
go run ./sources --static-dir ../client/build
```

The management API defaults to `127.0.0.1:47831`. Binding `--host 0.0.0.0` is supported for parity with the TypeScript server, but it exposes the management UI/API on the LAN and should be avoided unless deliberately secured.

## Build

From `service/`:

```powershell
go build -o build/portier-service ./sources
```

From the repository root:

```powershell
npm run build:service
```

Cross-compile for a specific platform (example: Windows from any OS):

```powershell
$env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
go -C service build -o build/windows/service.exe ./sources
```

## Options

CLI flags override environment variables, and environment variables override defaults.

| Option | Environment | Default |
| --- | --- | --- |
| `--config <path>` | `PORTIER_CONFIG` | `data/forwards.json` |
| `--host <host>` | `PORTIER_HOST` | `127.0.0.1` |
| `--port <port>` | `PORTIER_PORT` | `47831` |
| `--static-dir <path>` | `PORTIER_STATIC_DIR` | `web` |
| `--service` | n/a | `false` |

The static directory defaults to `web` relative to the working directory. This matches the packaged runtime layout where the web UI lives in `web/` next to the binary. For development, pass `--static-dir ../client/build` or use `npm run start:service`.

## Packaged Runtime Layout

When deployed as a package, the Go service expects:

```text
<install-dir>/
  service          (or service.exe on Windows)
  server.js        (Node server fallback, optional)
  web/
    index.html
    assets/
```

Start command:

```powershell
.\service.exe --service --config "C:\ProgramData\Portier\rules.json" --host 127.0.0.1 --port 47831 --static-dir ".\web"
```

## HTTP / Router Architecture

The HTTP layer is built from an explicit dependency container, `app.App`
(`sources/app/`), which bundles the resolved dependencies the handler needs
(`Manager`, `Options`, `StartedAt`, `Version`). `main.go` wires it up
(`options.Resolve` → `manager.NewFromConfig` → `app.New(...)` →
`api.NewHandler(app)`) and serves it via `http.Server` with graceful shutdown.
`api.NewHandler(app *app.App)` is the construction path.

`ServeHTTP` routes API paths (`/api` or `/api/*`) into `serveAPI` and otherwise
serves the static client (or a plain 404 when no client is built). `serveAPI`
first consults the **modular route table** (`dispatchModular`, exact method +
exact path — see `sources/api/routes.go`) for endpoints migrated into feature
route modules, then falls through to the legacy ordered method+path dispatch
(exact matches plus the `/api/forwards/groups/` and `/api/forwards/` prefixes),
falling through to a JSON `404 {"errors":["API route was not found."]}`. A method
mismatch on a known path returns that same JSON 404 (never a 405): the modular
table matches only on an exact method, so a wrong method falls through to the
legacy dispatch and its generic envelope. Shared response/request/error helpers
(`writeJSON`, `decodeRequest`, `readBody`, `writeManagerError`) live in
`sources/api/respond.go`; diagnose check helpers live in `sources/api/diagnose.go`.

**v1.15 — Go Service Modular Router (in progress):** the monolithic `api.go`
dispatcher is being reorganized into focused, `net/http`-compatible per-feature
route modules (`sources/api/<feature>_routes.go`) behind the `app.App` dependency
struct, preserving the REST contract, error envelopes, static serving, and
startup/shutdown semantics exactly (standard `net/http`, no router library).
**Migrated so far:** `GET /api/health` (`health_routes.go`) and `GET /api/runtime`
(`runtime_routes.go`); the shared response/request/error plumbing has been
extracted into `respond.go`. Everything else still flows through the ordered
`serveAPI` dispatch and will migrate feature-by-feature behind `validate:contract`.
See `audits/v1.15-go-router-audit-1.md` for the endpoint inventory, target layout,
route-registration pattern, and slice plan, and `docs/roadmap.md` (v1.15) for the
goals/non-goals.

## Implemented Endpoints

- `GET /api/health`
- `GET /api/runtime`
- `GET /api/forwards`
- `POST /api/forwards`
- `PATCH /api/forwards/:id`
- `DELETE /api/forwards/:id`
- `POST /api/forwards/:id/start`
- `POST /api/forwards/:id/stop`
- `POST /api/forwards/:id/diagnose`
- `POST /api/forwards/reorder`
- `POST /api/forwards/groups/:group/start`
- `POST /api/forwards/groups/:group/stop`
- `GET /api/status`
- `GET /api/ports/advisory`
- `GET /api/connections`
- `GET /api/config/export`
- `POST /api/config/import`
- `POST /api/config/plan`
- `POST /api/config/apply`
- `GET /api/activity`
- `DELETE /api/activity`

Response:

```json
{
  "ok": true,
  "server": "go",
  "name": "Portier"
}
```

Unknown `/api` routes return JSON 404 responses:

```json
{
  "errors": ["API route was not found."]
}
```

At startup, the Go service reads the resolved config path, validates rules, defaults missing UDP mode to `one-way`, and stores the rules in an in-memory manager. `GET /api/forwards` returns the TypeScript-compatible array shape with per-rule `advisories`.

If the config file is missing, the endpoint returns an empty array:

```json
[]
```

`GET /api/ports/advisory` accepts the same query params as the TypeScript server:

- `port`
- `listenHost`
- `purpose` (`forward` or `management`)

`GET /api/status` returns the TypeScript-compatible status array. TCP rules report actual running state, active connection count, byte counters, `startedAt`, and `lastError` when set. UDP rules report actual running state, packet counters (packetsIn, packetsOut), byte counters, `startedAt`, `lastError`, and `activeUdpSessions` (multi-client mode only).

`POST /api/forwards/:id/start` starts a real TCP listener for TCP rules and a real UDP listener for UDP rules. Both bind `listenHost:listenPort` before returning `running: true`. UDP rules support three modes:

- **one-way**: packets forwarded to target; no response path back to clients.
- **bidirectional-last-client**: responses from the target go back to the most recently seen client. Not suitable for concurrent clients — responses may be misrouted.
- **bidirectional-multi-client**: per-client sessions with dedicated target sockets. Responses route back to the correct originating client. Sessions expire after 60 seconds of idle.

`POST /api/forwards/:id/stop` closes the TCP listener and active sockets, or closes the UDP listener and all session sockets. Creating or importing enabled rules starts real forwarding after config persistence succeeds. Updating a forwarding-affecting field (listenHost, listenPort, targetHost, targetPort, protocol, udpMode) on a running rule restarts the forwarder.

`GET /api/config/export` returns the TypeScript-compatible export shape:

```json
{
  "version": "1",
  "exportedAt": "2026-06-06T00:00:00.000Z",
  "rules": []
}
```

Unknown `/api` routes return JSON 404 responses.

## Config Loading

The Go service reads config from `--config` / `PORTIER_CONFIG` / `data/forwards.json` during startup. Rule management routes persist changes back to the same path.

Supported shapes:

- The current TypeScript raw array of rules.
- An object wrapper with a `rules` array, such as an exported config file.

Invalid JSON, invalid rules, and duplicate listen bindings fail startup clearly.

The Go service saves config as the TypeScript-compatible raw rule array. It still accepts both raw arrays and exported config objects with a `rules` array when loading.

Saves are written as readable JSON through a temporary file in the target directory before replacing the configured file. Parent directories are created when needed. Invalid rules are rejected before writing so an existing config is not overwritten by invalid data.

## Activity Log

`GET /api/activity` returns a bounded in-memory log (newest-first, max 500 entries, default limit 100). The log resets on service restart. Events are emitted for rule lifecycle changes, TCP connections, and UDP packets/sessions.

Query parameters:

| Parameter | Description |
| --- | --- |
| `limit` | Maximum number of events to return (default 100, max 500) |
| `ruleId` | Filter by rule ID |
| `type` | Filter by event type (e.g. `rule.created`, `tcp.connection.opened`) |
| `severity` | Filter by severity (`info`, `success`, `warning`, `error`) |

Response shape:

```json
{
  "events": [
    {
      "id": "uuid",
      "timestamp": "2026-06-06T00:00:00.000Z",
      "type": "rule.created",
      "severity": "success",
      "ruleId": "...",
      "ruleName": "My rule",
      "protocol": "tcp",
      "message": "Rule created: My rule"
    }
  ]
}
```

**TCP events**: `tcp.connection.opened`, `tcp.connection.closed`, `tcp.connection.error` are emitted per connection. An error fires at most once per connection (either dial failure or copy failure); a close event fires only if no error occurred.

**UDP events**: `udp.packet.forwarded`, `udp.packet.returned`, `udp.packet.error` are throttled to at most once per second per forwarder to avoid log flooding. `udp.session.opened` and `udp.session.closed` are emitted per session in multi-client mode.

**Manager events**: `rule.created`, `rule.updated`, `rule.deleted`, `rule.started`, `rule.stopped`, `rule.error`, `config.exported`, `config.imported`, `config.import.failed`.

## Static Client Serving

The static directory defaults to `web` relative to the working directory, matching the packaged runtime layout. Override with `--static-dir` or `PORTIER_STATIC_DIR`.

If the static directory exists and contains `index.html`, the Go service serves files from that directory. Non-API unknown routes fall back to `index.html` for client-side routing.

If the static directory is missing or does not contain `index.html`, the server still starts and API routes still work. Startup logs include a warning that the web UI is unavailable.

## Tests

From `service/`:

```powershell
go test ./...
```

The tests cover option priority, validation, patch validation, forwarding-affecting field detection, advisories, config load/save, manager write paths (including activity events for all lifecycle operations), real TCP forwarding with activity events, real UDP forwarding (all three modes, session expiry, stats, stop, bind failure) with activity events, the health endpoint, rule CRUD, reorder, config import/export, UDP API start/stop/patch/delete, JSON 404s for unknown API routes, missing static directory behavior, static index fallback, and activity log endpoint (empty, populated, limit, ruleId filter, type filter, severity filter).
