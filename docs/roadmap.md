# Portier Roadmap

This roadmap tracks product direction at a high level. It is not a changelog: completed release
detail lives in [changelog.md](changelog.md), and detailed audit findings live in `audits/`.

## Current Status

- **v1.0–v1.18:** completed and released.
- **v1.19:** completed — 2.0 RC Hardening (release-candidate stabilization only); tagged `v1.19.0`.
- **2.0-RC documentation cleanup:** completed — public docs are release-facing and consistent.
- **v2.0:** released — Stable Local-First Portier; tagged `v2.0.0`. A version-only bump over v1.19
  (no API, OpenAPI-schema, or `rules.json` change) with validated release artifacts for Windows,
  macOS, and Linux. See the [changelog](changelog.md).
- **Post-2.0:** directional themes only (see [Post-2.0 Directions](#post-20-directions)); the
  pre-2.0 arc is closed.

## Completed Releases

For full per-release detail, see [changelog.md](changelog.md). For the audit trail behind the
hardening releases, see `audits/`.

- **v1.0 — Core runtime:** TCP/UDP forwarding, the TypeScript server, the Go service, the React UI,
  Playwright E2E, and the package layout.
- **v1.1 — Installation and distribution:** native service install flows, release artifacts, and
  validation scripts.
- **v1.2 — Diagnostics and operational polish:** runtime info, per-rule diagnose, safer networking
  UX, and diagnostics export.
- **v1.3 — Native CLI:** the Go `portier` CLI as an API client.
- **v1.4 — Live Connections:** read-only TCP connection and UDP session visibility.
- **v1.5 — Declarative config:** config plan/diff/apply with drift control and safe destructive
  confirmation.
- **v1.6 — Architecture and quality audit:** the first major audit/hardening pass; the glossary
  became canonical for terminology.
- **v1.7 — Cleanup and maintainability:** reduced duplicated lifecycle/config logic, clarified
  diagnose phases, normalized CLI exit behavior, and decomposed the settings UI.
- **v1.8 — Operator power tools:** rule groups and group actions, rule health, duplicate-rule UI,
  and related CLI/UI support.
- **v1.9 — Doctor and config toolkit:** read-only doctor/config-doctor tooling, explanations, report
  export, support bundles, and AI handoff prompts.
- **v1.10 — Policy guardrails:** offline/runtime policy checking, templates, review, and baselines.
- **v1.11 — Workflow automation:** local workflow plan/template/runbook/run/report tooling that
  composes existing safe operations.
- **v1.12 — Workflow history:** opt-in, bounded, privacy-preserving local workflow history.
- **v1.13 — Replay tool:** the separate offline `tools/replay` analysis tool for saved artifacts.
- **v1.14 — NestJS TypeScript server migration:** moved the TypeScript server to NestJS while
  preserving the REST contract and Go parity.
- **v1.15 — Go modular router:** moved the Go service API layer to focused `chi` route modules behind
  `app.App`.
- **v1.16 — Post-migration architecture & reliability audit:** a ten-audit review of v1.8–v1.15 plus
  a synthesis-and-fix-plan and docs polish. Outcome: 0 release blockers; the deferred findings
  (startup recovery, UDP prune wiring, generic-500 redaction, strict lint) were scheduled into
  v1.17–v1.19. Public reports live in `audits/`.
- **v1.17 — Migration & Recovery:** startup/config recovery so a bad or unbindable config no longer
  aborts startup (resolves the v1.16 R-1 lockout), recovery observable across API/CLI/UI, and an
  offline `portier config migrate` command. See [recovery.md](recovery.md).
- **v1.18 — Install, Service & Upgrade Experience:** native file-install packages (Windows MSI,
  macOS `.pkg`, Linux `.deb`/`.rpm`), arch-suffixed portable archives with checksums, install/remove
  and upgrade-preservation smokes, and split manual release workflows. See [installer.md](installer.md).
- **v1.19 — 2.0 RC Hardening:** release-candidate stabilization only — strict client lint and 100%
  client coverage, push/PR CI, single-source version tooling, an OpenAPI-driven API Reference, a
  generated/validated Postman collection with a local Newman smoke, UI consistency polish, and two
  bounded backend fixes (UDP session pruning and Go generic-500 redaction). No new features, no API
  or `rules.json` change.

## Road To 2.0 — Stable Local-First Edition

**Product decision:** Portier 2.0 remains local-first. Remote/team/auth management is deferred beyond
2.0. The goal is an extremely reliable local port-forwarding manager with strong installation,
upgrade, recovery, diagnostics, and release quality.

```text
v1.17 - Migration & Recovery
v1.18 - Install, Service & Upgrade Experience
v1.19 - 2.0 RC Hardening
v2.0  - Stable Local-First Portier
```

Portier 2.0 is ready when local install is reliable on supported platforms, upgrade from v1.x is
safe and documented, migration/recovery are tested, release artifacts are versioned and validate,
and remote/team/auth is explicitly out of scope.

## Portier v2.0 — Stable Local-First Portier

**Core theme:** stable, polished, local-first Portier.

### Release Promise

v2.0 is the first version where users can rely on stable local runtime behavior, a REST API
compatibility policy, a config migration/recovery policy, stable CLI behavior and exit codes,
diagnostics/history/replay artifact policy, robust install/upgrade, and validated release artifacts.

### Includes

Mature rule management; doctor/config tooling; policy/preflight/workflow features that have already
landed; observability/history; the offline replay tool; the NestJS TypeScript fallback server; the
modular Go service router; post-migration audit fixes; migration/recovery; polished install/upgrade;
and release-candidate hardening.

### Does Not Include

Remote management, team/user/role systems, an authentication model, cloud sync, OAuth, a plugin
framework, or cosmetic public API renames.

### Acceptance Criteria

- Release artifacts are built and validated; checksums are generated.
- The upgrade path from the previous stable v1.x is documented and tested.
- Version injection is correct on every surface.
- Install/upgrade docs are complete and local-first scope is clearly documented.
- No coverage gates are lowered; `validate:contract`, `validate:coverage`, `validate:config`,
  `validate:cli`, replay/tool validations, runtime smoke, and E2E all pass.
- Release notes are complete; the tag/publish process is explicit and manual.

## v2.0 Release Prep

The stable v2.0 release is a deliberate, manual sequence — nothing below happens automatically:

1. Bump every version surface: `npm run version:set 2.0.0`, then verify with `npm run version:check`.
2. Regenerate the OpenAPI artifact if the schema changed: `npm run apidoc:generate`.
3. Run the full local validation matrix in [checklist.md](checklist.md) › *Before A Version Release*.
4. Add a `2.0.0` entry to [changelog.md](changelog.md) and confirm [upgrade-v2.md](upgrade-v2.md)
   is accurate for the release.
5. Build and validate release artifacts via the manual (`workflow_dispatch`) release workflows.
6. Tag and publish as an explicit manual step.

## Post-2.0 Directions

These are **directional themes for after the stable v2.0 release**, not dated promises or fixed
commitments. They may be reordered, combined, or dropped as priorities become clear. Pre-2.0 work
stays closed; nothing here reopens it, and Portier stays local-first unless a theme is explicitly
rescoped.

### v2.1 — Trust & Distribution Polish

Make Portier feel safer and more professional to download and install, without changing the
local-first runtime model.

- Windows Authenticode signing.
- macOS Developer ID signing and notarization.
- Clearer download and checksum-verification guidance.
- A release-notes / GitHub Release publishing process.
- Small installer trust polish.

### v2.2 — Explicit Service Lifecycle

Improve service lifecycle ergonomics while preserving the explicit opt-in safety model.

- Polished opt-in service install/start/stop/status.
- Windows Service / Scheduled Task hardening.
- macOS LaunchAgent hardening.
- Linux systemd hardening.
- Deeper service-lifecycle smokes.
- Installers still do not silently enable or start services by default.

### v2.3 — Persistent Observability

Make operational history more useful without adding telemetry or remote collection.

- Optional persistent Activity Log / history.
- A retention policy.
- An improved diagnostics bundle/export.
- A "what happened while I was away?" operator view.
- Replay/tooling integration where useful.

### v2.4 — Doctor & Diagnostics 2

Improve troubleshooting and support workflows while keeping diagnostics local, deterministic, and
privacy-safe.

- Doctor / config-doctor UX polish.
- Actionable remediation hints.
- Consistent runtime/config diagnostics.
- Support-bundle redaction-guard polish.
- AI handoff prompt polish — offline and privacy-safe only.

### v2.5 — Rule Management Power Tools

Improve power-user rule management once the core 2.0 product is stable.

- Bulk edit / enable / disable.
- Saved filters and views.
- Rule templates.
- Duplicate-and-transform rule.
- Group UX improvements.
- Better import conflict resolution.

### v2.6 — Package Manager Distribution

Make installation easier through common package managers once signing/trust and release packaging
are mature.

- winget.
- A Homebrew tap.
- Chocolatey / Scoop if useful.
- Linux repo / package-manager exploration.
- A cautious stance on auto-update.

### v2.7+ — Remote/Team Exploration

Treat remote/team scenarios as exploration, not an assumed next feature — threat-model before
implementing.

- Only after threat modeling.
- Shared config profiles.
- Team conventions.
- Possibly signed config bundles.
- Auth-model exploration.
- The local-first product posture is preserved unless explicitly changed.
