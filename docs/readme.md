# Portier Docs

Project documentation index.

## Architecture and Design

- [architecture.md](architecture.md) — monorepo layout, server runtimes, package layout, forwarding internals, UDP modes, activity log, platform service support.
- [api-contract.md](api-contract.md) — full REST API contract: endpoints, request/response shapes, static file serving, error format.
- [recovery.md](recovery.md) — startup/config/autostart recovery policy (v1.17): failure classes, recovery principles, quarantine, observability direction.
- [glossary.md](glossary.md) — canonical Portier terms and frozen public-name exceptions.
- [service/readme.md](../service/readme.md) — native Go service (preferred packaged runtime): chi API router, route modules, `app.App` container.
- [server/readme.md](../server/readme.md) — TypeScript NestJS server (reference/Node fallback runtime): modules, controllers, OpenAPI generation.

## Releases

- [changelog.md](changelog.md) — version history and notable changes per release.
- [roadmap.md](roadmap.md) — release plan and the Road-to-2.0 sequence (v1.16–v2.0).

Audit reports live in `audits/`. During active audit work the directory may contain
raw local notes; before publication, reviewed public reports stay in `audits/` and
raw/internal notes move under gitignored `audits/private/`.

## Platform Deployment

- [installer.md](installer.md) — current packaged layout, platform install paths, release artifacts, and packaging validation.
- [scripts/windows/readme.md](../scripts/windows/readme.md) — Windows service install, packaging, both runtimes.
- [scripts/macos/readme.md](../scripts/macos/readme.md) — macOS LaunchAgent install, Go service and Node fallback modes.
- [scripts/linux/readme.md](../scripts/linux/readme.md) — Linux systemd unit, both runtimes, firewall notes.

## CLI and Tools

- [tools/cli/readme.md](../tools/cli/readme.md) — the Go `portier` CLI: commands, flags, exit-code policy, doctor/policy/workflow.
- [tools/replay/readme.md](../tools/replay/readme.md) — the offline `replay` analysis tool: supported artifacts, commands, offline/read-only guarantees.

## QA

- [checklist.md](checklist.md) — automated and manual platform QA checklist.

## Agent and Tooling

- [agentic.md](agentic.md) — agent support files: CLAUDE.md/AGENTS.md guidance, `.claude/` settings, hooks, audit skill, and local helper commands.
