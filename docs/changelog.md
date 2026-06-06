# Changelog

All notable changes to Portier are documented here.

---

## [1.1.0] - Unreleased

### Goal

Portier v1.1 makes Portier easy and safe to install as a native background service on Windows, macOS, and Linux, with automated package/service validation and clean release artifacts.

See `docs/installer-strategy.md` for scope, platform decisions, and implementation slices.

### Added

- Windows Inno Setup installer (`scripts/windows/release/portier.iss`): installs to `%ProgramFiles%\Portier\`, optional Windows Service registration with auto-start at boot, config at `%ProgramData%\Portier\rules.json`. Upgrade support: stops running service before overwriting binaries. Uninstall removes service registration and logs; preserves `rules.json` by default.
- `build-release.ps1` (`scripts/windows/release/`): reads version from `package.json`, runs `package:portier`, calls ISCC.exe, produces `build/releases/windows/Portier-Setup-<version>.exe`.
- macOS install scripts updated: `install-launch-agent.sh` now auto-copies `build/portier/` to `~/Applications/Portier/` by default; adds `--source-dir`, `--no-start`, and `--runtime service|node` options; fixes label consistency bug (`com.portier.port-forwarding` everywhere).
- `uninstall-launch-agent.sh` adds `--purge` flag for removing config and logs (config is preserved by default).
- `scripts/macos/release/build-release.sh` — builds `build/releases/macos/portier-portable-macos-<version>.tar.gz` from `build/portier/`.
- Signing and notarization documented in `deploy/macos/readme.md` (unsigned local builds supported; Developer ID signing documented as required for public distribution).
- Linux `install-service.sh` updated: auto-copies `build/portier/` to `/opt/portier/` by default; adds `--source-dir`, `--no-enable`, `--no-start`, and `--runtime service|node` options.
- `scripts/linux/release/build-release.sh` — builds `build/releases/linux/portier-<version>-linux.tar.gz` from `build/portier/`.
- `deploy/linux/readme.md` updated: install flags table, release archive section, firewall notes, journald commands, `--no-enable` documented.
- `scripts/windows/service/validate-user-install.ps1`: validates user-scope scheduled task flow with test name `PortierTestUser`, isolated temp dirs, auto-port detection, `-NoBuild`/`-KeepFiles`/`-Port` flags; never touches production.
- `scripts/windows/service/validate-machine-service.ps1`: validates machine-scope Windows Service flow with test name `PortierTestMachine`; requires Administrator; same flags; never touches production.
- `scripts/macos/service/validate-launch-agent.sh`: validates LaunchAgent flow with test label `com.portier.test`, temp plist at `~/Library/LaunchAgents/com.portier.test.plist`; no sudo required; `--no-build`/`--keep-files`/`--port` flags.
- `scripts/linux/service/validate-systemd-service.sh`: validates systemd flow with test unit `portier-test.service`, temp paths under `/tmp/portier-test-<pid>/`; requires root; fails clearly if not root or systemd unavailable.
- `scripts/validate-service.js`: cross-platform dispatcher — Windows runs user-scope, macOS runs LaunchAgent, Linux runs systemd; fails clearly on unsupported platforms.
- `npm run validate:service:current` / `validate:service:windows:user` / `validate:service:windows:machine` / `validate:service:macos` / `validate:service:linux` — explicit release validation commands.

- `scripts/package-release.js`: unified release packaging script for all platforms. Reads version from `package.json`, calls `package:portier`, produces portable archives (Windows `.zip`, macOS/Linux `.tar.gz`) and Windows installer (non-fatal if Inno Setup absent). Service binaries are platform-native; run on each target OS.
- `scripts/validate-artifacts.js`: validates `build/releases/<platform>/` layout, archive contents (required/forbidden files, readme.txt content), and optional installer artifact.
- Updated `readme.txt` in Windows/macOS/Linux build scripts: now includes portable archive notice ("does not install OS services"), `--config` / `--static-dir web` options, "not bundled in this archive" note for config.
- `npm run release` / `release:current` — full release packaging for current platform (portable + installer if available).
- `npm run release:portable` — portable archive only, skip installer.
- `npm run validate:release` / `validate:release:current` — validate release artifacts for current platform.
- `npm run validate:release:portable` — validate portable archive only.

### Planned

- macOS `.pkg` installer via `pkgbuild`/`productbuild` (requires macOS tooling; deferred).
- Linux `.deb` / `.rpm` packages (deferred beyond v1.1).

---

## [1.0.0] - 2026-06-06

### Added

- Native Go service runtime in `service/`, preferred for production packages while the TypeScript server in `server/` remains supported as the reference and fallback runtime.
- TCP forwarding in both runtimes with real listeners, bidirectional piping, status counters, and lifecycle cleanup.
- UDP forwarding in both runtimes across all supported modes: `one-way`, `bidirectional-last-client`, and `bidirectional-multi-client`.
- Rule CRUD, start/stop, update/restart behavior, duplicate listen binding rejection, persistent config, config import/export, and rule reorder.
- Activity Log with bounded in-memory events for rule lifecycle, TCP connections, UDP packets, UDP multi-client sessions, config export, and config import.
- Dark React web UI with Dashboard, Forward Rules, Activity, Settings, API Docs, mobile sidebar, add/edit/delete drawer flows, error banners, LAN exposure warnings, and common/recommended port advisories.
- Playwright E2E coverage for app load, add/edit/delete flows, start/stop, settings import, mobile navigation, and real TCP/UDP forwarding.
- Go service tests for config, manager behavior, API routes, TCP forwarding, UDP modes, activity events, validation, options, and advisories.
- Windows, macOS, and Linux packaging/install docs and scripts, including Go service mode and Node/TypeScript server fallback where applicable.

### Changed

- Repository convention standardized on `sources/` for source directories, `build/` for generated output, `deploy/` for install examples/templates, and `scripts/` for executable automation.
- Production package layout standardized to a flat install directory:

```text
<install-dir>/
  service          (or service.exe on Windows)
  server.js        (Node fallback, requires Node.js)
  web/
    index.html
    assets/
```

- Native Go service static directory defaults to `web` for packaged layout. Repository development commands may still pass `../client/build`.
- TypeScript server remains supported and can serve either repository `client/build` or packaged `web` via `--static-dir` / `PORTIER_STATIC_DIR`.
- Runtime config remains external in all modes; `rules.json` is not baked into binaries or packages.
- Manual QA scope narrowed to platform/package/service install behavior because core TCP/UDP correctness is automated.

### Validation

- Root validation scripts cover lint, typecheck, unit/integration tests, Go tests, build, Playwright E2E, package smoke checks, and repository hygiene checks.
- Real protocol coverage is automated for TCP, UDP one-way, UDP bidirectional-last-client, and UDP bidirectional-multi-client.

---

## [0.1.0] - 2026-06-04

First usable internal release of Portier, a local TCP/UDP port forwarding manager for development and LAN testing.

### Added

- TypeScript server with REST API on `127.0.0.1:47831` by default.
- TCP forwarding, UDP one-way forwarding, and UDP bidirectional-last-client forwarding.
- React management UI for listing, creating, editing, starting, stopping, and deleting forward rules.
- Shared TypeScript validation, common port advisories, recommended forward port range, and LAN exposure warnings.
- External JSON config path via `--config` / `PORTIER_CONFIG`.
- Static web serving via `--static-dir` / `PORTIER_STATIC_DIR`.
