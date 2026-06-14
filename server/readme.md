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
- All `/api/*` errors → the Portier `{ "errors": ["..."] }` envelope via the
  global `ApiErrorEnvelopeFilter` (v1.14 Slice 3): unmatched routes →
  `404 ["API route was not found."]`, controller-raised `400`s carry their
  messages, unknown errors → `500 ["Internal server error."]` (no leak). Non-API
  routes keep NestJS's default error shape. Controllers raise
  `ApiBadRequestException(string[])` rather than hand-rolling the envelope.

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
    activity/                   # GET + DELETE /api/activity (injected ACTIVITY_STORE; ActivityReader + ActivityClearer)
    status/                     # GET /api/status (injected STATUS_READER; response: *.response.dto + mapper)
    forwards/                   # GET /api/forwards (injected FORWARDS_READER; response: *.response.dto + mapper)
  common/
    api-error-envelope.ts        # pure toApiError(exception) + isApiPath — the /api error mapping
    api-error-envelope.filter.ts # global catch-all filter: /api/* → envelope, non-API → NestJS default
    api-errors.ts                # ApiBadRequestException(string[]) — controllers raise this, not a literal
    api-validation.pipe.ts       # ApiValidationPipe(Dto) — class-validator/-transformer → ApiBadRequestException
  testing/
    api-parity.ts               # Express↔Nest parity harness (boot, fetch, deterministic compare)
```

Every `sources/nest/` file is covered at **100%** (statements/branches/functions).
Startup logic lives in `bootstrap.ts` / `nest-options.ts` so the only
coverage-excluded file is the logic-free `main.ts` process entry, mirroring how
`sources/index.ts` is excluded.

## Scripts

```bash
npm run dev        -w server   # active Express server (watch)
npm run build      -w server   # tsc → build/ (includes the nest scaffold)
npm run typecheck  -w server
npm run test       -w server   # all server tests (Express + nest scaffold)

npm run build:nest -w server   # build (alias — nest is part of the unified server build)
npm run start:nest -w server   # run the NestJS scaffold (scaffold only — not the active server)
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
