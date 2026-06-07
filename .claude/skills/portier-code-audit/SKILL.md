# Portier Code Audit

Use this skill to review Portier changes for architecture, networking, security, packaging, and quality risks. Prioritize concrete findings over broad summaries. Cite files and lines where possible.

## When To Use

Use when asked to audit, review, or assess changes in Portier — especially changes touching forwarding logic, validation, management API/UI exposure, service deployment, packaging, or shared contracts.

## Audit Workflow

1. Identify which areas are touched: `server/`, `service/` (Go), `client/`, `shared/`, `deploy/`, `scripts/`, docs, or packaging.
2. Inspect the changed files and their relevant surrounding context.
3. Run the narrowest applicable validation first, then broaden:
   - TS/JS changes → `npm run lint`, `npm run typecheck`, `npm run test`
   - Go changes → `go -C service vet ./...`, `go -C service test ./...`
   - Both → run all of the above
   - Packaging changes → `npm run validate:runtime:smoke` (preferred; no admin/root required); falls back to `npm run build:runtime:windows` on Windows or document the skip
4. Report findings by severity. Do not bury correctness or security risks in a summary.
5. Note what was run, what failed, and what was skipped with reasons.

## Architecture Checks

- `server/`, `client/`, `service/`, and `shared/` boundaries are respected.
- Shared validation and port advisory logic live in `shared/sources` — not duplicated in server or Go service.
- TCP and UDP forwarders are separated cleanly in both the Node server and Go service.
- `ForwardManager` (Node) and `manager` package (Go) own lifecycle orchestration.
- Activity log lives in `server/sources/activity/` (Node) and `service/sources/activity/` (Go); both must be kept in parity.
- Go service and Node server implement the same REST API contract (see `docs/api-contract.md`).

## Networking Checks

- TCP sockets are cleaned up on both error and close events.
- Backpressure is handled through stream piping or an explicit mechanism.
- UDP bidirectional-last-client limitation is documented; code must not claim full multi-client UDP.
- Duplicate `protocol + listenHost + listenPort` bindings are rejected.
- Shutdown closes all active sockets and servers — no lingering handles.
- Go service: goroutines spawned for connections must terminate on context cancellation or connection close.

## Security Checks

- Management UI/API defaults to `127.0.0.1`; binding to `0.0.0.0` produces strong danger messaging.
- Forward rules listening on `0.0.0.0` show clear LAN exposure warnings in the UI.
- No secrets, tokens, or credentials committed to source.
- No telemetry or remote update/download behavior.
- No automatic firewall or system-level changes without explicit user action.
- Input validation: port numbers in range 1–65535, hostnames are valid, no SSRF-capable proxy behavior.
- Static file serving is bounded to the configured `--static-dir`; no path traversal.

## Service And Packaging Checks

- Windows service install scripts require explicit Administrator elevation (`#Requires -RunAsAdministrator` or equivalent check).
- `sc.exe` is used directly; it is not downloaded.
- Config path defaults: `C:\ProgramData\Portier` (Windows), `/etc/portier` or `/var/lib/portier` (Linux/macOS).
- `rules.json` is never baked into the packaged binary.
- `web/` static dir is external — not embedded in the binary.
- Go service cross-compiled with `CGO_ENABLED=0` for portability.
- Packaged layout: `service`/`service.exe` + `server.js` + `web/` flat in install dir. Dev uses `--static-dir ../client/build`.
- `service/build/` and all `build/` output dirs are gitignored and not committed.

## Quality Checks

- TypeScript is strict and types are useful, not `any`-escaped.
- Go code passes `go vet` with no warnings.
- Errors are clear, actionable, and surface to the user or logs appropriately.
- Tests cover: validation rules, duplicate binding rejection, TCP/UDP lifecycle, config store, and activity log.
- Docs updated for any user-visible behavior change.
- No generated/build artifacts edited directly.

## Output Format

```markdown
# Portier Code Audit

## Summary
- Overall status: pass / needs work / blocked
- Main risk:
- Validation run:

## Findings
### Critical
### High
### Medium
### Low / Info

## Suggested next fixes

## Validation notes
```
