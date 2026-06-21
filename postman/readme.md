# Portier Postman collection

A ready-to-run [Postman](https://www.postman.com/) collection for the Portier management
API, **generated from the canonical OpenAPI contract** (`docs/openapi.json`). There is no
hand-maintained second API definition — regenerate after any contract change:

```powershell
npm run generate:postman   # write collection.json + environment.json from docs/openapi.json
npm run validate:postman   # check coverage, safety, and that the files are not stale
```

`validate:postman` runs in CI, so a stale or drifted collection fails the build.

## Files

- `collection.json` — the Postman v2.1 collection (import this).
- `environment.json` — a variable-driven environment (import and select it).

## Import and run

1. In Postman: **Import** → select both `collection.json` and `environment.json`.
2. Pick the **Portier Local** environment (top-right selector).
3. Start Portier locally (default management API at `http://127.0.0.1:47831`).
4. Open a request and **Send**, or run a folder with the Collection Runner.

## Environment variables

Edit `environment.json` (or override per-request in Postman) to point at your runtime and
choose test values. Nothing here is a secret — no tokens, passwords, or machine-specific
paths.

| Variable | Default | Purpose |
| --- | --- | --- |
| `host` | `127.0.0.1` | Management API host. |
| `port` | `47831` | Management API port. |
| `baseUrl` | `http://{{host}}:{{port}}` | Base URL every request is built on. |
| `apiPrefix` | `/api` | API route prefix. |
| `ruleName` | `portier-postman-demo` | Demo rule name used by the flow. |
| `listenHost` | `127.0.0.1` | Demo rule listen host (loopback). |
| `listenPort` | `48010` | Demo rule listen port. |
| `targetHost` | `127.0.0.1` | Demo rule target host. |
| `targetPort` | `48011` | Demo rule target port. |
| `protocol` | `tcp` | Demo rule protocol. |
| `udpMode` | `one-way` | UDP mode (UDP rules only). |
| `group` | `postman-demo-group` | Group label used by the group endpoints. |
| `advisoryPurpose` | `forward` | Port-advisory query `purpose`. |
| `advisoryPort` | `8080` | Port-advisory query `port`. |
| `createdRuleId` | _(empty)_ | Set by the Happy Path flow at run time. |
| `invalidRuleId` | `nonexistent-rule-00000000` | Known-missing id for negative tests. |

## Collection structure

- **Atomic Endpoint Tests** — one request per public API operation, grouped by tag. Each
  asserts the response status is one the contract documents. These are non-destructive:
  `id`/`group` path params resolve to `{{invalidRuleId}}` / `{{group}}` (a `404`, no
  mutation), config apply is `dryRun`, import is an empty merge, and reorder is an empty
  no-op. The one exception is **Create a forward rule**, which creates a single stopped
  (`enabled:false`) demo rule.
- **Happy Path Rule Flow** — an ordered, self-cleaning sequence: runtime check → create a
  stopped demo rule (captures `createdRuleId`) → verify in the list and status → rename →
  start and stop it on a loopback high port → export config → delete → confirm removal.
  Run it top-to-bottom (Collection Runner or in order).
- **Negative/Error Tests** — requests the API is documented to reject (invalid port `400`,
  unknown rule `404`, plan without `desired` `400`, invalid import mode `400`, unknown
  reorder id `404`, advisory missing required params `400`), asserting the status and the
  `{ errors: string[] }` envelope.

## Two ways the collection is validated

| | `validate:postman` | `validate:postman:local` |
| --- | --- | --- |
| What | Static generation/drift/safety check | Live [Newman](https://github.com/postmanlabs/newman) run against a real runtime |
| Runs in CI | **Yes** (push/PR) | No — local-only |
| Needs a runtime | No | Yes (started automatically) |
| Network | None | Loopback only |

### Local Newman runtime smoke

```powershell
npm run validate:postman:local
```

This is **self-contained**: it starts a fresh Portier runtime on a free port with an empty,
temporary `rules.json`, waits for liveness, runs the collection with [Newman](https://github.com/postmanlabs/newman)
(a dev dependency — `npm install` provides it, no global install, no network), then stops the
runtime and deletes the temp config. **No user or production config is touched.** Each of the
three top-level folders runs against its own freshly-started runtime so they stay independent
(the atomic **Create a forward rule** intentionally leaves a stopped rule, which would otherwise
collide with the Happy Path create); the Happy Path flow also deletes its own rule and asserts
the list is empty, so nothing it creates survives.

**Runtime target.** The smoke runs against the NestJS/TypeScript server (`server/build/index.js`),
because the collection is generated from *its* OpenAPI document. The Go production service is at
full parity on the frozen `/api` surface; it differs on exactly one path — the liveness probe:
OpenAPI/NestJS document `GET /health`, while the Go service serves `GET /api/health` and does not
serve `/health`. That single divergence is covered by `npm run validate:runtime:smoke` and the Go
route-inventory parity test, so this collection-vs-OpenAPI smoke targets the contract-faithful
NestJS server. (If `server/build/index.js` is missing, the command builds it first.)

Overrides (the management/listen ports default to free ports):

```powershell
$env:PORTIER_PORT=43117; npm run validate:postman:local      # fixed management port
npm run validate:postman:local -- --port 43117               # same, via flag
npm run validate:postman:local -- --folder "Happy Path Rule Flow"   # one folder
npm run validate:postman:local -- --keep-going --verbose     # run all folders, verbose
```

### Against an already-running instance

To point Newman at a runtime you started yourself, edit `environment.json` (or override per
request) and run Newman directly — no cloud account, sync, or external network needed:

```powershell
# Start Portier first, then:
newman run postman/collection.json -e postman/environment.json
```

Note the liveness divergence above: against the Go service the `health` request targets `/health`
and will `404` (use `--folder` to run the other folders, or run against the NestJS server).
