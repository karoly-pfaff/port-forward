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
upgrades (the canonical Windows WiX/MSI, the macOS `.pkg`, and the Linux `.deb`/`.rpm`)
must satisfy.

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
  Portier-<version>.msi               (WiX — canonical Windows installer)
  portier-<version>-windows-amd64.zip
  checksums.sha256

build/releases/macos/
  Portier-<version>.pkg               (native installer — built on macOS, unsigned)
  portier-<version>-macos-amd64.tar.gz   (Intel)
  portier-<version>-macos-arm64.tar.gz   (Apple Silicon)
  checksums.sha256

build/releases/linux/
  portier_<version>_amd64.deb         (native dpkg-deb installer — file-install, disabled unit)
  portier-<version>-1.x86_64.rpm      (native rpmbuild installer — file-install, disabled unit)
  portier-<version>-linux-amd64.tar.gz
  portier-<version>-linux-arm64.tar.gz
  checksums.sha256
```

Portable artifacts carry the architecture in their name (no ambiguous single-name portable).
Each platform lists its native installer/package first, then the portable archive(s)
(amd64 before arm64), then `checksums.sha256`. Windows is amd64-only.

The **WiX MSI is the canonical Windows installer** (silent install, Group Policy/SCCM/Intune,
standard Add/Remove Programs + repair). It depends on the WiX Toolset v7
(`scripts/windows/release/`); a full Windows release build fails if WiX is unavailable (use
`--portable-only` to build just the portable zip). The previous Inno Setup installer has been
retired from the release flow — its scripts are kept, manual-only, under
`scripts/windows/legacy/`.

The current MSI is a file-install package — it bundles the canonical service scripts but does
not auto-install the Windows Service yet, and never touches `rules.json` or
`%ProgramData%\Portier`. It is validated by two smokes (both non-interactive `msiexec`):
`npm run validate:install:msi` (extraction via `msiexec /a`, no admin — validates layout,
version, config boundary) and `npm run validate:install:msi:full` (a full elevated per-machine
`msiexec /i` then `/x`). The full smoke additionally asserts that **no Windows service and no
scheduled task is created**, that `%ProgramData%\Portier\rules.json` is preserved across install
and uninstall, and that uninstall removes the install dir; it runs elevated on the `Release
Windows` runner and skips honestly when not elevated. The MSI has **no service custom
actions**. See `scripts/windows/release/readme.md`.

On **macOS**, a native **`.pkg`** is the v1.18 installer track, built on macOS by `pkgbuild`
(`scripts/macos/release/build-release.sh`); the portable tar.gz remains the baseline. The
current `.pkg` is a file-install package: it installs the runtime layout to `/usr/local/portier`
and bundles the canonical LaunchAgent scripts under `/usr/local/portier/service-scripts/`, but
does not auto-install the LaunchAgent yet and never touches `rules.json`,
`~/Library/Application Support/Portier`, or logs. It is unsigned/not notarized (Gatekeeper will
warn). `validate:release` reports `.pkg` presence on macOS (not fatal yet) and verifies its
checksum. The `Release MacOS` workflow also introspects the payload (`pkgutil --payload-files`)
and runs a native install/uninstall smoke (`npm run validate:install:pkg`) that installs the
`.pkg`, asserts the layout/version and that no LaunchAgent is loaded/started, then removes it
and confirms user config is preserved. See `scripts/macos/readme.md`.

On **Linux**, the v1.18 portable release artifacts are the **amd64 + arm64 tar.gz**
(`portier-<version>-linux-amd64.tar.gz`, `portier-<version>-linux-arm64.tar.gz`, built by
`scripts/build-portable.js`), with the **systemd** scripts under `scripts/linux/service/` as
the canonical service layer. The
tar.gz contains the full runtime layout (incl. `api/openapi.json`); it is versioned,
checksummed in `checksums.sha256`, and GitHub-Release-ready. Installed mode lives at `/opt/portier`
(binaries + `web/`), config at `/etc/portier/rules.json`, logs via journald, and the unit at
`/etc/systemd/system/portier.service`. Two native packages are also produced and validated on
the `ubuntu-latest` release-CI runner: a **`.deb`** (`portier_<version>_amd64.deb`, dpkg-deb)
and a **`.rpm`** (`portier-<version>-1.x86_64.rpm`, rpmbuild), both built by one unified script
(`scripts/linux/release/build-release.sh --format deb|rpm`). They are file-install packages that
mirror each other: they lay the runtime under `/opt/portier` and install a **disabled** systemd
unit — they never enable/start the service or touch user config. Each has a payload +
install/remove smoke (`validate:install:deb`, `validate:install:rpm:payload`,
`validate:install:rpm`). See `scripts/linux/readme.md`.

**Cross-platform portable generation.** The Go CLI/service are pure Go (CGO disabled), so all
five portable artifacts — `windows-amd64.zip`, `linux-amd64.tar.gz`, `linux-arm64.tar.gz`,
`macos-amd64.tar.gz`, `macos-arm64.tar.gz` — can be **cross-built from any host (including
Windows)** with `npm run build:release:portable:all` (cross-compiles `windows/amd64`,
`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, packages the full runtime layout
— Unix tarballs with correct exec bits, the Windows zip with `.exe` — and writes
`checksums.sha256`). `npm run validate:release:portable:all` validates them **structurally**
(layout, platform binary names, tar exec bits, **binary machine-type matches the named arch**,
`api/openapi.json`, checksums); `validate:release:portable:{windows,macos,linux}` scope to one
platform. This is for GitHub-Release readiness only — it does **not** run a runtime smoke
against foreign binaries; native runtime validation must run on each OS/arch.
`build:release:current` builds the host platform's portables (both arches on macOS/Linux) + the
native installer; the Windows installer (MSI) and macOS `.pkg` remain native-built.

**Native arm64 runtime smoke.** `darwin/arm64` and `linux/arm64` portables are runtime-smoked on
**real arm64 runners** (not emulation) by two manual workflows — `Smoke macOS arm64`
(`macos-14`/Apple Silicon) and `Smoke Linux arm64` (`ubuntu-24.04-arm`). Each extracts the
shipped arm64 portable and runs it (`npm run validate:portable:smoke`): `portier version`, GET
`/api/health`, GET `/api/runtime` version, plus a Mach-O/ELF machine-type + `uname -m` check (the
smoke fails rather than claim arm64 on non-arm64 hardware). amd64 portables are runtime-smoked on
the amd64 release runners. Windows arm64 is not built. Emulation is never counted as native.

Each platform's release directory includes a `checksums.sha256` file (GNU coreutils text
format `<sha256>  <filename>`, lowercase hex, native installer/package first then portable
archive) covering every produced release artifact for that version — portable archives and
installer artifacts alike. Verify with `sha256sum -c checksums.sha256`. `build:release` regenerates it after the
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

### Native Release CI (split per platform)

Three manual GitHub Actions workflows (`workflow_dispatch` only) reproduce the release
build + validation on native hosted runners and upload the platform's release directory as
a workflow artifact for inspection. None publish a GitHub Release or create tags. Each
workflow builds only its own platform's artifacts, package-first then portable then
`checksums.sha256`.

- **Release Windows** (`.github/workflows/release-windows.yml`, `windows-latest`): installs
  WiX 7, builds/validates the canonical MSI → portable zip → `checksums.sha256`, and runs both
  the MSI extraction smoke and the full elevated MSI install/uninstall smoke (no service/task
  creation; ProgramData config preserved).
- **Release MacOS** (`.github/workflows/release-macos.yml`, `macos-latest`):
  builds/validates the native `.pkg` (pkgbuild) → portable tar.gz → `checksums.sha256`,
  introspects the `.pkg` payload, and runs a native `.pkg` install/uninstall smoke
  (`installer -pkg` then remove; asserts layout, version, **no LaunchAgent loaded/started**,
  and user-config preservation). The `.pkg` is unsigned (see Signing And Notarization).
- **Release Linux** (`.github/workflows/release-linux.yml`, `ubuntu-latest`):
  installs `rpm` tooling, then builds/validates the native `.deb` (dpkg-deb) + `.rpm` (rpmbuild)
  → portable tar.gz (amd64 + arm64) → `checksums.sha256`, runs the native runtime smoke,
  introspects both package payloads, and runs `.deb` (`apt`) and `.rpm` (`rpm`) install/remove
  smokes — each asserting layout, version, the systemd unit is **disabled + inactive**, and
  `/etc/portier/rules.json` preservation. Full systemd service validation needs root and stays
  a manual/native check.

Each workflow uploads only its own `build/releases/<platform>/**`; `docs/private/**` is never
staged into release output and a privacy guard fails the run before upload if any private
path appears.

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
  `scripts/windows/legacy/`, manual-only). macOS ships a native file-install `.pkg` (unsigned,
  built on macOS) alongside the portable tar.gz, and Linux ships native file-install `.deb`
  and `.rpm` packages (disabled systemd unit) alongside the portable tar.gz — all with green
  install/remove smokes. Do not add other platform package managers (Homebrew, winget,
  Chocolatey) without a deliberate product decision.
