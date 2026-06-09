# Portier CLI

The `portier` CLI is a Go-based command-line tool for managing the local Portier port forwarding service. It talks to the existing management API and works with both runtimes (Go service and TypeScript server). It is not a second Portier runtime.

## Location

`tools/cli/` — a user-facing tool, not repo automation, not a service runtime.

## Binary name

- `portier` on Linux and macOS
- `portier.exe` on Windows

The background service binary remains `service` / `service.exe`.

## Default management URL

```
http://127.0.0.1:47831
```

## Usage

```
portier [global flags] <command>
```

## Global flags

| Flag | Description |
|---|---|
| `--url <url>` | Full management API URL (overrides `--host` and `--port`) |
| `--host <host>` | Management host (default: `127.0.0.1`) |
| `--port <port>` | Management port (default: `47831`) |
| `--json` | Output as machine-readable JSON |
| `--version` | Show CLI version and exit |

## Environment variables

| Variable | Description |
|---|---|
| `PORTIER_URL` | Default management URL |

## Connection URL precedence

1. `--url` (wins over all others)
2. `--host` / `--port` (either or both; missing half uses default)
3. `PORTIER_URL` environment variable
4. Default: `http://127.0.0.1:47831`

## Commands

### `portier list`

List configured forwarding rules.

```
portier list [--json]
```

Calls `GET /api/forwards`. Displays rule name, protocol, listen endpoint, target endpoint, and enabled state.

Human output shows a compact aligned table. With `--json`: prints the raw `ForwardRuleResponse[]` array from the API.

### `portier status`

Show runtime status for all forwarding rules.

```
portier status [--json]
```

Calls `GET /api/status` (and `GET /api/forwards` for rule names in human mode). Displays name, protocol, running/stopped state, active connections, bytes in/out, and last error if present.

With `--json`: prints the raw `ForwardStatus[]` array.

### `portier activity`

Show recent activity events (rule lifecycle, TCP connections, UDP packets).

```
portier activity [options]

Options:
  --limit int       Number of events to return, 1–500 (default 50)
  --rule string     Filter by rule ID
  --type string     Filter by event type (e.g. rule.started, tcp.connection.opened)
  --severity string Filter by severity: info, success, warning, error
```

Calls `GET /api/activity`. Displays timestamp, severity, event type, rule name, and message.

Events are returned newest first.

With `--json`: prints a raw `ActivityEvent[]` array (not the `{"events":[...]}` wrapper).

Invalid `--limit` (outside 1–500) exits with code `2`.

### `portier start <id|name>`

Start a forwarding rule.

```
portier start <id|name> [--json]
```

Resolves the rule by exact ID or exact name. If multiple rules share the same name, use the rule ID.

Calls `POST /api/forwards/:id/start`.

Human output: `Started <name>  (<listen> → <target>)`.

With `--json`: prints `{"ok": true, "action": "start", "ruleId": "..."}`.

### `portier stop <id|name>`

Stop a forwarding rule.

```
portier stop <id|name> [--json]
```

Resolves the rule by exact ID or exact name.

Calls `POST /api/forwards/:id/stop`.

Human output: `Stopped <name>`.

With `--json`: prints `{"ok": true, "action": "stop", "ruleId": "..."}`.

### `portier diagnose <id|name>`

Run diagnostic checks against a forwarding rule without changing its state.

```
portier diagnose <id|name> [--json]
```

Resolves the rule by exact ID or exact name.

Calls `POST /api/forwards/:id/diagnose`. No rule state is mutated.

Human output: summary status and message, followed by a CHECK/STATUS/MESSAGE table for all checks.

With `--json`: prints the raw `RuleDiagnosticsResult` from the API.

### `portier config validate <file>`

Validate a local config file without importing it or contacting the service.

```
portier config validate <file> [--json]
```

Accepted file shapes:
- Raw JSON array: `[{ "name": "...", ... }, ...]`
- Wrapper object: `{ "rules": [...] }`
- Exported config: `{ "version": "1", "exportedAt": "...", "rules": [...] }`

Validates: non-empty name, valid protocol (`tcp`/`udp`), non-empty listen/target hosts, ports in range 1–65535, valid `udpMode` if present, no duplicate listen bindings.

Human output: `Config is valid.` with rule count, or `Config is invalid.` with a list of errors.

With `--json`: prints `{ "valid": true|false, "ruleCount": N, "tcpCount": N, "udpCount": N, "errors": [] }`.

Exit codes: `0` valid, `1` invalid or unreadable, `2` missing file path argument.

### `portier config export --out <file>`

Export the current rules from the running service to a file.

```
portier config export --out <file>
portier --json config export                # print raw config JSON to stdout
portier --json config export --out <file>   # write file + print result object
```

Calls `GET /api/config/export`. Writes a pretty-printed JSON config file. The file is only written after a successful API response.

Human output (with `--out`): `Exported N rules to <file>`.

With `--json` and `--out`: prints `{ "ok": true, "path": "...", "ruleCount": N }`.

With `--json` and no `--out`: prints the raw exported config JSON to stdout.

Without `--json` and no `--out`: exits with error code `2` (use `--out` or add `--json` to print to stdout).

### `portier config import <file> --mode merge|replace [--yes]`

Import rules from a local config file into the running service.

```
portier config import --mode merge <file>
portier config import --mode replace --yes <file>
```

The file is validated locally before any API call is made. Invalid files are rejected without contacting the service.

Calls `POST /api/config/import` with the given mode.

Modes:
- `merge` — add rules from the file; existing rules are preserved; ID conflicts generate new IDs; listen-binding conflicts abort the import
- `replace` — remove all existing rules and replace with the imported set; requires `--yes` to confirm

The `--yes` flag is required for `--mode replace`. Without it the command exits with code `2` and displays a clear warning. No interactive prompts.

Human output: `Imported config using merge mode.` / `Imported config using replace mode.`

With `--json`: prints `{ "ok": true, "mode": "merge"|"replace" }`.

### Rule identity

All commands that target a rule accept:
- An exact rule ID (preferred; always unambiguous)
- An exact rule name (succeeds only if the name is unique across all rules)

If multiple rules share the same name, the command exits with code 2 and lists the matching IDs on stderr. Use the rule ID in that case.

### `portier config plan <file>`

Compare a desired config file against the currently running configuration and show a structured plan.

```
portier config plan <file>
portier --json config plan <file>
portier config plan <file> --fail-on-drift
```

Validates the desired file locally (same validation as `config validate`), then calls `POST /api/config/plan`. If the file is invalid or unreadable, exits with code `2` without calling the API.

Human output when no drift:
```
No drift detected.  (5 unchanged)
```

Human output when drift is present — prints a "Config Plan" header, a summary line (`Add: N  Update: N  Remove: N  Unchanged: N`), then per-operation rows with field-level change detail:
```
  add        my-rule                  tcp   127.0.0.1:48000
  update     other-rule               tcp   127.0.0.1:48001 [destructive]
    listenPort: 15432 → 15433
  remove     old-rule                 udp   0.0.0.0:48002 [destructive]
```

Warnings and errors are printed after the operation list. Warnings do not change the exit code.

Flags:
- `--fail-on-drift` — exits with code `4` when any add, update, or remove is present and no plan errors occurred. Plan errors (exit `1`) take priority over drift (exit `4`).

With `--json`: prints the raw `ConfigPlanResponse` from the API.

Exit codes:
- `0` — success: no drift, or drift present but `--fail-on-drift` not set, and no plan errors
- `1` — plan errors returned (`hasErrors: true`) or API error
- `2` — invalid file, unreadable file, or missing argument
- `3` — connection failure (Portier service unreachable)
- `4` — drift detected with `--fail-on-drift` (only when no plan errors)

### `portier config diff <file>`

Human-friendly diff view of changes between the desired file and the running config.

```
portier config diff <file>
portier config diff <file> --show-unchanged
portier --json config diff <file>
portier config diff <file> --fail-on-drift
```

Uses the same underlying `POST /api/config/plan` response as `portier config plan`. Formats output as a compact diff with `+`/`~`/`-`/`=` prefixes and field-level change detail.

Human output when no drift:
```
No drift detected.
```

Human output when drift is present:
```
+ Add:       my-rule                  tcp   127.0.0.1:48000
~ Update:    other-rule               tcp   127.0.0.1:48001 [destructive]
  listenPort: 15432 → 15433
- Remove:    old-rule                 udp   0.0.0.0:48002 [destructive]
```

Unchanged rules are hidden by default. Warnings and errors are printed after the rule list.

Flags:
- `--show-unchanged` — include unchanged rules in output (shown with `=` prefix)
- `--fail-on-drift` — exits with code `4` when drift is present and no plan errors occurred
- `--json` (global) — prints the raw `ConfigPlanResponse` (same as `config plan --json`)

Exit codes are the same as `portier config plan`.

### `portier config apply <file> --yes`

Apply a desired config to the running configuration with explicit confirmation.

```
portier config apply <file> --yes
portier config apply <file> --dry-run
portier config apply <file> --backup-out <backup.json> --yes
portier --json config apply <file> --yes
```

Validates the desired file locally before calling `POST /api/config/apply`. When destructive operations (remove or forwarding-field update) are present, `--yes` is required or the API returns 400.

Flags:
- `--yes` — required for destructive operations.
- `--dry-run` — previews plan counts without mutating config. Does not require `--yes`.
- `--backup-out <file>` — exports the current config to a file before applying (skipped when `--dry-run`).

Exit codes: `0` success, `1` plan errors or API error, `2` usage/local validation, `3` connection error.

With `--json`: prints raw `ConfigApplyResponse { ok, dryRun, appliedAt, plan, applied }` from the API.

### `portier diagnostics export`

Build a JSON diagnostics bundle from the running Portier service.

```
portier diagnostics export --out <file>
portier diagnostics export --out <file> --run-diagnostics
portier diagnostics export --out <file> --activity-limit <n>
portier --json diagnostics export --out <file>
portier --json diagnostics export              # print bundle to stdout
```

Fetches runtime info, rules, statuses, and recent activity from the management API and writes a pretty-printed JSON support bundle. The bundle is built from independent sources — partial failures are recorded in `errors[]` rather than aborting the export.

Flags:
- `--out <file>` — output file path. Required in human mode; omit with `--json` to print to stdout.
- `--run-diagnostics` — run `POST /api/forwards/:id/diagnose` for each rule and include results.
- `--activity-limit <n>` — maximum activity events to include (1–500, default: 100). Exits code `2` if out of range.

**Bundle schema:**

```json
{
  "schemaVersion": "1",
  "exportedAt": "ISO 8601",
  "app": { "name": "Portier", "version": "..." },
  "runtime": { ... } | null,
  "rules": [ ... ],
  "statuses": [ ... ],
  "diagnostics": { "<ruleId>": { ... } },
  "diagnosticsNote": "...",
  "activity": { "included": true, "events": [...], "note": "..." },
  "metadata": { "managementUrl": "...", "source": "cli", "generatedBy": "portier diagnostics export" },
  "errors": [ { "source": "...", "message": "..." } ]
}
```

Included data sources:
- `GET /api/runtime` → `runtime`
- `GET /api/forwards` → `rules`
- `GET /api/status` → `statuses`
- `GET /api/activity?limit=<n>` → `activity.events`
- `POST /api/forwards/:id/diagnose` → `diagnostics` (only with `--run-diagnostics`)

Not included: logs, environment variables, OS usernames, home directory paths beyond those already in `runtime`, raw disk files.

Human output (with `--out`): `Exported diagnostics to <file>` with rule/status/activity counts.
With partial failures: `Exported diagnostics with warnings to <file>`.

With `--json` and `--out`: prints `{ "ok": true, "path": "...", "ruleCount": N, "statusCount": N, "activityCount": N, "diagnosticCount": N }`.
With `--json` and no `--out`: prints the full diagnostics bundle to stdout.

If `--out` is omitted in human mode, exits with code `2`.
If the output file cannot be written, exits nonzero.

**Partial failure behavior:**
- Each data source is fetched independently.
- Failures are recorded in `errors[]` with a `source` field (`runtime`, `rules`, `statuses`, `activity`, `diagnostics:<ruleId>`).
- If `rules` fetch fails, per-rule diagnostics are skipped entirely.
- The bundle is always written as long as the file write succeeds.
- Human output says "with warnings" when errors are present.
- JSON result includes `"warningCount": N` when errors are present.

### `portier runtime`

Show runtime info for the running Portier service.

```
portier runtime [--json]
```

Calls `GET /api/runtime`. Displays name, version, runtime, platform/arch, uptime, management URL, config path, static dir, and service mode.

With `--json`: prints the raw `RuntimeInfo` JSON from the API.

### `portier version`

Show the CLI version.

```
portier version
```

### `portier help`

Show help text.

```
portier help
portier --help
portier -h
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | General error or API error |
| `2` | Invalid arguments or usage error |
| `3` | Connection failure — Portier service unreachable |
| `4` | Drift detected — used by `portier config plan --fail-on-drift` and `portier config diff --fail-on-drift` |

## Runtime Package

The CLI binary is included in the Portier runtime package and release artifacts alongside the service binary:

```
<install-dir>/
  portier          (or portier.exe on Windows)   # this CLI
  service          (or service.exe on Windows)   # background service
  server.js                                      # Node.js fallback
  web/                                           # React management UI
  readme.txt
```

The `readme.txt` in the runtime package documents basic CLI usage for each platform.

The CLI is not added to PATH by the installer in v1.3. To use it from any directory, add `<install-dir>` to your PATH manually, or invoke it with the full path:

- Windows: `C:\Program Files\Portier\portier.exe runtime`
- macOS: `~/Applications/Portier/portier runtime`
- Linux: `/opt/portier/portier runtime`

## Building

```powershell
npm run build:cli
```

Output: `tools/cli/build/portier-cli` (or `portier-cli` without `.exe` on all platforms when using this standalone command).

The runtime build (`npm run build:runtime`) builds the CLI directly into `build/portier/portier[.exe]`.

## Testing

```powershell
npm run test:cli
```

Runs `go test ./...` inside `tools/cli/`. Uses `httptest` for API client tests — no running Portier service required.

### Live-runtime DTO parity guard

The CLI DTOs in `sources/client/client.go` are a third copy of the REST contract (alongside `@portier/shared` and the Go service's `domain`/`configplan` types). `httptest` mocks alone can mask drift between what the CLI expects and what a real runtime emits, so `npm run validate:contract` additionally captures live JSON responses from each running runtime and strictly decodes them (`DisallowUnknownFields`) into the CLI DTOs via `TestCLIDTOContractParity` (`sources/client/contract_decode_test.go`). That test only runs when `PORTIER_CLI_CONTRACT_FIXTURES` is set (which `validate:contract` does); plain `go test`/`npm run test:cli` skips it. When a new API response family is added, update both `scripts/validate-contract.js` (capture) and this test (decode + assert). `/api/connections` is intentionally out of scope — the CLI has no connections DTO/command.

## Validation

```powershell
npm run validate:cli
```

Runs tests, builds, and checks coverage. Fails clearly if Go is unavailable.

```powershell
npm run validate:coverage:cli
```

Coverage gate: runs `go test` with cross-package instrumentation and fails if total statement coverage falls below 92%. Genuinely untestable branches (main entry point, http.NewRequest errors, json.Marshal errors) are documented in `scripts/validate-coverage.js`. Use `npm run validate:coverage` to run all five component gates at once.

## Module structure

```
tools/cli/
  go.mod
  readme.md
  sources/
    main.go            entry point and command dispatch
    version/           CLI version constant
    client/            HTTP API client for the management API
    commands/          command handlers and connection config
    output/            human-readable and JSON output helpers
```

## Notes

- The CLI uses the management API. It does not read or write `rules.json` directly.
- Both runtimes (Go service and TypeScript server) implement the same API contract, so the CLI works with either.
- The management API defaults to `127.0.0.1:47831` and is not LAN-visible by default.
- Remote authentication is not supported in this version.
