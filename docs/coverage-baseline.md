# Coverage Baseline

Measured at v1.5.0 release (2026-06-09). Updated from v1.4.0 (cli 92.7%, client 90.56%, service 82.5%, shared 82.1%, server 82.88%). CLI updated at v1.5 Slice 4 (92.7% → 93.2%). Gates ratcheted at v1.5.0 release.

## v1.5.0 Release Gates

| Component | Statements | Branch | Functions | Gate (stmts/branch/funcs) | Status |
| --------- | ---------: | -----: | --------: | ------------------------: | ------ |
| tools/cli |      93.2% |      — |      98.6% |                       93% | gated  |
| client    |     95.1%  | 90.1%  |     79.9%  |                  94/90/79 | gated  |
| server    |     88.7%  | 91.0%  |     99.1%  |                  88/90/99 | gated  |
| service   |      85.8% |      — |      92.5% |                       85% | gated  |
| shared    |    100.00% |100.00% |    100.00% |             100/100/100   | gated  |

Gates are enforced by `npm run validate:coverage` (`scripts/validate-coverage.js`). Per-component: `npm run validate:coverage:<component>`.

## Summary Table

| Component     | Statements | Branch | Functions |         Gate | Tooling                           |
| ------------- | ---------: | -----: | --------: | -----------: | --------------------------------- |
| tools/cli     |      93.2% |      — |      98.6% |          93% | `go test` + validate-coverage --only cli |
| client        |     95.1%  | 90.1%  |     79.9%  |     94/90/79 | vitest + @vitest/coverage-v8    |
| service       |      85.8% |      — |      92.5% |          85% | `go test -coverpkg`               |
| shared        |    100.00% |100.00% |    100.00% | 100/100/100  | vitest + @vitest/coverage-v8    |
| server        |     88.7%  | 91.0%  |     99.1%  |     88/90/99 | vitest + @vitest/coverage-v8    |
| scripts       |        N/M |      — |          — |         none | not yet measured                  |

Coverage commands:

```
npm run coverage:shared            # shared vitest with v8 (writes coverage/shared/)
npm run coverage:server            # server vitest with v8 (writes coverage/server/)
npm run coverage:client            # client vitest with v8 (writes coverage/client/)
npm run coverage:service           # go test -p 1 -coverpkg=./sources/...
npm run coverage:cli               # go test — reporting only
npm run coverage:baseline          # all five in sequence (reporting only)
npm run validate:coverage          # runs all + enforces all gates; exits 1 on failure
npm run validate:coverage:shared   # shared only
npm run validate:coverage:server   # server only
npm run validate:coverage:client   # client only
npm run validate:coverage:service  # service only
npm run validate:coverage:cli      # cli only
```

All coverage output lands in `coverage/` (gitignored). TypeScript workspaces write `coverage-summary.json` per component; Go profiles are temporary and cleaned up after reporting.

**Windows vitest coverage note:** The v8 coverage provider on Windows has a path case-sensitivity issue (`C:\` vs `c:\`) that causes each source file to be counted twice in the aggregate — once with real coverage data and once with 0% — when `coverage.include` is combined with build artifacts in the workspace. Fixed by adding `build/**` and `*.config.ts` to each workspace's `coverage.exclude` in `vitest.config.ts`. This keeps the aggregate consistent with the documented baseline numbers.

---

## tools/cli — 93.2% statements (gate: 92%)

| Package                    | Coverage |
| -------------------------- | -------: |
| portier/cli/sources        |    ~29%* |
| portier/cli/sources/client |    ~9%*  |
| portier/cli/sources/commands |  ~85%* |
| portier/cli/sources/output |     ~2%* |
| **Total (cross-package)**  |  **93.2%** |

\* Per-package numbers reflect cross-package instrumentation totals; the combined 93.2% is the meaningful figure. Updated at v1.5 Slice 4: `configplancmd_test.go` (35 tests) + 5 client PlanConfig tests added; `config plan` and `config diff` commands fully covered.

Gate: 92%. Enforced by `npm run validate:coverage:cli` (scripts/validate-coverage.js --only cli).

Known untestable branches documented in scripts/validate-coverage.js: `main()` os.Exit, `http.NewRequest` error, `json.Marshal` error on CLI types, `json.NewEncoder(stdout).Encode` errors, and repeated `validateURL` branches across commands.

---

## client — 94.71% statements, 90.19% branch, 78.26% functions

Updated at v1.5 pre-release (2026-06-09). Previous: 90.56% stmts (v1.4.0). Baseline (v1.3.0): 89.0%.

| File                                        | Stmts  | Branch | Funcs  | Notes                             |
| ------------------------------------------- | -----: | -----: | -----: | --------------------------------- |
| sources/main.tsx                            |     0% |      — |      — | app entry point, not unit-tested  |
| sources/api/portierApi.ts                   |   100% |   100% |   100% | v1.5 pre: full unit test suite added |
| sources/components/AdvisoryList.tsx         |   100% |   100% |   100% | v1.5 pre: 7 unit tests added      |
| sources/app/App.tsx                         |  84.3% |  82.1% |  65.5% |                                   |
| sources/features/activity/ActivityLogView.tsx | 88.4% | 93.1% | 61.1% |                                   |
| sources/features/forwards/ForwardRuleList.tsx | 90.3% | 90.2% | 55.6% |                                   |
| sources/features/forwards/ForwardRuleForm.tsx | 97.7% | 92.4% | 77.8% |                                   |
| sources/features/settings/SettingsView.tsx  |  97.3% |  80.5% |  90.0% |                                   |
| sources/features/settings/diagnosticsExport.ts | 97.7% | 88.0% | 100% |                                   |
| sources/features/apidocs/ApiDocsView.tsx    |  99.4% |  66.7% |   100% |                                   |
| sources/features/dashboard/DashboardView.tsx | 100%  |  75.0% |   100% |                                   |
| sources/features/forwards/ForwardStatusBadge.tsx | 100% | 100% | 100% |                               |
| sources/features/forwards/RuleDiagnosticsPanel.tsx | 100% | 100% | 100% |                            |
| sources/app/Sidebar.tsx                     |   100% |   100% |   100% |                                   |
| sources/app/NavItem.ts                      |   100% |   100% |   100% |                                   |
| sources/features/connections/LiveConnectionsView.tsx | 100% | 94.1% | 88.2% |                            |
| sources/utils/format.ts                     |   100% |   100% |   100% |                                   |

Gate: 94/90/78. Enforced by `npm run validate:coverage:client`.

---

## server — 88.63% statements, 91.24% branch, 99.09% functions

Updated at v1.5 Slice 2 (2026-06-09). Previous: 87.11% (v1.5 pre-release). Before that: 82.88% stmts (v1.4.0). Baseline (v1.3.0): 71.9% stmts.

"All files" figure from `npm run coverage:server`.

| File                                                  | Stmts  | Branch | Funcs  | Notes                                            |
| ----------------------------------------------------- | -----: | -----: | -----: | ------------------------------------------------ |
| sources/index.ts                                      |     0% |      0%|      0%| app entry/wiring, not unit-tested                |
| sources/forwarders/types.ts                           |     0% |      — |      — | interface-only file, no executable code          |
| sources/diagnose.ts                                   |  85.9% |  86.5% |   100% | timeout paths (2s) not unit-tested               |
| sources/forwarders/udp-forwarder.ts                   |  86.3% |  84.0% |   100% | send error callbacks require specific timing     |
| sources/forward-manager.ts                            |  99.4% |  88.7% |   100% | v1.5 pre: 9 new tests; 1 unreachable path at 129-130 |
| sources/api.ts                                        |  92.8% |  87.0% |   100% | platform detection branches (Windows-only env)  |
| sources/config-plan.ts                                |  100%  |  100%  |   100% | v1.5 Slice 2: pure plan engine, 65 unit tests   |
| sources/logger.ts                                     |   100% |   100% |   100% | v1.5 pre: 6 unit tests added                    |
| sources/config-store.ts                               |   100% |   100% |   100% | v1.5 pre: non-array JSON test added              |
| sources/server-options.ts                             |   100% |   100% |   100% | v1.5 pre: unknown flag + missing value tests     |
| sources/connections/tcp-connection-registry.ts        |   100% |   100% |   100% |                                                  |
| sources/connections/udp-session-registry.ts           |   100% |   100% |   100% |                                                  |
| sources/forwarders/tcp-forwarder.ts                   |   100% |    90% |   100% | branch gap = optional registry param             |
| sources/activity/activity-store.ts                    |   100% |   100% |   100% |                                                  |

Gate: 87/89/99. Enforced by `npm run validate:coverage:server`. (Consider ratcheting to 88/91/99 after v1.5 Slice 2.)

Notes:
- `index.ts` bootstrap is integration-tested via E2E and `validate:contract`; 0% here is expected.
- `forward-manager.ts` lines 129-130 are a defensive safety net for a scenario that cannot be triggered through the public API (any invalid patch field is rejected at patch-validation stage first).

**tcp-forwarder.ts remaining branch gap (10%):**
Lines 94–97 and 164 are optional-registry guard branches (`if (connId)`). When `registry` is not passed to the constructor, `connId` is `undefined` and the right-hand side of these conditions is never reached. This is a by-design optional API path, not a meaningful test gap.

**udp-forwarder.ts remaining gaps (13.7% stmts):**
- Multi-client response error callback: `listenSocket.send()` failure inside the per-session targetSocket `message` handler.
- `if (!this.listenSocket) return` guard: requires a stop/receive race that cannot be reliably triggered without mocking.
- Multi-client send error callback: `session.targetSocket.send()` failure.
All three require mocking or specific timing and remain from Slice 1. The registry wiring added in Slice 4 is fully covered by the new integration tests.

---

## service — 85.8% statements (combined cross-package)

Updated at v1.5 Slice 3 (2026-06-09). Previous: 84.8% (v1.5 pre-release). Baseline (v1.3.0): 79.7%.

Per-package figures (package-internal test coverage):

| Package                          | Stmts (pkg) | Notes                                            |
| -------------------------------- | ----------: | ------------------------------------------------ |
| sources/connections              |       98.4% | TCP + UDP registries; all public methods covered |
| sources/advisory                 |      100.0% | fully covered                                    |
| sources/activity                 |       89.5% |                                                  |
| sources/api                      |       81.0% | v1.5 pre: 10 new tests; v1.5 Slice 3: 10 plan endpoint tests added |
| sources/config                   |       77.6% | v1.5 pre: object-without-rules-key test added    |
| sources/configplan               |      ~100%  | v1.5 Slice 3: new pure plan engine; 49 unit tests |
| sources/forwarders               |       85.1% | emitPacketError 0% gap remains                   |
| sources/manager                  |       83.2% | v1.5 pre: 7 new tests (SetLogger, activity, import modes) |
| sources/options                  |       81.0% | FromOSEnv 0% (requires real env, not unit-tested)|
| sources/validation               |       77.2% | v1.5 pre: 12 new tests covering optional ID, decode, patch, InputFromRule |
| sources/domain                   |        N/T  | type definitions, no test file                   |
| sources/logger                   |        N/T  | thin wrapper, no test file                       |
| sources/platform                 |        N/T  | OS-specific (Windows service), no test file      |
| sources/static                   |        N/T  | static file helper, no test file                 |
| sources/version                  |        N/T  | constant, no test file                           |
| sources/ (main.go)               |        N/T  | entry point, no test file                        |
| **Combined total (-coverpkg)**   |   **85.8%** |                                                  |

N/T = no test file. Most of these are thin wrappers, type definitions, or OS-integration code.

Gate: 84%. Enforced by `npm run validate:coverage:service`.

Notes:
- The combined coverage run uses `-p 1` (sequential) to avoid timing flakiness in `TestTCPForwarderEmitsConnectionClosedEvent` under parallel cross-package instrumentation. The per-package test for that package passes reliably.
- The `waitForTestCondition` timeout in `sources/forwarders/tcp_test.go` was raised from 2s to 5s for robustness under heavy instrumentation.
- `sources/connections/` package: Slice 5 added TCP registry (98.1% stmts), Slice 6 added UDP registry (98.4% combined). Gaps: `rand.Read` fallback in private `generateConnectionID` (untestable, same pattern as CLI), and stale-key defensive path in `OpenOrTouchSession` (unreachable through public API). All public methods are at 100% statements.

---

## shared — 100.0% statements, 100.0% branch, 100.0% functions

Updated at v1.5 pre-release (2026-06-09). Previous: 82.1%/54.3%/90.0% (v1.4.0).

| File                       | Stmts  | Branch | Funcs  | Notes                                   |
| -------------------------- | -----: | -----: | -----: | --------------------------------------- |
| sources/activity.ts        |     0% |      0%|      0%| type definitions only, no executable code |
| sources/index.ts           |   100% |   100% |   100% | v1.5 pre: all validation + advisory + listenKey paths covered |

v1.5 pre-release added 7 tests covering: listenKey consistency, validateForwardRule with empty listenHost, and all remaining uncovered advisory branches. All files: 100/100/100.

Gate: 100/100/100. Enforced by `npm run validate:coverage:shared`.

---

## scripts — Not currently measured

The scripts in `scripts/` are primarily Node.js automation and PowerShell/Bash helpers. There is no test suite for them. Behavioral coverage comes from:

- `validate:config`, `validate:contract`, `validate:binary`, `validate:scripts` — these scripts test other things, they are not self-tested.
- Static analysis is covered by `validate:scripts` for the shell scripts.
- The Node.js validation scripts (`validate-config.js`, `validate-contract.js`, etc.) have no unit tests.

Coverage measurement for scripts is not currently feasible without dedicated test infrastructure. Proposed follow-up: add a `tests/scripts/` suite with mocked child_process for the most critical script helpers.

---

## E2E, Contract, Config Fixtures

These are not part of statement coverage but are important for overall test confidence:

- **E2E (Playwright)**: 9 spec files, 32 tests total. Covers app load, CRUD, form validation, diagnostics, import/export, settings runtime info, plan & apply preview workflow, API docs (including `/api/connections`), Live Connections view (all tabs, filters, auto-refresh, rule filter), TCP real forwarding, and 3 UDP modes. Runs against the real TypeScript server. Does not produce statement coverage metrics. See `docs/e2e-coverage.md` for the full workflow matrix.
- **validate:contract**: API parity between TypeScript server and Go service. Runs when Go binary is present.
- **validate:config**: Fixture-based config compatibility (load, import, export, rejection, UDP defaults). 8 valid fixtures, multiple invalid fixtures.
- **validate:binary**: Runtime binary smoke tests (start, health, static, shutdown).
- **validate:scripts**: Static analysis + dry-run for installer/service shell scripts.

---

## High-Risk Coverage Gaps

### Resolved in v1.5 pre-release

- **shared index.ts** (100% ✓): all validation, advisory, and listenKey paths covered.
- **client portierApi.ts** (100% ✓): full unit test suite added with fetch mocking.
- **client AdvisoryList.tsx** (100% ✓): 7 unit tests covering all severity/compact variants.
- **server logger.ts** (100% ✓): 6 unit tests covering all log levels and errorFields.
- **server config-store.ts** (100% ✓): non-array JSON case added.
- **server server-options.ts** (100% ✓): unknown flag and missing-value error tests added.
- **server forward-manager.ts** (99.4% ✓): import/merge/replace modes, flush, activity paths, startRule failure all covered.

### Remaining gaps

- **server index.ts** (0%): bootstrap wiring, not unit-testable. Covered by E2E only.
- **server api.ts** (92.5%): platform detection branches (`normalizePlatform`/`normalizeArch`) are Windows-only environment-dependent; darwin/linux/unknown branches cannot be triggered on Windows.
- **server diagnose.ts** (85.9%): UDP bind timeout (2 s) and TCP connect timeout are not triggered in unit tests; UDP bind error and all other paths are covered.
- **server udp-forwarder.ts** (86.3%): multi-client send/return error callbacks require specific network error injection.
- **service forwarders**: `emitPacketError` at 0% — Go side, not critical but worth covering in v1.6.
- **service config** (77.6%): will grow with v1.5 diff/plan/apply; 100% required for new code.
- **service options** (81.0%): `FromOSEnv` requires real env vars, not unit-testable without env injection.
- **service platform** (0%): Windows Service integration, OS-specific.
- **scripts** (not measured): coverage gap for automation tooling.

### v1.5 Declarative Config & Drift Control targets

- **service configplan** (~100% ✓): `BuildConfigPlan` and `ExtractRulesRaw` at ~100% (49 unit tests). Slice 3 complete.
- **service api** (POST /api/config/plan handler ✓): 10 integration tests. Slice 3 complete.
- **server config-plan.ts** (100% ✓): pure plan engine, 65 unit tests. Slice 2 complete.
- **server config-store.ts** (100% ✓): high baseline maintained.
- **shared validation/types** (100% ✓): new diff/plan types and validation helpers — 100% required.
- **service api** (POST /api/config/apply handler ✓): 11 integration tests. Slice 5 complete.
- **tools/cli commands** (93.2% ✓): `config plan`, `config diff`, and `config apply` all implemented (Slices 4–5 complete). CLI gate at 93.2% (gate 92%).

---

## Ratchet Plan

### New v1.5 code — 100% gate from first commit

All new or materially changed implementation files in v1.5 must reach 100% meaningful coverage before merging. Gate enforcement: `npm run validate:coverage`.

### Existing baselines — incremental ratchet (non-blocking for unrelated work)

| Component | v1.3.0 | v1.4.0    | v1.5 pre  | v1.5 target | v1.6 target |
| --------- | -----: | --------: | --------: | ----------: | ----------: |
| cli       |  92.7% |     92.7% |     93.2% |       95%+  |        100% |
| client    |  89.2% |    90.56% |    94.71% |       96%+  |        100% |
| server    |  71.9% |    82.88% |    88.63% |       92%+  |        100% |
| service   |  79.7% |     82.5% |     85.8% |       90%+  |        100% |
| shared    |  82.1% |     82.1% |    100.0% |      100.0% |        100% |

v1.5 pre-release coverage uplift: shared → 100%, client 90.56% → 94.71%, server 82.88% → 87.11%, service 82.5% → 84.8%. All gates ratcheted upward. v1.5 Slice 2: server 87.11% → 88.63% (`config-plan.ts` at 100%). v1.5 Slice 3: service 84.8% → 85.8% (`configplan` package added at ~100%, 49 unit tests). v1.5 Slice 4: cli 92.7% → 93.2% (`config plan` and `config diff` commands; 200+ CLI tests).

Do not block unrelated v1.5 work on legacy coverage gaps. Require 100% for newly added or materially changed files in v1.5 and v1.6.
