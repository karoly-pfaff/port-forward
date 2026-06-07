# Portier Roadmap

## Release Progression

- **v1.0** — Proved the core app and runtime behavior: TCP/UDP forwarding, both runtimes, Playwright E2E, package layout.
- **v1.1** — Made Portier easier to install and distribute: native OS service installers, release artifacts, service and package validation, cross-platform polish.
- **post-v1.1** — Config compatibility fixtures (`tests/fixtures/config/`), `validate:config` runner, and Settings import/export E2E (`settings.spec.ts`) added as pre-v1.2 groundwork.
- **v1.2** — Improve operational confidence: diagnostics, visibility, safer networking UX, and runtime transparency.

---

## Portier v1.2 — Diagnostics & Operational Polish

**Core theme: Make Portier easier to trust while it is running.**

v1.0 and v1.1 established what Portier is and how to get it running. v1.2 focuses on what happens after installation — giving users the tools to understand what the service is doing, verify that rules are working, and act on problems quickly.

### Goals

#### Runtime Info and Transparency

Surface what is actually running, not just that it is running.

Possible fields:
- Runtime implementation: Go service or Node/TypeScript server
- Version
- Platform (OS and architecture)
- Uptime
- Config path
- Static dir
- Management host and port
- Package or install mode, if detectable

Possible API direction:
- Add `GET /api/runtime` with runtime info
- Or extend the existing `GET /api/status` endpoint if that is cleaner

UI: display in the Settings view alongside management endpoint info. Keep it read-only and compact.

#### Rule Diagnostics

Add a per-rule check flow that lets users verify a rule before or after starting it.

For TCP:
- Check whether the listen host/port can bind
- Check whether the target host resolves
- Check whether the target port is reachable
- Report connection timeout or failure clearly

For UDP:
- Check whether the listen host/port can bind
- Check whether the target host resolves
- Explain the selected UDP mode
- Show a warning for `bidirectional-last-client` limitations
- Show active session count for `bidirectional-multi-client` if relevant

UI direction:
- Add a **Diagnose** or **Test** button per rule in the rules table or edit drawer
- Show results as clear pass / warn / fail rows with short descriptions
- Do not block rule creation — this is informational, not a gatekeeper

API direction:
- `POST /api/forwards/:id/diagnose` — run checks and return structured results
- Results include: step name, status (pass/warn/fail), message

#### Safer Networking UX

Improve how the UI communicates network safety decisions.

Ideas:
- Stronger inline LAN exposure warning when `listenHost` is `0.0.0.0`, visible in the form and rule row
- Clearer common/privileged port warnings in the add/edit form
- **Local-only preset** — fills `listenHost` with `127.0.0.1` and labels it clearly
- **LAN-exposed preset** — fills `listenHost` with `0.0.0.0` and shows an explicit warning before the user saves
- Better duplicate binding/conflict error message (what is already bound, not just that it is bound)
- Windows-specific note in the UI where relevant: forwarded LAN ports may require a Windows Firewall rule

#### Activity Log Polish

Make the Activity Log more useful during real operation.

Ideas:
- Per-rule activity filter: "show events for this rule"
- "Last events for this rule" link from the rules table row
- Clear activity log button
- Export activity log as JSON or text
- Better event grouping or throttle display (e.g., "UDP: 1,240 packets in the last 10s" instead of raw entries)
- Keep packet stats accurate while avoiding activity event spam

#### Settings / Runtime / Config Polish

Improve the Settings view as an operational control room.

Ideas:
- Show config path (current, from runtime or env)
- Copy config path to clipboard
- Show runtime info (version, platform, uptime) — linked to `GET /api/runtime`
- Show service binary or static dir path if available
- Better import/export UX: clearer replace vs. merge warnings, validation preview improvements
- Link to platform-specific install docs from Settings
- Show current app version in the UI footer or Settings

#### Diagnostics Export

Add a simple diagnostics export for support and debugging.

Possible contents:
- Runtime info (version, platform, uptime, config path, static dir)
- Current rules (same as config export)
- Rule statuses and traffic stats
- Recent activity (last N events)
- No secrets expected, but avoid capturing sensitive local paths unless the user intentionally includes them

Possible API direction:
- `GET /api/diagnostics` — returns a structured JSON bundle
- Or a client-side "copy diagnostics" that assembles info from existing endpoints

---

### Suggested Implementation Slices

1. ~~**Runtime info endpoint and UI display**~~ — ✓ Complete. `GET /api/runtime` in both runtimes, Settings Runtime/Environment section, shared `RuntimeInfo` type, contract validator updated.
2. ~~**Rule diagnostics API**~~ — ✓ Complete. `POST /api/forwards/:id/diagnose` in both runtimes, shared `RuleDiagnosticsResult` type, structured pass/warn/fail/skip checks, contract validator updated, `docs/api-contract.md` updated, client in-app API Docs updated, API Docs tests added.
3. ~~**Rule diagnostics UI**~~ — ✓ Complete. Diagnose button (stethoscope icon) in every rule row; inline collapsible diagnostics panel with pass/warn/fail/skip display; loading, error, and clear states; duplicate-click prevention; delete clears diagnosis. 133 client tests passing.
4. ~~**Activity Log polish**~~ — ✓ Complete. View Activity button per rule row, rule filter banner, type filter, clear filters, `DELETE /api/activity` on both runtimes, Export JSON (client-side), Clear Log button, packet throttle note, `docs/api-contract.md` updated, client in-app API Docs updated, contract validator updated.
5. ~~**Settings / runtime / config polish**~~ — ✓ Complete. Copy buttons for config path, static dir, management URL; datetime export filename; export note excludes Activity Log; export success/error feedback; import mode with descriptions above file picker; replace confirm backup export button; `PORTIER_APP_VERSION` constant; version in sidebar footer. 24 new tests.
6. ~~**Safer networking UX pass**~~ — ✓ Complete. Listen host presets (Local only / LAN exposed), inline LAN warning in form, platform-aware firewall note, friendly conflict error copy, improved LAN_EXPOSURE advisory message. `ForwardRuleForm` accepts optional `runtimePlatform` prop. 17 new/updated tests.
7. ~~**Diagnostics export**~~ — ✓ Complete. Client-side bundle: runtime, rules, statuses, activity, UI-session diagnostics. Download Diagnostics JSON button in Settings. Partial-failure handling with `errors` array. No backend endpoint. 216 tests passing.
8. ~~**v1.2 readiness audit, version bump, changelog, tag**~~ — ✓ Complete. All slices shipped; version bumped to 1.2.0; changelog finalized; tag ready.

---

### Non-Goals for v1.2

The following are explicitly out of scope for v1.2:

- Auto-update or self-update behavior
- Tray app or desktop GUI wrapper
- Full desktop GUI redesign
- Firewall auto-rule management (creating or modifying OS firewall rules)
- Homebrew, winget, or Chocolatey package registry submissions
- `.deb` / `.rpm` packages unless explicitly promoted from deferred status
- Cloud sync of rules or config
- User accounts or authentication
- Remote management (managing Portier from another machine)
- Telemetry of any kind
- New forwarding protocols
- Large UI redesign or component framework swap
