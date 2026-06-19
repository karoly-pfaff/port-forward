# Portier QA Checklist

This document is the practical validation checklist for humans and coding agents.
It is intentionally current-state and task-oriented. Historical validation notes belong in
`docs/changelog.md`, `docs/roadmap.md`, or `audits/`.

## How To Use This Checklist

- Run the smallest relevant checks while developing.
- Broaden validation before merge when a change touches shared contracts, both runtimes,
  packaging, service scripts, or user-facing workflows.
- For docs-only changes, run markdown/whitespace checks and inspect links when practical.
- For release work, use the release candidate checklist, not only the pre-commit checklist.
- Always report what was run and what was skipped.

## Before A Local Commit

For most source changes:

- [ ] Review the diff and confirm it only contains intentional files.
- [ ] Check for unrelated generated output, local config, logs, or build artifacts.
- [ ] Run formatting if relevant:

```powershell
npm run format
```

- [ ] Run the narrowest relevant tests:

```powershell
npm run test:shared
npm run test:server
npm run test:client
npm run test:service
npm run test:cli
npm run test:replay
```

- [ ] Run static checks when TypeScript or shared code changed:

```powershell
npm run lint
npm run typecheck
```

- [ ] Run `git diff --check` before committing.

Docs-only changes:

- [ ] Keep docs current, not historical. Move release history to `docs/changelog.md`.
- [ ] Check links and headings touched by the change.
- [ ] Run:

```powershell
git diff --check -- docs
```

## Before Opening A PR Or Handing Off Agent Work

- [ ] Summarize changed files and why they changed.
- [ ] State validation commands run and results.
- [ ] State validation intentionally not run and why.
- [ ] Call out risk areas, follow-up work, or platform checks still needed.
- [ ] If API behavior changed, update `docs/api-contract.md`, the in-app API Docs view,
  and relevant tests.
- [ ] If user-facing terminology changed, check `docs/glossary.md`.
- [ ] If CLI behavior changed, update `tools/cli/readme.md` and black-box CLI tests.
- [ ] If package layout or service behavior changed, update `docs/installer.md` and
  platform docs under `scripts/<platform>/`.
- [ ] If OpenAPI changed, regenerate/release the API doc artifact as appropriate:

```powershell
npm run apidoc:generate
npm run apidoc:release
```

## Before Merge

Run the relevant subset first, then the broader commands if the change is not trivial.

Baseline:

```powershell
npm run lint
npm run typecheck
npm run test
npm run check
```

Build:

```powershell
npm run build
```

Contract/config parity when shared API, validation, config, rule lifecycle, diagnostics,
activity, status, group, health, OpenAPI, or runtime behavior changes:

```powershell
npm run validate:contract
npm run validate:config
npm run validate:openapi:go
```

> **`validate:contract` runs prebuilt artifacts** — the TypeScript server (`server/build/index.js`)
> and the Go service binary (`build/portier/service.exe`, or the platform equivalent). After
> changing `server/` or `service/` source, rebuild first or contract validation reads stale
> runtime output. Rebuild with `npm run build:runtime` (full, also refreshes the package), or
> targeted: `npm run build -w server` and `go -C service build -o build/portier/service.exe ./sources`.

Coverage when tests, gates, core behavior, or release readiness changes:

```powershell
npm run validate:coverage
```

Tooling:

```powershell
npm run validate:cli
npm run validate:replay
```

Browser E2E when the web UI or user-visible workflows change:

```powershell
npm run build:client
npm run test:e2e
```

Runtime smoke when packaging, static serving, Go service startup, bundled server, or
release layout changes. `validate:runtime:smoke` covers both normal startup and
configuration-recovery startup (boots a packaged runtime against a corrupt `rules.json`
and asserts `GET /api/runtime` reports `recovery.active`). To run only the recovery
scenario, use `validate:runtime:recovery-smoke`.

`validate:runtime` also asserts packaged version reporting: the bundled
`api/openapi.json` is valid JSON whose `info.version` matches the package major.minor,
the packaged CLI reports the package version (`portier version`), and the smoke run
asserts `GET /api/runtime` reports `version` equal to the package version.

```powershell
npm run validate:runtime:smoke
```

## Before A Version Release

Run this before tagging or publishing release artifacts.

### Required Validation

- [ ] `npm run lint`
- [ ] `npm run typecheck`
- [ ] `npm run test`
- [ ] `npm run build`
- [ ] `npm run check`
- [ ] `npm run validate:config`
- [ ] `npm run validate:contract`
- [ ] `npm run validate:openapi:go`
- [ ] `npm run validate:coverage`
- [ ] `npm run validate:cli`
- [ ] `npm run validate:replay`
- [ ] `npm run build:client`
- [ ] `npm run test:e2e`
- [ ] `npm run validate:runtime:smoke`
- [ ] `npm run validate:upgrade:current` (after `build:release:current`)

### Version Surfaces To Bump

When releasing a new version, update every surface below to the same version (none
are auto-derived; the two OpenAPI surfaces are hardcoded and easy to forget):

- [ ] `package.json` (root)
- [ ] `client/package.json`
- [ ] `server/package.json`
- [ ] `shared/package.json`
- [ ] `service/sources/version/version.go` (`Version`)
- [ ] `tools/cli/sources/version/version.go` (`Version`)
- [ ] `tools/replay/sources/version/version.go` (`Version`)
- [ ] `server/sources/openapi/openapi.ts` (`OPENAPI_DOC_VERSION`, minor `x.y` convention)
- [ ] `server/sources/api/runtime/runtime.schema.ts` (`version` example, full `x.y.z`)
- [ ] Regenerate the OpenAPI doc after bumping: `npm run apidoc:generate` (updates `server/build/api/openapi.json` + the tracked `docs/openapi.json`)

### Release Artifact Validation

- [ ] Build current-platform artifacts:

```powershell
npm run build:release:current
```

- [ ] Validate current-platform artifacts:

```powershell
npm run validate:release:current
```

- [ ] If only validating the portable artifact:

```powershell
npm run build:release:portable
npm run validate:release:portable
```

- [ ] Confirm generated artifacts contain the expected package layout:

```text
portier / portier.exe
service / service.exe
server.js
web/index.html
web/assets/
api/openapi.json
readme.txt
```

- [ ] Confirm artifacts do not contain `node_modules`, source trees, `rules.json`,
  coverage output, Playwright reports, local logs, or secrets.
- [ ] `validate:release` asserts the bundled `api/openapi.json` is present, non-empty,
  valid JSON, and its `info.version` matches the package major.minor (e.g. `1.18.0` →
  `1.18`). Packaged CLI/service version reporting is asserted by `validate:runtime:smoke`.
- [ ] `build:release` writes a `SHA256SUMS` file (GNU coreutils format, sorted, lowercase
  hex) next to the artifacts, covering every produced release artifact for the version
  (portable archives and installer artifacts alike, including the Windows `.msi`).
  `validate:release` requires it and verifies every listed hash matches, every listed file
  exists, and every produced artifact is listed — failing on a missing/malformed/duplicate
  entry or a hash mismatch. To verify checksums only:

```powershell
npm run validate:release:checksums
```

- [ ] Windows MSI (WiX) is the **canonical Windows installer**. A full Windows
  `build:release` builds `Portier-<version>.msi` and **fails if WiX 7 is unavailable**
  (use `--portable-only` to build just the portable zip). Windows release output is the
  MSI + portable zip + `SHA256SUMS`. `validate:release` requires the MSI on Windows and
  verifies its checksum. The current MSI is a file-install package (bundles the canonical
  service scripts, no Windows Service auto-install yet, never touches user config). The
  retired Inno installer lives in `scripts/windows/legacy/` (manual-only; not built or
  validated).
- [ ] Windows MSI install smoke (Windows-only; **no admin required**):

```powershell
npm run validate:msi:install
```

  Validates the MSI's installed layout, bundled service scripts, `api/openapi.json`
  (valid JSON + version), installed CLI version, and the config/data boundary (no
  `rules.json` inside the install dir; a seeded external data dir is untouched). It uses
  `msiexec /a` extraction when non-elevated and a full silent `msiexec /i`+`/x`
  install/uninstall when elevated (or `--full-install`). Not part of the cross-platform
  release matrix. See `scripts/windows/release/readme.md`.
- [ ] macOS `.pkg` (native installer track, **macOS-only**). On macOS, `build:release`
  also builds `Portier-<version>.pkg` via `pkgbuild` (skipped non-fatally if `pkgbuild` is
  absent). macOS release output is the `.pkg` + portable tar.gz + `SHA256SUMS`.
  `validate:release` reports `.pkg` presence on macOS (not fatal yet — the track is in
  progress) and verifies its checksum. The current `.pkg` is a file-install package
  (installs to `/usr/local/portier`, bundles the canonical LaunchAgent scripts, no
  LaunchAgent auto-install yet, unsigned, never touches user config). Payload introspection
  (`pkgutil --payload-files`) and a `.pkg` install smoke are follow-ups. See
  `scripts/macos/readme.md`.
- [ ] Linux release (Linux-only). `build:release` produces the portable
  `portier-<version>-linux.tar.gz` (full runtime layout, incl. `api/openapi.json`) **and**
  the native `portier_<version>_amd64.deb` (dpkg-deb; built on Debian/Ubuntu, skipped with a
  notice elsewhere) + `SHA256SUMS`. `validate:release` checks the tar.gz
  layout/OpenAPI/version, confirms the `.deb` is present, and verifies every checksum; the
  artifacts are versioned and GitHub-Release-ready. The `.deb` is a **file-install**: it lays
  the runtime under `/opt/portier` and installs a **disabled** systemd unit — it never
  enables/starts the service or touches user config (`scripts/linux/release/build-deb.sh`).
  Systemd is the canonical service layer (`scripts/linux/service/`); `install-service.sh
  --dry-run` prints the install plan without root. Full systemd install validation
  (`validate:service:linux`) requires `sudo`/a Linux host. `.rpm` is a planned later track.
  See `scripts/linux/readme.md`.
- [ ] Cross-platform portable artifact generation (from any host, incl. Windows). The Go
  binaries are pure Go (CGO disabled), so all three portable artifacts — Windows `.zip`,
  Linux `.tar.gz`, macOS `.tar.gz` — can be cross-built and structurally validated without a
  native runner:

```powershell
npm run build:runtime               # produce build/portier/ (neutral assets) once
npm run build:release:portable:all  # cross-build windows zip + linux/macos tar.gz (amd64)
npm run validate:release:portable:all
```

  `build:release:portable:all` cross-compiles `portier`/`service` for `windows`, `linux`, and
  `darwin` (amd64), packages the full runtime layout (Unix tarballs get **0755 exec bits** on
  the binaries; the Windows zip uses `.exe`), and writes per-platform `SHA256SUMS`.
  `validate:release:portable:all` validates each artifact **structurally** (layout, platform
  binary names, tar exec bits, `api/openapi.json` JSON + version, forbidden content incl.
  `docs/private`, checksum) using adm-zip / tar-stream — it does **NOT** run a runtime smoke
  against foreign binaries. **Native runtime smoke must still run on each OS**
  (`validate:runtime:smoke`); structural validation does not replace it. `build:release:current`
  still owns the current-platform release (portable + native installer, e.g. the MSI). arm64 is
  a documented follow-up. See `scripts/build-portable.js`.

- [ ] Run the upgrade-preservation smoke. It extracts the current-platform portable
  archive into a temp install dir, runs the packaged runtime against an external temp
  data dir, replaces the install dir with a freshly extracted copy (simulating an
  upgrade), and asserts user config (`rules.json`), the sentinel rule, and recovery
  backup/quarantine side files survive, that the runtime restarts healthy, reports the
  package version, and does not enter recovery mode for valid config. It is portable and
  current-platform for now (same-version replacement); it accepts `--from`/`--to` archives
  for future cross-version runs. No admin or OS-service install required.

```powershell
npm run validate:upgrade:current
```

- [ ] Confirm version numbers and release notes are correct.
- [ ] Confirm checksums/signing/notarization status is documented if applicable.

### Native Release CI Matrix (GitHub Actions)

A manual GitHub Actions workflow (`.github/workflows/release-matrix.yml`,
`workflow_dispatch` only) builds and validates the release artifacts on native
hosted runners and uploads them as **workflow artifacts** for inspection. It does
**not** publish a GitHub Release and does **not** create tags.

- [ ] Trigger it from the Actions tab ("release-matrix" → "Run workflow"). It has no
  push/PR triggers — manual only, so first-run issues stay controlled.
  Each job builds only its own platform's artifacts.
- [ ] `windows-release` (`windows-latest`): installs WiX 7 (dotnet global tool),
  builds/validates the canonical MSI + Windows portable zip, runs the non-elevated
  MSI install smoke (`msiexec /a` extraction; the `/i` + `/x` half is an honest skip
  without admin), and runs runtime + upgrade smoke. Uploads
  `build/releases/windows/**` as `portier-release-windows`.
- [ ] `macos-release` (`macos-latest`): builds/validates the native `.pkg` (pkgbuild)
  + macOS portable tar.gz, runs runtime + upgrade smoke, and introspects the `.pkg`
  payload (`pkgutil --payload-files`). Uploads `build/releases/macos/**` as
  `portier-release-macos`. The `.pkg` is unsigned (Gatekeeper warnings expected).
- [ ] `linux-release` (`ubuntu-latest`): builds/validates the native `.deb` (dpkg-deb)
  + Linux portable tar.gz, runs runtime + upgrade smoke, and introspects the `.deb`
  payload (`dpkg-deb --contents`). Uploads `build/releases/linux/**` as
  `portier-release-linux`. Full systemd service validation (`validate:service:linux`)
  needs root/systemd and is **not** run in CI — it stays a manual/native check.
- [ ] Privacy: only `build/releases/**` is uploaded. `validate:artifacts` logs the
  exact upload set and hard-fails on any `private` path segment before upload;
  `docs/private/**` is never staged into release output.

### Platform Service Validation

Run explicitly on target platforms. These are not part of `npm run check`.
Validation scripts use test-specific names, ports, and temp dirs; production installs are
not touched.

Windows:

```powershell
npm run validate:service:windows:user
npm run validate:service:windows:machine
```

macOS:

```bash
npm run validate:service:macos
```

Linux:

```bash
npm run validate:service:linux
```

Current platform helper:

```powershell
npm run validate:service:current
```

Each service validation should cover:

- [ ] Package copy to an isolated temp install directory.
- [ ] Service/task/agent registration with a test-specific name.
- [ ] Service start.
- [ ] `/api/health` responds.
- [ ] Web UI HTML is served at `/`.
- [ ] Service stop.
- [ ] Service/task/agent unregistration.
- [ ] Temp files cleaned up, unless `--keep-files` was requested.

Supported flags:

- `--no-build` / `-NoBuild`: use existing `build/portier/`.
- `--keep-files` / `-KeepFiles`: preserve temp files on failure.
- `--port` / `-Port`: override management port.

## Manual Human QA Before Public Distribution

Automation covers package layout, runtime smoke, protocol E2E, and service lifecycle.
Humans still need to check the platform behaviors automation cannot fully judge:

- [ ] Windows firewall prompt behavior and messaging.
- [ ] macOS Gatekeeper/quarantine/signing/notarization behavior.
- [ ] Linux service permission and distro-specific systemd behavior.
- [ ] Installer UX and uninstall/update clarity.
- [ ] Public release notes and upgrade guidance.
- [ ] Downloaded artifact trust story: signature, checksum, or explicit unsigned-build note.

## Change-Specific Checklist

Use these when the change touches the listed area.

### REST API Or Shared Contract

- [ ] Update shared types/validation.
- [ ] Update both TypeScript server and Go service when behavior is observable.
- [ ] Update CLI DTOs if the CLI reads the shape.
- [ ] Update `docs/api-contract.md`.
- [ ] Update in-app API Docs and tests.
- [ ] Run `npm run validate:contract`.
- [ ] Run `npm run validate:openapi:go` when route/status/OpenAPI inventory changes.

### Config Import/Export/Plan/Apply

- [ ] Preserve atomic validation before mutation.
- [ ] Preserve duplicate-binding behavior.
- [ ] Preserve dry-run and destructive-confirmation behavior.
- [ ] Update fixtures if compatibility behavior changes.
- [ ] Run `npm run validate:config` (also asserts R-1 recovery startup: a malformed/
  duplicate-binding config makes the runtime start in recovery mode, not abort —
  rebuild the runtimes first, as for `validate:contract`).
- [ ] Run `npm run validate:contract`.
- [ ] For offline normalization, `portier config migrate` is dry-run by default and
  backup-first on `--write`; never auto-migrate persisted config at startup.

### CLI

- [ ] Keep commands pure API clients unless explicitly offline.
- [ ] Preserve documented exit-code policy.
- [ ] Add/adjust black-box command tests.
- [ ] Update `tools/cli/readme.md`.
- [ ] Run `npm run validate:cli`.

### Replay Tool

- [ ] Keep replay offline and read-only.
- [ ] Do not contact runtime, execute workflows, mutate inputs, or upload data.
- [ ] Add/adjust replay tests.
- [ ] Update `tools/replay/readme.md` if behavior changes.
- [ ] Run `npm run validate:replay`.

### Web UI

- [ ] Use glossary-backed terminology.
- [ ] Preserve accessibility roles/labels.
- [ ] Prefer role/label selectors in tests.
- [ ] Run client tests.
- [ ] Run relevant E2E when user workflows change.

### Packaging Or Service Scripts

- [ ] Preserve external config; do not package user `rules.json`.
- [ ] Do not silently add firewall rules or system-level behavior.
- [ ] Keep test validation names/paths separate from production names/paths.
- [ ] Update `docs/installer.md` and platform readmes.
- [ ] Run `npm run validate:runtime:smoke`.
- [ ] Run `npm run validate:upgrade:current` when packaging/upgrade behavior changes
  (config/data must survive package replacement).
- [ ] Run platform service validation on the target OS when practical.

### Docs

- [ ] Keep current operational guidance in docs.
- [ ] Keep history in `docs/changelog.md`, `docs/roadmap.md`, or `audits/`.
- [ ] For audit publication, keep reviewed public reports in `audits/` and raw/internal
  notes in gitignored `audits/private/`.
- [ ] Before moving an audit into public `audits/`, check for secrets, local machine
  paths, raw config/log output, and sensitive security disclosure.
- [ ] Update `docs/glossary.md` for new public terms.
- [ ] Update this checklist if validation expectations change.
