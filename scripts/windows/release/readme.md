# Portier Windows MSI (WiX)

WiX Toolset source for the Portier Windows **MSI** — the **canonical Windows installer**
(silent install, Group Policy/SCCM/Intune, standard Add/Remove Programs + repair).

The portable zip remains the universal baseline. The legacy Inno Setup installer has been
retired to `scripts/windows/legacy/` (manual-only; not built by the release flow).

## Status

- **File-install MSI.** Packages the same layout as the portable archive
  (`portier.exe`, `service.exe`, `server.js`, `web\`, `api\openapi.json`, `readme.txt`)
  into `%ProgramFiles%\Portier`, and bundles the canonical Windows service scripts under
  `%ProgramFiles%\Portier\service\`.
- **No Windows Service auto-install yet.** The MSI adds no service custom actions. An admin
  registers the service using the bundled canonical `service\install-service.ps1` (the
  single source of the service definition — no duplicated metadata in the MSI). Wiring the
  service through that script from the MSI is a follow-up.
- **Never touches user data.** The MSI does not create, overwrite, or migrate
  `rules.json` and does not touch `%ProgramData%\Portier`.

## Files

- `portier.wxs` — WiX 7 (v4 schema) source. `Files` elements harvest the packaged runtime
  and the service scripts. `UpgradeCode` is fixed — do not change it.
- `build-release.ps1` — resolves the `wix` tool (PATH, then `%USERPROFILE%\.dotnet\tools`),
  accepts the OSMF EULA, and builds `Portier-<version>.msi`.

## Prerequisites

- WiX Toolset v7 (`dotnet tool install --global wix`). Verify with `wix --version`.
- The packaged runtime at `build\portier\` (`npm run build:runtime`).

WiX v7 requires accepting FireGiant's Open Source Maintenance Fee (OSMF) EULA (free for
open-source use). `build-release.ps1` passes `-acceptEula wix7` so the build runs
non-interactively. See https://docs.firegiant.com/wix/osmf/.

## Build

The MSI is built automatically as part of the Windows release when WiX is available:

```powershell
npm run build:release:current
```

The MSI is the canonical Windows installer, so a full Windows `build:release` **fails** if
WiX is unavailable (use `--portable-only` to build just the portable zip).
Output: `build\releases\windows\Portier-<version>.msi`. It is included in `checksums.sha256` and
verified by `npm run validate:release:current`.

Build the MSI directly:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\windows\release\build-release.ps1
```

## MSI install smoke

A non-interactive MSI install/layout smoke validates that the MSI installs the expected
layout and keeps user config/data outside the install dir:

```powershell
npm run validate:msi:install
```

It auto-selects a mode by privilege:

- **Elevated** (or `--full-install`): real per-machine install to a temp `INSTALLFOLDER`
  via `msiexec /i ... /qn`, layout assertions, then `msiexec /x ... /qn` uninstall, asserting
  the install dir is removed.
- **Non-elevated** (default in CI/dev shells): `msiexec /a ... /qn TARGETDIR=<temp>`
  (administrative-install extraction) lays out the exact payload without admin and without
  touching the system; the register/uninstall half is reported as an honest skip.

Either mode asserts the installed layout (`portier.exe`, `service.exe`, `server.js`, `web\`,
`api\openapi.json`, `readme.txt`, bundled `service\*.ps1`), that `api\openapi.json` is valid
JSON whose `info.version` matches the package major.minor, that the installed `portier.exe`
reports the package version, that no `rules.json` is created inside the install dir, and that
a seeded external data dir (`rules.json` + backup/quarantine sentinels) is untouched.
Flags: `--msi <path>`, `--data-dir <dir>`, `--full-install`, `--keep-temp`.

## Validation gates the MSI must satisfy

Layout parity (incl. `api\openapi.json`), version reporting, `checksums.sha256` checksum
coverage, the MSI install smoke above, upgrade preservation
(`npm run validate:upgrade:current`), and — once service custom actions are wired — service
lifecycle.

## Native release CI

The **Release Windows** workflow (`.github/workflows/release-windows.yml`, manual
`workflow_dispatch`) installs WiX 7 as a dotnet global tool
(`dotnet tool install --global wix --version 7.*`, adding `%USERPROFILE%\.dotnet\tools`
to `PATH`), then runs `build:release:current` (MSI + portable zip),
`validate:release:current`, `validate:release:checksums`, `validate:runtime:smoke`,
`validate:upgrade:current`, and the non-elevated `validate:msi:install` (`msiexec /a`
extraction). It uploads `build/releases/windows/**` (MSI, portable zip,
`checksums.sha256`) as the `portier-release-windows` workflow artifact. It does not build
cross-platform portables, publish a GitHub Release, or create tags.
