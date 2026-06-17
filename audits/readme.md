# Portier Audits

This directory contains curated public audit reports for Portier. Raw/internal audit
working notes belong under `audits/private/`, which remains gitignored.

The synthesis files are the best entry points. Individual audit reports are preserved
as period-accurate records of what was inspected, what was validated, and which
follow-up work was accepted or deferred.

## v1.16 Post-Migration Architecture And Reliability Audit

The v1.16 audit set covered the post-NestJS and post-chi-router product state. It
contains 10 audit reports plus one synthesis/fix-plan.

Result: 3 PASS, 7 PASS WITH NOTES, 0 FAIL, 0 release blockers.

Recommended entry point:

- [v1.16 Audit Synthesis & Fix Plan](v1.16-audit-synthesis-and-fix-plan.md)

Reports:

1. [Contract & Runtime Parity](v1.16-contract-parity-audit-1.md)
2. [Architecture & Module Boundary](v1.16-architecture-boundary-audit-1.md)
3. [Resilience & Data Durability](v1.16-resilience-durability-audit-1.md)
4. [Security & Local-Safety](v1.16-security-local-safety-audit-1.md)
5. [Observability & Replay](v1.16-observability-replay-audit-1.md)
6. [Automation, Policy & Workflow](v1.16-automation-policy-audit-1.md)
7. [Testing & Coverage](v1.16-testing-coverage-audit-1.md)
8. [Complexity, Duplication & Maintainability](v1.16-complexity-maintainability-audit-1.md)
9. [Documentation, Glossary & Operator UX](v1.16-docs-ux-audit-1.md)
10. [Release Readiness & Packaging](v1.16-release-readiness-audit-1.md)

## Earlier Audit Reports

The earlier v1.6 architecture and quality audit set is also curated for publication.
It includes topical audits, a synthesis/fix-plan, and a release-readiness checkpoint.

Recommended entry point:

- [v1.6 Audit Synthesis & Fix Plan](v1.6-audit-synthesis-and-fix-plan.md)

Reports:

1. [Architecture](v1.6-architecture-audit-1.md)
2. [Coverage](v1.6-coverage-audit-1.md)
3. [Complexity / Metrics](v1.6-complexity-metrics-audit-1.md)
4. [Design Pattern Usage](v1.6-design-pattern-audit-1.md)
5. [SOLID](v1.6-solid-audit-1.md)
6. [Duplication](v1.6-duplication-audit-1.md)
7. [Error Handling & Exception Flow](v1.6-error-flow-audit-1.md)
8. [Resilience & Fault Tolerance](v1.6-resilience-audit-1.md)
9. [Readability & Naming](v1.6-readability-naming-audit-1.md)
10. [Testing](v1.6-testing-audit-1.md)
11. [Release-Readiness Checkpoint](v1.6-release-readiness-checkpoint.md)
