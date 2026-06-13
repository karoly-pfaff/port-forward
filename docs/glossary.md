# Portier Glossary

Canonical terms used across Portier docs, code reviews, audit work, and future v1.x development. The goal is one shared vocabulary so the same concept is named the same way in prose.

This file does **not** rename any existing API field, REST path, or CLI command. Where a frozen public term differs from the preferred prose term, the exception is called out. Treat "avoid/alias" words as synonyms in writing — do not rename code to match them.

Source: `audits/v1.6-readability-naming-audit-1.md` (Naming-A).

---

## Core concepts

| Term | Meaning | Notes / frozen exceptions |
| --- | --- | --- |
| **Forward rule** (short: **rule**) | A configured TCP/UDP port forward: `protocol + listenHost:listenPort → targetHost:targetPort`, with `enabled` and (for UDP) `udpMode`. | Domain type `ForwardRule`; UI nav "Forward Rules". Prefer "forward rule" / "rule"; avoid "forward" alone as a noun. |
| **Group** | An optional, behavior-neutral label on a forward rule (`ForwardRule.group`) used to organize rules (v1.8 Operator Power Tools foundation). Metadata only — does **not** affect forwarding, lifecycle, duplicate-binding, or status. Optional; trimmed; empty normalizes to absent; ≤ 64 chars, no control characters. | Added v1.8 Slice 1. A material (drift-producing) but **non-destructive** plan field. Not a "tag" or "profile" (those are not multi-valued/separate concepts yet). Frozen field name `group`. |
| **`/api/forwards`** | The REST collection of forward rules (`GET`/`POST`/`PATCH`/`DELETE /api/forwards[/:id]`). | **Frozen REST path** — the one place "forward" is the noun. Do not rename. |
| **Config** | Overloaded prefix — always qualify it. Distinct meanings: rule persistence (`rules.json`), the plan/apply engine, the export/import DTO, and the desired-state input. The bare word "config" alone is ambiguous; use one of the qualified terms below. | — |
| **Exported config** | The serialized bundle of all current rules: `{ version: "1", exportedAt, rules: [...] }`. | DTO `ExportedConfig`; `GET /api/config/export`. |
| **Desired config** | The target rule set supplied as input to plan/apply (the `desired` request field). | Type `DesiredConfig`. Avoid "target config". |
| **Config plan** | A computed diff between the running rules and a desired config: operations (add/update/remove/unchanged), summary counts, drift/error flags, warnings. Read-only — computes, never mutates. | `POST /api/config/plan`, `ConfigPlanResponse`. |
| **Config diff** | A human-readable *presentation* of a config plan (CLI `config diff`). | Same underlying plan as `config plan`; a view, not a separate computation. |
| **Config apply** | Plan-based application of a desired config: build a plan, gate on errors/destructive/dry-run, then (on drift) perform a replace-import. | `POST /api/config/apply`. "Apply" is plan-based; it *uses* replace-import internally — that is an implementation detail, not "import". |
| **Config import** | Bulk load of an exported config in one of two modes. | `POST /api/config/import`; `ImportMode` = `merge` \| `replace`. Avoid "load". |
| **Config export** | Read-only dump of the current rules as an exported config. | `GET /api/config/export`. |
| **Drift** | The running config differs from the desired config (`plan.summary.hasDrift`). | CLI `--fail-on-drift`. Avoid "difference". |
| **Import mode: merge** | Add imported rules to the existing set; clashing IDs are regenerated; a listen-binding conflict (with an existing rule or within the imported set) rejects the whole import. | — |
| **Import mode: replace** | Stop all rules, remove all existing rules, apply the imported set, restart enabled ones. A duplicate listen binding within the imported set rejects the whole import. | Config apply uses replace-import under the hood. |

---

## Runtime terminology

| Term | Meaning | Notes |
| --- | --- | --- |
| **Runtime** | The generic concept of "an implementation of the Portier service contract", and the name of the API field/endpoint that reports which one is running. | Reserve "runtime" for the generic concept and the API surface. Do not call a specific implementation "the runtime". `GET /api/runtime`; field `runtime: "node" \| "go"`. |
| **Server** / **TypeScript server** / **Node fallback** | The Node/Express implementation (`@portier/server`, `server.js`). The documented fallback runtime; requires Node.js. | API runtime value is `"node"`. When distinguishing repo components, "server" means this. Avoid "TypeScript runtime" as a label. |
| **Service** / **Go service** / **native service** | The native Go implementation (`service` / `service.exe`, `service/`). The **preferred** packaged production runtime. | API runtime value is `"go"`. Avoid "Go server". |
| **CLI** | The Go API client under `tools/cli/` (`portier` / `portier.exe`). Talks to the management API; does **not** start the service or contain runtime/forwarding logic. | Stays a pure API client. |
| **Client** / **web UI** | The React single-page app under `client/`, served from `web/`. | "Client" = the browser UI, distinct from the "CLI". |

The two runtimes implement **one** REST contract; the CLI carries a third (DTO-only) copy. `validate:contract` guards parity across all three.

---

## Rule lifecycle and state

| Term | Meaning | Notes |
| --- | --- | --- |
| **enabled** | Persisted *desired* on/off state of a rule (`ForwardRule.enabled`, stored in `rules.json`). Enabled rules are started at launch. | Desired state, not observed state. |
| **running** | Observed *runtime* state — whether the forwarder is currently bound and serving (`ForwardStatus.running`). | Observed state. A rule can be **enabled but not running** after a failed start (e.g. port already in use). |
| **status** | The observed runtime view of a rule: `running`, byte/packet counters, `lastError`, active connections/sessions (`ForwardStatus`, `GET /api/status`). | Distinct from the persisted rule. |
| **health** | An operator-facing *interpretation* of a rule's status — `healthy` / `warning` / `error` (`ForwardStatus.health`). **Derived deterministically** from `enabled`/`running`/`lastError`; performs no target probing. | Added v1.8. **Distinct from `status`/`running`** (lifecycle) — health is the operator reading. `error` = has `lastError`; `warning` = enabled but not running; `healthy` = running clean or intentionally stopped. Does NOT imply active monitoring. |
| **lastError** | The last forwarding error observed for a rule (bind failure, socket error), surfaced on its status. | Best-effort diagnostic; cleared on a successful (re)start. |
| **Startup behavior** | On launch, enabled rules are started automatically (TS `loadAndStartEnabled`, Go `StartEnabled`). | There is no separate "autostart" field — "enabled" *is* the start-on-launch flag. |
| **Duplicate rule** | A UI convenience (web client) that opens the create form pre-filled from an existing rule, so a new rule can be built from it. Added v1.8 Slice 8. | **Client-only, create flow** — no backend "duplicate" endpoint. The source rule is never modified; the duplicate saves through the normal create path (`POST /api/forwards`). The copy gets a `<name> copy` name, **autostart forced off**, no id, and the source's `group`; runtime-only state (status/`lastError`/health) is not copied. |

---

## Activity and diagnostics

| Term | Meaning | Notes |
| --- | --- | --- |
| **Activity event** | An entry in the in-memory activity log (`ActivityEvent`, `GET /api/activity`): rule lifecycle, TCP connection, UDP packet/session, and config events. | Use "activity event", not "log event"/"log entry". |
| **Activity type** | The event-type value (e.g. `rule.started`, `config.import.failed`). 17 canonical values. | **Contract value**, not a cosmetic label — guarded by `validate:contract` (see `docs/api-contract.md` and the Fix Slice 5 rule). |
| **Activity severity** | The event severity: `info` \| `success` \| `warning` \| `error`. | Contract value; guarded for value/parity. |
| **Advisory** | The structured port-advisory **object** `{ code, severity, message }` (`PortAdvisory`, `GET /api/ports/advisory`). | The *object*. Advisory wording is canonical in `@portier/shared`. |
| **Warning** | A **severity/category**, not an object: advisory severity `warning`, and the plan type `ConfigPlanWarning`. | Keep distinct from "advisory". Do not call an advisory object "a warning". |
| **Diagnose** | A per-rule diagnostic action: run a set of checks against one rule (`POST /api/forwards/:id/diagnose`, CLI `diagnose`, `RuleDiagnosticsResult`). | Verb/action on a single rule. |
| **Diagnostic check** | One check within a diagnose result (`DiagnosticCheck`: id/label/status/message). | The unit inside a diagnose. |
| **Diagnostics export** | A support **bundle** across the whole runtime (CLI `diagnostics export`). A distinct feature from per-rule diagnose. | Don't conflate with "diagnose". |
| **Doctor** | A deterministic operator diagnostic that runs a set of **checks** and reports a graded summary (v1.9 Doctor & Config Toolkit). Two forms: **config doctor** (CLI `config doctor <file>`) — an **offline** analysis of a local config file; and the live **runtime doctor** (CLI `doctor`) — read-only checks against the running runtime (reachability, version, rule health, config readability). | A graded multi-check report, not a single per-rule action (that is "diagnose"). The offline config doctor must not require a live runtime; neither doctor mutates anything or probes targets. The live doctor reads rule health from the API `health` field — it does not re-derive it. |
| **Doctor check** | One finding within a doctor report (`DoctorCheckResult`: `code`/`severity`/`title`/`message`/optional `details`). Each has a stable, operator-facing **check code** (e.g. `config.valid`, `config.lan_exposure`). | The unit inside a doctor report. Check codes are a CLI/tool contract — do not rename casually. |
| **Support bundle** | The doctor-centric **directory** of artifacts produced by CLI `support-bundle --out <dir>` (v1.9): `manifest.json`, `doctor.json` (same schema as `doctor --json`), `doctor.txt`, `explanations.json`, and optional `runtime.json`/`config-export.json`. | A directory of doctor artifacts. Distinct from **Diagnostics export** (the single-file JSON bundle). Reuses the doctor report schema — no second schema. Excludes env/process/logs/tokens. |
| **AI handoff prompt** | A reusable copy-paste prompt (`docs/prompts/doctor.md`, v1.9) for asking an AI assistant to interpret doctor output. **Portier sends nothing** — the user pastes locally-generated output into an assistant of their choice. | Plain-text template only; no AI integration/upload/telemetry. Tells the assistant to treat the report as source of truth and never to request secrets. |
| **Policy** | An operator-defined set of **safe-operation guardrails** evaluated against a config file (v1.10). A small JSON file (`schemaVersion: 1` + a `rules` object of boolean guardrails: `requireGroup`, `allowLanExposure`, `allowPrivilegedPorts`, `allowAutostart`, `forbidDuplicateBindings`). Evaluated **offline** by CLI `policy check --config <file> --policy <file>`. | Guardrails, **not** runtime/config diagnostics ("doctor") and **not** field validity ("validate"). Dry-run only in v1.10 — no enforcement/automation/mutation. No `allowUdp`/protocol restriction (UDP is first-class). |
| **Policy finding** | One result within a policy report (`PolicyFinding`: `code`/`severity`/`title`/`message`/optional `details`), aggregated into a `PolicyReport` (`findings`/`summary`/`result`). Each has a stable, operator-facing **policy code** (e.g. `policy.lan_exposure_forbidden`, `policy.valid`). | The unit inside a policy report. Policy codes are a CLI/tool contract — do not rename casually. Distinct from a **doctor check**. |
| **Policy template** | A built-in, named starter **policy** file (v1.10). Three deterministic templates — `local-safe`, `managed`, `permissive` — each a complete `schemaVersion: 1` policy printed by CLI `policy template <name>` (`--list` to enumerate, `--out <file>` to save). | A convenience for producing a valid policy file; the rendered/saved output is directly usable by `policy check`. Same schema as a hand-written policy — no extra guardrails, no protocol restriction/`allowUdp`. |
| **Explanation** | Static, deterministic, **offline** reference text for one stable code (`{code, title, meaning, action, severity, related}`). Covers both doctor/check codes and policy finding codes. Surfaced by CLI `explain <code>` / `explain --list`, and inline via `--explain` on `doctor` / `config doctor` / `policy check`. | One shared type/registry model (`tools/cli/sources/explain`); each domain owns its explanation data. Reference text only — never implies probing, enforcement, or automatic remediation. A policy explanation frames a finding as a **policy choice**, not necessarily a defect. |
| **Live connection** | A tracked **TCP** connection (`TcpConnectionInfo`, "Live Connections" view, `GET /api/connections`). | TCP = connection. |
| **UDP session** | A tracked **UDP** flow (`UdpSessionInfo`). UDP has no connection, so Portier tracks sessions. | UDP = session. Keep the TCP/UDP split deliberate. |

---

## Naming rules / exceptions

- Prefer **"forward rule"** in prose; **"rule"** is the acceptable short form. Avoid "forward" alone as a noun.
- **`/api/forwards`** is a frozen REST path — do not rename it.
- **Do not rename public API fields** for cosmetic consistency — including `listenHost` / `targetHost` (rule config) and `clientAddress` / `targetAddress` (observed connection). They name the same kind of value at different layers; that is intentional and frozen.
- When distinguishing runtimes, prefer **"Go service"** and **"TypeScript server / Node fallback"**. Reserve **"runtime"** for the generic concept and the `runtime` API field (values `node` / `go`).
- Use **"config apply"** only for plan-based apply, never for raw import. Use **"config import"** for `/api/config/import` (modes `merge` / `replace`).
- Use **"activity event"** (not "log event") for `/api/activity`; activity type/severity are contract values.
- Use **"advisory"** for the object and **"warning"** for a severity/category — keep them distinct.
- Use **"live connection"** for TCP and **"UDP session"** for UDP.
- Use **"diagnose"** for the per-rule action and **"diagnostics export"** for the support bundle.
- Use **"doctor"** for a graded, multi-check diagnostic report (e.g. **config doctor**); keep it distinct from per-rule "diagnose". **Doctor check codes** (e.g. `config.valid`) are stable operator-facing identifiers.
- Use **"policy"** for operator-defined safe-operation guardrails (CLI `policy check`); keep it distinct from "doctor" (diagnostics) and "validate" (field validity). **Policy finding codes** (e.g. `policy.lan_exposure_forbidden`) are stable operator-facing identifiers. There is no protocol-restriction/`allowUdp` policy — UDP is first-class.

New docs, API responses, CLI output, or UI labels should use these terms, or update this glossary if a genuinely new concept is introduced. Renames of frozen public terms are out of scope; CLI command-file naming cleanup and UI wording alignment are deferred follow-ups (see the readability/naming audit).
