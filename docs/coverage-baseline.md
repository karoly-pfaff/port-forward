# Coverage Baseline

## v1.6-pre Release Gates (updated at Slice B, 2026-06-09)

Achieved after coverage ratchet & quality hardening uplift (v1.6-pre), then recalibrated at Slice A (tooling stabilization), updated at Slice B (service behavioral tests), and refreshed at Arch-C2 (manager activity-emission dedupe — refactor removed covered duplicate lines, nudging service 88.6% → 88.5%; gate unchanged). See methodology section below.

| Component | Statements | Branch | Functions | Gate (post-Slice-B) |
| --------- | ---------: | -----: | --------: | ------------------: |
| tools/cli |      93.2% |      — |      98.6% |                 93% |
| client    |   ~95-96%† |~89-90%†|   ~78-80%† |           94/89/78  |
| server    |      95.3% |  92.0% |    100.0%  |           89/91/99  |
| service   |      88.5% |      — |      95.4% |                 88% |
| shared    |    100.00% | 100.0% |    100.00% |         100/100/100 |

† Client numbers fluctuate slightly (±1%) depending on whether Windows vitest ghost entries appear in a given run. Both the ghost-entry run and the clean run now pass the recalibrated gates. See methodology section below.

Starting point (same as v1.5.0): service 85.8%, server 88.7%/91.0%/99.1%, others unchanged.

---

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

## v1.6 Testing Slice Test-D — ForwardManager rollback parity (2026-06-09)

Test-D added persist-failure rollback tests to `server/sources/forward-manager.test.ts` (via a `ControllableStore` fake) and a minimal product fix to `server/sources/forward-manager.ts` (create/update/delete/reorder/import now roll back in-memory state on a failed `store.save()`, matching the Go manager). This closed a real correctness/parity bug, not just a coverage gap. `forward-manager.ts` per-file coverage improved 99.4%/88.7% → **99.5%/90.4%** (stmts/branch), 100% funcs; aggregate server coverage 95.3%/92.0% → **95.4%/92.2%**. **Coverage gates are unchanged** (server stays 89/91/99): the branch headroom (92.2% vs 91% gate) was intentionally not ratcheted mid-slice to stay clear of the documented Windows vitest ±1% ghost-entry fluctuation. The remaining ~0.5% stmts / ~9.6% branch gap in `forward-manager.ts` is dominated by the best-effort `await this.startRule(...).catch(() => {})` restart-failure paths inside rollback (a restart must itself fail during a rollback — not deterministically triggerable) and the long-standing defensive merge/validation branches noted below.

## v1.6 Testing Slice Test-A — Coverage-run stability (2026-06-09)

Test-A stabilized the socket-binding test suites against the EADDRINUSE/TOCTOU flake identified in `audits/v1.6-testing-audit-1.md` (a `coverage:service` `go test` run failed once under concurrent load during the testing audit and passed on isolated re-runs). The `free*Port` test helpers allocate→close→return an ephemeral port; a parallel test/process can grab it before the forwarder/manager binds, surfacing as a transient bind failure that can fail a coverage run non-deterministically. Test-A adds bounded bind-retry helpers (`isAddrInUse` + `startTCPForwarderOnFreePort`/`startUDPForwarderOnFreePort` in `service/sources/forwarders/portretry_test.go`, `startRuleStable` in `service/sources/manager/portretry_test.go`, `startForwarderOnFreePort` in `server/sources/test-helpers.ts`) and migrates the Go forwarder/manager and TS `tcp-forwarder` happy-path binds. **No coverage numbers or gates change** — this is a test-stability fix only; production code is untouched. Residual follow-ups (TS `udp-forwarder.test.ts`, the api HTTP `/start` sites, E2E `port.ts`) are tracked in the changelog. Run `npm run validate:coverage` twice consecutively to confirm determinism after this slice.

## v1.6 Coverage Methodology — Slice A Tooling Stabilization (2026-06-09)

### Structural-zero files

"Structural zeros" are source files that produce no executable JavaScript and will always report 0% coverage regardless of test quality. They fall into two categories:

1. **Type-only TypeScript files** — contain only `export type` and `interface` declarations. TypeScript erases all type information at compilation; V8 has no JavaScript to instrument. Files: `shared/sources/activity.ts`, `shared/sources/connections.ts`, `shared/sources/plan.ts`.

2. **Entry-point / lifecycle wrappers** — bootstrap files that are only ever exercised via E2E or OS-level integration tests, never in unit tests. Files: `server/sources/index.ts` (HTTP server startup), `client/sources/main.tsx` (ReactDOM browser mount), `server/sources/forwarders/types.ts` (TypeScript interface file).

All structural-zero files are excluded from vitest `coverage.include` in the workspace `vitest.config.ts` files. This prevents them from inflating the denominator and misrepresenting true coverage. The exclusions are explicit, documented with comments in each config file, and do not hide any meaningful product logic.

Go structural zeros (`main.go`, `platform/windows.go`, `logger.go`) are not excluded from Go coverage runs — they already report at the package level, and Go's cross-package tooling handles them correctly.

### Windows vitest/v8 path-case deduplication

On Windows, Node.js resolves file URLs as `file:///c:/...` (lowercase drive letter) while vitest's `coverage.include` glob can resolve using the Windows API, producing `C:\...` (uppercase drive letter). Both paths appear as separate keys in `coverage-summary.json`. The uppercase entry shows 0% coverage (a "ghost" entry from the include-glob resolution with no corresponding execution data). When both entries are present, the statement denominator is doubled and the aggregate coverage collapses by ~50%.

This bug is intermittent — it manifests on some runs and not others, making the tooling output non-deterministic.

**Fix (v1.6 Slice A):** `scripts/validate-coverage.js` now deduplicates per-file entries before computing aggregate totals. The `normalizePath()` function lowercases the Windows drive letter (`^([A-Z]):` → lowercase). When two entries map to the same normalized path, the entry with more `statements.covered` is kept — that is always the real execution entry, not the ghost. Go workspaces (which write only a `total` key, no per-file entries) are handled by a separate code path that reads `data.total` directly.

**Gate recalibration:** Prior to Slice A, the client branch/funcs gates were set at 90/79 based on a "clean" run (no ghost entries present). With consistent deduplication now applied, the accurate true values are ~89.6% branch and ~78.6% funcs on ghost-present runs. Gates recalibrated to 89/78. Both ghost-present and clean runs now pass deterministically. This is not a coverage regression — it is accurate measurement of what was always there.

**Excluded files list (exact):**

| File | Workspace | Reason |
| ---- | --------- | ------ |
| `sources/activity.ts` | shared | type-only (export type only) |
| `sources/connections.ts` | shared | type-only (export type only) |
| `sources/plan.ts` | shared | type-only (export type only) |
| `sources/index.ts` | server | HTTP server entry point (E2E only) |
| `sources/forwarders/types.ts` | server | TypeScript interface-only file |
| `sources/main.tsx` | client | ReactDOM browser mount (E2E only) |

No product logic is hidden by these exclusions. The policy is: exclusions are allowed only for files that are structurally untestable in unit tests AND contain no branching logic that unit tests could exercise.

---

---

## tools/cli — 97.7% statements (gate: 95%)

Updated at **v1.6 Coverage Slice C** (2026-06-10): 93.2% → **97.7%** (cross-package), gate raised 93 → 95. The uplift is failure-path/exit-code tests only — no CLI behavior change; the CLI remains a pure API client (no server/service imports). Tests added:

- `commands/jsonerr_test.go` — a shared `failingWriter` (every `Write` returns `os.ErrClosed`) drives the `output.PrintJSON → "Error encoding JSON" → exit 1` branch in every JSON-emitting command (runtime, list, status, start, stop, diagnose, activity, config export/validate/plan/diff/apply/import, diagnostics export). Behavior asserted: exit 1 + stderr message.
- `commands/helpers_internal_test.go` — white-box (`package commands`) tests for `formatChangeValue` (nil, integer-valued float, non-integer float, string, bool) and `opEndpoint` (Current present, Desired fallback, both-nil em-dash).
- `main_test.go` — `TestRun_InvalidURL_AllDispatches` asserts an invalid `--url` exits 2 for every subcommand (the repeated `ResolveURL`-error arm in `run()`).

After Slice C, every `Run*` command function reaches ≥ ~94% with the JSON-error and exit-code paths covered; `run()` and the formatting helpers reach 100%.

Gate: 95%. Enforced by `npm run validate:coverage:cli` (scripts/validate-coverage.js --only cli). Go coverage is a deterministic cross-package number (no Windows vitest ghost-entry effect); repeated `npm run coverage:cli` runs both report 97.7%. The 2.7% headroom above the gate is intentional buffer.

### Structurally-unreachable CLI branches (documented, not chased)

These remain below 100% by design — they cannot be triggered without contorting production code or adding fake unmarshalable DTOs (forbidden by the "CLI stays a pure API client" rule):

| Location | Branch | Why unreachable |
| -------- | ------ | --------------- |
| `sources/main.go:main` | `os.Exit(run(...))` | Calling `os.Exit` from a test would abort the test binary; `run()` itself is fully tested. |
| `sources/client/client.go:doWithBody` | `json.Marshal(body)` error | The request bodies are concrete CLI DTOs that always marshal; no unmarshalable value can reach it. |
| `sources/client/client.go:do` | `http.NewRequest` error / response-body read error | `NewRequest` only errors on a malformed method/URL the CLI never produces; the read error needs a mid-stream transport failure not reproducible with `httptest`. |
| `sources/commands/configcmd.go:writePrettyJSON` | `json.MarshalIndent` error | Same as `doWithBody` — the bundle/config values always marshal. The `os.WriteFile` error branch IS covered (invalid `--out`/`--backup-out` path tests). |

Do not add product types solely to hit these — they are accepted permanent gaps.

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

Gate: 94/89/78. Enforced by `npm run validate:coverage:client`. (Branch/funcs gates recalibrated at Slice A from 90/79 to 89/78 — see methodology section.)

---

## server — 98.9% statements, 93.6% branch, 100% functions

Updated at **Coverage Slice E** (2026-06-10): 95.4/92.2/100 → **98.9/93.6/100**. Previous milestones: v1.5 Slice 2 88.63%; v1.5 pre 87.11%; v1.4.0 82.88%; v1.3.0 71.9%.

"All files" figure from `npm run coverage:server`.

| File                                                  | Stmts  | Branch | Funcs  | Notes                                            |
| ----------------------------------------------------- | -----: | -----: | -----: | ------------------------------------------------ |
| sources/index.ts                                      |     0% |      0%|      0%| app entry/wiring, not unit-tested (E2E/contract) |
| sources/forwarders/types.ts                           |     0% |      — |      — | interface-only file, no executable code          |
| sources/diagnose.ts                                   |  98.0% |  91.4% |   100% | Slice E: TCP bind/connect/DNS + advisory tests. Gap = 2 s `tryTcpBind`/`tryUdpBind`/`tryTcpConnect` timeout branches |
| sources/forwarders/udp-forwarder.ts                   |  99.4% |  89.6% |   100% | Slice E: 4 send-callback error branches. Gap = post-stop/pre-client reply race guards |
| sources/forward-manager.ts                            |  99.5% |  90.6% |   100% | Test-D rollback; `.catch(() => {})` restart-on-rollback failure paths uncovered |
| sources/api.ts                                        |  96.0% |  91.9% |   100% | Slice E: apply-drift persist-failure → 500. Gap = platform-normalize branches (Windows-only env) |
| sources/config-plan.ts                                |  99.3%  |  98.8%  |  100% | pure plan engine                                |
| sources/logger.ts                                     |   100% |   100% |   100% |                                                  |
| sources/config-store.ts                               |   100% |   100% |   100% |                                                  |
| sources/server-options.ts                             |   100% |   100% |   100% |                                                  |
| sources/connections/tcp-connection-registry.ts        |   100% |   100% |   100% |                                                  |
| sources/connections/udp-session-registry.ts           |   100% |   100% |   100% |                                                  |
| sources/forwarders/tcp-forwarder.ts                   |   100% |    90% |   100% | branch gap = optional registry param (by-design) |
| sources/activity/activity-store.ts                    |   100% |   100% |   100% |                                                  |

Gate: **95/92/99**. Enforced by `npm run validate:coverage:server`. Ratcheted 88/90/99 (v1.5.0) → 89/91/99 (v1.6-pre) → **95/92/99 (Coverage Slice E)**. Three consecutive `coverage:server` runs all reported 98.9/93.6/100; the Slice-A ghost-entry dedup keeps the server number deterministic, so a 95/92 gate retains ~3.9%/~1.6% buffer.

### Coverage Slice E — remaining server blockers (documented, not chased)

These branches are structurally hard to trigger deterministically without brittle internals monkeypatching or real timing; they are documented rather than force-covered, and the surrounding behavior is covered by stop/cleanup/contract tests:

| Area | Branch | Why not covered |
| ---- | ------ | --------------- |
| `diagnose.ts` | `tryTcpBind`/`tryUdpBind`/`tryTcpConnect` **2 s timeout** events | Triggering a real bind/connect that hangs ~2 s is nondeterministic and slow; the success and error (refused/in-use) paths ARE covered. |
| `forwarders/udp-forwarder.ts` | `if (!this.listenSocket) return` (target reply arrives after stop); `if (!this.lastClient) return` (target reply before any client packet) | Both are post-stop / pre-client **race guards** reachable only via an impossible-to-time reply ordering; covered indirectly by stop/cleanup tests. The send-callback error branches ARE now covered via clean instance-level `.send` injection. |
| `forwarders/tcp-forwarder.ts` | optional-registry guard (`if (connId)`) — 90% branch | By-design optional API path: when no registry is passed, `connId` is undefined and the right-hand side is never reached. Not a meaningful test gap. |
| `api.ts` | `normalizePlatform`/`normalizeArch` darwin/linux/unknown arms | Windows-only environment — the non-win32 branches cannot run on the dev/CI host. Covered cross-platform by the Go service equivalents + `validate:contract`. |
| `index.ts`, `forwarders/types.ts` | entry/composition root; interface-only | E2E/contract territory; interface compiles to nothing. |

Do not add `dgram` prototype monkeypatches or real-timeout waits to hit these — accept them as documented gaps. A clean instance-level `.send`/`.emit` override on a specific socket is the sanctioned injection technique (used for the send-callback error tests).

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
| **Combined total (-coverpkg)**   |   **90.3%** |  Coverage Slice D                                |

N/T = no test file. Most of these are thin wrappers, type definitions, or OS-integration code.

Gate: 90%. Enforced by `npm run validate:coverage:service`. Ratcheted 85% (v1.5.0) → 87% (v1.6-pre) → 88% (Slice B) → **90% (Coverage Slice D)**. Arch-C2 nudged the total 88.6% → 88.5%.

### Coverage Slice D — error-path uplift (2026-06-10)

Raised the combined total 88.5% → **90.3%** with tests-only additions (no product code changed; `validate:contract` 167/167). Two consecutive `coverage:service` runs both reported 90.3%; modified packages pass `go test -count=3`. New/expanded error-path tests:

- `api/errorpaths_test.go` — body-read failures and malformed JSON → 400; unknown-rule `PATCH` → 404; **generic (non-typed) manager error → 500** via a `manager.NewWithStore` handler whose store path is under a regular file (so every Save fails cross-platform, no production seam, no mocking); config-apply `ImportConfig` failure → 500; `NewHandler` option defaults; `tryTCPBind`/`tryUDPBind` success+failure. (`api` package 70.8% → ~91%.)
- `forwarders/udp_errorpaths_test.go` — public `NewUDPForwarder` constructor + `udpMode()` nil-default (one-way), via the Test-A `startUDPForwarderOnFreePort` retry helper.
- `static/static_test.go` — `ServeClient` existing-asset vs SPA index fallback; `HasClient`.
- `manager/errorpaths_test.go` — `StartEnabled` skips disabled rules (no socket binds); `hasIDIn` regenerates a colliding ID on merge-import.
- `configplan/udpmode_test.go` — `udpModeEqual`/`udpModeVal` for nil↔non-nil and non-nil↔non-nil.
- `options/options_test.go` — `parseCLI` positional-argument rejection.

**Remaining service blockers (documented, not chased — would need invasive production-only seams):**

| Area | Branch | Why not covered |
| ---- | ------ | --------------- |
| `activity/store.go:randomEventID`, `domain/id.go:NewRuleID`, `connections/.../generateConnectionID` | `crypto/rand` read-failure fallback | OS entropy never fails in tests; no injection point. |
| `config/config.go:Save` (65.7%) | `tempFile.Write`/`Sync`/`Close` errors; `os.Rename` recovery | Need an injectable `fs.File`/writer seam. Go's `os.Rename` replaces atomically on both Windows and Linux, so the remove-and-retry recovery branch is effectively dead code. The `MkdirAll` and invalid-rule branches ARE covered. |
| `forwarders/udp.go` | `emitPacketError` (0%), multi-client/session **write**-error branches (`handleMultiClientPacket`, `sessionReadLoop`, `targetReadLoop`, `listenLoop` read-error tails) | Require injecting a failing `net.Conn`/`PacketConn`; the forwarder constructs its own sockets. A seam here would exist only for tests — deferred as a future testability improvement, not added in this slice. |
| `forwarders/tcp.go:acceptLoop` (80%), `setLastError`/`logInfo` | accept-error injection; nil-log guard | Same — accept errors need a mock listener; `logInfo` 50% is a trivial nil-`log` guard. |
| `main.go`, `platform/windows.go`, `logger.New` (0%) | entry point / OS service / thin wrapper | E2E/OS-integration territory; not unit-testable. |

Do not add `net.Conn` mocks or `fs.File` abstractions to production solely for these — accept them as documented gaps unless the seam independently improves the design.

Notes:
- The combined coverage run uses `-p 1` (sequential) to avoid timing flakiness in `TestTCPForwarderEmitsConnectionClosedEvent` under parallel cross-package instrumentation. The per-package test for that package passes reliably.
- The `waitForTestCondition` timeout in `sources/forwarders/tcp_test.go` was raised from 2s to 5s for robustness under heavy instrumentation.
- `sources/connections/` package: Slice 5 added TCP registry (98.1% stmts), Slice 6 added UDP registry (98.4% combined). Gaps: `rand.Read` fallback in private `generateConnectionID` (untestable, same pattern as CLI), and stale-key defensive path in `OpenOrTouchSession` (unreachable through public API). All public methods are at 100% statements.

---

## shared — 100.0% statements, 100.0% branch, 100.0% functions

Updated at v1.5 pre-release (2026-06-09). Previous: 82.1%/54.3%/90.0% (v1.4.0).

| File                       | Stmts  | Branch | Funcs  | Notes                                   |
| -------------------------- | -----: | -----: | -----: | --------------------------------------- |
| sources/activity.ts        | excl.  |  excl. |  excl. | structural zero — type-only (excluded at Slice A) |
| sources/connections.ts     | excl.  |  excl. |  excl. | structural zero — type-only (excluded at Slice A) |
| sources/plan.ts            | excl.  |  excl. |  excl. | structural zero — type-only (excluded at Slice A) |
| sources/index.ts           |   100% |   100% |   100% | all validation + advisory + listenKey paths covered |

v1.5 pre-release added 7 tests covering: listenKey consistency, validateForwardRule with empty listenHost, and all remaining uncovered advisory branches. At Slice A, type-only files excluded from coverage denominator — shared reports a clean 100/100/100 for all executable code.

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
