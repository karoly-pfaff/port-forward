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
release layout changes:

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
- [ ] Confirm version numbers and release notes are correct.
- [ ] Confirm checksums/signing/notarization status is documented if applicable.

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
- [ ] Run `npm run validate:config`.
- [ ] Run `npm run validate:contract`.

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
