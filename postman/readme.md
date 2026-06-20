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

## Optional: run from the CLI with Newman

The collection runs in CI only as a static drift/safety check (`validate:postman`); it is
**not** executed against a live server in CI. To run it locally against a running Portier
instance you can use [Newman](https://github.com/postmanlabs/newman) (install separately,
not a project dependency):

```powershell
# Start Portier first, then:
newman run postman/collection.json -e postman/environment.json
```

No cloud Postman account, sync, or network access beyond your local Portier is required.
