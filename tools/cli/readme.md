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

### Rule identity

All commands that target a rule accept:
- An exact rule ID (preferred; always unambiguous)
- An exact rule name (succeeds only if the name is unique across all rules)

If multiple rules share the same name, the command exits with code 2 and lists the matching IDs on stderr. Use the rule ID in that case.

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

## Building

```powershell
npm run build:cli
```

Output: `tools/cli/build/portier` (or `portier.exe` on Windows).

## Testing

```powershell
npm run test:cli
```

Runs `go test ./...` inside `tools/cli/`. Uses `httptest` for API client tests — no running Portier service required.

## Validation

```powershell
npm run validate:cli
```

Runs tests then builds. Fails clearly if Go is unavailable.

## Planned commands (not yet implemented)

Future slices will add:

- `portier config export --out <file>` — export rules config
- `portier config import <file> --mode merge|replace` — import rules config
- `portier diagnostics export --out <file>` — export diagnostics bundle

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
