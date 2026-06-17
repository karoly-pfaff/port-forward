# Changelog

All notable changes to Portier are documented here.

This changelog is written for both humans and coding agents. It summarizes what changed, why it matters, and the validation signal for each release. Detailed implementation history, audit notes, and commit-level rationale live in `audits/` and Git history.

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
