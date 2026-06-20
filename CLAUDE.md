# Portier Claude Code Guide

Read `AGENTS.md` first. This file keeps Claude-specific working preferences and a short copy of
the highest-impact project rules. Keep it compact; long slice history belongs in `docs/roadmap.md`,
`docs/changelog.md`, `docs/checklist.md`, or `audits/`.

## Project

- Portier is a local TCP/UDP port-forwarding manager for development and LAN testing.
- `service/` is the native Go production runtime; `server/` is the TypeScript reference/fallback.
- Both runtimes expose the same REST API contract.
- `client/` is the React UI, `shared/` owns shared types/validation/advisories, and `tools/cli`
  is the Go `portier` CLI.
- Runtime config stays external. Do not package or overwrite `rules.json` casually.

## Naming And Layout

- Use `sources`, not `src`; `build`, not `dist`.
- Use lowercase filenames for normal markdown docs.
- Keep tool-required files uppercase: `AGENTS.md`, `CLAUDE.md`, `SKILL.md`, root `README.md`.
- React component/view files under `client/sources/` use CamelCase filenames.
- Go CLI command files under `tools/cli/sources/commands/` are named by command responsibility
  (`config.go`, `doctor.go`, `policy.go`, etc.), not `cmd` suffixes.
- Use canonical wording from `docs/glossary.md`. Do not rename stable API paths or fields for
  cosmetic consistency.

## Architecture Guardrails

- Management UI/API defaults to `127.0.0.1:47831`; do not expose it on `0.0.0.0` by default.
- API changes update `docs/api-contract.md`, the in-app API Docs view, and tests.
- Cross-runtime behavior changes need TypeScript and Go coverage plus `npm run validate:contract`.
- Config plan/apply logic belongs in the plan-engine helpers, not HTTP handlers.
- Mutating manager operations must validate before mutation and roll back after persistence failure.
- Duplicate listen bindings are defined by `protocol + listenHost + listenPort`.
- Go service rule IDs come from `domain.NewRuleID()`.
- Go forwarding lifecycle uses `forwarders.Forwarder` via `forwarders.NewForwarder`.
- Diagnose checks, config plan/apply responses, activity event values, rule groups, group actions,
  and rule health are observable behavior. Keep runtime parity where applicable.
- CLI commands are pure API clients unless explicitly offline. Follow the exit-code policy in
  `tools/cli/readme.md`.
- Doctor and policy commands are deterministic, read-only diagnostics. They must not mutate files or
  runtime state, probe forwarding targets, collect secrets/env/process/logs, or add telemetry.
- The version has one source of truth (root `package.json`). Never hand-edit a version string —
  use `npm run version:set <x.y.z>` / `version:bump`; `version:check` guards every surface in CI.
  See `AGENTS.md` › Versioning. The `/portier-version` skill wraps this.

## Safety

- Treat `0.0.0.0` as LAN exposure and warn clearly.
- Do not silently change firewall, OS service, launchd, systemd, scheduled task, or SCM behavior.
- Do not add telemetry, remote update, or download behavior.
- Do not store secrets in repo files.
- Do not modify user config files unless explicitly requested.
- Do not edit generated outputs directly.
- Service scripts and installer templates are sensitive; touch them only for service/packaging work.

## Avoid Editing Unless Asked

- `node_modules/`
- `build/`, `server/build/`, `service/build/`, `client/build/`, `shared/build/`
- `coverage/`
- `.git/`
- `.env`, `.env.*`, `*.log`, `rules.json`
- `test-results/`, `playwright-report/`
- generated package output under `build/windows/`, `build/macos/`, `build/linux/`
- production install/config examples such as `C:\ProgramData\Portier` except docs/templates

## Validation

Run the narrowest useful command first, then broaden when risk warrants it.

```powershell
npm run lint
npm run typecheck
npm run test
npm run build
npm run check
```

Focused commands:

```powershell
npm run test:shared
npm run test:server
npm run test:client
npm run test:service
npm run test:cli
npm run validate:cli
npm run validate:replay
npm run validate:config
npm run validate:contract
npm run validate:openapi:go
npm run validate:runtime:smoke
npm run validate:coverage
```

E2E:

```powershell
npm run test:e2e:install
npm run build:client
npm run test:e2e
npm run test:e2e:fresh
```

Explicit platform service validation:

```powershell
npm run validate:service:windows:user
npm run validate:service:windows:machine
npm run validate:service:macos
npm run validate:service:linux
```

Coverage gates are canonical in `scripts/validate-coverage.js`; do not preserve old threshold
history in this file.

## Frontend Guidance

- Match existing app conventions; Portier is an operational tool, not a marketing site.
- Use accessible labels, roles, and native controls where practical.
- Prefer role/label based tests over CSS selectors.
- Visible UI terminology should follow `docs/glossary.md`.
- Client-only conveniences must not change API/runtime semantics.
- When a UI change touches an E2E-covered workflow, run the relevant E2E spec or explain why not.

## Hooks And Helpers

Claude helper scripts live in `.claude/hooks/` and can be run manually:

```powershell
node .claude/hooks/format-changed-files.js
node .claude/hooks/validate-after-task.js
```

They are conservative helpers, not a substitute for task-specific validation.

## Response Style

- Be concise about what changed, what was validated, and any remaining risk.
- Mention commands that could not be run.
- For reviews, lead with findings and file/line references.
- Do not paste long release history into chat unless explicitly requested.
