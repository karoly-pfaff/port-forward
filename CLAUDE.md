# Portier Claude Code Guide

## Project Identity

- App name: Portier
- Repository name: portier-port-forwarding
- Purpose: local TCP/UDP port forwarding manager for development and LAN testing

## Important Naming Conventions

- Use `sources`, not `src`.
- Use `build`, not `dist`.
- Use `scripts` for executable automation.
- Use lowercase filenames for normal markdown docs.
- Keep tool-required files uppercase: `AGENTS.md`, `CLAUDE.md`, and `SKILL.md`.
- Keep `README.md` file in the root uppercase.
- React component and view files under `client/sources/` use **CamelCase** filenames (e.g., `ForwardRuleList.tsx`, `StatCard.tsx`, `Sidebar.tsx`).
- Non-component files (utilities, types, config) keep the existing repo convention (e.g., `format.ts`, `nav.ts`, `portierApi.ts`).
- Go CLI command files under `tools/cli/sources/commands/` are named by **responsibility**: each command file is a bare noun matching its command (`config.go`, `diagnose.go`, `diagnostics.go`, `runtime.go`, `list.go`, `status.go`, `start.go`, `stop.go`, `activity.go`), plus cross-cutting helpers named for what they own (`url.go` = connection-target/URL resolution, `resolver.go` = rule id/name resolution, `root.go` = dispatch/help). Do **not** add a `cmd` suffix (the historical `configcmd.go`/`diagnosticscmd.go` suffix was only collision-avoidance and was removed in v1.7 Slice 2), and do **not** name a file for a type it merely returns when its real job is something else (the old `config.go` held URL resolution, not config commands). Renames are `git mv` only — never change the `commands`/`commands_test` package, exported symbols, command names, flags, output, or exit codes for a cosmetic rename. The CLI stays a pure API client.
- For canonical Portier terminology (forward rule, config plan/apply/import, runtime/server/service, advisory vs warning, diagnose vs diagnostics export, live connection vs UDP session), see `docs/glossary.md`. New docs/API/CLI/UI wording should use glossary terms, or update the glossary if a genuinely new concept is introduced. Do not rename public API fields or REST paths (e.g. `/api/forwards`, `listenHost`/`targetHost`, `clientAddress`/`targetAddress`) for cosmetic consistency; document any exception in the glossary.

## Architecture

- `server/sources` handles the Node.js service, REST API, config persistence, and forwarding lifecycle.
- `service/sources` handles the native Go service runtime focused on smaller binaries and service deployment.
- `client/sources` handles the React UI only.
- `shared/sources` owns types, validation, port constants, and port advisory logic.
- `scripts` contains executable scripts; each platform subdir (`windows/`, `macos/`, `linux/`) also contains its platform docs and templates.
- `tools` contains user-facing and developer-facing project tools. `tools/cli/` is the v1.3 portier CLI (a Go-based API client for the management API). Future possible tools: `tools/bench/`, `tools/replay/`. Do not place tools in `scripts/` or `service/`.

## Coding Rules

- Prefer small, readable changes.
- Do not rewrite the entire app unless explicitly asked.
- Keep TypeScript simple and explicit.
- Keep TCP and UDP forwarding logic understandable and testable.
- Keep runtime config external.
- Do not bake `rules.json` into packaged executables.
- Do not expose the management UI/API on `0.0.0.0` by default.
- The management UI/API defaults to `127.0.0.1:47831`.
- Recommended forward listen port range: `48000-48999`.
- Update docs when behavior changes.

## Security And Safety Rules

- Treat `0.0.0.0` as LAN exposure and warn clearly.
- Do not silently change firewall, service, or system-level behavior.
- Do not add telemetry.
- Do not add remote update/download behavior.
- Do not store secrets in repo files.
- Do not modify user config files unless explicitly requested.
- Do not change service scripts casually.
- Do not edit generated outputs or build artifacts directly.

## Files Claude Should Generally Avoid Editing

Do not edit these unless explicitly asked:

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
- `C:\ProgramData\Portier` examples except docs/templates
- generated package outputs under `build/windows/`
- `test-results/` (Playwright artifacts — gitignored, auto-generated)
- `playwright-report/` (Playwright HTML reports — gitignored)

## Preferred Validation Commands

Run the narrowest relevant validation first, then broaden before finishing:

```powershell
npm run lint
npm run typecheck
npm run test
npm run build
npm run check
```

For E2E tests (browser-level validation of user flows):

```powershell
npm run build:client       # required before running E2E
npm run test:e2e           # runs Playwright against TypeScript server on port 47890
npm run test:e2e:fresh     # build:client + test:e2e combined
```

For CLI (Go — tools/cli/):

```powershell
npm run test:cli              # go test ./... inside tools/cli (uses httptest; no running service needed)
npm run build:cli             # builds tools/cli/build/portier[.exe]
npm run validate:cli          # test:cli + build:cli
```

For coverage (reporting and gate validation):

```powershell
npm run coverage:shared     # shared TypeScript (vitest + v8)
npm run coverage:server     # server TypeScript (vitest + v8)
npm run coverage:client     # client TypeScript/React (vitest + v8)
npm run coverage:service    # Go service sequential coverage (~30s)
npm run coverage:cli        # Go CLI reporting only (no gate)
npm run coverage:baseline   # all five in sequence (reporting only)
npm run validate:coverage   # runs all + enforces gates; exits 1 if any gate fails
npm run validate:coverage:shared   # shared only
npm run validate:coverage:server   # server only
npm run validate:coverage:client   # client only
npm run validate:coverage:service  # service only
npm run validate:coverage:cli      # cli only
```

Coverage outputs written to `coverage/` (gitignored). Vitest writes json-summary per workspace; Go profiles are written and removed per run. Gates are defined in `scripts/validate-coverage.js`.

Baseline (v1.6-pre, recalibrated at Slice A; cli updated at Coverage Slice C, service at Coverage Slice D, server at Coverage Slice E): cli 97.7% (gate 95%), client ~95%/~89-90%/~78-80% (gate 94/89/78), service 90.3% (gate 90%), shared 100% (gate 100/100/100), server 98.9%/93.6%/100% (gate 95/92/99). Client numbers vary ±1% due to Windows vitest ghost-entry deduplication — both runs pass. See `docs/coverage-baseline.md`.

v1.6-pre gates (post-Slice-A; cli raised at Coverage Slice C, service at Coverage Slice D, server at Coverage Slice E): cli `{stmts:95}`, client `{stmts:94, branch:89, funcs:78}`, server `{stmts:95, branch:92, funcs:99}`, service `{stmts:90}`, shared `{stmts:100, branch:100, funcs:100}`.

v1.6 Coverage Slice E (TypeScript server error paths): raised server coverage 95.4/92.2/100 → 98.9/93.6/100 (gates 89/91/99 → 95/92/99) with tests-only additions — no product code changed; `validate:contract` 167/167. `diagnose.test.ts` 85.9% → 98.0%: TCP listen-bind failure (occupied port), target-connect success (local server) / refused (unbound port) / DNS-resolution failure (`.invalid` host → target-connect skip), one-way udp-mode, and 0.0.0.0 / privileged-port / common-port advisory warns. `udp-forwarder.test.ts` 86.3% → 99.4%: the four send-callback error branches (one-way target send, multi-client session send on reuse, last-client return send, multi-client return send) emit `udp.packet.error` + record lastError — driven by clean instance-level `.send` injection (matching the existing `socket.emit("error")` tests), with a `waitUntil` deadline poll (no fixed sleeps). `api.test.ts`: apply-with-drift persist failure → 500 (parity with Go Slice D, via an inline failing `RuleStore`). Structurally-hard branches documented in `docs/coverage-baseline.md` (not chased): the 2 s diagnose bind/connect **timeout** branches (`tryTcpBind`/`tryUdpBind`/`tryTcpConnect` timeout), the UDP `if (!this.listenSocket)`/`if (!this.lastClient)` post-stop/pre-client race guards, the `tcp-forwarder.ts` optional-registry guard (90% branch, by-design), and the api.ts platform-normalize branches (Windows-only env). Source: `audits/v1.6-coverage-audit-1.md`, `audits/v1.6-testing-audit-1.md`.

**Durable testing rule (Coverage Slice E):** Server coverage must prioritize meaningful runtime error paths — UDP send/socket errors, diagnose timeout/error aggregation, API rejection mapping, and cleanup/idempotency — over line-hits. Socket and timeout tests must use deadline polling and Test-A helpers, never fixed sleeps. Forcing send-callback error branches via a clean instance-level `.send`/`.emit` override on a specific socket is allowed; brittle prototype/`dgram` monkeypatching is not. Structurally-unreachable Node socket internals (real send-buffer timeouts, post-stop reply races) are documented in `docs/coverage-baseline.md`, not force-covered.

v1.6 Testing Slice Test-E (client/E2E meaningful coverage): added a populated Live Connections TCP-table E2E (real forwarded connection held open → populated row) and an API-failure error-banner E2E (`page.route` aborts `/api/connections` → `role="alert"`), and cleaned brittle selectors (removed `label.auto-refresh-toggle`, `.diag-panel-title`, `.diag-panel-body`). Tests-only; E2E 34/34 (run twice, no fixed sleeps); client coverage/gates unchanged; `validate:contract` 167/167. See `docs/e2e-coverage.md`. Source: `audits/v1.6-testing-audit-1.md`.

**Durable testing rule (Test-E):** E2E must cover critical user-visible workflows, not only empty states — keep at least one Live Connections **populated-state** test (real held connection → populated table row) and one **API-failure** regression (`page.route` abort → `role="alert"` banner, app stays usable). Prefer role/label/visible-text selectors over CSS-class selectors; reserve exact-copy assertions for short stable empty-state messages and contractually meaningful strings, and use `{ exact: true }` / role scoping to avoid strict-mode ambiguity. Socket/timing E2E uses real runtime + Playwright auto-waiting/deadline polling, never fixed sleeps; route interception is allowed only to force otherwise-undeterministic failure states (document it in `docs/e2e-coverage.md`). When the UI lacks an accessible hook a test needs, prefer adding a minimal accessibility attribute over asserting a styling class.

v1.6 Coverage Slice D (Go service error paths): raised service coverage 88.5% → 90.3% (gate 88 → 90) with tests-only additions — no product code changed; `validate:contract` 167/167. New white-box tests (`service/sources/api/errorpaths_test.go`) cover API rejection/error paths: empty/failed body reads → 400, malformed JSON → 400, unknown rule PATCH → 404, generic manager error → 500 (via a `manager.NewWithStore` handler whose persist always fails — `os.MkdirAll` under a regular file, cross-platform), config-apply ImportConfig failure → 500, `NewHandler` option defaults, and `tryTCPBind`/`tryUDPBind` success+failure. Plus `forwarders/udp_errorpaths_test.go` (public `NewUDPForwarder` + `udpMode` nil-default via the Test-A `startUDPForwarderOnFreePort` helper), `static/static_test.go` (`ServeClient` asset vs SPA-fallback, `HasClient`), `manager/errorpaths_test.go` (`StartEnabled` skip-disabled, `hasIDIn` merge-ID-collision regeneration), `configplan/udpmode_test.go` (`udpModeEqual`/`udpModeVal` nil and non-nil branches), and one `options` positional-arg case. Structurally-unreachable/seam-requiring branches documented in `docs/coverage-baseline.md` (not chased): crypto/rand fallbacks (`randomEventID`/`NewRuleID`/`generateConnectionID`), `config.Save` write/sync/rename injection (Go `os.Rename` replaces atomically, so the recovery path is dead), and the UDP forwarder read-loop send-error branches (`emitPacketError`, `handleMultiClientPacket`/`sessionReadLoop` write errors) which need a `net.Conn` injection seam not worth adding now. Source: `audits/v1.6-coverage-audit-1.md`.

**Durable testing rule (Coverage Slice D):** Service coverage must prioritize correctness/error paths — persistence failures, forwarder start/stop failures, API rejection paths (400/404/409/500), and rollback consistency — over line-hits. Do not chase structurally-unreachable socket internals (read-loop send errors, crypto/rand fallbacks, atomic-rename recovery) with invasive production-only seams; add a seam only if it improves production design, otherwise document the gap in `docs/coverage-baseline.md`. Socket tests must use the Test-A stabilized port helpers and deadline polling — never fixed sleeps. Generic (non-typed) manager errors are reproduced cleanly via `manager.NewWithStore` with a persist-failing store path, not by mocking.

v1.6 Coverage Slice C (CLI command edge cases): raised CLI coverage 93.2% → 97.7% (gate 93 → 95) by adding failure-path/exit-code tests — no CLI behavior change, CLI stays a pure API client. New tests: a shared `failingWriter` exercising the `output.PrintJSON → "Error encoding JSON" → exit 1` branch in every JSON-emitting command (`commands/jsonerr_test.go`); white-box tests for `formatChangeValue` (nil/int-float/non-int-float/string/bool) and `opEndpoint` (current/desired/both-nil) in `commands/helpers_internal_test.go`; and a `run()` dispatch table asserting an invalid `--url` exits 2 for every subcommand (`main_test.go`). Structurally-unreachable CLI branches left uncovered and documented in `docs/coverage-baseline.md`: `main()` os.Exit wrapper, `client.do` http.NewRequest/read-body errors, `client.doWithBody` and `writePrettyJSON` `json.Marshal(Indent)` errors (impossible with current DTOs — do not add fake unmarshalable types). Source: `audits/v1.6-coverage-audit-1.md`.

**Durable testing rule (Coverage Slice C):** CLI command coverage must include failure-path behavior, not only happy paths. Every JSON-emitting command must keep its `output.PrintJSON` encode-failure path tested (exit 1 + "Error encoding JSON"), and the config plan/diff/apply/export/validate commands must keep exit-code priority tested: local file errors (2), connection/API errors (3/1), plan errors (1), destructive-without-`--yes` (1 via 400), dry-run (0, no mutation), and drift with `--fail-on-drift` (4). Do not chase structurally-unreachable branches (os.Exit wrappers, `json.Marshal` errors on concrete DTOs) — document them in `docs/coverage-baseline.md` instead.

v1.6 Slice B (post-Slice-B): service 88.6% (gate raised to 88%). 9 new Go tests for manager rollback and config error paths.

v1.6 Architecture Slice Arch-A (contract drift guard): aligned the Go `LAN_EXPOSURE` advisory message (`service/sources/advisory/advisory.go`) to the canonical TypeScript wording in `@portier/shared` (`shared/sources/index.ts`); advisory code/severity unchanged. `validate:contract` now also asserts advisory/plan **content** (message text), not just codes/shape: in-runtime canonical assertions plus a cross-runtime `compareParity` pass diffing normalized advisory and config-plan payloads field-by-field (timestamps/generated ids excluded). `validate:contract` 166/166 against both runtimes (was 156/156). No API shape/behavior change, no plan/apply semantic change, no coverage gate change. Treat `@portier/shared` as canonical for advisory wording; the Go service must match it. Source: `audits/v1.6-architecture-audit-1.md`.

v1.6 Architecture Slice Arch-B (apply orchestration extraction): config apply transformation moved out of the HTTP handlers into tested plan-engine helpers — `buildApplyImportFromPlan` (`server/sources/config-plan.ts`) and `BuildApplyImportFromPlan` (`service/sources/configplan/plan.go`). Behavior preserved exactly; `validate:contract` 166/166; server coverage 95.2/91.6 → 95.3/92.0, service unchanged; no gate change. Source: `audits/v1.6-architecture-audit-1.md`. Next: Arch-C (Go service safe dedupe — unify UUID generation, collapse repeated manager activity-emission blocks).

**Durable architecture rule (Arch-B):** Config apply orchestration must live beside the plan engine (`server/sources/config-plan.ts`, `service/sources/configplan/plan.go`), not inside HTTP handlers. HTTP handlers may enforce request/response concerns, confirmation (`yes`), dry-run, and status codes, but plan/apply transformation logic (deriving the desired replace list, id injection/preservation, applied counts) must remain in the tested config-plan/configplan helpers. The TypeScript and Go helpers must stay semantically mirrored; new apply behavior must add unit tests in both runtimes plus `validate:contract` parity assertions where externally observable. `validate:contract` is the parity guard for observable behavior.

v1.6 Architecture Slice Arch-C1 (Go ID dedupe + dead code): replaced the duplicate UUID generators `manager.randomID` and `api.newApplyRuleID` with one shared `domain.NewRuleID()` (`service/sources/domain/id.go`); removed a no-op `name` assignment in `api.go` `buildRuleLiveSummary`. Behavior preserved; `validate:contract` 166/166; coverage neutral; no gate change. Source: `audits/v1.6-architecture-audit-1.md`. Next: Arch-C2 (collapse repeated Go manager activity-emission blocks into one helper).

**Durable architecture rule (Arch-C1):** Go service rule ID generation must have one shared implementation (`domain.NewRuleID`). New rule IDs for manager-created rules and config-apply-created rules must use the same generator so the two paths cannot silently drift in format. Do not reintroduce per-package UUID helpers.

v1.6 Architecture Slice Arch-C2 (manager activity-emission dedupe): collapsed the 10 repeated rule-scoped emission blocks in `manager.go` into one `emitRuleEvent(eventType, severity, rule, message)` helper (create/update/delete/start/stop/error). Config-level events (export/import/import-failed) keep their own `emitActivity` calls (they carry `details`, no rule fields). Payloads preserved byte-for-byte; full-payload regression tests added in `manager_test.go`. `validate:contract` 166/166; service 88.6% → 88.5% (no gate change). Arch-C complete. Source: `audits/v1.6-architecture-audit-1.md`. Next: Arch-D (CLI DTO parity guard) or resume coverage push.

**Durable architecture rule (Arch-C2):** Rule-scoped Go manager activity events must be emitted through `emitRuleEvent` (so ruleId/ruleName/protocol shape cannot drift between call sites); non-rule-scoped config events stay on `emitActivity` with their own details. Activity payloads are user-visible diagnostics — any change must be covered by tests asserting type, severity, ruleId, ruleName, protocol, message, and details.

v1.6 Architecture Slice Arch-D (CLI DTO live-runtime parity guard): `validate:contract` now captures live JSON from both runtimes and strictly decodes it into the CLI DTOs via `TestCLIDTOContractParity` (`tools/cli/sources/client/contract_decode_test.go`, env-gated by `PORTIER_CLI_CONTRACT_FIXTURES`). Proves the CLI's third contract copy matches real runtime output, not just httptest mocks. `validate:contract` 167 passed; no coverage/gate change; CLI stays a pure API client. Arch-D completes the audit's contract-parity remediation. Source: `audits/v1.6-architecture-audit-1.md`. Next: resume coverage push.

**Durable architecture rule (Arch-D):** The CLI DTOs (`tools/cli/sources/client/client.go`) are a third copy of the REST contract and must be guarded against **live** runtime responses, not only httptest mocks. The CLI must stay a pure API client — it must not import server/service internals or become a second runtime. When a new API response family is added, update BOTH the `validate:contract` live capture (`scripts/validate-contract.js`) AND the CLI decode guard (`contract_decode_test.go`). `/api/connections` is out of CLI scope (no CLI DTO/command).

v1.6 Testing Slice Test-A (port-allocation stabilization): removed the EADDRINUSE/TOCTOU flake in socket-binding tests. `free*Port` helpers allocate→close→return a port; another parallel test/process can grab it before the forwarder/manager binds. Fix is test-only (Pattern C: bounded retry around the *bind step only*, only on EADDRINUSE, never the whole test). Helpers: `isAddrInUse` + `startTCPForwarderOnFreePort`/`startUDPForwarderOnFreePort` (`service/sources/forwarders/portretry_test.go`), `startRuleStable` (`service/sources/manager/portretry_test.go`, rebinds via `UpdateRule`), `startForwarderOnFreePort` (`server/sources/test-helpers.ts`). `scripts/validate-contract.js` `startServer` retries child bind on a fresh port and `waitForReady` bails on child exit. Migrated: Go `tcp_test.go`/`udp_test.go`/`manager_test.go` happy-path binds, TS `tcp-forwarder.test.ts`. Residuals (follow-up): TS `udp-forwarder.test.ts`, the api HTTP `/start` sites, E2E `port.ts` (E2E is serial — `workers:1` — so no intra-suite race). No coverage/gate change. Source: `audits/v1.6-testing-audit-1.md`.

**Durable testing rule (Test-A):** Tests must not rely on allocate-close-rebind port helpers when a listener handoff or bind-retry can be used. Socket readiness/ordering must use deadline polling, not fixed sleeps. EADDRINUSE flakes must be fixed at the helper/bind-operation level (bounded retry on a fresh port, only on EADDRINUSE) — never hidden by retrying whole tests or whole suites, and never by arbitrary sleeps. New socket-binding tests should build+start through the `startForwarderOnFreePort`/`startTCPForwarderOnFreePort`/`startUDPForwarderOnFreePort`/`startRuleStable` helpers (or an equivalent bounded bind-retry), and intentional bind-failure tests (which reserve a port to force EADDRINUSE) must stay outside those helpers.

v1.6 Fix Slice 3 (finish Test-A residual migration): consumed the bind-retry/live-handoff helpers in the two named TS residual suites. New TS helpers in `server/sources/test-helpers.ts`: `startRuleStable(manager, ruleId, freePort)` (mirrors the Go helper — retries `manager.startRule` on EADDRINUSE by rebinding the rule's `listenPort` via `updateRule`), `startTcpServerOnFreePort()` and `bindUdpSocketOnFreePort()` (bind to port 0 and return the LIVE listener/socket + actual port — no close-rebind window). Migrated: `server/sources/forwarders/udp-forwarder.test.ts` (forwarder listen binds → `startForwarderOnFreePort` via a local `startUdpForwarder`; UDP target/echo sockets → `bindUdpSocketOnFreePort`; several fixed-sleep session-count waits → `waitUntil` deadline polls) and `server/sources/forward-manager.test.ts` (manager `startRule` sites incl. the Test-D rollback tests → `startRuleStable`; TCP target servers → `startTcpServerOnFreePort`; removed the local allocate-then-bind `startTcpTarget`). Intentional-conflict tests left bypassing the helpers (UDP "start() rejects when port is already bound" with a live blocker; manager "logs rule.error … when startRule fails" with a live occupier; the merge-import rollback test already tolerates the race by asserting not-running). Tests-only; 58 migrated tests pass ×3; server 354 tests, 98.9/93.8/100 unchanged; `validate:contract` 171/171, `validate:config` 71/71, all gates unchanged. **Remaining accepted residuals (documented, bounded):** the HTTP `/start` sites in `server/sources/api.test.ts` and `service/sources/api/api_test.go` (the bind happens behind the HTTP handler and each test asserts the start-response shape, so a robust fix needs an HTTP-level recreate-on-EADDRINUSE helper — a larger, lower-value follow-up), and the E2E `tests/e2e/helpers/port.ts` allocate-close helpers (E2E is fully serial — `workers:1`, `fullyParallel:false` — so no intra-process race is possible; only a rare cross-process race remains, against a single dedicated server). Revisit trigger: if an HTTP `/start` or E2E EADDRINUSE flake is actually observed. Source: `audits/v1.6-audit-synthesis-and-fix-plan.md`.

v1.6 Fix Slice 4 (validate-contract.js scenario registry): the ~1090-line monolithic `runScenarios` in `scripts/validate-contract.js` is now 12 named scenario-group functions (`runRuntimeScenarios`, `runForwardsScenarios`, `runForwardLifecycleScenarios`, `runActivityScenarios`, `runConfigExportImportScenarios`, `runPortAdvisoryScenarios`, `runForwardDeleteScenarios`, `runConnectionsScenarios`, `runConfigPlanScenarios`, `runConfigApplyScenarios`, `runErrorEnvelopeScenarios`, `runDiagnoseScenarios`) driven by an ordered `scenarioGroups` registry; `runScenarios` is a slim wrapper building `ctx = { api, runtime, fixtureDir, state }` and iterating the registry, then the Arch-D CLI fixture capture last. Cross-group rule ids (`tcpId`/`udpId`/`udpDefaultId`) flow via `ctx.state`; group-local values (`exportedConfig`) stay local. Added `expectedRuntimeApiValue(label)` for the runtime-label→API-value mapping (Naming-D). Script-only; assertions verbatim; `validate:contract` 171/171 unchanged (the file uses intentional `\x00` sort-key separators in the `normalize*` helpers — preserve them on any future edit). Parity comparison (`compareParity`), CLI DTO guard, and child cleanup unchanged. Source: complexity/SOLID/pattern/duplication/readability audits.

**Durable rule (Fix Slice 4):** Contract scenarios must live in named `run<Area>Scenarios(ctx)` groups registered in the ordered `scenarioGroups` array — not in a monolithic `runScenarios` body. A new API endpoint family adds or extends a focused group + a registry entry; cross-group state flows via `ctx.state`, group-local state stays local. `validate:contract` scenario-count changes must be intentional and documented (171 at Slice 4; **183 at Slice 5**). `scripts/validate-contract.js` contains intentional embedded `\x00` bytes (sort-key separators) — edit with tools that preserve them (the file reads as "binary" to ripgrep).

v1.6 Fix Slice 5 (activity value parity guard): `validate:contract` now guards the activity taxonomy at the value level, not just `ActivityEvent` shape. Canonical `EXPECTED_ACTIVITY_TYPES` (17) / `EXPECTED_ACTIVITY_SEVERITIES` (4) in `scripts/validate-contract.js`; `runActivityValueSetParityChecks()` reads the TS union (`shared/sources/activity.ts`) and Go consts (`service/sources/activity/activity.go`) from source (binary-independent) and asserts both declared sets equal the canonical set; `runActivityScenarios` asserts every emitted type/severity ∈ allowed set + the representative `rule.*` pairs; the config-import group asserts the `config.imported`/`config.import.failed` (success/error) emission. Count 171 → 183. No product/API-shape/gate change; no event names changed.

**Durable rule (Fix Slice 5):** Activity event-type and severity values are contract values, not cosmetic labels. The canonical set is 17 types + 4 severities (`info`/`success`/`warning`/`error`); there is no `config.reordered`. Any new type/severity must be added consistently across `@portier/shared` (`shared/sources/activity.ts`), the Go consts (`service/sources/activity/activity.go`), `docs/api-contract.md`, the emitting code in both managers, and the `validate:contract` `EXPECTED_ACTIVITY_*` sets — together, in one change. `validate:contract` must keep asserting value-level membership AND cross-runtime source-set parity, not only shape. The CLI stays a pure API client (no activity-type logic).

v1.6 Fix Slice 6 (TS emitRuleEvent helper): the TypeScript `ForwardManager` now emits the 6 rule-scoped events (`rule.created`/`updated`/`deleted`/`started`/`stopped`/`error`) through a single private `emitRuleEvent(type, severity, rule, message)` helper (`server/sources/forward-manager.ts`), mirroring the Go `manager.emitRuleEvent` (Arch-C2) — payloads byte-identical, `activity?.add` optional-chaining preserved, `ActivityStore` still owns id/timestamp. The 5 config-level events (`config.exported`/`config.imported`/3× `config.import.failed`) stay on direct `activity?.add` (carry `details`, not rule-scoped). 3 full-payload regression tests added. No API/type/severity/message/order/count change; server branch 93.8 → 94.0, gates unchanged; `validate:contract` 183/183.

**Durable rule (Fix Slice 6):** Rule-scoped activity events must be emitted through the dedicated `emitRuleEvent` helper in BOTH runtimes (`server/sources/forward-manager.ts`, `service/sources/manager/manager.go`) so their `{type, severity, ruleId, ruleName, protocol, message}` payload cannot drift between call sites. Config-level events that carry a `details` map (or are not rule-scoped) stay on the direct `activity?.add`/`emitActivity` path unless a dedicated config-event helper is introduced in both runtimes. Activity payloads are user-visible diagnostics — any change to a rule-scoped emission must keep full-payload tests (type/severity/ruleId/ruleName/protocol/message) green in both runtimes, and value changes follow the Fix Slice 5 rule.

v1.6 Fix Slice 7 (config apply importConfig result.errors invariant): both apply handlers (`server/sources/api.ts`, `service/sources/api/api.go`) now inspect the `ImportResult` instead of discarding it. After import, if `result.errors` is non-empty → `200 ok:false` with the errors surfaced through the existing `plan.errors` field (code `IMPORT_ERROR`) + `plan.summary.hasErrors:true` + zero applied counts (no new `ConfigApplyResponse` field). Currently no `result.errors` path is reachable from apply (duplicate bindings pre-blocked by the plan engine's `detectDuplicateKeys`, invalid rules by plan validation, persist throws → 500, merge errors N/A in replace) — this is a belt-and-suspenders guard. Go construction extracted to the unit-tested `applyImportErrorResponse` helper (handler guard structurally unreachable without the deferred manager-interface seam; documented in `docs/coverage-baseline.md`). `validate:contract` 183 → 185 (duplicate-binding apply parity). CLI unaffected (`applyExitCode` already maps `ok:false`→1). No API-shape/valid-behavior change; gates unchanged.

**Durable rule (Fix Slice 7):** Config apply must never report `ok:true` when the underlying import operation reports errors. Plan-level errors (including duplicate desired listen bindings — `protocol+listenHost+listenPort` — flagged by the plan engine) must stop apply BEFORE import and stay contract-covered. Import-level errors that surface after plan validation must be reported as `ok:false` via the existing `plan.errors`/`summary.hasErrors` envelope with zero applied counts, byte-for-byte parity across TS and Go — never a new response field, never a silent success. Both apply handlers must inspect the `ImportResult`, not discard it.

v1.6 Testing Slice Test-D (ForwardManager persist-failure rollback parity): the TypeScript `ForwardManager` previously mutated its in-memory rule map before `persist()` with no rollback, so a failed `store.save()` left in-memory rules inconsistent with `rules.json` and divergent from the Go manager (which rolls back). Fixed in `server/sources/forward-manager.ts`: `addRule`/`updateRule`/`deleteRule`/`reorderRules`/`importConfig` now restore prior state on a persist failure (and restart a forwarder that was stopped for a forwarding-field update, best-effort so a restart error cannot mask the persist error). Rollback parity is proven by `ControllableStore`-driven tests in `server/sources/forward-manager.test.ts` mirroring the Go `Test*PersistFailureRollsBack` tests. No API/forwarding/plan-apply change; rollback only triggers on disk-write failure. Source: `audits/v1.6-testing-audit-1.md`.

**Durable testing rule (Test-D):** Persist-failure paths are correctness paths, not optional edge cases. Both runtimes' rule managers (`server/sources/forward-manager.ts` and `service/sources/manager/manager.go`) must roll back so that a failed persist leaves NO partial in-memory or running-state mutation, for create / update / delete / reorder / import. Any new mutating manager method that persists must snapshot-and-restore on persist failure, and must have rollback tests in BOTH runtimes (TS via a controllable failing store; Go via `failingStore`). Runtime parity for these paths is mandatory — if the two runtimes ever intentionally diverge, document it explicitly in both test suites and here.

v1.6 Fix Slice 1 (TS replace-import duplicate-binding parity): TypeScript `ForwardManager.importConfig` now rejects duplicate listen bindings within the imported set for BOTH replace and merge modes via a module-level `ensureNoDuplicateBindings(rules)` mirroring the Go `manager.ensureNoDuplicateBindings` (same message wording), run after validation and before any mutation — no in-memory mutation, no persist, no forwarder start/stop, one `config.import.failed` (severity error). The HTTP layer maps `result.errors` → 422 `{errors, result}` in both runtimes. Config **apply** was already protected (the plan engine's `detectDuplicateKeys` flags intra-desired duplicate bindings → `summary.hasErrors` → apply returns `ok:false` before calling `importConfig`), resolving the duplicate-binding half of Resilience-C. `validate:contract` 167 → 171 (duplicate-binding parity scenario, both modes × both runtimes). No valid-import or merge-vs-existing behavior change; no gate change. Source: `audits/v1.6-audit-synthesis-and-fix-plan.md`.

**Durable rule (Fix Slice 1):** Config import (replace AND merge) must enforce the same duplicate listen-binding rule in both runtimes. A listen binding is `protocol + listenHost + listenPort`; two rules in the imported set with that key conflict even if their IDs differ, and the whole import is rejected (422, no mutation, no persist). Import parity changes require unit coverage in BOTH runtimes (TS `forward-manager.test.ts`, Go `manager_test.go`) plus a `validate:contract` parity scenario. Keep the TS and Go duplicate-binding message wording aligned. The CLI stays a pure API client (no duplicate-binding logic).

v1.6 Fix Slice 2 (TS ConfigStore atomic write parity): TypeScript `ConfigStore.save` (`server/sources/config-store.ts`) now writes crash-safely — unique same-directory temp file (`.portier-forwards-<pid>-<ts>-<rand>.tmp`) → `fsync` via FileHandle → atomic `rename` over `rules.json`, mirroring Go `config.Store.Save`. The previous file stays intact until rename; the temp is removed on any pre-rename failure (best-effort, never masking the original error); the persistence error propagates so `ForwardManager` rollback (Test-D) still runs. `load` unchanged. No remove-and-retry recovery branch (Node `fs.rename` replaces atomically on POSIX `rename(2)` and Windows `MoveFileExW REPLACE_EXISTING`); directory fsync omitted (not portable on Windows — same limit Go accepts). A minimal test-only `ConfigStoreFileOps` seam (optional 2nd constructor arg, defaults to real fs) allows forcing write/sync/rename failures without a real disk-full event; not exposed through any product/API path. `config-store.ts` 100/100/100; gates unchanged. Source: `audits/v1.6-resilience-audit-1.md`.

**Durable rule (Fix Slice 2):** Config persistence must be crash-safe in BOTH runtimes. Rules-config writes use same-directory temp file + `fsync`/flush where practical + atomic `rename`; a failed save must not corrupt the previous `rules.json`, and the original persistence error must reach the caller so manager rollback can execute. New persistence durability behavior needs failure-path tests in the owning runtime (TS via the `ConfigStoreFileOps` seam, Go via a temp/rename seam). Do not add a broad filesystem abstraction; keep the seam minimal (mkdir/open-write-sync-close/rename/remove). Directory-fsync metadata durability on Windows is an accepted documented limit in both runtimes.

v1.7 Slice 1 (validate-contract.js outer child-cleanup guard): `scripts/validate-contract.js` now tracks every spawned runtime child in a module-level `activeChildren` set (added in `startServer`, removed on the child's `exit` event and in `cleanup`) and kills any survivors via a synchronous `killAllChildren()` wired to `process.on("exit")`, `SIGINT`/`SIGTERM` (kill then exit 130), and the outer `main().catch` (kill then exit 1). This is the deferred Resilience-F outer-guard that Fix Slice 4 left out. The primary ordered `cleanup(ts)`/`cleanup(go)` `try/finally` path is unchanged — the guard only adds the failure-path net (unexpected throw, early exit, never-ready startup, interrupt signal) and makes per-runtime cleanup idempotent. Script-only; `\x00` sort-key separators preserved; `validate:contract` 185/185 unchanged; no product/API/scenario/gate change. Source: `audits/v1.6-resilience-audit-1.md` (Resilience-F).

**Durable rule (v1.7 Slice 1):** Any script that spawns runtime child processes (`validate-contract.js`, and future `validate:*` runners) must terminate them on ALL exit paths, not only the happy-path `try/finally`. Track live children in a set and kill them from a synchronous guard wired to `process.on("exit")`, the termination signals (`SIGINT`/`SIGTERM`), and the top-level promise `.catch`. The ordered per-child `cleanup()` stays the primary path; the guard is the belt-and-suspenders net and must be idempotent. Do not regress this into a single un-guarded `try/finally`.

v1.7 Slice 3 (Go Forwarder interface + StartRule dedupe): the Go manager starts/stops/inspects rules through a small shared `forwarders.Forwarder` interface (`Start() error`, `Stop()`, `Status() domain.ForwardStatus`) built by the `forwarders.NewForwarder(rule, log, onEvent, tcpReg, udpReg)` factory (`service/sources/forwarders/forwarder.go`). `*TCPForwarder`/`*UDPForwarder` already satisfy it — the interface adds shape, not behavior, so `tcp.go`/`udp.go` were untouched. `manager.runtimeState` holds one `forwarder forwarders.Forwarder` (not separate `tcp`/`udp` pointers); `StartRule`/`stopRuntime`/`statusForRule`'s running branch are single-path. The protocol switch lives ONLY in `NewForwarder` (unknown protocol → nil → manager no-op). The not-running synthetic status in `statusForRule` keeps its per-protocol shape (that is API-response shape, not lifecycle). Behavior/contract unchanged; `validate:contract` 185/185; `NewForwarder` 100% covered; service 90.3→90.1 (gate 90 PASS). Source: `docs/roadmap.md` v1.7.

**Durable rule (v1.7 Slice 3):** Go rule lifecycle in the manager must go through the `forwarders.Forwarder` interface + `NewForwarder` factory — the manager must not reacquire per-protocol forwarder pointers or duplicate per-protocol start/stop construction. Protocol dispatch belongs in `NewForwarder` (one switch); a new forwarding protocol adds a case there and a type satisfying `Forwarder`, not another branch in `StartRule`/`stopRuntime`. Keep the interface minimal (start/stop/status) — do not grow it into a framework or push API-response-shape concerns into it. The TypeScript server is not required to mirror this internal structure (no cross-runtime parity obligation for it), but any new observable forwarding behavior still needs `validate:contract` parity.

v1.7 Slice 4 (TS UDP emit facade): the TypeScript UDP forwarder builds all activity events through a small module-private `UdpEventEmitter` facade (`server/sources/forwarders/udp-forwarder.ts`), constructed once from `rule` + `onEvent`. Methods: `packetError(message)` (no details), `packetForwarded`/`packetReturned`/`sessionOpened`/`sessionClosed`(message, details); one private `emit(type, severity, message, details?)` stamps `ruleId`/`ruleName`/`protocol:"udp"` and dispatches via the optional `onEvent`. Throttle timing, `status.lastError` mutation, and session/registry bookkeeping stay in the forwarder body. Payloads byte-identical (same fields/order, `details` omitted when absent). Facade is NOT exported (UDP-specific; TCP has its own events) — tested through the public forwarder, not by importing the class. 2 full-envelope regression tests added; `validate:contract` 185/185; server 98.9/94.0/100 → 98.9/94.1/100 (gates unchanged). Source: `docs/roadmap.md` v1.7.

**Durable rule (v1.7 Slice 4):** Repeated UDP (and similarly repetitive per-protocol) activity-event construction must go through a small co-located emit facade so the `{type, severity, ruleId, ruleName, protocol, message, details?}` envelope cannot drift between call sites — do not re-spell the full event literal at each emission. Keep such a facade module-private and close to its one forwarder unless it is genuinely reused (TCP and UDP have distinct event sets — do not force a shared cross-protocol emitter or an event bus/observer framework). The facade owns ONLY event construction + dispatch; throttle timing, `status`/`lastError` mutation, and session/registry bookkeeping stay in the forwarder. Emitted payloads must stay byte-identical (fields, order, `details` presence) and value changes follow the Fix Slice 5 activity-taxonomy rule. Prove facade output with full-envelope tests through the public forwarder; do not couple tests to the private facade name. This is a TS-internal structural choice — no Go parity obligation, but observable event changes still need `validate:contract`.

v1.7 Slice 5 (diagnose check-phase helper split): `diagnoseRule` in BOTH runtimes (`server/sources/diagnose.ts`, `service/sources/api/diagnose.go`) is reduced to an explicit ordered phase list of one-check helpers — `checkListenHost`/`checkLanExposure`/`checkPrivilegedPort`/`checkCommonPort`/`checkListenBind`/`checkTargetHost`/`checkTargetConnect`/`checkUdpMode` (UDP only) — each returning one `DiagnosticCheck` built verbatim. `targetResolved` derives from the `target-host` check's pass status (not a separate mutable flag). I/O helpers and `buildSummary` unchanged. Output byte-identical (IDs/labels/severities/order/messages/details); `validate:contract` 185/185. Added one ordering regression test per runtime (the prior find-by-id tests + contract ID-membership did NOT guard exact order). Source: `docs/roadmap.md` v1.7.

**Durable rule (v1.7 Slice 5):** The diagnose flow must stay a flat, explicit ordered list of small named per-check phase helpers in BOTH runtimes, kept structurally mirrored (split/extend in lockstep — they cannot drift). A new diagnostic check is a new `checkX` helper inserted at the right position in the `diagnoseRule` phase list in both `diagnose.ts` and `diagnose.go`, plus its `validate:contract` assertion and the per-runtime ordering test — NOT inline appended logic, and NOT a generic rule-engine/registry abstraction. Each phase helper returns exactly one `DiagnosticCheck` and must keep its ID/label/severity/message/details byte-identical across runtimes; check order is observable contract — keep the ordering regression tests (`diagnose.test.ts` "check ordering", Go `diagnose_phases_test.go`) and the contract ID assertion green. Do not derive `targetResolved` from anything other than the target-host check's pass status.

v1.7 Slice 6 (CLI config loader/mapper cleanup): the CLI config commands (`tools/cli/sources/commands/config.go`) share the read→parse→validate prelude via `loadConfigForCommand(filePath, verb, errExit, stderr) ([]rawConfigRule, int, bool)` (used by import/plan/diff/apply — import errExit 1, plan/diff/apply errExit 2; stderr wording unchanged) and the rule→DTO mapping via `toConfigRules([]rawConfigRule) []client.ConfigRule` (used by `buildPlanRequest`/`buildApplyRequest`/`buildImportRequest`). `RunConfigValidate` keeps its own JSON-aware parse-error flow (does not use the helper). Output formatters and `writePrettyJSON` already isolated. CLI behavior/exit codes/output byte-identical; CLI stays a pure API client; `validate:contract` 185/185; CLI coverage 97.7→97.9 (gate 95 PASS); all 5 helpers 100% covered by existing black-box tests. Source: `docs/roadmap.md` v1.7.

**Durable rule (v1.7 Slice 6):** CLI config commands must share their file load+parse+validate prelude (`loadConfigForCommand`) and their local-rule→`client.ConfigRule` mapping (`toConfigRules`, via the `build*Request` helpers) — do not re-inline `os.ReadFile`+`parseLocalConfig`+`validateLocalConfig` or the field-by-field DTO loop per command. New config commands reuse these helpers (pass the command's verb + error exit code). `RunConfigValidate` is the documented exception (JSON-aware parse-error output) — leave it bespoke unless a JSON-aware variant is added deliberately. Per-command exit-code semantics (import/plan/diff/apply local-input file errors → 2 — see Slice 7; plan errors → 1; drift+`--fail-on-drift` → 4; connection → 3) and exact stderr/stdout wording are observable CLI behavior and must stay byte-identical through any refactor — guard with the existing black-box `commands_test` behavior tests; do not add brittle private-helper tests. The CLI stays a pure API client (no server/service imports, no DTO changes).

v1.7 Slice 7 (CLI exit-code normalization review): audited all CLI command exit codes and fixed one clear inconsistency — `config import` local file read/parse/validation errors `1`→`2`, matching `config apply`/`plan`/`diff` (one-line change: `loadConfigForCommand(…, "import", 2, …)`). Documented exit-code policy in `tools/cli/readme.md`. Three import local-error tests tightened `!= 0`→`== 2`. No other command changed; `validate`'s `1`-for-invalid/unreadable is the documented validator-semantics exception. CLI behavior otherwise unchanged; `validate:contract` 185/185; coverage gate 95 PASS. Source: `docs/roadmap.md` v1.7.

**Durable rule (v1.7 Slice 7) — CLI exit-code policy:** `0` success; `1` API/runtime/operation error (server rejection/error, local output-**write** failure) and `validate`'s invalid/unreadable result; `2` invalid arguments/usage AND invalid local **input** config file (unreadable/malformed/validation failure) for `config import`/`plan`/`diff`/`apply`; `3` connection failure (service unreachable); `4` drift with `--fail-on-drift`. Intentional (documented, not to be "fixed"): local **input** errors are `2` but local **output**-write failures are `1`; `config validate` reports invalid/unreadable as `1` (its exit code is its result) while missing the path arg is `2`; a rule `<id|name>` matching nothing is `1` (target absent, like a 404) while an ambiguous name is `2` (fixable selector). New CLI commands MUST follow this policy; route API/connection errors through `exitWithError` (→ `1`/`3`); use `2` for bad local input and `1` for output/operation failures. Changes to a command's exit codes are observable behavior — update `tools/cli/readme.md` + the per-command help and add/adjust a black-box `commands_test` assertion in the same change.

v1.7 Slice 8 (SettingsView decomposition): `client/sources/features/settings/SettingsView.tsx` (was 736 lines) is now a ~90-line orchestrator composing per-panel components (`ExportConfigSection`/`PlanApplySection`/`ImportConfigSection`/`RuntimeEnvironmentSection`/`DiagnosticsExportSection`) over a shared `SettingsSection` wrapper, with logic in hooks (`useRuntimeInfo`/`useClipboardCopy`/`useConfigExport`). Each stateful panel owns its own state; the one cross-panel concern (config export, shared by the Export panel + the Import backup button via the `exporting` flag) is created once in `SettingsView` via `useConfigExport()` and passed to both. DOM byte-identical; JSX/classes/aria/onClick moved verbatim; `SettingsView.test.tsx` (63 tests) unchanged and green; client gates 94/89/78 unchanged; settings E2E 6/6. Source: `docs/roadmap.md` v1.7.

**Durable rule (v1.7 Slice 8) — settings/feature-view decomposition:** A feature view that has grown multiple stateful concerns must be a thin orchestrator composing focused panel components, NOT one giant component. Each panel owns its own state; lift state to the parent ONLY for a genuine cross-panel concern (e.g. the shared config-export `exporting` flag) and pass it down — do not prop-drill an entire view's state. Extract a hook (`useX`) when logic + state is cohesive and reusable/testable on its own (runtime fetch, clipboard, export flow); extract a small presentational primitive (`SettingsSection`) for repeated wrapper markup. Decomposition must be DOM-preserving: move JSX, class names, `aria-label`/`role`/`id`/`htmlFor`, and `onClick` expressions **verbatim** — no relabeling, no structure change, no accessibility regression. Because the tests render the composed top-level view and assert via role/label/text, a correct decomposition needs NO test changes; if a test must change to pass, the DOM changed — re-check. Keep new component files CamelCase and hooks camelCase under the feature folder. This is client-internal — no API/contract impact, but run the feature's E2E spec when it has one.

v1.7 Slice 9 (UI wording consistency pass): audited client UI copy against `docs/glossary.md`; fixed one glossary-backed inconsistency — the sidebar nav label for the Live Connections view was "Connections", now "Live Connections" (`client/sources/app/NavItem.ts`), matching the view title + glossary. Updated the `tests/e2e/connections.spec.ts` `goToConnections` helper (nav button name → "Live Connections"; arrival now confirmed via the unique `tablist`/"Connection views" role, since the title text is no longer unique). Rest of the UI already glossary-aligned (Listen/Target Host/Port; Running/Stopped/Error; TCP Connections/UDP Sessions; Active/Idle; Diagnose action vs Diagnostics Export bundle; Autostart label for `enabled`). "API Docs" (nav/header) vs "API Reference" (page heading) intentionally left (no glossary anchor; label-vs-heading is acceptable). No behavior/API/DTO change; `validate:contract` 185/185; client gates unchanged; full E2E 34/34. Source: `docs/roadmap.md` v1.7, `docs/glossary.md`.

**Durable rule (v1.7 Slice 9) — UI wording:** User-facing client labels (nav, titles, field labels, buttons, status, help text) must use `docs/glossary.md` canonical terms. A view's nav label should match its title/canonical term when the glossary anchors it (e.g. "Live Connections", not bare "Connections"); a short nav label MAY differ from a more descriptive page heading only when no glossary term applies and both are unambiguous. Keep observed vs desired state distinct (running/stopped/error vs enabled; "Autostart" = the UI label for `enabled`), TCP "connection" vs UDP "session", and "Diagnose" (per-rule action) vs "Diagnostics export" (bundle). Do NOT rename frozen public surfaces for cosmetic UI consistency (`/api/forwards`, `listenHost`/`targetHost`, `clientAddress`/`targetAddress`, the `connections` view *id*). A visible label change is observable behavior: update any test/E2E that selects by that text in the same change, and prefer role-based selectors when a renamed label becomes non-unique (e.g. nav button + view title sharing text → assert a unique `role`).

v1.8 Slice 1 (rule group metadata contract): added an optional `group` label to forward rules as the v1.8 Operator Power Tools data foundation — `group?: string` on the shared `ForwardRule`/`ConfigPlanRuleSnapshot` (`shared/sources/index.ts`, `plan.ts`), `Group *string json:"group,omitempty"` on Go `domain.ForwardRule`/`ForwardRuleResponse`/`configplan.RuleSnapshot`, and `group` on the CLI `ConfigRule`/`ConfigPlanRuleSnapshot`/`ForwardRuleResponse` DTOs (all `omitempty`). Validation is identical and parity-tested in both runtimes (`validateGroup` in `@portier/shared`; `validateGroup`/`normalizeGroup` in `service/sources/validation`): optional; trimmed; empty/whitespace → absent; ≤ 64 chars (`PORTIER_GROUP_MAX_LENGTH`/`domain.GroupMaxLength`); no control chars (C0 + DEL); non-string rejected. PATCH: `""` clears, non-empty sets, absent/`null` unchanged (Go can't distinguish JSON `null` from absent, so TS matches it). `group` is a plan **material** field (group-only change = `update`) but **not** a forwarding field (non-destructive, no restart) — added to `MATERIAL_FIELDS` (TS) and the Go `diffMaterialFields` group branch, kept OUT of `FORWARDING_FIELDS`/`forwardingFields`. Preserved through export/import and plan/apply (snapshot builders + `buildApplyImportFromPlan`/`BuildApplyImportFromPlan` + `toRuleResponse`). Runtime forwarding/lifecycle/duplicate-binding/status unchanged. `validate:contract` **185 → 204** (self-contained `rule-group` scenario group + `config import→export` roundtrip + cross-runtime `plan:GROUP_CHANGE` parity; CLI DTO guard green). No UI group editing yet, no CLI group commands/filtering yet (deferred). No coverage gate changed. Source: `docs/roadmap.md` v1.8, `docs/api-contract.md`, `docs/glossary.md`.

**Durable rule (v1.8 Slice 1) — rule group metadata:** Rule `group` is **optional, behavior-neutral metadata** until group operations are introduced — it must NOT affect forwarding, lifecycle, start/stop/reorder, duplicate-binding (still `protocol + listenHost + listenPort` only), or status. Config import/export/plan/apply must **preserve** `group` in BOTH runtimes; an ungrouped/legacy rule omits the field entirely and must produce no drift. `group` validation (optional, trimmed, empty→absent, ≤ 64 chars, no control chars, non-string rejected, same error wording) must stay **parity-tested** across TypeScript (`@portier/shared`) and Go (`service/sources/validation`) — add cases in both plus a `validate:contract` scenario when the rule changes. In a plan, `group` stays a material but non-destructive field (in `MATERIAL_FIELDS`, never in `FORWARDING_FIELDS`). PATCH semantics are frozen: `""`/whitespace clears, non-empty sets, absent/`null` leaves unchanged. The CLI stays a pure API client (forward `group`, light local pre-validation, no group-specific logic). Do NOT widen this into tags/profiles/multi-group/colors or rename the frozen `group` field for cosmetic reasons.

v1.8 Slice 2 (rule group UI editing and display): surfaced `group` in the web UI — client-only, no shared/server/service/CLI code touched, `validate:contract` 204/204. Rule form (`client/sources/features/forwards/RuleForm.ts` + `ForwardRuleForm.tsx`): optional **Group** input under Name; `group: string` added to `RuleFormState`/`emptyForm`/`ruleToForm`/`formToPayload`; `formToPayload` always sends raw `group` so the Slice 1 PATCH contract handles set (non-empty) / clear (empty → `""`) / create-normalize; `maxLength={PORTIER_GROUP_MAX_LENGTH}`; explicit `aria-label="Group"` because the always-present hint `<span>` inside the `<label>` would otherwise pollute the accessible name (same fix the Listen Host field uses). Client-side `validateForwardRule` already rejects over-long/control-char groups on submit (server stays authoritative). Rule list (`ForwardRuleList.tsx`): subtle `.rule-group-label` chip beneath the name when present (no placeholder when absent), and the search box now matches `group`. Tests: 5 form + 3 list unit tests, 1 E2E (`tests/e2e/portier.spec.ts`: create-with-group → chip + API, edit-clear → gone + API). Client gates 94/89/78 unchanged. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 2) — rule group UI:** When surfacing an optional metadata field in the rule form, drive it through `RuleForm.ts` (`RuleFormState` + `emptyForm`/`ruleToForm`/`formToPayload`), not ad-hoc component state, and **always send the raw value** in `formToPayload` so the Slice 1 PATCH contract does the work (empty string clears on edit / normalizes to absent on create; non-empty sets) — do not special-case "clear" in the client. A form input whose `<label>` also contains a persistent hint/help `<span>` MUST carry an explicit `aria-label` (the label's full text content, hint included, otherwise becomes the accessible name and breaks `getByRole("textbox", { name })` and screen-reader output) — mirror the Listen Host field. Display group as a subtle, secondary chip (`.rule-group-label`) that does not overpower status/protocol/endpoints, render nothing when absent (no empty placeholder), and keep server-side validation authoritative (the client `maxLength`/`validateForwardRule` checks are convenience only). Reuse the existing rule search for group matching; do NOT add group filtering/grouping/bulk UI in this slice. This is client-internal — no API/contract/coverage-gate change.

v1.8 Slice 3 (group filtering and grouping groundwork): added a **Filter by group** `<select>` to the Forward Rules list (`ForwardRuleList.tsx`) — client-only, read-only navigation, `validate:contract` 204/204. Options derived from the current rules (distinct trimmed group names, `localeCompare` `sensitivity:"base"` sort) + `All Groups` + `Ungrouped` (only when ungrouped rules exist); the control renders only when ≥1 rule has a group. Sentinels `ALL_GROUPS`/`UNGROUPED` (wrapped `__…__` tokens, cannot collide with real groups). Filter applies in the same `filteredRules` pass as search+status (AND-combine); a derived `activeGroupFilter` clamps a stale selection back to All Groups (no out-of-range `<select>` value, never hides all rules from a deleted group); drag-reorder disabled while a group filter is active. Empty states unchanged ("No rules match the current filter."). Tests: 8 new list unit tests + 1 E2E; one Slice 2 chip assertion scoped to the table since the group name now also appears as an `<option>`. Client gates unchanged. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 3) — rule list group filter:** Group filtering is **read-only client navigation** — it must not mutate rules or touch the API/runtime. Derive group options from the live `rules` prop (distinct, trimmed, locale-insensitive sorted); never persist them separately. Apply the group filter in the same single `filteredRules` predicate as the existing search/status filters so all three AND-combine; keep the existing empty-state messages. Always **clamp a stale specific-group selection** to "all" via a derived value (a controlled `<select>` must never hold a value with no matching `<option>`, and a vanished group must not silently filter out every rule). Offer "Ungrouped" only when an ungrouped rule exists, and render the whole control only when at least one group exists (no dead single-option control). Use sentinel tokens that cannot collide with a real (non-empty, trimmed) group for the non-group selections. Keep it a native labelled `<select>` (keyboard-accessible, `aria-label`); do not add visual grouping headers, group start/stop, bulk actions, colours, or filter persistence in this slice. When a list value (like a group) now appears in BOTH a row chip and a filter option, scope chip assertions to the table (e.g. `.rule-group-label` / `within(table)`) so they stay unambiguous. Client-internal — no API/contract/gate change.

v1.8 Slice 4 (group operations API): added `POST /api/forwards/groups/:group/{start,stop}` in BOTH runtimes — start/stop all rules sharing a `group`. Behaviour over existing metadata: never mutates rule definitions/order/`enabled`/`group`/duplicate-binding; per-rule lifecycle + activity events identical to single-rule start/stop (no new event type). `ForwardManager.startGroup`/`stopGroup` (TS) + `Manager.StartGroup`/`StopGroup` (Go) iterate the matched rules in **rule order**; start ignores `enabled` (matches single-rule start), skips already-running (`already_running`), partial-fails one rule without aborting the rest (`failed` + reason); stop skips not-running (`not_running`). No match → **404**; invalid group → **400** (group-operation targets must be non-empty: `validateGroupName` in `@portier/shared`, `validation.ValidateGroupName` in Go, both add `group is required.`). Response `GroupActionResponse {group,action,total,succeeded,skipped,failed,results:[{ruleId,ruleName,status,reason?}]}` — shared `groups.ts` + `summarizeGroupAction`, mirrored by Go `domain.GroupActionResponse` + `buildGroupActionResponse`. Route nests under `/api/forwards/groups/:group/...` (4 segments, no collision with `:id/start`); Go parses the escaped path so an encoded `/` in a group stays one segment. Client `setGroupRunning` added as groundwork (NO UI wiring); `ApiDocsView` documents both endpoints. `validate:contract` **204 → 225** (`group-action` group + `group:start` parity). No UI/CLI group commands. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 4) — group operations:** Group start/stop are **behaviour over existing rule metadata, NOT config mutation** — they must never change rule definitions, order, `enabled`/autostart, `group`, or duplicate-binding, and must reuse the existing single-rule start/stop path (same per-rule activity events; do NOT invent a group-level event type unless the activity model gains one deliberately in both runtimes). Result ordering MUST follow the manager's **rule order** (deterministic); the summary is `{group, action, total, succeeded(=started/stopped), skipped, failed, results[]}` with per-rule `status` + optional `reason` (`already_running`/`not_running` skip tokens, or the error message for `failed`). Start ignores `enabled` (parity with single-rule start); a single rule's start failure is a `failed` result, never an aborted operation (partial success). An empty match is a **404**, not a silent success. Group-operation path targets must be validated as **non-empty** group labels (`validateGroupName`/`ValidateGroupName`, message `group is required.`) in addition to the normal length/control-char rules. TS and Go group-operation behaviour + response shape + error behaviour MUST stay contract-tested (`validate:contract` `group-action` group and `group:start` parity); add scenarios in lockstep when behaviour changes. Keep the shared `summarizeGroupAction` and Go `buildGroupActionResponse` counting mirrored. The CLI stays a pure API client (no group commands/DTOs this slice); the client `setGroupRunning` method is groundwork only — no UI wiring until a later slice. Ungrouped bulk actions are intentionally deferred.

v1.8 Slice 5 (CLI group commands): exposed the Slice 4 group API through the Go CLI — `portier group list|start|stop <group>` (singular `group`, matching `config`). CLI-only, pure API client, `validate:contract` 225/225. `group list` derives groups from `GET /api/forwards` + `GET /api/status` (alphabetical, rule+running counts; `No rule groups configured.` exit 0 when none); `group start`/`stop` call `POST /api/forwards/groups/:group/{start,stop}` and print a summary + `RULE/STATUS/REASON` table (`--json` = full `GroupActionResponse`). Exit codes (v1.7 Slice 7 policy): 0 success incl. skips, 1 API error / no-match 404 / `failed>0`, 2 missing/invalid group, 3 connection. Group arg trimmed + locally validated (non-empty/≤64/no-control via `validateGroupArg` reusing `groupMaxLength`/`hasControlChar`), server authoritative. New `client.StartGroup`/`StopGroup` + `GroupActionResponse`/`GroupActionResult` DTOs (`url.PathEscape` for the group segment); `group` wired into `main.go` + root help. Arch-D guard extended: CLI fixture capture starts a dedicated grouped rule → `group-start.json` strictly decoded into the CLI DTO (main capture rule stays ungrouped so plan/apply stay drift-free). Black-box `commands_test` for all paths; `main_test` dispatch table + `group`. CLI gate 95% holds. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 5) — CLI group commands:** The CLI group commands are a **pure API client** over the Slice 4 endpoints — they MUST NOT reimplement group semantics, mutate rules, or talk to anything but `GET /api/forwards`, `GET /api/status`, and `POST /api/forwards/groups/:group/{start,stop}`. `group list` is derived client-side from the live rules+status (alphabetical, distinct non-empty trimmed groups, rule+running counts) — never persisted; an empty result prints a clear message and exits **0**. Exit codes follow the v1.7 Slice 7 policy and the Slice 4 semantics: a **skip** (`already_running`/`not_running`) is success (**0**); `failed>0` OR the API's no-match **404** is **1**; a missing/invalid local group argument is **2** (validate non-empty/≤`groupMaxLength`/no-control locally with `validateGroupArg`, but let the server stay authoritative — do not duplicate server-only checks); unreachable is **3** (via `exitWithError`). `--json` must emit the full `GroupActionResponse` for start/stop and `{groups:[{group,total,running}]}` for list. New CLI DTOs are a contract copy — keep them guarded by the Arch-D `TestCLIDTOContractParity` (capture a live `group-start.json`; keep the main CLI-contract capture rule **ungrouped** so plan/apply fixtures stay drift-free). Prefer black-box `commands_test` over private-helper tests. Do not add group rename/delete, ungrouped bulk, profiles, tags, or colours.

v1.8 Slice 6 (UI group action buttons): exposed group start/stop in the web UI — client-only, `validate:contract` 225/225. A compact toolbar (`Start group "<name>"` / `Stop group "<name>"`) appears under the rule-list header (`ForwardRuleList.tsx`) only when **Filter by group** is set to one concrete group (hidden for All Groups + Ungrouped, and only when an `onGroupAction` prop is supplied). Click → `App.handleGroupAction` → `setGroupRunning` (`POST /api/forwards/groups/:group/{start,stop}`) → `refreshAll`; per-action loading (`Starting…`/`Stopping…`, both buttons disabled = no double-submit); inline summary `Started group "web": N succeeded, N skipped (N total)` (`role="status"`), warning when `failed>0` (`role="alert"`), server error message on rejection/404 (`role="alert"`). Result cleared when the group selection changes. 11 `ForwardRuleList` + 2 `App` unit tests + 1 E2E. Client gates unchanged. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 6) — UI group actions:** Operator bulk-by-group actions in the web UI are a thin client over the Slice 4 endpoints (via `setGroupRunning`) — no reimplemented group semantics, no rule mutation, no API/DTO change. Only surface group start/stop for **one concrete group** (the active `Filter by group` selection) — never for All Groups or Ungrouped (there is no ungrouped bulk endpoint). After a successful action, **refresh rule/status data through the existing `refreshAll`/`onRefresh` pattern** (do not hand-mutate local state). Show a **scoped inline result** near the buttons (`role="status"` for success/skip, `role="alert"` for `failed>0` or an error) using existing error conventions — do NOT add a new global notification system; clear the result when the group selection changes. Prevent double-submit by disabling both buttons while a request is in flight. Group-action buttons must carry the **group-scoped accessible name** (`Start group "web"`) so they stay unambiguous. Keep the rule list otherwise unchanged (no redesign). The component keeps the action behind an optional `onGroupAction` prop so non-action usages render no toolbar; the API call + refresh live in the parent (`App`), the local busy/result UI lives in the list. No profiles/tags/colors/rename/management screen; this is client-internal — no API/contract/coverage-gate change.

v1.8 Slice 7 (rule health basics): added an operator-facing `health` (`healthy`/`warning`/`error`) on `ForwardStatus`, derived deterministically from existing runtime state — `validate:contract` 225 → 234 (`status:health` parity). Shared `RuleHealth` + required `health` on `ForwardStatus` + `deriveRuleHealth({enabled,running,lastError})` (`@portier/shared`); Go `domain.RuleHealth` + `DeriveRuleHealth` + `Health` on `domain.ForwardStatus`. The **manager** is the single place that derives health (it owns `enabled`): TS `ForwardManager.getStatus` (forwarders now return a health-less `ForwarderStatus` = `Omit<ForwardStatus,"health">`), Go `Manager.statusForRule`. Priority: `error` if `lastError` present; else `warning` if `enabled && !running`; else `healthy`. UI: `RuleHealthBadge` dot (green/amber/red, `role="img"` + label/tooltip) — since v1.8 Slice 9 in its **own `Health` column** on the rule list (dot + one-word `healthShortLabel`, `aria-hidden`), distinct from the unchanged Status column. CLI: `portier status` HEALTH column + `health` in the DTO/`--json`. ~30 existing `ForwardStatus` fixtures gained `health`. No probing, no background check, no rule mutation; `lastError` lifecycle unchanged. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 7) — rule health:** Rule `health` is a **derived, deterministic interpretation** of existing runtime state (`enabled`/`running`/`lastError`) — it must NEVER imply active target probing or background monitoring unless such probing is explicitly implemented (this slice implements none). Keep `health` and `status`/`running` **distinct**: status is lifecycle, health is the operator reading; do not collapse them or replace the status badge with health. The single derivation helper is `deriveRuleHealth` (`@portier/shared`) / `domain.DeriveRuleHealth` (Go) — keep the two byte-identical in logic and **parity-tested** (`validate:contract` `status:health` + the `rule-health` scenario); the priority is fixed (`error` > `warning` > `healthy`). Health is computed by the **manager** (which has the rule's `enabled`), not by forwarders — TS forwarders return `ForwarderStatus` (`Omit<ForwardStatus,"health">`) and the manager adds `health`; Go forwarders leave `Health` zero and `statusForRule` sets it. Do not move health derivation into the forwarders, the API handlers, or the client. The client/CLI must **read** `health` from the status response, never recompute it (one source of truth). Health adds no network I/O and never mutates a rule. Do not add health history, a metrics endpoint, target reachability polling, auto-restart, or retry in this slice.

v1.8 Slice 8 (duplicate rule action): client-only duplicate-rule convenience — `validate:contract` 234/234 unchanged (no shared/server/service/CLI/API/DTO/config change, no backend endpoint). Each rule row has a **Duplicate** action (`Copy` icon, accessible name `Duplicate rule <name>`) that opens the existing drawer in **create mode** pre-filled from the source. `ForwardRuleForm` gained an optional `duplicateSource` prop (used only when not editing; edit wins), title "Duplicate Rule" + "New rule copied from …" subtitle, drawer aria-label "Duplicate Forward Rule"; `ForwardRuleList` gained optional `onDuplicate` (button renders only when provided); `App` tracks `duplicateSourceId`, derives the source rule, clears it on every form open/close path, and keys the drawer `dup-<id>`. Prefill via `ruleToDuplicateForm` (`RuleForm.ts`): copies editable fields (protocol, listen/target host+port, udpMode, **group**), name → `<name> copy` (`duplicateName`), **drops id** (saves via create `POST`, not PATCH) and **forces `enabled:false`** (a created enabled rule auto-starts via `addRule`→`startRule`; a duplicate must not auto-start). Runtime-only state (status/`lastError`/health/active counts) is inherently excluded (never in the form). Validation unchanged — duplicate-binding caught by the normal create path if the user doesn't change listen host/port/protocol. Four pre-existing E2E selectors that matched action buttons by loose substring name (`{name:"Edit"}` ⊂ "Duplicate rule E2E Edit Original") were tightened to `exact:true`. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 8) — duplicate rule:** Duplicate-rule is a **client-only create-flow convenience** — it must NOT add a backend duplicate endpoint, mutate the source rule, or change any API/DTO/config shape (the duplicate saves through the existing create `POST /api/forwards`, `id` undefined). A duplicate prefill (`ruleToDuplicateForm`) copies only the editable definition fields **including `group`**, sets the name to `<name> copy`, **drops the id**, and **forces autostart (`enabled`) off** — never copy runtime-only state (id/status/`lastError`/health/active connections/sessions), and never let a duplicate auto-start on save (a created `enabled` rule starts immediately). The duplicate must NOT bypass or special-case validation: it passes the same client + server validation as any new rule, so an unchanged listen binding surfaces the normal duplicate-binding error — do not pre-resolve or auto-increment ports. Open it through the existing rule drawer in **create mode** (distinct title/subtitle so it's clearly a new rule, not an edit) via the optional `duplicateSource` prop, ignored when `editingRule` is set; keep the row action behind an optional `onDuplicate` prop and give it the **rule-scoped** accessible name `Duplicate rule <name>`. When a new always-present row action's accessible name embeds the rule name, audit existing tests/E2E that select sibling action buttons by **loose** substring name (`{name:"Edit"}` etc.) and make them `exact:true` — a rule named after an action word will otherwise collide. No rule templates/saved templates/profile cloning/group-level duplicate/management screen. This is client-internal — no contract/coverage-gate change.

v1.8 Slice 9 (config preview UX polish): client-only polish of the Settings **Plan & Apply** preview (`PlanApplySection.tsx` + new helpers in `planHelpers.ts` + scoped CSS) — `validate:contract` 234/234, no API/DTO/config or plan/apply semantics change. Summary gained a labelled `Destructive: N` count. Each operation renders as a card: action badge (Add/Update/Remove/Unchanged), rule name + protocol, a plain-language impact note (`describeOperationImpact`), and text tags `Destructive` (from `op.destructive`) / `Metadata only` (from `isMetadataOnlyUpdate`). Each change row shows a friendly field label (`formatFieldLabel`), a `forwarding`/`metadata` tag (`changeImpact`), and `before → after`. Impact for updates is derived from the **actual changed fields** via a client-side mirror of the server's `FORWARDING_FIELDS` (`protocol`/`listenHost`/`listenPort`/`targetHost`/`targetPort`/`udpMode`) — a group/name/autostart-only edit reads "Metadata only — the forwarder is not restarted" and never implies a restart. Server stays authoritative for the `destructive` Apply-gating; no-drift/error/warning/confirmation/apply states unchanged. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 9) — config preview UI:** The config plan/apply preview is a **read-only presentation of the existing plan response** — it must NOT change plan/apply semantics, the `destructive` gating, or any API/DTO/config shape, and must keep the server authoritative for `destructive` (the Apply button still gates on `summary.destructive` + the confirmation checkbox). Destructive and metadata states must be conveyed by **text** (badges/tags + a plain-language impact note), never colour alone (accessibility). A group-only (or any non-forwarding) update must be shown as **metadata-only** and must **not imply a socket restart** — derive an update's restart/impact from the **actual changed fields**, using a client-side mirror of the server's `FORWARDING_FIELDS` set (`protocol`/`listenHost`/`listenPort`/`targetHost`/`targetPort`/`udpMode`); if that set changes server-side, update the `planHelpers.ts` copy in the same spirit (it is display-only — do not import server internals, and do not move classification into shared/server for this). Keep the preview's pure logic in `planHelpers.ts` (`formatFieldLabel`/`changeImpact`/`isMetadataOnlyUpdate`/`describeOperationImpact`) with unit tests, and keep `PlanApplySection` rendering thin. Preserve the existing no-drift/in-sync, plan-error (apply disabled), warning, destructive-confirmation, and apply success/failure/`ok:false` states and their `role="alert"`/`role="status"` semantics. No full redesign, no diff-viewer, no new config workflow. Client-internal — no contract/coverage-gate change.

**Durable rule (v1.8) — Forward Rules row action menu:** The Forward Rules row keeps **only Start/Stop as an inline icon button**; the remaining per-row actions — **Edit, Duplicate, Diagnose, Activity, Delete** (in that order, Edit first) — live in a per-row **overflow ("kebab", `MoreVertical`) menu** (`ForwardRuleList.tsx`) opened by a `More actions for <rule name>` trigger (`aria-haspopup="menu"` + `aria-expanded`). The trigger is **seamless — styled like the reorder drag handle** (`.row-menu-trigger`: no button border/background, muted glyph brightening on hover / `aria-expanded`), NOT a boxed `btn-icon`. The dropdown is `role="menu"` (labelled `Actions for <rule name>`) of `role="menuitem"` buttons; **Delete keeps the red `row-menu-item--danger` styling** and still triggers the existing inline Confirm/Cancel delete prompt (do not replace that confirm flow with a bare menu click). Only one row menu is open at a time; it closes on **outside click and Escape** (a single document listener gated on the open id); the trigger is disabled while the row is busy or mid delete-confirm. When adding or moving a row action: keep Start/Stop inline, give menu items plain text names (`Edit`/`Duplicate`/`Diagnose`/`Activity`/`Delete`) with their icon, and remember that **tests must open the menu first** and target `role="menuitem"` (not a row button) — both unit (`openRowMenu` helper) and E2E (click the `More actions for <name>` trigger, then the `menuitem`). This is client-internal — no API/contract/coverage-gate change.

Coverage policy: require 100% meaningful coverage for all newly added or materially changed files in v1.5 and v1.6. Existing baselines ratcheted incrementally. Do not block unrelated work on legacy uncovered areas. Do not lower gates without explicit rationale.

E2E install (one-time): `npm run test:e2e:install`

Do not add `test:e2e` to `npm run test` or `npm run check`. E2E is a separate step.

E2E spec files:
- `tests/e2e/portier.spec.ts` — app load, CRUD, start/stop, merge import, form validation, diagnose
- `tests/e2e/settings.spec.ts` — replace-mode import (v1-mixed), invalid-JSON rejection, export shape, runtime info, plan preview, plan apply
- `tests/e2e/connections.spec.ts` — Live Connections view: title/tabs, empty states, tab switching, filters, auto-refresh, footer counts, rule filter
- `tests/e2e/activity.spec.ts` — activity log view opens and shows events
- `tests/e2e/apidocs.spec.ts` — endpoint list renders, GET /api/connections listed
- `tests/e2e/dashboard.spec.ts` — dashboard stat cards render
- `tests/e2e/mobile.spec.ts` — mobile hamburger / sidebar toggle
- `tests/e2e/tcp.spec.ts` — TCP real forwarding
- `tests/e2e/udp.spec.ts` — UDP one-way, last-client, multi-client, activity assertions

Settings E2E intentionally does not run the full fixture matrix. `validate:config` owns exhaustive TS/Go parity testing.

**Additional validation suites (run explicitly — slower/platform-sensitive, not part of `npm run check`):**

```powershell
npm run validate:config            # fixture-based rules.json compatibility validation
npm run validate:contract          # API contract parity: TypeScript server + Go service if available
npm run validate:binary            # runtime binary behavior: build:runtime then 5 behavioral tests
npm run validate:runtime:behavior  # alias for validate:binary (fits validate:runtime:* namespace)
npm run validate:scripts           # installer script static analysis + dry-run on current platform
```

- `validate:config`: loads fixtures from `tests/fixtures/config/`; checks load, import, export, rejection, duplicate bindings, UDP defaults; TypeScript always checked, Go checked when binary present; `--skip-go` to force skip. Uses temp dirs and free ports; never reads real `rules.json`.
- `validate:contract`: skips Go parity clearly if binary not present; `--skip-go` to force skip. Verifies status codes, response shapes, field names, error shapes, advisory/plan content parity (Arch-A: canonical message text per runtime plus a cross-runtime `compareParity` field-by-field diff), **and** (since Arch-D) CLI DTO parity — captures live runtime JSON and strictly decodes it into the CLI DTOs via `tools/cli`'s `TestCLIDTOContractParity` (skips if `go` toolchain absent). Uses the binary at `build/portier/service[.exe]` if present (else `service/build/portier-service[.exe]`); rebuild it after changing Go advisory/plan code or the guard tests stale output.
- `validate:binary`: runs `build:runtime` first; use `--no-build` to reuse existing `build/portier/`.
- `validate:scripts`: always runs static analysis; dynamic dry-run only on current platform.

Do not add these to `npm run test` or `npm run check`.

Go test binary path (Windows Firewall stability): both `test:service` (`node scripts/coverage-service.js --test-only`) and `coverage:service` (`node scripts/coverage-service.js`) are served by the single `scripts/coverage-service.js` — it **compiles** the Go test binaries to a stable, gitignored path (`build/tests/<pkg>.test.exe`) with `go test -c` and then **runs them from there** (`--test-only` skips coverage instrumentation/merge). Some service tests genuinely bind `0.0.0.0` (e.g. `TestDiagnoseLANExposureWarns`, which exercises the real diagnose listen-bind on a `0.0.0.0` rule), which triggers a Windows Firewall prompt; the firewall rule is keyed by executable path, so a fixed path lets one "allow" decision persist instead of re-prompting every run. **The old `go test -o build/tests/ ./...` form did NOT achieve this and was removed** — `-o` only *copies* the linked binary afterward; `go test` still ran the socket-binding binary from a fresh `…\Temp\go-build<random>\b001\<pkg>.test.exe` path every invocation, so every run re-prompted and accumulated another firewall rule. `go test -c` compiles without running, so executing the fixed-path binary ourselves is what makes the path stable. Do not regress `test:service`/`coverage:service` back to a plain `go test [-o] ./...` run, and do not change the diagnose test to loopback (the `0.0.0.0` bind is faithful product behavior, not a test artifact). Because `covdata` only sees packages linked into a test binary, `coverage:service` re-adds 0%-coverage blocks for test-less, un-imported packages (`sources/logger`, the `sources` main package, `sources/platform`) so the cross-package denominator — and the calibrated 90% gate — stays comparable to the old single-profile run.

Naming convention:
- `npm run test` = unit/integration test runner (Vitest + Go test)
- `npm run test:e2e` = Playwright browser E2E tests
- `npm run validate:config` = fixture-based rules.json compatibility validation
- `npm run validate:contract` = TS/Go API parity validation
- `npm run validate:binary` / `validate:runtime:behavior` = packaged service binary behavior validation
- `npm run validate:scripts` = installer/service script static + dry-run validation

**Protocol coverage (automated E2E — do not revert to manual QA):**
- TCP real forwarding: `tests/e2e/tcp.spec.ts`
- UDP one-way, bidirectional-last-client, bidirectional-multi-client: `tests/e2e/udp.spec.ts`

**Remaining manual QA:** Firewall and OS permission behavior only.

**OS service install validation is automated — run explicitly before release:**

```powershell
npm run validate:service:windows:user     # Windows scheduled task (no admin)
npm run validate:service:windows:machine  # Windows Service (admin)
npm run validate:service:current          # current platform
```

```bash
npm run validate:service:macos   # macOS LaunchAgent (no sudo)
npm run validate:service:linux   # Linux systemd (sudo)
```

These use test-specific names and temp dirs. Never touch production installs.
Do not add these to `npm run check`. Run them explicitly when releasing.

**Package build is automated — do not treat it as manual QA:**

```powershell
npm run validate:runtime           # validate existing build/portier/ layout
npm run validate:runtime:build     # build then validate
npm run validate:runtime:smoke     # build, validate, and run smoke test (preferred)
```

**macOS release archive — run explicitly when changing macOS scripts:**

```bash
npm run build:release:portable   # build:runtime then portable tar.gz (on macOS)
```

Output: `build/releases/macos/portier-portable-macos-<version>.tar.gz`. Requires `bash` and `tar`. Do not add to `npm run check` or any automated validation chain.

**Linux release archive — run explicitly when changing Linux scripts:**

```bash
npm run build:release:portable   # build:runtime then portable tar.gz (on Linux)
```

Output: `build/releases/linux/portier-<version>-linux.tar.gz`. Requires `bash` and `tar`. Do not add to `npm run check` or any automated validation chain.

**Windows release artifacts — run explicitly when changing release files:**

```powershell
npm run build:release:current    # portable zip + Inno Setup installer (installer non-fatal if absent)
npm run build:release:portable   # portable zip only
```

Output: `build/releases/windows/portier-<version>-windows-portable.zip` and `Portier-Setup-<version>.exe`. If Inno Setup is unavailable, the portable zip is still produced — report the missing installer clearly. Do not add to `npm run check` or any automated validation chain.

If a task touches packaging, run `npm run validate:runtime:smoke` when possible. The script:
- Builds `build/portier/` via `build:runtime` on the current platform
- Validates the layout (`service`/`service.exe`, `server.js`, `web/`, `readme.txt`)
- Smoke-tests the packaged binary on a free port without requiring admin/root

The platform-specific scripts still output to their own dirs when called directly:

```powershell
npm run build:runtime:windows     # Windows: produces build/windows/
npm run build:runtime:macos       # macOS/cross-compile: produces build/macos/
npm run build:runtime:linux       # Linux/cross-compile: produces build/linux/
```

If packaging cannot run because prerequisites are unavailable (e.g., Go is not installed), document that limitation clearly.

If a task touches formatting, run `npm run format` if available. Otherwise use `npm run lint` or the Prettier commands defined in `package.json`.

## UI Cleanup Guidance

- Keep the interface simple and practical.
- Prioritize clarity over visual flair.
- Make rule status obvious: running, stopped, error.
- Make protocol obvious: TCP or UDP.
- Make listen endpoint and target endpoint readable.
- Show LAN exposure warning for `0.0.0.0`.
- Show common port warnings inline.
- Keep add/edit rule flows compact.
- Do not hide dangerous states behind subtle styling.
- Avoid complex UI frameworks unless already present.
- Prefer accessible HTML controls and clear labels.

## Packaged Runtime Layout

Production/install layout for all platforms:

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

- `portier` / `portier.exe` = CLI binary (talks to management API; does not start the service)
- `service` / `service.exe` = native Go runtime; default static dir is `web`
- `server.js` = bundled Node/TypeScript fallback
- `web/` = built React client UI (external; not baked into binary)
- `rules.json` = always external; never packaged

Dev build output (repo-internal, not distributed): `service/build/portier-service`, `server/build/`, `client/build/`, `tools/cli/build/portier-cli`.

Packaging scripts:
- `build:runtime` → `build/portier/` (cross-platform, primary generic output; builds CLI + service)
- `build:runtime:windows` → `build/windows/`, `build:runtime:macos` → `build/macos/`, `build:runtime:linux` → `build/linux/`
- `build:clean` removes `build/portier/`, all platform package output dirs, `build/releases/`, and `tools/cli/build/`

Validation scripts:
- `validate:runtime` → validates `build/portier/` layout
- `validate:runtime:build` → builds then validates
- `validate:runtime:smoke` → builds, validates, and smoke-tests the packaged binary
- `validate:service:windows:user` → Windows scheduled task install/start/stop/uninstall (no admin)
- `validate:service:windows:machine` → Windows Service install/start/stop/uninstall (admin required)
- `validate:service:macos` → macOS LaunchAgent install/start/stop/uninstall (no sudo)
- `validate:service:linux` → Linux systemd install/start/stop/uninstall (root/sudo)
- `validate:service:current` → runs the appropriate platform script

## Remaining Work

Both runtimes are feature-complete. Package build correctness, OS service install/uninstall flows, and release artifact generation are automated. Remaining manual work is firewall and OS permission behavior. See `docs/checklist.md`.

v1.1 is complete: distribution, installers, release artifacts, service and package validation, platform polish. Tagged 1.1.0. See `docs/installer-strategy.md` for scope and slice history.

v1.2 is complete: runtime info endpoint, rule diagnostics API and UI, Activity Log polish, safer networking UX, settings/config polish, and diagnostics export. Tagged 1.2.0. See `docs/roadmap.md` for goals, slices, and non-goals.

v1.3 is complete: Go CLI under `tools/cli/`. All 8 slices done: `tools/cli/` module, HTTP API client (`ConnectionError`/`APIError`), `--url`/`--host`/`--port`/`PORTIER_URL` connection options, `--json` flag, `runtime`/`list`/`status`/`activity` commands (activity supports `--limit`/`--rule`/`--type`/`--severity`), `start`/`stop`/`diagnose` lifecycle and diagnostics commands (accept exact rule ID or unique name; duplicate names → exit 2 with ID disambiguation), `config validate`/`config export`/`config import` commands (local validation before API, replace requires `--yes`), `diagnostics export --out <file>` (builds JSON support bundle; `--run-diagnostics`; `--activity-limit` 1–500; partial-failure tolerant with `errors[]`), output helpers (`FormatBool`/`FormatBytes`/`FormatTimestamp`/`PrintTable`), safe rule resolver (`ResolveRule`), `ExportConfig`/`ImportConfig`/`BaseURL` API client additions, 153+ CLI tests, `build:cli`/`test:cli`/`validate:cli` npm scripts; CLI binary (`portier`/`portier.exe`) now built into `build/portier/` by all platform build scripts and included in release artifacts; Windows installer includes `portier.exe`; no PATH integration in v1.3; coverage gate enforces 92% threshold (92.7% actual after post-v1.3 ratchet). Tagged 1.3.0. The CLI talks to the management API; it does not replace the web UI or any runtime. See `tools/cli/readme.md` and `docs/roadmap.md`.

v1.4 is complete: Live Connection Inspector — read-only TCP connection and UDP session tracking in both runtimes, exposed via `GET /api/connections`, with a dedicated Live Connections view in the web UI. Coverage hardened across both runtimes before the feature was built. Shared live-connection types in `@portier/shared`, contract validation updated to 116/116, API Docs updated. Tagged 1.4.0. See `docs/roadmap.md` and `docs/changelog.md`.

v1.5 is complete: Declarative Config & Drift Control — plan/diff/apply workflows for comparing desired config files with the running configuration, previewing changes, and applying them safely from the CLI or UI. All 8 slices done: shared plan/diff/apply types in `@portier/shared` (`shared/sources/plan.ts`); pure plan engine in TypeScript server (`server/sources/config-plan.ts`) with id-first matching, identity key fallback, 8 material field diff, destructive flag, `REMOVE_EXISTING`/`LAN_EXPOSURE` warnings; `POST /api/config/plan` endpoint live on TypeScript server (Slice 2) and Go service (Slice 3, `service/sources/configplan/`); `POST /api/config/apply` endpoint live on TypeScript server and Go service (Slice 5) — plan errors → ok:false; dryRun → ok:true no mutation; destructive without yes → 400; drift → replace import with ID injection; `validate:contract` 156/156 (all plan + apply assertions against both runtimes); API Docs updated (no planned badges); 65 TS engine unit tests + 11 TS API integration tests + 49 Go engine unit tests + 10 Go plan API tests + 11 Go apply API tests; `portier config plan <file>`, `portier config diff <file>`, `portier config apply <file>` CLI commands (Slices 4–5) with structured output, field-level change detail, `--fail-on-drift` (exit 4), `--show-unchanged`, `--yes`, `--dry-run`, `--backup-out`, `--json`; 230+ CLI tests; CLI coverage 93.2% (gate 93%); Settings UI Plan & Apply section (Slice 6) — `planHelpers.ts` (4 helpers, 17 tests), `planConfig`/`applyConfig` API helpers with tests, Plan & Apply Config section in SettingsView with file picker, plan preview (summary counts/errors/warnings/operation list/destructive confirmation checkbox), apply with `yes:true`, `ok:false` error path, form-clear on success; 23 new SettingsView unit tests; E2E test in `settings.spec.ts`; client branch coverage 90.1% (gate 90%); all 5 coverage gates ratcheted to v1.5.0 values (Slice 7); full validation suite passed, version bumped to 1.5.0, changelog finalized (Slice 8). Tagged 1.5.0. See `docs/roadmap.md` and `docs/changelog.md`.

v1.6-pre is in progress: Coverage Ratchet & Quality Hardening — targeted test uplift for service (≥90% target), server (≥95% stmts target), CLI (≥95% target), and client (preserve ≥95% stmts). Adds meaningful tests for uncovered edge cases; ratchets all five coverage gates. No new product features. See `docs/roadmap.md` and `docs/checklist.md`.

v1.6 is planned: Architecture, Quality & Maintainability Audit — a dedicated audit and hardening release after v1.4 and v1.5 have raised coverage enough to make refactoring safe. Inspects architecture boundaries, runtime parity, forwarding correctness, API contract, CLI quality, UI quality, test quality, security/safety posture, packaging, and documentation. The v1.4/v1.5 coverage push is the prerequisite safety net for this work. Raw audit notes should not be added to docs/; durable outcomes belong in curated docs or a tracked backlog. See `docs/roadmap.md`.

Release artifact commands:
- `npm run build:release:current` — portable archive + installer for current platform
- `npm run build:release:portable` — portable archive only
- `npm run validate:release:portable` — validate portable archive layout and contents
- `npm run validate:release:current` — validate portable + installer artifacts

Do not add release artifact commands to `npm run check`. They are explicit release steps.

## Review Checklist

- TypeScript types are strict and useful.
- TCP sockets clean up on close/error.
- UDP behavior is documented, especially bidirectional-last-client limitations.
- Duplicate `protocol + listenHost + listenPort` bindings are rejected.
- Management API remains localhost by default.
- Port advisory rules live in `shared/sources`.
- Tests cover validation and lifecycle behavior where practical.
- `README.md`, `docs/architecture.md`, and `docs/checklist.md` are updated for user-visible behavior changes.
- When an API endpoint is added, removed, or changed: update both `docs/api-contract.md` (durable external contract) AND the client in-app API Docs view (`client/sources/features/apidocs/ApiDocsView.tsx`) (user-facing in-app reference). Update `ApiDocsView.test.tsx` for new endpoints. Do not mark an API slice complete until both documentation surfaces and their tests are updated.

## Claude Response Style

- Summarize changed files.
- Mention validation commands run.
- Mention anything not run and why.
- Call out risks and follow-up tasks.
- Do not claim success unless validation passed or limitations are stated.

## Claude Code Hooks And Settings

Claude Code project settings live in `.claude/settings.json`. Hook event names and schemas can vary by Claude Code version, so treat the hook wiring as conservative project guidance. If a local Claude Code install rejects the hook section, keep the scripts and adjust only the event names/schema for that installed version.
