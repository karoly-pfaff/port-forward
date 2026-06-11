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

**Durable rule (v1.7 Slice 6):** CLI config commands must share their file load+parse+validate prelude (`loadConfigForCommand`) and their local-rule→`client.ConfigRule` mapping (`toConfigRules`, via the `build*Request` helpers) — do not re-inline `os.ReadFile`+`parseLocalConfig`+`validateLocalConfig` or the field-by-field DTO loop per command. New config commands reuse these helpers (pass the command's verb + error exit code). `RunConfigValidate` is the documented exception (JSON-aware parse-error output) — leave it bespoke unless a JSON-aware variant is added deliberately. Per-command exit-code semantics (import file errors → 1; plan/diff/apply → 2; plan errors → 1; drift+`--fail-on-drift` → 4; connection → 3) and exact stderr/stdout wording are observable CLI behavior and must stay byte-identical through any refactor — guard with the existing black-box `commands_test` behavior tests; do not add brittle private-helper tests. The CLI stays a pure API client (no server/service imports, no DTO changes).

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

Go test binary path (Windows Firewall stability): `test:service` and `coverage:service` pass `-o build/tests/` so the compiled Go test binaries (`api.test.exe`, etc.) are written to a stable, gitignored path instead of go's per-run temp dir. Some service tests genuinely bind `0.0.0.0` (e.g. `TestDiagnoseLANExposureWarns`, which exercises the real diagnose listen-bind on a `0.0.0.0` rule), which triggers a Windows Firewall prompt; the firewall rule is keyed by executable path, so a fixed path lets one "allow" decision persist instead of re-prompting every run. Do not remove the `-o build/tests/` flag — and do not change the diagnose test to loopback (the `0.0.0.0` bind is faithful product behavior, not a test artifact).

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
