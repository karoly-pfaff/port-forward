# Portier Claude Code Guide

## Project Identity

- App name: Portier
- Repository name: portier-port-forwarding
- Purpose: local TCP/UDP port forwarding manager for development and LAN testing

## Important Naming Conventions

- Use `sources`, not `src`.
- Use `build`, not `dist`.
- Use `deploy`, not `deployment`.
- Use `scripts` for executable automation.
- Use lowercase filenames for normal markdown docs.
- Keep tool-required files uppercase: `AGENTS.md`, `CLAUDE.md`, and `SKILL.md`.
- Keep `README.md` file in the root uppercase.
- React component and view files under `client/sources/` use **CamelCase** filenames (e.g., `ForwardRuleList.tsx`, `StatCard.tsx`, `Sidebar.tsx`).
- Non-component files (utilities, types, config) keep the existing repo convention (e.g., `format.ts`, `nav.ts`, `portierApi.ts`).

## Architecture

- `server/sources` handles the Node.js service, REST API, config persistence, and forwarding lifecycle.
- `service/sources` handles the native Go service runtime focused on smaller binaries and service deployment.
- `client/sources` handles the React UI only.
- `shared/sources` owns types, validation, port constants, and port advisory logic.
- `deploy` contains service examples, docs, and templates only.
- `scripts` contains executable scripts.

## Coding Rules

- Prefer small, readable changes.
- Do not rewrite the entire app unless explicitly asked.
- Keep TypeScript simple and explicit.
- Keep TCP and UDP forwarding logic understandable and testable.
- Keep runtime config external.
- Do not bake `rules.json` into packaged executables.
- Do not expose the management UI/API on `0.0.0.0` by default.
- The management UI/API defaults to `127.0.0.1:47831`.
- Recommended forward listen port range: `48000-48999`.
- Update docs when behavior changes.

## Security And Safety Rules

- Treat `0.0.0.0` as LAN exposure and warn clearly.
- Do not silently change firewall, service, or system-level behavior.
- Do not add telemetry.
- Do not add remote update/download behavior.
- Do not store secrets in repo files.
- Do not modify user config files unless explicitly requested.
- Do not change service scripts casually.
- Do not edit generated outputs or build artifacts directly.

## Files Claude Should Generally Avoid Editing

Do not edit these unless explicitly asked:

- `node_modules/`
- `build/`
- `server/build/`
- `service/build/`
- `client/build/`
- `shared/build/`
- `coverage/`
- `.git/`
- `.env`
- `.env.*`
- `*.log`
- `rules.json`
- `C:\ProgramData\Portier` examples except docs/templates
- generated package outputs under `build/windows/`
- `test-results/` (Playwright artifacts — gitignored, auto-generated)
- `playwright-report/` (Playwright HTML reports — gitignored)

## Preferred Validation Commands

Run the narrowest relevant validation first, then broaden before finishing:

```powershell
npm run lint
npm run typecheck
npm run test
npm run build
npm run check
```

For E2E tests (browser-level validation of user flows):

```powershell
npm run build:client       # required before running E2E
npm run test:e2e           # runs Playwright against TypeScript server on port 47890
npm run test:e2e:fresh     # build:client + test:e2e combined
```

E2E install (one-time): `npm run test:e2e:install`

Do not add `test:e2e` to `npm run test` or `npm run check`. E2E is a separate step.

**Protocol coverage (automated E2E — do not revert to manual QA):**
- TCP real forwarding: `tests/e2e/tcp.spec.ts`
- UDP one-way, bidirectional-last-client, bidirectional-multi-client: `tests/e2e/udp.spec.ts`

**Remaining manual QA:** Firewall and OS permission behavior only.

**OS service install validation is automated — run explicitly before release:**

```powershell
npm run check:service:windows:user     # Windows scheduled task (no admin)
npm run check:service:windows:machine  # Windows Service (admin)
npm run check:service:current          # current platform
```

```bash
npm run check:service:macos   # macOS LaunchAgent (no sudo)
npm run check:service:linux   # Linux systemd (sudo)
```

These use test-specific names and temp dirs. Never touch production installs.
Do not add these to `npm run check`. Run them explicitly when releasing.

**Package build is automated — do not treat it as manual QA:**

```powershell
npm run check:package           # validate existing build/portier/ layout
npm run check:package:build     # build then validate
npm run check:package:smoke     # build, validate, and run smoke test (preferred)
```

If a task touches packaging, run `npm run check:package:smoke` when possible. The script:
- Builds `build/portier/` via `package:portier` on the current platform
- Validates the layout (`service`/`service.exe`, `server.js`, `web/`, `readme.txt`)
- Smoke-tests the packaged binary on a free port without requiring admin/root

The platform-specific scripts still output to their own dirs when called directly:

```powershell
npm run package:windows     # Windows: produces build/windows/
npm run package:macos       # macOS/cross-compile: produces build/macos/
npm run package:linux       # Linux/cross-compile: produces build/linux/
```

If packaging cannot run because prerequisites are unavailable (e.g., Go is not installed), document that limitation clearly.

If a task touches formatting, run `npm run format` if available. Otherwise use `npm run lint` or the Prettier commands defined in `package.json`.

## UI Cleanup Guidance

- Keep the interface simple and practical.
- Prioritize clarity over visual flair.
- Make rule status obvious: running, stopped, error.
- Make protocol obvious: TCP or UDP.
- Make listen endpoint and target endpoint readable.
- Show LAN exposure warning for `0.0.0.0`.
- Show common port warnings inline.
- Keep add/edit rule flows compact.
- Do not hide dangerous states behind subtle styling.
- Avoid complex UI frameworks unless already present.
- Prefer accessible HTML controls and clear labels.

## Packaged Runtime Layout

Production/install layout for all platforms:

```text
<install-dir>/
  service          (or service.exe on Windows)
  server.js        (Node fallback — requires Node.js)
  web/
    index.html
    assets/
```

- `service` / `service.exe` = native Go runtime; default static dir is `web`
- `server.js` = bundled Node/TypeScript fallback
- `web/` = built React client UI (external; not baked into binary)
- `rules.json` = always external; never packaged

Dev build output (repo-internal, not distributed): `service/build/portier-service`, `server/build/`, `client/build/`.

Packaging scripts:
- `package:portier` → `build/portier/` (cross-platform, primary generic output)
- `package:windows` → `build/windows/`, `package:macos` → `build/macos/`, `package:linux` → `build/linux/`
- `package:clean` removes `build/portier/` and all platform package output dirs

Validation scripts:
- `validate:package` → validates `build/portier/` layout
- `validate:package:build` → builds then validates
- `validate:package:smoke` → builds, validates, and smoke-tests the packaged binary
- `validate:service:windows:user` → Windows scheduled task install/start/stop/uninstall (no admin)
- `validate:service:windows:machine` → Windows Service install/start/stop/uninstall (admin required)
- `validate:service:macos` → macOS LaunchAgent install/start/stop/uninstall (no sudo)
- `validate:service:linux` → Linux systemd install/start/stop/uninstall (root/sudo)
- `validate:service:current` → runs the appropriate platform script

## Remaining Work

Both runtimes are feature-complete. Package build correctness and OS service install/uninstall flows are automated. Remaining manual work is firewall and OS permission behavior. See `docs/checklist.md`.

## Review Checklist

- TypeScript types are strict and useful.
- TCP sockets clean up on close/error.
- UDP behavior is documented, especially bidirectional-last-client limitations.
- Duplicate `protocol + listenHost + listenPort` bindings are rejected.
- Management API remains localhost by default.
- Port advisory rules live in `shared/sources`.
- Tests cover validation and lifecycle behavior where practical.
- `README.md`, `docs/architecture.md`, and `docs/checklist.md` are updated for user-visible behavior changes.

## Claude Response Style

- Summarize changed files.
- Mention validation commands run.
- Mention anything not run and why.
- Call out risks and follow-up tasks.
- Do not claim success unless validation passed or limitations are stated.

## Claude Code Hooks And Settings

Claude Code project settings live in `.claude/settings.json`. Hook event names and schemas can vary by Claude Code version, so treat the hook wiring as conservative project guidance. If a local Claude Code install rejects the hook section, keep the scripts and adjust only the event names/schema for that installed version.
