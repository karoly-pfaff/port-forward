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
  Portier-Setup-1.1.0.exe
  portier-portable-windows.zip

build/releases/macos/
  Portier-1.1.0.pkg
  portier-portable-macos.tar.gz

build/releases/linux/
  portier-1.1.0-linux.tar.gz
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
npm run package:portier
npm run validate:package
npm run validate:package:build
npm run validate:package:smoke    # preferred: builds, validates layout, smoke-tests binary
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

### Slice 2 — Windows installer (Inno Setup)

- Build Inno Setup script for machine-wide install.
- Install to `%ProgramFiles%\Portier\` with config at `%ProgramData%\Portier\`.
- Optional Windows Service registration during install.
- Uninstall preserves `rules.json` by default.
- Add `validate:installer:windows` script if automatable.

### Slice 3 — macOS package and LaunchAgent polish

- `.pkg` installer or improved install flow using `pkgbuild` / `productbuild`.
- LaunchAgent script improvements identified during v1.0 testing.
- Signing and notarization documentation added.
- `validate:service:macos` passes on target hardware.

### Slice 4 — Linux install scripts

- Complete install/uninstall/start/stop/status scripts under `scripts/linux/`.
- systemd unit generation with correct `ExecStart` for Go service.
- Node fallback mode documented in `deploy/systemd/readme.md`.
- `validate:service:linux` passes on target Linux host.

### Slice 5 — Explicit service validation scripts

- `validate:service:windows:user` — Windows scheduled task (no admin).
- `validate:service:windows:machine` — Windows Service (admin).
- `validate:service:macos` — macOS LaunchAgent.
- `validate:service:linux` — Linux systemd.
- `validate:service:current` — current platform.

### Slice 6 — Release artifact generation

- `package:releases` script (or per-platform variants) producing `build/releases/` layout.
- Portable archives and installer outputs in the defined layout.
- Smoke test each portable archive on the corresponding platform.

### Slice 7 — v1.1 readiness audit

- All Slice 2–6 items passing on each target platform.
- Version bumped to `1.1.0`.
- Changelog entry added.
- Git tag `v1.1.0` created.
