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

Calls `GET /api/status` (and `GET /api/forwards` for rule names in human mode). Displays name, protocol, running/stopped state, **health** (`healthy`/`warning`/`error` — derived from the runtime status, distinct from the running/stopped state), active connections, bytes in/out, and last error if present.

With `--json`: prints the raw `ForwardStatus[]` array (each entry includes the `health` field).

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

### `portier config doctor <file>`

Run deterministic, **offline** diagnostic checks on a local config file. Does **not** require or contact a running Portier service, and never modifies the file.

```
portier config doctor [--strict] [--explain] [--out <file>] <file> [--json]
```

Accepted file shapes are the same as `config validate` (raw array, wrapper object, or exported config).

Each finding is a *doctor check* with a stable, machine-readable **code**, a `severity` (`info`/`warning`/`error`), a short `title`, an actionable `message`, and optional deterministic `details`. Stable check codes:

| Code | Severity | Meaning |
| --- | --- | --- |
| `config.read_failed` | error | The file could not be read. |
| `config.parse_failed` | error | The file is not valid Portier config JSON. |
| `config.empty` | warning | The config parsed but defines no rules. |
| `config.validation_failed` | error | One or more rules have invalid fields. |
| `config.duplicate_binding` | error | Two rules share a listen binding (`protocol`+`listenHost`+`listenPort`). |
| `config.lan_exposure` | warning | A rule listens on `0.0.0.0` (exposed on the LAN). |
| `config.privileged_port` | warning | A rule listens on a privileged port (`< 1024`). |
| `config.valid` | info | The config is readable, parseable, and valid. |

Human output groups the checks under a `Portier Config Doctor` heading with `[INFO]`/`[WARN]`/`[ERROR]` tags, a severity summary, and a `Result: passed`/`Result: failed` line. With `--json`, prints the full report: `{ "checks": [ { "code", "severity", "title", "message", "details"? } ], "summary": { "info": N, "warning": N, "error": N }, "strict": false, "result": "passed" }`.

**Structured details (JSON).** Config doctor findings carry deterministic, JSON-serializable `details` derived **only from the offline config** (rule names/hosts/ports/groups/ids — no environment, process, log, filesystem, runtime, or probe data):

- `config.duplicate_binding` → `{ "bindings": [ { "protocol", "listenHost", "listenPort", "rules": [ { "id"?, "name", "enabled", "group"? } ] } ], "errors": [...] }` (bindings sorted by protocol→host→port; rules in file order).
- `config.lan_exposure` / `config.privileged_port` → `{ "rules": [ { "id"?, "name", "protocol", "listenHost", "listenPort", "enabled", "group"? } ] }` (all affected rules aggregated into one check, file order).
- `config.empty` → `{ "ruleCount": 0 }`; `config.validation_failed` → `{ "errors": [...], "ruleCount": N }`; `config.valid` → `{ "ruleCount": N, "tcpCount": N, "udpCount": N }`.

These details flow through `--out` and are surfaced inline by `--explain`; the live `portier doctor` and the support bundle are unaffected.

**Config summary (JSON).** When the file parses, the config doctor JSON report also carries a compact, deterministic top-level `config` summary describing the config's shape (derived only from the offline config — no environment/runtime/filesystem data, and no repeat of full rule data):

```
"config": {
  "ruleCount": 4,
  "enabledRuleCount": 3,
  "disabledRuleCount": 1,
  "protocols": { "tcp": 3, "udp": 1 },
  "groupCount": 2,
  "groups": [
    { "name": "admin",   "ruleCount": 1, "enabledRuleCount": 1, "disabledRuleCount": 0 },
    { "name": "backend", "ruleCount": 2, "enabledRuleCount": 1, "disabledRuleCount": 1 }
  ],
  "ungroupedRuleCount": 1
}
```

Groups are sorted by (trimmed) name; an empty/whitespace group counts as ungrouped. The summary is present for an empty config (`ruleCount: 0`) and for a validation-failure config (parse succeeded), and absent on read/parse failure. It is unchanged by `--explain` and `--strict`, flows through `--out`, and appears **only** for `config doctor` (the live `portier doctor` JSON has no `config` field).

Exit codes: `0` doctor completed with no error-severity checks (**warnings alone exit `0`** unless `--strict`), `1` one or more error-severity checks **or any warning when `--strict`**, `2` missing file-path argument or usage error.

**`--strict`** treats warnings as failures (a warning-only report exits `1`). It changes only the exit-code interpretation — the same checks run and nothing is modified.

**`--out <file>`** also writes the JSON report (same `{ checks, summary, strict, result }` shape as `--json`) to a file. A file-write failure is an operation failure → exit `1`.

**`--explain`** adds an inline explanation (code, meaning, next action) for each emitted check (human blocks; an additive `explanations` map in JSON), reusing the `portier explain` registry. It changes nothing about which checks run, the result, or the exit code. Place `--strict`/`--explain`/`--out` **before** the config file (`config doctor --explain <file>`), consistent with the other config subcommands.

> Note: `config doctor` is read-only and offline. Warnings are derived from the file's contents only — they do **not** imply any runtime probing or target reachability check.

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

Exit codes: `0` success, `1` API error, `2` usage / invalid local config file (unreadable, malformed, validation failure) / missing `--yes` for replace, `3` connection error. (Local-input errors use `2`, consistent with `config plan`/`diff`/`apply`.)

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

### `portier group <list|start|stop>`

Operate on forwarding rules by their optional `group` label (v1.8). Group operations act on **existing rules only** — they never create, delete, or modify rule definitions, order, or metadata; they reuse the per-rule start/stop lifecycle.

```
portier group list
portier group start <group>
portier group stop <group>
portier --json group list
portier --json group start <group>
```

- **`group list`** — derives the distinct non-empty groups from the current rules (alphabetical) with per-group rule and running counts. Prints `No rule groups configured.` (exit `0`) when no rule has a group. Human output is a `GROUP / RULES / RUNNING` table; `--json` prints `{ "groups": [ { "group", "total", "running" } ] }`.
- **`group start <group>`** — calls `POST /api/forwards/groups/:group/start`. Starts every rule in the group (in rule order); already-running rules are reported `skipped` (`already_running`); a per-rule start failure is reported `failed` without aborting the rest.
- **`group stop <group>`** — calls `POST /api/forwards/groups/:group/stop`. Stops every running rule in the group; rules that are not running are reported `skipped` (`not_running`).

Human output for start/stop is a summary line (`N succeeded, N skipped, N failed (N total)`) followed by a `RULE / STATUS / REASON` table. With `--json`: prints the full `GroupActionResponse { group, action, total, succeeded, skipped, failed, results[{ ruleId, ruleName, status, reason? }] }`.

The group argument is trimmed and validated locally (non-empty, ≤ 64 characters, no control characters) before the request; the server remains authoritative.

Exit codes: `0` success — **including** rules skipped for `already_running` / `not_running`; `1` API/runtime error, **no matching group** (the API returns `404`), or one or more rules `failed`; `2` missing or invalid group argument; `3` connection error.

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

### `portier doctor`

Run deterministic diagnostic checks against the **live** Portier runtime. Read-only: never mutates the runtime or its config, and never probes forwarding targets.

```
portier doctor [--strict] [--explain] [--out <file>] [--json]
```

Reuses the same doctor report model as `config doctor` (Slice 1). Each finding is a *doctor check* with a stable `code`, a `severity` (`info`/`warning`/`error`), a `title`, a `message`, and optional `details`. Stable check codes:

| Code | Severity | Meaning |
| --- | --- | --- |
| `runtime.reachable` | info | The runtime responded (`GET /api/runtime`). |
| `runtime.unreachable` | error | The runtime could not be reached. Short-circuits later checks. |
| `runtime.version` | info / warning | Runtime version; **warning** (never failure) when it differs from the CLI version. |
| `runtime.status_read` | info | Rule status was read (`GET /api/status`). |
| `runtime.status_failed` | error | Rule status could not be read. |
| `rules.none` | warning | No forwarding rules are configured. |
| `rules.present` | info | N forwarding rules are configured. |
| `rules.health_ok` | info | No rule reports warning or error health. |
| `rules.health_warning` | warning | One or more rules report warning health. |
| `rules.health_error` | error | One or more rules report error health. |
| `config.export_read` | info | The current config was read (`GET /api/config/export`, read-only). |
| `config.export_failed` | error | The current config could not be read. |

Rule health comes straight from the API's `health` field (v1.8) — the CLI does **not** re-derive it. Human output groups checks under a `Portier Doctor` heading with `[INFO]`/`[WARN]`/`[ERROR]` tags, a severity summary, and a `Result: passed`/`Result: failed` line; `--json` prints the full report (`{ checks[], summary, strict, result }`).

Exit codes: `0` doctor completed with no error-severity checks (**warnings alone exit `0`** unless `--strict`), `1` one or more error-severity checks **or any warning when `--strict`**, `2` usage error (unexpected argument).

**`--strict`** treats warnings as failures: a warning-only report exits `1` instead of `0` (errors still exit `1`, info-only still exits `0`). Strict mode changes only the exit-code interpretation — it does not change which checks run, and never mutates the runtime or config. In human output a `Strict mode: warnings are treated as failures.` note is shown when a warning-only report fails because of strict; in JSON, `"strict"` and `"result"` reflect the mode and outcome.

**`--out <file>`** also writes the JSON report — the same `{ checks, summary, strict, result }` shape as `--json` — to a file (for CI artifacts / support bundles). It writes the file regardless of `--json`; stdout stays human output unless `--json` is set. A file-write failure is an operation failure → exit `1` (reported on stderr), independent of the diagnostic result. Export never mutates the runtime or config.

**`--explain`** adds an inline explanation (code, meaning, next action, related codes) for each check **that appears in the report**, reusing the same registry as `portier explain`. In human output each check is followed by an indented `Code:`/`Meaning:`/`What to do:` block; in JSON an additive top-level `explanations` map (`code → { title, meaning, action, ... }`) is added for the emitted codes only. `--explain` does **not** change which checks run, the summary, the result, or the exit code; combine it freely with `--strict`, `--json`, and `--out` (with `--out --explain` the exported JSON includes the explanations).

> **Unreachable-runtime exit code:** unlike the other live commands (which exit `3` on connection failure), `portier doctor` reports an unreachable runtime as a `runtime.unreachable` **check** and exits `1` — the doctor always completes and emits a report, and an unreachable runtime is an error-severity finding. This is an intentional, documented deviation specific to the doctor command.

### `portier explain <code>`

Explain a stable doctor/check, policy, or workflow code: what it means and what to do next. Fully **offline** — static reference data, no runtime contact, no external lookup, no AI, and nothing is changed or fixed.

```
portier explain <code> [--json]
portier explain --list [--json]
```

Covers all 41 stable codes: the 8 config-doctor codes and 12 live-doctor codes emitted by `portier config doctor` / `portier doctor`, the 6 policy finding codes emitted by `portier policy check` (`policy.valid`, `policy.group_required`, `policy.lan_exposure_forbidden`, `policy.privileged_port_forbidden`, `policy.autostart_forbidden`, `policy.duplicate_binding_forbidden`), and the 15 workflow step-validation codes emitted by `portier workflow plan` (`workflow.step.valid` plus the 14 invalid-step codes such as `workflow.step.unknown_report_from` and `workflow.step.conflicting_config_sources`). Human output:

```
config.duplicate_binding

Meaning:
Two or more rules use the same protocol, listen host, and listen port.

What to do:
Change one of the conflicting rules so each active binding (protocol + listen host + listen port) is unique.

Related:
- config.validation_failed
```

With `--json`, prints the explanation `{ "code", "title", "meaning", "action", "severity", "related": [...] }`. `portier explain --list` prints all known codes sorted (with titles), and `portier --json explain --list` prints the full array of explanations.

Exit codes: `0` explanation (or list) printed; `2` unknown code, missing code argument, or usage error. An unknown code prints a clear error pointing to `--list` and never returns a generic explanation.

### `portier support-bundle --out <directory>`

Collect deterministic doctor artifacts into a local directory for CI artifacts, bug reports, and future tooling. Read-only: contacts the live runtime but never mutates the runtime or its config. (Distinct from `portier diagnostics export`, which writes a single JSON bundle file; `support-bundle` is the doctor-centric **directory** bundle.)

```
portier support-bundle --out <directory> [--strict] [--json]
```

The bundle directory contains:

| File | Contents |
| --- | --- |
| `manifest.json` | Bundle metadata: `schemaVersion`, `generatedAt`, `cliVersion`, `source`, `generatedBy`, `runtimeUrl`, `strict`, `result`, `artifacts`, `warnings`. |
| `doctor.json` | The live doctor report — **the same schema as `portier doctor --json`** (`checks`/`summary`/`strict`/`result`). |
| `doctor.txt` | The human-readable doctor report (same as `portier doctor`). |
| `explanations.json` | All known doctor/check code explanations (same as `explain --list --json`). |
| `runtime.json` | Runtime metadata — omitted (with a manifest warning) if the runtime is unreachable. |
| `config-export.json` | Read-only config export — omitted (with a manifest warning) if unavailable. |

If the runtime is unreachable the bundle is **still created** with `manifest.json`, `doctor.json` (containing `runtime.unreachable`), `doctor.txt`, and `explanations.json`; the command exits `1` because the doctor report has an error.

**Safety:** the bundle deliberately **excludes** OS environment variables, process lists, logs, tokens, and arbitrary filesystem scans. It **may** include rule names, hosts, and ports, because those are part of Portier configuration/diagnostics.

`--strict` applies strict interpretation to the bundled doctor report (warnings fail). `--json` prints a machine-readable bundle summary (`{ out, artifacts, strict, result, warnings }`) to stdout.

Exit codes: `0` bundle written and the doctor report passed; `1` bundle written but the doctor report failed (errors / unreachable / a warning under `--strict`) **or** a filesystem/write failure; `2` missing `--out` or usage error. An existing **non-empty** output directory (or a path that is a file) is **refused** with exit `1` — a bundle is never silently overwritten; an existing empty directory is used. No zip output yet (directory only).

### Using doctor output with an AI assistant

If you want to ask an AI assistant to help interpret a doctor report, Portier ships a reusable, copy-paste **AI handoff prompt** in [`docs/prompts/doctor.md`](../../docs/prompts/doctor.md) (a full version and a short version).

**Portier never sends anything anywhere** — there is no AI integration, upload, or telemetry. The prompt is plain text *you* paste into an assistant of your choice, together with output you generated locally (e.g. `portier doctor --json --explain`). The prompt tells the assistant to treat the pasted report as the only source of truth, to distinguish info / warning / error / strict failures, to ground its analysis in the data, and — importantly — **never to ask you for secrets, tokens, keys, environment dumps, process lists, or logs** (the doctor tools never collect those). It asks for a verdict, prioritized findings, a risk level, and separated "safe now" vs "needs admin/network/security review" next steps.

### `portier policy check (--config <file> | --runtime) --policy <file>`

Evaluate a Portier config against a small JSON **policy** file. Choose **exactly one config source**: a local file (`--config`, fully offline) or the live runtime config (`--runtime`, read-only). Either way this is **dry-run evaluation only** — there is no enforcement, no automation, no target probing, and no config/policy/runtime mutation.

```
portier policy check --config <config-file> --policy <policy-file> [--json] [--explain] [--out <file>]
portier policy check --runtime --policy <policy-file> [--json] [--explain] [--out <file>]
```

**Config source (exactly one required):**

| Source | Behavior |
| --- | --- |
| `--config <file>` | Evaluate a local config file. **Fully offline** — does not contact or even resolve the runtime URL. |
| `--runtime` | Evaluate the **live runtime config**, read via the existing read-only config-export path (`GET /api/config/export`). Uses the global `--url`/`--host`/`--port` like other live commands. No new endpoint, no probing, read-only. |

Supplying **both** `--config` and `--runtime`, or **neither**, is a usage error (exit `2`). `--policy` is required and is validated **before** any runtime I/O (so an unsupported policy schema exits `2`, not a runtime error).

Both the human report (a `Source: config file` / `Source: runtime` line under the title) and the JSON report carry an additive `source` field recording where the evaluated config came from. A runtime config evaluates identically to the same config in a local file — the findings, summary, result, explanations, and JSON shape are the same. In runtime mode, an **unreachable runtime exits `3`** and a **config-export/API failure exits `1`** (the standard live-CLI convention) — a runtime read failure is never wrapped as a policy finding, and no report is invented when the config could not be read.

The policy file is a small JSON document (`schemaVersion: 1`) with a `rules` object of boolean guardrails:

```json
{
  "schemaVersion": 1,
  "rules": {
    "requireGroup": false,
    "allowLanExposure": true,
    "allowPrivilegedPorts": true,
    "allowAutostart": true,
    "forbidDuplicateBindings": true
  }
}
```

| Policy rule | Effect |
| --- | --- |
| `requireGroup` | When `true`, every rule must have a non-empty (trimmed) group. |
| `allowLanExposure` | When `false`, listening on `0.0.0.0` is a violation. |
| `allowPrivilegedPorts` | When `false`, listen ports below `1024` are violations. |
| `allowAutostart` | When `false`, autostart-enabled rules (`enabled: true`) are violations. |
| `forbidDuplicateBindings` | When `true`, duplicate `protocol` + `listenHost` + `listenPort` bindings are violations. |

Each **omitted** field falls back to the **permissive default** shown above (`requireGroup`/`forbidDuplicateBindings` default off, `allow*` default on), so an empty `rules` object permits everything — operators opt **into** each restriction. **Unknown fields are rejected** (exit `2`) so a typo cannot silently relax a guardrail. There is intentionally **no `allowUdp`/protocol-restriction policy** — UDP is first-class and is evaluated by the same general guardrails as TCP.

Human output lists each finding with an `[INFO]`/`[ERROR]` tag, a severity summary, and a `Result: passed`/`Result: failed` line. `--json` emits the full report:

```json
{
  "findings": [
    {
      "code": "policy.lan_exposure_forbidden",
      "severity": "error",
      "title": "Rule \"Admin UI\" listens on 0.0.0.0",
      "message": "This rule listens on 0.0.0.0, but the policy forbids LAN exposure.",
      "details": { "rule": { "name": "Admin UI", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 48080, "enabled": true, "group": "admin" } }
    }
  ],
  "summary": { "info": 0, "warning": 0, "error": 1 },
  "result": "failed"
}
```

Stable finding codes: `policy.valid` (info, emitted when the config complies), `policy.group_required`, `policy.lan_exposure_forbidden`, `policy.privileged_port_forbidden`, `policy.autostart_forbidden`, `policy.duplicate_binding_forbidden`. Evaluation is deterministic: per-rule findings appear in config file order (within a rule: group → LAN exposure → privileged port → autostart); duplicate-binding findings come last, one per conflicting binding, sorted by protocol → listen host → port.

**`--explain`** adds an inline explanation (code, meaning, next action, related codes) for each emitted finding, reusing the same registry as `portier explain`. In human output each finding is followed by an indented `Code:`/`Meaning:`/`What to do:` block; in `--json` an additive top-level `explanations` map (`code → { title, meaning, action, ... }`, deduplicated by code) is added for the emitted finding codes only. `--explain` does **not** change the findings, summary, result, or exit code — without it the JSON is byte-identical to before (the `explanations` map is omitted). Explanations describe what a guardrail means and what an operator can do; they never claim Portier enforces a policy or fixes a violation automatically.

**`--out <file>`** also writes the policy report to a file as deterministic pretty JSON — the **same shape as `--json`** (`findings` + `summary` + `result`, plus the additive `explanations` map under `--explain`) — for CI artifacts, reviews, and support handoff. It writes the file regardless of `--json`; in human mode stdout stays human (with a `Report written to <file>` confirmation), and in `--json` mode stdout stays pure JSON and is **byte-identical to the file**. A file-write failure is an operation failure → exit `1` (reported on stderr), overriding the report's own exit code; the file is only written after a successful marshal (no partial writes) and parent directories are not created. A malformed/unreadable config or policy still exits `2` and **writes no file**. Export never mutates the config or policy file and never contacts the runtime.

Exit codes: `0` no violations; `1` one or more violations, an `--out` write failure, or (runtime mode) a config-export/API failure; `2` missing/invalid arguments (including a missing `--out` value, or both/neither of `--config` and `--runtime`), or an unreadable/malformed config or policy file (including an unsupported `schemaVersion`); `3` (runtime mode) the Portier runtime is unreachable.

### `portier policy review --current <file> --candidate <file> --policy <file>`

A **policy-aware config review**: compare a current config with a candidate config and evaluate **only the candidate** against a policy — answering "if I moved from current to candidate, would the candidate pass this policy, and what changed?". Fully **offline and dry-run** — it reads the three files, never contacts the runtime, never probes targets, and never modifies any file except the requested `--out` file. It does **not** apply or import anything.

```
portier policy review --current <current-config> --candidate <candidate-config> --policy <policy-file> [--json] [--explain] [--out <file>]
```

The review reports a **compact change summary** (current vs candidate rule/group counts and their delta) plus the candidate's policy findings — reusing the exact `policy check` finding/report semantics (no second schema). Human output:

```
Portier Policy Review

Config changes:
- Rules: 2 → 3 (+1)
- Enabled rules: 1 → 3 (+2)
- Disabled rules: 1 → 0 (-1)
- Groups: 1 → 2 (+1)
- Ungrouped rules: 1 → 0 (-1)

Policy findings:
[INFO]  Config complies with the policy
        The config satisfies all enabled policy rules.

Summary:
  0 info
  0 warnings
  0 errors

Result: passed
```

`--json` emits `{ "review": { "current", "candidate", "delta" }, "findings": [...], "summary": {...}, "result": "passed" }` — `current`/`candidate` are the same shape as the `config doctor` config summary; `delta` is the scalar `candidate − current` difference. `--explain` adds the same additive `explanations` map as `policy check --json --explain` (and inline explanation blocks in human output) without changing the review, findings, summary, result, or exit code. `--out <file>` also writes the JSON review (byte-identical to `--json` stdout; human mode confirms with `Review written to <file>`). The change summary is intentionally compact — this is **not** a full config diff engine.

Exit codes: `0` the candidate passes the policy; `1` the candidate violates the policy, or an `--out` write failure; `2` missing/invalid arguments, or an unreadable/malformed config or policy file (including an unsupported `schemaVersion`).

### `portier policy baseline create|compare`

A dry-run **acceptance workflow**: save an accepted set of policy findings as a **baseline**, then compare later policy reports against it. Fully **offline** — both commands operate on the policy report JSON files produced by `policy check` / `policy review` (`--json` or `--out`); they never contact the runtime and never mutate inputs. A baseline is an accepted **snapshot of findings, not a config copy** (no raw config, no secrets, no runtime host data).

```
portier policy baseline create --from-report <report.json> --out <baseline.json>
portier policy baseline compare --baseline <baseline.json> --report <report.json> [--json]
```

**create** reads a policy report and writes a compact baseline (`schemaVersion: 1`, `createdAt` (RFC3339 UTC), `source`, `result`, and a fingerprinted `findings` array). The `policy.valid` "no violations" marker is excluded. Exit codes: `0` written; `1` output-file write failure; `2` usage error or an unreadable/malformed report.

**compare** classifies each finding in a fresh report against the baseline as **new** (in the report, not the baseline), **resolved** (in the baseline, not the report), or **unchanged** (in both), matched by a deterministic fingerprint:

```
Portier Policy Baseline Compare

New findings:
- policy.privileged_port_forbidden: This rule listens on a privileged port (below 1024), but the policy forbids privileged ports.

Resolved findings:
- none

Unchanged findings:
- policy.lan_exposure_forbidden: This rule listens on 0.0.0.0, but the policy forbids LAN exposure.

Result: failed
```

`--json` emits `{ "summary": { "new", "resolved", "unchanged" }, "result", "new": [...], "resolved": [...], "unchanged": [...] }`. The result is `failed` only when there are **new** findings — **resolved-only changes do not fail**. Exit codes: `0` no new findings; `1` one or more new findings; `2` usage error or an unreadable/malformed baseline or report (including an unsupported baseline `schemaVersion`).

Finding **fingerprints** are deterministic and source-independent: rule-scoped findings use `code|name|protocol|listenHost|listenPort`; duplicate bindings use `code|protocol|listenHost|listenPort|<sorted rule names>`; anything else falls back to `code|message`. A finding fingerprints identically whether its report came from offline `--config` or `--runtime`, and never depends on volatile rule IDs.

### `portier policy template <name> [--out <file>]` / `--list`

Print a built-in **policy template** (or list the available ones) so you don't have to write the policy JSON schema by hand. Fully **offline** — it never contacts the runtime and never modifies any file except the requested `--out` file. A rendered template is a complete policy file (`schemaVersion: 1`) that can be passed straight to `portier policy check --policy <file>`.

```
portier policy template --list
portier policy template <name> [--out <file>]
```

Built-in templates (sorted by name):

| Template | Purpose |
| --- | --- |
| `local-safe` | Local workstation use: forbids LAN exposure and privileged ports, keeps autostart allowed, no group requirement. |
| `managed` | Stricter managed baseline: requires groups; forbids LAN exposure, privileged ports, and autostart. |
| `permissive` | Mirrors Portier's default permissive policy — only duplicate bindings are forbidden. |

Every template uses `schemaVersion: 1` with only the five standard boolean guardrails. There is intentionally **no protocol allowlist/denylist and no `allowUdp`** — UDP is first-class.

Output:

| Command | stdout | `--out` file |
| --- | --- | --- |
| `policy template <name>` | bare policy JSON | — |
| `policy template <name> --json` | metadata wrapper `{ name, title, description, policy }` | — |
| `policy template <name> --out f` | `Policy written to f` | bare policy JSON |
| `policy template <name> --json --out f` | metadata wrapper (pure JSON) | bare policy JSON |
| `policy template --list` | compact human list | — |
| `policy template --list --json` | `{ "templates": [{ name, title, description }] }` | — |

The `--out` file always contains the **bare policy JSON** (no metadata wrapper), so it is directly usable by `policy check`. Flags may appear before or after the template name (`policy template managed --out f`).

Exit codes: `0` success; `1` an `--out` write failure (operation failure); `2` a usage error — an unknown template, a missing template name, too many names, a missing `--out` value, or `--list` combined with a name or `--out`.

Example — generate a policy and immediately check a config against it:

```
portier policy template managed --out policy.json
portier policy check --config portier.json --policy policy.json
```

### Using policy output with an AI assistant

To ask an AI assistant to help interpret a policy report, Portier ships a reusable, copy-paste prompt in [`docs/prompts/policy.md`](../../docs/prompts/policy.md). As with the doctor prompt, **Portier never sends anything anywhere** (no AI integration, upload, or telemetry) — it is plain text *you* paste into an assistant of your choice along with output you generated locally (e.g. `portier policy check --json --explain`). The policy prompt frames each finding as a **policy choice**, not necessarily a product defect, and separates safe next actions from changes that need admin/security review.

### `portier workflow plan --file <workflow.json>`

Plan and validate a local **workflow** — an ordered sequence of existing safe Portier operations described in a small JSON file. Fully **offline** and **dry-run**: it reads the workflow file, validates its schema and step references, and prints a deterministic plan. It does **not execute** any step, never contacts the runtime, never reads the files a step refers to, never applies or imports configs, never enforces a policy, and never mutates any file except the requested `--out` file.

```
portier workflow plan --file <workflow.json> [--json] [--explain] [--out <file>]
```

Workflow file format (`schemaVersion: 1`):

```json
{
  "schemaVersion": 1,
  "name": "local-policy-check",
  "steps": [
    { "id": "check-current", "type": "policy.check",
      "config": "portier.json", "policy": "local-safe.policy.json" },
    { "id": "compare-baseline", "type": "policy.baseline.compare",
      "baseline": "policy-baseline.json", "reportFrom": "check-current" }
  ]
}
```

Supported step types and their required fields:

| Step type | Required fields |
| --- | --- |
| `policy.check` | Exactly one of `config` (a file) or `runtime` (`true`), plus `policy` (a file). |
| `policy.review` | `current`, `candidate`, and `policy` (all files). |
| `policy.baseline.compare` | `baseline` (a file), plus exactly one of `report` (a file) or `reportFrom` (the id of an earlier step). |

Validation rules: `schemaVersion` must be `1`; at least one step is required; every step needs a unique, non-empty `id`; the step `type` must be supported with its required fields present; a `reportFrom` must reference a step that appears **earlier** in the file. Unknown fields anywhere in the file are **rejected** (to catch typos). Planning is structural only — it validates references; it does **not** open or parse the files a step refers to (so a missing referenced file does not fail the plan). `name` is optional (it renders as `(unnamed)` when absent).

The plan lists each step as `[VALID]`/`[INVALID]` with the inputs it would use (valid) or the reason it is invalid, then a summary and a `Result: valid`/`Result: invalid` line. Every step also carries a stable validation **code** (e.g. `workflow.step.unknown_report_from`; valid steps use `workflow.step.valid`). `--json` emits the full plan (`{ schemaVersion, name, steps, summary, result }`); `--out <file>` also writes that JSON to a file (with `--json`, stdout and the file are byte-identical).

**`--explain`** adds an inline explanation (code, meaning, next action, related codes) for each **invalid** step, reusing the same registry as `portier explain`. In human output each invalid step is followed by an indented `Code:`/`Meaning:`/`What to do:` block; in `--json` an additive top-level `explanations` map (`code → { title, meaning, action, ... }`, deduplicated, **invalid-step codes only**) is added. A valid step needs no explanation, so it is not explained. `--explain` does **not** change the steps, summary, result, or exit code — without it the JSON is byte-identical to before (the `explanations` map is omitted), and `--json --out --explain` keeps stdout/file byte-identical. Use `portier explain <code>` to look up any workflow code directly.

Exit codes: `0` the workflow plan is valid; `1` the workflow parsed but the plan is invalid (one or more invalid steps) **or** an `--out` write failure; `2` missing/invalid arguments (including a missing `--file` or `--out` value), or an unreadable/malformed workflow file (including a missing or unsupported `schemaVersion`, or no steps). Workflow planning never contacts the runtime, so there is no connection-failure (`3`) exit code.

> This slice validates, plans, and explains workflows only — it does not run them. Step execution and reporting are deferred to a later v1.11 slice.

### `portier workflow runbook --file <workflow.json>`

Preview the ordered list of Portier CLI commands a **valid** workflow maps to — the manual commands you would run to carry it out. Still **not execution**: it parses and validates the workflow, then maps each step to a command. It does **not execute** any command, never contacts the runtime, never reads the files a step refers to (or inspects their contents), and never mutates any file except the requested `--out` file.

```
portier workflow runbook --file <workflow.json> [--json] [--out <file>]
```

Each step maps to a command preview:

| Step | Command preview |
| --- | --- |
| `policy.check` (config) | `portier policy check --config <f> --policy <f>` |
| `policy.check` (runtime) | `portier policy check --runtime --policy <f>` |
| `policy.review` | `portier policy review --current <f> --candidate <f> --policy <f>` |
| `policy.baseline.compare` (report file) | `portier policy baseline compare --baseline <f> --report <f>` |
| `policy.baseline.compare` (reportFrom) | `... --report <report-from:step-id>` + a note |

For a `reportFrom` step the runbook does **not** invent a file path — it emits an explicit `<report-from:step-id>` **placeholder** and a note telling you to replace it with the report that step produces (e.g. run that step with `--out`).

Human output is a numbered list ending in `Result: ready`. `--json` emits `{ workflow, steps: [{ id, type, command, display, notes }], summary: { total }, result }`, where `command` is the canonical argv token array and `display` is a best-effort copy/paste string (Portier never shell-executes it; tokens with whitespace/quotes are single-quoted). `--out <file>` also writes that JSON (with `--json`, stdout and the file are byte-identical).

If the workflow is **invalid**, `workflow runbook` prints the plan (the validation errors) and exits `1` — exactly like `workflow plan` — and produces **no runbook and no `--out` file**. Fix the workflow (see `workflow plan --explain`) and re-run.

Exit codes: `0` the workflow is valid and a runbook was produced; `1` the workflow parsed but the plan is invalid, **or** an `--out` write failure; `2` missing/invalid arguments (including a missing `--file` or `--out` value), or an unreadable/malformed workflow file. Runbook generation never contacts the runtime, so there is no connection-failure (`3`) exit code.

> The runbook is a **preview only** — Portier does not run the listed commands. Workflow execution is deferred to a later v1.11 slice.

### `portier workflow template <name>` / `--list`

Print a built-in **workflow template** (or list the available ones) so you don't have to write the workflow JSON schema by hand. Fully **offline** — never contacts the runtime, never executes a step, never reads referenced workflow input files, and never modifies any file except the requested `--out` file. A rendered template is a complete workflow file (`schemaVersion: 1`) that can be passed straight to `portier workflow plan --file <file>`.

```
portier workflow template --list
portier workflow template <name> [--out <file>]
```

Built-in templates (sorted by name):

| Template | Purpose |
| --- | --- |
| `policy-baseline-check` | Checks a config against a policy, then compares the resulting report against a policy baseline (two steps; the compare uses `reportFrom`). |
| `policy-check-local` | Checks a local config file against a local policy file. |
| `policy-check-runtime` | Checks the live runtime config against a local policy file. |
| `policy-review` | Compares a current config with a candidate config and evaluates the candidate against a policy. |

Every template uses `schemaVersion: 1` and only the existing validated step types (`policy.check`, `policy.review`, `policy.baseline.compare`). They are dry-run starter files — there are intentionally **no execution, scheduler, or mutation/apply/import fields**.

Output:

| Command | stdout | `--out` file |
| --- | --- | --- |
| `workflow template <name>` | bare workflow JSON | — |
| `workflow template <name> --json` | metadata wrapper `{ name, title, description, workflow }` | — |
| `workflow template <name> --out f` | `Workflow written to f` | bare workflow JSON |
| `workflow template <name> --json --out f` | metadata wrapper (pure JSON) | bare workflow JSON |
| `workflow template --list` | compact human list | — |
| `workflow template --list --json` | `{ "templates": [{ name, title, description }] }` | — |

The `--out` file always contains the **bare workflow JSON** (no metadata wrapper), so it is directly usable by `workflow plan --file`. Flags may appear before or after the template name (`workflow template policy-review --out f`).

Exit codes: `0` success; `1` an `--out` write failure (operation failure); `2` a usage error — an unknown template, a missing template name, too many names, a missing `--out` value, or `--list` combined with a name or `--out`.

Example — generate a workflow and immediately plan it:

```
portier workflow template policy-baseline-check --out workflow.json
portier workflow plan --file workflow.json
```

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
| `2` | Invalid arguments or usage error, including an invalid local **input** config file (unreadable, malformed, or failing local validation) for `config import`/`plan`/`diff`/`apply` |
| `3` | Connection failure — Portier service unreachable |
| `4` | Drift detected — used by `portier config plan --fail-on-drift` and `portier config diff --fail-on-drift` |

Policy notes (intentional, not inconsistencies):

- **Local input vs local output.** A bad local *input* config file (read/parse/validation) is a user-input error → `2`. A failed local *output* write (`config export`, `config apply --backup-out`, `diagnostics export`) is an I/O/operation failure → `1`.
- **`config validate` is a validator.** Its exit code *is* its result: `0` valid, `1` invalid **or** unreadable, `2` missing the file-path argument. So `validate` reports an invalid/unreadable file as `1`, unlike the config-consuming commands above (which use `2`).
- **Rule selectors.** A rule `<id|name>` that matches nothing exits `1` (the target does not exist, like an API 404); one that matches multiple names exits `2` (the selector is ambiguous and fixable — the matching IDs are listed).
- **API vs connection.** Any server-side rejection/error is `1`; an unreachable service is `3`.
- **`doctor` is a reporter.** Like `config doctor`, `portier doctor` always completes and emits a report, so its exit code reflects the *findings*: `0` no error-severity checks (warnings still `0`), `1` one or more error-severity checks. An unreachable runtime is reported as a `runtime.unreachable` error check and exits `1`, **not** `3` — an intentional deviation from the connection-failure policy, scoped to the doctor commands.
- **`policy check` is a reporter.** Fully offline, it always completes and emits a report, so its exit code reflects the *findings*: `0` no violations, `1` one or more violations. An unreadable/malformed config or policy file (or an unsupported `schemaVersion`) is a local **input** error → `2`, consistent with the other config-consuming commands.

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
    commands/          command handlers (the Run* functions) and connection config
    config/            local config-file domain: Rule, ParseLocal, Validate, Summary, FindDuplicateBindings, RuleDetail
    doctor/            doctor model + checks: Report/CheckResult/Severity, live + config-doctor check builders, explanation registry, Emit
    policy/            offline policy model + evaluator: Report/Finding/Severity, Parse, Evaluate, PrintHuman
    planview/          renders config plan/diff/apply API responses: PrintPlan, PrintDiff, PrintApply, PlanExitCode, ApplyExitCode
    output/            human-readable and JSON output helpers (PrintJSON, WritePrettyJSON, PrintTable, PluralWord/PluralRule)
```

The general model/result types and their pure, offline logic live in the focused
`config`, `doctor`, and `policy` packages. The `commands` package holds only the
`Run*` command handlers, which compose those packages with the API client. This
keeps `commands` about dispatch, not data types.

## Notes

- The CLI uses the management API. It does not read or write `rules.json` directly.
- Both runtimes (Go service and TypeScript server) implement the same API contract, so the CLI works with either.
- The management API defaults to `127.0.0.1:47831` and is not LAN-visible by default.
- Remote authentication is not supported in this version.
