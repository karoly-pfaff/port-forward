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

1. **Runtime info endpoint and UI display** — `GET /api/runtime` in both runtimes, display in Settings view
2. **Rule diagnostics API** — `POST /api/forwards/:id/diagnose` in both runtimes, structured pass/warn/fail result
3. **Rule diagnostics UI** — Diagnose button, results panel in the rules view or edit drawer
4. **Activity Log polish** — per-rule filter, clear button, export, throttle display improvements
5. **Settings / runtime / config polish** — config path display, copy button, runtime info, import/export UX improvements
6. **Safer networking UX pass** — stronger LAN warnings, presets, better conflict messages, Windows Firewall note
7. **Diagnostics export** — `GET /api/diagnostics` or client-side bundle, download as JSON
8. **v1.2 readiness audit, version bump, changelog, tag**

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
