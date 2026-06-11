# Portier Agent Guide

## Project

- App name: Portier
- Repository name: portier-port-forwarding
- Purpose: local TCP/UDP port forwarding manager for development and LAN testing

## Structure

- `server/sources` = Node.js TypeScript service, REST API, TCP/UDP forwarding engine
- `service/sources` = native Go service implementation for smaller binaries and service deployment
- `client/sources` = React TypeScript web UI
- `shared/sources` = shared types, validation, constants, port advisory utilities
- `scripts` = executable automation scripts; platform subdirs contain `release/` (artifact builders), `service/` (OS service lifecycle), and platform-specific docs/templates
- `tools` = user-facing and developer-facing project tools (v1.3: `tools/cli/` — the portier CLI)
- `build` = generated build output

## Server Runtimes

- `service/` is the native Go service and the preferred production runtime. Default static dir: `web`.
- `server/` is the TypeScript server and remains supported as reference/fallback. Default static dir: `client/build`.
- Both implement the same REST API contract.

## Terminology

- Canonical Portier terms live in `docs/glossary.md` (forward rule, config plan/apply/import, runtime/server/service, advisory vs warning, diagnose vs diagnostics export, live connection vs UDP session). New docs/API/CLI/UI wording should use glossary terms, or update the glossary for a genuinely new concept.
- Do not rename public API fields or REST paths (`/api/forwards`, `listenHost`/`targetHost`, `clientAddress`/`targetAddress`) for cosmetic consistency; document any exception in the glossary.

## Packaged Runtime Layout

```text
<install-dir>/
  portier          (or portier.exe on Windows)   — CLI
  service          (or service.exe on Windows)   — background service
  server.js        (Node fallback — requires Node.js)
  web/
    index.html
    assets/
  readme.txt
```

Development build output (not distributed):

```text
service/build/portier-service
server/build/
client/build/
tools/cli/build/portier-cli
```

## Setup Commands

```powershell
npm install
```

## Development Commands

```powershell
npm run start:dev
npm run dev -w server
npm run dev -w client
```

## Validation Commands

```powershell
npm run lint
npm run typecheck
npm run test
npm run build
npm run check
```

## E2E Tests

Playwright E2E tests live in `tests/e2e/`. They run against the TypeScript server serving the built React client.

**Prerequisites (one-time):**
```powershell
npm run test:e2e:install   # installs Playwright Chromium browser
npm run build:client       # must be built before running E2E
```

**Run:**
```powershell
npm run test:e2e           # headless
npm run test:e2e:headed    # visible browser
npm run test:e2e:fresh     # build:client then run
```

E2E server binds to `127.0.0.1:47890`. Do not include `test:e2e` in `npm run test` or `npm run check` — E2E is a separate step.

Files:
- `playwright.config.ts` — webServer, browser, reporter config
- `tests/e2e/portier.spec.ts` — App shell, CRUD, start/stop, merge import, form validation, diagnose
- `tests/e2e/settings.spec.ts` — Replace import, invalid JSON, export shape, runtime info, plan preview, plan apply
- `tests/e2e/connections.spec.ts` — Live Connections view: title/tabs, empty states, switching, stats bar, filters, auto-refresh, footer counts, rule filter
- `tests/e2e/activity.spec.ts` — Activity log view opens and shows events
- `tests/e2e/apidocs.spec.ts` — Endpoint list renders, GET /api/connections listed
- `tests/e2e/dashboard.spec.ts` — Dashboard stat cards render
- `tests/e2e/mobile.spec.ts` — Mobile hamburger / sidebar toggle
- `tests/e2e/tcp.spec.ts` — Real TCP forwarding E2E
- `tests/e2e/udp.spec.ts` — UDP one-way, last-client, multi-client E2E; UDP and TCP activity assertions
- `tests/e2e/helpers/port.ts` — `getFreePort`, `getFreeTcpPort`, `getFreeUdpPort`
- `tests/e2e/helpers/network.ts` — TCP/UDP echo servers, receivers, clients
- `tests/e2e/helpers/ui.ts` — `addRuleViaUI`, `startRuleViaUI`, `stopRuleViaUI`
- `tests/e2e/helpers/api.ts` — `clearAllRules`, `createRule`, `startRule`, `stopRule`
- `tests/e2e/helpers/setup.ts` — creates temp config before server starts
- `tests/tsconfig.json` — TypeScript config for E2E files
- `tests/fixtures/config/` — config compatibility fixtures (valid and invalid `rules.json` samples)

**Protocol automation coverage:**
- TCP forwarding: automated E2E via `tcp.spec.ts`
- UDP one-way: automated E2E via `udp.spec.ts`
- UDP bidirectional-last-client: automated E2E via `udp.spec.ts`
- UDP bidirectional-multi-client: automated E2E via `udp.spec.ts`

**Settings import/export E2E coverage (`settings.spec.ts`):**
- Replace-mode import using `v1-mixed.json` fixture: preview counts, confirm dialog, success message, all 4 rules visible in Forward Rules, pre-existing rule gone
- Invalid JSON import: parse error alert, no preview/import button, existing rule preserved
- Export download: file downloaded with correct filename pattern, `ExportedConfig` shape (`version`, `exportedAt`, `rules`), created rule present

**Config compatibility coverage (`validate:config` — not E2E, not manual):**
- Exhaustive fixture-based validation of all 16 fixtures: valid config load, import/export via API, UDP mode defaults, duplicate binding rejection, invalid field rejection, malformed JSON rejection, Go/TS parity
- E2E intentionally does not run the full fixture matrix — `validate:config` owns that

**OS service install validation (run explicitly on the target platform, not part of `npm run check`):**
- Windows scheduled task (no admin): `npm run validate:service:windows:user`
- Windows Service (admin): `npm run validate:service:windows:machine`
- macOS LaunchAgent (no sudo): `npm run validate:service:macos`
- Linux systemd (root/sudo): `npm run validate:service:linux`
- Current platform default: `npm run validate:service:current`

Each script uses test-specific names, ports, and temp paths. Never touches production installs.

**Remaining manual QA (cannot be automated):**
- Firewall and OS permission behavior on each platform

**Automated (not manual):**
- Package build correctness: `npm run validate:runtime:smoke`
- TCP/UDP protocol forwarding: `npm run test:e2e`
- OS service install/start/stop/uninstall: `npm run validate:service:*`

## Coverage Commands

Run coverage for individual components, all at once, or validate all gates:

```powershell
npm run coverage:shared     # shared TypeScript (vitest + v8)
npm run coverage:server     # server TypeScript (vitest + v8)
npm run coverage:client     # client TypeScript/React (vitest + v8)
npm run coverage:service    # Go service (go test -p 1 -coverpkg) — takes ~30s
npm run coverage:cli        # Go CLI reporting only (scripts/coverage-tools-cli.js)
npm run coverage:baseline   # all five in sequence (reporting only)
npm run validate:coverage   # runs all + enforces all gates; exits 1 if any gate fails
npm run validate:coverage:shared   # shared only
npm run validate:coverage:server   # server only
npm run validate:coverage:client   # client only
npm run validate:coverage:service  # service only
npm run validate:coverage:cli      # cli only
```

Coverage outputs (gitignored):
- `coverage/shared/`, `coverage/server/`, `coverage/client/` — vitest json-summary + text
- `coverage/` — Go .out profiles (written and removed per run)

Baseline (v1.6-pre, recalibrated at Slice A, cli at Coverage Slice C, service at Coverage Slice D, server at Coverage Slice E): cli 97.7%, client ~95%/~89-90%/~78-80%, service 90.3%, shared 100%, server 98.9%/93.6%/100%.
Client numbers vary ±1% per run (Windows vitest ghost-entry deduplication — both runs pass gates). See `docs/coverage-baseline.md` for full breakdown and methodology.

v1.6 Architecture Slice Arch-A (contract drift guard) did not change any coverage number or gate: it aligned the Go `LAN_EXPOSURE` advisory message to the canonical `@portier/shared` wording and strengthened `validate:contract` to compare advisory/plan content (not just codes/shape). `@portier/shared` is canonical for advisory wording; the Go service must match it. Source: `audits/v1.6-architecture-audit-1.md`.

v1.6 Architecture Slice Arch-B (apply orchestration extraction) moved config apply transformation out of the HTTP handlers into tested plan-engine helpers (`buildApplyImportFromPlan` in `server/sources/config-plan.ts`, `BuildApplyImportFromPlan` in `service/sources/configplan/plan.go`). Behavior preserved exactly; `validate:contract` 166/166; server coverage 95.2/91.6 → 95.3/92.0, service unchanged; no gate change.

**Durable architecture rule (Arch-B):** Config apply orchestration must live beside the plan engine, not inside HTTP handlers. Handlers own request/response, confirmation (`yes`), dry-run, and status codes; plan/apply transformation (desired replace list, id injection/preservation, applied counts) stays in the tested config-plan/configplan helpers. Keep the TypeScript and Go helpers semantically mirrored; add unit tests in both runtimes plus `validate:contract` parity assertions for any new externally observable apply behavior. `validate:contract` is the parity guard. Source: `audits/v1.6-architecture-audit-1.md`.

v1.6 Architecture Slice Arch-C1 (Go ID dedupe + dead code) replaced the duplicate UUID generators `manager.randomID` and `api.newApplyRuleID` with one shared `domain.NewRuleID()` (`service/sources/domain/id.go`) and removed a no-op `name` assignment in `api.go`. Behavior preserved; `validate:contract` 166/166; coverage neutral; no gate change.

**Durable architecture rule (Arch-C1):** Go service rule ID generation must have one shared implementation (`domain.NewRuleID`). Manager-created and config-apply-created rule IDs must use the same generator so the two paths cannot silently drift in format. Do not reintroduce per-package UUID helpers. Source: `audits/v1.6-architecture-audit-1.md`.

v1.6 Architecture Slice Arch-C2 (manager activity-emission dedupe) collapsed the 10 repeated rule-scoped emission blocks in `manager.go` into one `emitRuleEvent(eventType, severity, rule, message)` helper; config-level events keep their own `emitActivity` calls (they carry `details`, no rule fields). Payloads preserved byte-for-byte; full-payload regression tests added. `validate:contract` 166/166; service 88.6% → 88.5% (no gate change). Arch-C complete.

**Durable architecture rule (Arch-C2):** Rule-scoped Go manager activity events must be emitted via `emitRuleEvent` so ruleId/ruleName/protocol cannot drift between call sites; non-rule-scoped config events stay on `emitActivity` with their own details. Activity payloads are user-visible diagnostics — changes must be covered by tests asserting type, severity, ruleId, ruleName, protocol, message, and details. Source: `audits/v1.6-architecture-audit-1.md`.

v1.7 Slice 3 (Go Forwarder interface + StartRule dedupe): the Go manager starts/stops/inspects rules through a small `forwarders.Forwarder` interface (`Start() error`, `Stop()`, `Status() domain.ForwardStatus`) built by the `forwarders.NewForwarder(rule, log, onEvent, tcpReg, udpReg)` factory (`service/sources/forwarders/forwarder.go`). `*TCPForwarder`/`*UDPForwarder` already satisfy it (interface = shape, not behavior — `tcp.go`/`udp.go` untouched). `manager.runtimeState` holds one `forwarder forwarders.Forwarder`; `StartRule`/`stopRuntime`/`statusForRule` running branch are single-path; the protocol switch lives only in `NewForwarder` (unknown → nil → manager no-op). `statusForRule`'s not-running synthetic status keeps per-protocol shape (API-response concern). Behavior/contract unchanged; `validate:contract` 185/185; `NewForwarder` 100% covered; service 90.3→90.1 (gate 90 PASS). 3 socket-free factory tests added. Source: `docs/roadmap.md` v1.7.

**Durable architecture rule (v1.7 Slice 3):** Go rule lifecycle in the manager must go through the `forwarders.Forwarder` interface + `NewForwarder` factory — no reacquiring per-protocol forwarder pointers and no duplicated per-protocol start/stop construction in the manager. Protocol dispatch belongs in `NewForwarder` (one switch); a new forwarding protocol adds a case there and a `Forwarder`-satisfying type, not another branch in `StartRule`/`stopRuntime`. Keep the interface minimal (start/stop/status) — do not grow it into a framework or push API-response-shape concerns into it. The TS server need not mirror this internal structure, but new observable forwarding behavior still needs `validate:contract` parity.

v1.7 Slice 4 (TS UDP emit facade): the TS UDP forwarder builds all 5 activity-event kinds through a small module-private `UdpEventEmitter` (`server/sources/forwarders/udp-forwarder.ts`), built once from `rule` + `onEvent`. `packetError(message)` (no details) / `packetForwarded`/`packetReturned`/`sessionOpened`/`sessionClosed`(message, details) → one private `emit(type, severity, message, details?)` stamping `ruleId`/`ruleName`/`protocol:"udp"` + dispatch via optional `onEvent`. Throttle timing, `status.lastError`, and session/registry bookkeeping stay in the forwarder. Payloads byte-identical; facade not exported (UDP-specific); tested via the public forwarder. 2 full-envelope regression tests; `validate:contract` 185/185; server 98.9/94.0/100 → 98.9/94.1/100 (gates unchanged).

**Durable architecture rule (v1.7 Slice 4):** Repetitive per-protocol activity-event construction must go through a small co-located emit facade so the `{type, severity, ruleId, ruleName, protocol, message, details?}` envelope cannot drift between call sites — do not re-spell the full event literal at each emission. Keep the facade module-private and close to its forwarder unless genuinely reused (TCP/UDP have distinct event sets — no forced shared emitter, no event bus/observer framework). The facade owns ONLY construction + dispatch; throttle timing, `status`/`lastError` mutation, and session/registry bookkeeping stay in the forwarder. Payloads must stay byte-identical (fields, order, `details` presence); value changes follow the Fix Slice 5 activity-taxonomy rule. Prove output with full-envelope tests through the public forwarder, not by importing the private facade. TS-internal choice — no Go parity obligation, but observable event changes still need `validate:contract`.

v1.6 Architecture Slice Arch-D (CLI DTO live-runtime parity guard) added live-fixture capture to `validate:contract` plus `TestCLIDTOContractParity` (`tools/cli/sources/client/contract_decode_test.go`), which strictly decodes real runtime responses into the CLI DTOs (env-gated by `PORTIER_CLI_CONTRACT_FIXTURES`). `validate:contract` 167 passed; no coverage/gate change; CLI stays a pure API client. Completes the audit's contract-parity remediation.

**Durable architecture rule (Arch-D):** The CLI DTOs are a third copy of the REST contract and must be guarded against live runtime responses, not only httptest mocks. The CLI must remain a pure API client (no server/service imports, not a second runtime). New API response families must update both the `validate:contract` live capture and the CLI decode guard (`contract_decode_test.go`). `/api/connections` is out of CLI scope. Source: `audits/v1.6-architecture-audit-1.md`.

v1.6 Testing Slice Test-A (port-allocation stabilization) removed the EADDRINUSE/TOCTOU flake in socket-binding tests. Test-only fix (Pattern C: bounded retry around the bind step only, only on EADDRINUSE). Helpers: `isAddrInUse` + `startTCPForwarderOnFreePort`/`startUDPForwarderOnFreePort` (`service/sources/forwarders/portretry_test.go`), `startRuleStable` (`service/sources/manager/portretry_test.go`), `startForwarderOnFreePort` (`server/sources/test-helpers.ts`); `scripts/validate-contract.js` retries the child bind on a fresh port. Migrated Go `tcp_test.go`/`udp_test.go`/`manager_test.go` and TS `tcp-forwarder.test.ts`. Residual follow-ups: TS `udp-forwarder.test.ts`, api HTTP `/start` sites, E2E `port.ts` (E2E serial → no intra-suite race). No coverage/gate change. Source: `audits/v1.6-testing-audit-1.md`.

**Durable testing rule (Test-A):** Tests must not rely on allocate-close-rebind port helpers when a listener handoff or bind-retry can be used. Socket readiness/ordering must use deadline polling, not fixed sleeps. EADDRINUSE flakes must be fixed at the helper/bind-operation level (bounded retry on a fresh port, only on EADDRINUSE) — never hidden by retrying whole tests/suites and never by arbitrary sleeps. Build+start new socket-binding tests through the `startForwarderOnFreePort`/`startTCPForwarderOnFreePort`/`startUDPForwarderOnFreePort`/`startRuleStable` helpers (or equivalent bounded bind-retry); keep intentional bind-failure tests (which reserve a port to force EADDRINUSE) outside those helpers. Source: `audits/v1.6-testing-audit-1.md`.

v1.6 Fix Slice 3 (finish Test-A residual migration): consumed the bind-retry/live-handoff helpers in the two named TS residual suites. New `server/sources/test-helpers.ts` helpers: `startRuleStable` (mirrors the Go helper — retries `manager.startRule` on EADDRINUSE by rebinding `listenPort` via `updateRule`), `startTcpServerOnFreePort`/`bindUdpSocketOnFreePort` (bind port 0, return the LIVE listener/socket + actual port). Migrated `udp-forwarder.test.ts` (listen binds → `startForwarderOnFreePort`; UDP targets → `bindUdpSocketOnFreePort`; some fixed-sleep waits → `waitUntil`) and `forward-manager.test.ts` (manager `startRule` incl. Test-D rollback tests → `startRuleStable`; TCP targets → `startTcpServerOnFreePort`; removed local `startTcpTarget`). Intentional-conflict tests kept outside the helpers. Tests-only; 58 tests pass ×3; server 354 tests, 98.9/93.8/100 unchanged; gates/contract/config unchanged. Remaining accepted residuals (documented, bounded): HTTP `/start` sites in `api.test.ts`/`api_test.go` (bind behind handler → needs an HTTP-level recreate-on-EADDRINUSE helper, larger follow-up) and E2E `tests/e2e/helpers/port.ts` (E2E serial — `workers:1` — no intra-process race). Revisit trigger: an actually-observed HTTP `/start` or E2E EADDRINUSE flake. Source: `audits/v1.6-audit-synthesis-and-fix-plan.md`.

v1.6 Fix Slice 4 (validate-contract.js scenario registry): split the ~1090-line `runScenarios` into 12 named `run<Area>Scenarios(ctx)` group functions driven by an ordered `scenarioGroups` registry; `runScenarios` is now a slim wrapper building `ctx = { api, runtime, fixtureDir, state }` and iterating the registry, then the Arch-D CLI fixture capture last. Cross-group rule ids flow via `ctx.state`; `exportedConfig` stays group-local. Added `expectedRuntimeApiValue(label)` (Naming-D). Script-only; assertions verbatim; `validate:contract` 171/171 unchanged; parity comparison + CLI DTO guard + child cleanup untouched. The file uses intentional embedded `\x00` sort-key separators in `normalize*` — preserve them on edits. Source: complexity/SOLID/pattern/duplication/readability audits.

**Durable rule (Fix Slice 4):** Contract scenarios live in named `run<Area>Scenarios(ctx)` groups registered in the ordered `scenarioGroups` array, not a monolithic body. New endpoint families add/extend a focused group + registry entry; cross-group state via `ctx.state`. `validate:contract` count changes must be intentional and documented (171 at Slice 4; 183 at Slice 5). `scripts/validate-contract.js` has intentional embedded `\x00` bytes (sort-key separators) — edit with tools that preserve them.

v1.6 Fix Slice 5 (activity value parity guard): `validate:contract` guards the activity taxonomy at the value level. Canonical `EXPECTED_ACTIVITY_TYPES` (17) / `EXPECTED_ACTIVITY_SEVERITIES` (4) in `scripts/validate-contract.js`; `runActivityValueSetParityChecks()` reads the TS union + Go consts from source (binary-independent) and asserts both equal the canonical set; `runActivityScenarios` asserts emitted-value membership + representative `rule.*` pairs; the import group asserts `config.imported`/`config.import.failed` emission. Count 171 → 183. No product/API-shape/gate change.

**Durable rule (Fix Slice 5):** Activity event-type/severity values are contract values, not cosmetic labels (17 types + 4 severities `info`/`success`/`warning`/`error`; no `config.reordered`). New types/severities must be added consistently across `shared/sources/activity.ts`, `service/sources/activity/activity.go`, `docs/api-contract.md`, both managers' emit sites, and the `validate:contract` `EXPECTED_ACTIVITY_*` sets — together. `validate:contract` keeps asserting value membership AND cross-runtime source-set parity, not only shape. CLI stays a pure API client.

v1.6 Fix Slice 6 (TS emitRuleEvent helper): TS `ForwardManager` emits the 6 rule-scoped events through a single private `emitRuleEvent(type, severity, rule, message)` helper, mirroring Go `manager.emitRuleEvent` (Arch-C2) — payloads byte-identical, `activity?.add` optional-chaining preserved, ActivityStore owns id/timestamp. The 5 config-level events stay on direct `activity?.add` (carry `details`). 3 full-payload tests added. No API/type/severity/message/order/count change; server branch 93.8→94.0, gates unchanged; `validate:contract` 183/183.

**Durable rule (Fix Slice 6):** Rule-scoped activity events must go through the `emitRuleEvent` helper in BOTH runtimes so the `{type, severity, ruleId, ruleName, protocol, message}` payload cannot drift between call sites. Config-level events carrying `details` (or not rule-scoped) stay on direct `activity?.add`/`emitActivity` unless a dedicated config-event helper is added in both runtimes. Any rule-scoped emission change must keep full-payload tests green in both runtimes; value changes follow the Fix Slice 5 rule.

v1.6 Fix Slice 7 (config apply importConfig result.errors invariant): both apply handlers now inspect the `ImportResult` (previously discarded). After import, non-empty `result.errors` → `200 ok:false` with errors via existing `plan.errors` (`IMPORT_ERROR`) + `hasErrors:true` + zero counts (no new field). No `result.errors` path is currently reachable from apply (duplicate bindings pre-blocked by `detectDuplicateKeys`, invalid rules by plan validation, persist throws→500); defensive guard. Go construction in unit-tested `applyImportErrorResponse` (handler guard unreachable without deferred manager-interface seam). `validate:contract` 183→185. CLI unaffected. Gates unchanged.

**Durable rule (Fix Slice 7):** Config apply must never report `ok:true` when the underlying import reports errors. Plan-level errors (incl. duplicate desired listen bindings) stop apply BEFORE import and stay contract-covered; import-level errors after plan validation are reported as `ok:false` via the existing `plan.errors`/`summary.hasErrors` envelope with zero applied counts, parity across TS and Go — never a new field, never a silent success. Both handlers must inspect the `ImportResult`, not discard it.

v1.6 Testing Slice Test-D (ForwardManager persist-failure rollback parity) fixed a real bug: the TypeScript `ForwardManager` mutated its in-memory rule map before `persist()` with no rollback, diverging from the Go manager which rolls back. `server/sources/forward-manager.ts` now restores prior state on a persist failure in `addRule`/`updateRule`/`deleteRule`/`reorderRules`/`importConfig` (restarting a forwarder stopped for a forwarding-field update, best-effort). Proven by `ControllableStore`-driven tests in `server/sources/forward-manager.test.ts` mirroring the Go `Test*PersistFailureRollsBack` tests. No API/forwarding/plan-apply change. Source: `audits/v1.6-testing-audit-1.md`.

**Durable testing rule (Test-D):** Persist-failure paths are correctness paths, not optional edge cases. Both rule managers (`server/sources/forward-manager.ts`, `service/sources/manager/manager.go`) must roll back so a failed persist leaves NO partial in-memory or running-state mutation, for create/update/delete/reorder/import. New mutating manager methods that persist must snapshot-and-restore on failure and have rollback tests in BOTH runtimes (TS via a controllable failing store; Go via `failingStore`). Intentional runtime divergence on these paths must be documented in both suites and the durable docs. Source: `audits/v1.6-testing-audit-1.md`.

v1.6 Fix Slice 1 (TS replace-import duplicate-binding parity): TypeScript `ForwardManager.importConfig` now rejects duplicate listen bindings within the imported set for BOTH replace and merge modes via a module-level `ensureNoDuplicateBindings(rules)` mirroring Go's `manager.ensureNoDuplicateBindings` (same wording), run after validation and before any mutation — no mutation/persist/start/stop, one `config.import.failed` (error). HTTP maps `result.errors` → 422 `{errors, result}` in both runtimes. Config **apply** was already protected (plan engine `detectDuplicateKeys` → `hasErrors` → `ok:false` before `importConfig`), resolving the duplicate-binding half of Resilience-C. `validate:contract` 167 → 171. No valid-import / merge-vs-existing / gate change. Source: `audits/v1.6-audit-synthesis-and-fix-plan.md`.

**Durable rule (Fix Slice 1):** Config import (replace AND merge) must enforce the same duplicate listen-binding rule in both runtimes. A listen binding is `protocol + listenHost + listenPort`; two rules in the imported set with that key conflict even if IDs differ, rejecting the whole import (422, no mutation/persist). Import parity changes need unit coverage in BOTH runtimes plus a `validate:contract` parity scenario; keep the TS/Go duplicate-binding message wording aligned. CLI stays a pure API client.

v1.6 Fix Slice 2 (TS ConfigStore atomic write parity): TypeScript `ConfigStore.save` (`server/sources/config-store.ts`) now writes crash-safely — unique same-directory temp file → `fsync` via FileHandle → atomic `rename`, mirroring Go `config.Store.Save`. Previous file intact until rename; temp removed on any pre-rename failure (best-effort, never masking the original error); persistence error propagates so `ForwardManager` rollback (Test-D) runs. `load` unchanged. No remove-and-retry recovery branch (Node `fs.rename` replaces atomically on POSIX + Windows `MoveFileExW REPLACE_EXISTING`); directory fsync omitted (Windows portability — same limit Go accepts). Minimal test-only `ConfigStoreFileOps` seam (optional 2nd constructor arg, defaults to real fs) forces write/sync/rename failures without a real disk-full event; not in any product/API path. `config-store.ts` 100/100/100; gates unchanged. Source: `audits/v1.6-resilience-audit-1.md`.

**Durable rule (Fix Slice 2):** Config persistence must be crash-safe in BOTH runtimes. Rules-config writes use same-directory temp file + `fsync`/flush where practical + atomic `rename`; a failed save must not corrupt the previous `rules.json`, and the original persistence error must reach the caller so manager rollback can execute. New persistence durability behavior needs failure-path tests in the owning runtime (TS via the `ConfigStoreFileOps` seam, Go via a temp/rename seam). Keep the seam minimal (mkdir/open-write-sync-close/rename/remove); do not add a broad filesystem abstraction. Directory-fsync metadata durability on Windows is an accepted documented limit in both runtimes.

v1.6 Coverage Slice C raised CLI coverage 93.2% → 97.7% (gate 93 → 95) with failure-path/exit-code tests only — no CLI behavior change, CLI stays a pure API client. Added `failingWriter` JSON-encode-error tests for every JSON-emitting command (`commands/jsonerr_test.go`), white-box `formatChangeValue`/`opEndpoint` tests (`commands/helpers_internal_test.go`), and a `run()` invalid-`--url` dispatch table (`main_test.go`). Structurally-unreachable branches documented (not chased) in `docs/coverage-baseline.md`: `main()` os.Exit, `client.do`/`doWithBody` and `writePrettyJSON` marshal/request errors. Source: `audits/v1.6-coverage-audit-1.md`.

**Durable testing rule (Coverage Slice C):** CLI command coverage must include failure-path behavior, not only happy paths. Keep the `output.PrintJSON` encode-failure path tested (exit 1 + "Error encoding JSON") for JSON-emitting commands, and keep config plan/diff/apply/export/validate exit-code priority tested: local file errors (2), connection/API errors (3/1), plan errors (1), destructive confirmation (1 via 400), dry-run (0, no mutation), drift `--fail-on-drift` (4). Do not contort production code or add fake unmarshalable DTOs to hit structurally-unreachable branches; document them in `docs/coverage-baseline.md`. Source: `audits/v1.6-coverage-audit-1.md`.

v1.6 Coverage Slice D raised service coverage 88.5% → 90.3% (gate 88 → 90) with tests-only error-path additions (no product code changed; `validate:contract` 167/167). White-box tests for API rejection paths (`api/errorpaths_test.go`: 400/404/500, `NewHandler` defaults, `tryTCPBind`/`tryUDPBind`), UDP constructor/mode-default, static asset serving, manager `StartEnabled`/`hasIDIn`, configplan udpMode diff, and an options positional-arg case. A persist-failing manager is built via `manager.NewWithStore` with a bad store path (no Load), so generic manager errors map to 500 without a production seam. Source: `audits/v1.6-coverage-audit-1.md`.

**Durable testing rule (Coverage Slice D):** Service coverage must prioritize correctness/error paths (persistence failures, forwarder start/stop failures, API rejection paths, rollback consistency) over line-hits. Do not chase structurally-unreachable socket internals (read-loop send errors, crypto/rand fallbacks, atomic-rename recovery) with invasive production-only seams — add a seam only if it improves production design, otherwise document the gap in `docs/coverage-baseline.md`. Socket tests must use the Test-A stabilized port helpers and deadline polling, never fixed sleeps. Source: `audits/v1.6-coverage-audit-1.md`.

v1.6 Coverage Slice E raised server coverage 95.4/92.2/100 → 98.9/93.6/100 (gates 89/91/99 → 95/92/99), tests-only (`validate:contract` 167/167). `diagnose.test.ts` gained TCP bind-fail / target-connect success+refused / DNS-fail-skip / one-way-mode / advisory-warn tests; `udp-forwarder.test.ts` gained the four send-callback error branches via clean instance-level `.send` injection + `waitUntil` deadline polling; `api.test.ts` gained an apply-drift persist-failure → 500 parity test. Structurally-hard branches (diagnose 2s timeouts, UDP post-stop/pre-client race guards, tcp-forwarder optional-registry guard, api.ts platform-normalize) documented in `docs/coverage-baseline.md`. Source: `audits/v1.6-coverage-audit-1.md`.

**Durable testing rule (Coverage Slice E):** Server coverage must prioritize meaningful runtime error paths (UDP send/socket errors, diagnose timeout/error aggregation, API rejection mapping, cleanup/idempotency) over line-hits. Socket/timeout tests use deadline polling + Test-A helpers, never fixed sleeps. Forcing a send-callback error via a clean instance-level `.send`/`.emit` override on one socket is allowed; brittle `dgram` prototype monkeypatching is not. Document structurally-unreachable Node socket internals in `docs/coverage-baseline.md` rather than force-covering. Source: `audits/v1.6-coverage-audit-1.md`.

v1.6 Testing Slice Test-E added a populated Live Connections TCP-table E2E and an API-failure error-banner E2E (`page.route` abort → `role="alert"`), and removed brittle selectors (`label.auto-refresh-toggle`, `.diag-panel-title`, `.diag-panel-body`). Tests-only; E2E 34/34 (twice); client coverage/gates unchanged; `validate:contract` 167/167. See `docs/e2e-coverage.md`.

**Durable testing rule (Test-E):** E2E must cover critical user-visible workflows, not only empty states — keep ≥1 Live Connections populated-state test (real held connection → populated row) and ≥1 API-failure regression (`page.route` abort → `role="alert"`, app stays usable). Prefer role/label/visible-text selectors over CSS classes; reserve exact-copy assertions for short stable empty-state/contractual strings and use `{ exact: true }`/role scoping to avoid strict-mode ambiguity. Socket/timing E2E uses real runtime + Playwright auto-waiting, never fixed sleeps; route interception only to force otherwise-undeterministic failures (documented in `docs/e2e-coverage.md`). Prefer adding a minimal accessibility attribute over asserting a styling class when a test needs a hook. Source: `audits/v1.6-testing-audit-1.md`.

Gates (in `scripts/validate-coverage.js`, ratcheted at v1.6-pre, client branch/funcs recalibrated at Slice A, service raised at Slice B, cli raised at Coverage Slice C):
- cli: statements ≥ 95%
- client: statements ≥ 94%, branches ≥ 89%, functions ≥ 78%
- server: statements ≥ 95%, branches ≥ 92%, functions ≥ 99%
- service: statements ≥ 90%
- shared: statements ≥ 100%, branches ≥ 100%, functions ≥ 100%

Coverage policy: require 100% meaningful coverage for all newly added or materially changed files in v1.5 and v1.6. Existing baselines ratcheted incrementally. Do not lower gates without explicit rationale. Do not remove gates to make a release pass.

---

## CLI Commands (v1.3)

```powershell
npm run build:cli              # build tools/cli/sources → tools/cli/build/portier[.exe]
npm run test:cli               # go test ./... inside tools/cli
npm run validate:cli           # test:cli + build:cli
npm run validate:coverage:cli  # cli coverage gate only (fails below 92%; 92.7% actual)
```

CLI binary: `portier` / `portier.exe`. Background service remains `service` / `service.exe`.

Global flags: `--url`, `--host`, `--port`, `--json`, `--version`, `-h`/`--help`.  
Environment: `PORTIER_URL`. Default URL: `http://127.0.0.1:47831`.  
Exit codes: `0` success, `1` API error, `2` invalid args, `3` connection failure, `4` drift detected (`config plan --fail-on-drift` / `config diff --fail-on-drift`).

Implemented commands: `list`, `status`, `activity` (with `--limit`/`--rule`/`--type`/`--severity`), `start <id|name>`, `stop <id|name>`, `diagnose <id|name>`, `config validate <file>`, `config export --out <file>`, `config import --mode merge|replace [--yes] <file>`, `config plan <file> [--fail-on-drift]`, `config diff <file> [--show-unchanged] [--fail-on-drift]`, `diagnostics export --out <file> [--run-diagnostics] [--activity-limit N]`, `runtime`, `version`, `help`.

Rule-targeting commands (`start`, `stop`, `diagnose`) accept an exact rule ID or an exact rule name. Duplicate names produce exit 2 with an ID disambiguation table on stderr.

`portier config validate` validates a local file without contacting the service. `portier config export/import` use `GET /api/config/export` and `POST /api/config/import`. Import validates locally first — invalid files are rejected without an API call. Replace mode requires `--yes`.

`portier diagnostics export` builds a local JSON support bundle (schemaVersion, runtime, rules, statuses, activity, diagnostics, metadata) from independent API calls. Partial source failures are recorded in `errors[]` rather than aborting. `--run-diagnostics` adds per-rule diagnose results. `--activity-limit` (1–500, default 100). 280+ CLI tests. Coverage: 97.7% total, gate 95% (raised at Coverage Slice C).

The CLI binary (`portier`/`portier.exe`) is included in the runtime package (`build/portier/`) and release artifacts. It is not added to PATH by the installer in v1.3.

## Packaging Commands

```powershell
npm run build:runtime             # cross-platform: builds build/portier/ on the current OS
npm run build:runtime:windows     # Windows package (build/windows/)
npm run build:runtime:macos       # macOS package (build/macos/)
npm run build:runtime:linux       # Linux package (build/linux/)
npm run build:clean               # clean build/portier/ and all platform package dirs
```

## macOS LaunchAgent Commands

Lifecycle management (user-level, no sudo):

```bash
bash scripts/macos/service/install-launch-agent.sh    # copies build/portier/ → ~/Applications/Portier/, registers LaunchAgent
bash scripts/macos/service/uninstall-launch-agent.sh  # stops and removes LaunchAgent; preserves rules.json
bash scripts/macos/service/start-launch-agent.sh      # start (or restart) the LaunchAgent
bash scripts/macos/service/stop-launch-agent.sh       # stop the LaunchAgent
bash scripts/macos/service/status-launch-agent.sh     # show LaunchAgent status via launchctl
```

Install script supports: `--source-dir`, `--install-dir`, `--config-path`, `--host`, `--port`, `--runtime service|node`, `--no-start`.
Uninstall script supports: `--purge` (removes config and logs; off by default).

macOS release archive:

```bash
npm run build:release:portable        # build:runtime then portable tar.gz
```

Output: `build/releases/macos/portier-portable-macos-<version>.tar.gz`

Unsigned. Gatekeeper may quarantine downloaded binaries. Sign with Developer ID for public distribution.
Do not add `build:release:*` to `npm run check` — it is a release step.

## Linux systemd Service Commands

Lifecycle management (requires root/sudo):

```bash
sudo bash scripts/linux/service/install-service.sh    # copies build/portier/ → /opt/portier/, registers systemd service
sudo bash scripts/linux/service/uninstall-service.sh  # stops and removes service; preserves /etc/portier/rules.json
sudo bash scripts/linux/service/start-service.sh      # start (or restart) the service
sudo bash scripts/linux/service/stop-service.sh       # stop the service
sudo bash scripts/linux/service/status-service.sh     # show service status via systemctl
```

Install script supports: `--source-dir`, `--install-dir`, `--config-path`, `--host`, `--port`, `--runtime service|node`, `--no-enable`, `--no-start`.
Uninstall script supports: `--remove-files` (removes `/opt/portier/`), `--remove-config` (removes config directory; off by default).

Linux release archive:

```bash
npm run build:release:portable        # build:runtime then portable tar.gz
```

Output: `build/releases/linux/portier-<version>-linux.tar.gz`

No signing required for Linux tar.gz. Firewall rules for forwarded ports are the user's responsibility (ufw, iptables, firewalld).
Do not add `build:release:*` to `npm run check` — it is a release step.

## Windows Release Commands

Build the portable zip and Inno Setup installer for Windows 10+ (Inno Setup 6 required for the installer):

```powershell
npm run build:release:current     # portable zip + installer (installer non-fatal if Inno Setup absent)
npm run build:release:portable    # portable zip only
```

Output:
- `build/releases/windows/portier-<version>-windows-portable.zip`
- `build/releases/windows/Portier-Setup-<version>.exe` (when Inno Setup available)

Build script: `scripts/windows/release/build-release.ps1`
- `-Version 1.2.0` — override version string (default: reads from `package.json`)
- `-NoPackage` — skip `build:runtime` step
- `-InnoPath "C:\..."` — path to `ISCC.exe` if not on PATH

The installer is unsigned. It does NOT create Windows Firewall rules. Config is preserved on uninstall.
Do not add `build:release:*` to `npm run check` — it is a release step.

## Package Validation Commands

```powershell
npm run validate:runtime           # validate existing build/portier/ layout
npm run validate:runtime:build     # build then validate
npm run validate:runtime:smoke     # build, validate, and run smoke test (preferred for release)
```

`validate:runtime:smoke` is the recommended pre-release package check. It does not require
Administrator or root. It does not install OS services.

## OS Service Install Validation Commands

Run these explicitly on the target platform — not included in `npm run check`:

```powershell
npm run validate:service:windows:user     # Windows scheduled task (no Administrator required)
npm run validate:service:windows:machine  # Windows Service (Administrator required)
```

```bash
npm run validate:service:macos    # macOS LaunchAgent (no sudo required)
npm run validate:service:linux    # Linux systemd (requires root/sudo)
npm run validate:service:current  # current platform, user-scope where possible
```

All service validation scripts accept:
- `--no-build` / `-NoBuild` — skip `build:runtime` build step
- `--keep-files` / `-KeepFiles` — preserve temp files for debugging
- `--port` / `-Port` — override the test port

Normal development validation: `npm run check`
Release package validation: `npm run validate:runtime:smoke`
Release service validation: `npm run validate:service:current` (or per-platform variants)

## Additional Validation Suites

Run explicitly — not part of `npm run check`. Slower or platform-sensitive.

```powershell
npm run validate:config            # fixture-based config compatibility validation
npm run validate:contract          # API contract parity (TypeScript + Go if available)
npm run validate:binary            # runtime binary behavior (build:runtime + 5 behavioral tests)
npm run validate:runtime:behavior  # alias for validate:binary (fits validate:runtime:* namespace)
npm run validate:scripts           # installer script static analysis + dry-run on current platform
```

**`validate:config`** — Loads every fixture from `tests/fixtures/config/` (17 fixtures: 8 valid, 8 invalid, 1 malformed JSON) and runs:
1. Static JSON parsing — valid fixtures parse; malformed-json fixture does not.
2. Config file loading — starts the TypeScript server (and Go service if available) with each valid fixture as `rules.json`; verifies rule count. The `{rules:[...]}` wrapper shape is tested against the Go service only (TypeScript config requires a raw array).
3. HTTP API import/export — imports each valid fixture via `POST /api/config/import`, verifies rule counts and UDP mode defaults, checks export shape stability.
4. Invalid fixture rejection — each invalid-field rule is posted via `POST /api/forwards` and must return 400 with `errors[]`.
5. Duplicate binding — posting a second rule with the same listen key must return 409.

TypeScript runtime is always checked. Go runtime is checked when `build/portier/service[.exe]` or `service/build/portier-service[.exe]` is present. Pass `--skip-go` to force skip. No real `rules.json` is used.

**`validate:contract`** — Starts the TypeScript server (and the Go binary if present) and runs all API scenarios: CRUD forwards, start/stop, status, activity, config export/import, port advisory, error shapes, duplicate binding, unknown-ID 404s. Since Arch-A it also verifies advisory/plan **content** parity: in-runtime canonical assertions for `LAN_EXPOSURE`/`MANAGEMENT_LAN_EXPOSURE` advisory code+severity+message and the `LAN_EXPOSURE` plan warning, plus a cross-runtime `compareParity` pass that captures deterministic normalized advisory and config-plan payloads from both runtimes and diffs them field-by-field (timestamps and generated ids excluded), reporting the exact mismatching path. Since Arch-D it also captures live runtime JSON and strictly decodes it into the CLI DTOs via `tools/cli`'s `TestCLIDTOContractParity` (skips if the `go` toolchain is absent). 167/167 against both runtimes. The Go binary used is `build/portier/service[.exe]` if present, else `service/build/portier-service[.exe]` — rebuild after Go advisory/plan changes. Skips Go parity with a clear message if the binary is not built. Use `--skip-go` to force skip.

**`validate:binary`** (also `validate:runtime:behavior`) — Runs `build:runtime` then tests `build/portier/service[.exe]` behavior:
1. `/api/health` responds on a free port
2. `/` serves HTML when `web/` static dir is present
3. API works when static dir is missing; `/` returns non-200
4. Invalid JSON config → process exits with non-zero code
5. Process terminates within 5s after kill signal

Pass `--no-build` to reuse an existing `build/portier/`.

**`validate:scripts`** — Static analysis + dry-run:
- All install scripts: no silent firewall commands
- Validate scripts: test-specific names (not production), no hard-coded port 47831
- `install-service.ps1`: `Format-Argument` quoting helper, `-DryRun` parameter
- `install-launch-agent.sh`: plist uses absolute paths (not `~`), `--dry-run` flag
- `install-service.sh`: default `INSTALL_DIR=/opt/portier`, default `CONFIG_PATH=/etc/portier/rules.json`, `--dry-run` flag
- Dry-run execution on the current platform validates planned output contains required fields without performing any real install

Naming convention:
- `npm run test` = unit/integration test runner (Vitest + Go test)
- `npm run test:e2e` = Playwright browser E2E tests
- `npm run validate:config` = fixture-based rules.json compatibility validation
- `npm run validate:contract` = TS/Go API parity validation runner
- `npm run validate:binary` / `validate:runtime:behavior` = packaged binary behavior validation
- `npm run validate:scripts` = installer script static analysis + dry-run validation

**Installer dry-run flags** (added to production install scripts):
- Windows: `install-service.ps1 -DryRun` — prints install plan (scope, paths, command line) and exits without creating dirs or registering services
- macOS: `install-launch-agent.sh --dry-run` — prints install plan (label, plist path, paths, ProgramArguments) and exits without creating files or loading LaunchAgent
- Linux: `install-service.sh --dry-run` — prints install plan (paths, service unit, ExecStart) and exits without creating files or running systemctl

## Release Artifact Commands

Build the current platform's portable archive (and installer if tooling is available):

```powershell
npm run build:release:current       # portable + installer (non-fatal if installer tools absent)
npm run build:release:portable      # portable archive only
```

Validate release artifacts:

```powershell
npm run validate:release:portable     # checks archive contents, readme.txt, forbidden files
npm run validate:release:current      # also checks installer artifact if present
```

Output layout: `build/releases/windows/`, `build/releases/macos/`, `build/releases/linux/`.

Archive filenames are versioned:
- Windows: `portier-<version>-windows-portable.zip`, `Portier-Setup-<version>.exe`
- macOS: `portier-portable-macos-<version>.tar.gz`
- Linux: `portier-<version>-linux.tar.gz`

Service binaries are platform-native. Run on each OS for that OS's artifacts.
Do not add `build:release:*` or `validate:release:*` to `npm run check` — they are release steps.

## Installer Strategy

v1.1 focuses on distribution and native OS service installers. The v1.1 scope, platform strategy, install layouts, artifact targets, and implementation slices are defined in `docs/installer-strategy.md`.

## Roadmap

v1.2 delivered diagnostics and operational polish: runtime info endpoint, rule diagnostics, activity log improvements, safer networking UX, settings polish, and diagnostics export.

v1.3 targets native CLI and automation: a Go-based `portier` CLI under `tools/cli/` that talks to the existing management API for terminal and script workflows. The CLI is an API client — not a runtime, not a scripts/ helper. Slices 2–7 complete: `tools/cli/` module scaffolded, HTTP client (`ConnectionError`/`APIError`), connection options (`--url`/`--host`/`--port`/`PORTIER_URL`), `--json` flag, `runtime`/`list`/`status`/`activity`/`start`/`stop`/`diagnose`/`config`/`diagnostics` commands, safe rule resolver, local config validation, `ExportConfig`/`ImportConfig`/`BaseURL` API client additions, diagnostics bundle builder (partial-failure tolerant, `--run-diagnostics`, `--activity-limit`), 153 CLI tests, `build:cli`/`test:cli`/`validate:cli` npm scripts; CLI binary now included in runtime package and release artifacts.

v1.4 delivered the Live Connection Inspector: `GET /api/connections` in both runtimes, TCP and UDP session tracking, rule summaries, and a dedicated Live Connections UI view (TCP/UDP/Summary tabs, filters, auto-refresh). Coverage hardened before the feature was built; 116/116 contract checks pass. Tagged 1.4.0.

v1.5 delivered declarative config and drift control. All 8 slices complete: shared types, API contract, matching semantics (Slice 1); pure plan engine and `POST /api/config/plan` in TypeScript server (Slice 2) and Go service (Slice 3); `portier config plan` and `portier config diff` CLI commands (Slice 4); `POST /api/config/apply` in both runtimes and `portier config apply --yes/--dry-run/--backup-out` CLI command (Slice 5); Settings UI Plan & Apply section — `planHelpers.ts`, `planConfig`/`applyConfig` API helpers, full plan preview with summary counts/errors/warnings/operations/destructive confirm, apply flow with form-clear-on-success (Slice 6); `validate:contract` 156/156; coverage gate hardening — all 5 gates ratcheted to v1.5.0 values (Slice 7); readiness audit, version bump to 1.5.0, changelog finalized (Slice 8). Tagged 1.5.0. See `docs/roadmap.md`.

Quality target for v1.4 and v1.5: all newly added or materially changed implementation areas should reach 100% meaningful test coverage, with explicit coverage gates where practical. This coverage push is a deliberate prerequisite for v1.6.

v1.6 is a dedicated Architecture, Quality & Maintainability Audit: a structured multi-angle inspection of architecture boundaries, runtime parity, forwarding correctness, API contract, CLI quality, UI quality, test quality, security/safety posture, packaging, and documentation consistency. The high coverage built in v1.4 and v1.5 is the safety net that makes v1.6 refactoring and hardening work safe to perform. Raw audit notes should not be dumped into `docs/`; durable audit outcomes belong in curated docs or a tracked backlog. See `docs/roadmap.md`.

## Coding Guidelines

- Keep TypeScript simple and explicit.
- Prefer small modules over clever abstractions.
- Keep shared validation and constants in `shared/sources`.
- Keep TCP and UDP forwarding logic separate.
- `ForwardManager` should own lifecycle orchestration.
- Keep runtime config external.
- Do not bake `rules.json` into executables.
- Do not expose the management UI/API on `0.0.0.0` by default.
- Management UI/API default: `127.0.0.1:47831`.
- Recommended forward listen port range: `48000-48999`.
- Go service static dir default: `web` (packaged layout). Dev: pass `--static-dir ../client/build`.
- TypeScript server static dir default: `client/build`. Prod: pass `--static-dir web` or set `PORTIER_STATIC_DIR`.

## Safety Rules

- Warn clearly when a forward rule listens on `0.0.0.0`.
- Treat management binding to `0.0.0.0` as dangerous.
- Do not silently change firewall rules.
- Do not add telemetry.
- Do not add remote update/download behavior.
- Do not store secrets in repo files.
- Do not modify local user config files unless explicitly requested.

## Do Not Edit Unless Explicitly Asked

- `node_modules/`
- `build/`
- `server/build/`
- `service/build/`
- `client/build/`
- `shared/build/`
- `coverage/`
- `.git/`
- `.env`
- `.env.*`
- `*.log`
- `rules.json`
- generated package outputs
- user-local config files

## API Documentation Rule

Whenever an API endpoint is added, removed, or changed, update **both** documentation surfaces:

1. `docs/api-contract.md` — the durable external/project API contract.
2. `client/sources/features/apidocs/ApiDocsView.tsx` — the user-facing in-app API reference.

Also update `client/sources/features/apidocs/ApiDocsView.test.tsx` for new endpoints.

Do not consider an API slice complete until both documentation surfaces and their tests are updated.

## Networking Checklist

- TCP sockets clean up on error and close.
- UDP `bidirectional-last-client` mode is documented as limited.
- Do not claim full multi-client UDP support unless implemented.
- Duplicate `protocol + listenHost + listenPort` bindings are rejected.
- Shutdown closes active sockets and servers.

## Response Expectations

- Summarize changed files.
- List validation commands run.
- State anything not run and why.
- Call out follow-up tasks and risks.

## Repository Naming

- Use `sources/` for TypeScript source directories.
- Use `build/` for generated build outputs.
- Keep executable automation scripts under `scripts/`, with `release/` (artifact builders), `service/` (OS service lifecycle), and platform docs/templates under `scripts/windows/`, `scripts/macos/`, and `scripts/linux/`.
- Use `tools/` for user-facing or developer-facing project tools that are not repo automation. `tools/cli/` is the v1.3 portier CLI. Future possible tools: `tools/bench/` (benchmarking), `tools/replay/` (scenario replay). Do not mix tools into `scripts/` or `service/`.
- Normal documentation filenames are lowercase, such as `docs/architecture.md` and `docs/checklist.md`. The root `README.md` is uppercase.
- Keep tool-required files uppercase: `AGENTS.md`, `CLAUDE.md`, and `SKILL.md` in Codex/Claude skill directories.
- React component and view files under `client/sources/` use **CamelCase** filenames (e.g., `ForwardRuleList.tsx`, `StatCard.tsx`). Non-component files use the existing repo convention (e.g., `format.ts`, `nav.ts`).
- Go CLI command files under `tools/cli/sources/commands/` are named by **responsibility** — each command file is a bare noun matching its command (`config.go`, `diagnose.go`, `diagnostics.go`, `runtime.go`, `list.go`, `status.go`, `start.go`, `stop.go`, `activity.go`), with cross-cutting helpers named for what they own (`url.go` = connection/URL resolution, `resolver.go` = rule id/name resolution, `root.go` = dispatch/help). No `cmd` suffix (removed in v1.7 Slice 2), and do not name a file for a type it merely returns when its real job is something else. Cosmetic renames are `git mv` only — never change the `commands`/`commands_test` package, exported symbols, command names, flags, output, or exit codes. The CLI stays a pure API client.
