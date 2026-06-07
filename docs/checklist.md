# QA Checklist

## Automated Release Validation

Run these before tagging:

- [ ] `npm run lint`
- [ ] `npm run typecheck`
- [ ] `npm run test`
- [ ] `npm run build`
- [ ] `npm run build:client`
- [ ] `npm run test:e2e`
- [ ] `npm run check`
- [ ] `go test ./...` from `service/`
- [ ] `go build ./...` from `service/`
- [ ] `npm run build:service`
- [ ] `npm run validate:runtime:smoke` — builds `build/portier/`, validates layout, runs smoke test

## Additional Validation Suites

Run explicitly — not part of `npm run check`. Slower or platform-sensitive.

- [ ] `npm run validate:contract` — API contract parity: runs all API scenarios against TypeScript server; if Go binary available, runs same scenarios against Go service and compares response shapes, status codes, field names, and error shapes. (`scripts/validate-contract.js`)
- [ ] `npm run validate:binary` (or `validate:runtime:behavior`) — Runtime binary behavior: starts `build/portier/service[.exe]`, verifies health, static serving, missing-static-dir handling, invalid-config failure, and clean shutdown. Runs `build:runtime` first unless `--no-build` is passed. (`scripts/validate-binary.js`)
- [ ] `npm run validate:scripts` — Installer script analysis: static analysis of all platform install and validation scripts (no firewall commands, test-specific names in validate scripts, production path defaults in install scripts, quoting correctness); plus dry-run execution on the current platform. (`scripts/validate-scripts.js`)

## Explicit OS Service Install Validation

These commands must be run explicitly on the target platform before distribution. They are not run by `npm run check`.

Each script uses test-specific service names, ports, and temp directories. Production Portier installs and config are never touched.

- [ ] `npm run validate:service:windows:user` — Windows scheduled task (no Administrator required)
- [ ] `npm run validate:service:windows:machine` — Windows Service (Administrator required)
- [ ] `npm run validate:service:macos` — macOS LaunchAgent (no sudo required)
- [ ] `npm run validate:service:linux` — Linux systemd unit (root/sudo required)

Each script validates:
1. Package copy to isolated temp install dir
2. Service/task/agent registration with test-specific name
3. Service start
4. `/api/health` responds 200
5. Web UI HTML served at `/`
6. Service stop
7. Service/task/agent unregistered and verified removed
8. Temp files cleaned up

Flags supported by all scripts:
- `--no-build` / `-NoBuild` — skip `npm run build:runtime`, use existing `build/portier/`
- `--keep-files` / `-KeepFiles` — preserve temp directories on failure for debugging
- `--port` / `-Port` — override the management port (default: auto-detect free port)

## Automated Coverage Confirmed

Shared and TypeScript coverage:

- [ ] Shared validation and port advisories
- [ ] TypeScript server CRUD HTTP layer
- [ ] TypeScript server import/export HTTP layer
- [ ] TypeScript server status, start/stop, reorder, duplicate binding, and static serving behavior
- [ ] Client App integration flows
- [ ] Settings import/FileReader flow
- [ ] Dashboard, Activity, Settings, API Docs, Forward Rules, and drawer flows

Playwright E2E coverage:

- [ ] App load
- [ ] Add/edit/delete rule flows
- [ ] Start/stop rule flow
- [ ] Settings config import
- [ ] Mobile sidebar behavior
- [ ] TCP real forwarding
- [ ] UDP one-way real forwarding
- [ ] UDP bidirectional-last-client real forwarding
- [ ] UDP bidirectional-multi-client real forwarding
- [ ] TCP and UDP activity assertions

Go service coverage:

- [ ] Config load/save and import/export
- [ ] Manager lifecycle, duplicate binding rejection, update/restart behavior, and reorder
- [ ] API routes and error shapes
- [ ] TCP real forwarding and activity
- [ ] UDP one-way, bidirectional-last-client, bidirectional-multi-client, stats, sessions, stop, and activity
- [ ] Port advisories, validation, options, static serving, and health endpoint

## Manual Platform QA Required Before Distribution

Manual QA is now limited to firewall behavior and production install paths. Core TCP/UDP protocol correctness and OS service install/uninstall flows are automated.

### Package Build and Smoke Test (Automated)

- [ ] `npm run validate:runtime:smoke` passes — builds `build/portier/`, validates layout and content, runs smoke test.
  - Validates: `service`/`service.exe`, `server.js`, `web/index.html`, `web/assets/`, `readme.txt`.
  - Validates: `readme.txt` mentions management URL and config path.
  - Validates: `node_modules`, `rules.json`, `sources/`, `client/`, `server/` are absent from the package.
  - Smoke test: starts the packaged binary, polls `/api/health`, GETs `/`, verifies HTML is served, stops cleanly.

### OS Service Install Validation (Automated — Run Explicitly)

Service install, start, health check, stop, and uninstall are validated by explicit commands on each platform:

- [ ] `npm run validate:service:windows:user` — Windows scheduled task, no Administrator required.
- [ ] `npm run validate:service:windows:machine` — Windows Service, Administrator required.
- [ ] `npm run validate:service:macos` — macOS LaunchAgent, no sudo required.
- [ ] `npm run validate:service:linux` — Linux systemd unit, root/sudo required.

Each script uses test-specific names and temp directories. Production Portier installs are not touched.

Pass `--no-build` to reuse an existing `build/portier/` and skip the package build step.

### Windows Firewall (Manual — On Real Production Install)

- [ ] Windows Firewall prompts or required inbound rules documented and observed for LAN-visible forwarded ports.
- [ ] Production install to `%ProgramFiles%\Portier` (Machine) or `%LOCALAPPDATA%\Portier` (User) verified.
- [ ] Config preserved at `%ProgramData%\Portier\rules.json` (Machine) or `%APPDATA%\Portier\rules.json` (User) after uninstall.

### macOS Firewall (Manual — On Real Production Install)

- [ ] macOS Firewall prompts or required settings documented and observed for LAN-visible forwarded ports.
- [ ] Production install to `~/Applications/Portier` with config at `~/Library/Application Support/Portier/rules.json` verified.
- [ ] Config preserved after `scripts/macos/service/uninstall-launch-agent.sh`.

### Linux Firewall (Manual — On Real Production Install)

- [ ] Firewall rules for LAN-visible forwarded ports documented and observed.
- [ ] Production install to `/opt/portier` with config at `/etc/portier/rules.json` verified.
- [ ] Config preserved after `scripts/linux/service/uninstall-service.sh` without `--remove-config`.

## Post-v1.0 Follow-Ups

- Drag-and-drop rule reorder testing, if drag-and-drop UI replaces the current Move Up/Down controls.
- macOS `.app` bundle or Homebrew formula.
- Linux hardening beyond the example systemd unit.

## v1.2 Roadmap

v1.2 focuses on diagnostics and operational polish. See `docs/roadmap.md` for goals, slices, and non-goals.

Checklist items to add per slice as work proceeds:

- [ ] **Slice 1** — Runtime info endpoint and UI display
- [ ] **Slice 2** — Rule diagnostics API
- [ ] **Slice 3** — Rule diagnostics UI
- [ ] **Slice 4** — Activity Log polish
- [ ] **Slice 5** — Settings / runtime / config polish
- [ ] **Slice 6** — Safer networking UX pass
- [ ] **Slice 7** — Diagnostics export
- [ ] **Slice 8** — v1.2 readiness audit, version bump, changelog, tag

---

## v1.1 Installer Readiness

v1.1 focuses on distribution and native OS service installers. See `docs/installer-strategy.md` for scope and implementation slices.

Checklist items to add per slice as work proceeds:

- [x] **Slice 2** — Windows Inno Setup installer: machine-wide install (`%ProgramFiles%\Portier`), optional Windows Service task, uninstall preserves `rules.json`. Scripts in `scripts/windows/release/`. Build via `npm run release:current`. Requires Inno Setup 6.
- [x] **Slice 3** — macOS LaunchAgent polish: auto-copy from `build/portier/`, `--source-dir`/`--no-start`/`--runtime` options, label bug fixed, `--purge` on uninstall, `scripts/macos/release/build-release.sh` for portable tar.gz, signing/notarization docs. Service lifecycle scripts in `scripts/macos/service/`. Build via `npm run release:current` (on macOS). Validate via `npm run validate:service:macos`.
- [x] **Slice 4** — Linux install/uninstall/start/stop/status scripts complete; `--source-dir`/`--no-enable` added to `install-service.sh`; `scripts/linux/release/build-release.sh` for portable tar.gz; service lifecycle scripts in `scripts/linux/service/`; systemd unit examples and docs updated.
- [x] **Slice 5** — `validate:service:*` scripts unified: all support `--no-build`, `--keep-files`, `--port`; test-specific names/paths/ports on all platforms; `validate:service:current` dispatches by OS with unsupported-platform error. Windows user-scope validated on Windows host. macOS/Linux validation requires the respective OS.
- [x] **Slice 6** — `release:current` and `release:portable` produce `build/releases/<platform>/` with versioned portable archives. `validate:release:portable` validates layout, required files, forbidden files, and readme.txt content. Windows zip via `Compress-Archive`; macOS/Linux tar.gz via `scripts/macos/release/build-release.sh` and `scripts/linux/release/build-release.sh`. Windows installer non-fatal if Inno Setup absent. macOS .pkg and Linux .deb/.rpm deferred.
- [x] **Slice 7** — v1.1 readiness audit passed; version bumped to `1.1.0`; changelog entry finalized; tag created.
