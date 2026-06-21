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
- [ ] If API behavior changed, update `docs/api-contract.md`, the in-app API Reference view,
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

## Standing Release Guards (2.0 stable line)

Portier **v2.0.0** is released and tagged — the stable local-first milestone; see
[changelog.md](changelog.md) for what shipped. These guards remain the standing policy for any
change on top of the stable line, and the full release matrix below applies before any future tag:

- [ ] No version surface is bumped outside an explicit version release (the v2.0 bump is its own
  step; use `npm run version:set` / `version:bump`, never hand-edit).
- [ ] No API behavior, OpenAPI schema (beyond version metadata), or `rules.json` format change —
  unless a proven release blocker justifies it, documented if so.
- [ ] Client lint runs clean (React-hooks rules; `--max-warnings 0`); do not lower or bypass any
  lint gate.
- [ ] Client coverage gate reflects the measured actual (currently 100/100/100); never document an
  unverified coverage figure, and never lower a gate.
- [ ] The Postman collection stays **generated** from `docs/openapi.json` (not a hand-maintained
  second contract); `npm run validate:postman` passes and runs in push/PR CI; `npm run
  validate:postman:local` stays a local-only Newman smoke. No Postman cloud publishing.
- [ ] No automatic startup migration is added; release publishing stays an explicit manual step (no
  workflow auto-publishes a GitHub Release or auto-creates a tag).
- [ ] `docs/private/**` stays out of logs, artifacts, release output, and public docs.
- [ ] For docs/planning-only changes, run `npm run lint` + `npm run typecheck`; rerun heavy hosted
  release/smoke workflows only when build/release scripts change.

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
- [ ] `build:release` writes a `checksums.sha256` file (GNU coreutils format, sorted, lowercase
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
  MSI + portable zip + `checksums.sha256`. `validate:release` requires the MSI on Windows and
  verifies its checksum. The current MSI is a file-install package (bundles the canonical
  service scripts, no Windows Service auto-install yet, never touches user config). The
  retired Inno installer lives in `scripts/windows/legacy/` (manual-only; not built or
  validated).
- [ ] Windows MSI extraction smoke (Windows-only; **no admin required**):

```powershell
npm run validate:install:msi
```

  `msiexec /a` lays out the exact payload without admin and validates the installed
  layout, bundled service scripts, `api/openapi.json` (valid JSON + version), installed
  CLI version, and the config/data boundary (no `rules.json` inside the install dir; a
  seeded external data dir is untouched).
- [ ] Windows MSI full install/uninstall smoke (Windows-only; **needs an elevated shell**):

```powershell
npm run validate:install:msi:full
```

  Real per-machine `msiexec /i` then `/x`. Asserts the installed layout + CLI version,
  that **no Windows service and no scheduled task is created** (the MSI is file-install
  only — no service custom actions are wired), that `%ProgramData%\Portier\rules.json`
  is preserved across install **and** uninstall, and that uninstall removes the install
  dir. Honestly **skips** (exit 0) when not elevated; the `Release Windows` workflow runs
  it elevated. Service auto-install is still **not** wired. Not part of the cross-platform
  release matrix. See `scripts/windows/release/readme.md`.
- [ ] macOS `.pkg` (native installer track, **macOS-only**). On macOS, `build:release`
  also builds `Portier-<version>.pkg` via `pkgbuild` (skipped non-fatally if `pkgbuild` is
  absent). macOS release output is the `.pkg` + portable tar.gz + `checksums.sha256`.
  `validate:release` reports `.pkg` presence on macOS (not fatal — the portable tar.gz is
  the cross-host baseline) and verifies its checksum. The current `.pkg` is a file-install package
  (installs to `/usr/local/portier`, bundles the canonical LaunchAgent scripts, no
  LaunchAgent auto-install yet, unsigned, never touches user config).
- [ ] macOS `.pkg` install/uninstall smoke (macOS-only; needs sudo):

```bash
npm run validate:install:pkg
```

  Installs the `.pkg` with `installer -pkg … -target /`, asserts the installed layout
  (`/usr/local/portier/...` incl. bundled LaunchAgent scripts) and CLI version, that **no
  LaunchAgent is loaded/started** and a seeded `~/Library/Application Support/Portier`
  sentinel is untouched, then removes it (delete install dir + `pkgutil --forget`) and
  asserts clean removal with user data preserved. macOS-only (exits 0 with a skip notice
  elsewhere). See `scripts/macos/readme.md`.
- [ ] Linux release (Linux-only). `build:release` produces the native
  `portier_<version>_amd64.deb` (dpkg-deb) and `portier-<version>-1.x86_64.rpm` (rpmbuild —
  needs the `rpm` package; skipped with a notice when absent), then the portable
  `portier-<version>-linux-amd64.tar.gz` and `portier-<version>-linux-arm64.tar.gz` (full
  runtime layout, incl. `api/openapi.json`) + `checksums.sha256`. `validate:release` confirms
  the `.deb` is present, checks the tar.gz layout/OpenAPI/version, and verifies every checksum
  (covering the `.rpm` too); the artifacts are versioned and GitHub-Release-ready. Both
  packages are **file-install**, mirror each other: they lay the runtime under `/opt/portier`
  and install a **disabled** systemd unit — never enabling/starting the service or touching
  user config (one builder: `scripts/linux/release/build-release.sh --format deb|rpm`).
  Systemd is the canonical service layer (`scripts/linux/service/`); `install-service.sh
  --dry-run` prints the install plan without root. See `scripts/linux/readme.md`.
- [ ] Linux `.deb` / `.rpm` install/remove smoke (Linux-only; needs sudo):

```bash
npm run validate:install:deb   # apt-get install ./*.deb → assert → apt-get remove
npm run validate:install:rpm:payload   # rpm -qlp layout/forbidden-content check (needs the rpm CLI)
npm run validate:install:rpm   # rpm -i ./*.rpm → assert → rpm -e
```

  Each install smoke asserts the installed layout (`/opt/portier/...` +
  `/lib/systemd/system/portier.service`) and CLI version, that the systemd unit is
  **disabled and inactive** (the package never enables/starts it) and a seeded
  `/etc/portier/rules.json` sentinel is untouched, then removes the package and asserts the
  runtime files + unit are gone, the service is not running, and user config is preserved.
  Linux-only (exit 0 with a skip notice elsewhere). These are package-lifecycle smokes; full
  systemd service install validation (`validate:service:linux`, needs `sudo`/systemd PID 1)
  stays a separate manual/native check. See `scripts/linux/readme.md`.
- [ ] Cross-platform portable artifact generation (from any host, incl. Windows). The Go
  binaries are pure Go (CGO disabled), so all five portable artifacts can be cross-built and
  structurally validated without a native runner:
  - `portier-<version>-windows-amd64.zip`
  - `portier-<version>-linux-amd64.tar.gz`, `portier-<version>-linux-arm64.tar.gz`
  - `portier-<version>-macos-amd64.tar.gz`, `portier-<version>-macos-arm64.tar.gz`

```powershell
npm run build:runtime               # produce build/portier/ (neutral assets) once
npm run build:release:portable:all  # cross-build windows amd64 + linux/macos amd64+arm64
npm run validate:release:portable:all
```

  `build:release:portable:all` cross-compiles `portier`/`service` for `windows/amd64`,
  `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`, packages the full runtime
  layout (Unix tarballs get **0755 exec bits** on the binaries; the Windows zip uses `.exe`),
  and writes per-platform `checksums.sha256`. `validate:release:portable:all` validates each
  artifact **structurally** (layout, platform binary names, tar exec bits, **binary
  machine-type matches the named arch** (ELF/Mach-O/PE), `api/openapi.json` JSON + version,
  forbidden content incl. `docs/private`, checksum) — it does **NOT** run a runtime smoke
  against foreign binaries. **Native runtime smoke must still run on each OS/arch**
  (`validate:runtime:smoke`); structural validation does not replace it. Per-platform variants
  (`validate:release:portable:{windows,macos,linux}`) validate just that platform's arches.
  `build:release:current` builds the host platform's portables (both arches on macOS/Linux) +
  the native installer; only the host arch is runtime-smoked. See `scripts/build-portable.js`.
- [ ] Native arm64 portable runtime smoke (real arm64 runners — not emulation). Two manual
  workflows run the shipped arm64 portable natively and assert `portier version`, `/api/health`,
  and `/api/runtime` version, plus the Mach-O/ELF machine type and `uname -m` (the smoke
  refuses to call itself arm64 unless the host really is):
  - **Smoke MacOS arm64** (`.github/workflows/smoke-macos-arm64.yml`, `macos-14` / Apple Silicon)
  - **Smoke Linux arm64** (`.github/workflows/smoke-linux-arm64.yml`, `ubuntu-24.04-arm`)

```bash
npm run validate:portable:smoke   # extracts + runs the HOST-arch portable (native arch only)
```

  This replaces "structural-only" confidence for `linux/arm64` and `darwin/arm64` with real
  native runtime coverage. amd64 portables remain runtime-smoked on the amd64 release runners;
  Windows arm64 is not built. Emulation is never counted as native. These stay isolated workflows
  (not extra jobs in the release workflows) because the arm64 runtime needs a different-arch runner
  than the amd64 release build, scarcer arm64 capacity should not block the amd64 release, and
  Windows needs none (it ships amd64-only).

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

### Push/PR CI vs manual release workflows

Two distinct kinds of GitHub Actions workflow:

- **Push/PR CI** (`.github/workflows/portier-ci.yml`, automatic on `push` to `main` and on
  `pull_request`): fast everyday-development checks. A `ubuntu-latest` **baseline** job runs
  `npm ci`, `npm run lint` (`--max-warnings 0`), `npm run typecheck`, `npm run test` (shared +
  server + client + Go service), the honest client coverage gate
  (`npm run validate:coverage:client`), `npm run validate:scripts`, and the Go↔OpenAPI
  inventory check (`npm run validate:openapi:go`); a lean `windows-latest` job runs
  lint/typecheck/script-sanity to catch Windows-specific path issues. It builds **no** release
  packages, uploads **no** artifacts, and **never** publishes a Release or creates a tag.
  (Full Playwright E2E and `validate:contract` are intentionally left to the manual pre-release
  matrix / local runs — too slow for every push.)
- **Manual release workflows** (`workflow_dispatch` only — Release Windows / MacOS / Linux +
  the two arm64 smokes): build and validate platform packages and upload release artifacts.
  These remain manual; full E2E and release packaging do not run on every push.

### Native Release CI (GitHub Actions, split per platform)

Three manual GitHub Actions workflows (`workflow_dispatch` only) build and validate the
release artifacts on native hosted runners and upload them as **workflow artifacts** for
inspection. None publish a GitHub Release or create tags. Each workflow builds only its
own platform's artifacts, in **package-first** order (installer, then portable archive,
then `checksums.sha256`).

- [ ] **Release Windows** (`.github/workflows/release-windows.yml`, `windows-latest`):
  installs WiX 7 (dotnet global tool); builds/validates `Portier-<version>.msi` (canonical)
  → `portier-<version>-windows-amd64.zip` → `checksums.sha256` (incl. structural portable
  validation); runs the MSI extraction smoke (`msiexec /a`) **and** the full elevated MSI
  install/uninstall smoke (`msiexec /i` + `/x`, asserting no service/task creation and
  ProgramData config preservation), plus runtime + upgrade smoke. Uploads
  `build/releases/windows/**` as `portier-release-windows`. (Windows is amd64-only.)
- [ ] **Release MacOS** (`.github/workflows/release-macos.yml`, `macos-latest`):
  builds/validates `Portier-<version>.pkg` (pkgbuild, unsigned) →
  `portier-<version>-macos-amd64.tar.gz` + `portier-<version>-macos-arm64.tar.gz` →
  `checksums.sha256` (incl. structural validation of both arches); runs runtime + upgrade
  smoke (host arch), `.pkg` payload introspection (`pkgutil --payload-files`), and the `.pkg`
  install/uninstall smoke (`validate:install:pkg`). Uploads `build/releases/macos/**` as
  `portier-release-macos`.
- [ ] **Release Linux** (`.github/workflows/release-linux.yml`, `ubuntu-latest`):
  installs `rpm` tooling, then builds/validates `portier_<version>_amd64.deb` (file-install,
  disabled unit) → `portier-<version>-1.x86_64.rpm` (file-install, disabled unit) →
  `portier-<version>-linux-amd64.tar.gz` + `portier-<version>-linux-arm64.tar.gz` →
  `checksums.sha256` (incl. structural validation of both arches); runs runtime + upgrade
  smoke (host arch), `.deb` + `.rpm` payload introspection, and the `.deb` (`validate:install:deb`)
  and `.rpm` (`validate:install:rpm`) install/remove smokes (each asserts disabled+inactive
  unit and config preservation).
  Uploads `build/releases/linux/**` as `portier-release-linux`. Full systemd service
  validation (`validate:service:linux`) needs root/systemd and is **not** run in CI — it
  stays a manual/native check.
- [ ] Privacy: each workflow uploads only its own `build/releases/<platform>/**`.
  `validate:artifacts` logs the exact upload set (package-first, `checksums.sha256` last)
  and hard-fails on any `private` path segment before upload; `docs/private/**` is never
  staged into release output.

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
- [ ] Update the in-app API Reference (OpenAPI-driven) and tests.
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
