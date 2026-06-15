# @portier/server

The Portier **TypeScript runtime/fallback server**: the REST management API, config
persistence, and forwarding lifecycle for the Node-based runtime. The native Go
service (`service/`) remains the preferred runtime; this server is the Node
fallback and the reference TypeScript implementation that `validate:contract`
checks for parity against Go.

## Two coexisting implementations

This server has two implementations of the same REST API: the **Express** app
(the default active runtime) and a **NestJS** app (served only under
`npm run start:nest`). Both serve the same `/api` surface; the migration to NestJS
is incremental and reversible, and both currently coexist:

| | Active runtime | Shadow runtime (NestJS) |
|---|---|---|
| Entry point | `sources/index.ts` | `sources/nest/main.ts` |
| Composition | `sources/api.ts` (`createApp`) | `sources/nest/app.module.ts` |
| Framework | Express 5 | NestJS 11 (Express 5 platform) |
| Status | **Active** — serves the real API, CLI, and web UI | **Shadow** — served only under `start:nest` |
| Scripts | `dev`, `build`, `start` (via root) | `start:nest`, `build:nest`, `test:nest` |

The **Express server is the default active server.** The NestJS app under
`sources/nest/` does not replace it, is not started by the normal runtime, and
does not change any REST/API behavior. Every NestJS endpoint is checked
byte-for-byte against its Express counterpart and guarded by
`npm run validate:contract` (234/234). The eventual runtime switch (making NestJS
the default) is a deliberate, separately-validated step that preserves a rollback
path — see *Legacy Express server (future work)* below.

### NestJS scaffold surface

- `GET /health` → `{ ok: true, server: "node", name: "Portier" }` — a liveness
  probe that needs no runtime manager. It is **outside the frozen `/api`
  contract**; aligning to the documented `GET /api/health` (currently
  "TypeScript server: not implemented") is deferred to the endpoint-migration
  slices so it lands behind `validate:contract`.
- `GET /api/ports/advisory` (v1.14 Slice 2) — the first migrated read-only API
  route. **Byte-for-byte identical to the existing Express route** (parity-tested)
  and served only under `start:nest`; the Express server still serves the
  default route unchanged.
- `GET /api/activity` (v1.14 Slice 4) + `DELETE /api/activity` (v1.14 Slice 8 —
  first migrated write) over an injected store (`ACTIVITY_STORE` token, default =
  a fresh domain `ActivityStore`). GET reads via `ActivityReader`; DELETE clears
  via `ActivityClearer` and returns `204` empty (`@HttpCode(204)`, no DTO — no
  input). Byte-for-byte parity-tested; shadow-only under `start:nest`.
- `GET /api/status` (v1.14 Slice 5) — read-only per-rule status over a narrow
  `StatusReader` (`STATUS_READER` token, default = a trivial empty reader; the
  domain `ForwardManager` satisfies it and is bound in tests / when Nest is
  active). The first manager-dependent read; byte-for-byte parity-tested with
  stopped rules (no volatile fields); shadow-only under `start:nest`.
- `GET /api/forwards` (v1.14 Slice 6) — read-only rule list (each rule decorated
  with port advisories via the shared `getPortAdvisories`) over a narrow
  `ForwardsReader` (`FORWARDS_READER` token, same provider pattern as status).
  Byte-for-byte parity-tested; shadow-only. The write/lifecycle routes under
  `/api/forwards/...` stay with Express, deferred.
- `GET /api/runtime` (v1.14 Slice 9) — the first migrated endpoint with
  **volatile** fields (`uptimeSeconds`, process `pid`/`platform`/`arch`). Both the
  Express route and the Nest `RuntimeService` call one shared pure builder
  (`buildRuntimeInfo` in `sources/runtime-info.ts`), so the shape cannot drift.
  Volatile values come from three narrow readers — `ClockReader`/`CLOCK_READER`
  (generic `now()`, now shared from `common/clock.reader.ts`),
  `ProcessReader`/`PROCESS_READER`, and `RuntimeInfoReader`/`RUNTIME_INFO_READER`.
  Byte-for-byte parity-tested by booting Express and Nest with the **same fixed
  clock + runtime info** (real process → matching `pid`/`platform`/`arch`), so
  `uptimeSeconds` is deterministic and **no field is normalized/stripped**.
  Shadow-only.
- `GET /api/config/export` (v1.14 Slice 10) — the second volatile read endpoint
  (`exportedAt`). Both the Express manager (`ForwardManager.exportConfig`) and the
  Nest `ConfigExportService` build the snapshot via the shared pure builder
  `buildExportedConfig` (in `sources/config-export.ts`), with `exportedAt` stamped
  from the shared `ClockReader`. Read-only over a narrow
  `ConfigExportReader`/`CONFIG_EXPORT_READER`; the `config.exported` activity
  emission (a write side-effect) stays with the Express manager, deferred with the
  config-write migration. Byte-for-byte parity-tested by booting both apps with the
  same fixed clock + the same seeded manager (`exportedAt` and rules deterministic,
  **no field normalized/stripped**). Shadow-only; config import/write stays with
  Express.
- `GET /api/connections` (v1.14 Slice 11) — the last volatile read endpoint
  (`generatedAt`). Both the Express route and the Nest `ConnectionsService` build
  the snapshot via the shared pure builder `buildLiveConnections` (in
  `sources/connections-snapshot.ts` — the route's former inline per-rule
  aggregation), with `generatedAt` from the shared `ClockReader`. Read-only over a
  narrow `ConnectionsReader`/`CONNECTIONS_READER`. Byte-for-byte parity-tested with
  **seeded fixed `TcpConnectionInfo`/`UdpSessionInfo` records (no sockets/registries)**
  and a pinned clock, so every field is deterministic (**no normalization/stripping**).
  Shadow-only; no connection lifecycle/mutation.
- All `/api/*` errors → the Portier `{ "errors": ["..."] }` envelope via the
  global `ApiErrorEnvelopeFilter` (v1.14 Slice 3): unmatched routes →
  `404 ["API route was not found."]`, controller-raised `400`s carry their
  messages, unknown errors → `500 ["Internal server error."]` (no leak). Non-API
  routes keep NestJS's default error shape. Controllers raise
  `ApiBadRequestException(string[])` rather than hand-rolling the envelope.

### Migration status (endpoint inventory)

The **read-side `/api` migration is complete** (v1.14 Slice 11); the rule CRUD
trio is migrated — **create** (`POST /api/forwards`, Slice 14), **update**
(`PATCH /api/forwards/:id`, Slice 15), and **delete** (`DELETE /api/forwards/:id`,
Slice 16) — plus the single-rule lifecycle pair, **start**
(`POST /api/forwards/:id/start`, Slice 17) and **stop**
(`POST /api/forwards/:id/stop`, Slice 18), **reorder**
(`POST /api/forwards/reorder`, Slice 19), **diagnose**
(`POST /api/forwards/:id/diagnose`, Slice 20), and the **group-action pair** —
**group stop** (`POST /api/forwards/groups/:group/stop`, Slice 21) and **group
start** (`POST /api/forwards/groups/:group/start`, Slice 22). The entire
`/api/forwards` surface is migrated, and the config milestone is **complete** — the
**non-mutating** `POST /api/config/plan` dry-run (Slice 23), the **mutating**
`POST /api/config/import` (Slice 24), and the **mutating** `POST /api/config/apply`
(Slice 25). **Static client serving** (Slice 26) is migrated too — see the
*Static client serving* section below. Every endpoint below is **shadow-only** —
served by the Nest app only under `npm run start:nest`; the Express server
(`sources/index.ts` + `sources/api.ts`) remains the **default active runtime** and
serves all routes unchanged. `validate:contract` is 234/234.

| Endpoint | Module | Request DTO | Response DTO | Provider token | Builder / volatile |
|---|---|---|---|---|---|
| `GET /health` | `health/` | — (no input) | typed constant¹ | — | — |
| `GET /api/ports/advisory` | `api/ports/` | `PortsAdvisoryQueryDto` + pipe | `PortsAdvisoryResponseDto` | — (pure shared logic) | — |
| `GET /api/activity` | `api/activity/` | endpoint-local coercion² | `ActivityListResponseDto` | `ACTIVITY_STORE` (`ActivityReader`) | — |
| `DELETE /api/activity` | `api/activity/` | — (no input) | — (`204` no body)³ | `ACTIVITY_STORE` (`ActivityClearer`) | — |
| `GET /api/status` | `api/status/` | — (no input) | `StatusListResponseDto` | `STATUS_READER` | — |
| `GET /api/forwards` | `api/forwards/` | — (no input) | `ForwardsListResponseDto` | `FORWARDS_READER` | — |
| `POST /api/forwards` | `api/forwards/` | `CreateForwardRuleBodyDto`⁴ | `ForwardRuleResponseDto` (`201`) | `FORWARD_RULE_CREATOR` (`ForwardRuleCreator`) | — |
| `PATCH /api/forwards/:id` | `api/forwards/` | `:id` (`@ApiParam`) + `UpdateForwardRuleBodyDto`⁴ | `ForwardRuleResponseDto` (`200`) | `FORWARD_RULE_UPDATER` (`ForwardRuleUpdater`) | — |
| `DELETE /api/forwards/:id` | `api/forwards/` | `:id` (`@ApiParam`)⁴ | — (`204` no body)³ | `FORWARD_RULE_DELETER` (`ForwardRuleDeleter`) | — |
| `POST /api/forwards/:id/start` | `api/forwards/` | `:id` (`@ApiParam`)⁴ | `ForwardStatusDto` (`200`)⁵ | `FORWARD_RULE_STARTER` (`ForwardRuleStarter`) | — |
| `POST /api/forwards/:id/stop` | `api/forwards/` | `:id` (`@ApiParam`)⁴ | `ForwardStatusDto` (`200`)⁵ | `FORWARD_RULE_STOPPER` (`ForwardRuleStopper`) | — |
| `POST /api/forwards/reorder` | `api/forwards/` | `ReorderForwardRulesBodyDto` + pipe⁶ | `ForwardRuleResponseDto[]` (`200`) | `FORWARD_RULES_REORDERER` (`ForwardRulesReorderer`) | — |
| `POST /api/forwards/:id/diagnose` | `api/forwards/` | `:id` (`@ApiParam`)⁴ | `RuleDiagnosticsResultDto` (`200`)⁷ | `DIAGNOSTIC_READER` + `CLOCK_READER` | `diagnoseRule` (`diagnosedAt`) |
| `POST /api/forwards/groups/:group/stop` | `api/forwards/` | `:group` (`@ApiParam`)⁸ | `GroupActionResponseDto` (`200`) | `FORWARD_GROUP_STOPPER` (`ForwardGroupStopper`) | — |
| `POST /api/forwards/groups/:group/start` | `api/forwards/` | `:group` (`@ApiParam`)⁸ | `GroupActionResponseDto` (`200`) | `FORWARD_GROUP_STARTER` (`ForwardGroupStarter`) | — |
| `GET /api/runtime` | `api/runtime/` | — (no input) | `RuntimeInfoResponseDto` | `RUNTIME_INFO_READER` + `CLOCK_READER` + `PROCESS_READER` | `buildRuntimeInfo` (`uptimeSeconds`) |
| `GET /api/config/export` | `api/config/` | — (no input) | `ConfigExportResponseDto` | `CONFIG_EXPORT_READER` + `CLOCK_READER` | `buildExportedConfig` (`exportedAt`) |
| `POST /api/config/plan` | `api/config/` | `ConfigPlanBodyDto`⁹ | `ConfigPlanResponseDto` (`200`) | `CONFIG_PLAN_READER` + `CLOCK_READER` | `buildConfigPlan` (`generatedAt`) |
| `POST /api/config/import` | `api/config/` | `ConfigImportBodyDto`¹⁰ | `ConfigImportResponseDto` (`200`) / `ConfigImportErrorResponseDto` (`422`)¹⁰ | `CONFIG_IMPORTER` (`ConfigImporter`) | — |
| `POST /api/config/apply` | `api/config/` | `ConfigApplyBodyDto`¹¹ | `ConfigApplyResponseDto` (`200`)¹¹ | `CONFIG_APPLIER` (`ConfigApplier`) + `CLOCK_READER` | `appliedAt` + `plan.generatedAt` |
| `GET /api/connections` | `api/connections/` | — (no input) | `ConnectionsResponseDto` | `CONNECTIONS_READER` + `CLOCK_READER` | `buildLiveConnections` (`generatedAt`) |

Every migrated endpoint has a byte-for-byte Express↔Nest parity test
(`*.integration.test.ts`) covering empty/default and (where relevant) seeded
state plus documented error cases; none normalizes or strips a volatile field.

Notes: ¹ `/health` is a scaffold liveness probe **outside the `/api` contract**;
it returns a typed constant from `HealthService` (no domain/runtime data to map),
so it needs no response-DTO mapper. ² `GET /api/activity`'s query is pure silent
coercion-with-fallback (always `200`) — a transform-only DTO would add ceremony
and risk parity drift, so the coercion stays endpoint-local in the service
(documented parity exception); its **response** is still mapped. ³ `204`-empty
responses have no JSON body, so they have no response DTO (the absent body is the
response, matching Express). ⁴ `CreateForwardRuleBodyDto`/`UpdateForwardRuleBodyDto`
are **documentation/typing** DTOs only — body validation is delegated to the shared
`validateForwardRule`/`validateForwardRulePatch` (`@portier/shared`) inside
`ForwardManager.addRule`/`updateRule` (the single contract validators, which also
preserve the partial-patch "absent field is not `undefined`" merge semantics), so
**no validation pipe runs on the body** (re-expressing the validators as
class-validator constraints would risk error-message/coercion/`id` drift — a
documented parity exception); the `:id` path param is a plain string with no
validation (Express does none — the manager's `requireRule` returns `404` for an
unknown id), documented via `@ApiParam`. Manager `ValidationError`/`NotFoundError`/`ConflictError`
are translated to `400`/`404`/`409` via the shared `mapManagerError`. A created
**enabled** rule starts its forwarder, and an **update** restarts a *running* rule
only when a forwarding field (`protocol`/`listenHost`/`listenPort`/`targetHost`/`targetPort`/`udpMode`)
changes — metadata-only updates (name/group/autostart) do not restart and a stopped
rule is not started (identical to Express); parity tests use `enabled:false`/stopped
rules (no sockets). ⁵ The **start** response body is a `ForwardStatus` (not a rule),
so it reuses the `ForwardStatusDto` schema (shared with `GET /api/status`) via a
small `toForwardStatusResponseDto` mapper; `@HttpCode(200)` is required because
NestJS defaults `POST`→`201`. **Delete** (`DELETE /api/forwards/:id`) returns `204` with no
body, stops a running forwarder first (runtime cleanup), rolls back on a persist
failure, and returns `404` for an unknown id — all inherited by delegating to
`ForwardManager.deleteRule`; parity tests delete `enabled:false` stopped rules (no
sockets) and cover success/404/repeat-delete and GET-after-DELETE. **Start**
(`POST /api/forwards/:id/start`) ⁵ opens the rule's forwarder and returns `200` +
its `ForwardStatus` (note: NestJS would default `POST`→`201`, so `@HttpCode(200)`
matches Express); it is **idempotent** (an already-running rule returns its status
without restarting), autostart/`enabled` is not a precondition, and an unknown id →
`404` — all inherited by delegating to `ForwardManager.startRule`. A started
status carries a volatile `startedAt`, so byte-for-byte parity uses the
**idempotent already-running** path on a **shared** manager (the listener is opened
once via the Test-A `startRuleStable` bind-retry helper, then both runtimes return
the identical pinned status); a separate single-runtime cold-start integration test
opens a real TCP and a real UDP listener through the Nest endpoint and stops it
(`manager.stopAll()`) afterwards — no leaked sockets, no fixed ports. **Stop**
(`POST /api/forwards/:id/stop`) is the natural pair — `200` + the rule's
`ForwardStatus` (`@HttpCode(200)`), idempotent (a not-running rule returns its
status without touching a socket), unknown id → `404`, all inherited by delegating
to `ForwardManager.stopRule`. A *stopped* status is fully deterministic
(`running:false`, zeroed counters, **no `startedAt`**), so — unlike start —
byte-for-byte parity needs no shared manager: the already-stopped path is
parity-tested with **zero sockets** (separate managers), and the running-rule stop
is parity-tested by starting each manager on its own free port (`startRuleStable`)
and stopping via the endpoint, with `GET /api/status`-after-stop parity and
`stopAll()` cleanup in `finally`. ⁶ **Reorder** (`POST /api/forwards/reorder`) is
the FIRST migrated endpoint with a **real validated body DTO** — its `{ ids:
string[] }` check is simple enough to re-express exactly in class-validator, so
`ReorderForwardRulesBodyDto` (`@IsArray` + `@IsString({ each: true })`, both
carrying the exact Express message `"ids must be an array of strings."`) runs
through `ApiValidationPipe` (not delegated to the manager). Returns `200` + the
full reordered `ForwardRuleResponse[]` (same shape/mapper as `GET /api/forwards`),
`@HttpCode(200)`. Reorder is **metadata only** (opens/closes no socket): the listed
ids go first, any unlisted rule keeps its relative order at the end (partial sets
allowed, duplicate ids tolerated, empty list is a no-op); an unknown id → `404`
(via `ForwardManager.reorderRules` → `mapManagerError`) and no reorder is persisted;
a persist failure rolls back the order. Parity-tested with **separate seeded
managers** (no sockets) for full/partial/duplicate/empty reorder + GET-after-reorder
+ unknown-id 404 + invalid-body 400. ⁷ **Diagnose** (`POST /api/forwards/:id/diagnose`)
is READ-ONLY — it inspects the rule + its running state (the narrow
`DIAGNOSTIC_READER` = `getRule` + `getStatus`), runs the shared `diagnoseRule` probes
(listen-bind / target-host DNS / target-connect + listen-host/LAN/privileged/common
advisories + UDP mode), and returns `200` + a `RuleDiagnosticsResultDto`
(`@HttpCode(200)`); an unknown id → `404` (the service throws `ApiNotFoundException`
directly, matching Express's INLINE 404 — `getRule` returns `undefined`, it does not
throw, so this is not a `mapManagerError` path). Its volatile `diagnosedAt` is pinned
for parity via the **shared `CLOCK_READER`** on the Nest side and the existing
`AppOptions.now` seam on the Express side — to enable that, `diagnoseRule` gained an
optional `now: Date = new Date()` param (behavior-preserving, mirroring the
`exportConfig(now)` seam; production output unchanged). Because diagnose probes the
network, byte-for-byte parity uses a **UDP** rule (its `target-connect` is always
`skip` — no TCP probe; `127.0.0.1` resolves instantly; only a transient UDP
listen-bind), a **shared** manager, **sequential** calls (so the listen-bind probes
never overlap), and the pinned clock — no field is stripped/normalized. The 404 path
(no probes, no `diagnosedAt`) is parity-tested with separate managers. ⁸ **Group
stop** (`POST /api/forwards/groups/:group/stop`) is the FIRST migrated group action —
it stops every rule sharing a `group` label (in rule order) via `stopGroup` and
returns `200` + a `GroupActionResponse` summary (`@HttpCode(200)`). The `:group` path
param is normalized exactly like Express (`decodeURIComponent(...).trim()`) and
validated by the SHARED `validateGroupName` (a `400` on empty/invalid — delegated, not
re-expressed in class-validator), and an empty match set → `404`; the service throws
the shared `ApiBadRequestException`/`ApiNotFoundException` directly (matching Express's
inline `400`/`404`). Group stop is **behaviour over rule metadata** — it never mutates
rule definitions/order/`enabled`/`group`. Because stopping a group of already-stopped
rules is a deterministic no-op (every result `skipped`/`not_running`, NO socket),
byte-for-byte parity needs no sockets: it uses **separate seeded `enabled:false`
managers** (success → all `not_running` skips + GET-unchanged, `404` unknown group,
`400` empty group, encoded-group normalization). `GroupActionResponse` has no volatile
field, so no clock seam is needed. **Group start** (`POST /api/forwards/groups/:group/start`,
Slice 22) is the lifecycle pair — same validation/normalization/response DTO
(`ForwardGroupStarter`/`FORWARD_GROUP_STARTER` + `startGroup`), `200` +
`GroupActionResponse`, `400`/`404` identically. It opens sockets for *stopped* rules,
but the `GroupActionResponse` carries NO volatile field (results are `started`/
`skipped`/`failed`, no `startedAt`), so byte-for-byte parity is achievable: the
`400`/`404`/encoded-normalization cases are socket-free (separate managers), the
success case uses the **idempotent already-running** path on a **shared** manager
(open the listener once via Test-A `startRuleStable`, then both runtimes return the
identical `already_running` skips — 1 socket, `stopAll()` cleanup), and a separate
single-runtime cold-start test starts a real listener through the Nest endpoint
(`started` + `GET /api/status` running) and `stopAll()`s afterwards. The group-action
pair (and the whole `/api/forwards` surface) is now migrated.

⁹ **Config plan** (`POST /api/config/plan`, Slice 23 — the first config-milestone
endpoint) is a **NON-MUTATING dry-run**: it diffs the desired config against the
current rules via the shared `buildConfigPlan` and returns the plan (operations,
summary, errors, warnings) — it never mutates rules, opens sockets, or emits
activity. The body validation is **delegated** to the service to match Express's
`"desired" in body` key-presence check exactly (`400 ["desired is required."]` only
when the `desired` key is absent; `desired: null` is allowed and surfaces as a plan
error, NOT a `400`, so the `ConfigPlanBodyDto` is documentation/typing only — no
validation pipe, since class-validator `@IsDefined` would reject `null` and diverge).
Its volatile `generatedAt` is pinned via the **shared `CLOCK_READER`** (Nest) and the
existing `AppOptions.now` seam (Express — `buildConfigPlan` already accepted a `now`
arg; the Express route now threads `options.now`, behavior-preserving). The response
is mapped through `toConfigPlanResponseDto` (a `structuredClone` deep copy — the plan
is deeply nested). Byte-for-byte parity uses a **shared** manager (plan is
non-mutating, so sharing is safe) + the pinned clock — `400`-missing-desired, empty
plan, add/update/unchanged drift (with a before/after manager-`listRules()` assertion
proving non-mutation), invalid-desired-rule → plan error (still `200`), and
`desired: null` → plan error — all match Express with no field stripped.

¹⁰ **Config import** (`POST /api/config/import`, Slice 24 — the first MUTATING config
endpoint) imports a config (`{mode, config}`) in `replace`/`merge` mode via a narrow
`ConfigImporter`/`CONFIG_IMPORTER` (`importConfig` + `listRules`; domain
`ForwardManager`), reusing the SHARED `ForwardManager.importConfig` (same
replace/merge mutation, duplicate-binding/merge-conflict rejection, **persist
rollback**, enabled-rule start, and `config.imported`/`config.import.failed` activity).
The body validation is **delegated** to inline service checks to match Express's
SHORT-CIRCUIT order exactly (`400 ["mode must be replace or merge."]` first, then
`400 ["config must be a valid Portier config object with version 1 and a rules
array."]` — class-validator would accumulate both and diverge), so `ConfigImportBodyDto`
is documentation/typing only. The **`200 {result, rules}`** (import result + the full
advisory-decorated rule list) and the **`422 {errors, result}`** (import errors WITH
the result — NOT the plain `{errors}` envelope) are **RETURNED with their status via
`@Res({ passthrough: true })`** (matching Express's `response.status(...).json(...)`),
since the `422` body carries `result` and so must NOT flow through the error-envelope
filter (`toApiError` strips extra fields); only the two `400`s are thrown as
`ApiBadRequestException` and DO flow through the filter. The import response has **no
volatile field**, so no clock seam is needed. All `422` paths reject BEFORE mutating.
Byte-for-byte parity uses **separate seeded managers** (import mutates) with fixed-id
`enabled:false` rules (deterministic, socket-free): `400` mode/config, `200` replace
into empty + over seeded (with `GET /api/forwards` + `GET /api/config/export`
state-after parity), `422` invalid rule (+ no-mutation assertion), and `422` duplicate
binding. Rollback on persist failure is inherited from `importConfig` (manager-tested,
Test-D) and unit-covered via a throwing fake (→ generic `500`).

¹¹ **Config apply** (`POST /api/config/apply`, Slice 25 — the FINAL config endpoint)
applies a desired config (`{desired, yes?, dryRun?}`) via a narrow
`ConfigApplier`/`CONFIG_APPLIER` (`listRules` + `importConfig`; domain `ForwardManager`;
a separate token from `CONFIG_IMPORTER` so each config endpoint is independently
overridable), running the SHARED `buildConfigPlan` + `buildApplyImportFromPlan` and, on
the non-dry-run drift path, the SHARED `ForwardManager.importConfig` (`replace`). It
mirrors Express's exact ordering: missing `desired` key → `400`; `plan.summary.hasErrors`
→ `200 ok:false`; `dryRun` → `200 ok:true` (BEFORE the destructive gate); `destructive
&& !yes` → `400`; `hasDrift` → replace import (+ a belt-and-suspenders import-error
guard → `200 ok:false` via `plan.errors`); else → `200 ok:true`. **All success/`ok:false`
outcomes are status `200`** (only the two gating errors are `400`), so the controller is
`@HttpCode(200)` and the service THROWS `ApiBadRequestException` for the `400`s (through
the shared `{errors}` filter) and RETURNS the `ConfigApplyResponse` for everything else —
**no `@Res`** is needed (unlike import's `422`). Body validation is **delegated** to inline
service checks (the `"desired" in body` key-presence check — `desired: null` → plan error,
not `400` — plus the destructive gate), so `ConfigApplyBodyDto` is documentation/typing
only. The response carries **two** volatile timestamps — the top-level `appliedAt` and the
embedded `plan.generatedAt` — both stamped from **one** clock instant: the Express route
threads `options.now` into both `appliedAt` and `buildConfigPlan`'s `now` (a
production-invisible refinement; production uses the real wall clock), and the Nest service
injects `CLOCK_READER`; parity pins the same clock in both runtimes (no field
stripped/normalized). Byte-for-byte parity uses **separate seeded managers** (apply
mutates) with fixed-id `enabled:false` rules (deterministic, socket-free) + the pinned
clock: `400` missing desired, dry-run (pinned `appliedAt`+`generatedAt`, no mutation),
apply-add (with `GET /api/forwards` + `GET /api/config/export` state-after parity),
destructive blocked without `yes` (state unchanged), destructive applied with `yes:true`
(rule removed), invalid desired rule → `ok:false` (no mutation), and no-drift → `ok:true`.
The import-error guard branch and a persist-failure re-throw (→ generic `500`) are
service unit-covered via a fake applier.

## Static client serving (Slice 26)

The Nest shadow runtime can serve the packaged web client with Express-equivalent
semantics (`server/sources/nest/static/static-serving.ts`):

- **Enable rule** mirrors Express: a static dir is usable only when it contains an
  `index.html` (`hasStaticClient`/`resolveStaticOptions`). A missing/absent dir is
  allowed — the API (and the `/api/*` JSON-404 envelope) stays fully usable.
- **Assets:** `configureStaticAssets(app, dir)` registers `app.useStaticAssets(dir)`
  (the `express.static` equivalent, pre-router) when enabled.
- **SPA fallback:** an unmatched **non-API** GET/HEAD route serves `index.html`. Nest
  surfaces unmatched routes as `NotFoundException` → the global
  `ApiErrorEnvelopeFilter`, so that filter (the single unmatched-route handler) owns
  the fallback, delegating the static decision to an injected `StaticFallback`
  (`STATIC_FALLBACK` token). The `/api/*` envelope branch runs first and is untouched.
- **Shadow-only & default-off:** `STATIC_FALLBACK` defaults to `disabledStaticFallback`
  (the scaffold wires no static dir), so the live scaffold serves no static assets and
  the API works with no client build. Static serving is proven via tests + the reusable
  `configureStaticAssets` helper; live wiring into `npm run start:nest` is deferred to
  the runtime-switch slice (no `main.ts`/`bootstrap.ts` change).
- **Parity:** Express↔Nest raw status+body parity for `/`, SPA routes, a real asset, a
  missing asset (→ index.html, matching Express), and an unmatched `/api/*` route (→ the
  JSON envelope). When static is disabled, both keep the API usable and serve no SPA
  index — the non-API 404 *body shape* still differs (Express HTML vs NestJS JSON), the
  documented pre-existing scaffold boundary.
- **OpenAPI:** static serving adds **no** routes; `docs/api/openapi.json` is unchanged
  and a generator test asserts the doc contains only `/health` + `/api/*` paths.

With static serving migrated, **v1.14 is ready for the final migration audit /
switch-readiness review**; the live runtime switch (making NestJS the default) is a
deliberate, separately-validated later step.

## DTO / OpenAPI schema ownership

Each feature owns its OpenAPI schema classes. Metadata-only decorated
`@ApiProperty` classes live in feature-local `*.schema.ts` files inside the feature
folder (e.g. `api/forwards/forward-rule.schema.ts`, `api/config/config-plan.schema.ts`,
`api/ports/port-advisory.schema.ts`); the response **mappers** (`to*ResponseDto`) stay
in the sibling `*.response.dto.ts` (which re-exports the schema class), and validated
request bodies (class-validator, instantiated by the pipe) stay in `*.body.dto.ts`.

`common/` holds **only genuinely cross-feature** schemas — currently just
`api-error.schema.ts` (`ApiErrorResponseDto`, the shared `{ errors }` envelope). Do
**not** reintroduce a single large centralized schema bucket; a feature-owned schema
belongs in its feature folder, even when another feature imports it (e.g.
`ForwardRuleDto` is forwards-owned and imported by config-export). The `*.schema.ts`
files are metadata-only (never instantiated) and are coverage-excluded by the
`sources/nest/**/*.schema.ts` glob; their generated schemas are asserted through the
OpenAPI generator tests, and the mappers they pair with are 100% covered.

## OpenAPI artifact placement

`npm run generate:apidoc` generates the OpenAPI document from the NestJS
controller/DTO metadata (offline — no listener) and writes two artifacts:

- **Primary (server-owned, generated):** `server/build/api/openapi.json` — under the
  gitignored `build/` output; this is the generator's primary target.
- **Docs copy (tracked, reviewed):** `docs/api/openapi.json` — synced byte-for-byte
  from the primary artifact. A generator test (`openapi.test.ts`) is the drift guard:
  it fails if the tracked copy is stale (run `npm run generate:apidoc` and commit).

The serialized document is deterministic — `components.schemas` is sorted by name in
`serializeOpenApiDocument` so output is stable regardless of module-evaluation order
(the schema map order is cosmetic; `$ref`s resolve by name).

For packaging, `copyOpenApiToRelease(releaseDir, sourcePath)`
(`sources/nest/openapi/openapi.ts`) copies the already-generated primary artifact to
`<releaseDir>/api/openapi.json` without regenerating. A packaging step should call it
with `resolveOpenApiPaths().primary` as the source after generation has run; the
OpenAPI generator never depends on a release directory existing.

## Legacy Express server (future work)

When NestJS eventually becomes the default runtime, the existing Express
implementation (`sources/index.ts` + `sources/api.ts` and its supporting modules)
will be **preserved under a `legacy/` directory**, not deleted — so the runtime
switch keeps a clear rollback path. That move is **future work**: this codebase has
not moved any Express code yet, and the Express server remains the active runtime.
Until then, do not delete the Express server, and keep the reusable domain modules
(`forward-manager.ts`, `config-plan.ts`, `connections-snapshot.ts`, `diagnose.ts`,
the activity store, etc.) shared by both runtimes.

Layout:

```text
sources/nest/
  main.ts                       # logic-free process entry — excluded from coverage (importing it starts a listener)
  bootstrap.ts                  # bootstrap(listen) + reportBootstrapFailure(error) — fully covered
  nest-options.ts               # resolveNestListenOptions(env) — fully covered
  app.factory.ts                # createNestApp() — builds the app without listening (shared by bootstrap + tests)
  app.module.ts                 # root module; imports feature modules; registers the global error filter
  health/                       # GET /health (controller → service → module)
  api/                          # each feature: controller → service → reader/writer; *.schema.ts (OpenAPI classes, coverage-excluded);
                                #   *.response.dto.ts (mappers, covered); *.body.dto.ts (validated request DTOs)
    ports/                      # GET /api/ports/advisory — request: *.query.dto + pipe; port-advisory.schema.ts; response: *.response.dto + mapper
    activity/                   # GET + DELETE /api/activity (ACTIVITY_STORE + ActivityReader/ActivityClearer in activity.reader.ts; activity.schema.ts)
    status/                     # GET /api/status (injected STATUS_READER; uses forwards/forward-status.schema.ts)
    forwards/                   # forwards CRUD/lifecycle/group/diagnose; forward-rule(.body).schema.ts, forward-status.schema.ts, group-action.schema.ts, rule-diagnostics.schema.ts
    runtime/                    # GET /api/runtime (volatile: CLOCK_READER + PROCESS_READER + RUNTIME_INFO_READER; shared buildRuntimeInfo; runtime.schema.ts)
    config/                     # GET /api/config/export, POST /api/config/{plan,import,apply} (CONFIG_EXPORT/PLAN_READER, CONFIG_IMPORTER/APPLIER + shared CLOCK_READER; config-{export,plan,import,apply}.schema.ts)
    connections/                # GET /api/connections (volatile generatedAt: CONNECTIONS_READER + shared CLOCK_READER; shared buildLiveConnections; connections.schema.ts)
  static/
    static-serving.ts           # resolveStaticOptions/hasStaticClient/configureStaticAssets (useStaticAssets) + StaticFallback/STATIC_FALLBACK (SPA index fallback)
  common/
    clock.reader.ts              # ClockReader/CLOCK_READER/defaultClockReader — shared live-clock provider for volatile-timestamp endpoints
    api-error-envelope.ts        # pure toApiError(exception) + isApiPath — the /api error mapping
    api-error-envelope.filter.ts # global catch-all filter: /api/* → envelope, non-API → SPA index fallback (STATIC_FALLBACK) else NestJS default
    api-errors.ts                # ApiBadRequestException(string[]) — controllers raise this, not a literal
    api-validation.pipe.ts       # ApiValidationPipe(Dto) — class-validator/-transformer → ApiBadRequestException
    api-error.schema.ts          # ApiErrorResponseDto — the ONLY common (cross-feature) OpenAPI schema; metadata-only, coverage-excluded
  openapi/
    openapi.ts                  # generate/serialize(sorted schemas)/resolveOpenApiPaths/writeOpenApiArtifacts/copyOpenApiToRelease — fully covered
    generate.ts                 # logic-free `npm run generate:apidoc` entry (writes server/build/api/openapi.json + docs copy) — coverage-excluded
  testing/
    api-parity.ts               # Express↔Nest parity harness (boot, fetch, deterministic compare)
```

### API documentation (generated OpenAPI)

The API documentation is **generated from the NestJS controller/DTO metadata**
(`@ApiTags`/`@ApiOperation`/`@Api*Response`/`@ApiProperty`) — it is not
hand-written. Generate it with:

```bash
npm run generate:apidoc          # from the repo root (delegates to -w server)
npm run generate:apidoc -w server
```

- **Output** — see *OpenAPI artifact placement* above: the primary artifact is the
  server-owned `server/build/api/openapi.json`, synced byte-for-byte to the tracked,
  reviewable `docs/api/openapi.json`. Regenerate whenever a migrated endpoint or its
  DTOs change; an `openapi.test.ts` drift guard fails CI if the tracked copy is stale
  (the message tells you to run `generate:apidoc`).
- Generation is **offline** — it inspects the Nest app's metadata via
  `SwaggerModule.createDocument` without listening on a socket, **always closes
  the app cleanly** afterwards (no leaked listener — covered by a test), and
  **does not change Express** (the default runtime) or switch the active runtime.
  No Swagger UI / `/docs` route is exposed.
- The document is **deterministic** (schemas sorted by name, stable JSON + trailing
  newline), so regeneration is idempotent regardless of module-evaluation order.
- **DTOs are the OpenAPI schema source.** The decorated schema classes live in
  feature-local `*.schema.ts` files (see *DTO / OpenAPI schema ownership* above),
  each `implements` its `@portier/shared` type so a contract drift is a compile
  error; the response **mappers** stay in each feature's `*.response.dto.ts` (the
  covered logic). The only common schema is `ApiErrorResponseDto` (`{ errors: string[] }`).

Every `sources/nest/` file with executable logic is covered at **100%**
(statements/branches/functions). The only coverage-excluded nest files are the
two logic-free process entries (`main.ts`, `openapi/generate.ts` — their helpers
are fully covered) and the metadata-only `common/api-schemas.ts` (decorated
`@ApiProperty` schema classes that are never instantiated — no executable logic).
This mirrors how `sources/index.ts` is excluded.

## Scripts

```bash
npm run dev        -w server   # active Express server (watch)
npm run build      -w server   # tsc → build/ (includes the nest scaffold)
npm run typecheck  -w server
npm run test       -w server   # all server tests (Express + nest scaffold)

npm run build:nest -w server      # build (alias — nest is part of the unified server build)
npm run start:nest -w server      # run the NestJS scaffold (scaffold only — not the active server)
npm run generate:apidoc -w server # regenerate docs/api/openapi.json from Nest metadata
npm run test:nest  -w server   # run only the nest scaffold tests
```

`start:nest` binds `127.0.0.1` only and defaults to port `47832` (override with
`PORTIER_NEST_PORT` / `PORTIER_NEST_HOST`) so it never collides with the
management server's default `127.0.0.1:47831`.

## Migration rules

- Controllers are **transport adapters only**; behaviour lives in services;
  features compose as modules. Controllers must **not hand-roll the `{ errors }`
  envelope** — raise `ApiBadRequestException(string[])` (or another API exception)
  and let the shared `ApiErrorEnvelopeFilter` produce the contract shape.
- **Every migrated endpoint has explicit DTOs:** a **request DTO** for any
  query/route/body input (validated via `ApiValidationPipe(Dto)`, `class-validator`
  /`class-transformer`, matching Express coercion exactly), and an **explicit
  response DTO** always — the boundary between domain/runtime data and HTTP JSON.
  Controllers map the service result through a small pure `to*ResponseDto` mapper
  (a fresh copy that preserves the Express JSON shape byte-for-byte) rather than
  returning raw domain objects. The `@portier/shared` types are the REST contract
  shape, so the mapper is a structural copy (it would become an explicit field
  pick only to hide a future internal field).
- Documented exceptions: an endpoint with **no input** needs no request DTO; a
  `204`-empty response (`DELETE /api/activity`) has **no body → no response DTO**;
  and a query that is pure silent coercion-with-fallback (always `200`, e.g.
  `GET /api/activity`) keeps its coercion endpoint-local in the service rather
  than adding a transform-only DTO. The request validation pipe takes the DTO
  class **explicitly** (esbuild doesn't emit `design:paramtypes`) and throws
  `ApiBadRequestException` → `400 { errors }`. Mappers/DTOs are 100% covered.
- **DTOs are also the API-doc source.** Each migrated endpoint must carry enough
  Swagger metadata to generate good OpenAPI docs in the **same slice** (no
  "document it later"): `@ApiTags`/`@ApiOperation` on the controller, an
  `@Api*Response` for each status (`@ApiOkResponse({ type, isArray })`,
  `@ApiNoContentResponse` for `204`, `@ApiBadRequestResponse({ type: ApiErrorResponseDto })`
  for validation errors), `@ApiQuery` for query params (the esbuild/tsx transform
  does not emit the `@Query` DTO's reflected type, so query params are documented
  explicitly), and a decorated schema class (in `common/api-schemas.ts`,
  `@ApiProperty` with **explicit** types, `implements` the shared type) for every
  response/body shape. After changing an endpoint or its DTOs, run
  `npm run generate:apidoc` and commit the updated `docs/api/openapi.json` (the
  drift test enforces this).
- **API contract parity is mandatory** — every migration step keeps
  `npm run validate:contract` green (TS↔Go), and no public API path/DTO is
  renamed for cosmetic reasons. Each migrated endpoint is also checked
  **byte-for-byte against the existing Express route** with the parity harness
  (`sources/nest/testing/api-parity.ts`).
- The existing Express server **remains the active runtime** until a NestJS
  replacement is explicitly validated (contract + runtime smoke + E2E).
- No endpoint is migrated without tests and contract validation.
- A **write/mutation** endpoint adds a narrow write interface (e.g.
  `ActivityClearer { clear(): void }`) alongside the read one; mutation parity is
  proven against the **same seeded store instance** shared with Express, and a
  subsequent read confirms the mutation. Match the Express HTTP status with
  `@HttpCode(...)` (Nest defaults `DELETE` to `200`; a `204`-empty Express route
  needs `@HttpCode(204)` + a `void` return). No DTO when the endpoint has no
  query/body input.
- An endpoint with **volatile fields** (timestamps, uptime, pid) gets its volatile
  values from **narrow injected readers** (`ClockReader`/`CLOCK_READER`,
  `ProcessReader`/`PROCESS_READER`, `RuntimeInfoReader`/`RUNTIME_INFO_READER`),
  never read inline, so the service stays pure. Parity is **byte-for-byte with no
  field stripping/normalization**: extract a **shared pure builder** that both the
  Express route and the Nest service call (e.g. `buildRuntimeInfo` in
  `sources/runtime-info.ts`) so they cannot drift, and boot **both** apps with the
  **same fixed volatile inputs** (a minimal optional, production-invisible clock
  seam on Express `AppOptions` — like the existing `runtimeInfo.startedAt`) so the
  otherwise-volatile field is deterministic. Never strip a field before comparing,
  and never use fake timers / mutate global process state. `ClockReader` is generic
  (reused by `/api/connections` `generatedAt`, `/api/config/export` `exportedAt`,
  `/api/config/plan` `generatedAt`, `/api/forwards/:id/diagnose` `diagnosedAt`, and
  `/api/config/apply`'s `appliedAt` + embedded `plan.generatedAt` — one instant pins both).
- An endpoint that needs runtime/domain state is wired through a **narrow
  injection token + interface** (e.g. `ACTIVITY_STORE`/`ActivityReader`,
  `STATUS_READER`/`StatusReader`), with a fake/seeded instance in tests —
  **never** a real forwarding/socket runtime in endpoint tests (seed only
  **stopped** rules, no listeners). Production defaults to a trivial empty
  reader; tests bind the real domain object (`ActivityStore`, `ForwardManager`)
  to the token — seed a mutable default via `app.get(TOKEN)`, or use
  `@nestjs/testing` `overrideProvider` when the dependency can't be seeded
  post-construction (e.g. a store-backed `ForwardManager`).
- **New migration code reaches 100% meaningful coverage** — keep startup glue in
  covered helpers (not the process entry) and never broadly exclude new Nest code
  or lower a coverage gate to pass.
- Build output stays under `build/`, never `dist/`.
