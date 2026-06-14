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
npm run build:release:portable
```

Output: `build/releases/linux/portier-<version>-linux.tar.gz`

The archive contains the clean runtime layout (`service`, `server.js`, `web/`, `readme.txt`). No signing required for Linux tar.gz archives. See `scripts/linux/readme.md`.

### Linux systemd Service

Build the package and install in one step:

```bash
npm run build:runtime
sudo bash scripts/linux/service/install-service.sh
```

The install script auto-copies `build/portier/` into `/opt/portier/`, creates `/etc/portier/rules.json` if missing, generates `/etc/systemd/system/portier.service`, enables and starts the service.

Node fallback (requires Node.js on the target machine):

```bash
sudo bash scripts/linux/service/install-service.sh --runtime node
```

The Go service ExecStart:

```text
/opt/portier/service --service --config /etc/portier/rules.json --host 127.0.0.1 --port 47831 --static-dir /opt/portier/web
```

Open `http://127.0.0.1:47831` after installation.

```bash
sudo bash scripts/linux/service/status-service.sh
sudo bash scripts/linux/service/stop-service.sh
sudo bash scripts/linux/service/start-service.sh
sudo bash scripts/linux/service/uninstall-service.sh   # preserves rules.json
```

See `scripts/linux/readme.md` for flags, manual unit file install, and firewall notes.

### macOS Release Archive (v1.1)

Build a portable tar.gz for distribution:

```bash
npm run build:release:portable
```

Output: `build/releases/macos/portier-portable-macos-<version>.tar.gz`

The archive contains the clean runtime layout (`service`, `server.js`, `web/`, `readme.txt`). Unsigned — macOS Gatekeeper may quarantine downloaded binaries; use `xattr -cr` to clear. Sign with Developer ID for public distribution. See `scripts/macos/readme.md` for signing notes.

### macOS LaunchAgent

Build the package and install:

```bash
npm run build:runtime
bash scripts/macos/service/install-launch-agent.sh
```

The install script auto-copies `build/portier/` to `~/Applications/Portier/`, creates `~/Library/Application Support/Portier/rules.json` if missing, generates `~/Library/LaunchAgents/com.portier.port-forwarding.plist` with absolute paths, and bootstraps the agent for the current user. No `sudo` is required.

Open `http://127.0.0.1:47831` after installation. Logs are written to `~/Library/Logs/Portier/`.

```bash
bash scripts/macos/service/status-launch-agent.sh
bash scripts/macos/service/stop-launch-agent.sh
bash scripts/macos/service/start-launch-agent.sh
bash scripts/macos/service/uninstall-launch-agent.sh   # preserves rules.json and logs
```

Use `--runtime node` to run with `server.js` instead of the native binary. Use `--purge` on uninstall to also remove config and logs. See `scripts/macos/readme.md` for full options.

Forwarded ports on `0.0.0.0` may trigger macOS Firewall prompts. The management UI stays on `127.0.0.1:47831` and is not LAN-visible by default.

### Windows Installer (v1.1)

Portier v1.1 adds an [Inno Setup](https://jrsoftware.org/isinfo.php) installer for machine-wide installation on Windows 10+ (64-bit).

**Build the release artifacts** (requires Inno Setup 6 for the installer):

```powershell
npm run build:release:current
```

Output: `build/releases/windows/portier-<version>-windows-portable.zip` and `Portier-Setup-<version>.exe` (if Inno Setup is available)

The installer:
- Installs binaries to `%ProgramFiles%\Portier\`.
- Creates config directory at `%ProgramData%\Portier\` and writes an empty `rules.json` if absent.
- Offers an optional **Windows Service** task (checked by default): registers `Portier` as a Windows Service that auto-starts at boot.
- On uninstall: stops and removes the service, removes `logs\`, and preserves `rules.json`.
- Does not create Windows Firewall rules.

The installer is unsigned. Windows SmartScreen may warn before running it. Sign with an EV certificate for public distribution. See `scripts/windows/readme.md` for full details and build options.

---

### Windows Executable and Service

Build the Windows package:

```powershell
npm run build:runtime:windows
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
powershell -ExecutionPolicy Bypass -File .\scripts\windows\service\install-service.ps1
```

User install (no Administrator — installs to `%LOCALAPPDATA%\Portier`):

```powershell
Copy-Item -Recurse -Force .\build\windows\* "$env:LOCALAPPDATA\Portier\"
powershell -ExecutionPolicy Bypass -File .\scripts\windows\service\install-service.ps1 -Scope User
```

Config for machine install: `%ProgramData%\Portier\rules.json`. Config for user install: `%APPDATA%\Portier\rules.json`. Use `-UseNode` to run `node server.js` instead of the Go binary (requires Node.js).

```powershell
.\scripts\windows\service\status-service.ps1 [-Scope User]
.\scripts\windows\service\stop-service.ps1   [-Scope User]
.\scripts\windows\service\start-service.ps1  [-Scope User]
.\scripts\windows\service\uninstall-service.ps1 [-Scope User]   # preserves rules.json
```

See `scripts/windows/readme.md` for detailed Windows packaging and service notes.
See `scripts/macos/readme.md` for detailed macOS LaunchAgent notes.
See `scripts/linux/readme.md` for detailed Linux systemd notes.

### Release Artifact Generation (v1.1)

Build the current platform's portable archive and installer (if tooling is available):

```powershell
npm run build:release:current
```

Portable archive only (skip installer):

```powershell
npm run build:release:portable
```

Validate the artifacts after building:

```powershell
npm run validate:release:portable
npm run validate:release:current
```

**Output layout:**

```text
build/releases/
  windows/
    portier-<version>-windows-portable.zip   (from Compress-Archive)
    Portier-Setup-<version>.exe              (Inno Setup installer, if available)
  macos/
    portier-portable-macos-<version>.tar.gz
  linux/
    portier-<version>-linux.tar.gz
```

Each portable archive contains the clean runtime layout (`service`/`service.exe`, `server.js`, `web/`, `readme.txt`). Config (`rules.json`) is never bundled.

Service binaries are platform-native. For a multi-platform release, run `release:current` on each target OS. The Windows installer is skipped (non-fatal) when Inno Setup is unavailable — the portable zip is still produced.

macOS `.pkg` and Linux `.deb`/`.rpm` are out of v1.1 scope. See `docs/installer.md`.

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
- **Forward Rules** — rules table with search, status filter, auto-refresh, Move Up/Down ordering, Add/Edit/Delete drawer, Diagnose action (per-rule diagnostic checks), and View Activity shortcut (per rule).
- **Activity** — in-memory activity log with severity, type, and rule filters, export as JSON, clear log, and auto-refresh. Resets on server restart.
- **Settings** — runtime/environment info with copy buttons, config export (datetime-stamped JSON), config import (merge or replace, with backup prompt on replace), and Download Diagnostics JSON (local bundle, no upload).
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

*Settings import/export flows (`tests/e2e/settings.spec.ts`):*
- Replace-mode import using `v1-mixed.json` fixture: preview counts (4 rules, 1 TCP/3 UDP), confirm dialog, success message, all four rules visible in Forward Rules, pre-existing rule gone
- Invalid JSON import: client-side parse error alert, no preview or import button, existing rules untouched
- Export download: filename matches `portier-rules-YYYY-MM-DD.json`, `ExportedConfig` shape valid (`version`, `exportedAt`, `rules`), created rule present in export

Config fixtures reused from `tests/fixtures/config/`. E2E intentionally does not repeat the full fixture matrix — `validate:config` owns exhaustive TS/Go parity.

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

**Additional validation suites (explicit, not part of `npm run check`):**

```powershell
npm run validate:config            # config compatibility: fixture-based rules.json validation
npm run validate:contract          # API contract parity: TypeScript + Go service if binary present
npm run validate:binary            # runtime binary behavior: 5 behavioral tests against build/portier/
npm run validate:runtime:behavior  # alias for validate:binary (fits validate:runtime:* namespace)
npm run validate:scripts           # installer static analysis + dry-run on current platform
```

- `validate:config` — loads every fixture from `tests/fixtures/config/` and validates config compatibility: valid fixtures load and import correctly, invalid fixtures are rejected, duplicate bindings are caught, UDP mode defaults are applied, and both config shapes (raw array and Go-only wrapper) behave as documented. TypeScript runtime is always checked; Go runtime is checked when the binary is available. Pass `--skip-go` to force skip. No real `rules.json` is used.
- `validate:contract` — runs all API scenarios (CRUD, start/stop, activity, config export/import, port advisory, error shapes) against the TypeScript server; if Go binary is built, runs the same suite against it and compares results. Skips Go parity with a clear message if the binary is absent. Pass `--skip-go` to force skip.
- `validate:binary` — builds `build/portier/` then tests: health, static serving, missing-static-dir fallback, invalid-config exit, and clean shutdown. Pass `--no-build` to reuse an existing build.
- `validate:scripts` — static analysis of all platform install and validate scripts (no firewall commands, test names in validate scripts, production path defaults, path quoting); plus dry-run execution on the current platform.

Naming convention:
- `npm run test` = unit/integration test runner (Vitest + Go test)
- `npm run test:e2e` = Playwright browser E2E tests
- `npm run validate:config` = fixture-based rules.json compatibility validation
- `npm run validate:contract` = TS/Go API parity validation
- `npm run validate:binary` / `validate:runtime:behavior` = packaged binary behavioral validation
- `npm run validate:scripts` = installer/service script static + dry-run validation

**Manual QA still required:**

- Firewall and OS permission behavior (Windows Firewall prompts, macOS firewall dialogs, Linux firewall rules)

**Automated (not manual):**

- Package build and layout: `npm run validate:runtime:smoke`
- OS service install/start/stop/uninstall: `npm run validate:service:*`
- Config compatibility: `npm run validate:config`
- API contract parity: `npm run validate:contract`
- Runtime binary behavior: `npm run validate:binary`
- Installer script analysis: `npm run validate:scripts`

## Portier CLI (v1.3)

The `portier` CLI is a Go-based command-line tool for managing the local Portier service from the terminal or scripts. It talks to the existing management API. It is not a second runtime.

**Build:**

```powershell
npm run build:cli
```

Output: `tools/cli/build/portier-cli`. The runtime build (`npm run build:runtime`) also builds the CLI directly into `build/portier/portier[.exe]` as part of the runtime package.

**Commands (Slices 2–7):**

```
portier list              # list configured forwarding rules
portier status            # show rule runtime status
portier activity          # show recent activity events (--limit, --rule, --type, --severity)
portier start <id|name>   # start a rule (exact ID or unique name)
portier stop <id|name>    # stop a rule
portier diagnose <id|name># run diagnostics (pass/warn/fail/skip per check)
portier config validate <file>                    # validate a local config file (no API call)
portier config export --out <file>                # export current rules to a file
portier config import --mode merge|replace <file> # import rules (--yes required for replace)
portier diagnostics export --out <file>           # build a diagnostics support bundle
portier runtime           # show runtime info from GET /api/runtime
portier version           # show CLI version
portier help              # show help
```

**Global flags:**

```
--url string    Full management API URL (default: http://127.0.0.1:47831)
--host string   Management host
--port int      Management port
--json          Machine-readable JSON output
--version       Show CLI version
```

**Environment:** `PORTIER_URL` overrides the default URL.

**Exit codes:** `0` success · `1` API error · `2` invalid args · `3` connection failure

**Test:**

```powershell
npm run test:cli                   # go test ./... inside tools/cli
npm run validate:cli               # test:cli + build:cli
npm run validate:coverage:cli      # CLI coverage gate only (threshold: 92%, actual: 92.7%)
npm run validate:coverage          # all five components — coverage gates enforced
```

See [tools/cli/readme.md](tools/cli/readme.md) for full usage, exit codes, and planned commands.

### Replay tool (`portier-replay`)

`portier-replay` is a **separate, offline analysis tool beside the CLI** (under `tools/replay/`, its own Go module) — not a `portier` subcommand. It reads existing Portier workflow artifacts (run/plan reports, history exports, support-report bundles) and reports what offline analysis each saved artifact can support. It is strictly offline and read-only: it never executes workflows, contacts the runtime, reads referenced config/policy/baseline/report files, mutates inputs, or uploads anything.

```powershell
portier-replay [--json] plan --from <file-or-dir> [--out <file>]

npm run build:replay      # builds tools/replay/build/portier-replay
npm run test:replay       # go test ./... inside tools/replay
npm run validate:replay   # test:replay + build:replay
```

See [tools/replay/README.md](tools/replay/README.md) for details and the safety boundary.

---

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
npm run build:runtime
npm run build:runtime:windows
npm run build:runtime:macos
npm run build:runtime:linux
npm run build:clean
npm run validate:runtime           # validate existing build/portier/ layout
npm run validate:runtime:build     # build then validate
npm run validate:runtime:smoke     # build, validate, and run smoke test
npm run validate:service:current          # OS service install validation for current platform
npm run validate:service:windows:user     # Windows user-scope (scheduled task, no admin)
npm run validate:service:windows:machine  # Windows machine-scope (Windows Service, admin required)
npm run validate:service:macos            # macOS LaunchAgent (no sudo)
npm run validate:service:linux            # Linux systemd (requires sudo)
npm run build:release:current             # portable archive + installer for current platform
npm run build:release:portable            # portable archive only (skip installer)
npm run validate:release:current          # validate release artifacts for current platform
npm run validate:release:portable         # validate portable archive only
npm run build:cli                         # build CLI binary into tools/cli/build/portier[.exe]
npm run test:cli                          # go test ./... inside tools/cli
npm run validate:cli                      # test:cli + build:cli
npm run validate:coverage:cli             # CLI coverage gate only (threshold: 92%)
npm run validate:coverage                 # all five components — coverage gates enforced
```

macOS LaunchAgent scripts (run on macOS):

```bash
bash scripts/macos/build-runtime.sh
bash scripts/macos/service/install-launch-agent.sh [--node-mode] [--install-dir PATH] [--config-path PATH]
bash scripts/macos/service/status-launch-agent.sh
bash scripts/macos/service/start-launch-agent.sh
bash scripts/macos/service/stop-launch-agent.sh
bash scripts/macos/service/uninstall-launch-agent.sh
```

Linux service scripts (run as root on Linux):

```bash
bash scripts/linux/release/build-release.sh    # portable tar.gz → build/releases/linux/
sudo bash scripts/linux/service/install-service.sh [--runtime node] [--source-dir PATH] [--install-dir PATH] [--config-path PATH] [--no-enable]
sudo bash scripts/linux/service/status-service.sh
sudo bash scripts/linux/service/start-service.sh
sudo bash scripts/linux/service/stop-service.sh
sudo bash scripts/linux/service/uninstall-service.sh [--remove-files] [--remove-config]
```

## Release

Current version: **1.4.0**

- [docs/changelog.md](docs/changelog.md) — what changed in each release.
- [docs/installer.md](docs/installer.md) — v1.1 installer and distribution strategy.
- [docs/roadmap.md](docs/roadmap.md) — v1.4 delivered the Live Connection Inspector (`GET /api/connections`, TCP + UDP tracking in both runtimes, Live Connections UI, 116/116 contract checks); v1.5 plans Declarative Config & Drift Control (plan/diff/apply workflows); v1.6 plans a dedicated Architecture, Quality & Maintainability Audit after v1.4/v1.5 coverage work creates the safety net.

## Agent Workflow

`AGENTS.md` contains lightweight project guidance for Codex and other coding agents. `CLAUDE.md` contains Claude Code guidance for architecture review, UI cleanup, and risk checks. Use `npm run check` to run lint, typecheck, and tests for an agent task.

See `docs/agentic.md` for agent setup: Claude Code settings, manual helper hooks, and audit skill usage.

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
- `POST /api/forwards/:id/diagnose` — diagnostic checks without mutating state (v1.2)
- `GET /api/ports/advisory?port=48001&listenHost=0.0.0.0&purpose=forward`
- `GET /api/status`
- `GET /api/runtime` — runtime environment info (v1.2)
- `GET /api/activity?limit=100&severity=error` (optional: ruleId, type, severity)
- `DELETE /api/activity` — clear the activity log (v1.2)
- `GET /api/config/export`
- `POST /api/config/import` — body: `{ mode, config }`
- `POST /api/forwards/reorder` — body: `{ ids: string[] }`
