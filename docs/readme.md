# Portier Docs

Project documentation index.

## Architecture and Design

- [architecture.md](architecture.md) — monorepo layout, server runtimes, package layout, forwarding internals, UDP modes, activity log, platform service support.
- [api-contract.md](api-contract.md) — full REST API contract: endpoints, request/response shapes, static file serving, error format.

## Releases

- [changelog.md](changelog.md) — version history and notable changes per release.

Raw release readiness audit reports live in `audits/` (gitignored, local only).

## Platform Deployment

- [scripts/windows/readme.md](../scripts/windows/readme.md) — Windows service install, packaging, both runtimes.
- [scripts/macos/readme.md](../scripts/macos/readme.md) — macOS LaunchAgent install, Go service and Node fallback modes.
- [scripts/linux/readme.md](../scripts/linux/readme.md) — Linux systemd unit, both runtimes, firewall notes.

## QA

- [checklist.md](checklist.md) — automated and manual platform QA checklist.

## Agent and Tooling

- [agentic.md](agentic.md) — agent support files: CLAUDE.md/AGENTS.md guidance, `.claude/` settings, hooks, audit skill, and local helper commands.
