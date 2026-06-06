# Agent Workflow

Portier can use Codex and Claude together, but they should work on separate scopes to avoid conflicting edits.

Both runtimes are feature-complete. The Go service in `service/` is the preferred production runtime; the TypeScript server in `server/` remains supported as reference and fallback. Remaining work is manual platform QA and post-v1.0 features tracked in `docs/checklist.md`.

## Suggested Split

Use Codex for:

- implementation slices
- tests
- build fixes
- packaging scripts
- small refactors
- endpoint/UI wiring

Use Claude for:

- architecture review
- UI cleanup
- UX polish
- documentation polish
- risk review
- code audit skill
- hook/settings maintenance

## Coordination Rules

- Avoid running Codex and Claude on the same files at the same time.
- Prefer one branch or task per agent.
- Always run validation before merging.
- Use `AGENTS.md` for general agent rules.
- Use `CLAUDE.md` and `.claude/` for Claude-specific workflow.

## Codex Task Review

The minimal Codex task review checklist lives in `.codex/skills/portier-task-review/SKILL.md`.

If local Codex skill discovery is unavailable, use `AGENTS.md` plus:

```powershell
npm run check
```

## Generated Files

Generated and dependency artifacts should not be edited directly. Regenerate outputs from source instead of changing files under `node_modules/`, `build/`, or `coverage/`.
