# Portier Windows MSI (WiX)

WiX Toolset source for the Portier Windows **MSI** — the enterprise/admin installer
track (silent install, Group Policy/SCCM/Intune, standard Add/Remove Programs + repair).

The Inno Setup installer (`scripts/windows/release/`) remains the **default consumer
installer**. The portable zip remains the universal baseline. The MSI is additive and does
not replace either.

## Status (v1.18 spike)

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
- `build-msi.ps1` — resolves the `wix` tool (PATH, then `%USERPROFILE%\.dotnet\tools`),
  accepts the OSMF EULA, and builds `Portier-<version>.msi`.

## Prerequisites

- WiX Toolset v7 (`dotnet tool install --global wix`). Verify with `wix --version`.
- The packaged runtime at `build\portier\` (`npm run build:runtime`).

WiX v7 requires accepting FireGiant's Open Source Maintenance Fee (OSMF) EULA (free for
open-source use). `build-msi.ps1` passes `-acceptEula wix7` so the build runs
non-interactively. See https://docs.firegiant.com/wix/osmf/.

## Build

The MSI is built automatically as part of the Windows release when WiX is available:

```powershell
npm run build:release:current
```

MSI build failure is **non-fatal** — the portable zip and Inno installer are unaffected.
Output: `build\releases\windows\Portier-<version>.msi`. It is included in `SHA256SUMS` and
verified by `npm run validate:release:current`.

Build the MSI directly:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\windows\wix\build-msi.ps1
```

## Validation gates the MSI must satisfy

Layout parity (incl. `api\openapi.json`), version reporting, `SHA256SUMS` checksum
coverage, upgrade preservation (`npm run validate:upgrade:current`), and service lifecycle.
A non-interactive MSI install smoke (silent `msiexec` install into a temp prefix, asserting
layout + uninstall) is the next gate to add.
