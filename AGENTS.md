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

v1.7 Slice 5 (diagnose check-phase helper split): `diagnoseRule` in BOTH runtimes (`server/sources/diagnose.ts`, `service/sources/api/diagnose.go`) reduced to an explicit ordered phase list of one-check helpers — `checkListenHost`/`checkLanExposure`/`checkPrivilegedPort`/`checkCommonPort`/`checkListenBind`/`checkTargetHost`/`checkTargetConnect`/`checkUdpMode` (UDP only). `targetResolved` derives from the target-host check's pass status. I/O helpers + `buildSummary` unchanged. Output byte-identical; `validate:contract` 185/185; one ordering regression test added per runtime (find-by-id tests + contract ID-membership did not guard order).

**Durable architecture rule (v1.7 Slice 5):** The diagnose flow must stay a flat ordered list of small named per-check phase helpers in BOTH runtimes, kept structurally mirrored (split/extend in lockstep — they cannot drift). A new diagnostic check is a new `checkX` helper inserted at the right position in `diagnoseRule` in both `diagnose.ts` and `diagnose.go`, plus a `validate:contract` assertion and the per-runtime ordering test — not inline appended logic, not a generic rule-engine/registry. Each phase helper returns exactly one `DiagnosticCheck` with ID/label/severity/message/details byte-identical across runtimes; check order is observable contract — keep the ordering tests (`diagnose.test.ts` "check ordering", Go `diagnose_phases_test.go`) and the contract ID assertion green. Derive `targetResolved` only from the target-host check's pass status.

v1.7 Slice 6 (CLI config loader/mapper cleanup): CLI config commands (`tools/cli/sources/commands/config.go`) share the read→parse→validate prelude via `loadConfigForCommand(filePath, verb, errExit, stderr)` (import errExit 1; plan/diff/apply errExit 2) and the local-rule→`client.ConfigRule` mapping via `toConfigRules`, used by `buildPlanRequest`/`buildApplyRequest`/`buildImportRequest`. `RunConfigValidate` keeps its bespoke JSON-aware parse-error flow. Behavior/exit codes/output byte-identical; CLI stays a pure API client; `validate:contract` 185/185; coverage 97.7→97.9 (gate 95 PASS); 5 helpers 100% covered by existing black-box tests.

**Durable architecture rule (v1.7 Slice 6):** CLI config commands must share their file load+parse+validate prelude (`loadConfigForCommand`) and their local-rule→`client.ConfigRule` mapping (`toConfigRules`, via the `build*Request` helpers) — no per-command re-inlining of `os.ReadFile`+`parseLocalConfig`+`validateLocalConfig` or the DTO loop. New config commands reuse these (pass verb + error exit code); `RunConfigValidate` is the documented bespoke exception (JSON-aware parse errors). Per-command exit-code semantics (import/plan/diff/apply local-input file errors → 2 — see Slice 7; plan errors → 1; drift+`--fail-on-drift` → 4; connection → 3) and exact stderr/stdout wording are observable CLI behavior — keep byte-identical through refactors, guarded by the black-box `commands_test` tests; no brittle private-helper tests. CLI stays a pure API client (no server/service imports, no DTO changes).

v1.7 Slice 7 (CLI exit-code normalization review): audited all CLI command exit codes; fixed one clear inconsistency — `config import` local file read/parse/validation errors `1`→`2` to match `config apply`/`plan`/`diff` (one-line `loadConfigForCommand(…, "import", 2, …)`). Documented the policy in `tools/cli/readme.md`; tightened three import local-error tests `!= 0`→`== 2`. `validate`'s `1`-for-invalid/unreadable is the documented validator-semantics exception. `validate:contract` 185/185; CLI coverage gate 95 PASS.

**Durable rule (v1.7 Slice 7) — CLI exit-code policy:** `0` success; `1` API/runtime/operation error (server rejection, local output-**write** failure) and `validate`'s invalid/unreadable result; `2` invalid arguments/usage AND invalid local **input** config (unreadable/malformed/validation failure) for `config import`/`plan`/`diff`/`apply`; `3` connection failure; `4` drift with `--fail-on-drift`. Intentional, documented (not to be "fixed"): local **input** errors `2` vs local **output**-write failures `1`; `config validate` reports invalid/unreadable as `1` (exit code is its result) while missing the path arg is `2`; rule `<id|name>` matching nothing is `1` (target absent, 404-like) while ambiguous is `2` (fixable selector). New CLI commands MUST follow this policy: route API/connection errors through `exitWithError` (→ `1`/`3`); `2` for bad local input, `1` for output/operation failures. Exit-code changes are observable behavior — update `tools/cli/readme.md` + per-command help + a black-box `commands_test` assertion in the same change.

v1.7 Slice 8 (SettingsView decomposition): `client/sources/features/settings/SettingsView.tsx` (was 736 lines) is now a ~90-line orchestrator composing per-panel components (`ExportConfigSection`/`PlanApplySection`/`ImportConfigSection`/`RuntimeEnvironmentSection`/`DiagnosticsExportSection`) over a shared `SettingsSection` wrapper, with logic in hooks (`useRuntimeInfo`/`useClipboardCopy`/`useConfigExport`). Each panel owns its state; the shared config-export flow (Export panel + Import backup button) is created once via `useConfigExport()` and passed to both. DOM byte-identical; `SettingsView.test.tsx` (63 tests) unchanged + green; client gates 94/89/78 unchanged; settings E2E 6/6.

**Durable rule (v1.7 Slice 8) — settings/feature-view decomposition:** A feature view with multiple stateful concerns must be a thin orchestrator composing focused panel components, not one giant component. Each panel owns its state; lift to the parent ONLY for a genuine cross-panel concern (e.g. the shared export `exporting` flag) and pass it down — no whole-view prop drilling. Extract a hook (`useX`) for cohesive logic+state (fetch/clipboard/export); extract a small presentational primitive for repeated wrapper markup. Decomposition must be DOM-preserving: move JSX, classes, `aria-label`/`role`/`id`/`htmlFor`, and `onClick` verbatim — no relabeling/structure/accessibility change. Tests render the composed top-level view (role/label/text) so a correct decomposition needs NO test changes; if a test must change, the DOM changed — re-check. Component files CamelCase, hooks camelCase, under the feature folder. Client-internal (no API/contract impact); run the feature's E2E spec when it has one.

v1.7 Slice 9 (UI wording consistency pass): audited client UI copy vs `docs/glossary.md`; one glossary-backed fix — sidebar nav label "Connections" → "Live Connections" (`client/sources/app/NavItem.ts`), matching the view title + glossary. Updated `tests/e2e/connections.spec.ts` `goToConnections` (nav name → "Live Connections"; confirm arrival via the unique `tablist`/"Connection views" role since the title text is no longer unique). Rest already aligned; "API Docs" (nav) vs "API Reference" (heading) intentionally left. No behavior/API change; `validate:contract` 185/185; full E2E 34/34.

**Durable rule (v1.7 Slice 9) — UI wording:** User-facing client labels must use `docs/glossary.md` canonical terms. A view's nav label should match its title/canonical term when the glossary anchors it ("Live Connections", not bare "Connections"); a short nav label MAY differ from a descriptive page heading only when no glossary term applies and both are unambiguous. Keep observed vs desired state distinct (running/stopped/error vs enabled; "Autostart" = UI label for `enabled`), TCP "connection" vs UDP "session", "Diagnose" (action) vs "Diagnostics export" (bundle). Do NOT rename frozen public surfaces for cosmetic UI consistency (`/api/forwards`, `listenHost`/`targetHost`, `clientAddress`/`targetAddress`, the `connections` view *id*). A visible label change is observable behavior — update any test/E2E selecting by that text in the same change, and prefer a unique role-based selector when a renamed label becomes non-unique (nav button + view title sharing text).

v1.8 Slice 1 (rule group metadata contract): added an optional `group` label to forward rules (v1.8 Operator Power Tools foundation). `group?: string` on shared `ForwardRule`/`ConfigPlanRuleSnapshot`; `Group *string json:"group,omitempty"` on Go `domain.ForwardRule`/`ForwardRuleResponse`/`configplan.RuleSnapshot`; `group` on CLI DTOs (all `omitempty`). Validation parity-tested in both runtimes: optional, trimmed, empty→absent, ≤ 64 chars, no control chars, non-string rejected. PATCH: `""` clears, non-empty sets, absent/`null` unchanged. Plan **material** but **non-destructive** field (in `MATERIAL_FIELDS`/Go group-diff branch; never in `FORWARDING_FIELDS`). Preserved through export/import/plan/apply. Runtime forwarding/lifecycle/duplicate-binding/status unchanged. `validate:contract` 185 → 204 (`rule-group` group + `plan:GROUP_CHANGE` parity). No UI/CLI group operations yet (deferred). No gate change. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 1) — rule group metadata:** Rule `group` is optional, behavior-neutral metadata until group operations land — it must NOT affect forwarding, lifecycle, duplicate-binding (`protocol + listenHost + listenPort` only), or status. Config import/export/plan/apply must preserve `group` in BOTH runtimes; an ungrouped rule omits the field and produces no drift. Group validation (optional, trimmed, empty→absent, ≤ 64 chars, no control chars, same wording) must stay parity-tested across TS (`@portier/shared`) and Go (`service/sources/validation`), with a `validate:contract` scenario. In a plan, `group` is material but non-destructive (in `MATERIAL_FIELDS`, never `FORWARDING_FIELDS`). PATCH: `""`/whitespace clears, non-empty sets, absent/`null` unchanged. CLI stays a pure API client. Do NOT widen into tags/profiles/multi-group/colors or rename the frozen `group` field.

v1.8 Slice 2 (rule group UI editing and display): surfaced `group` in the web UI — client-only, `validate:contract` 204/204. Optional **Group** input in the rule drawer (`RuleForm.ts` state + `ForwardRuleForm.tsx`); `formToPayload` always sends raw `group` so the Slice 1 PATCH contract clears (empty)/sets (non-empty); `maxLength=64`; explicit `aria-label="Group"` (the in-`<label>` hint span would otherwise pollute the accessible name — same fix as Listen Host). Subtle `.rule-group-label` chip under the name in `ForwardRuleList.tsx` (none when absent); search now matches `group`. 5 form + 3 list unit tests + 1 E2E (create-with-group, edit-clear). Client gates unchanged. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 2) — rule group UI:** Surface optional metadata fields through `RuleForm.ts` (`RuleFormState`/`formToPayload`), always sending the raw value so the Slice 1 PATCH contract handles set/clear (no client-side special-case for clearing). A form input whose `<label>` also holds a persistent hint `<span>` MUST carry an explicit `aria-label` or the accessible name breaks (mirror Listen Host). Display group as a subtle secondary chip (`.rule-group-label`), nothing when absent, server validation authoritative. Reuse existing search for group; no group filtering/grouping/bulk UI this slice. Client-internal — no API/contract/gate change.

v1.8 Slice 3 (group filtering groundwork): added a read-only **Filter by group** `<select>` to `ForwardRuleList.tsx` — client-only, `validate:contract` 204/204. Options derived from live rules (distinct trimmed, `localeCompare` `sensitivity:"base"`) + All Groups + Ungrouped (only if ungrouped rules exist); rendered only when ≥1 group exists. Filter AND-combines with search+status in one `filteredRules` pass; derived `activeGroupFilter` clamps stale selections to All Groups; reorder disabled while active; empty states unchanged. 8 list unit tests + 1 E2E; one Slice 2 chip assertion scoped to the table. Client gates unchanged. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 3) — rule list group filter:** Group filtering is read-only client navigation (no rule mutation, no API/runtime touch). Derive options from the live `rules` prop (distinct/trimmed/locale-insensitive sorted), never persist them. Apply the group filter in the SAME `filteredRules` predicate as search/status so they AND-combine; keep existing empty-state messages. Always clamp a stale specific-group selection to "all" via a derived value (a controlled `<select>` must never hold an option-less value, and a deleted group must not hide every rule). Show "Ungrouped" only when an ungrouped rule exists; render the control only when ≥1 group exists. Use sentinel tokens that cannot collide with real groups. Native labelled `<select>`, keyboard-accessible; no visual grouping headers / start-stop / bulk / colours / persistence this slice. When a value appears in both a row chip and a filter option, scope chip assertions to the table. Client-internal — no API/contract/gate change.

v1.8 Slice 4 (group operations API): added `POST /api/forwards/groups/:group/{start,stop}` in BOTH runtimes — start/stop all rules sharing a `group`. Behaviour over metadata: no mutation of rule defs/order/`enabled`/`group`/duplicate-binding; reuses single-rule start/stop (same per-rule activity events, no new type). `ForwardManager.startGroup`/`stopGroup` + Go `Manager.StartGroup`/`StopGroup` iterate matched rules in **rule order**; start ignores `enabled`, skips already-running (`already_running`), partial-fails one without aborting (`failed`); stop skips not-running (`not_running`). No match → 404; invalid group → 400 (non-empty required: `validateGroupName`/`ValidateGroupName`, `group is required.`). Response `GroupActionResponse {group,action,total,succeeded,skipped,failed,results[]}` — shared `groups.ts` `summarizeGroupAction` mirrored by Go `buildGroupActionResponse`. Client `setGroupRunning` groundwork (no UI); `ApiDocsView` updated. `validate:contract` 204 → 225 (`group-action` + `group:start` parity). No UI/CLI commands. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 4) — group operations:** Group start/stop are behaviour over existing rule metadata, NOT config mutation — never change rule defs/order/`enabled`/`group`/duplicate-binding, and reuse the single-rule start/stop path (same per-rule activity events; no new group event type unless added deliberately in both runtimes). Results MUST follow the manager's **rule order**; summary `{group,action,total,succeeded,skipped,failed,results[{ruleId,ruleName,status,reason?}]}`. Start ignores `enabled` (parity with single-rule start); one rule's failure is a `failed` result, never an aborted op (partial success). Empty match → 404, not silent success. Group-operation path targets validated as non-empty (`validateGroupName`/`ValidateGroupName`, `group is required.`) plus normal length/control rules. TS+Go behaviour/shape/errors stay contract-tested (`group-action` group + `group:start` parity); keep `summarizeGroupAction`/`buildGroupActionResponse` counting mirrored. CLI stays a pure API client (no group commands/DTOs); client `setGroupRunning` is groundwork only (no UI). Ungrouped bulk actions deferred.

v1.8 Slice 5 (CLI group commands): `portier group list|start|stop <group>` over the Slice 4 API — CLI-only, pure API client, `validate:contract` 225/225. `list` derives groups from forwards+status (alphabetical, rule+running counts; empty → message + exit 0); `start`/`stop` call the group endpoints and print a summary + `RULE/STATUS/REASON` table (`--json` = full `GroupActionResponse`). Exit codes (v1.7 policy): 0 success incl. skips, 1 API error/no-match-404/`failed>0`, 2 missing/invalid group, 3 connection. Group arg trimmed + locally validated (`validateGroupArg`). New `client.StartGroup`/`StopGroup` + DTOs (`url.PathEscape`); Arch-D guard captures `group-start.json` (main capture rule stays ungrouped). Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 5) — CLI group commands:** CLI group commands are a pure API client over the Slice 4 endpoints — no reimplemented group semantics, no rule mutation. `group list` is derived client-side from live rules+status (distinct non-empty trimmed groups, counts), never persisted; empty → message + exit 0. Exit codes (v1.7 Slice 7 + Slice 4 semantics): skip (`already_running`/`not_running`) = success 0; `failed>0` or API no-match 404 = 1; missing/invalid local group arg = 2 (validate non-empty/≤`groupMaxLength`/no-control via `validateGroupArg`, server authoritative); unreachable = 3. `--json` emits full `GroupActionResponse` (start/stop) or `{groups:[{group,total,running}]}` (list). Keep the new CLI DTOs guarded by Arch-D `TestCLIDTOContractParity` (live `group-start.json`; keep the main capture rule ungrouped so plan/apply stay drift-free). Black-box `commands_test`. No rename/delete, ungrouped bulk, profiles, tags, colours.

v1.8 Slice 6 (UI group action buttons): exposed group start/stop in the web UI — client-only, `validate:contract` 225/225. Toolbar (`Start group "<name>"` / `Stop group "<name>"`) under the rule-list header (`ForwardRuleList.tsx`) only when Filter by group is one concrete group (hidden for All Groups + Ungrouped, gated by an `onGroupAction` prop). Click → `App.handleGroupAction` → `setGroupRunning` → `refreshAll`; loading (`Starting…`/`Stopping…`, both disabled = no double-submit); inline summary `role="status"`, warning `role="alert"` when `failed>0`, server error `role="alert"` on rejection/404; result cleared on selection change. 11 list + 2 App unit tests + 1 E2E. Client gates unchanged. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 6) — UI group actions:** UI bulk-by-group actions are a thin client over the Slice 4 endpoints (`setGroupRunning`) — no reimplemented semantics, no rule mutation, no API/DTO change. Surface group start/stop only for one concrete group (the active Filter by group selection) — never All Groups or Ungrouped. After success, refresh via the existing `refreshAll`/`onRefresh` pattern (don't hand-mutate state). Show a scoped inline result near the buttons (`role="status"` success/skip, `role="alert"` for `failed>0`/error) — no new global notification system; clear it on selection change. Disable both buttons in-flight (no double-submit). Buttons carry the group-scoped accessible name (`Start group "web"`). Keep the API call + refresh in the parent (`App`), the busy/result UI in the list, behind an optional `onGroupAction` prop. No rule-list redesign, no profiles/tags/colors/rename/management. Client-internal — no API/contract/gate change.

v1.8 Slice 7 (rule health basics): added derived `health` (`healthy`/`warning`/`error`) on `ForwardStatus` — `validate:contract` 225 → 234 (`status:health` parity). Shared `RuleHealth` + `health` field + `deriveRuleHealth({enabled,running,lastError})`; Go `domain.RuleHealth`/`DeriveRuleHealth`/`Health`. Manager is the single derivation site (TS `getStatus`, forwarders return `ForwarderStatus` = `Omit<...,"health">`; Go `statusForRule`). Priority `error`>`warning`>`healthy`. UI `RuleHealthBadge` dot — since v1.8 Slice 9 in its own `Health` column (dot + one-word `healthShortLabel`, `aria-hidden`), distinct from the Status column; CLI `status` HEALTH column + `--json` health. No probing/background check/mutation; `lastError` lifecycle unchanged. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 7) — rule health:** Rule `health` is a derived, deterministic interpretation of existing runtime state (`enabled`/`running`/`lastError`) — it must NEVER imply active target probing or background monitoring unless such probing is explicitly implemented (none here). Keep `health` and `status`/`running` distinct (status = lifecycle, health = operator reading); do not collapse them or replace the status badge. One derivation helper — `deriveRuleHealth` (TS) / `domain.DeriveRuleHealth` (Go) — kept logic-identical and parity-tested (`status:health` + `rule-health` scenario); priority is fixed (`error` > `warning` > `healthy`). Derive in the **manager** (it has `enabled`), not in forwarders/handlers/client; forwarders return `ForwarderStatus`/leave `Health` zero. Client and CLI **read** `health`, never recompute it. No new network I/O, no rule mutation, no health history/metrics/probing/auto-restart/retry this slice.

v1.8 Slice 8 (duplicate rule action): client-only duplicate-rule convenience — `validate:contract` 234/234 unchanged (no shared/server/service/CLI/API/DTO/config change, no backend endpoint). Each rule row has a **Duplicate** action (`Copy` icon, accessible name `Duplicate rule <name>`) opening the existing drawer in **create mode** pre-filled from the source. `ForwardRuleForm` gained optional `duplicateSource` (used only when not editing; edit wins), title "Duplicate Rule" + subtitle, drawer aria-label "Duplicate Forward Rule"; `ForwardRuleList` gained optional `onDuplicate`; `App` tracks `duplicateSourceId` and keys the drawer `dup-<id>`. `ruleToDuplicateForm` (`RuleForm.ts`) copies editable fields incl. **group**, name → `<name> copy`, drops id (create `POST`, not PATCH), forces `enabled:false` (a created enabled rule auto-starts). Runtime-only state never copied. Validation unchanged (duplicate-binding caught by create). Four loose-substring E2E selectors tightened to `exact:true`. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 8) — duplicate rule:** Duplicate-rule is a **client-only create-flow convenience** — no backend duplicate endpoint, no source mutation, no API/DTO/config change (saves via the existing create `POST /api/forwards`, `id` undefined). Prefill (`ruleToDuplicateForm`) copies only editable fields **including `group`**, sets name `<name> copy`, **drops id**, **forces autostart off** (a created `enabled` rule auto-starts); never copy runtime-only state (id/status/`lastError`/health/active counts). Do NOT bypass/special-case validation — the duplicate passes the same client+server validation, so an unchanged listen binding shows the normal duplicate-binding error (no port auto-increment). Open via the rule drawer in create mode (distinct title/subtitle) through the optional `duplicateSource` prop (ignored when `editingRule` is set); keep the row action behind optional `onDuplicate` with the rule-scoped name `Duplicate rule <name>`. When a new always-present row action embeds the rule name, retighten sibling action selectors that match by loose substring name to `exact:true`. No templates/saved templates/profile clone/group-level duplicate. Client-internal — no contract/coverage-gate change.

v1.8 Slice 9 (config preview UX polish): client-only polish of the Settings Plan & Apply preview (`PlanApplySection.tsx` + helpers in `planHelpers.ts` + scoped CSS) — `validate:contract` 234/234, no API/DTO/config or plan/apply semantics change. Summary gained `Destructive: N`; each op is a card with an action badge, plain-language impact note (`describeOperationImpact`), and text tags `Destructive`/`Metadata only`; each change row shows a friendly field label (`formatFieldLabel`), a `forwarding`/`metadata` tag (`changeImpact`), and `before → after`. Update impact derives from actual changed fields via a client mirror of the server `FORWARDING_FIELDS` set, so a group-only edit reads "Metadata only — the forwarder is not restarted". Server stays authoritative for `destructive` Apply-gating; existing no-drift/error/warning/confirmation/apply states unchanged. Source: `docs/roadmap.md` v1.8.

**Durable rule (v1.8 Slice 9) — config preview UI:** The config plan/apply preview is a **read-only presentation of the existing plan response** — no plan/apply semantics, `destructive` gating, or API/DTO/config change; server stays authoritative for `destructive`. Convey destructive/metadata state by **text** (badges/tags + impact note), never colour alone. A group-only (or any non-forwarding) update must be shown **metadata-only** and must **not imply a socket restart** — derive an update's impact from the actual changed fields via a client-side mirror of the server's `FORWARDING_FIELDS` (`protocol`/`listenHost`/`listenPort`/`targetHost`/`targetPort`/`udpMode`); keep that copy display-only (no server-internal imports, no moving classification into shared). Keep pure logic in `planHelpers.ts` (`formatFieldLabel`/`changeImpact`/`isMetadataOnlyUpdate`/`describeOperationImpact`) with unit tests, `PlanApplySection` thin, and preserve the existing no-drift/error/warning/confirmation/apply states + their `role` semantics. No redesign, no diff viewer, no new workflow. Client-internal — no contract/coverage-gate change.

**Durable rule (v1.8) — Forward Rules row action menu:** Forward Rules rows keep **only Start/Stop as an inline button**; **Edit, Duplicate, Diagnose, Activity, Delete** (in that order, Edit first) live in a per-row kebab (`MoreVertical`) overflow menu (`ForwardRuleList.tsx`) opened by a `More actions for <rule name>` trigger (`aria-haspopup`/`aria-expanded`). The trigger is **seamless — styled like the reorder drag handle** (`.row-menu-trigger`: no border/background, muted glyph brightening on hover/open), not a boxed `btn-icon`. Dropdown is `role="menu"` (`Actions for <rule name>`) of `role="menuitem"` buttons, **Delete red** (`row-menu-item--danger`) still firing the inline Confirm/Cancel delete prompt. One menu open at a time; closes on outside-click + Escape; trigger disabled while busy/confirming. Adding/moving a row action: keep Start/Stop inline, menu items get plain text names + icon, and **tests must open the menu first** and target `role="menuitem"` (unit `openRowMenu` helper; E2E click `More actions for <name>` then the `menuitem`). Client-internal — no API/contract/coverage-gate change.

**Durable rule (v1.9 Slice 4) — doctor strict mode:** `--strict` changes **only the exit-code interpretation** of a doctor report (warnings become failing findings) — it must **never change which checks run**, never mutate runtime/config, never add probing. Both doctor commands share the strict-aware helpers (`doctorExitCode(report, strict)` / `doctorResultLabel`) and the one `emitDoctorReport(..., strict, ...)` path — no per-command exit logic. Frozen normal-mode semantics: info-only → 0, warnings-only → 0, errors → 1; strict only adds warnings-only → 1. Keep JSON **additive/backward-compatible**: `checks`/`summary` stay on the pure `DoctorReport`; `strict`/`result` are added via the embedded `doctorReportJSON` wrapper at emit (don't pollute `DoctorReport` or restructure the shape). `result` mirrors the exit code (`passed`=0/`failed`=1). Human output stays deterministic: the `Result:` line always prints; the `Strict mode: warnings are treated as failures.` note prints only when `strict && errors==0 && warnings>0`. For positional doctor commands (`config doctor`) flags precede the file (like apply/plan/diff) — document it, no bespoke interspersed parsing. CLI stays a pure client; no API/DTO/contract change.

**Durable rule (v1.9 Slice 3) — doctor explain:** Every **stable doctor/check code must have an explanation** in the `explanations` registry (`tools/cli/sources/commands/explain.go`) — the `explain_internal_test.go` guard (via the explicit `allDoctorCodes` constant list) fails if a code constant has no entry, so **a new doctor code adds/updates its explanation in the same slice**. Explanations are **static, deterministic, offline** reference data — no runtime contact, no external docs fetch, no AI, and must **not claim active probing or automatic remediation** (keep `action` to operator guidance, not a fix promise). Key the registry by the existing doctor code **constants** (no duplicated string literals); keep `related` pointing only at real codes (guarded), and keep `Explanation` small (`code/title/meaning/action/severity/related`) — no docs system / plugin / per-code loader. Unknown/missing code → **exit 2** (clear error → `--list`, never a generic fallback); `--list` stays sorted/deterministic. `explain` is fully offline — no URL resolution/client, and stays out of the invalid-URL dispatch test. No API/DTO/contract change. This is the canonical place to explain replay/config/error codes later — extend the same registry, don't fork a second explainer.

**Durable rule (v1.9 Slice 2) — live runtime doctor:** `portier doctor` is a **read-only, deterministic** live diagnostic — never mutate runtime/config, never start/stop rules, **never probe forwarding targets** (unless target probing is explicitly added later). It **reuses the Slice 1 doctor model** (`DoctorReport`/`DoctorCheckResult` + human/JSON/summary/exit helpers) — no second doctor or health model. Rule **health is read from the API `health` field** — the CLI must **not re-derive** it (runtime is source of truth). Codes (`runtime.*`, `rules.*`, `config.export_*`) are **stable operator-facing identifiers** (test new ones); checks run in a **fixed deterministic order** with stable JSON shape. Version mismatch is a **warning, never an error**. Exit codes follow the doctor policy (0/1/2); the **unreachable-runtime → exit 1 (not 3)** deviation is intentional and documented in `tools/cli/readme.md` (the doctor always completes and emits a report). CLI stays a **pure API client** (reuse `GET /api/runtime`/`/api/status`/`/api/config/export`; no new API/DTO/contract). New live checks need **meaningful httptest-driven black-box tests**. No background monitoring, auto-fix, server/service doctor endpoint, or UI doctor panel this slice.

**Durable rule (v1.9 Slice 1) — doctor model & offline config doctor:** Doctor **check codes** (e.g. `config.valid`, `config.lan_exposure`) are **stable operator-facing identifiers** (a CLI/tool contract) — do not rename casually; test any new code. Keep the result model small: `DoctorCheckResult {code, severity(info|warning|error), title, message, details?}` → `DoctorReport {checks[], summary}` (`tools/cli/sources/commands/doctor.go`) — **not** a diagnostics/plugin/check-pack framework. Doctor output must be **deterministic** (stable check order — file order for per-rule advisories — and stable JSON shape) and **symbol-free ASCII** (`[INFO]`/`[WARN]`/`[ERROR]`). The **offline `portier config doctor <file>` must not require/contact a live runtime** and must never modify the file; reuse `parseLocalConfig`/`validateLocalConfig` (don't fork validation), and partition duplicate-binding from field errors via the shared `duplicateBindingErrPrefix`. Warnings (LAN exposure `0.0.0.0`, privileged port `< 1024`) are derived **only from file contents** — they must **not imply runtime probing/target reachability**, and warnings alone exit `0` (no `--strict` yet). Exit codes follow the v1.7 policy: `0` no error-severity checks, `1` ≥1 error-severity check, `2` missing/invalid local argument. The CLI stays a **pure API client** (no shared/server/service imports; mirror shared thresholds locally; document deferred duplication like the full common-port table). Every new doctor check needs **meaningful tests** (prefer black-box command tests; white-box only for model/helper branches). No server/service doctor endpoint, no UI doctor panel, no live runtime doctor, no config mutation this slice.

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
