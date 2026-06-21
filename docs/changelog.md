# Changelog

All notable changes to Portier are documented here.

This changelog is written for both humans and coding agents. It summarizes what changed, why it matters, and the validation signal for each release. Detailed implementation history, audit notes, and commit-level rationale live in `audits/` and Git history.

## [2.0.0] - 2026-06-21 - Stable Local-First Release

Portier 2.0 is the first stable local-first release. It closes the road-to-2.0 arc — migration & recovery (v1.17), install/service/upgrade (v1.18), RC hardening (v1.19), and the 2.0-RC documentation cleanup. This is a stability and packaging milestone, not a redesign: the final bump changes only version metadata. The REST API, the OpenAPI schema (beyond its version), the persisted `rules.json` format, and CLI behavior are all unchanged from v1.19, no migration runs, and no coverage or lint gate was lowered.

What Portier 2.0 is:
- A stable, **local-first** TCP/UDP port-forwarding manager for development and LAN testing. The management UI/API stays on `127.0.0.1:47831` by default; no telemetry, cloud sync, auto-update, or remote/team/auth.
- **Go service** — the preferred production runtime (small binary, no Node.js dependency, no warm-up).
- **TypeScript/NestJS server** — the supported reference/fallback runtime, behind the same REST API contract.
- **React management UI** — Dashboard, Forward Rules, Activity Log, Live Connections, Settings, and an OpenAPI-driven in-app API Reference.
- **TCP and UDP forwarding**, including the three UDP modes (one-way, bidirectional-last-client, bidirectional-multi-client).
- **Declarative config** import / export / plan / apply with drift control and destructive-change confirmation.
- The Go `portier` **CLI** (a pure API client) and the separate offline `replay` analysis tool.
- **Native installers and portables** — file-install Windows MSI, macOS `.pkg`, and Linux `.deb`/`.rpm`, plus arch-suffixed portable archives, each covered by a `checksums.sha256` manifest. Installers never enable or start a service, create a scheduled task, or touch user config.
- **Documented, tested upgrade path** — in-place v1.x → v2.0 ([upgrade-v2.md](upgrade-v2.md)) with startup/config recovery ([recovery.md](recovery.md)); `rules.json` is preserved and never rewritten on startup.
- **Consumer artifacts** — a canonical `docs/openapi.json` and a generated, drift-checked Postman collection (`postman/`).

Compatibility:
- No API behavior change, no OpenAPI schema change beyond version metadata (1.19 → 2.0), no `rules.json` format change, and no startup migration in this release.

Known non-blocking deferrals (tracked as post-2.0 themes): code signing / notarization, automated GitHub Release publishing, arm64 **native** packages (arm64 ships as validated portables), service auto-install from installers (file-install stays by design), and remote/team/auth management. See [roadmap.md](roadmap.md#post-20-directions).

- **Validation signal:** lint / typecheck clean; full unit suites (shared / server / client / Go service / CLI / replay); client coverage held at **100 / 100 / 100** with no gate lowered; `validate:contract` 237/0/0; `validate:openapi:go` PASS; `validate:config` PASS; `validate:postman` 129/0 plus a live Newman smoke; current-platform release build/validate + runtime, recovery, and upgrade smokes green; full Playwright E2E; `version:check` 10/10 at `2.0.0` (OpenAPI `2.0`). Cross-platform packages and native arm64 runtime smokes are produced and validated by the manual per-platform release workflows.

## [1.19.0] - 2026-06-21 - 2.0 RC Hardening

A release-candidate hardening milestone — stabilization only, no new features, no new installer formats, and no new service lifecycle behavior. The persisted `rules.json` format and the REST API are unchanged; the only OpenAPI change is the version metadata (1.18 → 1.19). No coverage or lint gate was lowered.

- **Client quality gates:** React + `react-hooks` linting (`rules-of-hooks` + `exhaustive-deps` as errors), repo-wide `@typescript-eslint/no-explicit-any` as an error, and `eslint . --max-warnings 0`. Client coverage reached and is held at **100 / 100 / 100**.
- **CI:** a fast push/PR CI workflow that runs the real quality gates and never builds packages, uploads artifacts, publishes, or tags. The five release/smoke workflows stay manual (`workflow_dispatch`).
- **Versioning:** single-source version tooling (`version:set`/`bump`/`check`/`list`) with a CI drift guard across all 10 version surfaces.
- **API Reference:** the in-app API view is now generated from the canonical `docs/openapi.json` (no hand-maintained endpoint table).
- **Postman:** a `postman/collection.json` + `environment.json` generated from the OpenAPI contract and drift-checked in CI (`validate:postman`), plus a local-only Newman runtime smoke (`validate:postman:local`) that runs the collection against a live runtime and self-cleans.
- **UI:** design-token consolidation, Title-Case chrome alignment, Live Connections summary cards, Activity Log naming, and a simplified status column (errors surface in Health + Activity Log).
- **Runtime fixes:** UDP expired-session pruning is now wired into the Go UDP forwarder (one-way sessions no longer grow unbounded), and the Go generic `500` path redacts internal error text to match the NestJS error envelope.
- **Validation signal:** lint / typecheck clean; shared 105, server 638, client 570 (100/100/100), Go service 90.4% (gate 90/95); `validate:contract` 237/0/0; `validate:openapi:go` PASS; `validate:postman` 129/0; live Newman smoke green; full Playwright E2E 45/45; `version:check` 10/10.

## [1.18.0] - 2026-06-20 - Install, Service & Upgrade Experience

The install, service, and upgrade release: native installers plus validated, checksummed release artifacts for all three platforms, and an upgrade path that preserves user config. The persisted `rules.json` format, the REST API, and the OpenAPI schema are unchanged; no installer enables or starts a service on its own, and no coverage gate was lowered.

Native installers (all **file-install** — they lay down the runtime and never enable/start a service, create a scheduled task, or touch user config):
- **Windows:** the WiX **MSI** is now the canonical Windows installer (silent install, Group Policy/SCCM/Intune, Add/Remove Programs, repair); the legacy Inno Setup installer is retired to `scripts/windows/legacy/` (manual-only). Ships with the portable zip.
- **macOS:** an unsigned `.pkg` (pkgbuild) installing to `/usr/local/portier` and bundling the LaunchAgent scripts; not notarized (Gatekeeper warns).
- **Linux:** native `.deb` (dpkg-deb) and `.rpm` (rpmbuild) that mirror each other — runtime under `/opt/portier` with a **disabled** systemd unit; config at `/etc/portier/rules.json` is preserved.

Release artifacts:
- Portable archives for Windows amd64, macOS amd64/arm64, and Linux amd64/arm64, each carrying the architecture in its name and structurally validated (layout, exec bits, and a PE/ELF/Mach-O machine-type check that the binary matches the named arch).
- A `checksums.sha256` manifest (GNU coreutils text format) covering every artifact, ordered package/installer first, portables second, checksums last. (This replaces the earlier `SHA256SUMS` name.)

Install & upgrade validation:
- Native install/remove smokes for every package — Windows MSI full elevated install/uninstall, macOS `.pkg`, Linux `.deb`, and Linux `.rpm` — each asserting the installed layout, version, config preservation across install and removal, and that no service/agent/task is enabled or started.
- Native **arm64 runtime smoke** on real arm64 hardware (macOS Apple Silicon and Linux arm64): `portier version`, `/api/health`, and `/api/runtime` against the shipped arm64 portable, refusing to claim arm64 unless the host really is. Emulation is never counted as native.
- An upgrade-preservation smoke replaces the install directory with a fresh extraction and asserts user config, rules, and recovery side files survive while the runtime restarts healthy.

Release CI:
- Three split, manual (`workflow_dispatch`) GitHub Actions workflows — **Release Windows**, **Release MacOS**, **Release Linux** — build and validate each platform's artifacts on native hosted runners and upload them for inspection, plus two manual arm64 smoke workflows. None publish a GitHub Release or create tags. Each run uploads only its own `build/releases/<platform>/**`, and a privacy guard fails the run before upload if any private path would be staged.

Deferred (non-blocking follow-ups):
- Code signing / notarization (Windows Authenticode, macOS Developer ID + notarization).
- Automated GitHub Release publishing and tagging.
- arm64 **native packages** (`.pkg`/`.deb`/`.rpm`); arm64 ships as validated portables.
- Auto-installing the OS service from the installer (MSI service custom actions, `.pkg` LaunchAgent load, package systemd enable) — all installers stay file-install by design.
- Deeper opt-in service-lifecycle smokes (full root/systemd start-stop) remain manual/native checks.
- Other package managers (Homebrew, winget, Chocolatey).

Validation:
- lint, typecheck, full unit/contract/coverage suites (no gate lowered), runtime + recovery smoke, current-platform release build/validate, and the per-platform release + arm64 smoke workflows all green.

## [1.17.0] - 2026-06-18 - Migration & Recovery

A reliability release that resolves the v1.16 audit finding **R-1** (fatal startup lockout): a bad or unbindable configuration no longer kills the service. Portier now starts in a recoverable, observable state across both runtimes (Go service and TypeScript/NestJS), and gains an offline config migration/normalization command. No installer/upgrade work (v1.18) or lint/security hardening (v1.19) was started; no coverage gate was lowered.

Startup recovery (R-1 resolved, Go + TypeScript/NestJS):
- A malformed, schema-invalid, or unreadable `rules.json` no longer aborts startup — the management API stays reachable with no active rules.
- Bad config is preserved: malformed/schema-invalid files are quarantined in place to a timestamped `rules.json.corrupt-<UTC>` sibling (never overwriting a prior quarantine); an unreadable file is left untouched. Writes are blocked while recovery is active, so a fresh empty config can never silently overwrite the bad one.
- Persisted duplicate listen bindings no longer abort startup: the rules load, conflicting enabled rules are skipped at autostart (no arbitrary winner) and reported per-rule, and unrelated rules still start.
- A per-rule autostart bind failure is non-fatal: the rule is left enabled-but-stopped with `lastError`/`health: "error"` while other rules and the API start. Create/update/import duplicate validation stays strict.

Recovery surfacing (additive, backward-compatible):
- `GET /api/runtime` carries an always-present `recovery` block (`{ active: false }` normally; reason/message/configPath/quarantinePath/writesBlocked/detectedAt when active), identical across runtimes and documented in OpenAPI.
- Diagnostics/support bundle includes the recovery state via `runtime.json`; `portier doctor` emits a `config.recovery_active` warning when active; the web UI shows a recovery banner only when active. Per-rule autostart/duplicate failures stay rule-level and do not trigger the global banner.

Runtime validation:
- `validate:runtime:smoke` now covers normal **and** configuration-recovery startup; a packaged-runtime recovery smoke boots the Go service against a corrupt `rules.json` and asserts `/api/runtime.recovery.active`. `validate:config` was updated to assert recovery-mode startup (a bad config no longer aborts).

Config migration & versioning:
- New offline `portier config migrate <file>`: classifies the file (bare-array / wrapper-object / exported), validates, and normalizes a valid config to the canonical bare-array shape. Dry-run by default; `--write` is **backup-first** (`<file>.bak-<UTC>`) and atomic. A malformed, schema-invalid, or unsupported-version file is never written or overwritten.
- The persisted `rules.json` remains a backward-compatible unversioned bare array (no startup auto-migration); the export/import envelope version remains `"1"`; unsupported/future envelope versions are refused.

Validation:
- lint, typecheck, full unit tests (shared/server/client/service), CLI and replay tests, all coverage gates (server/service/client/cli — none lowered), config compatibility (`validate:config`), contract parity (`validate:contract`, 237 checks), Go OpenAPI inventory, runtime smoke + recovery smoke, and current-platform release artifact build/validate all passed.
- Known caveat (pre-existing, unchanged since v1.14): the Playwright E2E web server fails to boot because it launches the NestJS server via `tsx` from the repo root, where `tsconfig.json` lacks `experimentalDecorators`. Product behavior is covered by contract parity against the built server, runtime/recovery smoke, and the full unit suites. Tracked as a follow-up.

Deferred follow-ups (unchanged scope boundaries):
- Install / service / upgrade experience → v1.18.
- Go generic-500 message redaction and strict lint hardening → v1.19.
- A versioned *persisted* config envelope and cross-version migration steps → future (not needed at a single config version).

## [1.16.0] - 2026-06-17 - Post-Migration Architecture & Reliability Audit

A hardening release, not a feature release: a ten-audit review of Portier after the v1.14 NestJS migration and v1.15 Go chi-router migration, plus a synthesis-and-fix-plan and a small docs-polish pass. No API/contract behavior changed, no coverage gate was lowered, and no installer/upgrade work was moved into v1.16.

Audits completed (verdicts): Contract Parity (PASS), Architecture & Module Boundary (PASS), Resilience & Data Durability (PASS WITH NOTES), Security & Local-Safety (PASS WITH NOTES), Observability & Replay (PASS WITH NOTES), Automation/Policy/Workflow (PASS WITH NOTES), Testing & Coverage (PASS WITH NOTES), Complexity/Duplication/Maintainability (PASS WITH NOTES), Documentation/Glossary/Operator UX (PASS WITH NOTES), Release Readiness & Packaging (PASS WITH NOTES). Result: 3 PASS, 7 PASS WITH NOTES, 0 FAIL, 0 release blockers. A synthesis-and-fix-plan consolidated and classified all findings.

Docs polish (Option A — no behavior change):
- API error/method conventions documented (`{ "errors": [...] }` envelope; unknown `/api/*` and wrong-method both return the 404-not-405 envelope; Go-only `GET /api/health` vs NestJS-only `GET /health`; generic-500 behavior and the v1.19 redaction follow-up).
- Docs index navigation completed (roadmap, CLI/replay, service/server readmes).
- Reserved `policy.Warning` severity note added to the CLI docs.
- Config-export topology-sensitivity warning added to the API contract docs.
- Release "Version Surfaces To Bump" checklist added; `api/openapi.json` added to the documented package-layout block.

Validation:
- lint, typecheck, full unit tests, CLI tests, contract parity, OpenAPI inventory, all coverage gates (no gate lowered), config compatibility, CLI/replay validation, build, and runtime smoke passed.
- Known caveat: the Playwright E2E web server currently fails to boot because it launches the NestJS server via `tsx` from the repo root, where `tsconfig.json` lacks `experimentalDecorators` (pre-existing since the v1.14 NestJS switch; product behavior is validated by contract parity against the built server and by runtime smoke). Tracked as a follow-up.

Deferred follow-ups (no v1.16 blockers):
- R-1 fatal autostart / corrupt-config startup lockout → v1.17.
- O-1 unwired UDP `PruneExpired` / one-way session registry growth → v1.17 or a dedicated fix slice.
- S-1 Go generic-500 message redaction (parity with the NestJS fixed message) → v1.19.
- L-1 strict lint hardening → v1.19.
- Install / service / upgrade experience → v1.18.

## [1.15.0] - 2026-06-17 - Go Service Modular Router

The native Go service API layer was reorganized from a monolithic dispatcher into focused route modules on `github.com/go-chi/chi/v5`, behind the explicit `app.App` dependency container.

Highlights:
- The Go service remains the preferred packaged runtime.
- All API routes are mounted through chi feature modules for health, runtime, ports, activity, status, connections, forwards, and config.
- The old ordered API dispatch path was removed; `api.go` now owns handler setup, API/static routing, and generic API 404 behavior.
- API and static-file boundaries are unchanged: Portier still routes only `/api` requests through the API router.
- Wrong methods on known API paths still return Portier's JSON 404 envelope, preserving previous behavior.
- Encoded group and rule path behavior is guarded so `%2F` and other encoded segments keep their intended meaning.
- A Go OpenAPI inventory check was added to catch route/status drift against the generated OpenAPI artifact.

Validation:
- API contract parity stayed green.
- Go build, vet, unit tests, OpenAPI inventory, runtime smoke, lint, typecheck, and coverage gates passed.
- No API response shape, error envelope, packaging, or runtime behavior changed.

## [1.14.1] - 2026-06-16 - Restore Config Export Activity

Restored the live `config.exported` activity event emitted by `GET /api/config/export`.

Highlights:
- Export response shape stayed unchanged.
- Import, plan, apply, CLI, replay, and runtime behavior were not changed.
- Version metadata was bumped for the patch release.

Validation:
- Contract and targeted server behavior checks passed.

## [1.14.0] - 2026-06-16 - TypeScript Server Migration To NestJS

The TypeScript server moved from Express to NestJS while preserving the existing REST API contract.

Highlights:
- The packaged `server.js` now boots the NestJS server.
- Express runtime dependencies were removed from the TypeScript server path.
- Controllers and providers now own the TypeScript server API structure.
- OpenAPI generation was wired into the NestJS server surface.
- The Go service remained the preferred production runtime and continued to share the same API contract.

Validation:
- Contract parity, server tests, lint, typecheck, build, OpenAPI generation, and runtime smoke passed.
- Server coverage gates remained strong.

## [1.13.0] - 2026-06-14 - Local Replay And Offline Analysis

Added a standalone offline replay tool for inspecting Portier exports without contacting a running service.

Highlights:
- New `tools/replay` workspace package.
- Commands include `plan`, `analyze`, `timeline`, `compare`, and `explain`.
- Replay works from local exported data only.
- The tool avoids shell execution, network access, service mutation, and live runtime side effects.
- Replay received its own tests and validation path.

Validation:
- Replay tests and coverage passed.
- Existing API contract and runtime validation remained green.

## [1.12.0] - 2026-06-14 - Local History And Observability

Added opt-in local workflow history for CLI and workflow tooling.

Highlights:
- History commands can list, show, export, summarize, prune, and clear local records.
- Stored records are compact metadata, not raw configs or secrets.
- History remains local and does not contact a remote service.
- The feature supports auditability without changing the Portier runtime API.

Validation:
- CLI and history tests passed.
- Existing contract and runtime validation remained green.

## [1.11.0] - 2026-06-13 - Local Intelligence And Workflow Automation

Added local workflow helpers for planning, templates, runbooks, reports, and AI handoff prompts.

Highlights:
- Workflow commands help generate reviewable plans and reports.
- Templates and runbooks make repeatable operational work easier.
- The tooling is local-first and conservative: no scheduler, no shell execution, and no automatic mutation.
- AI handoff output is intended to help agents understand the next safe action.

Validation:
- Workflow and CLI tests passed.
- Existing API and runtime checks remained green.

## [1.10.0] - 2026-06-13 - Automation, Policies, And Safe Operations

Added policy and guardrail tooling for safer local operations.

Highlights:
- Policy checks can run offline or against a read-only runtime view.
- Commands cover policy review, templates, baseline creation, baseline comparison, explanations, and report export.
- Policy tooling reports issues; it does not enforce, mutate, or schedule operations.
- CLI packaging and command organization were cleaned up.

Validation:
- CLI policy tests passed.
- Existing contract and runtime validation remained green.

## [1.9.0] - 2026-06-13 - Doctor And Config Toolkit

Added doctor-style diagnostics and safer config inspection commands.

Highlights:
- Doctor commands can inspect runtime health, explain findings, emit stricter output, and build support bundles.
- Config commands can summarize and inspect local or runtime configuration.
- AI prompt output helps hand off diagnostic context to an agent.
- Diagnostic tooling is read-only unless the user explicitly runs an existing mutating command.

Validation:
- CLI diagnostic tests passed.
- Existing API contract and runtime validation remained green.

## [1.8.0] - 2026-06-11 - Operator Power Tools

Added rule grouping and operator-focused rule management.

Highlights:
- Forward rules gained optional `group` metadata.
- Group start/stop APIs were added in both runtimes.
- CLI commands can list, start, and stop groups.
- The UI can edit, display, filter, and act on groups.
- Health, duplicate-rule visibility, config preview, action menus, and table affordances were improved.
- Group metadata is behavior-neutral except where group operations explicitly act on it.

Validation:
- TypeScript and Go contract parity covered group metadata and group actions.
- Client, CLI, service, and E2E checks passed for the new group flows.

## [1.7.0] - 2026-06-11 - Cleanup And Maintainability

Focused on maintainability without changing user-facing behavior.

Highlights:
- Contract test child-process cleanup was hardened.
- Go forwarding lifecycle now uses a small `Forwarder` interface and factory.
- UDP activity-event construction in the TypeScript server was centralized.
- Diagnostic checks were split into named, ordered phases in both runtimes.
- CLI config loading and exit-code policy were normalized.
- Settings UI was decomposed into smaller panels and hooks.
- UI wording was aligned with the glossary.

Validation:
- Contract parity, CLI tests, service tests, client tests, and relevant E2E checks passed.

## [1.6.0] - 2026-06-11 - Architecture And Quality Audit

Completed a broad architecture, coverage, and parity audit.

Highlights:
- Coverage gates and validation expectations were strengthened.
- TypeScript rollback, duplicate-binding behavior, atomic config writes, and activity parity were hardened.
- Config apply orchestration was moved beside the plan engine.
- Go rule ID generation and manager activity emission were deduplicated.
- The glossary became the source of canonical product terminology.

Validation:
- Contract parity and coverage validation passed.
- No public API renames were made for cosmetic consistency.

## [1.5.0] - 2026-06-09 - Declarative Config Plan And Apply

Added the declarative config workflow for safer configuration changes.

Highlights:
- Added plan, diff, apply, import, and export flows for configuration.
- The UI gained preview and confirmation flows for config changes.
- CLI config commands expose the same workflow.
- Destructive changes require explicit confirmation.
- Drift detection helps avoid applying stale assumptions.

Validation:
- Contract parity, config fixture validation, CLI tests, UI tests, and E2E checks passed.

## [1.4.0] - 2026-06-09 - Live Connections

Added live connection and UDP session visibility.

Highlights:
- Added API support for live TCP connections and UDP sessions.
- The UI gained the Live Connections view with filters, counts, and empty states.
- TCP and UDP forwarding paths expose connection/session details for diagnostics.
- Dashboard and API reference views were updated.

Validation:
- API contract checks, client tests, and TCP/UDP E2E coverage passed.

## [1.3.0] - 2026-06-08 - CLI

Introduced the Go-based `portier` command-line interface.

Highlights:
- CLI commands cover runtime info, rule list/status, activity, start/stop, diagnose, config workflows, and diagnostics export.
- The CLI is packaged alongside the service binary.
- CLI behavior stays client-side and communicates through the public API.

Validation:
- CLI tests and package validation passed.
- Existing API contract checks remained green.

## [1.2.0] - 2026-06-07 - Diagnostics And Runtime Visibility

Improved runtime inspection, diagnostics, and safer networking feedback.

Highlights:
- Added runtime info and rule diagnose APIs.
- Improved activity log and settings UI.
- Added diagnostics export support.
- Added safer networking advisories for common and potentially exposed bindings.
- Added config compatibility fixture validation groundwork.

Validation:
- Contract, unit, config validation, and E2E checks passed.

## [1.1.0] - 2026-06-06 - Installers And Service Validation

Added release packaging and platform service validation.

Highlights:
- Added Windows, macOS, and Linux release packaging paths.
- Windows installer support was added.
- macOS LaunchAgent and Linux systemd service scripts were added.
- Service install, start, stop, and uninstall validation became explicit platform checks.

Validation:
- Package and platform service validation paths were documented and tested where applicable.

## [1.0.0] - 2026-06-06 - Initial Stable Release

Initial stable Portier release.

Highlights:
- React web UI for managing forward rules.
- TypeScript server runtime and native Go service runtime.
- TCP and UDP forwarding support.
- REST API for rules, status, activity, diagnostics, and configuration.
- E2E coverage for the main user workflows.

Validation:
- Build, test, and E2E checks passed for the initial stable surface.

## [0.1.0] - 2026-06-04 - Prototype

Initial project prototype and repository setup.

Highlights:
- Established the first local forwarding manager structure.
- Added early server, client, and shared package foundations.
- Set the direction for Portier as a local development and LAN testing tool.
