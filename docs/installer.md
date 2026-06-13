# Portier v1.1 Installer and Distribution Strategy

## v1.1 Goal

> Portier v1.1 makes Portier easy and safe to install as a native background service on Windows, macOS, and Linux, with automated package/service validation and clean release artifacts.

v1.0 proved the app and runtime behavior. TCP and UDP forwarding are correct and tested. The Go service and TypeScript server both work. The management UI is complete.

v1.1 focuses on distribution, installers, and platform polish. It should not introduce new forwarding or product features unless a feature is directly required by installation or runtime quality (for example: a `--version` flag needed by an installer).

---

## Supported Runtimes

### A — Native Go service (preferred)

- Smaller binary, no Node.js dependency, no warm-up time.
- Best suited for OS service installation on all platforms.
- Package filename: `service` on macOS/Linux, `service.exe` on Windows.
- Default static dir for packaged layout: `web`.

### B — Node/TypeScript server (reference and fallback)

- Supported as a fallback when Go toolchain is unavailable during packaging.
- Required by users who prefer or need Node.js-managed processes.
- Package filename: `server.js`.
- Default static dir in development: `client/build`. In packaged layout: pass `--static-dir web` or set `PORTIER_STATIC_DIR`.
- Not deprecated. Not legacy. Remains a first-class fallback runtime.

### Shared behavior

Both runtimes:
- Serve the same `web/` React UI.
- Use the same external config path per platform.
- Implement the same REST API contract.
- Default to `127.0.0.1:47831` for the management endpoint.

---

## Platform Strategy

### Windows

**v1.1 goal:** add a real installer.

- Preferred first installer: **Inno Setup** (`.exe` installer).
- Machine-wide install:
  - Binaries to `%ProgramFiles%\Portier\`.
  - Config and logs to `%ProgramData%\Portier\`.
  - Service registration as a Windows Service.
- User-scope install remains supported through scripts and scheduled task.
- Installer must not silently edit firewall rules.
- Uninstall must preserve `rules.json` by default.
- Enterprise path (WiX/MSI) is out of v1.1 scope unless explicitly chosen.

### macOS

**v1.1 goal:** improve macOS installation substantially.

- Preferred target: `.pkg` installer or packageable install flow.
- Default service scope: user-level LaunchAgent (no `sudo` required).
- Default install layout:
  - Binaries under `~/Applications/Portier/`.
  - Config under `~/Library/Application Support/Portier/`.
  - Logs under `~/Library/Logs/Portier/`.
- Signing and notarization should be documented (see [Signing and Notarization](#signing-and-notarization)).
- Full `.app` bundle or tray app is out of v1.1 scope.

### Linux

**v1.1 goal:** complete install/uninstall/start/stop/status helper scripts.

- Primary service model: **systemd**.
- Preferred runtime: Go service.
- Node/TypeScript server remains documented as fallback (requires Node.js).
- `.deb`, `.rpm`, Homebrew, winget, and Chocolatey packages are out of v1.1 scope.

---

## Install Layouts

These are the intended production install paths. Build artifacts and development directories do not appear here.

### Windows — machine-wide

```text
C:\Program Files\Portier\
  service.exe
  server.js
  web\
    index.html
    assets\

C:\ProgramData\Portier\
  rules.json
  logs\
```

### Windows — user-scope

```text
%LOCALAPPDATA%\Portier\
  service.exe
  server.js
  web\
    index.html
    assets\

%APPDATA%\Portier\
  rules.json
  logs\
```

### macOS — user-level

```text
~/Applications/Portier/
  service
  server.js
  web/
    index.html
    assets/

~/Library/Application Support/Portier/
  rules.json

~/Library/Logs/Portier/
```

### Linux — machine-wide

```text
/opt/portier/
  service
  server.js
  web/
    index.html
    assets/

/etc/portier/
  rules.json
```

---

## Release Artifact Layout

v1.1 target output under `build/releases/`:

```text
build/releases/windows/
  Portier-Setup-<version>.exe
  portier-<version>-windows-portable.zip

build/releases/macos/
  portier-portable-macos-<version>.tar.gz   (.pkg deferred to future release)

build/releases/linux/
  portier-<version>-linux.tar.gz
```

Portable archives contain the clean runtime layout:

```text
service          (or service.exe on Windows)
server.js
web/
  index.html
  assets/
readme.txt
```

Installers copy this layout into the platform-appropriate directory. Config (`rules.json`) is not packaged as a user data file by default. A sample config may be included as example documentation only.

---

## Validation Policy

### Normal validation (run for every change)

```powershell
npm run lint
npm run typecheck
npm run test
npm run build
```

### E2E / protocol validation

```powershell
npm run build:client
npm run test:e2e
```

TCP and UDP correctness are automated, not manual QA-only.

### Package validation (run when packaging changes)

```powershell
npm run build:runtime
npm run validate:runtime
npm run validate:runtime:build
npm run validate:runtime:smoke    # preferred: builds, validates layout, smoke-tests binary
```

### Explicit service validation (run on target platform before release)

```powershell
npm run validate:service:windows:user     # Windows scheduled task (no admin)
npm run validate:service:windows:machine  # Windows Service (admin required)
```

```bash
npm run validate:service:macos    # macOS LaunchAgent (no sudo)
npm run validate:service:linux    # Linux systemd (root/sudo)
npm run validate:service:current  # current platform default
```

These are not run by `npm run check`. They require the target platform and use test-specific names, ports, and temp dirs. They never touch production installs.

### Manual / platform QA

- Firewall prompts and OS permission dialogs on each platform.
- Real machine sanity checks after production install.
- Signing and notarization behavior when developer credentials are involved.
- Installer UX review.

---

## Signing and Notarization

### Windows

- Unsigned installers and binaries may trigger Windows SmartScreen warnings.
- Code signing with an EV certificate eliminates SmartScreen for machine-wide installs.
- v1.1 can produce unsigned local installers when a code-signing certificate is unavailable.
- Signing should be documented as a required step for public distribution.

### macOS

- Unsigned packages and binaries trigger Gatekeeper warnings and may be quarantined.
- Developer ID Application signing and notarization are required for public distribution without user intervention.
- When Developer ID credentials are available, the build pipeline should sign and notarize the `.pkg` and the `service` binary.
- Local development builds do not require notarization; `xattr -d com.apple.quarantine` or Gatekeeper bypass is acceptable for internal testing.

### Linux

- No signing requirement for `.tar.gz` archives.
- Future `.deb` / `.rpm` repository signing (GPG key, apt/yum repo) is out of v1.1 scope.

---

## Non-Goals for v1.1

The following are explicitly out of scope for v1.1. They may be considered for future releases.

- New port forwarding or protocol features.
- Firewall auto-rule management (silently creating/removing OS firewall rules).
- Auto-update mechanism.
- System tray app or desktop GUI shell.
- Full `.app` bundle for macOS.
- Homebrew tap.
- Winget package.
- Chocolatey package.
- `.deb` / `.rpm` packages (unless explicitly promoted).
- WiX / MSI installer (unless explicitly chosen over Inno Setup).
- Cloud sync, user accounts, or telemetry.

---

## Implementation Slices

### Slice 1 — Installer strategy (this document)

Defines scope, platform decisions, layouts, and validation policy for v1.1.

### Slice 2 — Windows installer (Inno Setup) ✓

- `scripts/windows/release/portier.iss` — Inno Setup 6 script.
- `scripts/windows/release/build-release.ps1` — build wrapper: reads version from `package.json`, runs `build:runtime`, invokes ISCC.exe.
- Installs to `%ProgramFiles%\Portier\` with config at `%ProgramData%\Portier\`.
- Optional Windows Service registration task (checked by default for machine-wide installs).
- Uninstall stops and removes the service; `rules.json` is preserved.
- Logs directory is removed on uninstall; config directory is not.
- Upgrade support: installer stops any running service before overwriting binaries.
- `npm run build:release:current` — full build (portable + installer; installer non-fatal if Inno Setup absent).
- Output: `build/releases/windows/Portier-Setup-<version>.exe`
- Requires Inno Setup 6: https://jrsoftware.org/isinfo.php
- Installer is unsigned; sign with an EV certificate before public distribution.

### Slice 3 — macOS package and LaunchAgent polish ✓

- `install-launch-agent.sh` updated: auto-copies `build/portier/` to `~/Applications/Portier/` by default; adds `--source-dir`, `--no-start`, and `--runtime service|node` options; fixes LaunchAgent label consistency (`com.portier.port-forwarding` everywhere).
- `uninstall-launch-agent.sh` updated: adds `--purge` flag for removing config and logs (off by default — config is always preserved).
- `scripts/macos/release/build-release.sh` added: builds `build/releases/macos/portier-portable-macos-<version>.tar.gz` from `build/portier/`.
- Signing and notarization documented in `scripts/macos/readme.md`.
- `validate:service:macos` passes on macOS with `npm run validate:service:macos`.
- `.pkg` installer is documented as a follow-up; requires macOS tooling (`pkgbuild`/`productbuild`).
- Output: `build/releases/macos/portier-portable-macos-<version>.tar.gz`

### Slice 4 — Linux install scripts ✓

- `install-service.sh` updated: auto-copies `build/portier/` to `/opt/portier/` by default; adds `--source-dir`, `--no-enable`, `--no-start`, `--runtime service|node` options.
- `uninstall-service.sh`: stops, disables, and removes the unit file; preserves `/etc/portier/rules.json` by default; `--remove-files` removes `/opt/portier/`; `--remove-config` removes config directory.
- `start-service.sh`, `stop-service.sh`, `status-service.sh`: lifecycle helpers that require root.
- `scripts/linux/release/build-release.sh` added: builds `build/releases/linux/portier-<version>-linux.tar.gz` from `build/portier/`.
- systemd unit examples updated: `portier.service.example` (Go service), `portier-node.service.example` (Node fallback).
- `scripts/linux/readme.md` updated: install flags table with `--source-dir` / `--no-enable`, release archive section, firewall notes, journald log commands.
- Output: `build/releases/linux/portier-<version>-linux.tar.gz`
- `validate:service:linux` must be run explicitly on a Linux host with systemd and root.

### Slice 5 — Explicit service validation scripts ✓

- `validate:service:windows:user` — Windows scheduled task, no Administrator required. Test task name: `PortierTestUser`. Temp paths under `$TEMP\PortierTestUser\`.
- `validate:service:windows:machine` — Windows Service, Administrator required. Test service name: `PortierTestMachine`. Temp paths under `$TEMP\PortierTestMachine\`.
- `validate:service:macos` — macOS LaunchAgent, no sudo required. Test label: `com.portier.test`. Plist at `~/Library/LaunchAgents/com.portier.test.plist`. Temp paths under `$TMPDIR`.
- `validate:service:linux` — Linux systemd, root/sudo required. Test unit: `portier-test.service`. Temp paths under `/tmp/portier-test-<pid>/`. Fails clearly if not root or systemd unavailable.
- `validate:service:current` — cross-platform Node dispatcher. Runs user-scope on Windows, LaunchAgent on macOS, systemd on Linux. Fails clearly on unsupported platforms.
- All scripts support `--no-build` / `-NoBuild`, `--keep-files` / `-KeepFiles`, `--port` / `-Port`.
- None touch production service names, install directories, config paths, or port 47831.
- Scripts live in `scripts/<os>/service/` (validate and lifecycle scripts).

### Slice 6 — Release artifact generation ✓

- `scripts/build-release.js`: unified Node script for all platforms. Reads version from `package.json` (or `--version`). Calls `build:runtime` (unless `--no-build`), then builds portable archives and, where tooling is available, installer artifacts.
- Windows: produces `portier-<version>-windows-portable.zip` from `build/portier/` via PowerShell `Compress-Archive`. Calls `scripts/windows/release/build-release.ps1 -NoPackage` for the Inno Setup installer (non-fatal if ISCC.exe is absent).
- macOS: delegates to `scripts/macos/release/build-release.sh --no-package --version <v>` → `portier-portable-macos-<version>.tar.gz`.
- Linux: delegates to `scripts/linux/release/build-release.sh --no-package --version <v>` → `portier-<version>-linux.tar.gz`.
- Service binaries are platform-native; only the current OS can produce its artifacts. For multi-platform releases, run on each target OS.
- `scripts/validate-release.js`: validates `build/releases/<platform>/` layout. Lists archive contents, checks required files (`service`/`service.exe`, `server.js`, `web/index.html`, `web/assets/`, `readme.txt`), checks forbidden files absent, extracts and checks readme.txt content.
- All three `readme.txt` files (Windows/macOS/Linux) updated: added "does not install OS services", "not bundled in this archive", `--config` / `--static-dir web` flags, portable archive notice.
- npm scripts: `build:release`, `build:release:current`, `build:release:portable`, `validate:release`, `validate:release:current`, `validate:release:portable`.
- Output: `build/releases/windows/portier-<version>-windows-portable.zip`, `build/releases/macos/portier-portable-macos-<version>.tar.gz`, `build/releases/linux/portier-<version>-linux.tar.gz`, `build/releases/windows/Portier-Setup-<version>.exe` (when Inno Setup available).
- macOS `.pkg` and Linux `.deb`/`.rpm` remain out of v1.1 scope.

### Slice 7 — v1.1 readiness audit ✓

- All Slice 2–6 items passing on Windows (current platform). macOS/Linux service validation requires respective OS hosts.
- Version bumped to `1.1.0` across all package.json files.
- Changelog entry finalized (date: 2026-06-06).
- Git tag `v1.1.0` ready to create.
