# Portier Installation And Distribution

This document describes the current packaging and service-install strategy. Historical
installer slice notes belong in `docs/changelog.md` and `docs/roadmap.md`, not here.

## Supported Runtimes

Portier ships two server runtimes:

- **Go service** (`service` or `service.exe`): preferred for production deployment.
  It has a smaller footprint and no Node.js dependency.
- **Node fallback** (`server.js`): supported fallback runtime. It requires Node.js and
  should be run with `--static-dir web` in packaged layouts.

Both serve the same `web/` React UI, use external config, implement the same REST API,
and default the management endpoint to `127.0.0.1:47831`.

## Packaged Layout

Portable and installed packages use this runtime layout:

```text
<install-dir>/
  portier          (or portier.exe on Windows)
  service          (or service.exe on Windows)
  server.js
  web/
    index.html
    assets/
  readme.txt
```

`rules.json` is user data and is not bundled into binaries or archives. A sample config
may be included only as documentation.

Development build output stays repo-internal:

```text
service/build/portier-service
server/build/
client/build/
tools/cli/build/portier-cli
```

## Platform Install Paths

Windows machine-wide:

```text
C:\Program Files\Portier\
  portier.exe
  service.exe
  server.js
  web\

C:\ProgramData\Portier\
  rules.json
  logs\
```

Windows user-scope:

```text
%LOCALAPPDATA%\Portier\
  portier.exe
  service.exe
  server.js
  web\

%APPDATA%\Portier\
  rules.json
  logs\
```

macOS user-level:

```text
~/Applications/Portier/
  portier
  service
  server.js
  web/

~/Library/Application Support/Portier/
  rules.json

~/Library/Logs/Portier/
```

Linux machine-wide:

```text
/opt/portier/
  portier
  service
  server.js
  web/

/etc/portier/
  rules.json
```

## Service Models

- Windows machine-wide installs use a Windows Service.
- Windows user-scope installs use a Scheduled Task.
- macOS installs use a user LaunchAgent by default.
- Linux installs use systemd.

Service scripts must use test-specific names during validation and must not touch
production installs. Installers and scripts must preserve user config by default.

## Release Artifacts

Current-platform release commands write artifacts under `build/releases/<platform>/`:

```powershell
npm run build:release:current
npm run build:release:portable
npm run validate:release:current
npm run validate:release:portable
```

Expected artifact names:

```text
build/releases/windows/
  Portier-Setup-<version>.exe
  portier-<version>-windows-portable.zip

build/releases/macos/
  portier-portable-macos-<version>.tar.gz

build/releases/linux/
  portier-<version>-linux.tar.gz
```

The Windows installer depends on Inno Setup when building on Windows. If Inno Setup is
unavailable, the portable archive can still be produced and should be reported clearly.

## Validation

For ordinary changes:

```powershell
npm run lint
npm run typecheck
npm run test
npm run build
npm run check
```

For packaging changes:

```powershell
npm run validate:runtime:smoke
npm run validate:release:portable
```

For target-platform service validation:

```powershell
npm run validate:service:windows:user
npm run validate:service:windows:machine
npm run validate:service:current
```

```bash
npm run validate:service:macos
npm run validate:service:linux
```

These service validations are explicit release checks, not part of `npm run check`.
They use isolated temp paths, test-specific service names, and non-production ports.

## Signing And Notarization

- Windows public distribution should sign installers and binaries. Unsigned builds may
  trigger SmartScreen warnings.
- macOS public distribution should use Developer ID signing and notarization. Local
  development builds may remain unsigned.
- Linux portable archives do not require signing, but future package repositories would
  need normal repository signing.

## Safety Rules

- Do not silently create or remove firewall rules.
- Do not add telemetry, cloud sync, or auto-update behavior.
- Preserve `rules.json` on uninstall unless the user explicitly requests purge.
- Keep config and logs outside the packaged runtime directory.
- Do not add platform package managers (`deb`, `rpm`, Homebrew, winget, Chocolatey) or
  an MSI/WiX path without a deliberate product decision.
