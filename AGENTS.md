# Portier Agent Guide

Portier is a local TCP/UDP port-forwarding manager for development and LAN testing.
This file is intentionally short so coding agents can absorb the project rules quickly.
Put detailed history in `docs/changelog.md`, `docs/roadmap.md`, `docs/checklist.md`, or
`audits/`, not here.

## Repository Map

- `shared/sources`: shared TypeScript types, validation, constants, and port advisories.
- `server/sources`: TypeScript service, REST API, JSON config store, and TCP/UDP forwarding.
- `service/sources`: native Go service runtime, preferred for production packages.
- `client/sources`: React TypeScript web UI.
- `tools/cli`: Go `portier` CLI. It is a pure API client unless a command is explicitly offline.
- `tools/replay`: Go replay tooling.
- `scripts`: build, release, validation, and platform service automation.
- `docs`: durable product docs. `docs/glossary.md` is canonical for user-facing terms.
- `build`, `coverage`, `test-results`, and package output directories are generated.

Use `sources`, not `src`; `build`, not `dist`; lowercase normal docs; uppercase only for
tool-required files such as `AGENTS.md`, `CLAUDE.md`, `SKILL.md`, and root `README.md`.

## Runtime Model

- `service/` is the preferred production runtime. Default static dir: `web`.
- `server/` is the TypeScript reference/fallback runtime. Default static dir: `client/build`.
- Both runtimes implement the same REST API contract and must stay behaviorally aligned for
externally observable behavior.
- The management UI/API defaults to `127.0.0.1:47831`. Do not expose it on `0.0.0.0` by default.
- Packaged layout is:

```text
<install-dir>/
  portier          (or portier.exe on Windows)
  service          (or service.exe on Windows)
  server.js        (Node fallback, requires Node.js)
  web/
    index.html
    assets/
  readme.txt
```

Runtime config stays external. Never bake `rules.json` into binaries or packages.

## Common Commands

Setup:

```powershell
npm install
```

Development:

```powershell
npm run start:dev
npm run dev -w server
npm run dev -w client
```

Default validation:

```powershell
npm run lint
npm run typecheck
npm run test
npm run build
npm run check
```

Focused validation:

```powershell
npm run test:shared
npm run test:server
npm run test:client
npm run test:service
npm run test:cli
npm run validate:cli
npm run validate:replay
```

E2E tests live in `tests/e2e/` and run against the TypeScript server plus built React client:

```powershell
npm run test:e2e:install
npm run build:client
npm run test:e2e
npm run test:e2e:fresh
```

E2E binds `127.0.0.1:47890`. Do not add E2E to `npm run test` or `npm run check`.

Additional suites are explicit and should be selected when relevant:

```powershell
npm run validate:config
npm run validate:contract
npm run validate:openapi:go
npm run validate:runtime:smoke
npm run validate:binary
npm run validate:scripts
npm run validate:coverage
npm run validate:service:current
```

Platform service validation is target-platform work and must not touch production installs:

```powershell
npm run validate:service:windows:user
npm run validate:service:windows:machine
npm run validate:service:macos
npm run validate:service:linux
```

Coverage gates are canonical in `scripts/validate-coverage.js`. Do not duplicate threshold
history here; update the script and durable docs together when gates change.

## Versioning

The version has **one source of truth** — the root `package.json` `version` — and one script
that keeps every other surface in lockstep. Never hand-edit a version string; that is how the
client sidebar shipped `v1.14.1` across four releases.

```powershell
npm run version:list          # show every surface's current value
npm run version:check         # verify all surfaces match root (runs in CI + `npm run check`)
npm run version:set 1.19.0    # write the version everywhere (semver required)
npm run version:bump minor    # next major|minor|patch from root, then set
```

Managed surfaces (full version): root/`client`/`server`/`shared` `package.json`,
`shared/sources/index.ts` `PORTIER_APP_VERSION`, and the three Go `version.go` files
(`service`, `tools/cli`, `tools/replay`). OpenAPI major.minor: `OPENAPI_DOC_VERSION` in
`server/sources/openapi/openapi.ts` and `docs/openapi.json` `info.version`. The list lives in
`scripts/generate-version.js` (`SURFACES`) — add an entry there to bring a new surface under the tool.
`version:set` patches `docs/openapi.json` `info.version` in place; run `npm run apidoc:generate`
only when the API schema itself also changed. `package-lock.json`'s top-level `version` is
intentionally unmaintained — leave it.

## Contract And Architecture Rules

- Use `docs/glossary.md` terms for docs, API docs, CLI, and UI copy. Do not rename frozen public
  API fields or paths for cosmetics: `/api/forwards`, `listenHost`, `targetHost`,
  `clientAddress`, and `targetAddress` are intentionally stable.
- Shared TypeScript validation/advisory wording is canonical where both runtimes expose the same
  result. Keep Go parity covered by `validate:contract`.
- API changes require updates to `docs/api-contract.md`, `client/sources/features/apidocs/ApiDocsView.tsx`,
  and corresponding tests.
- Config plan/apply orchestration belongs beside the plan engines
  (`server/sources/config-plan.ts`, `service/sources/configplan/`), not buried in HTTP handlers.
- Config import/apply must be atomic: validate first, reject duplicate listen bindings
  (`protocol + listenHost + listenPort`), preserve IDs where specified, and never report success
  when an import result contains errors.
- Rule managers must roll back in-memory and running-state changes after persistence failures.
- Go rule ID generation uses `domain.NewRuleID()`. Do not reintroduce per-package UUID helpers.
- Go lifecycle management goes through `forwarders.Forwarder` and `forwarders.NewForwarder`.
  Protocol dispatch belongs in the factory, not duplicated across manager methods.
- Rule-scoped activity events should be emitted through the local helper/facade in that runtime so
  event envelopes cannot drift. Activity event names/severities are public diagnostics; update
  shared types, Go constants, docs, emitters, and contract validation together.
- Diagnose output is observable. Keep check order, IDs, severity, labels, messages, and details
  mirrored across the TypeScript and Go runtimes.
- Forward rule `group` is optional behavior-neutral metadata except for explicit group actions.
  It must not affect duplicate-binding, forwarding lifecycle, or status.
- Group start/stop operations act over existing rules in rule order. They do not mutate rule
  definitions, enabled state, groups, or ordering.
- Rule `health` is derived from existing runtime state (`enabled`, `running`, `lastError`).
  It must not imply target probing, monitoring, auto-restart, or mutation.
- Client-only conveniences such as duplicate-rule, filtering, menus, and config-preview polish must
  not change API or runtime semantics.
- CLI commands follow the documented exit-code policy in `tools/cli/readme.md`. Keep the CLI a pure
  API client unless a command is explicitly designed to be offline.
- Doctor and policy tooling is read-only and deterministic. It must not mutate config/runtime,
  probe forwarding targets, collect env/process/log/secrets, or add telemetry.

## Safety Rules

- Treat `0.0.0.0` as LAN exposure and warn clearly.
- Do not silently change firewall, service, launchd, systemd, scheduled task, or SCM behavior.
- Do not add telemetry, remote updates, downloads, or secret collection.
- Do not modify user config files unless the task explicitly asks for it.
- Do not edit generated outputs or build artifacts directly; regenerate from sources.
- Service scripts and installer templates are sensitive. Edit them only for tasks about service or
  packaging behavior, and run the relevant validation.

## Files To Avoid Unless Asked

- `node_modules/`
- `build/`, `server/build/`, `service/build/`, `client/build/`, `shared/build/`
- `coverage/`
- `.git/`
- `.env`, `.env.*`, `*.log`, `rules.json`
- `test-results/`, `playwright-report/`
- generated package output under `build/windows/`, `build/macos/`, `build/linux/`
- production install/config examples such as `C:\ProgramData\Portier` except docs/templates

## Testing Expectations

- Run the narrowest relevant tests first, then broaden when the change affects shared contracts,
  multiple runtimes, packaging, or user-facing workflows.
- Use `validate:contract` for externally observable API/runtime parity.
- Use `validate:config` for config compatibility and import/export behavior.
- Use E2E for browser-visible workflows, especially forwarding, settings import/export, live
  connections, mobile navigation, and UI error states.
- Socket and timing tests should use free-port helpers, deadline polling, and Playwright auto-waiting.
  Avoid fixed sleeps.
- Prefer black-box behavior tests for CLI and UI. Use white-box tests when they protect important
  helper branches without making tests brittle.

## Keeping This File Useful

- Keep this guide evergreen and compact. If a rule needs slice history, rationale, measurements, or
  a long exception list, link to an audit or product doc instead of pasting it here.
- Historical release summaries belong in `docs/changelog.md`; future plans in `docs/roadmap.md`;
  validation matrices in `docs/checklist.md`; deep audit findings in `audits/`.
- When adding a new durable rule, write it as a short invariant plus the validation command that
  protects it. Avoid version/slice labels unless the label is needed to find the source document.
