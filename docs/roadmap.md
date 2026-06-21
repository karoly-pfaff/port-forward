# Portier Roadmap

This roadmap tracks product direction at a high level. It is not a changelog: completed release
detail lives in [changelog.md](changelog.md), and detailed audit findings live in `audits/`.

## Current Status

- **v1.0–v1.18:** completed and released.
- **v1.19:** completed — 2.0 RC Hardening (release-candidate stabilization only); tagged `v1.19.0`.
- **2.0-RC documentation cleanup:** current — a public-docs pass ahead of the stable release.
- **v2.0:** next — Stable Local-First Portier. The version bump, release-artifact build, and
  tag/publish are a separate, explicit, manual step.

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

## Post-2.0 — Optional Hardening Bucket

Deliberately deferred beyond 2.0 and not blocking the stable release:

- Code signing and notarization (Windows Authenticode, macOS Developer ID + notarization).
- Automated GitHub Release publishing and tagging.
- arm64 **native packages** (`.pkg`/`.deb`/`.rpm`); arm64 ships as validated portables today.
- Auto-installing the OS service from the installer (installers stay file-install by design).
- Other package managers (Homebrew, winget, Chocolatey).
- Remote/team/auth management — out of scope for the local-first product unless deliberately
  rescoped.
