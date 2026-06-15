# @portier/server

The Portier **TypeScript runtime/fallback server**: the REST management API, config
persistence, and forwarding lifecycle for the Node-based runtime. The native Go
service (`service/`) remains the preferred runtime; this server is the Node
fallback and the reference TypeScript implementation that `validate:contract`
checks for parity against Go.

## Two implementations during the v1.14 migration

v1.14 migrates this server from a single Express app to a NestJS
modules/controllers/services structure. The migration is **incremental and
reversible** — both implementations currently coexist:

| | Active runtime | Migration scaffold |
|---|---|---|
| Entry point | `sources/index.ts` | `sources/nest/main.ts` |
| Composition | `sources/api.ts` (`createApp`) | `sources/nest/app.module.ts` |
| Framework | Express 5 | NestJS 11 (Express 5 platform) |
| Status | **Active** — serves the real API, CLI, and web UI | **Scaffold only** — not wired into the runtime |
| Scripts | `dev`, `build`, `start` (via root) | `start:nest`, `build:nest`, `test:nest` |

The **existing Express server is the active server.** The NestJS scaffold under
`sources/nest/` does not replace it, is not started by the normal runtime, and
does not change any REST/API behavior. Endpoint migration happens in later v1.14
slices, each guarded by `npm run validate:contract`.

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
Slice 16) — plus the first lifecycle endpoint, **start**
(`POST /api/forwards/:id/start`, Slice 17). Every endpoint below is **shadow-only** — served by the
Nest app only under `npm run start:nest`; the Express server (`sources/index.ts` +
`sources/api.ts`) remains the **default active runtime** and serves all routes
unchanged. `validate:contract` is 234/234.

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
| `GET /api/runtime` | `api/runtime/` | — (no input) | `RuntimeInfoResponseDto` | `RUNTIME_INFO_READER` + `CLOCK_READER` + `PROCESS_READER` | `buildRuntimeInfo` (`uptimeSeconds`) |
| `GET /api/config/export` | `api/config/` | — (no input) | `ConfigExportResponseDto` | `CONFIG_EXPORT_READER` + `CLOCK_READER` | `buildExportedConfig` (`exportedAt`) |
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
(`manager.stopAll()`) afterwards — no leaked sockets, no fixed ports.

**Deferred (Milestone 3+, write/lifecycle/static):** rule stop, reorder, group
actions, diagnose, `POST /api/config/import`, `POST /api/config/plan`/`apply`, and
static client serving — all stay with Express. The next slice continues
**write/lifecycle** with `POST /api/forwards/:id/stop` (the natural pair).

Layout:

```text
sources/nest/
  main.ts                       # logic-free process entry — excluded from coverage (importing it starts a listener)
  bootstrap.ts                  # bootstrap(listen) + reportBootstrapFailure(error) — fully covered
  nest-options.ts               # resolveNestListenOptions(env) — fully covered
  app.factory.ts                # createNestApp() — builds the app without listening (shared by bootstrap + tests)
  app.module.ts                 # root module; imports feature modules; registers the global error filter
  health/                       # GET /health (controller → service → module)
  api/
    ports/                      # GET /api/ports/advisory — request: *.query.dto + pipe; response: *.response.dto + mapper
    activity/                   # GET + DELETE /api/activity (ACTIVITY_STORE + ActivityReader/ActivityClearer in activity.reader.ts; response: *.response.dto + mapper)
    status/                     # GET /api/status (injected STATUS_READER; response: *.response.dto + mapper)
    forwards/                   # GET /api/forwards (injected FORWARDS_READER; response: *.response.dto + mapper)
    runtime/                    # GET /api/runtime (volatile: CLOCK_READER + PROCESS_READER + RUNTIME_INFO_READER; shared buildRuntimeInfo; response: *.response.dto + mapper)
    config/                     # GET /api/config/export (volatile exportedAt: CONFIG_EXPORT_READER + shared CLOCK_READER; shared buildExportedConfig; response: *.response.dto + mapper)
    connections/                # GET /api/connections (volatile generatedAt: CONNECTIONS_READER + shared CLOCK_READER; shared buildLiveConnections; response: *.response.dto + mapper)
  common/
    clock.reader.ts              # ClockReader/CLOCK_READER/defaultClockReader — shared live-clock provider for volatile-timestamp endpoints
    api-error-envelope.ts        # pure toApiError(exception) + isApiPath — the /api error mapping
    api-error-envelope.filter.ts # global catch-all filter: /api/* → envelope, non-API → NestJS default
    api-errors.ts                # ApiBadRequestException(string[]) — controllers raise this, not a literal
    api-validation.pipe.ts       # ApiValidationPipe(Dto) — class-validator/-transformer → ApiBadRequestException
    api-schemas.ts               # @ApiProperty OpenAPI schema classes (ApiErrorResponseDto + response DTOs + item shapes) — metadata-only, coverage-excluded
  openapi/
    openapi.ts                  # generateOpenApiDocument()/serialize/write helpers + OPENAPI_OUTPUT_PATH — fully covered
    generate.ts                 # logic-free `npm run generate:apidoc` entry — coverage-excluded
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

- **Output: `docs/api/openapi.json`** — an OpenAPI 3 document, **tracked in git**
  so it can be reviewed/versioned. Regenerate it whenever a migrated endpoint or
  its DTOs change; an `openapi.test.ts` drift guard fails CI if the tracked file
  is stale (the message tells you to run `generate:apidoc`).
- Generation is **offline** — it inspects the Nest app's metadata via
  `SwaggerModule.createDocument` without listening on a socket, **always closes
  the app cleanly** afterwards (no leaked listener — covered by a test), and
  **does not change Express** (the default runtime) or switch the active runtime.
  No Swagger UI / `/docs` route is exposed.
- The document is **deterministic** (stable JSON + trailing newline), so
  regeneration is idempotent.
- **Response DTOs are the OpenAPI schema source.** The decorated schema classes
  live in `common/api-schemas.ts` (`@ApiProperty`, each `implements` its
  `@portier/shared` type so a contract drift is a compile error); the response
  **mappers** stay in each feature's `*.response.dto.ts` (the covered logic). The
  error envelope is `ApiErrorResponseDto` (`{ errors: string[] }`).

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
  (reused by `/api/connections` `generatedAt`, `/api/config/export` `exportedAt`).
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
