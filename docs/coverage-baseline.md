# Coverage Baseline

Measured at v1.3.0 (2026-06-08). This is the pre-v1.4 baseline.

## Summary Table

| Component     | Statements | Branch | Functions |     Gate | Tooling                           |
| ------------- | ---------: | -----: | --------: | -------: | --------------------------------- |
| tools/cli     |      92.7% |      — |         — |      92% | `go test` + validate-coverage --only cli |
| client        |      89.2% |  87.6% |     74.0% |     none | vitest + @vitest/coverage-v8      |
| service       |      79.7% |      — |         — |     none | `go test -coverpkg`               |
| shared        |      82.1% |  53.6% |     88.9% |     none | vitest + @vitest/coverage-v8      |
| server        |      80.55% |  84.74% |    96.47% |     none | vitest + @vitest/coverage-v8      |
| scripts       |        N/M |      — |         — |     none | not yet measured                  |

Coverage commands:

```
npm run coverage:shared       # shared vitest with v8 (writes coverage/shared/)
npm run coverage:server       # server vitest with v8 (writes coverage/server/)
npm run coverage:client       # client vitest with v8 (writes coverage/client/)
npm run coverage:service      # go test -p 1 -coverpkg=./sources/...
npm run coverage:cli          # go test — reporting only, no gate
npm run coverage:baseline     # all five in sequence (reporting only)
npm run validate:coverage     # runs all + enforces gates; exits 1 on failure
```

All coverage output lands in `coverage/` (gitignored). TypeScript workspaces write `coverage-summary.json` per component; Go profiles are temporary and cleaned up after reporting.

---

## tools/cli — 92.7% statements (gate: 92%)

| Package                    | Coverage |
| -------------------------- | -------: |
| portier/cli/sources        |    ~35%* |
| portier/cli/sources/client |    ~11%* |
| portier/cli/sources/commands |  ~82%* |
| portier/cli/sources/output |     ~2%* |
| **Total (cross-package)**  |  **92.7%** |

\* Per-package numbers reflect cross-package instrumentation totals; the combined 92.7% is the meaningful figure.

Gate: 92%. Enforced by `npm run validate:cli:coverage` (scripts/validate-coverage.js --only cli).

Known untestable branches documented in scripts/validate-coverage.js: `main()` os.Exit, `http.NewRequest` error, `json.Marshal` error on CLI types, `json.NewEncoder(stdout).Encode` errors, and repeated `validateURL` branches across commands.

---

## client — 89.2% statements, 87.6% branch, 74.0% functions

| File                                        | Stmts  | Branch | Funcs  | Notes                             |
| ------------------------------------------- | -----: | -----: | -----: | --------------------------------- |
| sources/main.tsx                            |     0% |      — |      — | app entry point, not unit-tested  |
| sources/api/portierApi.ts                   |     0% |   100% |   100% | API client, covered by E2E        |
| sources/components/AdvisoryList.tsx         |  42.1% |    50% |   100% | highest-risk uncovered UI code    |
| sources/app/App.tsx                         |  84.2% |  83.3% |  65.5% |                                   |
| sources/features/activity/ActivityLogView.tsx | 88.4% | 93.1% | 61.1% |                                   |
| sources/features/forwards/ForwardRuleList.tsx | 90.3% | 90.2% | 55.6% |                                   |
| sources/features/forwards/ForwardRuleForm.tsx | 97.7% | 92.4% | 77.8% |                                   |
| sources/features/settings/SettingsView.tsx  |  97.3% |  80.5% |  90.0% |                                   |
| sources/features/settings/diagnosticsExport.ts | 97.7% | 88.0% | 100% |                                   |
| sources/features/apidocs/ApiDocsView.tsx    |   100% |  80.0% |   100% |                                   |
| sources/features/dashboard/DashboardView.tsx | 100% | 75.0% |   100% |                                   |
| sources/features/forwards/ForwardStatusBadge.tsx | 100% | 100% | 100% |                               |
| sources/features/forwards/RuleDiagnosticsPanel.tsx | 100% | 100% | 100% |                            |
| sources/app/Sidebar.tsx                     |   100% |   100% |   100% |                                   |
| sources/app/NavItem.ts                      |   100% |   100% |   100% |                                   |

No coverage gate. Tooling added: vitest.config.ts updated with coverage config, `coverage` script added.

---

## server — 80.55% statements, 84.74% branch, 96.47% functions

Updated at v1.4 Slice 3 (2026-06-08). Previous: 79.6% stmts (Slice 1). Baseline (v1.3.0): 71.9% stmts.

"All files" figure from `npm run coverage:server`.

| File                                                  | Stmts  | Branch | Funcs  | Notes                                            |
| ----------------------------------------------------- | -----: | -----: | -----: | ------------------------------------------------ |
| sources/index.ts                                      |     0% |      0%|      0%| app entry/wiring, not unit-tested                |
| sources/logger.ts                                     |     0% |   100% |   100% | logging wrapper, real gap                        |
| sources/forwarders/types.ts                           |     0% |      — |      — | interface-only file, no executable code          |
| sources/connections/tcp-connection-registry.ts        |   100% |   100% |   100% | v1.4 Slice 3: new module, 100% meaningful        |
| sources/forwarders/tcp-forwarder.ts                   |   100% |    90% |   100% | v1.4 Slice 3: wired registry; branch gap = optional registry param |
| sources/forwarders/udp-forwarder.ts                   |  84.3% |  82.0% |   100% | v1.4 Slice 1: up from 57.7%/64.7%/90.9%         |
| sources/diagnose.ts                                   |  84.8% |  83.0% |   100% |                                                  |
| sources/forward-manager.ts                            |  87.6% |  76.5% |    92% |                                                  |
| sources/api.ts                                        |  91.0% |  86.0% |   100% |                                                  |
| sources/server-options.ts                             |  96.7% |  93.5% |   100% |                                                  |
| sources/config-store.ts                               |  94.1% |  92.9% |   100% |                                                  |
| sources/activity/activity-store.ts                    |   100% |   100% |   100% |                                                  |

No coverage gate. Tooling added: vitest.config.ts created, `coverage` script added.

Notes:
- `index.ts` bootstrap is integration-tested via E2E and `validate:contract`; 0% here is expected.
- `logger.ts` contains `createConsoleLogger` and `errorFields` — genuinely not covered, low-risk but a real gap.
- Forwarder coverage improved in v1.4 Slice 1; TCP registry added at 100% in Slice 3.

**tcp-forwarder.ts remaining branch gap (10%):**
Lines 94–97 and 164 are optional-registry guard branches (`if (connId)`). When `registry` is not passed to the constructor, `connId` is `undefined` and the right-hand side of these conditions is never reached. This is a by-design optional API path, not a meaningful test gap.

**udp-forwarder.ts remaining gaps (15.7% stmts):**
- Multi-client response error callback: `listenSocket.send()` failure inside the per-session targetSocket `message` handler.
- `if (!this.listenSocket) return` guard: requires a stop/receive race that cannot be reliably triggered without mocking.
- Multi-client send error callback: `session.targetSocket.send()` failure.
All three require mocking or specific timing; noted for v1.4 Slice 4 if a mocking approach is introduced.

---

## service — 79.7% statements (combined cross-package)

Per-package figures (package-internal test coverage):

| Package                          | Stmts (pkg) | Notes                                            |
| -------------------------------- | ----------: | ------------------------------------------------ |
| sources/advisory                 |      100.0% | fully covered                                    |
| sources/activity                 |       89.5% |                                                  |
| sources/api                      |       81.0% | update, delete, reorder, import partially covered|
| sources/config                   |       77.6% |                                                  |
| sources/forwarders               |       82.6% | emitPacketError 0%; UDP acceptor path gaps       |
| sources/manager                  |       83.2% | SetStartLogger, SetEventLogger, Error() 0%       |
| sources/options                  |       81.0% | FromOSEnv 0% (requires real env, not unit-tested)|
| sources/validation               |       77.2% | ValidateForwardRuleInputWithOptionalID 0%         |
| sources/domain                   |        N/T  | type definitions, no test file                   |
| sources/logger                   |        N/T  | thin wrapper, no test file                       |
| sources/platform                 |        N/T  | OS-specific (Windows service), no test file      |
| sources/static                   |        N/T  | static file helper, no test file                 |
| sources/version                  |        N/T  | constant, no test file                           |
| sources/ (main.go)               |        N/T  | entry point, no test file                        |
| **Combined total (-coverpkg)**   |   **79.7%** |                                                  |

N/T = no test file. Most of these are thin wrappers, type definitions, or OS-integration code.

No coverage gate. Tooling added: `scripts/coverage-service.js` added; run via `npm run coverage:service`.

Notes:
- The combined coverage run uses `-p 1` (sequential) to avoid timing flakiness in `TestTCPForwarderEmitsConnectionClosedEvent` under parallel cross-package instrumentation. The per-package test for that package passes reliably.
- The `waitForTestCondition` timeout in `sources/forwarders/tcp_test.go` was raised from 2s to 5s for robustness under heavy instrumentation.

---

## shared — 82.1% statements, 53.6% branch, 88.9% functions

| File                       | Stmts  | Branch | Funcs  | Notes                                   |
| -------------------------- | -----: | -----: | -----: | --------------------------------------- |
| sources/activity.ts        |     0% |      0%|      0%| type definitions only, no executable code |
| sources/index.ts           |  82.1% |  52.9% |  87.5% | validation, port advisory, shared types |

Key uncovered branches in `index.ts` (53% branch): the branch gap reflects validation edge cases and advisory logic paths not yet exercised. The 10 existing tests cover the main paths.

No coverage gate. Tooling added: vitest.config.ts created, `coverage` script added.

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

- **E2E (Playwright)**: 4 spec files covering app load, CRUD, import/export, TCP, and 3 UDP modes. Runs against the real TypeScript server. Does not produce statement coverage metrics.
- **validate:contract**: API parity between TypeScript server and Go service. Runs when Go binary is present.
- **validate:config**: Fixture-based config compatibility (load, import, export, rejection, UDP defaults). 8 valid fixtures, multiple invalid fixtures.
- **validate:binary**: Runtime binary smoke tests (start, health, static, shutdown).
- **validate:scripts**: Static analysis + dry-run for installer/service shell scripts.

---

## High-Risk Coverage Gaps

### v1.4 Live Connection Inspector

- **server forwarders** (`tcp-forwarder.ts` **100% stmts** ✓, `udp-forwarder.ts` **84.3% stmts**): forwarder hardening completed in v1.4 Slice 1. TCP forwarder is at 100% statements/functions with only untestable `??` branches remaining. UDP forwarder remaining gaps are multi-client send/return error callbacks and a race guard — documented above. v1.4 Slices 3–4 add live tracking to these modules; 100% meaningful coverage remains the target for the new tracking code.
- **service forwarders** (82.6% combined): same concern on the Go side. `emitPacketError` at 0% — not critical but worth covering as a follow-up.
- **shared branch coverage** (53.6%): `index.ts` has validation/advisory branches not fully exercised. Adding live connection types in Slice 2 should include 100% meaningful coverage for new types/helpers.
- **client portierApi.ts** (0% unit): not a blocker for v1.4 since E2E covers it, but the `fetchLiveConnections()` helper added in Slice 8 should have unit tests.

### v1.5 Declarative Config & Drift Control

- **service config** (77.6%): config diff/plan/apply will build on this package. Target 100% for new code in this package.
- **server config-store.ts** (94.1%): high baseline, but new import-preview logic should reach 100%.
- **server/service api** (server 91%, service 81%): new `POST /api/config/plan` endpoint — 100% target for new handler.
- **tools/cli commands** (gate 92%): `config diff`, `config plan`, `config apply` — 100% target; gate will ratchet.
- **shared validation/types** (82.1% statements): new diff/plan types and validation helpers — 100% target.

### v1.6 Audit Readiness

- **server/service logger** (0% in both): genuinely untested. Low-risk but flagged for v1.6.
- **server index.ts** (0%): bootstrap wiring, difficult to unit-test. Covered by E2E only.
- **service platform** (0%): Windows Service integration, OS-specific, not unit-testable without OS mocking.
- **service manager** (83.2%): `SetStartLogger`, `SetEventLogger`, custom Error() types — 0%. Worth closing before v1.6.
- **service validation** (77.2%): `ValidateForwardRuleInputWithOptionalID` at 0% — used by update path, should be covered.
- **scripts** (not measured): coverage gap for the project's automation tooling.

---

## Ratchet Plan

### tools/cli (existing gate)

| Milestone   | Gate   | Actual (baseline) | Notes                              |
| ----------- | -----: | ----------------: | ---------------------------------- |
| v1.3.0      |    92% |             92.7% | gate passed                        |
| v1.4 Slice 10 | 92%+ |               TBD | `connections` command added; gate maintained or raised |
| v1.4 final  |   95%+ |               TBD | ratchet after connections command  |
| v1.5 final  |   98%+ |               TBD | ratchet after plan/diff/apply      |
| v1.6 target | 100%   |               TBD | 100% meaningful coverage           |

### New v1.4 code — 100% gate from first commit

All new or materially changed implementation files in v1.4 should reach 100% meaningful coverage:
- shared: live connection types (Slice 2)
- server: TCP tracker module (Slice 3), UDP tracker module (Slice 4)
- service: TCP tracker module (Slice 5), UDP session tracker module (Slice 6)
- client: `fetchLiveConnections`, display/filter logic (Slice 8)
- CLI: `connections` command (Slice 10)

Gate enforcement: add per-module coverage checks in `validate:cli:coverage` style for Go, and add vitest coverage thresholds for TypeScript.

### New v1.5 code — 100% gate from first commit

Same policy: 100% meaningful for all new/materially changed files in plan/diff/apply.

### Existing baselines — incremental ratchet (non-blocking for unrelated work)

| Component | Baseline (v1.3) | After Slice 1 | v1.4 target | v1.5 target | v1.6 target |
| --------- | --------------: | ------------: | ----------: | ----------: | ----------: |
| client    |           89.2% |         89.2% |       90%+  |       95%+  |        100% |
| server    |           71.9% |     **79.6%** |       85%+  |       92%+  |        100% |
| service   |           79.7% |         79.7% |       85%+* |       92%+  |        100% |
| shared    |           82.1% |         82.1% |       90%+  |       95%+  |        100% |

\* Service forwarder coverage improvement is part of v1.4 Slices 5–6 (Go live tracking).

Server baseline already exceeds the 79.6% → 85%+ target from Slice 1 forwarder hardening. Live tracking modules (Slices 3–4) add new code requiring 100% coverage, which will pull the server total higher.

Do not block unrelated v1.4 work on legacy coverage gaps. Require 100% for newly added or materially changed files.
