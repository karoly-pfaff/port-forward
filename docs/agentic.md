# Agentic Coding Setup

Portier includes agent support files (currently Claude Code) so coding agents have repo-specific guardrails before editing.

## Root Guidance

`CLAUDE.md` describes the project identity, repository layout, validation commands, security expectations, and review checklist. Claude Code reads this file as project guidance when working in the repository.

## Project Settings

`.claude/settings.json` is a conservative best-effort Claude Code settings file. It allows normal edits in `server/sources`, `client/sources`, `shared/sources`, docs, deploy docs/templates, scripts, and top-level project docs while denying edits to dependency, generated, build, coverage, Git, secret, local config, and log paths.

The settings also call out service scripts as paths that should only be edited when the task is specifically about service behavior.

Claude Code settings schemas and hook event names can vary by installed version. The helper scripts are intentionally left manual in settings until the installed Claude Code version's hook schema is verified.

## Hooks

Hook scripts live under `.claude/hooks/`.

- `format-changed-files.js` looks for changed `.ts`, `.tsx`, `.json`, `.css`, and `.md` files and formats them only if a local Prettier binary exists. It skips `node_modules`, `build`, `coverage`, and `.git`.
- `validate-after-task.js` detects which source areas changed and runs the matching subset: `npm run lint`, `npm run typecheck`, and `npm run test:shared`/`test:server`/`test:client` for TypeScript changes; `go vet ./...` and `go test ./...` for Go changes. It exits non-zero when validation fails.

Run them manually from the repository root:

```powershell
node .claude/hooks/format-changed-files.js
node .claude/hooks/validate-after-task.js
```

The hooks do not run `npm run build:runtime:windows` automatically. Packaging is heavier and should be run only when explicitly requested or when a packaging task needs it.

## Code Audit Skill

The reusable Portier audit skill lives at:

```text
.claude/skills/portier-code-audit/SKILL.md
```

Ask Claude Code to use the `portier-code-audit` skill when reviewing changes. The audit checks source boundaries, TCP/UDP lifecycle behavior, duplicate binding validation, management UI/API exposure, packaging expectations, and documentation/test coverage.

## Local Helpers

Useful validation commands:

```powershell
npm run check
npm run build
```

E2E tests (requires `npm run build:client` first):

```powershell
npm run test:e2e
npm run test:e2e:fresh   # build:client + test:e2e combined
```

Package smoke test (preferred pre-release package check):

```powershell
npm run validate:runtime:smoke   # build, validate layout, smoke-test binary
```

Go service validation (when Go is installed):

```powershell
go -C service vet ./...
go -C service test ./...
go -C service build ./...
npm run build:service
```

## Generated Files

Do not edit generated/build artifacts directly. Regenerate them from source instead. In particular, avoid direct edits under:

- `node_modules/`
- `build/`
- `coverage/`
- `build/windows/`
- `build/macos/`
- `build/linux/`
