# Portier

Portier is a local TCP/UDP port forwarding manager with a native Go service runtime, a TypeScript server runtime, and a simple React web UI.

## Install

```powershell
npm install
```

## Run

### Development Mode

Run the server and client together:

```powershell
npm run start:dev
```

Run them separately:

```powershell
npm run dev -w server
npm run dev -w client
```

The server listens on `http://127.0.0.1:47831` by default. The client runs at `http://127.0.0.1:5173` and proxies `/api` requests to the server.

Rules are stored in `data/forwards.json` by default. Set `PORTIER_CONFIG` to use another path.

### Repository Build Runtime

Build all packages:

```powershell
npm run build
```

Start the built TypeScript server from the repository (serves `client/build` on the same origin as the API):

```powershell
npm run start:server
```

Or run it directly with explicit options:

```powershell
$env:NODE_ENV = "production"
node server/build/index.js --service --static-dir client/build
```

This is a repository smoke-test/runtime command. Production packages use the flat install layout (`service` or `service.exe`, `server.js`, and `web/`). API routes stay under `/api`; non-API routes fall back to `index.html`. Use `--static-dir` or `PORTIER_STATIC_DIR` to point either runtime at a different web UI directory.

### Native Go Service

The `service/` directory contains the native Go service implementation. It is the preferred runtime for production deployment: smaller binary, no Node.js dependency, no warm-up time. The TypeScript server in `server/` remains supported as a reference implementation and fallback runtime. Both runtimes implement the same API contract.

Build the Go service from the repository root:

```powershell
npm run build:service
```

The output is:

```text
service/build/portier-service
```

Run it against the dev client build:

```powershell
npm run start:service
```

### Linux Release Archive (v1.1)

Build a portable tar.gz for distribution:

```bash
npm run installer:linux
```

Output: `build/releases/linux/portier-<version>-linux.tar.gz`

The archive contains the clean runtime layout (`service`, `server.js`, `web/`, `readme.txt`). No signing required for Linux tar.gz archives. See `deploy/systemd/readme.md`.

### Linux systemd Service

Build the package and install in one step:

```bash
npm run package:portier
sudo bash scripts/linux/install-service.sh
```

The install script auto-copies `build/portier/` into `/opt/portier/`, creates `/etc/portier/rules.json` if missing, generates `/etc/systemd/system/portier.service`, enables and starts the service.

Node fallback (requires Node.js on the target machine):

```bash
sudo bash scripts/linux/install-service.sh --runtime node
```

The Go service ExecStart:

```text
/opt/portier/service --service --config /etc/portier/rules.json --host 127.0.0.1 --port 47831 --static-dir /opt/portier/web
```

Open `http://127.0.0.1:47831` after installation.

```bash
sudo bash scripts/linux/status-service.sh
sudo bash scripts/linux/stop-service.sh
sudo bash scripts/linux/start-service.sh
sudo bash scripts/linux/uninstall-service.sh   # preserves rules.json
```

See `deploy/systemd/readme.md` for flags, manual unit file install, and firewall notes.

### macOS Release Archive (v1.1)

Build a portable tar.gz for distribution:

```bash
npm run installer:macos
```

Output: `build/releases/macos/portier-portable-macos-<version>.tar.gz`

The archive contains the clean runtime layout (`service`, `server.js`, `web/`, `readme.txt`). Unsigned — macOS Gatekeeper may quarantine downloaded binaries; use `xattr -cr` to clear. Sign with Developer ID for public distribution. See `deploy/macos/readme.md` for signing notes.

### macOS LaunchAgent

Build the package and install:

```bash
npm run package:portier
bash scripts/macos/install-launch-agent.sh
```

The install script auto-copies `build/portier/` to `~/Applications/Portier/`, creates `~/Library/Application Support/Portier/rules.json` if missing, generates `~/Library/LaunchAgents/com.portier.port-forwarding.plist` with absolute paths, and bootstraps the agent for the current user. No `sudo` is required.

Open `http://127.0.0.1:47831` after installation. Logs are written to `~/Library/Logs/Portier/`.

```bash
bash scripts/macos/status-launch-agent.sh
bash scripts/macos/stop-launch-agent.sh
bash scripts/macos/start-launch-agent.sh
bash scripts/macos/uninstall-launch-agent.sh   # preserves rules.json and logs
```

Use `--runtime node` to run with `server.js` instead of the native binary. Use `--purge` on uninstall to also remove config and logs. See `deploy/macos/readme.md` for full options.

Forwarded ports on `0.0.0.0` may trigger macOS Firewall prompts. The management UI stays on `127.0.0.1:47831` and is not LAN-visible by default.

### Windows Installer (v1.1)

Portier v1.1 adds an [Inno Setup](https://jrsoftware.org/isinfo.php) installer for machine-wide installation on Windows 10+ (64-bit).

**Build the installer** (requires Inno Setup 6 and Go):

```powershell
npm run installer:windows
```

Output: `build/releases/windows/Portier-Setup-<version>.exe`

The installer:
- Installs binaries to `%ProgramFiles%\Portier\`.
- Creates config directory at `%ProgramData%\Portier\` and writes an empty `rules.json` if absent.
- Offers an optional **Windows Service** task (checked by default): registers `Portier` as a Windows Service that auto-starts at boot.
- On uninstall: stops and removes the service, removes `logs\`, and preserves `rules.json`.
- Does not create Windows Firewall rules.

The installer is unsigned. Windows SmartScreen may warn before running it. Sign with an EV certificate for public distribution. See `deploy/windows/readme.md` for full details and build options.

---

### Windows Executable and Service

Build the Windows package:

```powershell
npm run package:windows
```

The output is created under `build\windows`:

```text
build\windows\
  service.exe
  server.js
  web\
    index.html
    assets\
  readme.txt
```

Run the Go service manually for testing:

```powershell
.\build\windows\service.exe --config ".\rules.json" --host 127.0.0.1 --port 47831 --static-dir ".\build\windows\web"
```

Machine install (Administrator required — installs to `%ProgramFiles%\Portier`):

```powershell
Copy-Item -Recurse -Force .\build\windows\* "$env:ProgramFiles\Portier\"
powershell -ExecutionPolicy Bypass -File .\scripts\windows\install-service.ps1
```

User install (no Administrator — installs to `%LOCALAPPDATA%\Portier`):

```powershell
Copy-Item -Recurse -Force .\build\windows\* "$env:LOCALAPPDATA\Portier\"
powershell -ExecutionPolicy Bypass -File .\scripts\windows\install-service.ps1 -Scope User
```

Config for machine install: `%ProgramData%\Portier\rules.json`. Config for user install: `%APPDATA%\Portier\rules.json`. Use `-UseNode` to run `node server.js` instead of the Go binary (requires Node.js).

```powershell
.\scripts\windows\status-service.ps1 [-Scope User]
.\scripts\windows\stop-service.ps1   [-Scope User]
.\scripts\windows\start-service.ps1  [-Scope User]
.\scripts\windows\uninstall-service.ps1 [-Scope User]   # preserves rules.json
```

See `deploy/windows/readme.md` for detailed Windows packaging and service notes.
See `deploy/macos/readme.md` for detailed macOS LaunchAgent notes.
See `deploy/systemd/readme.md` for detailed Linux systemd notes.

## Package Layout

The runtime package layout for all platforms:

```text
<install-dir>/
  service          (or service.exe on Windows)
  server.js        (Node fallback — requires Node.js)
  web/
    index.html
    assets/
```

- `service` / `service.exe` — native Go service binary
- `server.js` — bundled Node/TypeScript fallback
- `web/` — built React client UI
- `rules.json` remains external and is never packaged

Development build output (repo-internal, not distributed):

```text
service/build/portier-service
server/build/
client/build/
```

## Management UI

Open `http://127.0.0.1:47831` in a browser to use the management interface. It has five views:

- **Dashboard** — stat cards (total/running/stopped/error, TCP/UDP counts), top rules by traffic, recent activity, quick actions.
- **Forward Rules** — rules table with search, status filter, auto-refresh, Move Up/Down ordering, and an Add/Edit/Delete drawer.
- **Activity** — in-memory activity log with severity filter, limit selector, and auto-refresh. Resets on server restart.
- **Settings** — management endpoint info, recommended port range, config export (download JSON) and import (merge or replace, with validation preview).
- **API Docs** — client-side reference page listing all REST endpoints.

The sidebar is accessible on mobile via a hamburger button in the header. The management UI only binds to `127.0.0.1` by default and is not reachable from the LAN.

## Recommended Forwarding Ports

Portier recommends forwarding listen ports in the `48000-48999` range. This keeps forwards away from common system, database, and development ports while staying easy to remember.

Common ports are warned about, not blocked. A rule on `5173`, `8080`, or `5432` might be intentional, but it is also easy to collide with Vite, an alternate HTTP server, or PostgreSQL. Invalid ports outside `1-65535` are blocked.

Using `0.0.0.0` as a forwarding `listenHost` exposes the forwarded port on all available network interfaces, including LAN interfaces. Use `127.0.0.1` when the forward should only be available on the local machine.

## Example TCP Rule

```json
{
  "name": "Local web app",
  "protocol": "tcp",
  "listenHost": "0.0.0.0",
  "listenPort": 48001,
  "targetHost": "127.0.0.1",
  "targetPort": 3000,
  "enabled": true
}
```

This listens on `0.0.0.0:48001` and forwards TCP traffic to `127.0.0.1:3000`.

## Example UDP Rule

```json
{
  "name": "UDP metrics",
  "protocol": "udp",
  "listenHost": "0.0.0.0",
  "listenPort": 48002,
  "targetHost": "127.0.0.1",
  "targetPort": 4100,
  "enabled": true,
  "udpMode": "one-way"
}
```

This forwards UDP packets received on `0.0.0.0:48002` to `127.0.0.1:4100`.

## LAN Exposure Warning

Forwarded listen ports are separate from the management UI/API bind address. A LAN-visible forwarding rule can be useful, but a LAN-visible management UI/API is high risk. Keep management on `127.0.0.1:47831` unless you deliberately need remote administration and have secured that access.

## Windows Firewall Note

Forwarded listen ports are separate from the management UI/API bind address. If a forward rule listens on a LAN-visible address such as `0.0.0.0`, the operating system firewall must allow inbound traffic to that forwarded port. On Windows, this may trigger Windows Firewall prompts or require an inbound firewall rule. If a rule appears to start but another machine cannot connect, check Windows Firewall and any endpoint security software.

## UDP Modes

- **one-way**: packets are forwarded from client to target only. No response is sent back.
- **bidirectional-last-client**: target responses are forwarded back to the most recent UDP client. Concurrent clients may receive incorrect or missing responses.
- **bidirectional-multi-client**: each source address/port gets its own target socket. Responses are routed back to the correct client. Sessions expire after 60 seconds of idle.

## Testing

### Unit and Integration Tests

```powershell
npm run test
```

Runs shared, server, client, and Go service tests. This requires the Go toolchain. If Go is unavailable, run the TypeScript suites individually with `npm run test:shared`, `npm run test:server`, and `npm run test:client`.

### Playwright E2E Tests

E2E tests run Chromium against the TypeScript server serving the built React client.

**Prerequisites:**

1. Install Playwright browsers (one-time):

```powershell
npm run test:e2e:install
```

2. Build the React client (required for each client change):

```powershell
npm run build:client
```

**Run E2E tests:**

```powershell
npm run test:e2e           # headless Chromium
npm run test:e2e:headed    # visible browser window
npm run test:e2e:debug     # Playwright Inspector
npm run test:e2e:fresh     # build:client then run tests
```

The E2E server starts on `127.0.0.1:47890` (distinct from the dev server at `47831`).
Test data is isolated via a temp config in `test-results/` and reset before each test.
Artifacts (screenshots, traces, videos) are written to `test-results/` only on failure.

**What E2E covers:**

*UI flows (`tests/e2e/portier.spec.ts`):*
- App load: shell, navigation, header, empty state
- Add Rule: drawer opens, form fills, rule appears in table
- Edit Rule: drawer pre-fills, changes save, list updates
- Start/Stop: status transitions between Running and Stopped
- Delete: confirmation required, rule removed
- Activity: view opens and events are recorded
- Settings: config import (file upload, merge mode, success)
- API Docs: endpoint list is visible
- Mobile sidebar: hamburger opens sidebar, navigation closes it
- Dashboard: stat cards render

*Protocol forwarding (`tests/e2e/tcp.spec.ts`, `tests/e2e/udp.spec.ts`):*
- TCP real forwarding: data passes end-to-end through a live forwarder to an in-process echo server
- UDP one-way: packet delivered to receiver with no response path
- UDP bidirectional-last-client: echo returned to the sender's source port
- UDP bidirectional-multi-client: two concurrent clients each receive their own echo (no cross-contamination)
- Activity log: TCP connection events and UDP forwarding events recorded and verifiable

**OS service install validation (explicit release commands, not run automatically):**

```powershell
npm run validate:service:current              # current OS, user-scope (no admin on Windows)
npm run validate:service:windows:user         # Windows scheduled task (no admin required)
npm run validate:service:windows:machine      # Windows Service (Administrator required)
```

```bash
npm run validate:service:macos    # macOS LaunchAgent (no sudo required)
npm run validate:service:linux    # Linux systemd (requires sudo)
```

These scripts use test-specific service names, ports, and temp directories. They never touch production Portier installs or config.

**Manual QA still required:**

- Firewall and OS permission behavior (Windows Firewall prompts, macOS firewall dialogs, Linux firewall rules)

**Automated (not manual):**

- Package build and layout: `npm run validate:package:smoke`
- OS service install/start/stop/uninstall: `npm run validate:service:*`

## Scripts

```powershell
npm run start:server
npm run start:dev
npm run build
npm run build:server
npm run build:service
npm run build:client
npm run start:service
npm run test
npm run test:e2e
npm run test:e2e:fresh
npm run lint
npm run typecheck
npm run check
npm run package:portier
npm run package:windows
npm run package:macos
npm run package:linux
npm run package:clean
npm run validate:package           # validate existing build/portier/ layout
npm run validate:package:build     # build then validate
npm run validate:package:smoke     # build, validate, and run smoke test
npm run validate:service:current          # OS service install validation for current platform
npm run validate:service:windows:user     # Windows user-scope (scheduled task, no admin)
npm run validate:service:windows:machine  # Windows machine-scope (Windows Service, admin required)
npm run validate:service:macos            # macOS LaunchAgent (no sudo)
npm run validate:service:linux            # Linux systemd (requires sudo)
npm run installer:windows                 # build Inno Setup installer (requires Inno Setup 6)
npm run installer:windows:no-package      # build installer only, skip package step
npm run installer:macos                   # build macOS portable tar.gz
npm run installer:macos:no-package        # macOS tar.gz only, skip package step
npm run installer:linux                   # build Linux portable tar.gz
npm run installer:linux:no-package        # Linux tar.gz only, skip package step
```

macOS LaunchAgent scripts (run on macOS):

```bash
bash scripts/macos/build-package.sh
bash scripts/macos/install-launch-agent.sh [--node-mode] [--install-dir PATH] [--config-path PATH]
bash scripts/macos/status-launch-agent.sh
bash scripts/macos/start-launch-agent.sh
bash scripts/macos/stop-launch-agent.sh
bash scripts/macos/uninstall-launch-agent.sh
```

Linux service scripts (run as root on Linux):

```bash
bash scripts/linux/build-release.sh            # portable tar.gz → build/releases/linux/
sudo bash scripts/linux/install-service.sh [--runtime node] [--source-dir PATH] [--install-dir PATH] [--config-path PATH] [--no-enable]
sudo bash scripts/linux/status-service.sh
sudo bash scripts/linux/start-service.sh
sudo bash scripts/linux/stop-service.sh
sudo bash scripts/linux/uninstall-service.sh [--remove-files] [--remove-config]
```

## Release

Current version: **1.0.0**

- [docs/changelog.md](docs/changelog.md) — what changed in each release.
- [docs/installer-strategy.md](docs/installer-strategy.md) — v1.1 installer and distribution strategy.

## Agent Workflow

`AGENTS.md` contains lightweight project guidance for Codex and other coding agents. `CLAUDE.md` contains Claude Code guidance for architecture review, UI cleanup, and risk checks. Use `npm run check` to run lint, typecheck, and tests for an agent task.

See `docs/agent-workflow.md` for guidance on using Codex and Claude together. See `docs/claude-code.md` for Claude Code settings, manual helper hooks, and audit skill usage. Claude UI cleanup should start with `docs/client.md` and `docs/api-contract.md`.

## Activity Log

Portier records recent forwarding and lifecycle events in an in-memory bounded activity log. The log is visible in the UI under the **Activity** sidebar item.

Events include:
- Rule created, updated, deleted, started, stopped, and errors
- TCP connection opened, closed, and errors
- UDP packet forwarded, returned (bidirectional mode), and errors

**Limitations:**
- The activity log is in-memory only. It resets when the server restarts.
- The store is bounded to the latest 500 events.
- UDP packet events are throttled to at most one log entry per second per rule.

## REST API

- `GET /api/forwards`
- `POST /api/forwards`
- `PATCH /api/forwards/:id`
- `DELETE /api/forwards/:id`
- `POST /api/forwards/:id/start`
- `POST /api/forwards/:id/stop`
- `GET /api/ports/advisory?port=48001&listenHost=0.0.0.0&purpose=forward`
- `GET /api/status`
- `GET /api/activity?limit=100&severity=error` (optional: ruleId, type, severity)
- `GET /api/config/export`
- `POST /api/config/import` — body: `{ mode, config }`
- `POST /api/forwards/reorder` — body: `{ ids: string[] }`
