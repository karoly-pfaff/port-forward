# QA Checklist

## Automated Release Validation

Run these before tagging:

- [ ] `npm run lint`
- [ ] `npm run typecheck`
- [ ] `npm run test`
- [ ] `npm run build`
- [ ] `npm run build:client`
- [ ] `npm run test:e2e`
- [ ] `npm run check`
- [ ] `go test ./...` from `service/`
- [ ] `go build ./...` from `service/`
- [ ] `npm run build:service`
- [ ] `npm run validate:runtime:smoke` — builds `build/portier/`, validates layout, runs smoke test

## Additional Validation Suites

Run explicitly — not part of `npm run check`. Slower or platform-sensitive.

- [ ] `npm run validate:config` — Config compatibility: loads every fixture from `tests/fixtures/config/`, verifies valid fixtures load and import correctly, invalid fixtures are rejected with appropriate errors, duplicate bindings are caught, UDP mode defaults are applied, and the export shape is stable. TypeScript runtime always checked; Go runtime checked when binary is available. (`scripts/validate-config.js`)
- [ ] `npm run validate:contract` — API contract parity: runs all API scenarios against TypeScript server; if Go binary available, runs same scenarios against Go service and compares response shapes, status codes, field names, and error shapes. (`scripts/validate-contract.js`)
- [ ] `npm run validate:binary` (or `validate:runtime:behavior`) — Runtime binary behavior: starts `build/portier/service[.exe]`, verifies health, static serving, missing-static-dir handling, invalid-config failure, and clean shutdown. Runs `build:runtime` first unless `--no-build` is passed. (`scripts/validate-binary.js`)
- [ ] `npm run validate:scripts` — Installer script analysis: static analysis of all platform install and validation scripts (no firewall commands, test-specific names in validate scripts, production path defaults in install scripts, quoting correctness); plus dry-run execution on the current platform. (`scripts/validate-scripts.js`)

## Explicit OS Service Install Validation

These commands must be run explicitly on the target platform before distribution. They are not run by `npm run check`.

Each script uses test-specific service names, ports, and temp directories. Production Portier installs and config are never touched.

- [ ] `npm run validate:service:windows:user` — Windows scheduled task (no Administrator required)
- [ ] `npm run validate:service:windows:machine` — Windows Service (Administrator required)
- [ ] `npm run validate:service:macos` — macOS LaunchAgent (no sudo required)
- [ ] `npm run validate:service:linux` — Linux systemd unit (root/sudo required)

Each script validates:
1. Package copy to isolated temp install dir
2. Service/task/agent registration with test-specific name
3. Service start
4. `/api/health` responds 200
5. Web UI HTML served at `/`
6. Service stop
7. Service/task/agent unregistered and verified removed
8. Temp files cleaned up

Flags supported by all scripts:
- `--no-build` / `-NoBuild` — skip `npm run build:runtime`, use existing `build/portier/`
- `--keep-files` / `-KeepFiles` — preserve temp directories on failure for debugging
- `--port` / `-Port` — override the management port (default: auto-detect free port)

## Coverage Tooling Notes

- [x] **v1.6 Fix Slice 8 — Portier terminology glossary (Naming-A)** — Docs-only; no code/API/test/gate change, no renames. Added `docs/glossary.md` (canonical terms: forward rule, config plan/apply/import/export, desired/exported config, drift, runtime/server/service/`node`/`go`, enabled vs running, activity event/type/severity, advisory vs warning, diagnose vs diagnostics export, live connection vs UDP session) with the frozen public-API exceptions documented (`/api/forwards`, `listenHost`/`targetHost`, `clientAddress`/`targetAddress`, CLI commands). Pointers added from `CLAUDE.md`, `AGENTS.md`, and `docs/api-contract.md`; glossary text kept in one place. **New docs/API/CLI/UI wording should follow `docs/glossary.md`, or update the glossary when introducing a genuinely new public term.** CLI file naming (Naming-C) and UI wording (Naming-E) deferred. Source: `audits/v1.6-readability-naming-audit-1.md`. Next: v1.6 release-readiness checkpoint.
- [x] **v1.6 Fix Slice 7 — Config apply `importConfig` result.errors invariant (Error-C·2/Resilience-C)** — Apply can never report `ok:true` when the underlying import reports errors. Both handlers discarded the `ImportResult`; audit confirmed no `result.errors` path is currently reachable from apply (validation pre-validated, duplicate bindings pre-blocked by `detectDuplicateKeys`→`hasErrors`, persist throws→500, merge-only N/A in replace), but the discarded result left the invariant unenforced. Defensive fix in `server/sources/api.ts` + `service/sources/api/api.go`: after import, if `result.errors` non-empty → `200 ok:false` with errors surfaced via existing `plan.errors` (`IMPORT_ERROR`) + `hasErrors:true` + zero counts (no new response field). Go construction extracted to unit-testable `applyImportErrorResponse` (handler branch structurally unreachable without the deferred manager-interface refactor; documented in `docs/coverage-baseline.md`). Tests: TS duplicate-binding pre-block + defensive branch via `ImportErrorManager` subclass; Go duplicate-binding pre-block + `TestApplyImportErrorResponse`. `validate:contract` 183 → 185 (duplicate-binding apply parity, both runtimes). CLI unaffected (`applyExitCode` already maps `ok:false`→exit 1). No API-shape/valid-behavior change; server branch 94.0→94.1, service 90.3→90.2, gates unchanged; full test 357→359. Durable rule in `CLAUDE.md`/`AGENTS.md`. Source: error-flow/resilience audits. Next: Fix Slice 8 — Naming-A glossary.
- [x] **v1.6 Fix Slice 6 — TS `ForwardManager.emitRuleEvent` helper (Dup-C2/Naming-B/Error-B; Arch-C2 symmetry)** — Behavior-preserving refactor; payloads byte-identical, no API/type/severity/message/order/count change. Added private `emitRuleEvent(type, severity, rule, message)` to `server/sources/forward-manager.ts` (mirrors Go `manager.emitRuleEvent`; populates ruleId/ruleName/protocol; keeps `activity?.add` optional-chaining; ActivityStore still owns id/timestamp) and replaced the 6 inline rule-scoped blocks (created/updated/deleted/started/stopped/error). The 5 config-level events (`config.exported`/`config.imported`/3× `config.import.failed`) stay on direct `activity?.add` (not rule-scoped, carry `details`), mirroring Go. Added 3 full-payload regression tests (create/update/delete; start/stop via `startRuleStable`; start-failure rule.error) asserting exact type/severity/ruleId/ruleName/protocol/message. server 98.9/93.8/100 → 98.9/94.0/100, **gates unchanged**; `validate:contract` 183/183; full test 354 → 357. Durable rule in `CLAUDE.md`/`AGENTS.md`. Source: duplication/readability/error-flow audits. Next: Fix Slice 7 — config apply result.errors general case.
- [x] **v1.6 Fix Slice 5 — Activity event-type/severity value parity guard (Error-A/Resilience-E/Dup-G)** — Contract/script hardening only; no product/API-shape/gate change. Added canonical `EXPECTED_ACTIVITY_TYPES` (17) / `EXPECTED_ACTIVITY_SEVERITIES` (4) to `scripts/validate-contract.js` + three guard layers: (1) once-run source-of-truth parity (`runActivityValueSetParityChecks`) reads the TS union + Go consts from source (binary-independent) and asserts both declared sets equal the canonical set — drift fails with `extra`/`missing` diff (negative-tested `rule.error→rule.failure`); (2) per-runtime membership (every emitted type/severity ∈ allowed set) in `runActivityScenarios`; (3) per-runtime representative emitted values — `rule.created/updated/started/stopped` (success/info/success/info) and the import error path `config.imported/success` + `config.import.failed/error`. No assertions weakened, no event names changed. **`validate:contract` 171 → 183** (intentional: +4 once, +6 membership/representative ×2, +2 config ×2; stable ×2 runs); `\x00` separators preserved. `docs/api-contract.md` lists the value sets + parity rule. Lint/typecheck/coverage/test green. Durable rule in `CLAUDE.md`/`AGENTS.md`. Source: error-flow/resilience/duplication audits. Next: Fix Slice 6 — TS ForwardManager emitRuleEvent helper.
- [x] **v1.6 Fix Slice 4 — `validate-contract.js` scenario registry (Complexity-A/SOLID-A/Pattern-A/Dup-A/Naming-D)** — Script-only refactor; no product/API/contract/gate change, `validate:contract` 171/171 (run twice). Split the ~1090-line `runScenarios` into 12 named group functions (`runRuntimeScenarios`, `runForwardsScenarios`, `runForwardLifecycleScenarios`, `runActivityScenarios`, `runConfigExportImportScenarios`, `runPortAdvisoryScenarios`, `runForwardDeleteScenarios`, `runConnectionsScenarios`, `runConfigPlanScenarios`, `runConfigApplyScenarios`, `runErrorEnvelopeScenarios`, `runDiagnoseScenarios`) driven by an ordered `scenarioGroups` registry; each takes a small `ctx` (`{api, runtime, fixtureDir, state}`); cross-group rule ids flow via `ctx.state` (local `exportedConfig` stays local). Added `expectedRuntimeApiValue(label)` helper for the runtime-label→API-value mapping (Naming-D). Assertions moved verbatim (byte-level transform preserving the intentional `\x00` sort-key separators); parity comparison + CLI DTO guard + child cleanup untouched (optional outer-guard hardening deferred as Resilience-F). Lint/typecheck/coverage/full-test all green. Durable rule in `CLAUDE.md`/`AGENTS.md`. Source: complexity/SOLID/pattern/duplication/readability audits. Next: Fix Slice 5 — activity event-type/severity value parity guard.
- [x] **v1.6 Fix Slice 3 — Finish Test-A EADDRINUSE residual helper migration (test determinism)** — Tests/test-infra only; no product/API/contract/gate change. New `server/sources/test-helpers.ts` helpers: `startRuleStable` (mirrors Go `portretry_test.go` — retries `manager.startRule` only on EADDRINUSE by rebinding `listenPort` via `updateRule`), `startTcpServerOnFreePort`/`bindUdpSocketOnFreePort` (bind port 0, return the live listener/socket + actual port). Migrated `udp-forwarder.test.ts` (listen binds → `startForwarderOnFreePort` via local `startUdpForwarder`; UDP targets → `bindUdpSocketOnFreePort`; some fixed sleeps → `waitUntil`) and `forward-manager.test.ts` (manager `startRule` incl. Test-D rollback tests → `startRuleStable`; TCP targets → `startTcpServerOnFreePort`; removed local `startTcpTarget`). Intentional-conflict tests kept outside the helpers (UDP blocker, manager occupier, race-tolerant merge rollback). 58 tests pass ×3; server 354 tests, 98.9/93.8/100 unchanged; `validate:contract` 171/171, `validate:config` 71/71, all gates unchanged. Accepted bounded residuals (documented): HTTP `/start` sites in `api.test.ts`/`api_test.go` (bind behind handler → needs an HTTP-level recreate-on-EADDRINUSE helper, larger follow-up) and E2E `port.ts` (serial `workers:1` → no intra-process race). Durable rule in `CLAUDE.md`/`AGENTS.md`. Source: `audits/v1.6-audit-synthesis-and-fix-plan.md`. Next: validate-contract.js scenario registry (Complexity-A/SOLID-A/Pattern-A/Dup-A/Naming-D).
- [x] **v1.6 Fix Slice 2 — TS `ConfigStore` atomic write parity with Go (durability)** — Closed Resilience-A. `server/sources/config-store.ts` `save` now writes crash-safely: unique same-directory temp file → `fsync` via FileHandle → atomic `rename` over `rules.json` (mirrors Go `config.Store.Save`). Previous file intact until rename; temp removed on any pre-rename failure (best-effort, never masks the original error); persistence error propagates so `ForwardManager` rollback (Test-D) runs. `load` unchanged; serialized output byte-identical. No remove-and-retry recovery branch (Node `fs.rename` replaces atomically on POSIX + Windows `MoveFileExW REPLACE_EXISTING`); directory fsync omitted (Windows portability — same limit Go accepts). Minimal test-only `ConfigStoreFileOps` seam (optional 2nd constructor arg, defaults to real fs; not in any product/API path) forces write/sync/rename failures. 5 new tests (success leaves no temp; write-fail keeps existing file byte-for-byte + temp cleaned; sync-fail cleanup; rename-fail keeps target + removes temp; first-save-fail creates no target). `config-store.ts` 100/100/100; server 98.9/93.6/100 → 98.9/93.8/100; **all gates unchanged**. `config-store.test.ts` ×4 stable; `validate:config` 71/71; `validate:contract` 171/171. Durable rule in `CLAUDE.md`/`AGENTS.md`. Source: `audits/v1.6-resilience-audit-1.md`. Next: finish Test-A EADDRINUSE residual helper migration.
- [x] **v1.6 Fix Slice 1 — TS replace-import duplicate listen-binding parity (real parity bug)** — Closed the one MUST-do correctness/parity divergence (Error-flow #1 / Resilience-B). Go rejected duplicate listen bindings within the imported set for both modes; TS replace mode had no guard and merge only checked imported-vs-existing. Product fix in `server/sources/forward-manager.ts`: module-level `ensureNoDuplicateBindings(rules)` (mirrors Go wording) runs in `importConfig` after validation, before any mutation, for both modes → `{imported:0, errors:[msg]}` + one `config.import.failed` (error), no mutation/persist/start/stop. HTTP maps `result.errors` → 422 `{errors, result}` (both runtimes, unchanged). 3 TS unit tests + `validate:contract` parity scenario (167 → 171). Config **apply** verified already-protected (plan engine `detectDuplicateKeys` → `hasErrors` → `ok:false` before `importConfig`) — resolves the duplicate-binding half of Resilience-C. No valid-import / merge-vs-existing / gate change. Durable rule in `CLAUDE.md`/`AGENTS.md`. Source: `audits/v1.6-audit-synthesis-and-fix-plan.md`. Next: Resilience-A (TS ConfigStore atomic write parity).
- [x] **v1.6 Testing Slice Test-E — Client/E2E meaningful coverage (32 → 34 E2E, brittle selectors cleaned)** — Tests-only; no product code changed, client coverage unchanged (95.2/90.2/80.3), `validate:contract` 167/167. Added `connections.spec.ts` (I) populated TCP table (real held connection → populated row, empty state gone) and (J) API-failure `role="alert"` banner (Playwright `page.route` abort). Removed brittle `label.auto-refresh-toggle` (→ aria-label attribute + visible-label click) and `.diag-panel-title`/`.diag-panel-body` (→ accessible "Close diagnostics" button + "Listen address" check). Full E2E 34/34, run twice (real socket + auto-refresh polling, no fixed sleeps). Remaining E2E gaps in `docs/e2e-coverage.md` (UDP populated table deferred, duplicate-binding error, a11y/keyboard smoke). Source: `audits/v1.6-testing-audit-1.md`. Next: short v1.6 readiness audit/checkpoint (summarize hardening slices, verify gates, list accepted gaps, decide if more coverage slices add value before release).
- [x] **v1.6 Coverage Slice E — TypeScript server error paths (95.4/92.2/100 → 98.9/93.6/100, gates 89/91/99 → 95/92/99)** — Tests-only; no product code changed, `validate:contract` 167/167. `diagnose.test.ts` (→98.0%): TCP bind-fail, target-connect success/refused, DNS-fail→skip, one-way mode, 0.0.0.0/privileged/common-port advisory warns. `udp-forwarder.test.ts` (→99.4%): four send-callback error branches (one-way/multi-client send, last-client/multi-client return) via clean instance-level `.send` injection + `waitUntil` deadline polling. `api.test.ts`: apply-drift persist-failure → 500 (Go-parity, inline failing `RuleStore`). Three consecutive `coverage:server` runs stable; gates raised statements 89→95, branches 91→92 (functions 99). Remaining blockers documented in `docs/coverage-baseline.md` (2 s diagnose timeouts, UDP post-stop/pre-client race guards, tcp optional-registry guard, api.ts platform-normalize). Source: `audits/v1.6-coverage-audit-1.md`, `audits/v1.6-testing-audit-1.md`. Next: client/E2E thinness slice (Live Connections populated-table E2E, offline/error-banner E2E, brittle selector cleanup).
- [x] **v1.6 Coverage Slice D — Go service error paths (88.5% → 90.3%, gate 88 → 90)** — Tests-only error/correctness-path coverage; no product code changed, `validate:contract` 167/167. New `api/errorpaths_test.go` (body-read/malformed → 400, unknown rule → 404, generic manager error → 500, config-apply import failure → 500, `NewHandler` defaults, `tryTCPBind`/`tryUDPBind`), `forwarders/udp_errorpaths_test.go` (`NewUDPForwarder` + `udpMode` nil-default), `static/static_test.go` (`ServeClient`/`HasClient`), `manager/errorpaths_test.go` (`StartEnabled` skip-disabled, `hasIDIn` merge collision), `configplan/udpmode_test.go` (`udpModeEqual`/`udpModeVal`), and an `options` positional-arg test. Persist-failure 500s use a `manager.NewWithStore` bad-path store (no mocking, cross-platform). Two consecutive `coverage:service` runs both 90.3%; modified packages stable at `-count=3`; gate raised 88 → 90. Remaining blockers documented in `docs/coverage-baseline.md` (crypto/rand fallbacks, `config.Save` injection, UDP read-loop send errors — all need seams not worth adding). Source: `audits/v1.6-coverage-audit-1.md`. Next: server high-value gaps (UDP forwarder send-error callbacks, diagnose timeout/error paths).
- [x] **v1.6 Coverage Slice C — CLI command edge cases (93.2% → 97.7%, gate 93 → 95)** — Raised CLI coverage past the audit's 95% target with failure-path/exit-code tests only; no CLI behavior change, CLI stays a pure API client. Added: `commands/jsonerr_test.go` (a shared `failingWriter` covering the `output.PrintJSON → "Error encoding JSON" → exit 1` branch in all 14 JSON-emitting commands), `commands/helpers_internal_test.go` (white-box `formatChangeValue` + `opEndpoint` branch tests), and `main_test.go` `TestRun_InvalidURL_AllDispatches` (invalid `--url` → exit 2 for every subcommand). Gate raised 93 → 95 in `scripts/validate-coverage.js` (Go cross-package number is deterministic; repeated `coverage:cli` runs both report 97.7%, 2.7% buffer). Structurally-unreachable branches documented in `docs/coverage-baseline.md` (not chased): `main()` os.Exit, `client.do`/`doWithBody` request/marshal errors, `writePrettyJSON` marshal error. `validate:contract` 167/167 unchanged (no production code touched). Source: `audits/v1.6-coverage-audit-1.md`. Next: service/server high-value gaps (service manager/forwarder/API error paths; server UDP send-error + diagnose timeout paths).
- [x] **v1.6 Testing Slice Test-D — TypeScript ForwardManager persist-failure rollback parity** — Closed the HIGH parity gap from `audits/v1.6-testing-audit-1.md`. New `ControllableStore`-driven rollback tests in `server/sources/forward-manager.test.ts` exposed a **real bug**: the TS `ForwardManager` mutated its in-memory rule map before `persist()` with no rollback (Go rolls back), so a failed `store.save()` left in-memory rules inconsistent with `rules.json`. Minimal product fix in `server/sources/forward-manager.ts` mirroring Go: `addRule`/`updateRule`/`deleteRule`/`reorderRules`/`importConfig` restore prior state on persist failure (and best-effort restart a forwarder stopped for a forwarding-field update). 8 new tests mirror the Go `Test*PersistFailureRollsBack` scenarios plus two TS-side extensions (delete-running, import rollback). `forward-manager.ts` 99.4%/88.7% → 99.5%/90.4% stmts/branch, 100% funcs; server gate 89/91/99 unchanged (95.4%/92.2%/100% actual). No API/forwarding/plan-apply change; rollback only triggers on disk-write failure. `vitest forward-manager.test.ts` ×3 stable; `validate:coverage` + `validate:contract` (167/167) pass. Durable rule in `CLAUDE.md`/`AGENTS.md`. Source: `audits/v1.6-testing-audit-1.md`. Next: resume coverage push (CLI command edge cases → 95%+, then service/server high-value gaps from `audits/v1.6-coverage-audit-1.md`).
- [x] **v1.6 Testing Slice Test-A — Stabilized test port allocation (EADDRINUSE/TOCTOU)** — Removed the allocate→close→rebind flake flagged in `audits/v1.6-testing-audit-1.md`. Bounded bind-retry helpers (Pattern C — retry the bind step only, only on EADDRINUSE, never whole tests/suites): `isAddrInUse` + `startTCPForwarderOnFreePort`/`startUDPForwarderOnFreePort` (`service/sources/forwarders/portretry_test.go`), `startRuleStable` (`service/sources/manager/portretry_test.go`, rebinds via `UpdateRule`), `startForwarderOnFreePort` (`server/sources/test-helpers.ts`). Migrated all happy-path binds in Go `tcp_test.go`/`udp_test.go`/`manager_test.go` and TS `tcp-forwarder.test.ts` (intentional bind-failure tests left untouched). `scripts/validate-contract.js` `startServer` retries the child bind on a fresh port; `waitForReady` bails on child exit. Removed the 20 ms ordering `time.Sleep` in `udp_test.go` → deadline poll. Verified with `go test ./... -count=3` (forwarders/manager) and vitest forwarder suites; full `npm run validate:coverage` run twice consecutively. Test-only; no coverage numbers or gates change. Residual follow-ups (tracked): TS `udp-forwarder.test.ts`, the api HTTP `/start` sites (`api_test.go`/`api.test.ts`), E2E `port.ts` (serial `workers:1` → no intra-suite race). Durable testing rule in `CLAUDE.md`/`AGENTS.md`. Source: `audits/v1.6-testing-audit-1.md`. Next: Test-D (TS persist-failure rollback parity tests matching the Go manager).
- [x] **v1.6 Architecture Slice Arch-D — CLI DTO live-runtime parity guard** — `validate:contract` now captures live JSON from both runtimes (runtime/forwards/status/activity/export/plan/apply-dry-run/advisory/diagnose) and strictly decodes them (`DisallowUnknownFields`) into the CLI DTOs via `TestCLIDTOContractParity` (`tools/cli/sources/client/contract_decode_test.go`). Proves the CLI's third contract copy stays in sync with real runtime output, not just `httptest` mocks. Env-gated (`PORTIER_CLI_CONTRACT_FIXTURES`), so `test:cli`/`validate:cli` skip it; CLI stays a pure API client (no service imports). `validate:contract` 167 passed; no coverage/gate change. `/api/connections` out of scope (no CLI DTO). Durable rule: CLI DTOs guarded against live responses; new API response families must update both the capture and the decode guard. Arch-D completes the audit's contract-parity remediation (Arch-A/B/C1/C2/D done). Source: `audits/v1.6-architecture-audit-1.md`. Next: resume coverage push (CLI command edge cases → 95%+, then service/server high-value gaps from `audits/v1.6-coverage-audit-1.md`).
- [x] **v1.6 Architecture Slice Arch-C2 — Collapsed Go manager activity-emission blocks** — Replaced the 10 repeated rule-scoped emission blocks in `manager.go` with one `emitRuleEvent(eventType, severity, rule, message)` helper (create/update/delete/start/stop/error). Payloads preserved byte-for-byte. The 5 config-level emissions (export/import/import-failed) carry a `details` map and no rule fields, so they stay on `emitActivity` directly (documented). Added full-payload regression tests (`TestCreateRuleActivityPayload`, `TestUpdateRuleActivityPayload`, `TestDeleteRuleActivityPayload`, `TestStartStopRuleActivityPayload`, `TestFailedStartActivityPayload`) + `assertRuleEventPayload` asserting type/severity/ruleId/ruleName/protocol/message and no-details. `validate:contract` 166/166; service 88.6% → 88.5% (covered duplicate lines removed; still above 88% gate, no gate change). Durable rule: rule-scoped manager activity events go through `emitRuleEvent`; activity payload changes are user-visible diagnostics needing payload-level tests. Source: `audits/v1.6-architecture-audit-1.md`. Arch-C complete (C1 + C2). Next: Arch-D (CLI DTO parity guard) or resume coverage push (CLI command edge cases).
- [x] **v1.6 Architecture Slice Arch-C1 — Unified Go rule ID generation + dead-code removal** — Replaced duplicate UUID generators `manager.randomID` and `api.newApplyRuleID` with one shared `domain.NewRuleID()` (`service/sources/domain/id.go`); canonical impl keeps the timestamp fallback the manager already had (api path previously ignored RNG errors). Call sites: `CreateRule`, `ImportConfig` merge, and `configApply` (via `BuildApplyImportFromPlan`'s injectable id seam from Arch-B). Removed unused `crypto/rand`/`encoding/hex` imports and the no-op `name` assignment in `buildRuleLiveSummary`. Added `domain/id_test.go` (non-empty, UUID-v4 format, uniqueness). Behavior preserved exactly; `validate:contract` 166/166; coverage neutral (service 88.6%); no gate change. Durable rule: Go service rule ID generation must have one shared implementation so manager-created and apply-created IDs cannot drift in format. Source: `audits/v1.6-architecture-audit-1.md`. Next: Arch-C2 (collapse repeated Go manager activity-emission blocks into one helper, preserving event payloads exactly).
- [x] **v1.6 Architecture Slice Arch-B — Config apply orchestration extracted** — Apply transformation moved from the HTTP handlers into tested plan-engine helpers: `buildApplyImportFromPlan` (`server/sources/config-plan.ts`) and `BuildApplyImportFromPlan` (`service/sources/configplan/plan.go`). Helpers are pure (derive replace rule list + applied counts; inject/preserve ids; no manager/import, no mutation, no file/socket I/O); id generator injectable for tests, defaults to existing per-runtime generator (no new UUID impl). Handlers keep only request/response/gating concerns. 10 TS + 8 Go helper unit tests added. Behavior preserved exactly; `validate:contract` 166/166; server coverage 95.2/91.6 → 95.3/92.0; service unchanged 88.6%; no gate change. Durable rule: apply orchestration lives beside the plan engine, not in HTTP handlers; TS and Go helpers stay semantically mirrored; `validate:contract` is the parity guard. Source: `audits/v1.6-architecture-audit-1.md`. Next: Arch-C (Go service safe dedupe — unify UUID generation, collapse repeated manager activity-emission blocks).
- [x] **v1.6 Architecture Slice Arch-A — Contract drift guard** — Aligned the Go service `LAN_EXPOSURE` advisory message (`service/sources/advisory/advisory.go`) to the canonical TypeScript wording in `@portier/shared`; code/severity unchanged. Strengthened `scripts/validate-contract.js` with in-runtime canonical advisory/plan-warning assertions plus a cross-runtime `compareParity` pass that diffs normalized advisory and config-plan payloads field-by-field (timestamps/ids excluded). Added `TestLANExposureCanonicalContent` / `TestManagementLANExposureCanonicalContent` Go tests. `validate:contract` 166/166 against both runtimes; guard verified to fail against the stale pre-alignment binary. No API shape/behavior change, no plan/apply semantic change, no coverage gate change. Source: `audits/v1.6-architecture-audit-1.md`. Next: Arch-B (extract config apply orchestration into tested plan/apply engine functions in both runtimes).
- [x] **v1.6 Slice B — Service manager rollback and config error-path tests** — 9 new Go tests: 7 in `manager_test.go` (CreateRule/UpdateRule/DeleteRule/ReorderRules persist-failure rollback, UpdateRule restart-after-rollback, NewFromConfig load error, StartEnabled bind failure), 2 in `config_test.go` (Load non-ErrNotExist error, Save MkdirAll failure). Service coverage 87.7% → 88.6%, gate raised 87% → 88%. All tests pass without sockets or permissions tricks.
- [x] **v1.6 Slice A — Coverage tooling stabilized** — Windows vitest/v8 path-case deduplication bug fixed in `scripts/validate-coverage.js`. Structural-zero files excluded from vitest `coverage.include` in shared/server/client workspaces. Client branch/funcs gates recalibrated to accurate post-dedup values (89/78). Two consecutive `npm run validate:coverage` runs both pass. See `docs/coverage-baseline.md` methodology section.

## Automated Coverage Confirmed

Shared and TypeScript coverage:

- [ ] Shared validation and port advisories
- [ ] TypeScript server CRUD HTTP layer
- [ ] TypeScript server import/export HTTP layer
- [ ] TypeScript server status, start/stop, reorder, duplicate binding, and static serving behavior
- [ ] Client App integration flows
- [ ] Settings import/FileReader flow
- [ ] Dashboard, Activity, Settings, API Docs, Forward Rules, and drawer flows
- [ ] Rule diagnostics UI: Diagnose button, loading state, pass/warn/fail/skip panel, error display, duplicate prevention, clear on close/delete
- [ ] Diagnostics export: Download Diagnostics JSON button in Settings, bundle structure (schemaVersion, runtime, rules, statuses, activity, diagnostics, metadata), partial-failure errors array, empty-diagnostics note, filename pattern, disabled-while-generating
- [ ] **API documentation rule:** when an API endpoint is added, removed, or changed — both `docs/api-contract.md` and the client in-app API Docs view (`client/sources/features/apidocs/ApiDocsView.tsx`) must be updated, along with `ApiDocsView.test.tsx`.
- [x] **TypeScript server forwarder coverage hardening (v1.4 Slice 1)** — tcp-forwarder.ts raised to 100% statements/functions; udp-forwarder.ts raised to 84.3% statements/100% functions. 7 new TCP tests + 13 new UDP tests. Remaining UDP gaps (multi-client send/return error callbacks, race guard) documented in `docs/coverage-baseline.md`. Server overall: 71.9% → 79.6% stmts.
- [x] **TypeScript server UDP session tracking (v1.4 Slice 4)** — added `UdpSessionRegistry` (`server/sources/connections/udp-session-registry.ts`): runtime-local UUID IDs, composite session keys (`ruleId:mode:clientAddress:clientPort`), `openOrTouchSession`/`recordInbound`/`recordOutbound`/`closeSession`/`closeSessionsForRule`/`pruneExpired`/`snapshot`/`snapshotForRule` API. Constants: `UDP_SESSION_IDLE_MS = 30_000`, `UDP_SESSION_EXPIRE_MS = 300_000`. `snapshot` filters expired sessions without pruning; `pruneExpired` removes them explicitly. Wired into `UdpForwarder`: one-way and last-client tracked via inbound packet handler (last-client change detects new client, closes old session, opens new); multi-client tracked per client endpoint in `handleMultiClientMessage`; `recordOutbound` called on target responses in last-client and multi-client; `closeSession` called on multi-client timeout; `closeSessionsForRule` called in `stop()`. `ForwardManager` owns shared `UdpSessionRegistry`, injects via 4th constructor parameter, exposes `getLiveUdpSessions()` for internal use. No public `GET /api/connections` endpoint added yet. Registry: 100% stmts/branch/funcs; 49 unit tests. `udp-forwarder.ts`: 86.3% stmts (up from 84.3%), 84% branch, 100% funcs; 9 new integration tests. Server overall: 80.55% → 82.21% stmts.
- [x] **Go service TCP live tracking (v1.4 Slice 5)** — added `TcpConnectionRegistry` (`service/sources/connections/tcp_connection_registry.go`): runtime-local UUID IDs, `OpenConnection`/`AddBytesIn`/`AddBytesOut`/`CloseConnection`/`CloseConnectionsForRule`/`Snapshot`/`SnapshotForRule` API, concurrency-safe (mutex for map ops, atomic ops for byte counters), serializable `TcpConnectionInfo` snapshots with `durationMs` at snapshot time. `NewTCPForwarderWithRegistry` constructor added; `countingWriter` extended with `onBytes` callback; `CloseConnectionsForRule` called in `Stop()` after `wg.Wait()` as belt-and-suspenders cleanup. `Manager` owns shared `TcpConnectionRegistry`, passes it to each `TCPForwarder` via `NewTCPForwarderWithRegistry`, exposes `GetLiveTCPConnections()` for internal use. No public `GET /api/connections` endpoint added yet. Registry: 98.1% stmts (100% all public methods; 1 untestable `rand.Read` error path in private `generateConnectionID`); 26 unit tests. Forwarder: 8 new integration tests. Service overall: 79.7% → 80.6% stmts.

Playwright E2E coverage (32 tests across 9 spec files — see `docs/e2e-coverage.md` for full matrix):

- [x] App load, sidebar navigation, mobile hamburger/sidebar, Dashboard stat cards
- [x] Add/edit/delete rule flows
- [x] Start/stop rule flow
- [x] Rule form validation — name required error
- [x] Diagnose rule — panel opens, results shown, close button
- [x] Settings config import (merge — inline 1 TCP rule)
- [x] Settings config import (replace — v1-mixed fixture, 4 rules; verify Forward Rules view)
- [x] Settings config import (invalid JSON — parse error, state preserved)
- [x] Settings config export (download shape: version, exportedAt, rules array, rule present)
- [x] Settings Runtime/Environment section shows Node server runtime info
- [x] API Docs view — endpoint list, GET /api/connections listed
- [x] Live Connections view — title/tabs/empty states, tab switching, summary stats bar
- [x] Live Connections — protocol filter set/clear, auto-refresh toggle, footer counts
- [x] Live Connections — rule filter dropdown populated when rule is running
- [x] TCP real forwarding
- [x] UDP one-way, bidirectional-last-client, bidirectional-multi-client real forwarding
- [x] TCP and UDP activity event assertions

Config compatibility coverage (`validate:config` — not manual QA):

- [ ] Valid fixture config load (raw array — all protocols and UDP modes)
- [ ] Valid fixture config load (Go wrapper shape)
- [ ] Valid fixture import via HTTP API (all 8 valid fixtures)
- [ ] UDP default mode normalization (no udpMode → one-way)
- [ ] Export shape stability (version, exportedAt, rules[])
- [ ] Duplicate binding rejection (409)
- [ ] Invalid field rejection — port out of range, missing name, empty host, bad protocol, bad udpMode (400)
- [ ] Malformed JSON rejection (server exit)

Go service coverage:

- [ ] Config load/save and import/export
- [ ] Manager lifecycle, duplicate binding rejection, update/restart behavior, and reorder
- [ ] API routes and error shapes
- [ ] TCP real forwarding and activity
- [ ] UDP one-way, bidirectional-last-client, bidirectional-multi-client, stats, sessions, stop, and activity
- [ ] Port advisories, validation, options, static serving, and health endpoint

## Manual Platform QA Required Before Distribution

Manual QA is now limited to firewall behavior and production install paths. Core TCP/UDP protocol correctness and OS service install/uninstall flows are automated.

### Package Build and Smoke Test (Automated)

- [ ] `npm run validate:runtime:smoke` passes — builds `build/portier/`, validates layout and content, runs smoke test.
  - Validates: `service`/`service.exe`, `server.js`, `web/index.html`, `web/assets/`, `readme.txt`.
  - Validates: `readme.txt` mentions management URL and config path.
  - Validates: `node_modules`, `rules.json`, `sources/`, `client/`, `server/` are absent from the package.
  - Smoke test: starts the packaged binary, polls `/api/health`, GETs `/`, verifies HTML is served, stops cleanly.

### OS Service Install Validation (Automated — Run Explicitly)

Service install, start, health check, stop, and uninstall are validated by explicit commands on each platform:

- [ ] `npm run validate:service:windows:user` — Windows scheduled task, no Administrator required.
- [ ] `npm run validate:service:windows:machine` — Windows Service, Administrator required.
- [ ] `npm run validate:service:macos` — macOS LaunchAgent, no sudo required.
- [ ] `npm run validate:service:linux` — Linux systemd unit, root/sudo required.

Each script uses test-specific names and temp directories. Production Portier installs are not touched.

Pass `--no-build` to reuse an existing `build/portier/` and skip the package build step.

### Windows Firewall (Manual — On Real Production Install)

- [ ] Windows Firewall prompts or required inbound rules documented and observed for LAN-visible forwarded ports.
- [ ] Production install to `%ProgramFiles%\Portier` (Machine) or `%LOCALAPPDATA%\Portier` (User) verified.
- [ ] Config preserved at `%ProgramData%\Portier\rules.json` (Machine) or `%APPDATA%\Portier\rules.json` (User) after uninstall.

### macOS Firewall (Manual — On Real Production Install)

- [ ] macOS Firewall prompts or required settings documented and observed for LAN-visible forwarded ports.
- [ ] Production install to `~/Applications/Portier` with config at `~/Library/Application Support/Portier/rules.json` verified.
- [ ] Config preserved after `scripts/macos/service/uninstall-launch-agent.sh`.

### Linux Firewall (Manual — On Real Production Install)

- [ ] Firewall rules for LAN-visible forwarded ports documented and observed.
- [ ] Production install to `/opt/portier` with config at `/etc/portier/rules.json` verified.
- [ ] Config preserved after `scripts/linux/service/uninstall-service.sh` without `--remove-config`.

## Post-v1.0 Follow-Ups

- Drag-and-drop rule reorder testing, if drag-and-drop UI replaces the current Move Up/Down controls.
- macOS `.app` bundle or Homebrew formula.
- Linux hardening beyond the example systemd unit.

## Coverage Baseline (v1.5.0)

Updated at v1.5.0 release (2026-06-09). See `docs/coverage-baseline.md` for full per-file breakdown and ratchet plan.

| Component  | Statements | Branch | Functions |      Gate | Run command                      |
| ---------- | ---------: | -----: | --------: | --------: | -------------------------------- |
| tools/cli  |      93.2% |      — |     98.6% |       93% | `npm run coverage:cli`           |
| client     |     95.1%  | 90.1%  |    79.9%  |  94/90/79 | `npm run coverage:client`        |
| service    |      85.8% |      — |     93.2% |       85% | `npm run coverage:service`       |
| shared     |     100.0% |  100%  |    100%   | 100/100/100 | `npm run coverage:shared`      |
| server     |     88.7%  | 91.0%  |    99.1%  |  88/90/99 | `npm run coverage:server`        |
| scripts    |        N/M |      — |         — |      none | not yet measured                 |

Gate format: `stmts/branch/funcs` thresholds. All gates enforced at v1.4.0.

Run all (reporting): `npm run coverage:baseline`  
Run all with gates: `npm run validate:coverage` (exits 1 on gate failure)  
Per-component gate: `npm run validate:coverage:<component>`

All v1.4 new/changed modules reached 100% meaningful coverage:
- `server/sources/connections/tcp-connection-registry.ts`: 100%
- `server/sources/connections/udp-session-registry.ts`: 100%
- `server/sources/forwarders/tcp-forwarder.ts`: 100% stmts (up from 68.5%)
- `service/sources/connections/tcp_connection_registry.go`: 98.1% stmts, 100% public methods
- `service/sources/connections/udp_session_registry.go`: 98.4% stmts, 100% public methods
- `client/sources/features/connections/LiveConnectionsView.tsx`: 100% stmts
- `client/sources/utils/format.ts`: 100%

Coverage policy for v1.4 and v1.5: 100% meaningful coverage for all newly added or materially changed files. Existing baselines ratcheted incrementally; do not block unrelated work on legacy gaps.

---

## v1.6-pre Coverage Ratchet & Quality Hardening

Pre-release hardening pass to raise the test safety net toward ~95% meaningful coverage before starting the v1.6 architecture and quality audit. No new product features.

- [ ] v1.6-pre roadmap added to `docs/roadmap.md` ✓
- [ ] v1.6-pre changelog entry added
- [ ] Coverage baseline re-measured and documented under v1.6-pre in `docs/coverage-baseline.md`
- [ ] Gap analysis performed; prioritized gap list produced
- **Service uplift** — meaningful Go tests for api, config, manager, validation, forwarder gaps → target ≥90%
- **Server uplift** — meaningful TS tests for api.ts, diagnose.ts, udp-forwarder.ts → target ≥95% stmts
- **CLI uplift** — meaningful CLI tests for remaining gaps → target ≥95%
- **Client branch/function uplift** — App.tsx, ActivityLogView, ForwardRuleList gaps → preserve ≥95% stmts
- [ ] Coverage gates ratcheted to stable achieved values; no gate lowered
- [ ] `validate:coverage` passes all gates at new values
- [ ] Full validation suite run: lint, typecheck, test, build, validate:contract, test:e2e, validate:cli, validate:runtime:smoke
- [ ] `docs/coverage-baseline.md` updated with before/after comparison and rationale

---

## v1.6 Planning

v1.6 is a dedicated audit and hardening release targeting a structured multi-angle inspection of the codebase after v1.4 and v1.5 have raised test coverage. See `docs/roadmap.md` for audit dimensions, slice plan, and non-goals.

Planning checklist:

- [x] v1.6 roadmap added to `docs/roadmap.md` as Architecture, Quality & Maintainability Audit
- [x] Audit dimensions recorded (architecture boundaries, runtime parity, forwarding correctness, API contract, CLI quality, client/UI quality, test quality, complexity/maintainability, security/safety, packaging, documentation consistency)
- [x] Coverage prerequisite explained: v1.4/v1.5 100% meaningful coverage target is the safety net required before the v1.6 audit and hardening work
- [x] Tentative audit slice plan recorded (12 slices)
- [x] Non-goals recorded (no new large features, no server/ removal, no major architecture rewrite without audit-backed plan)
- [x] v1.4/v1.5 coverage target linked to v1.6 audit safety net in `docs/roadmap.md`

---

## v1.5 Planning

v1.5 targets declarative config and drift control: plan/diff/apply workflows for comparing desired config files with the running configuration and applying changes safely. See `docs/roadmap.md` for goals, CLI commands, UI direction, slices, and non-goals.

Planning checklist:

- [x] v1.5 roadmap added to `docs/roadmap.md`
- [x] Diff/plan/apply scope recorded
- [x] CLI commands recorded (`config diff`, `config plan`, `config apply --yes/--dry-run/--backup-out`, `--fail-on-drift`, `--json`)
- [x] Proposed API direction recorded (`POST /api/config/plan`)
- [x] UI import preview direction recorded (Settings / Config Import Preview: Add/Update/Remove/Unchanged counts before confirm)
- [x] Non-goals recorded (auth, remote management, cloud sync, team sharing, scheduled rules, firewall management, service install from CLI, full rollback/history, TUI, traffic graphs)
- [x] 100% meaningful coverage target recorded for v1.4 and v1.5

Checklist items to add per slice as work proceeds:

- [x] **Slice 1** — Config diff/plan strategy and contract: matching semantics (id-first, then identity key protocol+listenHost+listenPort), operation model (add/update/remove/unchanged, destructive flag, changes[]), plan summary (hasDrift, hasErrors, counts), plan/apply types added to `@portier/shared` (`shared/sources/plan.ts`: `ConfigPlanOperationType`, `ConfigPlanChange`, `ConfigPlanRuleSnapshot`, `ConfigPlanOperation`, `ConfigPlanSummary`, `ConfigPlanError`, `ConfigPlanWarning`, `ConfigPlanResponse`, `DesiredConfig`, `ConfigPlanRequest`, `ConfigApplyRequest`, `ConfigApplyResponse`). `plan.test.ts` added with 20+ shape tests covering all types. `POST /api/config/plan` and `POST /api/config/apply` added to `docs/api-contract.md` as Planned (v1.5). Client in-app API Docs updated with planned badges and response/request field docs. `ApiDocsView.test.tsx` updated with 6 new tests. `validate:contract` updated with skip notes for both endpoints. `docs/e2e-coverage.md` updated with planned v1.5 workflows. `tools/cli/readme.md` updated with planned v1.5 CLI commands. `docs/roadmap.md` v1.5 section expanded with matching semantics, operation model, and summary model.
- [x] **Slice 2** — Backend plan endpoint: `POST /api/config/plan` in TypeScript server; config comparison logic; 100% meaningful coverage. Pure plan engine (`server/sources/config-plan.ts`): `buildConfigPlan`, `extractRulesArray`, `validatedToSnapshot`, `ruleToSnapshot`, `diffMaterialFields`, `isDestructiveUpdate`, `detectDuplicateIds`, `detectDuplicateKeys`; id-first matching, identity key (protocol+listenHost+listenPort) fallback, ambiguous match error, `DUPLICATE_DESIRED_ID`/`DUPLICATE_DESIRED_IDENTITY_KEY` guards, 8 material field diff, destructive flag (remove always; update destructive when forwarding fields change), `REMOVE_EXISTING` / `LAN_EXPOSURE` warnings. `POST /api/config/plan` route added to `server/sources/api.ts`; 400 for missing desired, 200 with structured plan errors for invalid desired. 65 plan engine unit tests + 11 API integration tests. `validate:contract` TS skip removed (11 real assertions); Go skip kept with parity note. API Docs parity badge added (TypeScript server — Go service parity pending); `ApiDocsView.test.tsx` updated. `docs/api-contract.md` heading updated to Implemented (TypeScript server). `docs/changelog.md` updated.
- [x] **Slice 3** — Go service plan parity: `POST /api/config/plan` in Go service; parity with TypeScript behavior; `validate:contract` skip replaced with real checks. Pure plan engine added at `service/sources/configplan/plan.go` (`BuildConfigPlan`, `ExtractRulesRaw`): id-first matching, identity key fallback, 8 material field diff, destructive flag, `REMOVE_EXISTING`/`LAN_EXPOSURE` warnings, all error codes. Route and handler added to `service/sources/api/api.go`; 400 for missing `desired`, 200 with structured errors for invalid desired. 49 Go engine unit tests + 10 Go API integration tests. `validate:contract` Go skip removed; 11 real plan assertions now run against Go (138/2 passed/skipped); API Docs parity badge removed (both runtimes implement). Service coverage: 84.8% → 85.8% statements.
- [x] **Slice 4** — CLI `config plan` and `config diff` commands; `--json` output; `--fail-on-drift` with exit code 4; 100% meaningful coverage. `portier config plan <file>` and `portier config diff <file>` added. Both validate the file locally then call `POST /api/config/plan`. Plan: structured summary + per-operation listing with field-level changes and `[destructive]` flag. Diff: `+`/`~`/`-`/`=` prefixed visual output; `--show-unchanged` flag. `--fail-on-drift` exits 4 when drift and no plan errors; plan errors take priority (exit 1). `--json` prints raw `ConfigPlanResponse`. New client types: `ConfigPlanRuleSnapshot`, `ConfigPlanChange`, `ConfigPlanOperation`, `ConfigPlanSummary`, `ConfigPlanError`, `ConfigPlanWarning`, `ConfigPlanResponse`, `ConfigPlanDesired`, `ConfigPlanRequest`, `PlanConfig` method. `configcmd.go` extended with `RunConfigPlan`, `RunConfigDiff`, all formatting helpers. New test file `configplancmd_test.go` (35 tests) + 5 client tests. CLI coverage: 92.7% → 93.2% (gate 92%). All 5 coverage gates pass.
- [x] **Slice 5** — CLI `config apply` with `--yes`, `--dry-run`, `--backup-out`; `POST /api/config/apply` in TypeScript server and Go service. Handler logic: plan errors → 200 ok:false (no mutation); dryRun → 200 ok:true (no mutation, no yes required); destructive without yes → 400; drift → replace import with current ruleId injected for key-matched rules; no drift → 200 ok:true no import. Response: `ConfigApplyResponse { ok, dryRun, appliedAt, plan, applied }`. CLI: local file validation, backup export (skipped on dry-run), human output ("Nothing to apply" / "Dry run complete" / "Config applied"). 25 `configapplycmd_test.go` tests + 7 client tests + 11 Go API tests. `validate:contract` 156/156. API Docs updated, `docs/api-contract.md` updated. All 5 coverage gates pass.
- [x] **Slice 6** — Settings UI Plan & Apply: `planHelpers.ts` (4 helpers, 17 tests); `planConfig`/`applyConfig` API helpers with tests; Plan & Apply section in SettingsView with file picker, plan preview (summary counts, errors, warnings, operation list, destructive checkbox), apply (yes:true, ok:false handling, form clear on success); 23 new unit tests; E2E test in `settings.spec.ts`; E2E file label exact-match fix in portier.spec.ts/settings.spec.ts; client branch 90.1% (gate 90%); all 5 coverage gates pass; 32/32 E2E pass.
- [x] **Slice 7** — Contract/config validation and coverage gate hardening: `validate:contract` 156/156 assertions pass against both runtimes for all plan and apply scenarios. Coverage gates ratcheted to v1.5.0 values: cli ≥93%, server ≥88/90/99, client ≥94/90/79, service ≥85%, shared 100%. All new v1.5 code verified at 100% meaningful coverage.
- [x] **Slice 8** — v1.5 readiness audit, version bump to 1.5.0, changelog finalized. Full validation suite passed: lint, typecheck, 756 unit tests, all 5 coverage gates, 156/156 contract assertions, 32/32 E2E. `validate:runtime:smoke` passed. `build:release:current` produced release artifacts. Tag v1.5.0 ready.

---

## v1.4 Roadmap

v1.4 targets live connection and session visibility: a read-only Live Connection Inspector showing active TCP connections and UDP sessions with duration, bytes, and client address. See `docs/roadmap.md` for API direction, data model, UI direction, slices, and non-goals.

Quality target: all newly added or materially changed code in v1.4 should reach 100% meaningful test coverage, with explicit coverage gates where practical. This covers CLI additions, Go service changes, TypeScript server changes, shared types, contract validators, and client-side logic.

- [x] 100% meaningful coverage target/gate tracked for all new v1.4 Live Connection Inspector code (TCP tracking, UDP tracking, `GET /api/connections` handler, contract validator, UI helpers — both runtimes). CLI `connections` command and Slice 9 rule-row live summary deferred to v1.5.

Checklist items to add per slice as work proceeds:

- [x] **Slice 1** — Live Connection Inspector contract and coverage strategy: `GET /api/connections` response shape finalized (`tcpConnections`, `udpSessions`, `ruleSummaries`, `generatedAt`); coverage gate plan recorded; `docs/api-contract.md` draft updated; `docs/checklist.md` updated.
- [x] **Slice 2** — Shared types and API contract validation: `LiveConnectionsResponse`, `TcpConnectionInfo`, `UdpSessionInfo`, `RuleLiveSummary`, `LiveConnectionStatus`, `UdpSessionStatus` added to `@portier/shared` (`shared/sources/connections.ts`); `validate:contract` updated with skip note for planned endpoint; `docs/api-contract.md` finalized (field directions, `lastTrafficAt` null, Shared Shapes updated); client in-app API Docs view updated with planned badge; `ApiDocsView.test.tsx` updated with 5 new tests; `connections.test.ts` added with 14 shape tests. Implementation still pending (Slices 3–7).
- [x] **Slice 3** — TypeScript server TCP live tracking: `TcpConnectionRegistry` added (`server/sources/connections/tcp-connection-registry.ts`); `TcpForwarder` wired to open/track/close entries per connection; `ForwardManager` owns shared registry and exposes `getLiveTcpConnections()`; registry cleanup on error path (via `closeBoth`) and stop (via `closeConnectionsForRule`); no double-counting on error/close race; no payload capture. Registry: 100% stmts/branch/funcs. `tcp-forwarder.ts`: 100% stmts/funcs, 90% branch (uncovered branches = optional registry param when omitted). Server overall: 79.6% → 80.55% stmts. 28 registry tests + 7 forwarder integration tests added.
- [x] **Slice 4** — TypeScript server UDP session tracking: active session registry for all UDP modes; startedAt, lastSeenAt, packets/bytes; named idle/expiry constants (30s idle, 5min expire); 100% meaningful coverage including idle/expiry edge cases.
- [x] **Slice 5** — Go service TCP live tracking: `TcpConnectionRegistry` (`service/sources/connections/tcp_connection_registry.go`); `TCPForwarder` wired to open/close/track bytes; `Manager` owns shared registry and exposes `GetLiveTCPConnections()`; concurrency-safe (mutex for map ops, atomic byte counters); `CloseConnectionsForRule` in Stop as belt-and-suspenders. Registry: 98.1% stmts (100% public methods). 26 registry unit tests + 8 forwarder integration tests. Service overall: 79.7% → 80.6% stmts.
- [x] **Slice 6** — Go service UDP session tracking: `UdpSessionRegistry` (`service/sources/connections/udp_session_registry.go`); composite session keys (`ruleId:mode:clientAddress:clientPort`); `OpenOrTouchSession`/`RecordInbound`/`RecordOutbound`/`CloseSession`/`CloseSessionsForRule`/`PruneExpired`/`Snapshot`/`SnapshotForRule` API; `UDPSessionIdleDuration = 30s`, `UDPSessionExpireDuration = 5min`; status `active`/`idle` calculated at snapshot time; expired sessions hidden from snapshot without explicit prune; all three UDP modes wired in `UDPForwarder`; one-way/last-client modes call `OpenOrTouchSession` + `RecordInbound` per inbound packet; last-client closes previous session on client-endpoint change; multi-client stores `registryID` on session struct, calls `RecordOutbound` in `sessionReadLoop`, `CloseSession` in `expireSession`; `Manager` owns shared `UdpSessionRegistry`, injects it via `NewUDPForwarderWithRegistry`, exposes `GetLiveUDPSessions()`; `CloseSessionsForRule` in Stop as belt-and-suspenders. Registry: 98.4% stmts (100% public methods; 2 untestable defensive paths: stale-key path in `OpenOrTouchSession`, `rand.Read` fallback in `generateConnectionID`). 33 registry unit tests + 11 forwarder integration tests. Service overall: 80.6% → 82.1% stmts.
- [x] **Slice 7** — `GET /api/connections` parity: `GET /api/connections` implemented on TypeScript server (`server/sources/api.ts`) and Go service (`service/sources/api/api.go`); `RuleLiveSummary` + `LiveConnectionsResponse` types added to `service/sources/connections/live_connections.go`; both runtimes return identical JSON shape (`generatedAt`, `tcpConnections[]`, `udpSessions[]`, `ruleSummaries[]`); arrays always non-null; `lastTrafficAt` null for idle rules; `ruleSummaries` covers all configured rules; `fetchLiveConnections()` client API helper added; contract validator skip replaced with 8 real checks; API Docs view `Planned — v1.4` badge removed; 6 TypeScript + 6 Go tests.
- [x] **Slice 8** — Client API and Live Connections UI: `fetchLiveConnections()` API helper; Live Connections view (table with protocol, rule, client, target, duration/idle, bytes in/out, packets, status); rule/protocol/status filters; manual refresh and auto-refresh toggle; empty state; loading/error handling; 100% meaningful coverage of helpers and display logic.
- [ ] **Slice 9** — Rule row live activity summary: compact active connections/sessions count and last-traffic age per rule row, using `GET /api/connections` data; subtle display; tests added. *Deferred to v1.5.*
- [ ] **Slice 10** — CLI `portier connections`: calls `GET /api/connections`; human aligned table; `--rule`, `--protocol`, `--json` flags; safe rule resolver reused; 100% meaningful coverage; `validate:coverage` threshold maintained or raised. *Deferred to v1.5.*
- [ ] **Slice 11** — Diagnostics export integration: decide whether to include live session snapshot in CLI and UI support bundle; implement if promoted; update tests and contract if changed. *Deferred to v1.5.*
- [x] **Slice 12** — Coverage gates and readiness audit: all v1.4 coverage targets verified; all new/changed modules at 100% meaningful coverage; coverage gates added for all 5 components (cli ≥92%, client ≥90/89/76, server ≥82/86/97, service ≥82%, shared ≥82/54/90); vitest config corrected for Windows path-case issue; `validate:coverage` passes all gates; per-component `validate:coverage:*` scripts added.
- [x] **Slice 13** — v1.4 version bump, changelog finalized, tag created, full validation suite passed (`lint`, `typecheck`, `test`, `build`, `validate:cli`, `validate:contract`, `validate:runtime:smoke`, `validate:release:current`).
- [x] **Slice 14** — Docs update: `docs/roadmap.md`, `docs/api-contract.md`, `docs/changelog.md`, `README.md`, `AGENTS.md`, `CLAUDE.md` all reflect delivered v1.4 behavior.

---

## v1.3 Roadmap

v1.3 targets native CLI and automation: a Go-based `portier` CLI that talks to the existing management API. See `docs/roadmap.md` for principles, command set, implementation structure, packaging direction, slices, and non-goals.

Checklist items to add per slice as work proceeds:

- [x] **Slice 1** — CLI strategy and command design: command set, rule lookup behavior, output modes, exit code contract, and module layout confirmed. Documented in `docs/roadmap.md`.
- [x] **Slice 2** — Go CLI skeleton and API client: `tools/cli/` module scaffolded, HTTP client (`ConnectionError`/`APIError` types), `--url`/`--host`/`--port`/`PORTIER_URL` connection options, `--json` flag, `runtime` command (human + JSON output), structured error output, 22+ tests using `httptest`, `build:cli`/`test:cli`/`validate:cli` npm scripts.
- [x] **Slice 3** — Read-only commands: `portier list`, `portier status`, `portier activity`; human table and `--json` output; `--limit`/`--rule`/`--type`/`--severity` filters for activity; output helpers (`FormatBool`, `FormatBytes`, `FormatTimestamp`, `PrintTable`); 59 CLI tests total.
- [x] **Slice 4** — Lifecycle commands: `portier start <id|name>`, `portier stop <id|name>`, `portier diagnose <id|name>`; safe rule resolver (exact ID wins, unique name match, ambiguous-name exit 2, not-found exit 1); `DiagnosticCheck`/`DiagnosticSummary`/`RuleDiagnosticsResult` types; `StartForward`/`StopForward`/`DiagnoseForward` client methods; stable JSON objects for start/stop; raw `RuleDiagnosticsResult` for diagnose; 89 CLI tests total.
- [x] **Slice 5** — Config commands: `portier config validate <file>` (local-only, all three shapes, field/binding validation); `portier config export --out <file>` (calls `GET /api/config/export`, stdout JSON mode, no partial writes); `portier config import --mode merge|replace [--yes] <file>` (local validate before API, replace requires `--yes`); `ConfigRule`/`ConfigExportResponse`/`ConfigImportRequest`/`ImportResult`/`ConfigImportResponse` types; `doWithBody` helper; 132 CLI tests total.
- [x] **Slice 6** — Diagnostics export: `portier diagnostics export --out <file>` (bundle schema, partial failure, `--run-diagnostics`, `--activity-limit`, 153 tests total).
- [x] **Slice 7** — CLI packaging: `portier`/`portier.exe` built into `build/portier/` by all platform build scripts; runtime and release validation require CLI binary; Windows installer includes `portier.exe`; `readme.txt` documents CLI usage; no PATH integration in v1.3.
- [x] **Slice 8** — v1.3 readiness audit, version bump, changelog finalized, tag created. Coverage gate added (`validate:cli:coverage`, 88% threshold, 90.1% actual); version bumped to 1.3.0; all validation suites passed.
- [x] **Post-v1.3 coverage ratchet** — CLI coverage gate raised to 92% (actual: 92.7%); 10 targeted tests added; first ratchet step toward v1.4/v1.5 100% meaningful coverage target.

---

## v1.2 Roadmap

v1.2 focuses on diagnostics and operational polish. See `docs/roadmap.md` for goals, slices, and non-goals.

Checklist items to add per slice as work proceeds:

- [x] **Slice 1** — Runtime info endpoint and UI display: `GET /api/runtime` in both runtimes, Settings Runtime/Environment section, shared `RuntimeInfo` type, contract validator updated.
- [x] **Slice 2** — Rule diagnostics API: `POST /api/forwards/:id/diagnose` in both runtimes; `docs/api-contract.md` updated; client in-app API Docs updated; API Docs tests updated.
- [x] **Slice 3** — Rule diagnostics UI: Diagnose button per rule row, inline diagnostics panel, all check states, clear on close/delete; tests added; API Docs updated.
- [x] **Slice 4** — Activity Log polish: View Activity button per rule row, filter banner, type filter, clear filters, `DELETE /api/activity` on both runtimes, Export JSON, Clear Log, packet throttle note; `docs/api-contract.md` updated; client in-app API Docs updated; contract validator updated; tests added.
- [x] **Slice 5** — Settings / runtime / config polish: copy buttons (config path, static dir, management URL), datetime export filename, export note excludes Activity Log, export success/error feedback, import mode with descriptions above file picker, replace confirm backup export button, `PORTIER_APP_VERSION` constant, version in sidebar footer; 24 new tests.
- [x] **Slice 6** — Safer networking UX: listen host presets (Local only / LAN exposed), inline LAN warning in form, platform-aware firewall note, friendly conflict error copy, improved LAN_EXPOSURE advisory message; 17 new/updated ForwardRuleForm tests.
- [x] **Slice 7** — Diagnostics export: Download Diagnostics JSON button in Settings; client-side bundle (runtime, rules, statuses, activity, UI-session diagnostics); partial-failure errors array; no backend endpoint; 216 client tests passing.
- [x] **Slice 8** — v1.2 readiness audit, version bump, changelog, tag: all validation passed; version bumped to 1.2.0; changelog finalized; tag ready.

---

## v1.1 Installer Readiness

v1.1 focuses on distribution and native OS service installers. See `docs/installer-strategy.md` for scope and implementation slices.

Checklist items to add per slice as work proceeds:

- [x] **Slice 2** — Windows Inno Setup installer: machine-wide install (`%ProgramFiles%\Portier`), optional Windows Service task, uninstall preserves `rules.json`. Scripts in `scripts/windows/release/`. Build via `npm run release:current`. Requires Inno Setup 6.
- [x] **Slice 3** — macOS LaunchAgent polish: auto-copy from `build/portier/`, `--source-dir`/`--no-start`/`--runtime` options, label bug fixed, `--purge` on uninstall, `scripts/macos/release/build-release.sh` for portable tar.gz, signing/notarization docs. Service lifecycle scripts in `scripts/macos/service/`. Build via `npm run release:current` (on macOS). Validate via `npm run validate:service:macos`.
- [x] **Slice 4** — Linux install/uninstall/start/stop/status scripts complete; `--source-dir`/`--no-enable` added to `install-service.sh`; `scripts/linux/release/build-release.sh` for portable tar.gz; service lifecycle scripts in `scripts/linux/service/`; systemd unit examples and docs updated.
- [x] **Slice 5** — `validate:service:*` scripts unified: all support `--no-build`, `--keep-files`, `--port`; test-specific names/paths/ports on all platforms; `validate:service:current` dispatches by OS with unsupported-platform error. Windows user-scope validated on Windows host. macOS/Linux validation requires the respective OS.
- [x] **Slice 6** — `release:current` and `release:portable` produce `build/releases/<platform>/` with versioned portable archives. `validate:release:portable` validates layout, required files, forbidden files, and readme.txt content. Windows zip via `Compress-Archive`; macOS/Linux tar.gz via `scripts/macos/release/build-release.sh` and `scripts/linux/release/build-release.sh`. Windows installer non-fatal if Inno Setup absent. macOS .pkg and Linux .deb/.rpm deferred.
- [x] **Slice 7** — v1.1 readiness audit passed; version bumped to `1.1.0`; changelog entry finalized; tag created.
