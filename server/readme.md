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
- `GET /api/activity` (v1.14 Slice 4) — read-only activity list over an injected
  `ActivityReader` (`ACTIVITY_STORE` token, default = a fresh domain
  `ActivityStore`). Byte-for-byte parity-tested; shadow-only under `start:nest`.
  `DELETE /api/activity` (a mutation) stays with Express, deferred.
- `GET /api/status` (v1.14 Slice 5) — read-only per-rule status over a narrow
  `StatusReader` (`STATUS_READER` token, default = a trivial empty reader; the
  domain `ForwardManager` satisfies it and is bound in tests / when Nest is
  active). The first manager-dependent read; byte-for-byte parity-tested with
  stopped rules (no volatile fields); shadow-only under `start:nest`.
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
    ports/                      # GET /api/ports/advisory (controller → service → module)
    activity/                   # GET /api/activity (controller → service → module; injected ACTIVITY_STORE)
    status/                     # GET /api/status (controller → service → reader; injected STATUS_READER)
  common/
    api-error-envelope.ts        # pure toApiError(exception) + isApiPath — the /api error mapping
    api-error-envelope.filter.ts # global catch-all filter: /api/* → envelope, non-API → NestJS default
    api-errors.ts                # ApiBadRequestException(string[]) — controllers raise this, not a literal
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
- **API contract parity is mandatory** — every migration step keeps
  `npm run validate:contract` green (TS↔Go), and no public API path/DTO is
  renamed for cosmetic reasons. Each migrated endpoint is also checked
  **byte-for-byte against the existing Express route** with the parity harness
  (`sources/nest/testing/api-parity.ts`).
- The existing Express server **remains the active runtime** until a NestJS
  replacement is explicitly validated (contract + runtime smoke + E2E).
- No endpoint is migrated without tests and contract validation.
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
