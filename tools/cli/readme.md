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

### `portier runtime`

Show runtime info for the running Portier service.

```
portier runtime [--json]
```

Calls `GET /api/runtime`. Displays name, version, runtime, platform/arch, uptime, management URL, config path, static dir, and service mode.

With `--json`: prints the raw `RuntimeInfo` JSON from the API.

Exit codes:
- `0` — success
- `3` — could not connect to the management API

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

- `portier list` — list all forwarding rules
- `portier status` — show rule statuses
- `portier activity` — show recent activity
- `portier start <id|name>` — start a rule
- `portier stop <id|name>` — stop a rule
- `portier diagnose <id|name>` — run diagnostics
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
