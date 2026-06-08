# Changelog

All notable changes to Portier are documented here.

---

## [Unreleased] — v1.3 in progress

### Added (v1.3 Slice 2 — Go CLI skeleton and API client)

- **`tools/cli/` module** — new Go module (`portier/cli`) under `tools/cli/` for the Portier CLI. Separate from the Go service (`portier/service`); the CLI is an API client, not a second runtime.
- **CLI binary name**: `portier` (Linux/macOS) / `portier.exe` (Windows). Background service binary remains `service` / `service.exe`.
- **`portier runtime` command** — calls `GET /api/runtime`; displays name, version, runtime, platform/arch, uptime, management URL, config path, static dir, service mode. Works with both the Go service and TypeScript server.
- **`--json` flag** — prints machine-readable JSON from the API response; no decorative text around JSON output.
- **Connection options** — `--url`, `--host`, `--port`, `PORTIER_URL` env var; precedence: `--url` > `--host`/`--port` > `PORTIER_URL` > default (`http://127.0.0.1:47831`).
- **`portier help` / `--help` / `-h`** — shows purpose, default URL, global flags, available commands, planned commands, exit codes.
- **`portier version` / `--version`** — shows `Portier CLI <version>`.
- **Exit code policy**: `0` success, `1` general/API error, `2` invalid arguments/usage, `3` connection failure.
- **`User-Agent` header**: `PortierCLI/<version>` on every API request.
- **Structured error handling**: `ConnectionError` (maps to exit 3 + hint) and `APIError` (non-2xx response with `errors[]` shape).
- **Tests**: 22+ Go tests using `httptest` covering URL resolution precedence, runtime human/JSON output, connection failure, API errors, User-Agent, invalid JSON, and error type unwrapping.
- **npm scripts**: `build:cli`, `test:cli`, `validate:cli`.
- **`tools/cli/readme.md`** — CLI usage, global flags, commands, exit codes, module structure, planned commands.

---

## [1.2.0] - 2026-06-07

### Goal

Portier v1.2 improves operational confidence with diagnostics, visibility, and safer networking UX. See `docs/roadmap.md` for goals, slices, and non-goals.

### Added (v1.2 Slice 7 — Diagnostics export / support bundle)

- **Download Diagnostics JSON** button in Settings, below Runtime / Environment. Assembles a local JSON support bundle and downloads it directly in the browser — no upload, no backend endpoint.
- **Bundle contents**: `schemaVersion`, `exportedAt`, `app` (name + version), `runtime` (from `GET /api/runtime`), `rules` (fresh fetch), `statuses` (fresh fetch), `diagnostics` (any results already run in the current UI session), `activity` (up to 100 recent events), and `metadata` (management URL, source, generatedBy).
- **Partial-failure handling**: each data source is fetched independently using `Promise.allSettled`. If any source fails, the rest are still included; an `errors` array is added to the bundle and a warning is shown in the UI. The download still proceeds.
- **Empty diagnostics note**: if no rule diagnostics have been run in the current session, the bundle includes `diagnosticsNote: "No rule diagnostics had been run in this UI session."` alongside an empty `diagnostics` object.
- **Excluded data**: raw config file contents from disk, logs, environment variables, OS user name, home directory beyond paths already in the runtime endpoint, node_modules, and build file listings. Nothing is uploaded or transmitted.
- **Filename format**: `portier-diagnostics-YYYYMMDD-HHMMSS.json` (local time; no characters invalid on Windows).
- **`diagnosticsExport.ts`** helper module added at `client/sources/features/settings/`: exports `buildDiagnosticsBundle()`, `buildDiagnosticsFilename()`, and `downloadJson()`.
- **`settings-warn` CSS class** added for the partial-data warning state.
- **New tests**: `diagnosticsExport.test.ts` (28 unit tests covering bundle structure, diagnostics inclusion, partial failure, filename, and download mechanics); 10 new `SettingsView` component tests covering render, fetch calls, success/partial/disabled states, bundle schema, and diagnostics from UI state. 216 tests total pass.

### Added (v1.2 Slice 6 — Safer networking UX)

- **Listen host presets** in the Add/Edit Rule drawer: "Local only" (127.0.0.1) and "LAN exposed" (0.0.0.0) quick-select buttons. Active preset is highlighted; manual host entry remains fully supported.
- **Inline LAN warning** in the rule form: when listenHost is `0.0.0.0`, a warning block appears directly below the listen host/port row stating that the rule listens on all interfaces and that Portier does not create firewall rules automatically.
- **Listen host hint text**: "Only this computer can connect" shown for 127.0.0.1; "Other devices on your network may be able to connect if firewall rules allow it" shown for 0.0.0.0.
- **Firewall note advisory card**: shown in the advisory section when listenHost is `0.0.0.0`. Platform-aware: shows a Windows-specific message if `runtimePlatform="windows"` is provided, otherwise a generic OS note. `ForwardRuleForm` now accepts an optional `runtimePlatform` prop.
- **Friendly conflict error copy**: when a save fails because the listen binding is already in use, the form shows "Another rule is already using this protocol, listen host, and listen port. Choose a different listen port, or stop/remove the conflicting rule." instead of the raw server error.
- **Improved LAN_EXPOSURE advisory message** (shared): updated to "Listening on 0.0.0.0 exposes this forwarded port on all interfaces. Other LAN devices may be able to connect if firewall settings allow it."
- **New tests** (ForwardRuleForm): Local only / LAN exposed presets set correct host; custom host entry still works; LAN warning visible for 0.0.0.0; LAN warning absent for 127.0.0.1; generic and Windows-specific firewall notes; save not blocked by LAN warning; friendly conflict error shown; privileged port advisory. 17 new/updated test cases.

### Added (v1.2 Slice 5 — Settings / runtime / config polish)

- **App version in sidebar footer**: Sidebar now shows `v{version}` below the management host. Sourced from new `PORTIER_APP_VERSION` constant in `@portier/shared`.
- **Copy buttons in Runtime / Environment**: Config path, static dir, and management URL each have a small "Copy" button. Uses `navigator.clipboard.writeText`; shows "Copied!" on success, "Failed" on error, resets after 2 seconds. Gracefully handles environments where clipboard is unavailable.
- **Export Config UX**:
  - Filename is now datetime-stamped: `portier-config-YYYYMMDD-HHMMSS.json` (local time).
  - Button renamed to "Download Config (JSON)" for clarity.
  - Helper text now states "The Activity Log is not included" so users know what the export covers.
  - Export shows a success confirmation message; errors are displayed inline instead of via `alert()`.
- **Import Config UX**:
  - Import mode (Merge / Replace) is now shown above the file picker with descriptive explanations, so users understand the difference before selecting a file.
  - Merge description: "adds rules from the file; skips rules with conflicting listen port bindings."
  - Replace description: "stops and deletes all current rules, then applies imported rules. Export a backup first if you want to keep your current rules."
  - Replace confirmation dialog now includes an "Export current config as backup" button for a one-click safety exit.
- **`PORTIER_APP_VERSION`** constant exported from `shared/sources/index.ts`.
- **New tests** (SettingsView): import mode descriptions visible before file selection; import button absent until valid file loaded; replace confirm shows backup export button; copy config path/static dir/management URL; clipboard failure shows "Failed" feedback; export success/error feedback. 18 new test cases.
- **New tests** (Sidebar): 6 tests covering render, nav click, open state, management host, and version footer.

### Added (v1.2 Slice 4 — Activity Log polish)

- **View activity** button (Activity icon) added to every rule row in the Forward Rules table. Clicking it navigates to the Activity view and pre-filters to that rule.
- Activity view **filter banner**: when navigating from a rule row, a status banner shows "Filtered to rule: [name]" with a Clear button. Handles deleted/missing rule names gracefully by displaying the raw rule ID.
- Activity view **type filter**: new dropdown lets users filter events by event type (rule lifecycle, connection, packet, session, config events). Type is passed as a query param to `GET /api/activity`.
- **Clear filters** button appears in the Activity header when any filter (rule, severity, type) is active.
- **Export JSON** button in the Activity footer exports currently loaded/filtered events as a JSON file. Filename format: `portier-activity-YYYYMMDD-HHMMSS.json`. Payload includes `exportedAt`, `filters`, and `events`. Client-side only; no backend endpoint.
- **Clear Log** button in the Activity footer calls `DELETE /api/activity` and empties the displayed list.
- Packet event **throttle note**: small explanatory text in the Activity body — "High-frequency packet events may be summarized or throttled; counters remain exact in rule status."
- `DELETE /api/activity` endpoint on both the TypeScript server and Go service: clears the in-memory activity log and returns `204 No Content`. Does not affect rules or forwarding state.
- `clearActivity()` API helper added to `client/sources/api/portierApi.ts`.
- `Store.Clear()` method added to `service/sources/activity/store.go` (Go).
- `Manager.ClearActivity()` method added to `service/sources/manager/manager.go` (Go).
- `docs/api-contract.md` updated with `DELETE /api/activity` section.
- Client in-app API Docs view updated with `DELETE /api/activity` entry.
- 18 new `ActivityLogView` component tests (21 total); 4 new `ForwardRuleList` activity navigation tests; 2 new `ApiDocsView` tests.
- 3 new TypeScript server API tests for `DELETE /api/activity`; 3 new Go API tests.
- `validate:contract` updated with `DELETE /api/activity` scenario and post-clear empty-array verification.

### Added (v1.2 Slice 3 — rule diagnostics UI)

- **Diagnose** action button (stethoscope icon) added to every rule row in the Forward Rules table.
- Clicking Diagnose calls `POST /api/forwards/:id/diagnose` and shows a collapsible diagnostics panel inline below the rule row — no navigation required.
- Diagnostics panel states: loading spinner while in flight, pass/warn/fail summary with timestamp, per-check rows with pass/warn/fail/skip status, and error display for API failures.
- Duplicate Diagnose clicks are prevented while a request is in flight for that rule; other rules remain clickable.
- Re-running Diagnose replaces the previous result immediately (pending → result).
- Closing the panel (✕ button) clears the result; deleting a rule also clears its diagnostic state.
- `diagnoseForwardRule(ruleId)` API helper added to `client/sources/api/portierApi.ts`.
- `DiagnosisEntry` union type exported from `ForwardRuleList.tsx` for App-level state management.
- `RuleDiagnosticsPanel` component added at `client/sources/features/forwards/RuleDiagnosticsPanel.tsx`.
- CSS for diagnostics panel added to `client/sources/styles/styles.css`.
- 11 new `RuleDiagnosticsPanel` unit tests; 9 new `ForwardRuleList` diagnostics tests; 2 new `App` integration tests (133 total client tests).

### Added (v1.2 Slice 2 — rule diagnostics API)

- `POST /api/forwards/:id/diagnose` endpoint on both the TypeScript server and Go service: runs diagnostic checks against an existing forward rule without changing rule state or opening long-lived sockets.
- Response shape: `RuleDiagnosticsResult` — ruleId, ruleName, protocol, summary (`pass`/`warn`/`fail`), checks array, diagnosedAt timestamp.
- Checks implemented: `listen-host`, `lan-exposure`, `privileged-port`, `common-port`, `listen-bind`, `target-host`, `target-connect`, `udp-mode` (UDP only).
- `listen-bind` skips the bind attempt when the rule is running and returns `pass` — Portier owns the socket.
- `target-connect` is always `skip` for UDP rules (UDP reachability cannot be verified without a protocol-specific response).
- `udp-mode` warns for `bidirectional-last-client`; passes for `one-way` and `bidirectional-multi-client`.
- `DiagnosticStatus`, `DiagnosticCheck`, `DiagnosticSummary`, `RuleDiagnosticsResult` types exported from `@portier/shared`.
- `getRule(id)` method added to `ForwardManager` (TypeScript server).
- `diagnoseRule()` helper added to `server/sources/diagnose.ts` (TypeScript) and `service/sources/api/diagnose.go` (Go).
- `DiagnosticCheck`, `DiagnosticSummary`, `RuleDiagnosticsResult` types added to `service/sources/domain/domain.go`.
- `validate:contract` updated with diagnose scenarios: unknown rule 404, TCP reachable target, UDP target-connect skip, LAN exposure warning, response shape verification.
- TypeScript tests and Go tests added for all check scenarios.

### Added (v1.2 Slice 1 — runtime info)

- `GET /api/runtime` endpoint on both the TypeScript server and Go service: returns name, version, runtime (`"node"`/`"go"`), platform, arch, uptimeSeconds, startedAt, managementHost/Port, configPath, staticDir, serviceMode, and pid.
- `RuntimeInfo` type exported from `@portier/shared`.
- `fetchRuntimeInfo()` API helper in the client.
- Settings view — **Runtime / Environment** section: shows live runtime info (runtime, version, platform/arch, uptime, management URL, config path, static dir, service mode, PID). Gracefully shows "Runtime information is unavailable from this backend." for older backends.
- Go service version package (`service/sources/version/version.go`): default `"dev"`, injectable at build time via `-ldflags`.
- `validate:contract` updated to verify GET /api/runtime shape and runtime field for both runtimes.

### Added (post-v1.1, pre-v1.2)

- Config compatibility fixtures (`tests/fixtures/config/`): 8 valid and 8 invalid `rules.json` fixtures covering TCP, all UDP modes, both config shapes (raw array and Go wrapper), and all field-level error categories.
- `npm run validate:config` — fixture-based compatibility runner (`scripts/validate-config.js`): validates config load, HTTP API import/export, UDP mode defaults, duplicate binding rejection, and invalid fixture rejection against the TypeScript runtime and the Go service when binary is available. Never reads or writes the user's real `rules.json`.

---

## [1.1.0] - 2026-06-06

### Goal

Portier v1.1 makes Portier easy and safe to install as a native background service on Windows, macOS, and Linux, with automated package/service validation and clean release artifacts.

See `docs/installer-strategy.md` for scope, platform decisions, and implementation slices.

### Added

- Windows Inno Setup installer (`scripts/windows/release/portier.iss`): installs to `%ProgramFiles%\Portier\`, optional Windows Service registration with auto-start at boot, config at `%ProgramData%\Portier\rules.json`. Upgrade support: stops running service before overwriting binaries. Uninstall removes service registration and logs; preserves `rules.json` by default.
- `build-release.ps1` (`scripts/windows/release/`): reads version from `package.json`, runs `build:runtime`, calls ISCC.exe, produces `build/releases/windows/Portier-Setup-<version>.exe`.
- macOS install scripts updated: `install-launch-agent.sh` now auto-copies `build/portier/` to `~/Applications/Portier/` by default; adds `--source-dir`, `--no-start`, and `--runtime service|node` options; fixes label consistency bug (`com.portier.port-forwarding` everywhere).
- `uninstall-launch-agent.sh` adds `--purge` flag for removing config and logs (config is preserved by default).
- `scripts/macos/release/build-release.sh` — builds `build/releases/macos/portier-portable-macos-<version>.tar.gz` from `build/portier/`.
- Signing and notarization documented in `scripts/macos/readme.md` (unsigned local builds supported; Developer ID signing documented as required for public distribution).
- Linux `install-service.sh` updated: auto-copies `build/portier/` to `/opt/portier/` by default; adds `--source-dir`, `--no-enable`, `--no-start`, and `--runtime service|node` options.
- `scripts/linux/release/build-release.sh` — builds `build/releases/linux/portier-<version>-linux.tar.gz` from `build/portier/`.
- `scripts/linux/readme.md` updated: install flags table, release archive section, firewall notes, journald commands, `--no-enable` documented.
- `scripts/windows/service/validate-user-install.ps1`: validates user-scope scheduled task flow with test name `PortierTestUser`, isolated temp dirs, auto-port detection, `-NoBuild`/`-KeepFiles`/`-Port` flags; never touches production.
- `scripts/windows/service/validate-machine-service.ps1`: validates machine-scope Windows Service flow with test name `PortierTestMachine`; requires Administrator; same flags; never touches production.
- `scripts/macos/service/validate-launch-agent.sh`: validates LaunchAgent flow with test label `com.portier.test`, temp plist at `~/Library/LaunchAgents/com.portier.test.plist`; no sudo required; `--no-build`/`--keep-files`/`--port` flags.
- `scripts/linux/service/validate-systemd-service.sh`: validates systemd flow with test unit `portier-test.service`, temp paths under `/tmp/portier-test-<pid>/`; requires root; fails clearly if not root or systemd unavailable.
- `scripts/validate-service.js`: cross-platform dispatcher — Windows runs user-scope, macOS runs LaunchAgent, Linux runs systemd; fails clearly on unsupported platforms.
- `npm run validate:service:current` / `validate:service:windows:user` / `validate:service:windows:machine` / `validate:service:macos` / `validate:service:linux` — explicit release validation commands.

- `scripts/build-release.js`: unified release packaging script for all platforms. Reads version from `package.json`, calls `build:runtime`, produces portable archives (Windows `.zip`, macOS/Linux `.tar.gz`) and Windows installer (non-fatal if Inno Setup absent). Service binaries are platform-native; run on each target OS.
- `scripts/validate-release.js`: validates `build/releases/<platform>/` layout, archive contents (required/forbidden files, readme.txt content), and optional installer artifact.
- Updated `readme.txt` in Windows/macOS/Linux build scripts: now includes portable archive notice ("does not install OS services"), `--config` / `--static-dir web` options, "not bundled in this archive" note for config.
- `npm run build:release` / `build:release:current` — full release packaging for current platform (portable + installer if available).
- `npm run build:release:portable` — portable archive only, skip installer.
- `npm run validate:release` / `validate:release:current` — validate release artifacts for current platform.
- `npm run validate:release:portable` — validate portable archive only.

### Deferred

- macOS `.pkg` installer via `pkgbuild`/`productbuild` (requires macOS tooling; deferred to future release).
- Linux `.deb` / `.rpm` packages (deferred beyond v1.1).

---

## [1.0.0] - 2026-06-06

### Added

- Native Go service runtime in `service/`, preferred for production packages while the TypeScript server in `server/` remains supported as the reference and fallback runtime.
- TCP forwarding in both runtimes with real listeners, bidirectional piping, status counters, and lifecycle cleanup.
- UDP forwarding in both runtimes across all supported modes: `one-way`, `bidirectional-last-client`, and `bidirectional-multi-client`.
- Rule CRUD, start/stop, update/restart behavior, duplicate listen binding rejection, persistent config, config import/export, and rule reorder.
- Activity Log with bounded in-memory events for rule lifecycle, TCP connections, UDP packets, UDP multi-client sessions, config export, and config import.
- Dark React web UI with Dashboard, Forward Rules, Activity, Settings, API Docs, mobile sidebar, add/edit/delete drawer flows, error banners, LAN exposure warnings, and common/recommended port advisories.
- Playwright E2E coverage for app load, add/edit/delete flows, start/stop, settings import, mobile navigation, and real TCP/UDP forwarding.
- Go service tests for config, manager behavior, API routes, TCP forwarding, UDP modes, activity events, validation, options, and advisories.
- Windows, macOS, and Linux packaging/install docs and scripts, including Go service mode and Node/TypeScript server fallback where applicable.

### Changed

- Repository convention standardized on `sources/` for source directories, `build/` for generated output, and `scripts/` for executable automation; platform docs and templates are co-located under `scripts/{platform}/`.
- Production package layout standardized to a flat install directory:

```text
<install-dir>/
  service          (or service.exe on Windows)
  server.js        (Node fallback, requires Node.js)
  web/
    index.html
    assets/
```

- Native Go service static directory defaults to `web` for packaged layout. Repository development commands may still pass `../client/build`.
- TypeScript server remains supported and can serve either repository `client/build` or packaged `web` via `--static-dir` / `PORTIER_STATIC_DIR`.
- Runtime config remains external in all modes; `rules.json` is not baked into binaries or packages.
- Manual QA scope narrowed to platform/package/service install behavior because core TCP/UDP correctness is automated.

### Validation

- Root validation scripts cover lint, typecheck, unit/integration tests, Go tests, build, Playwright E2E, package smoke checks, and repository hygiene checks.
- Real protocol coverage is automated for TCP, UDP one-way, UDP bidirectional-last-client, and UDP bidirectional-multi-client.

---

## [0.1.0] - 2026-06-04

First usable internal release of Portier, a local TCP/UDP port forwarding manager for development and LAN testing.

### Added

- TypeScript server with REST API on `127.0.0.1:47831` by default.
- TCP forwarding, UDP one-way forwarding, and UDP bidirectional-last-client forwarding.
- React management UI for listing, creating, editing, starting, stopping, and deleting forward rules.
- Shared TypeScript validation, common port advisories, recommended forward port range, and LAN exposure warnings.
- External JSON config path via `--config` / `PORTIER_CONFIG`.
- Static web serving via `--static-dir` / `PORTIER_STATIC_DIR`.
