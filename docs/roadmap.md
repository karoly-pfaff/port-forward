# Portier Roadmap

This roadmap tracks product direction and active planning. It is not a changelog.
Completed release slice logs belong in `docs/changelog.md` and detailed audit findings
belong in `audits/`.

## Current Status

- **v1.0-v1.16:** completed.
- **v1.17:** completed — Migration & Recovery (ready for release/tag as v1.17.0).
- **v1.18:** current — Install, Service & Upgrade Experience.
- **v1.19 and v2.0:** planned local-first path to a stable 2.0.

## Completed Releases

- **v1.0 - Core runtime:** proved the core TCP/UDP forwarding app, TypeScript server,
  Go service, React UI, Playwright E2E, and package layout.
- **v1.1 - Installation and distribution:** added native service installation flows,
  release artifacts, validation scripts, and platform polish.
- **v1.2 - Diagnostics and operational polish:** added runtime info, per-rule diagnose,
  safer networking UX, activity-log polish, settings improvements, and diagnostics export.
- **v1.3 - Native CLI:** introduced the Go `portier` CLI as an API client for runtime,
  rule, status, activity, diagnose, config, and diagnostics workflows.
- **v1.4 - Live Connections:** added read-only TCP connection and UDP session visibility
  through `GET /api/connections` and the Live Connections UI.
- **v1.5 - Declarative config and drift control:** added config plan/diff/apply flows,
  safe destructive confirmation, CLI/UI workflows, and contract parity coverage.
- **v1.6 - Architecture and quality audit:** ran the first major audit/hardening pass,
  improving parity, resilience, coverage, naming, contract guards, and maintainability.
- **v1.7 - Cleanup and maintainability:** reduced duplicated lifecycle/config logic,
  clarified diagnose phases, normalized CLI exit behavior, decomposed settings UI, and
  aligned glossary-backed wording.
- **v1.8 - Operator power tools:** added rule groups, group actions, rule health,
  duplicate-rule UI, config preview clarity, row action menu polish, and related CLI/UI
  support.
- **v1.9 - Doctor and config toolkit:** added read-only doctor/config-doctor tooling,
  explain output, strict mode, report export, support bundles, config summaries, and AI
  handoff prompts.
- **v1.10 - Policy guardrails:** added offline/runtime policy checking, explanations,
  report export, templates, policy review, baselines, runtime policy mode, and profile
  checks.
- **v1.11 - Workflow automation:** added local workflow plan/template/runbook/run/report
  tooling that composes existing safe operations without shell execution or mutation.
- **v1.12 - Workflow history:** added opt-in local workflow history, export, list
  filters, stats, prune/clear behavior, and compact privacy-preserving records.
- **v1.13 - Replay tool:** added the separate offline `tools/replay` analysis tool for
  saved artifacts, including plan/analyze/timeline/compare/explain behavior.
- **v1.14 - NestJS TypeScript server migration:** moved the TypeScript server to the
  NestJS structure while preserving the REST contract, static serving, OpenAPI artifact,
  runtime smoke, and contract parity.
- **v1.15 - Go modular router:** moved the Go service API layer to focused route modules
  on `chi`, behind `app.App`, preserving behavior, contract parity, OpenAPI inventory,
  static serving, and runtime smoke.

For completed release details, see `docs/changelog.md`. For the audit trail behind the
major hardening releases, see `audits/`.

## Portier v1.16 - Post-Migration Architecture And Reliability Audit

**Core theme:** Re-audit Portier after the NestJS server migration, Go modular router
migration, observability/history work, replay tooling, and automation/policy features.

This is an **audit/hardening release, not a feature release**. Do not add large features
in v1.16 unless an audit finding identifies a blocker. The goal is to verify and harden
everything added from v1.8 through v1.15 while keeping the public contract stable.

### Primary Goals

- Verify TypeScript NestJS server parity with the Go modular service.
- Verify `tools/cli` and `tools/replay` boundaries.
- Verify observability/history/replay durability and privacy.
- Verify automation, policy, workflow, group, profile, and snapshot behavior.
- Verify diagnostics/export/redaction behavior.
- Verify coverage gates still represent meaningful coverage.
- Verify docs and glossary match the current product.
- Produce a synthesis and execute only MUST/SHOULD fixes needed for v1.16.

### Audit Set

Each audit is read-only analysis producing a concise findings document.

1. **Contract and Runtime Parity Audit**
   - Compare TypeScript NestJS vs Go modular service public REST behavior.
   - Cover status codes, error envelopes, activity values, config plan/apply/import/export,
     diagnostics/history/observability/policy endpoints, and CLI DTO compatibility.
   - Output: contract gaps, parity risks, recommended contract scenarios.
   - Run this first.

2. **Architecture and Module Boundary Audit**
   - Review NestJS module/controller/service boundaries, Go app/router boundaries,
     shared domain logic, CLI vs replay ownership, and history/config/policy boundaries.
   - Output: module-boundary findings, architecture notes, refactor candidates.

3. **Resilience and Data Durability Audit**
   - Review rules config persistence, history/activity persistence, snapshots/backups,
     diagnostics export, replay artifacts, corrupt-file recovery, retention, and shutdown.
   - Output: durability risk table, corruption/recovery matrix, accepted gaps vs fixes.

4. **Security and Local-Safety Audit**
   - Review local management assumptions, LAN warnings, policy/preflight claims,
     diagnostic redaction, sensitive data in history/replay/bundles, file-path exposure,
     browser/local storage, and logging/export behavior.
   - Output: local security posture, redaction gaps, wording risks.

5. **Observability and Replay Audit**
   - Review persistent activity/history, connection/session history, timeline API/UI,
     metrics/stats, diagnostics bundle, and `tools/replay`.
   - Output: observability coverage map, replay correctness findings, schema risks.

6. **Automation, Policy, and Workflow Audit**
   - Review group/profile operations, policy metadata, preflight, temporary starts,
     recipes/workflows, snapshots/restore, saved filters, and local analyzers.
   - Output: workflow safety matrix, policy/preflight invariants, rollback gaps.

7. **Testing and Coverage Audit**
   - Review unit/integration/E2E balance, contract/CLI/replay/history coverage,
     migration coverage, flake risk, and coverage-gate strategy.
   - Output: coverage posture, gate recommendations, meaningful coverage gaps.

8. **Complexity, Duplication, and Maintainability Audit**
   - Review NestJS modules, Go routes, config/policy/history/replay logic, repeated DTO
     mapping, duplicate TS/Go semantics, CLI/replay overlap, and large UI components.
   - Output: complexity hotspots, duplication map, refactor candidates.

9. **Documentation, Glossary, and Operator UX Audit**
   - Review README, API contract, glossary, checklist, CLI docs, replay docs, UI/CLI
     wording, warnings, and deferred gaps.
   - Output: doc drift list, terminology updates, UX wording risks.

10. **Release Readiness and Packaging Audit**
    - Review build/release scripts, runtime smoke, release artifacts, version injection,
      CLI/replay packaging, static serving, and platform scripts.
    - Output: release readiness matrix, packaging gaps, final v1.16 decision.

### Output Artifacts

```text
audits/v1.16-contract-parity-audit-1.md
audits/v1.16-architecture-boundary-audit-1.md
audits/v1.16-resilience-durability-audit-1.md
audits/v1.16-security-local-safety-audit-1.md
audits/v1.16-observability-replay-audit-1.md
audits/v1.16-automation-policy-audit-1.md
audits/v1.16-testing-coverage-audit-1.md
audits/v1.16-complexity-maintainability-audit-1.md
audits/v1.16-docs-ux-audit-1.md
audits/v1.16-release-readiness-audit-1.md
audits/v1.16-audit-synthesis-and-fix-plan.md
```

### Process

1. Run the audits as read-only analysis.
2. Produce a synthesis and fix plan.
3. Classify findings: **MUST / SHOULD / NICE / DEFER / DO NOT DO**.
4. Execute only the MUST/SHOULD fixes needed for v1.16.
5. Curate audit documents for publication: keep reviewed public audit reports in
   `audits/`, and keep raw/internal audit notes under `audits/private/` (gitignored).
6. Run the release-readiness checkpoint.
7. Prepare the v1.16 release only after validation passes.

### Audit Principles

- Cite exact files, functions, modules, commands, and validation surfaces.
- Prefer small fix slices.
- Keep coverage meaningful, not theatrical.
- Do not let framework migrations hide behavior drift.
- Validate both runtimes; validate tools separately from the runtime.
- Do not expand scope into remote/team/auth/security unless intentionally scheduled.

### Acceptance Criteria

Portier v1.16 is ready when:

- All audits and the synthesis are complete.
- Audit documents are curated: public-ready reports live in `audits/`, and raw/internal
  notes live in `audits/private/`.
- No unresolved MUST findings remain.
- TypeScript NestJS server and Go modular service parity is verified.
- CLI and replay tool responsibilities are clear.
- Observability/history/replay durability is verified.
- Policy/preflight/workflow behavior is verified.
- `docs/` and the glossary are current.
- Coverage gates pass with no lowering.
- Relevant validation passes: `validate:contract`, `validate:coverage`, `validate:config`,
  `validate:cli`, replay validation, runtime smoke, and E2E.
- The release-readiness checkpoint is PASS, or conditional only on release mechanics.

### Non-Goals

- A broad rewrite.
- New remote/team/auth management.
- Public API renames.
- Contract codegen unless separately chosen.
- Chasing structurally unreachable branches with brittle tests.
- Lowering coverage gates to accommodate framework migrations.
- Speculative abstractions.

## Road To 2.0 - Stable Local-First Edition

**Product decision:** Portier 2.0 remains local-first. Remote/team/auth management is
deferred beyond 2.0. The goal is an extremely reliable local port-forwarding manager
with strong installation, upgrade, recovery, diagnostics, and release quality.

```text
v1.17 - Migration & Recovery
v1.18 - Install, Service & Upgrade Experience
v1.19 - 2.0 RC Hardening
v2.0  - Stable Local-First Portier
```

Portier 2.0 is ready when local install is reliable on supported platforms, upgrade
from v1.x is safe and documented, migration/recovery are tested, artifacts are
versioned, release artifacts validate, and remote/team/auth is explicitly out of scope.

## Portier v1.17 - Migration & Recovery

**Status:** Completed — ready for release/tag as v1.17.0.

**Core theme:** Make startup and configuration safe to recover and migrate.

**Delivered:**

- Startup configuration recovery (resolves the v1.16 R-1 lockout) across the Go service and
  TypeScript/NestJS: malformed, schema-invalid, unreadable, duplicate-bound, and
  autostart-failing configs no longer abort startup — the management API stays reachable, bad
  config is preserved/quarantined, and writes are blocked while recovery is active.
- Recovery state is observable through `GET /api/runtime`, OpenAPI, the diagnostics/support
  bundle, `portier doctor`, and a web UI banner.
- A packaged-runtime recovery smoke validates corrupt-`rules.json` startup.
- Offline `portier config migrate` with dry-run by default and a backup-first, atomic
  `--write`; persisted `rules.json` stays a backward-compatible unversioned bare array and the
  export/import envelope remains `version: "1"`.

See `docs/changelog.md` for the shipped detail and `docs/recovery.md` for the recovery policy.

**Deferred:**

- Native install / service / upgrade experience → v1.18.
- RC hardening (generic-500 redaction, strict lint) → v1.19.
- A versioned *persisted* config envelope and cross-version migration steps → future (not
  needed while there is a single config version).

## Portier v1.18 - Install, Service & Upgrade Experience

**Core theme:** Make Portier pleasant and reliable to install, run, stop, update, and
uninstall.

### Scope

- Make the Windows WiX/MSI installer the canonical Windows installer (replacing Inno);
  keep the portable zip.
- Build a native macOS `.pkg` installer alongside the portable archive (in progress).
- Ready the Linux release: portable tar.gz + a native `.deb` (file-install, disabled unit)
  + the systemd story; `.rpm` planned later.
- Cross-build Linux/macOS portable `.tar.gz` artifacts from any host (release-readiness
  infrastructure); native runtime validation still runs on each OS.
- Provide split, manual GitHub Actions native release workflows — Release Windows (MSI +
  portable), Release MacOS (`.pkg` + portable), Release Linux (`.deb` + portable + native
  smoke) — that build, validate, and upload each platform's release artifacts (package
  first, portable second, `checksums.sha256` last) for inspection — no GitHub Release or tags.
- Add native package install/uninstall smokes (Windows MSI `/i`+`/x`, macOS `.pkg`, Linux
  `.deb`) that prove the package installs the expected layout, never silently creates/starts a
  service or scheduled task, preserves user config, and removes cleanly.
- Produce arch-suffixed portable artifacts incl. arm64 (`linux-arm64`, `macos-arm64`
  alongside amd64; Windows amd64-only), structurally validated with a binary machine-type
  check; native runtime smoke remains host-arch only.
- Clarify service install/uninstall/update flows.
- Preserve and back up config during updates.
- Validate service startup, web UI serving, and CLI connectivity after install.
- Improve macOS and Linux install experience and guidance.
- Make version reporting consistent across CLI, runtimes, UI, diagnostics, artifacts,
  and installer metadata where practical.
- Strengthen release artifact validation and generate checksums.
- Document and test the v1.x upgrade path.

### Acceptance Criteria

- Windows installer and portable package validate.
- Current-platform release build validates.
- Version injection is verified.
- Service install/uninstall docs are accurate.
- macOS/Linux docs are usable.
- Upgrade from the previous stable version is documented.
- Checksums are generated.
- Runtime smoke passes from release artifacts.

## Portier v1.19 - 2.0 RC Hardening

**Core theme:** No new features. No architecture churn. Release-candidate stabilization
only.

### Scope

- Run the complete release validation matrix.
- Perform a local-safety audit.
- Complete docs pass: README, install docs, upgrade guide, CLI docs, API contract,
  glossary, troubleshooting, release notes, known limitations.
- Create `docs/upgrade-v2.md`.
- Classify remaining work as v2.1, later, not planned, or accepted local-first limitation.

### Acceptance Criteria

- No unresolved release blockers or MUST findings remain.
- All release validations pass.
- Release artifacts validate.
- Install/upgrade docs and upgrade guide are complete.
- Local-first scope is clearly documented.
- Remote/team/auth is deferred beyond 2.0.

## Portier v2.0 - Stable Local-First Portier

**Core theme:** Stable, polished, local-first Portier.

### Release Promise

Portier 2.0 is the first version where users can rely on stable local runtime behavior,
REST API compatibility policy, config migration policy, CLI behavior and exit-code
policy, diagnostics/history/replay artifact policy, robust install/upgrade, validated
release artifacts, and a documented recovery path.

### Includes

- Mature rule management.
- Doctor/config tooling.
- Policy/preflight/workflow features that have already landed.
- Observability/history.
- Offline replay tool.
- NestJS TypeScript fallback server.
- Modular Go service router.
- Post-migration audit fixes.
- Migration/recovery.
- Polished install/upgrade.
- Release-candidate hardening.

### Does Not Include

- Remote management.
- Team/user/role systems.
- Authentication model.
- Cloud sync.
- OAuth.
- Plugin framework.
- Cosmetic public API renames.

### Acceptance Criteria

- v1.19 release-readiness checkpoint passes.
- Release artifacts are built and validated.
- Upgrade path from the previous stable v1.x is documented and tested.
- Version injection is correct everywhere.
- Checksums are generated.
- Install docs are complete.
- No coverage gates are lowered.
- `validate:contract`, `validate:coverage`, `validate:config`, `validate:cli`,
  replay/tool validations, runtime smoke, and E2E all pass.
- Release notes are complete.
- Tag/publish process is explicit and manual.
