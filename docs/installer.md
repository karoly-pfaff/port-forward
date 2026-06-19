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

## Upgrade Safety

Portier separates the disposable install directory (binaries, `web/`, `api/openapi.json`)
from external data (`rules.json`, recovery backup/quarantine side files, logs). The
install directory can be replaced wholesale; user data is never inside it.

An upgrade must therefore preserve user config and data: replacing the binaries/web
assets must not touch `rules.json`, must not rewrite or auto-migrate config, and must
leave the runtime able to start healthy and serve the UI. There is no automatic config
migration on upgrade; `portier config migrate` stays explicit (see `docs/recovery.md`).

This guarantee is validated on the current platform by an upgrade-preservation smoke
(`npm run validate:upgrade:current`): it extracts the portable archive, runs the runtime
against an external data dir, replaces the install directory with a fresh extraction, and
asserts config, the configured rules, and recovery side files survive while the runtime
restarts healthy at the expected version. This is the safety gate that native installer
upgrades (the canonical Windows WiX/MSI and the planned macOS `.pkg`) must satisfy.

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
  Portier-<version>.msi         (WiX — canonical Windows installer)
  portier-<version>-windows-portable.zip
  SHA256SUMS

build/releases/macos/
  Portier-<version>.pkg         (native installer — built on macOS, in progress)
  portier-portable-macos-<version>.tar.gz
  SHA256SUMS

build/releases/linux/
  portier-<version>-linux.tar.gz
  SHA256SUMS
```

The **WiX MSI is the canonical Windows installer** (silent install, Group Policy/SCCM/Intune,
standard Add/Remove Programs + repair). It depends on the WiX Toolset v7
(`scripts/windows/release/`); a full Windows release build fails if WiX is unavailable (use
`--portable-only` to build just the portable zip). The previous Inno Setup installer has been
retired from the release flow — its scripts are kept, manual-only, under
`scripts/windows/legacy/`.

The current MSI is a file-install package — it bundles the canonical service scripts but does
not auto-install the Windows Service yet, and never touches `rules.json` or
`%ProgramData%\Portier`. Its install layout and config/data boundary are validated by
`npm run validate:msi:install` (a non-interactive `msiexec` smoke; no admin required — it
extracts via `msiexec /a` when not elevated, and does a full silent install/uninstall when
elevated). See `scripts/windows/release/readme.md`.

On **macOS**, a native **`.pkg`** is the v1.18 installer track, built on macOS by `pkgbuild`
(`scripts/macos/release/build-release.sh`); the portable tar.gz remains the baseline. The
current `.pkg` is a file-install package: it installs the runtime layout to `/usr/local/portier`
and bundles the canonical LaunchAgent scripts under `/usr/local/portier/service-scripts/`, but
does not auto-install the LaunchAgent yet and never touches `rules.json`,
`~/Library/Application Support/Portier`, or logs. It is unsigned/not notarized (Gatekeeper will
warn). `validate:release` reports `.pkg` presence on macOS (not fatal yet) and verifies its
checksum; a `.pkg` payload/install smoke is a follow-up. See `scripts/macos/readme.md`.

On **Linux**, the v1.18 release artifact is the **portable tar.gz**
(`portier-<version>-linux.tar.gz`, built by `scripts/linux/release/build-release.sh`), with
the **systemd** scripts under `scripts/linux/service/` as the canonical service layer. The
tar.gz contains the full runtime layout (incl. `api/openapi.json`); it is versioned,
checksummed in `SHA256SUMS`, and GitHub-Release-ready. Installed mode lives at `/opt/portier`
(binaries + `web/`), config at `/etc/portier/rules.json`, logs via journald, and the unit at
`/etc/systemd/system/portier.service`. `.deb`/`.rpm` packages are a **planned package-manager
track for a later v1.18 slice** (built/validated on native `ubuntu`/`fedora` runners once the
release CI lands) — not shipped in this slice. See `scripts/linux/readme.md`.

Each platform's release directory includes a `SHA256SUMS` file (GNU coreutils text
format, sorted, lowercase hex) covering every produced release artifact for that version —
portable archives and installer artifacts alike. `build:release` regenerates it after the
artifacts are built, and `validate:release` verifies it (every hash matches, every listed
file exists, and every produced artifact is listed). These checksums are the integrity
story for the currently unsigned artifacts; signing/notarization remain separate (see
below).

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
- The Windows MSI (WiX) is the canonical Windows installer (Inno Setup is retired to
  `scripts/windows/legacy/`, manual-only). The native macOS `.pkg` installer track is in
  progress for v1.18 (file-install; built on macOS). Linux ships the portable tar.gz in
  v1.18, with `.deb`/`.rpm` as a planned package track for a later slice. Do not add other
  platform package managers (Homebrew, winget, Chocolatey) without a deliberate product
  decision.
