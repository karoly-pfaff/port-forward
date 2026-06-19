# Legacy: Inno Setup installer (retired)

**Status: legacy / manual-only. Not built by the release flow.**

The canonical Windows installer is now the **WiX MSI** (`scripts/windows/release/`,
`Portier-<version>.msi`). The Inno Setup installer has been retired from the default
release build (`scripts/build-release.js` no longer invokes it) and is kept here only as a
migration reference.

These files are **not** part of `npm run build:release:current` and are not validated by
`npm run validate:release:*`. They are preserved so the Inno install/service definition can
be referenced while wiring equivalent behavior into the MSI.

Do not reintroduce the Inno build into the default release flow. New Windows installer work
belongs in the WiX MSI. See `scripts/windows/release/readme.md` and `docs/installer.md`.

## Contents

- `portier.iss` — Inno Setup 6 script (previous consumer installer).
- `build-release.ps1` — Inno build wrapper (`ISCC.exe`).

Building manually (requires Inno Setup 6) is possible from this directory but unsupported.
