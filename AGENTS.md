# Portier Agent Guide

## Project

- App name: Portier
- Repository name: portier-port-forwarding
- Purpose: local TCP/UDP port forwarding manager for development and LAN testing

## Structure

- `server/sources` = Node.js TypeScript service, REST API, TCP/UDP forwarding engine
- `service/sources` = native Go service implementation for smaller binaries and service deployment
- `client/sources` = React TypeScript web UI
- `shared/sources` = shared types, validation, constants, port advisory utilities
- `scripts` = executable automation scripts; platform subdirs contain `release/` (artifact builders), `service/` (OS service lifecycle), and platform-specific docs/templates
- `build` = generated build output

## Server Runtimes

- `service/` is the native Go service and the preferred production runtime. Default static dir: `web`.
- `server/` is the TypeScript server and remains supported as reference/fallback. Default static dir: `client/build`.
- Both implement the same REST API contract.

## Packaged Runtime Layout

```text
<install-dir>/
  service          (or service.exe on Windows)
  server.js        (Node fallback — requires Node.js)
  web/
    index.html
    assets/
```

Development build output (not distributed):

```text
service/build/portier-service
server/build/
client/build/
```

## Setup Commands

```powershell
npm install
```

## Development Commands

```powershell
npm run start:dev
npm run dev -w server
npm run dev -w client
```

## Validation Commands

```powershell
npm run lint
npm run typecheck
npm run test
npm run build
npm run check
```

## E2E Tests

Playwright E2E tests live in `tests/e2e/`. They run against the TypeScript server serving the built React client.

**Prerequisites (one-time):**
```powershell
npm run test:e2e:install   # installs Playwright Chromium browser
npm run build:client       # must be built before running E2E
```

**Run:**
```powershell
npm run test:e2e           # headless
npm run test:e2e:headed    # visible browser
npm run test:e2e:fresh     # build:client then run
```

E2E server binds to `127.0.0.1:47890`. Do not include `test:e2e` in `npm run test` or `npm run check` — E2E is a separate step.

Files:
- `playwright.config.ts` — webServer, browser, reporter config
- `tests/e2e/portier.spec.ts` — UI flow tests
- `tests/e2e/tcp.spec.ts` — TCP real forwarding E2E
- `tests/e2e/udp.spec.ts` — UDP one-way, last-client, multi-client E2E; activity assertions
- `tests/e2e/helpers/port.ts` — `getFreePort`, `getFreeTcpPort`, `getFreeUdpPort`
- `tests/e2e/helpers/network.ts` — TCP/UDP echo servers, receivers, clients
- `tests/e2e/helpers/ui.ts` — `addRuleViaUI`, `startRuleViaUI`, `stopRuleViaUI`
- `tests/e2e/helpers/api.ts` — `clearAllRules`, `createRule`, `startRule`, `stopRule`
- `tests/e2e/helpers/setup.ts` — creates temp config before server starts
- `tests/tsconfig.json` — TypeScript config for E2E files

**Protocol automation coverage:**
- TCP forwarding: automated E2E via `tcp.spec.ts`
- UDP one-way: automated E2E via `udp.spec.ts`
- UDP bidirectional-last-client: automated E2E via `udp.spec.ts`
- UDP bidirectional-multi-client: automated E2E via `udp.spec.ts`

**OS service install validation (run explicitly on the target platform, not part of `npm run check`):**
- Windows scheduled task (no admin): `npm run validate:service:windows:user`
- Windows Service (admin): `npm run validate:service:windows:machine`
- macOS LaunchAgent (no sudo): `npm run validate:service:macos`
- Linux systemd (root/sudo): `npm run validate:service:linux`
- Current platform default: `npm run validate:service:current`

Each script uses test-specific names, ports, and temp paths. Never touches production installs.

**Remaining manual QA (cannot be automated):**
- Firewall and OS permission behavior on each platform

**Automated (not manual):**
- Package build correctness: `npm run validate:runtime:smoke`
- TCP/UDP protocol forwarding: `npm run test:e2e`
- OS service install/start/stop/uninstall: `npm run validate:service:*`

## Packaging Commands

```powershell
npm run build:runtime             # cross-platform: builds build/portier/ on the current OS
npm run build:runtime:windows     # Windows package (build/windows/)
npm run build:runtime:macos       # macOS package (build/macos/)
npm run build:runtime:linux       # Linux package (build/linux/)
npm run build:clean               # clean build/portier/ and all platform package dirs
```

## macOS LaunchAgent Commands

Lifecycle management (user-level, no sudo):

```bash
bash scripts/macos/service/install-launch-agent.sh    # copies build/portier/ → ~/Applications/Portier/, registers LaunchAgent
bash scripts/macos/service/uninstall-launch-agent.sh  # stops and removes LaunchAgent; preserves rules.json
bash scripts/macos/service/start-launch-agent.sh      # start (or restart) the LaunchAgent
bash scripts/macos/service/stop-launch-agent.sh       # stop the LaunchAgent
bash scripts/macos/service/status-launch-agent.sh     # show LaunchAgent status via launchctl
```

Install script supports: `--source-dir`, `--install-dir`, `--config-path`, `--host`, `--port`, `--runtime service|node`, `--no-start`.
Uninstall script supports: `--purge` (removes config and logs; off by default).

macOS release archive:

```bash
npm run build:release:portable        # build:runtime then portable tar.gz
```

Output: `build/releases/macos/portier-portable-macos-<version>.tar.gz`

Unsigned. Gatekeeper may quarantine downloaded binaries. Sign with Developer ID for public distribution.
Do not add `build:release:*` to `npm run check` — it is a release step.

## Linux systemd Service Commands

Lifecycle management (requires root/sudo):

```bash
sudo bash scripts/linux/service/install-service.sh    # copies build/portier/ → /opt/portier/, registers systemd service
sudo bash scripts/linux/service/uninstall-service.sh  # stops and removes service; preserves /etc/portier/rules.json
sudo bash scripts/linux/service/start-service.sh      # start (or restart) the service
sudo bash scripts/linux/service/stop-service.sh       # stop the service
sudo bash scripts/linux/service/status-service.sh     # show service status via systemctl
```

Install script supports: `--source-dir`, `--install-dir`, `--config-path`, `--host`, `--port`, `--runtime service|node`, `--no-enable`, `--no-start`.
Uninstall script supports: `--remove-files` (removes `/opt/portier/`), `--remove-config` (removes config directory; off by default).

Linux release archive:

```bash
npm run build:release:portable        # build:runtime then portable tar.gz
```

Output: `build/releases/linux/portier-<version>-linux.tar.gz`

No signing required for Linux tar.gz. Firewall rules for forwarded ports are the user's responsibility (ufw, iptables, firewalld).
Do not add `build:release:*` to `npm run check` — it is a release step.

## Windows Release Commands

Build the portable zip and Inno Setup installer for Windows 10+ (Inno Setup 6 required for the installer):

```powershell
npm run release:current     # portable zip + installer (installer non-fatal if Inno Setup absent)
npm run release:portable    # portable zip only
```

Output:
- `build/releases/windows/portier-<version>-windows-portable.zip`
- `build/releases/windows/Portier-Setup-<version>.exe` (when Inno Setup available)

Build script: `scripts/windows/release/build-release.ps1`
- `-Version 1.1.0` — override version string (default: reads from `package.json`)
- `-NoPackage` — skip `build:runtime` step
- `-InnoPath "C:\..."` — path to `ISCC.exe` if not on PATH

The installer is unsigned. It does NOT create Windows Firewall rules. Config is preserved on uninstall.
Do not add `build:release:*` to `npm run check` — it is a release step.

## Package Validation Commands

```powershell
npm run validate:runtime           # validate existing build/portier/ layout
npm run validate:runtime:build     # build then validate
npm run validate:runtime:smoke     # build, validate, and run smoke test (preferred for release)
```

`validate:runtime:smoke` is the recommended pre-release package check. It does not require
Administrator or root. It does not install OS services.

## OS Service Install Validation Commands

Run these explicitly on the target platform — not included in `npm run check`:

```powershell
npm run validate:service:windows:user     # Windows scheduled task (no Administrator required)
npm run validate:service:windows:machine  # Windows Service (Administrator required)
```

```bash
npm run validate:service:macos    # macOS LaunchAgent (no sudo required)
npm run validate:service:linux    # Linux systemd (requires root/sudo)
npm run validate:service:current  # current platform, user-scope where possible
```

All service validation scripts accept:
- `--no-build` / `-NoBuild` — skip `build:runtime` build step
- `--keep-files` / `-KeepFiles` — preserve temp files for debugging
- `--port` / `-Port` — override the test port

Normal development validation: `npm run check`
Release package validation: `npm run validate:runtime:smoke`
Release service validation: `npm run validate:service:current` (or per-platform variants)

## Release Artifact Commands

Build the current platform's portable archive (and installer if tooling is available):

```powershell
npm run build:release:current       # portable + installer (non-fatal if installer tools absent)
npm run build:release:portable      # portable archive only
```

Validate release artifacts:

```powershell
npm run validate:release:portable     # checks archive contents, readme.txt, forbidden files
npm run validate:release:current      # also checks installer artifact if present
```

Output layout: `build/releases/windows/`, `build/releases/macos/`, `build/releases/linux/`.

Archive filenames are versioned:
- Windows: `portier-<version>-windows-portable.zip`, `Portier-Setup-<version>.exe`
- macOS: `portier-portable-macos-<version>.tar.gz`
- Linux: `portier-<version>-linux.tar.gz`

Service binaries are platform-native. Run on each OS for that OS's artifacts.
Do not add `build:release:*` or `validate:release:*` to `npm run check` — they are release steps.

## Installer Strategy

v1.1 focuses on distribution and native OS service installers. The v1.1 scope, platform strategy, install layouts, artifact targets, and implementation slices are defined in `docs/installer-strategy.md`.

## Roadmap

v1.2 targets diagnostics and operational polish: runtime info endpoint, rule diagnostics, activity log improvements, safer networking UX, settings polish, and diagnostics export. See `docs/roadmap.md`.

## Coding Guidelines

- Keep TypeScript simple and explicit.
- Prefer small modules over clever abstractions.
- Keep shared validation and constants in `shared/sources`.
- Keep TCP and UDP forwarding logic separate.
- `ForwardManager` should own lifecycle orchestration.
- Keep runtime config external.
- Do not bake `rules.json` into executables.
- Do not expose the management UI/API on `0.0.0.0` by default.
- Management UI/API default: `127.0.0.1:47831`.
- Recommended forward listen port range: `48000-48999`.
- Go service static dir default: `web` (packaged layout). Dev: pass `--static-dir ../client/build`.
- TypeScript server static dir default: `client/build`. Prod: pass `--static-dir web` or set `PORTIER_STATIC_DIR`.

## Safety Rules

- Warn clearly when a forward rule listens on `0.0.0.0`.
- Treat management binding to `0.0.0.0` as dangerous.
- Do not silently change firewall rules.
- Do not add telemetry.
- Do not add remote update/download behavior.
- Do not store secrets in repo files.
- Do not modify local user config files unless explicitly requested.

## Do Not Edit Unless Explicitly Asked

- `node_modules/`
- `build/`
- `server/build/`
- `service/build/`
- `client/build/`
- `shared/build/`
- `coverage/`
- `.git/`
- `.env`
- `.env.*`
- `*.log`
- `rules.json`
- generated package outputs
- user-local config files

## Networking Checklist

- TCP sockets clean up on error and close.
- UDP `bidirectional-last-client` mode is documented as limited.
- Do not claim full multi-client UDP support unless implemented.
- Duplicate `protocol + listenHost + listenPort` bindings are rejected.
- Shutdown closes active sockets and servers.

## Response Expectations

- Summarize changed files.
- List validation commands run.
- State anything not run and why.
- Call out follow-up tasks and risks.

## Repository Naming

- Use `sources/` for TypeScript source directories.
- Use `build/` for generated build outputs.
- Keep executable automation scripts under `scripts/`, with `release/` (artifact builders), `service/` (OS service lifecycle), and platform docs/templates under `scripts/windows/`, `scripts/macos/`, and `scripts/linux/`.
- Normal documentation filenames are lowercase, such as `docs/architecture.md` and `docs/checklist.md`. The root `README.md` is uppercase.
- Keep tool-required files uppercase: `AGENTS.md`, `CLAUDE.md`, and `SKILL.md` in Codex/Claude skill directories.
- React component and view files under `client/sources/` use **CamelCase** filenames (e.g., `ForwardRuleList.tsx`, `StatCard.tsx`). Non-component files use the existing repo convention (e.g., `format.ts`, `nav.ts`).
