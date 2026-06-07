---
name: portier-task-review
description: Review a completed Portier task for scope, validation, safety, docs, tests, and handoff readiness.
---

# Portier Task Review

Use this skill before handing off any Portier implementation task. Work through each section and report findings concisely. Cite files and lines for any issue found.

## Scope Check

- Changed files are within the stated task scope — no unrelated edits.
- No generated or build artifact files were edited directly (`build/`, `server/build/`, `client/build/`, `service/build/`, `shared/build/`).
- No dependency directories were touched (`node_modules/`).
- No user config files were modified without explicit instruction (`rules.json`, `.env`).

## Validation

Run the narrowest applicable set and report results:

| Area changed | Commands to run |
|---|---|
| TS/JS only | `npm run lint` → `npm run typecheck` → `npm run test` |
| Go only | `go -C service vet ./...` → `go -C service test ./...` |
| Both | All of the above |
| Packaging | `npm run validate:runtime:smoke` (preferred; no admin/root required) |

- Note any command that was skipped and why.

## Test Coverage

- Tests were added or updated when observable behavior changed.
- New validation rules have corresponding test cases in `shared/` or the relevant package.
- New API endpoints have handler-level tests in `server/` or `service/sources/api/`.
- TCP/UDP lifecycle changes have socket close and error tests.

## Safety

- Management API still defaults to `127.0.0.1`; no accidental `0.0.0.0` binding introduced.
- `0.0.0.0` forward rules still produce LAN exposure warnings in the UI.
- No secrets, tokens, or credentials in committed files.
- No telemetry added.
- No automatic firewall or system-level changes introduced.
- Service install scripts still require elevation if touching system state.

## Documentation

- `README.md` updated for any user-visible behavior change.
- `docs/architecture.md` updated if structural design changed.
- `docs/api-contract.md` updated if REST API surface changed.
- `docs/checklist.md` updated if tasks are completed or new ones discovered.
- In-code comments: present only where the *why* is non-obvious.

## Node ↔ Go Parity

If the task touched the Node server (`server/sources/`) or Go service (`service/sources/`):
- Both implement the same REST endpoint and response shape.
- Activity log behavior is consistent between runtimes.
- Configuration loading follows the same logic.

## Handoff Report Shape

```markdown
## Task Review

**Status:** ready / needs fixes / blocked

**Validation run:**
- npm run lint: pass / fail / skipped (reason)
- npm run typecheck: pass / fail / skipped
- npm run test: pass / fail / skipped
- go vet: pass / fail / skipped
- go test: pass / fail / skipped

**Changed files:** (list)

**Findings:** (list issues by severity, or "none")

**Follow-up tasks:** (list, or "none")
```
