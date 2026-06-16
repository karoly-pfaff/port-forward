# @portier/server

The Portier **TypeScript runtime/fallback server**: the REST management API, config
persistence, and forwarding lifecycle for the Node-based runtime. It is built with
**NestJS** and is the single TypeScript server implementation. The native Go service
(`service/`) remains the preferred packaged runtime; this server is the Node fallback
and the reference TypeScript implementation that `npm run validate:contract` checks
for parity against Go (**234/234**).

`sources/index.ts` is the entry point: it resolves options, loads the config and
starts enabled forwarders, builds the NestJS app with its providers wired to the
**live** `ForwardManager`/`ActivityStore`/runtime-info/static client via
`createNestApp(runtime)`, binds the HTTP server with socket tracking, and shuts down
gracefully (stop forwarders, flush config, destroy sockets, close the app).

## Live dependency wiring

Each feature provider sources its dependency from the global `RuntimeContextModule`
(`APP_RUNTIME`): the running server supplies an `AppRuntime` (the live manager +
activity store + runtime-info reader + static fallback) so every reader/writer token
resolves to the live `ForwardManager`, `ACTIVITY_STORE` to the live store, etc. When
no runtime is supplied — OpenAPI generation and tests — `APP_RUNTIME` is `null` and
the providers fall back to empty/in-memory defaults (tests override the specific
tokens they seed via `@nestjs/testing` `overrideProvider`). The static `AppModule`
(`APP_RUNTIME = null`) is the root used by docs/tests; `createLiveAppModule(runtime)`
returns the live root on a distinct `LiveAppModule` class (so Nest does not merge
`AppModule`'s static `@Module` metadata and double-register the global). `createNestApp`
is the single app factory.

## Health probe

`GET /health` → `{ ok: true, server: "node", name: "Portier" }` is a liveness probe
that needs no runtime manager. It is intentionally **outside the frozen `/api`
contract** (the documented `GET /api/health` is "TypeScript server: not implemented",
and aligning it would be a deliberate contract-tested change). The error model for
every `/api/*` route is the Portier `{ "errors": ["..."] }` envelope via the global
`ApiErrorEnvelopeFilter`: unmatched routes → `404 ["API route was not found."]`,
controller-raised `400`s carry their messages, unknown errors →
`500 ["Internal server error."]` (no leak); non-API routes keep NestJS's default
error shape. Controllers raise `ApiBadRequestException(string[])` (etc.) rather than
hand-rolling the envelope.

## Endpoint inventory

Every `/api` endpoint below is served by the NestJS server. `validate:contract` is
the parity guard against the Go service (**234/234**). Each endpoint has unit tests
(controller/service/reader/writer/mapper) plus an integration test that boots the
real Nest app; the two app-level integration tests (`app/app.integration.test.ts`
with `APP_RUNTIME = null`, `app/live-runtime.integration.test.ts` with a live seeded
runtime) exercise both branches of every feature provider.

| Endpoint | Module | Request DTO | Response DTO | Provider token | Builder / volatile |
|---|---|---|---|---|---|
| `GET /health` | `health/` | — (no input) | typed constant¹ | — | — |
| `GET /api/ports/advisory` | `api/ports/` | `PortsAdvisoryQueryDto` + pipe | `PortsAdvisoryResponseDto` | — (pure shared logic) | — |
| `GET /api/activity` | `api/activity/` | endpoint-local coercion² | `ActivityListResponseDto` | `ACTIVITY_STORE` (`ActivityReader`) | — |
| `DELETE /api/activity` | `api/activity/` | — (no input) | — (`204` no body)³ | `ACTIVITY_STORE` (`ActivityClearer`) | — |
| `GET /api/status` | `api/status/` | — (no input) | `StatusListResponseDto` | `STATUS_READER` | — |
| `GET /api/forwards` | `api/forwards/` | — (no input) | `ForwardsListResponseDto` | `FORWARDS_READER` | — |
| `POST /api/forwards` | `api/forwards/` | `CreateForwardRuleBodyDto`⁴ | `ForwardRuleResponseDto` (`201`) | `FORWARD_RULE_CREATOR` | — |
| `PATCH /api/forwards/:id` | `api/forwards/` | `:id` + `UpdateForwardRuleBodyDto`⁴ | `ForwardRuleResponseDto` (`200`) | `FORWARD_RULE_UPDATER` | — |
| `DELETE /api/forwards/:id` | `api/forwards/` | `:id`⁴ | — (`204` no body)³ | `FORWARD_RULE_DELETER` | — |
| `POST /api/forwards/:id/start` | `api/forwards/` | `:id`⁴ | `ForwardStatusDto` (`200`)⁵ | `FORWARD_RULE_STARTER` | — |
| `POST /api/forwards/:id/stop` | `api/forwards/` | `:id`⁴ | `ForwardStatusDto` (`200`)⁵ | `FORWARD_RULE_STOPPER` | — |
| `POST /api/forwards/reorder` | `api/forwards/` | `ReorderForwardRulesBodyDto` + pipe⁶ | `ForwardRuleResponseDto[]` (`200`) | `FORWARD_RULES_REORDERER` | — |
| `POST /api/forwards/:id/diagnose` | `api/forwards/` | `:id`⁴ | `RuleDiagnosticsResultDto` (`200`) | `DIAGNOSTIC_READER` + `CLOCK_READER` | `diagnoseRule` (`diagnosedAt`) |
| `POST /api/forwards/groups/:group/stop` | `api/forwards/` | `:group`⁷ | `GroupActionResponseDto` (`200`) | `FORWARD_GROUP_STOPPER` | — |
| `POST /api/forwards/groups/:group/start` | `api/forwards/` | `:group`⁷ | `GroupActionResponseDto` (`200`) | `FORWARD_GROUP_STARTER` | — |
| `GET /api/runtime` | `api/runtime/` | — (no input) | `RuntimeInfoResponseDto` | `RUNTIME_INFO_READER` + `CLOCK_READER` + `PROCESS_READER` | `buildRuntimeInfo` (`uptimeSeconds`) |
| `GET /api/config/export` | `api/config/` | — (no input) | `ConfigExportResponseDto` | `CONFIG_EXPORT_READER` + `CLOCK_READER` + `CONFIG_EXPORT_RECORDER`¹¹ | `buildExportedConfig` (`exportedAt`) |
| `POST /api/config/plan` | `api/config/` | `ConfigPlanBodyDto`⁸ | `ConfigPlanResponseDto` (`200`) | `CONFIG_PLAN_READER` + `CLOCK_READER` | `buildConfigPlan` (`generatedAt`) |
| `POST /api/config/import` | `api/config/` | `ConfigImportBodyDto`⁸ | `ConfigImportResponseDto` (`200`) / `ConfigImportErrorResponseDto` (`422`)⁹ | `CONFIG_IMPORTER` | — |
| `POST /api/config/apply` | `api/config/` | `ConfigApplyBodyDto`⁸ | `ConfigApplyResponseDto` (`200`)¹⁰ | `CONFIG_APPLIER` + `CLOCK_READER` | `appliedAt` + `plan.generatedAt` |
| `GET /api/connections` | `api/connections/` | — (no input) | `ConnectionsResponseDto` | `CONNECTIONS_READER` + `CLOCK_READER` | `buildLiveConnections` (`generatedAt`) |

Notes: ¹ `/health` returns a typed constant from `HealthService` (no domain data to
map → no response-DTO mapper). ² `GET /api/activity`'s query is pure silent
coercion-with-fallback (always `200`), kept endpoint-local in the service rather
than a transform-only DTO; its **response** is still mapped. ³ `204`-empty responses
have no body → no response DTO. ⁴ `Create`/`Update` body DTOs are
**documentation/typing** only — body validation is delegated to the shared
`validateForwardRule`/`validateForwardRulePatch` inside `ForwardManager.addRule`/
`updateRule` (the single contract validators, preserving partial-patch merge
semantics); the `:id` path param is a plain string (`requireRule` returns `404` for
an unknown id), documented via `@ApiParam`. Manager `ValidationError`/`NotFoundError`/
`ConflictError` map to `400`/`404`/`409` via the shared `mapManagerError`. A created
**enabled** rule starts its forwarder; an **update** restarts a *running* rule only
when a forwarding field changes. ⁵ The **start**/**stop** response body is a
`ForwardStatus` (reusing `ForwardStatusDto`); `@HttpCode(200)` is required (NestJS
defaults `POST`→`201`). Start is idempotent; a stopped status is fully deterministic
(`running:false`, zeroed counters, no `startedAt`). ⁶ **Reorder** has a real
validated body DTO (`@IsArray` + `@IsString({ each: true })`, message `"ids must be
an array of strings."`) through `ApiValidationPipe`; it is metadata-only (no
sockets) — listed ids first, unlisted rules appended in prior order; unknown id →
`404`, persist failure rolls back. ⁷ Group actions normalize `:group`
(`decodeURIComponent(...).trim()`), validate with the shared `validateGroupName`
(`400` on empty/invalid), and `404` on an empty match set; they are behaviour over
rule metadata (never mutate definitions/order/`enabled`/`group`). ⁸ `ConfigPlanBodyDto`/
`ConfigImportBodyDto`/`ConfigApplyBodyDto` are documentation/typing only — body
validation is delegated to inline service checks (key-presence / short-circuit
ordering that class-validator would diverge from). ⁹ Config import returns
`200 {result, rules}` or `422 {errors, result}` via `@Res({ passthrough: true })`
(the `422` carries `result` alongside errors, so it must not flow through the
`{errors}` filter); the two `400`s (bad mode/config) are thrown. Rollback on persist
failure is inherited from `ForwardManager.importConfig`. ¹⁰ Config apply mirrors the
exact gate order (missing `desired` → `400`; `hasErrors` → `200 ok:false`; `dryRun`
→ `200 ok:true` before the destructive gate; `destructive && !yes` → `400`; drift →
`replace` import; else `200 ok:true`) — all non-error outcomes are `200`, so the
controller is `@HttpCode(200)` (no `@Res`). Its two volatile timestamps (`appliedAt`
+ embedded `plan.generatedAt`) are stamped from **one** clock instant. ¹¹ `GET
/api/config/export` records exactly one `config.exported` activity event per
successful export via the narrow `CONFIG_EXPORT_RECORDER` (the live runtime binds an
activity-store-backed recorder; the static `AppModule`/OpenAPI/tests bind a no-op).
The reader (`listRules`) and the `buildExportedConfig` builder stay pure — the side
effect lives only in the recorder, sharing the canonical `configExportedActivityEvent`
payload with `ForwardManager.exportConfig` so they cannot drift.

**Volatile fields** (`uptimeSeconds`, `exportedAt`, `generatedAt`, `diagnosedAt`,
`appliedAt`) come from narrow injected readers — `CLOCK_READER` (generic, reused
across all timestamp endpoints), `PROCESS_READER`, `RUNTIME_INFO_READER` — never read
inline, so services stay pure and deterministically testable.

## Static client serving

`static/static-serving.ts` serves the packaged web client:

- **Enable rule:** a static dir is usable only when it contains an `index.html`
  (`hasStaticClient`/`resolveStaticOptions`). A missing/absent dir is allowed — the
  API (and the `/api/*` JSON-404 envelope) stays fully usable.
- **Assets:** `configureStaticAssets(app, dir)` registers `app.useStaticAssets(dir)`
  (pre-router) when enabled.
- **SPA fallback:** an unmatched **non-API** GET/HEAD route serves `index.html`. Nest
  surfaces unmatched routes as `NotFoundException` → the global
  `ApiErrorEnvelopeFilter`, so that filter (the single unmatched-route handler) owns
  the SPA fallback, delegating the decision to an injected `StaticFallback`
  (`STATIC_FALLBACK`). The `/api/*` envelope branch runs first.
- **Live wiring & default-off:** `sources/index.ts` builds the runtime's
  `staticFallback` (`createStaticFallback(staticClientDir)`) and calls
  `configureStaticAssets`. With no static dir wired (`STATIC_FALLBACK =
  disabledStaticFallback`), no static assets are served and the API works with no
  client build.
- Static serving adds **no** OpenAPI routes (the generated doc is only `/health` +
  `/api/*`).

## DTO / OpenAPI schema ownership

Each feature owns its OpenAPI schema classes. Metadata-only decorated `@ApiProperty`
classes live in feature-local `*.schema.ts` files (e.g.
`api/forwards/forward-rule.schema.ts`, `api/config/config-plan.schema.ts`,
`api/ports/port-advisory.schema.ts`); the response **mappers** (`to*ResponseDto`)
stay in the sibling `*.response.dto.ts` (which re-exports the schema class), and
validated request bodies (class-validator, instantiated by the pipe) stay in
`*.body.dto.ts`. `api/common/` holds **only genuinely cross-feature** schemas —
currently just `api-error.schema.ts` (`ApiErrorResponseDto`, the shared `{ errors }`
envelope).
Do **not** reintroduce a centralized schema bucket; a feature-owned schema belongs in
its feature folder even when another feature imports it (e.g. `ForwardRuleDto` is
forwards-owned and imported by config-export). The `*.schema.ts` files are
metadata-only (never instantiated) and coverage-excluded by the
`sources/**/*.schema.ts` glob; their generated schemas are asserted through the
OpenAPI generator tests, and the mappers they pair with are 100% covered.

## OpenAPI artifact placement

`npm run generate:apidoc` generates the OpenAPI document from the NestJS
controller/DTO metadata (offline — no listener) and writes two artifacts:

- **Primary (server-owned, generated):** `server/build/api/openapi.json` — under the
  gitignored `build/` output; the generator's primary target.
- **Docs copy (tracked, reviewed):** `docs/api/openapi.json` — synced byte-for-byte
  from the primary artifact. A generator test (`openapi.test.ts`) is the drift guard
  (it fails if the tracked copy is stale — run `generate:apidoc` and commit). The
  same test asserts the documented route inventory (`EXPECTED_ROUTES`); a
  new/removed/re-pathed endpoint must update it.

The serialized document is deterministic — `components.schemas` is sorted by name in
`serializeOpenApiDocument` so output is stable regardless of module-evaluation order.

**Release output:** packaging calls `copyOpenApiToRelease(releaseDir, sourcePath)`
(`openapi/openapi.ts`) to copy the already-generated primary artifact to
`<releaseDir>/api/openapi.json` **without regenerating**. The platform build scripts
(`scripts/{windows,macos,linux}/build-runtime.*`) run `generate:apidoc` and then
`copy:apidoc:release -w server -- <packageDir>`, so the packaged runtime
(`build/portier/`, the platform dirs, and release archives) includes
`api/openapi.json`. The generator never depends on a release directory existing.

## Packaged Node fallback

The packaged single-file Node fallback `server.js` is an esbuild `--bundle --minify
--format=cjs` of `sources/index.ts` (the NestJS entry). NestJS lazily `require()`s a
few optional transports it never uses for a plain HTTP app, so the build scripts mark
them external (`@nestjs/microservices`, `@nestjs/websockets/socket-module`,
`@nestjs/microservices/microservices-module`, `class-transformer/storage`). Because
the bundle is CommonJS and the repo root is `"type": "module"`, the build scripts also
write a `{ "type": "commonjs" }` `package.json` into the package dir so Node loads
`server.js` as CommonJS. The Go service remains the **preferred** packaged runtime
(the runtime smoke test exercises Go); `server.js` is the Node fallback and boots the
same NestJS app.

## Layout

The root of `sources/` holds only the process entry (`index.ts`); every other file
has a homed folder. API (HTTP) code lives under `api/` — per-feature folders plus
`api/common/` for the API-layer shared infrastructure; framework-free
domain/runtime/persistence logic lives in its own concern folder; and app/bootstrap
wiring lives under `app/`.

```text
sources/
  index.ts                      # the ONLY root file — server entry; boots NestJS via createNestApp + NestJS Logger (coverage-excluded)
  app/                          # app/bootstrap/runtime wiring
    app.factory.ts              # createNestApp(runtime?, options?) — live vs static root; resolveLoggerOption; configures static assets
    app.module.ts               # static AppModule (APP_RUNTIME=null) + createLiveAppModule(runtime); global error filter
    runtime-context.ts          # AppRuntime + APP_RUNTIME + @Global() RuntimeContextModule.forRoot(runtime|null)
    server-options.ts           # resolveServerOptions — CLI/env option resolution for the entry
  health/                       # GET /health (controller → service → module)
  api/                          # the HTTP/API layer
    <feature>/                  # controller → service → reader/writer; *.schema.ts (OpenAPI classes, coverage-excluded);
                                #   *.response.dto.ts (mappers, covered); *.body.dto.ts (validated request DTOs)
      ports/ activity/ status/ forwards/ runtime/ config/ connections/
    forwards/manager-error.ts   # mapManagerError — ForwardManager domain errors → API exceptions (forwards-owned)
    common/                     # API-layer shared infrastructure (used only by api/ features + the app filter registration):
      clock.reader.ts           # ClockReader/CLOCK_READER — shared live-clock provider (runtime/config/connections/diagnose)
      api-error-envelope.ts     # pure toApiError(exception) + isApiPath — the /api error mapping
      api-error-envelope.filter.ts  # global catch-all filter: /api/* → envelope, non-API → SPA index fallback (STATIC_FALLBACK)
      api-errors.ts             # ApiBadRequestException(string[]) etc. — raised by controllers, not literals
      api-validation.pipe.ts    # ApiValidationPipe(Dto) — class-validator/-transformer → ApiBadRequestException
      api-error.schema.ts       # ApiErrorResponseDto — the ONLY cross-feature OpenAPI schema (coverage-excluded)
  static/
    static-serving.ts           # resolveStaticOptions/hasStaticClient/configureStaticAssets + StaticFallback/STATIC_FALLBACK
  openapi/
    openapi.ts                  # generate/serialize(sorted)/resolveOpenApiPaths/writeOpenApiArtifacts/copyOpenApiToRelease — fully covered
    generate.ts                 # logic-free `npm run generate:apidoc` entry — coverage-excluded
    copy-release.ts             # logic-free `npm run copy:apidoc:release` entry — coverage-excluded
  # framework-free domain / runtime / persistence concern folders:
  forwarders/                   # forward-manager.ts (rule lifecycle/manager) + tcp/udp forwarders + types
  config/                       # config-export.ts (buildExportedConfig + configExportedActivityEvent) + config-plan.ts (buildConfigPlan/buildApplyImportFromPlan)
  persistence/                  # config-store.ts (atomic rules.json store)
  connections/                  # connections-snapshot.ts (buildLiveConnections) + tcp/udp connection registries
  runtime/                      # runtime-info.ts (buildRuntimeInfo)
  diagnostics/                  # diagnose.ts (diagnoseRule probes)
  activity/                     # activity-store.ts
  testing/                      # test-helpers.ts (test-only; coverage-excluded)
```

**Ownership rules:** the root `sources/` directory holds only `index.ts`. Domain
logic does NOT live under `api/` (which is HTTP-only); it lives in its concern folder
(`forwarders/`, `config/`, `persistence/`, `connections/`, `runtime/`, `diagnostics/`,
`activity/`). There is **no `sources/common/`** — API-layer shared infrastructure (the
error envelope/filter/exceptions, the validation pipe, the shared `CLOCK_READER`, and
the single cross-feature `ApiErrorResponseDto` schema) lives in **`api/common/`**,
because its only consumers are `api/` features (plus `app/app.module.ts`, which
registers the global error filter). A feature-owned helper lives in its
feature/concern folder (e.g. `manager-error.ts` is forwards-owned → `api/forwards/`).
DTO/schema ownership stays feature-local (`*.schema.ts` per feature).

**Logging policy:** server **runtime** logging goes through the NestJS `Logger` (the
entry uses `new Logger("Server")`; services use `new Logger(ClassName.name)` where
they log). `createNestApp` enables the logger for a live runtime
(`DEFAULT_SERVER_LOG_LEVELS`) and silences it for OpenAPI generation / tests
(`resolveLoggerOption`, overridable via `CreateNestAppOptions.logger`). There is no
custom logger abstraction and no global `console` monkey-patching. The only `console.*`
calls are in the logic-free **build-tooling** entries (`openapi/generate.ts`,
`openapi/copy-release.ts`) — normal CLI/script output, outside runtime logging.

## Scripts

```bash
npm run dev        -w server   # NestJS server (watch, sources/index.ts)
npm run start      -w server   # compiled server (node build/index.js)
npm run build      -w server   # tsc → build/
npm run typecheck  -w server
npm run test       -w server   # all server tests
npm run generate:apidoc      -w server  # regenerate openapi.json from NestJS metadata
npm run copy:apidoc:release  -w server -- <dir>  # copy the primary OpenAPI artifact into a package dir
```

From the repo root, `npm run start:server` boots the compiled server with
`--service --static-dir client/build`.

## Coverage

Every server file with executable logic is covered at **100%** (statements / branches
/ functions; gate `100/100/100` in `scripts/validate-coverage.js`). The only
coverage-excluded files are the logic-free process entries (`sources/index.ts`,
`openapi/generate.ts`, `openapi/copy-release.ts` — their helpers are fully covered),
the test-only `testing/test-helpers.ts`, and the metadata-only `**/*.schema.ts`
OpenAPI schema classes (never instantiated).
Structurally-unreachable defensive branches (2 s socket-bind/connect timeouts,
post-stop socket races, nullish-on-initialized-counter fallbacks) are narrowly
annotated with `/* v8 ignore … -- reason */` rather than broadly excluded.

## Server conventions

- Controllers are **transport adapters only**; behaviour lives in services; features
  compose as modules. Controllers must **not hand-roll the `{ errors }` envelope** —
  raise `ApiBadRequestException(string[])` (or another API exception) and let the
  shared `ApiErrorEnvelopeFilter` produce the contract shape; translate domain errors
  with `mapManagerError`.
- **Explicit DTOs:** a request DTO for any query/route/body input (validated via
  `ApiValidationPipe(Dto)` when the check is simple enough to re-express exactly;
  delegated to the shared/domain validator otherwise, with a documentation-only DTO),
  and an explicit response DTO always — the boundary between domain data and HTTP
  JSON, mapped through a small pure `to*ResponseDto`. Documented exceptions: no input
  → no request DTO; `204`-empty → no response DTO; pure silent coercion-with-fallback
  (`GET /api/activity`) keeps coercion endpoint-local. The pipe takes the DTO class
  **explicitly** (esbuild doesn't emit `design:paramtypes`).
- **Volatile fields** come from narrow injected readers (`CLOCK_READER` /
  `PROCESS_READER` / `RUNTIME_INFO_READER`), never inline; a shared pure builder
  (`buildRuntimeInfo`, `buildExportedConfig`, `buildConfigPlan`, `buildLiveConnections`)
  produces the response so the timestamp is deterministically testable.
- **Runtime/domain state** is wired through a narrow injection token + interface
  (`ACTIVITY_STORE`/`ActivityReader`, `STATUS_READER`, `FORWARD_RULE_*`, `CONFIG_*`,
  `CONNECTIONS_READER`, `DIAGNOSTIC_READER`, …); the live server binds the real
  `ForwardManager`/`ActivityStore` via `APP_RUNTIME`, the static `AppModule` falls
  back to empty defaults, and tests seed via `overrideProvider`. Endpoint tests seed
  only **stopped** rules (no listeners) unless deliberately exercising sockets (free
  ports + `stopAll()` cleanup).
- **API contract parity is mandatory** — `npm run validate:contract` (TS↔Go) stays
  green and no public API path/DTO is renamed for cosmetic reasons.
- **OpenAPI docs are generated, not hand-written** — each endpoint carries its
  `@ApiTags`/`@ApiOperation`/`@Api*Response`/`@ApiQuery` + a decorated `*.schema.ts`
  class in the same change; run `generate:apidoc` and commit `docs/api/openapi.json`.
- New code reaches **100% meaningful coverage**; build output stays under `build/`,
  never `dist/`.
```
