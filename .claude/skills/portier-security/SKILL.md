# Portier Security Review

Use this skill for a dedicated security pass on Portier changes. Focus on exposure, privilege, input handling, and resource safety. Cite specific files and lines.

## When To Use

Use before shipping any change that touches:
- Management API bind address or authentication
- Forward rule host/port input handling
- Service install scripts (privilege escalation paths)
- Static file serving or URL routing
- Packaging or binary distribution

## Exposure Checks

**Management API**
- Default bind is `127.0.0.1:47831`. Binding to `0.0.0.0` or any non-loopback address must produce a prominent danger warning at startup and in the UI. Never silently allow LAN exposure of the management API.
- There is no authentication on the management API by design (localhost-only trust). Confirm this assumption holds.

**Forward rules**
- Rules with `listenHost: "0.0.0.0"` expose a port on all interfaces (LAN + external). The UI must show a clear LAN exposure warning for every such rule.
- Document which ports are forwarded and to where. Never auto-create forward rules without user action.

## Input Validation Checks

- Port numbers: must be integers in `1–65535`. Reject 0 and values above 65535.
- Hostnames and IPs: must pass strict validation in `shared/sources/validation`. No raw user strings passed to DNS resolution or socket connect without validation.
- Protocol: must be exactly `"tcp"` or `"udp"`. Reject anything else.
- Duplicate bindings: `protocol + listenHost + listenPort` uniqueness enforced before bind.
- No SSRF risk: target host/port is user-controlled but only used for outbound TCP/UDP, not for HTTP requests. Confirm no HTTP fetch is made to the target.

## Socket And Resource Safety

- Every TCP socket must have both `close` and `error` listeners to prevent unhandled exceptions.
- Every TCP server must call `.close()` on shutdown; lingering handles block process exit.
- Go service: every goroutine spawned for a connection must read from a done channel or context to guarantee termination.
- UDP: sockets bound per rule must be unbound on rule stop. The bidirectional-last-client model must be clearly documented in comments and docs.
- No socket or file handle leaks on error paths — test that `close` is called even when the other side errors.

## Privilege And Service Checks

- Windows install scripts must check for Administrator privilege and fail clearly if absent (`#Requires -RunAsAdministrator` or explicit `net session` check).
- Linux systemd unit must run as a non-root user where possible. If root is required (binding to low ports), document why.
- macOS LaunchAgent must run as the logged-in user, not root.
- The service binary must not execute arbitrary shell commands derived from rule configuration.
- `sc.exe`, `launchctl`, `systemctl` usage must be explicit and auditable — no dynamic command construction from user input.

## Static File Serving

- `--static-dir` path is resolved at startup and must not allow traversal outside the configured directory.
- No path traversal: requests for `../` or absolute paths must be rejected by the file server.
- Verify the Go service uses `http.FileServer` with a scoped `http.Dir`, not a raw path join.

## Secrets And Supply Chain

- No hardcoded secrets, API keys, passwords, or tokens in any committed file.
- `rules.json` is runtime user config; it must never be committed.
- `.env` and `.env.*` files are gitignored.
- Dependencies: `npm audit` for Node packages; `go list -m all | govulncheck` for Go packages (if available). Flag any HIGH or CRITICAL CVEs.
- Packaging scripts must not download binaries from the internet at build or install time.

## Output Format

```markdown
# Portier Security Review

## Summary
- Overall status: clear / concerns / blocked
- Highest severity finding:
- Areas reviewed:

## Findings
### Critical
### High
### Medium
### Low / Info

## Mitigations recommended

## What was not checked and why
```
