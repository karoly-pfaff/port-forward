# Portier Agent Guide

## Project

- App name: Portier
- Repository name: portier-port-forwarding
- Purpose: local TCP/UDP port forwarding manager for development and LAN testing

## Structure

- `server/sources` = Node.js TypeScript service, REST API, TCP/UDP forwarding engine
- `service/sources` = native Go service implementation for smaller binaries and service deployment
- `client/sources` = React TypeScript web UI
- `shared/sources` = shared types, validation, constants, port advisory utilities
- `scripts` = executable automation scripts; platform subdirs contain `release/` (artifact builders), `service/` (OS service lifecycle), and platform-specific docs/templates
- `tools` = user-facing and developer-facing project tools (v1.3: `tools/cli/` — the portier CLI)
- `build` = generated build output

## Server Runtimes

- `service/` is the native Go service and the preferred production runtime. Default static dir: `web`.
- `server/` is the TypeScript server and remains supported as reference/fallback. Default static dir: `client/build`.
- Both implement the same REST API contract.

## Packaged Runtime Layout

```text
<install-dir>/
  portier          (or portier.exe on Windows)   — CLI
  service          (or service.exe on Windows)   — background service
  server.js        (Node fallback — requires Node.js)
  web/
    index.html
    assets/
  readme.txt
```

Development build output (not distributed):

```text
service/build/portier-service
server/build/
client/build/
tools/cli/build/portier-cli
```

## Setup Commands

```powershell
npm install
```

## Development Commands

```powershell
npm run start:dev
npm run dev -w server
npm run dev -w client
```

## Validation Commands

```powershell
npm run lint
npm run typecheck
npm run test
npm run build
npm run check
```

## E2E Tests

Playwright E2E tests live in `tests/e2e/`. They run against the TypeScript server serving the built React client.

**Prerequisites (one-time):**
```powershell
npm run test:e2e:install   # installs Playwright Chromium browser
npm run build:client       # must be built before running E2E
```

**Run:**
```powershell
npm run test:e2e           # headless
npm run test:e2e:headed    # visible browser
npm run test:e2e:fresh     # build:client then run
```

E2E server binds to `127.0.0.1:47890`. Do not include `test:e2e` in `npm run test` or `npm run check` — E2E is a separate step.

Files:
- `playwright.config.ts` — webServer, browser, reporter config
- `tests/e2e/portier.spec.ts` — UI flow tests (app load, CRUD, start/stop, merge import, activity, mobile)
- `tests/e2e/settings.spec.ts` — Settings import/export: replace-mode fixture import, invalid-JSON rejection, export download shape
- `tests/e2e/tcp.spec.ts` — TCP real forwarding E2E
- `tests/e2e/udp.spec.ts` — UDP one-way, last-client, multi-client E2E; activity assertions
- `tests/e2e/helpers/port.ts` — `getFreePort`, `getFreeTcpPort`, `getFreeUdpPort`
- `tests/e2e/helpers/network.ts` — TCP/UDP echo servers, receivers, clients
- `tests/e2e/helpers/ui.ts` — `addRuleViaUI`, `startRuleViaUI`, `stopRuleViaUI`
- `tests/e2e/helpers/api.ts` — `clearAllRules`, `createRule`, `startRule`, `stopRule`
- `tests/e2e/helpers/setup.ts` — creates temp config before server starts
- `tests/tsconfig.json` — TypeScript config for E2E files
- `tests/fixtures/config/` — config compatibility fixtures (valid and invalid `rules.json` samples)

**Protocol automation coverage:**
- TCP forwarding: automated E2E via `tcp.spec.ts`
- UDP one-way: automated E2E via `udp.spec.ts`
- UDP bidirectional-last-client: automated E2E via `udp.spec.ts`
- UDP bidirectional-multi-client: automated E2E via `udp.spec.ts`

**Settings import/export E2E coverage (`settings.spec.ts`):**
- Replace-mode import using `v1-mixed.json` fixture: preview counts, confirm dialog, success message, all 4 rules visible in Forward Rules, pre-existing rule gone
- Invalid JSON import: parse error alert, no preview/import button, existing rule preserved
- Export download: file downloaded with correct filename pattern, `ExportedConfig` shape (`version`, `exportedAt`, `rules`), created rule present

**Config compatibility coverage (`validate:config` — not E2E, not manual):**
- Exhaustive fixture-based validation of all 16 fixtures: valid config load, import/export via API, UDP mode defaults, duplicate binding rejection, invalid field rejection, malformed JSON rejection, Go/TS parity
- E2E intentionally does not run the full fixture matrix — `validate:config` owns that

**OS service install validation (run explicitly on the target platform, not part of `npm run check`):**
- Windows scheduled task (no admin): `npm run validate:service:windows:user`
- Windows Service (admin): `npm run validate:service:windows:machine`
- macOS LaunchAgent (no sudo): `npm run validate:service:macos`
- Linux systemd (root/sudo): `npm run validate:service:linux`
- Current platform default: `npm run validate:service:current`

Each script uses test-specific names, ports, and temp paths. Never touches production installs.

**Remaining manual QA (cannot be automated):**
- Firewall and OS permission behavior on each platform

**Automated (not manual):**
- Package build correctness: `npm run validate:runtime:smoke`
- TCP/UDP protocol forwarding: `npm run test:e2e`
- OS service install/start/stop/uninstall: `npm run validate:service:*`

## Coverage Commands

Run coverage for individual components, all at once, or validate all gates:

```powershell
npm run coverage:shared     # shared TypeScript (vitest + v8)
npm run coverage:server     # server TypeScript (vitest + v8)
npm run coverage:client     # client TypeScript/React (vitest + v8)
npm run coverage:service    # Go service (go test -p 1 -coverpkg) — takes ~30s
npm run coverage:cli        # Go CLI reporting only (scripts/coverage-cli.js)
npm run coverage:baseline   # all five in sequence (reporting only)
npm run validate:coverage   # runs all + enforces all gates; exits 1 if any gate fails
npm run validate:coverage:shared   # shared only
npm run validate:coverage:server   # server only
npm run validate:coverage:client   # client only
npm run validate:coverage:service  # service only
npm run validate:coverage:cli      # cli only
```

Coverage outputs (gitignored):
- `coverage/shared/`, `coverage/server/`, `coverage/client/` — vitest json-summary + text
- `coverage/` — Go .out profiles (written and removed per run)

Baseline (v1.4.0): cli 92.7%, client 90.56%, service 82.5%, shared 82.1%, server 82.88%.
See `docs/coverage-baseline.md` for full breakdown and ratchet plan.

Gates (in `scripts/validate-coverage.js`, set at v1.4.0):
- cli: statements ≥ 92%
- client: statements ≥ 90%, branches ≥ 89%, functions ≥ 76%
- server: statements ≥ 82%, branches ≥ 86%, functions ≥ 97%
- service: statements ≥ 82%
- shared: statements ≥ 82%, branches ≥ 54%, functions ≥ 90%

Coverage policy: require 100% meaningful coverage for all newly added or materially changed files in v1.5 and v1.6. Existing baselines ratcheted incrementally. Do not lower gates without explicit rationale. Do not remove gates to make a release pass.

---

## CLI Commands (v1.3)

```powershell
npm run build:cli              # build tools/cli/sources → tools/cli/build/portier[.exe]
npm run test:cli               # go test ./... inside tools/cli
npm run validate:cli           # test:cli + build:cli
npm run validate:coverage:cli  # cli coverage gate only (fails below 92%; 92.7% actual)
```

CLI binary: `portier` / `portier.exe`. Background service remains `service` / `service.exe`.

Global flags: `--url`, `--host`, `--port`, `--json`, `--version`, `-h`/`--help`.  
Environment: `PORTIER_URL`. Default URL: `http://127.0.0.1:47831`.  
Exit codes: `0` success, `1` API error, `2` invalid args, `3` connection failure.

Implemented commands: `list`, `status`, `activity` (with `--limit`/`--rule`/`--type`/`--severity`), `start <id|name>`, `stop <id|name>`, `diagnose <id|name>`, `config validate <file>`, `config export --out <file>`, `config import --mode merge|replace [--yes] <file>`, `diagnostics export --out <file> [--run-diagnostics] [--activity-limit N]`, `runtime`, `version`, `help`.

Rule-targeting commands (`start`, `stop`, `diagnose`) accept an exact rule ID or an exact rule name. Duplicate names produce exit 2 with an ID disambiguation table on stderr.

`portier config validate` validates a local file without contacting the service. `portier config export/import` use `GET /api/config/export` and `POST /api/config/import`. Import validates locally first — invalid files are rejected without an API call. Replace mode requires `--yes`.

`portier diagnostics export` builds a local JSON support bundle (schemaVersion, runtime, rules, statuses, activity, diagnostics, metadata) from independent API calls. Partial source failures are recorded in `errors[]` rather than aborting. `--run-diagnostics` adds per-rule diagnose results. `--activity-limit` (1–500, default 100). 163+ CLI tests. Coverage gate: 92.7% total, threshold 92%.

The CLI binary (`portier`/`portier.exe`) is included in the runtime package (`build/portier/`) and release artifacts. It is not added to PATH by the installer in v1.3.

## Packaging Commands

```powershell
npm run build:runtime             # cross-platform: builds build/portier/ on the current OS
npm run build:runtime:windows     # Windows package (build/windows/)
npm run build:runtime:macos       # macOS package (build/macos/)
npm run build:runtime:linux       # Linux package (build/linux/)
npm run build:clean               # clean build/portier/ and all platform package dirs
```

## macOS LaunchAgent Commands

Lifecycle management (user-level, no sudo):

```bash
bash scripts/macos/service/install-launch-agent.sh    # copies build/portier/ → ~/Applications/Portier/, registers LaunchAgent
bash scripts/macos/service/uninstall-launch-agent.sh  # stops and removes LaunchAgent; preserves rules.json
bash scripts/macos/service/start-launch-agent.sh      # start (or restart) the LaunchAgent
bash scripts/macos/service/stop-launch-agent.sh       # stop the LaunchAgent
bash scripts/macos/service/status-launch-agent.sh     # show LaunchAgent status via launchctl
```

Install script supports: `--source-dir`, `--install-dir`, `--config-path`, `--host`, `--port`, `--runtime service|node`, `--no-start`.
Uninstall script supports: `--purge` (removes config and logs; off by default).

macOS release archive:

```bash
npm run build:release:portable        # build:runtime then portable tar.gz
```

Output: `build/releases/macos/portier-portable-macos-<version>.tar.gz`

Unsigned. Gatekeeper may quarantine downloaded binaries. Sign with Developer ID for public distribution.
Do not add `build:release:*` to `npm run check` — it is a release step.

## Linux systemd Service Commands

Lifecycle management (requires root/sudo):

```bash
sudo bash scripts/linux/service/install-service.sh    # copies build/portier/ → /opt/portier/, registers systemd service
sudo bash scripts/linux/service/uninstall-service.sh  # stops and removes service; preserves /etc/portier/rules.json
sudo bash scripts/linux/service/start-service.sh      # start (or restart) the service
sudo bash scripts/linux/service/stop-service.sh       # stop the service
sudo bash scripts/linux/service/status-service.sh     # show service status via systemctl
```

Install script supports: `--source-dir`, `--install-dir`, `--config-path`, `--host`, `--port`, `--runtime service|node`, `--no-enable`, `--no-start`.
Uninstall script supports: `--remove-files` (removes `/opt/portier/`), `--remove-config` (removes config directory; off by default).

Linux release archive:

```bash
npm run build:release:portable        # build:runtime then portable tar.gz
```

Output: `build/releases/linux/portier-<version>-linux.tar.gz`

No signing required for Linux tar.gz. Firewall rules for forwarded ports are the user's responsibility (ufw, iptables, firewalld).
Do not add `build:release:*` to `npm run check` — it is a release step.

## Windows Release Commands

Build the portable zip and Inno Setup installer for Windows 10+ (Inno Setup 6 required for the installer):

```powershell
npm run build:release:current     # portable zip + installer (installer non-fatal if Inno Setup absent)
npm run build:release:portable    # portable zip only
```

Output:
- `build/releases/windows/portier-<version>-windows-portable.zip`
- `build/releases/windows/Portier-Setup-<version>.exe` (when Inno Setup available)

Build script: `scripts/windows/release/build-release.ps1`
- `-Version 1.2.0` — override version string (default: reads from `package.json`)
- `-NoPackage` — skip `build:runtime` step
- `-InnoPath "C:\..."` — path to `ISCC.exe` if not on PATH

The installer is unsigned. It does NOT create Windows Firewall rules. Config is preserved on uninstall.
Do not add `build:release:*` to `npm run check` — it is a release step.

## Package Validation Commands

```powershell
npm run validate:runtime           # validate existing build/portier/ layout
npm run validate:runtime:build     # build then validate
npm run validate:runtime:smoke     # build, validate, and run smoke test (preferred for release)
```

`validate:runtime:smoke` is the recommended pre-release package check. It does not require
Administrator or root. It does not install OS services.

## OS Service Install Validation Commands

Run these explicitly on the target platform — not included in `npm run check`:

```powershell
npm run validate:service:windows:user     # Windows scheduled task (no Administrator required)
npm run validate:service:windows:machine  # Windows Service (Administrator required)
```

```bash
npm run validate:service:macos    # macOS LaunchAgent (no sudo required)
npm run validate:service:linux    # Linux systemd (requires root/sudo)
npm run validate:service:current  # current platform, user-scope where possible
```

All service validation scripts accept:
- `--no-build` / `-NoBuild` — skip `build:runtime` build step
- `--keep-files` / `-KeepFiles` — preserve temp files for debugging
- `--port` / `-Port` — override the test port

Normal development validation: `npm run check`
Release package validation: `npm run validate:runtime:smoke`
Release service validation: `npm run validate:service:current` (or per-platform variants)

## Additional Validation Suites

Run explicitly — not part of `npm run check`. Slower or platform-sensitive.

```powershell
npm run validate:config            # fixture-based config compatibility validation
npm run validate:contract          # API contract parity (TypeScript + Go if available)
npm run validate:binary            # runtime binary behavior (build:runtime + 5 behavioral tests)
npm run validate:runtime:behavior  # alias for validate:binary (fits validate:runtime:* namespace)
npm run validate:scripts           # installer script static analysis + dry-run on current platform
```

**`validate:config`** — Loads every fixture from `tests/fixtures/config/` (17 fixtures: 8 valid, 8 invalid, 1 malformed JSON) and runs:
1. Static JSON parsing — valid fixtures parse; malformed-json fixture does not.
2. Config file loading — starts the TypeScript server (and Go service if available) with each valid fixture as `rules.json`; verifies rule count. The `{rules:[...]}` wrapper shape is tested against the Go service only (TypeScript config requires a raw array).
3. HTTP API import/export — imports each valid fixture via `POST /api/config/import`, verifies rule counts and UDP mode defaults, checks export shape stability.
4. Invalid fixture rejection — each invalid-field rule is posted via `POST /api/forwards` and must return 400 with `errors[]`.
5. Duplicate binding — posting a second rule with the same listen key must return 409.

TypeScript runtime is always checked. Go runtime is checked when `build/portier/service[.exe]` or `service/build/portier-service[.exe]` is present. Pass `--skip-go` to force skip. No real `rules.json` is used.

**`validate:contract`** — Starts the TypeScript server (and the Go binary if present) and runs all API scenarios: CRUD forwards, start/stop, status, activity, config export/import, port advisory, error shapes, duplicate binding, unknown-ID 404s. Skips Go parity with a clear message if the binary is not built. Use `--skip-go` to force skip.

**`validate:binary`** (also `validate:runtime:behavior`) — Runs `build:runtime` then tests `build/portier/service[.exe]` behavior:
1. `/api/health` responds on a free port
2. `/` serves HTML when `web/` static dir is present
3. API works when static dir is missing; `/` returns non-200
4. Invalid JSON config → process exits with non-zero code
5. Process terminates within 5s after kill signal

Pass `--no-build` to reuse an existing `build/portier/`.

**`validate:scripts`** — Static analysis + dry-run:
- All install scripts: no silent firewall commands
- Validate scripts: test-specific names (not production), no hard-coded port 47831
- `install-service.ps1`: `Format-Argument` quoting helper, `-DryRun` parameter
- `install-launch-agent.sh`: plist uses absolute paths (not `~`), `--dry-run` flag
- `install-service.sh`: default `INSTALL_DIR=/opt/portier`, default `CONFIG_PATH=/etc/portier/rules.json`, `--dry-run` flag
- Dry-run execution on the current platform validates planned output contains required fields without performing any real install

Naming convention:
- `npm run test` = unit/integration test runner (Vitest + Go test)
- `npm run test:e2e` = Playwright browser E2E tests
- `npm run validate:config` = fixture-based rules.json compatibility validation
- `npm run validate:contract` = TS/Go API parity validation runner
- `npm run validate:binary` / `validate:runtime:behavior` = packaged binary behavior validation
- `npm run validate:scripts` = installer script static analysis + dry-run validation

**Installer dry-run flags** (added to production install scripts):
- Windows: `install-service.ps1 -DryRun` — prints install plan (scope, paths, command line) and exits without creating dirs or registering services
- macOS: `install-launch-agent.sh --dry-run` — prints install plan (label, plist path, paths, ProgramArguments) and exits without creating files or loading LaunchAgent
- Linux: `install-service.sh --dry-run` — prints install plan (paths, service unit, ExecStart) and exits without creating files or running systemctl

## Release Artifact Commands

Build the current platform's portable archive (and installer if tooling is available):

```powershell
npm run build:release:current       # portable + installer (non-fatal if installer tools absent)
npm run build:release:portable      # portable archive only
```

Validate release artifacts:

```powershell
npm run validate:release:portable     # checks archive contents, readme.txt, forbidden files
npm run validate:release:current      # also checks installer artifact if present
```

Output layout: `build/releases/windows/`, `build/releases/macos/`, `build/releases/linux/`.

Archive filenames are versioned:
- Windows: `portier-<version>-windows-portable.zip`, `Portier-Setup-<version>.exe`
- macOS: `portier-portable-macos-<version>.tar.gz`
- Linux: `portier-<version>-linux.tar.gz`

Service binaries are platform-native. Run on each OS for that OS's artifacts.
Do not add `build:release:*` or `validate:release:*` to `npm run check` — they are release steps.

## Installer Strategy

v1.1 focuses on distribution and native OS service installers. The v1.1 scope, platform strategy, install layouts, artifact targets, and implementation slices are defined in `docs/installer-strategy.md`.

## Roadmap

v1.2 delivered diagnostics and operational polish: runtime info endpoint, rule diagnostics, activity log improvements, safer networking UX, settings polish, and diagnostics export.

v1.3 targets native CLI and automation: a Go-based `portier` CLI under `tools/cli/` that talks to the existing management API for terminal and script workflows. The CLI is an API client — not a runtime, not a scripts/ helper. Slices 2–7 complete: `tools/cli/` module scaffolded, HTTP client (`ConnectionError`/`APIError`), connection options (`--url`/`--host`/`--port`/`PORTIER_URL`), `--json` flag, `runtime`/`list`/`status`/`activity`/`start`/`stop`/`diagnose`/`config`/`diagnostics` commands, safe rule resolver, local config validation, `ExportConfig`/`ImportConfig`/`BaseURL` API client additions, diagnostics bundle builder (partial-failure tolerant, `--run-diagnostics`, `--activity-limit`), 153 CLI tests, `build:cli`/`test:cli`/`validate:cli` npm scripts; CLI binary now included in runtime package and release artifacts.

v1.4 delivered the Live Connection Inspector: `GET /api/connections` in both runtimes, TCP and UDP session tracking, rule summaries, and a dedicated Live Connections UI view (TCP/UDP/Summary tabs, filters, auto-refresh). Coverage hardened before the feature was built; 116/116 contract checks pass. Tagged 1.4.0.

v1.5 targets declarative config and drift control: plan/diff/apply workflows so users can compare desired config files with the running configuration, preview changes, and apply them safely from the CLI or UI. See `docs/roadmap.md`.

Quality target for v1.4 and v1.5: all newly added or materially changed implementation areas should reach 100% meaningful test coverage, with explicit coverage gates where practical. This coverage push is a deliberate prerequisite for v1.6.

v1.6 is a dedicated Architecture, Quality & Maintainability Audit: a structured multi-angle inspection of architecture boundaries, runtime parity, forwarding correctness, API contract, CLI quality, UI quality, test quality, security/safety posture, packaging, and documentation consistency. The high coverage built in v1.4 and v1.5 is the safety net that makes v1.6 refactoring and hardening work safe to perform. Raw audit notes should not be dumped into `docs/`; durable audit outcomes belong in curated docs or a tracked backlog. See `docs/roadmap.md`.

## Coding Guidelines

- Keep TypeScript simple and explicit.
- Prefer small modules over clever abstractions.
- Keep shared validation and constants in `shared/sources`.
- Keep TCP and UDP forwarding logic separate.
- `ForwardManager` should own lifecycle orchestration.
- Keep runtime config external.
- Do not bake `rules.json` into executables.
- Do not expose the management UI/API on `0.0.0.0` by default.
- Management UI/API default: `127.0.0.1:47831`.
- Recommended forward listen port range: `48000-48999`.
- Go service static dir default: `web` (packaged layout). Dev: pass `--static-dir ../client/build`.
- TypeScript server static dir default: `client/build`. Prod: pass `--static-dir web` or set `PORTIER_STATIC_DIR`.

## Safety Rules

- Warn clearly when a forward rule listens on `0.0.0.0`.
- Treat management binding to `0.0.0.0` as dangerous.
- Do not silently change firewall rules.
- Do not add telemetry.
- Do not add remote update/download behavior.
- Do not store secrets in repo files.
- Do not modify local user config files unless explicitly requested.

## Do Not Edit Unless Explicitly Asked

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
- generated package outputs
- user-local config files

## API Documentation Rule

Whenever an API endpoint is added, removed, or changed, update **both** documentation surfaces:

1. `docs/api-contract.md` — the durable external/project API contract.
2. `client/sources/features/apidocs/ApiDocsView.tsx` — the user-facing in-app API reference.

Also update `client/sources/features/apidocs/ApiDocsView.test.tsx` for new endpoints.

Do not consider an API slice complete until both documentation surfaces and their tests are updated.

## Networking Checklist

- TCP sockets clean up on error and close.
- UDP `bidirectional-last-client` mode is documented as limited.
- Do not claim full multi-client UDP support unless implemented.
- Duplicate `protocol + listenHost + listenPort` bindings are rejected.
- Shutdown closes active sockets and servers.

## Response Expectations

- Summarize changed files.
- List validation commands run.
- State anything not run and why.
- Call out follow-up tasks and risks.

## Repository Naming

- Use `sources/` for TypeScript source directories.
- Use `build/` for generated build outputs.
- Keep executable automation scripts under `scripts/`, with `release/` (artifact builders), `service/` (OS service lifecycle), and platform docs/templates under `scripts/windows/`, `scripts/macos/`, and `scripts/linux/`.
- Use `tools/` for user-facing or developer-facing project tools that are not repo automation. `tools/cli/` is the v1.3 portier CLI. Future possible tools: `tools/bench/` (benchmarking), `tools/replay/` (scenario replay). Do not mix tools into `scripts/` or `service/`.
- Normal documentation filenames are lowercase, such as `docs/architecture.md` and `docs/checklist.md`. The root `README.md` is uppercase.
- Keep tool-required files uppercase: `AGENTS.md`, `CLAUDE.md`, and `SKILL.md` in Codex/Claude skill directories.
- React component and view files under `client/sources/` use **CamelCase** filenames (e.g., `ForwardRuleList.tsx`, `StatCard.tsx`). Non-component files use the existing repo convention (e.g., `format.ts`, `nav.ts`).
