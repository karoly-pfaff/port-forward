# Architecture

## Monorepo Layout

- `shared/` contains TypeScript types and validation helpers used by both sides.
- `server/` contains the TypeScript server implementation, REST API, JSON config store, rule manager, and TCP/UDP forwarders.
- `service/` contains the native Go service implementation for smaller binaries and service deployment.
- `client/` contains the React TypeScript web UI.

## Server Flow

1. `server/sources/index.ts` creates a `ConfigStore`, loads saved rules, starts enabled rules, and starts the HTTP server.
2. `ForwardManager` owns the in-memory rule list, duplicate listen binding checks, persistence, and forwarder lifecycle.
3. `TcpForwarder` uses Node's `net` module to listen locally and pipe traffic to the target socket in both directions.
4. `UdpForwarder` uses Node's `dgram` module for one-way, `bidirectional-last-client`, and `bidirectional-multi-client` forwarding.
5. `api.ts` exposes the REST API with Express.

The management UI/API defaults to `127.0.0.1:47831`. CLI flags (`--host`, `--port`) override environment variables (`PORTIER_HOST`, `PORTIER_PORT`), and environment variables override those defaults. The default is intentionally local-only; `0.0.0.0` for management is treated as a danger advisory because it exposes the UI/API on the LAN.

## Server Runtimes

Portier has two supported server runtimes:

- `service/` is the native Go service implementation. It is the preferred runtime for production deployment: smaller binary, no Node.js dependency, no warm-up time. The Go service implements the full API contract including TCP forwarding, UDP forwarding (all three modes), activity log, config import/export, rule CRUD, and static client serving.

- `server/` is the TypeScript server implementation. It remains supported as the reference implementation and fallback runtime. Both runtimes expose the same API contract.

The Go service static directory defaults to `web` relative to the working directory. Override with `--static-dir <path>` or `PORTIER_STATIC_DIR`. The TypeScript server defaults to `client/build` and also accepts `--static-dir` / `PORTIER_STATIC_DIR`.

## Package Layout

The production/install layout for all platforms:

```text
<install-dir>/
  service          (or service.exe on Windows)
  server.js        (Node fallback — requires Node.js)
  web/
    index.html
    assets/
```

Development build output (repo-internal, not distributed):

```text
service/build/portier-service
server/build/
client/build/
```

`rules.json` is always external — never packaged into binaries or package archives.

## Status Tracking

Each forwarder reports:

- `running`
- `bytesIn`
- `bytesOut`
- TCP `activeConnections`
- UDP `packetsIn` and `packetsOut`
- `lastError`
- `startedAt`

Status is intentionally lightweight and resets when a forwarder is recreated.

## Persistence

Rules are stored as JSON in `data/forwards.json` by default. The server validates all loaded rules before accepting them.

## Duplicate Bindings

The manager rejects duplicate rules with the same:

- protocol
- listen host
- listen port

TCP and UDP may use the same port number because they are separate protocols.

## Port Advisories

Shared code defines the recommended forwarding listen range, `48000-48999`, plus a data-driven registry of common system, network, database, development, and debug ports.

Forward rules must use valid ports from `1-65535`, and duplicate protocol/host/port listen bindings are rejected. Common ports are warned about but not blocked because legitimate forwards may intentionally target those ports.

The advisory utility reports:

- common port usage
- privileged ports below `1024`
- forwarding ports outside `48000-48999`
- forwarding LAN exposure on `0.0.0.0`
- management LAN exposure on `0.0.0.0`

Example safe forwarding rules:

- TCP `0.0.0.0:48001` -> `127.0.0.1:3000`
- UDP `0.0.0.0:48002` -> `127.0.0.1:4100`

## Activity Log

`ActivityStore` maintains an in-memory bounded list of activity events (max 500, newest first). In the TypeScript server it is created in `index.ts` and injected into both `ForwardManager` and the API app. The Go service has a matching `activity.Store` created in `main.go` and injected into the manager via `SetActivityStore`.

`ForwardManager` records rule lifecycle events (`rule.created`, `rule.updated`, `rule.deleted`, `rule.started`, `rule.stopped`, `rule.error`) directly.

`TcpForwarder` and `UdpForwarder` receive an optional `onEvent` callback from `ForwardManager`. The callback is `activityStore.add.bind(activityStore)`.

- `TcpForwarder` records `tcp.connection.opened`, `tcp.connection.closed`, and `tcp.connection.error` per connection.
- `UdpForwarder` records `udp.packet.forwarded`, `udp.packet.returned`, and `udp.packet.error`. Packet events are throttled to at most one log entry per second per rule to avoid flooding.

The `GET /api/activity` endpoint queries the store and returns events in newest-first order, with optional filters (`limit`, `ruleId`, `type`, `severity`).

Activity is in-memory only. Restarting the server clears all stored events.

## UDP Modes

The `UdpForwarder` supports three modes, controlled by `rule.udpMode`:

- **one-way**: incoming packets forwarded to target; no response path.
- **bidirectional-last-client**: a single shared target socket; responses go back to the most recently seen client. Not suitable for concurrent clients.
- **bidirectional-multi-client**: a `Map<sessionKey, UdpSession>` per forwarder. Each unique source address/port gets its own target socket and response path. Sessions expire after 60 s of idle (configurable in the constructor for tests).

Session state includes the target socket and an idle timer. On `stop()`, all sessions are closed synchronously before the listen socket is destroyed.

## Config Import/Export

`ForwardManager.exportConfig()` serializes current rules to an `ExportedConfig` object (version, timestamp, rules array).

`ForwardManager.importConfig(config, mode)` validates all rules atomically before applying:
- Replace mode: `stopAll()`, clear map, apply rules, persist, restart enabled.
- Merge mode: validate all rules, check for listen binding conflicts, add non-conflicting rules.

No partial import — if any rule fails validation, the whole operation is rejected.

## Rule Ordering

`ForwardManager.reorderRules(ids)` rebuilds the internal `Map` in the specified order by re-inserting entries. Rules not in the list are appended at the end. Persists via `store.save()`. Does not restart any running rule.

The `POST /api/forwards/reorder` endpoint accepts `{ ids: string[] }` and returns the updated rule list.

## Platform Service Support

Portier ships with service/daemon integration examples for each supported platform. Runtime config stays external in all modes.

| Platform | Mechanism | Scripts | Docs |
|---|---|---|---|
| Linux | systemd unit | `scripts/linux/` | `deploy/linux/readme.md` |
| Windows | Windows service (`sc.exe`) | `scripts/windows/` | `deploy/windows/readme.md` |
| macOS | user-level LaunchAgent | `scripts/macos/` | `deploy/macos/readme.md` |

The macOS LaunchAgent runs under the logged-in user (`gui/<uid>`) without `sudo`. The plist is generated by `scripts/macos/service/install-launch-agent.sh` with absolute paths — launchd does not expand `~` in plist values. Config lives at `~/Library/Application Support/Portier/rules.json`. Logs go to `~/Library/Logs/Portier/`.

Each platform supports the Go service as the preferred runtime and the Node server as a supported fallback. Only one should run on a given host/port at a time.

### Windows Service Control Manager Integration

The Go service (`service.exe`) uses `golang.org/x/sys/windows/svc` to natively register with the Windows Service Control Manager. The `service/sources/platform/` package provides a cross-platform abstraction:

- `service_windows.go` — calls `svc.IsWindowsService()` to auto-detect SCM mode; calls `svc.Run()` to register and handle Start/Stop/Shutdown/Interrogate control codes.
- `service_other.go` — stubs for Linux and macOS (always returns `false` for `IsWindowsService`).

When started by SCM, `service.exe` reports `StartPending` → starts the HTTP/forwarding runtime → reports `Running`. On Stop/Shutdown, it cancels the runtime context, waits for shutdown (up to 10 s), and reports `StopPending`/`Stopped`.

When started from a terminal (not SCM), the process uses OS signal handling (Ctrl+C / SIGTERM) regardless of the `--service` flag. This lets you test the binary manually with the same arguments the installer uses.

## Production vs Development

In **production**, the server serves both the REST API and the built client from a single process:
- API routes are under `/api`.
- Non-API routes return `index.html` from the static directory for SPA routing.
- Static assets (JS, CSS) are served directly from the static directory.
- If the static directory is missing or has no `index.html`, the API continues to work and the server logs a warning.

The Go service static directory defaults to `web` relative to the working directory. Override with `--static-dir <path>` or `PORTIER_STATIC_DIR`. The TypeScript server defaults to `client/build`.

In **development**, the server handles the API and the Vite dev server handles the client. Vite proxies `/api` to the server, so client code uses relative `/api` paths in both modes.

## Client Flow

The client uses relative `/api` paths (`/api/forwards`, `/api/status`, etc.) for all requests. In development, Vite proxies these to the server. In production, the server handles them directly since both the API and the UI run on the same origin. This means the client bundle does not embed the host or port.

The app shell is split into focused components:
- **`TopHeader`**: brand, subtitle, Settings and API Docs shortcuts, mobile hamburger button.
- **`Sidebar`**: nav buttons for all five views, running status footer.
- **`RuleSummaryCards`**: reusable 4-card summary (Total/Running/Stopped/Error) used by the Dashboard, Rules, and Activity views.

The sidebar has five functional views:
- **Dashboard** (`view = "dashboard"`): stat cards, top rules by traffic, recent activity.
- **Forward Rules** (`view = "rules"`): stat cards, rules table with drag-to-reorder.
- **Activity** (`view = "activity"`): the activity log viewer with severity filter, limit selector, and auto-refresh.
- **Settings** (`view = "settings"`): management endpoint info, port range, config export/import.
- **API Docs** (`view = "api-docs"`): static list of all REST endpoints.

On mobile (≤700px), the sidebar is hidden and a hamburger button in the header opens it as an overlay.
