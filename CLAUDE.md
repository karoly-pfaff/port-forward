# Portier Claude Code Guide

## Project Identity

- App name: Portier
- Repository name: portier-port-forwarding
- Purpose: local TCP/UDP port forwarding manager for development and LAN testing

## Important Naming Conventions

- Use `sources`, not `src`.
- Use `build`, not `dist`.
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
- `scripts` contains executable scripts; each platform subdir (`windows/`, `macos/`, `linux/`) also contains its platform docs and templates.
- `tools` contains user-facing and developer-facing project tools. `tools/cli/` is the v1.3 portier CLI (a Go-based API client for the management API). Future possible tools: `tools/bench/`, `tools/replay/`. Do not place tools in `scripts/` or `service/`.

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

For CLI (Go — tools/cli/):

```powershell
npm run test:cli           # go test ./... inside tools/cli (uses httptest; no running service needed)
npm run build:cli          # builds tools/cli/build/portier[.exe]
npm run validate:cli       # test:cli + build:cli
```

E2E install (one-time): `npm run test:e2e:install`

Do not add `test:e2e` to `npm run test` or `npm run check`. E2E is a separate step.

E2E spec files:
- `tests/e2e/portier.spec.ts` — app load, CRUD, start/stop, merge import, mobile
- `tests/e2e/settings.spec.ts` — replace-mode import (v1-mixed), invalid-JSON rejection, export shape
- `tests/e2e/tcp.spec.ts` — TCP real forwarding
- `tests/e2e/udp.spec.ts` — UDP one-way, last-client, multi-client

Settings E2E intentionally does not run the full fixture matrix. `validate:config` owns exhaustive TS/Go parity testing.

**Additional validation suites (run explicitly — slower/platform-sensitive, not part of `npm run check`):**

```powershell
npm run validate:config            # fixture-based rules.json compatibility validation
npm run validate:contract          # API contract parity: TypeScript server + Go service if available
npm run validate:binary            # runtime binary behavior: build:runtime then 5 behavioral tests
npm run validate:runtime:behavior  # alias for validate:binary (fits validate:runtime:* namespace)
npm run validate:scripts           # installer script static analysis + dry-run on current platform
```

- `validate:config`: loads fixtures from `tests/fixtures/config/`; checks load, import, export, rejection, duplicate bindings, UDP defaults; TypeScript always checked, Go checked when binary present; `--skip-go` to force skip. Uses temp dirs and free ports; never reads real `rules.json`.
- `validate:contract`: skips Go parity clearly if binary not present; `--skip-go` to force skip.
- `validate:binary`: runs `build:runtime` first; use `--no-build` to reuse existing `build/portier/`.
- `validate:scripts`: always runs static analysis; dynamic dry-run only on current platform.

Do not add these to `npm run test` or `npm run check`.

Naming convention:
- `npm run test` = unit/integration test runner (Vitest + Go test)
- `npm run test:e2e` = Playwright browser E2E tests
- `npm run validate:config` = fixture-based rules.json compatibility validation
- `npm run validate:contract` = TS/Go API parity validation
- `npm run validate:binary` / `validate:runtime:behavior` = packaged service binary behavior validation
- `npm run validate:scripts` = installer/service script static + dry-run validation

**Protocol coverage (automated E2E — do not revert to manual QA):**
- TCP real forwarding: `tests/e2e/tcp.spec.ts`
- UDP one-way, bidirectional-last-client, bidirectional-multi-client: `tests/e2e/udp.spec.ts`

**Remaining manual QA:** Firewall and OS permission behavior only.

**OS service install validation is automated — run explicitly before release:**

```powershell
npm run validate:service:windows:user     # Windows scheduled task (no admin)
npm run validate:service:windows:machine  # Windows Service (admin)
npm run validate:service:current          # current platform
```

```bash
npm run validate:service:macos   # macOS LaunchAgent (no sudo)
npm run validate:service:linux   # Linux systemd (sudo)
```

These use test-specific names and temp dirs. Never touch production installs.
Do not add these to `npm run check`. Run them explicitly when releasing.

**Package build is automated — do not treat it as manual QA:**

```powershell
npm run validate:runtime           # validate existing build/portier/ layout
npm run validate:runtime:build     # build then validate
npm run validate:runtime:smoke     # build, validate, and run smoke test (preferred)
```

**macOS release archive — run explicitly when changing macOS scripts:**

```bash
npm run build:release:portable   # build:runtime then portable tar.gz (on macOS)
```

Output: `build/releases/macos/portier-portable-macos-<version>.tar.gz`. Requires `bash` and `tar`. Do not add to `npm run check` or any automated validation chain.

**Linux release archive — run explicitly when changing Linux scripts:**

```bash
npm run build:release:portable   # build:runtime then portable tar.gz (on Linux)
```

Output: `build/releases/linux/portier-<version>-linux.tar.gz`. Requires `bash` and `tar`. Do not add to `npm run check` or any automated validation chain.

**Windows release artifacts — run explicitly when changing release files:**

```powershell
npm run build:release:current    # portable zip + Inno Setup installer (installer non-fatal if absent)
npm run build:release:portable   # portable zip only
```

Output: `build/releases/windows/portier-<version>-windows-portable.zip` and `Portier-Setup-<version>.exe`. If Inno Setup is unavailable, the portable zip is still produced — report the missing installer clearly. Do not add to `npm run check` or any automated validation chain.

If a task touches packaging, run `npm run validate:runtime:smoke` when possible. The script:
- Builds `build/portier/` via `build:runtime` on the current platform
- Validates the layout (`service`/`service.exe`, `server.js`, `web/`, `readme.txt`)
- Smoke-tests the packaged binary on a free port without requiring admin/root

The platform-specific scripts still output to their own dirs when called directly:

```powershell
npm run build:runtime:windows     # Windows: produces build/windows/
npm run build:runtime:macos       # macOS/cross-compile: produces build/macos/
npm run build:runtime:linux       # Linux/cross-compile: produces build/linux/
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
- `build:runtime` → `build/portier/` (cross-platform, primary generic output)
- `build:runtime:windows` → `build/windows/`, `build:runtime:macos` → `build/macos/`, `build:runtime:linux` → `build/linux/`
- `build:clean` removes `build/portier/`, all platform package output dirs, and `build/releases/`

Validation scripts:
- `validate:runtime` → validates `build/portier/` layout
- `validate:runtime:build` → builds then validates
- `validate:runtime:smoke` → builds, validates, and smoke-tests the packaged binary
- `validate:service:windows:user` → Windows scheduled task install/start/stop/uninstall (no admin)
- `validate:service:windows:machine` → Windows Service install/start/stop/uninstall (admin required)
- `validate:service:macos` → macOS LaunchAgent install/start/stop/uninstall (no sudo)
- `validate:service:linux` → Linux systemd install/start/stop/uninstall (root/sudo)
- `validate:service:current` → runs the appropriate platform script

## Remaining Work

Both runtimes are feature-complete. Package build correctness, OS service install/uninstall flows, and release artifact generation are automated. Remaining manual work is firewall and OS permission behavior. See `docs/checklist.md`.

v1.1 is complete: distribution, installers, release artifacts, service and package validation, platform polish. Tagged 1.1.0. See `docs/installer-strategy.md` for scope and slice history.

v1.2 is complete: runtime info endpoint, rule diagnostics API and UI, Activity Log polish, safer networking UX, settings/config polish, and diagnostics export. Tagged 1.2.0. See `docs/roadmap.md` for goals, slices, and non-goals.

v1.3 is in progress: Go CLI under `tools/cli/`. Slice 2 complete: `tools/cli/` module, HTTP API client (`ConnectionError`/`APIError`), `--url`/`--host`/`--port`/`PORTIER_URL` connection options, `--json` flag, `runtime` command, `build:cli`/`test:cli`/`validate:cli` npm scripts. The CLI talks to the management API; it does not replace the web UI or any runtime. CLI binary: `portier`/`portier.exe`. See `tools/cli/readme.md` and `docs/roadmap.md`.

Release artifact commands:
- `npm run build:release:current` — portable archive + installer for current platform
- `npm run build:release:portable` — portable archive only
- `npm run validate:release:portable` — validate portable archive layout and contents
- `npm run validate:release:current` — validate portable + installer artifacts

Do not add release artifact commands to `npm run check`. They are explicit release steps.

## Review Checklist

- TypeScript types are strict and useful.
- TCP sockets clean up on close/error.
- UDP behavior is documented, especially bidirectional-last-client limitations.
- Duplicate `protocol + listenHost + listenPort` bindings are rejected.
- Management API remains localhost by default.
- Port advisory rules live in `shared/sources`.
- Tests cover validation and lifecycle behavior where practical.
- `README.md`, `docs/architecture.md`, and `docs/checklist.md` are updated for user-visible behavior changes.
- When an API endpoint is added, removed, or changed: update both `docs/api-contract.md` (durable external contract) AND the client in-app API Docs view (`client/sources/features/apidocs/ApiDocsView.tsx`) (user-facing in-app reference). Update `ApiDocsView.test.tsx` for new endpoints. Do not mark an API slice complete until both documentation surfaces and their tests are updated.

## Claude Response Style

- Summarize changed files.
- Mention validation commands run.
- Mention anything not run and why.
- Call out risks and follow-up tasks.
- Do not claim success unless validation passed or limitations are stated.

## Claude Code Hooks And Settings

Claude Code project settings live in `.claude/settings.json`. Hook event names and schemas can vary by Claude Code version, so treat the hook wiring as conservative project guidance. If a local Claude Code install rejects the hook section, keep the scripts and adjust only the event names/schema for that installed version.
