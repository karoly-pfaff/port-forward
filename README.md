# Portier

[![PortierCI](https://github.com/karoly-pfaff/port-forward/actions/workflows/portier-ci.yml/badge.svg?branch=main)](https://github.com/karoly-pfaff/port-forward/actions/workflows/portier-ci.yml)

Portier is a local TCP/UDP port-forwarding manager for development and LAN testing. It pairs a
native Go service runtime (and a TypeScript fallback) with a React web UI and a Go CLI, all
speaking the same local REST API. The management UI/API binds to `127.0.0.1:47831` by default and
is not reachable from the LAN.

Portier is local-first: no telemetry, no cloud sync, no auto-update, and no remote/team/auth
management.

## Quick Start

```powershell
npm install          # install workspace dependencies
npm run start:dev    # run the TypeScript server + Vite client together
```

The server listens on `http://127.0.0.1:47831`; the dev client runs on `http://127.0.0.1:5173`
and proxies `/api` requests to it. During development, rules are stored in `data/forwards.json`
(override with `PORTIER_CONFIG`).

To build everything and run the bundled runtime from the repository:

```powershell
npm run build
npm run start:service   # native Go service serving the built client
npm run start:server    # TypeScript server alternative
```

Open `http://127.0.0.1:47831` to use the management UI.

## Runtimes

Portier ships two server runtimes behind one REST API contract:

- **Go service** (`service` / `service.exe`) — the preferred production runtime: small binary, no
  Node.js dependency, no warm-up.
- **Node fallback** (`server.js`) — the supported TypeScript reference runtime; requires Node.js.

Both serve the same `web/` React UI and keep `rules.json` external. See
[docs/architecture.md](docs/architecture.md) for the runtime internals, forwarding paths, and UDP
modes.

## Management UI

The UI has five views:

- **Dashboard** — status stat cards, top rules by traffic, recent activity, quick actions.
- **Forward Rules** — rules table with search, status filter, drag-to-reorder, add/edit/delete, and
  per-rule diagnose.
- **Activity Log** — in-memory event log with severity/type/rule filters, JSON export, and clear.
- **Live Connections** — read-only TCP connection and UDP session visibility.
- **Settings** — runtime info, config export/import (merge or replace), and a local diagnostics
  bundle download.
- **API Reference** — an in-app API reference generated from the canonical OpenAPI contract (opened
  from the header).

The sidebar collapses behind a hamburger button on mobile.

## Forwarding Rules

Rules are TCP or UDP port forwards (`protocol + listenHost:listenPort -> targetHost:targetPort`).
Portier recommends listen ports in the `48000-48999` range; common system/database/development
ports are warned about, not blocked, and ports outside `1-65535` are rejected.

```json
{
  "name": "Local web app",
  "protocol": "tcp",
  "listenHost": "0.0.0.0",
  "listenPort": 48001,
  "targetHost": "127.0.0.1",
  "targetPort": 3000,
  "enabled": true
}
```

UDP rules add a `udpMode` of `one-way`, `bidirectional-last-client`, or
`bidirectional-multi-client` (see [docs/architecture.md](docs/architecture.md) for the trade-offs).

**LAN exposure:** a forwarding rule on `0.0.0.0` is reachable from the LAN and may require an OS
firewall allowance. The management UI/API is separate and stays on `127.0.0.1:47831` unless you
deliberately change it — keep it local-only unless you have secured remote administration.

## Install & Packaging

Native installers and portable archives are available for Windows, macOS, and Linux. All
installers are **file-install only** — they never enable or start a service, create a scheduled
task, or touch `rules.json`. See **[docs/installer.md](docs/installer.md)** for the canonical
packaging layout, platform install paths, release artifacts, checksums, and the upgrade-safety
guarantees, and **[docs/upgrade-v2.md](docs/upgrade-v2.md)** for upgrading from v1.x.

Per-platform service install/run notes:

- [scripts/windows/readme.md](scripts/windows/readme.md) — Windows service / scheduled task and MSI.
- [scripts/macos/readme.md](scripts/macos/readme.md) — macOS LaunchAgent and `.pkg`.
- [scripts/linux/readme.md](scripts/linux/readme.md) — Linux systemd and `.deb`/`.rpm`.

## CLI & Replay

The Go `portier` CLI manages a running service from the terminal; it is a pure API client, not a
second runtime. The separate offline `replay` tool analyzes saved Portier artifacts without
contacting a runtime.

```powershell
npm run build:cli       # build the CLI
npm run build:replay    # build the replay tool
```

See [tools/cli/readme.md](tools/cli/readme.md) and [tools/replay/readme.md](tools/replay/readme.md)
for full command, flag, and exit-code documentation.

## API & Postman

- Full contract: **[docs/api-contract.md](docs/api-contract.md)**.
- Canonical OpenAPI document: **[docs/openapi.json](docs/openapi.json)** (the in-app API Reference
  is generated from it).
- Ready-to-run Postman collection: **[postman/](postman/)** (`collection.json` + `environment.json`),
  generated from the OpenAPI contract. `npm run validate:postman` checks it for drift in CI, and
  `npm run validate:postman:local` runs it against a throwaway local runtime with Newman.

## Build & Validation

Everyday checks:

```powershell
npm run lint
npm run typecheck
npm run test
npm run build
npm run check          # version:check + lint + typecheck + test
```

Broader contract, coverage, packaging, and platform-service validation is documented in
**[docs/checklist.md](docs/checklist.md)**, the practical QA checklist for development, release
candidates, and platform release work.

## Documentation

- [docs/readme.md](docs/readme.md) — documentation index.
- [docs/architecture.md](docs/architecture.md) — runtime internals, forwarding, UDP modes, activity log.
- [docs/installer.md](docs/installer.md) — packaging, native installers, release artifacts, upgrade safety.
- [docs/upgrade-v2.md](docs/upgrade-v2.md) — upgrading from v1.x to v2.0.
- [docs/recovery.md](docs/recovery.md) — startup/config recovery behavior.
- [docs/api-contract.md](docs/api-contract.md) — REST API contract.
- [docs/glossary.md](docs/glossary.md) — canonical terminology.
- [docs/checklist.md](docs/checklist.md) — validation and release checklist.
- [docs/roadmap.md](docs/roadmap.md) — release direction and the road to 2.0.
- [docs/changelog.md](docs/changelog.md) — release history.
- [docs/agentic.md](docs/agentic.md) — coding-agent setup, `AGENTS.md`/`CLAUDE.md`, and helper hooks.

## Release Status

The current stable release is **v2.0.0** — the stable local-first milestone (see the
[changelog](docs/changelog.md)). It builds on the v1.x migration/recovery, install/upgrade, and
RC-hardening work; the REST API, the OpenAPI schema (beyond version metadata), and the `rules.json`
format are unchanged from v1.19.

Release workflows build and validate platform artifacts on demand and upload them for inspection;
tagging and GitHub Release publishing remain explicit manual steps — no workflow publishes a Release
or creates a tag automatically.

- Release history: [docs/changelog.md](docs/changelog.md).
- Packaging and release process: [docs/installer.md](docs/installer.md) and [docs/checklist.md](docs/checklist.md).
- Roadmap: [docs/roadmap.md](docs/roadmap.md).

## Credits

Portier was built with human direction and AI assistance, with the human firmly in the loop. See
[Credits](docs/credits.md).
