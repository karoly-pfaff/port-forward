# Portier Startup & Config Recovery Policy

**Status:** Design / policy (v1.17 Slice 1). No code, API, OpenAPI, or version behavior
changes accompany this document. It defines the intended recovery behavior so the
implementation slices that follow have a single, parity-aligned target.

**Release:** v1.17 — Migration & Recovery.
**Origin:** v1.16 Resilience & Data Durability Audit, Finding **R-1**.

---

## Overview

Portier should keep its **management API reachable** whenever a startup problem is
*recoverable*, so the operator can see and fix the problem through the UI, CLI, or REST
API instead of being locked out. Today, several startup-time conditions are **fatal** to
the whole process — including the management API — in both runtimes. This document
classifies those conditions, sets the recovery principles, and proposes the concrete
behavior, observability, and file-preservation policy v1.17 will implement.

The scope here is deliberately narrow: **startup-time recovery** for config loading,
config parsing, config validation, and autostart forwarding. It is **not** a general
config migration framework (see [Migration / version policy](#migration--version-policy)).

---

## Background — v1.16 R-1 finding

From `audits/v1.16-resilience-durability-audit-1.md` §10, Finding **R-1** (non-blocking,
deferred to v1.17):

> A single enabled rule that fails to start at boot (e.g. its listen port is already in
> use, or a privileged/`0.0.0.0` bind is denied) aborts the entire service startup,
> including the management API — in both runtimes. A malformed/invalid `rules.json` has
> the same effect. The operator cannot reach the UI/CLI to fix or disable the offending
> rule, because the management API never came up.

Confirmed in code during this slice:

- **Go** (`service/sources/main.go` → `runPortier`):
  - `manager.NewFromConfig(opts.ConfigPath)` calls `config.Store.Load()`; a malformed,
    unreadable, or schema-invalid file returns an error → `runPortier` returns
    `"Failed to load config"` → `main` → `os.Exit(1)`.
  - `NewWithStore` → `ensureNoDuplicateBindings(rules)` returns an error for a persisted
    duplicate listen binding → same fatal path.
  - `forwardManager.StartEnabled()` returns on the **first** failing enabled rule →
    `runPortier` returns `"Failed to start enabled forwarding rules"` → `os.Exit(1)`.
  - The `http.Server` is created and `ListenAndServe` is called **after** all of the
    above, so the API never binds when any of them fail.
- **TypeScript / NestJS** (`server/sources/index.ts` → `main`):
  - `manager.loadAndStartEnabled()` calls `store.load()` (throws on malformed / unreadable
    / schema-invalid), `ensureNoDuplicate(rule)` per loaded rule (throws on a persisted
    duplicate binding), and `await this.startRule(...)` **with no per-rule catch** (throws
    on the first autostart bind failure).
  - Any throw rejects `main()` → `main().catch(... process.exit(1))`. The Nest app never
    reaches `app.listen(...)`, so the API never binds.

This is **existing, consistent behavior, not a migration regression**, and it is currently
**undocumented**. v1.17 changes it.

---

## Goals

- Keep the management API reachable whenever config/rule recovery is possible.
- Never silently discard or overwrite user rules or a corrupt config file.
- Never report a rule as running when its forwarder did not actually start.
- Make autostart failure for one rule non-fatal to other rules and to the API.
- Make recovery state **observable** (UI / CLI / diagnostics) and **deterministic** (testable).
- Keep Go and TypeScript recovery behavior **parity-aligned** for all API-visible behavior.
- Preserve the local-first, no-telemetry posture and the existing no-secret-leak error policy.

## Non-goals

- A general config migration framework (only a forward-compatible version-detection policy).
- Partial/automatic "repair" of a corrupt or schema-invalid config (no silent salvage).
- Any change to the duplicate-binding *definition* (`protocol + listenHost + listenPort`)
  or to the validation rules themselves.
- New remote/team/auth surfaces, telemetry, or auto-download/update behavior.
- Persisting runtime-only state (running flag, live connections, UDP sessions) across restarts.
- Installer/service/upgrade work (v1.18) and lint/security hardening (v1.19).

---

## Failure classes

Each class is described with its **current** behavior (verified in code) and the policy
questions it raises. Recommended behavior is consolidated in the
[Recommended behavior table](#recommended-behavior-table).

### 1. Config file missing

- **Current:** Non-fatal in both runtimes. `os.ErrNotExist` / `ENOENT` → `[]` rules; the
  service starts empty. Correct today.
- **Policy:** Keep non-fatal. Start with empty rules. Do **not** emit a warning/advisory
  for a simply-absent file on first run (a missing file is the normal first-run state); an
  informational log line is sufficient. No quarantine (nothing to preserve).

### 2. Config file unreadable

Examples: permission denied, file locked by another process, transient IO error.

- **Current:** **Fatal.** `Load`/`load` returns/throws the underlying IO error → fatal
  startup in both runtimes.
- **Policy questions:** Should the service start with empty active rules? Should the
  original file be left untouched? Should the API expose a degraded/recovery state? Should
  writes be blocked until resolved?
- **Direction:** Start the API in **recovery mode** with **zero active rules**. **Do not
  quarantine** (the bytes may be perfectly good — we just could not read them; moving/copying
  may also fail or lose data). **Block automatic persistence/overwrite** so a later save
  cannot clobber a file we never successfully read. Surface a clear recovery advisory.
  Distinguish "unreadable" (transient/permissions — preserve, do not quarantine) from
  "malformed" (class 3, quarantine).

### 3. Config JSON malformed

Examples: invalid JSON, truncated file, wrong top-level type.

- **Current:** **Fatal.** Go `decodeRuleItems` → `"Invalid JSON"` / `"Config file must
  contain an array of forward rules."`; TS `JSON.parse` throw / non-array `Error`. Both
  fatal. (Asymmetry: Go also accepts a `{ "rules": [...] }` object form; TS accepts only a
  bare array — see [Open questions](#open-questions).)
- **Policy questions:** Quarantine the bad file? Start with empty rules? Make a
  backup/quarantine copy? Should a later write create a fresh config or require explicit
  operator action?
- **Direction:** **Quarantine** the malformed file (copy/rename — see
  [Quarantine policy](#quarantine-and-preservation-policy)), start the API in recovery mode
  with empty rules, and **block automatic overwrite**: do not let the runtime silently
  write a fresh empty `rules.json` over the corrupt one. The operator explicitly resolves it
  (import a known-good config, or save from the UI), which then clears recovery mode.

### 4. Config schema invalid

Examples: a malformed rule object, invalid protocol/port/host, a missing required field.

- **Current:** **Fatal.** Each rule is re-validated on load
  (`validation.DecodeAndValidateForwardRule` / `validateForwardRule`); the **first** invalid
  rule fails the whole load → fatal.
- **Policy questions:** Reject the whole file? Salvage valid rules? Quarantine invalid
  rules? Avoid partial salvage to prevent silent data loss?
- **Direction:** **Reject the whole file** and treat it like class 3: quarantine, recovery
  mode, empty active rules, no auto-overwrite. **Do not partially salvage** valid rules —
  silently dropping the invalid ones is data loss the operator did not see or approve, and
  it makes the on-disk file and the running set diverge. A deliberate, explicit import-repair
  flow (operator-initiated, showing exactly which rules are dropped) is the *only* acceptable
  salvage path, and it is **deferred** beyond this recovery slice.

### 5. Duplicate binding / semantic conflict in persisted config

Example: two persisted rules claim the same `protocol + listenHost + listenPort`.

- **Current:** **Fatal.** Go `NewWithStore` → `ensureNoDuplicateBindings` returns an error
  → fatal. TS `loadAndStartEnabled` → `ensureNoDuplicate(rule)` throws on the second
  conflicting rule → fatal.
- **Policy questions:** Start with config loaded but all forwarding stopped? Disable the
  conflicting rules? Start in recovery mode?
- **Direction:** This is the one class where the rules themselves are **individually valid**
  — the file is readable, parseable, and schema-valid. **Load the rules** (so they are
  visible and editable in the UI/CLI), start the API normally, and **do not autostart the
  conflicting rules**: they remain present, but stopped, with a recovery advisory and a
  rule-level error/health that names the conflict. Non-conflicting enabled rules start
  normally. This is a **soft recovery** (no quarantine — the file is not corrupt), distinct
  from the file-level recovery modes of classes 2–4.

### 6. Enabled rule autostart bind failure

Examples: listen port already in use, permission denied, `0.0.0.0`/privileged bind denied,
invalid target resolution, OS socket failure.

- **Current:** **Fatal.** `StartEnabled` (Go) returns on the first failure; TS
  `loadAndStartEnabled` awaits `startRule` with no catch. The per-rule machinery is already
  correct (`StartRule` records `lastError`, emits `rule.error`, leaves no false running
  state) — only the **boot loop** is fatal.
- **Policy questions:** Should the API still start? Should the failed rule stay enabled but
  stopped with `lastError`? Should other enabled rules continue? Should it emit
  activity/advisory? Should startup return success?
- **Direction:** **Non-fatal.** Catch per-rule autostart failures in the boot loop so one
  failure does not abort the loop or the process. The failed rule stays **enabled but
  stopped**, with `lastError` set and `health = "error"` (the existing single-rule path
  already does this). **Other enabled rules continue** starting. The API starts. A
  `rule.error` activity event is emitted (already happens in `StartRule`). Startup is a
  **success** overall (the boot result already reports a started count; recovery just means
  `started < enabled`).

### 7. Migration / version mismatch

Examples: a future config version, an old config requiring migration, a migration failure.

- **Current:** Largely **not applicable yet**. The on-disk `rules.json` is a **bare JSON
  array of rules with no top-level version field** (the *export* envelope has
  `version: "1"`, but that is the export/import shape, not the persisted file). So there is
  no version to mismatch today. Go additionally tolerates a `{ "rules": [...] }` object;
  neither runtime reads a version from the persisted file.
- **Policy questions:** What does v1.17 support? What is deferred? How do we avoid silently
  downgrading or corrupting a newer config?
- **Direction:** v1.17 defines the **policy**, not a full framework. If/when a versioned
  config envelope is introduced, an **older Portier that encounters a newer/unknown config
  version must not overwrite it** — it enters recovery mode with a clear "this config was
  written by a newer Portier" message and leaves the file intact. A *migration* (older →
  current) is **explicit, backed up, and reversible on failure** (see migration policy).
  Building the migration command itself is later v1.17 / future scope.

---

## Recovery principles

1. **API availability over fatal startup** whenever config/rule recovery is possible.
2. **Never silently discard user rules** (no partial salvage without explicit operator action).
3. **Never overwrite a malformed or unreadable config** without first preserving it.
4. **Never report a rule as running** unless its forwarder actually started.
5. **One rule's autostart failure must not** prevent other rules or the management API from starting.
6. **Recovery state must be observable** (runtime/diagnostics/activity, surfaced in UI and CLI).
7. **Recovery must be deterministic and testable** (no time/order-dependent behavior).
8. **Recovery behavior is parity-aligned** between Go and TypeScript for all API-visible behavior.
9. **Operator-facing messages are useful but leak no secrets** (reuse the existing no-leak
   error envelope; recovery messages name the rule/binding/path, not file contents).
10. **Local-first / no-telemetry posture is unchanged** (recovery writes only local,
    inspectable files; it phones nothing home).
11. **Recovery is a transient state with a clear exit:** an explicit operator action
    (import, save, fix-and-restart) clears it; recovery mode never becomes permanent silently.

---

## Recommended behavior table

| Failure class | Recommended behavior | Preserve original? | Start API? | Start valid rules? | Rule status | Emit activity? | Surface in runtime/diagnostics? | API contract change? | Tests needed |
|---|---|---|---|---|---|---|---|---|---|
| 1. File missing | Start empty, normal | N/A (none) | ✅ | ✅ (none) | n/a | No (info log only) | Normal (not degraded) | None | Unit (already works) |
| 2. File unreadable | Recovery mode, empty active rules, **block writes** | ✅ leave untouched (no quarantine) | ✅ | ❌ (none active) | n/a (no rules loaded) | ✅ recovery event | ✅ recovery block + advisory | New (recovery block) — **deferred, documented** | Integration (where testable) |
| 3. JSON malformed | **Quarantine**, recovery mode, empty rules, **block auto-overwrite** | ✅ quarantine copy | ✅ | ❌ | n/a | ✅ recovery event | ✅ recovery block + advisory | New (recovery block) — **deferred, documented** | Unit + integration |
| 4. Schema invalid | **Reject whole file** (no partial salvage), quarantine, recovery mode | ✅ quarantine copy | ✅ | ❌ | n/a | ✅ recovery event | ✅ recovery block + advisory | New (recovery block) — **deferred, documented** | Unit + integration |
| 5. Duplicate binding (persisted) | Load rules, start API, **do not autostart conflicting rules** | ✅ no change (file valid) | ✅ | ✅ non-conflicting only | conflicting → stopped + error/health | ✅ advisory per conflict | ✅ advisory + rule health | Likely none (uses `lastError`/`health`); recovery advisory optional | Unit + integration + parity |
| 6. Autostart bind failure | **Non-fatal**; other rules continue; API starts | ✅ no change | ✅ | ✅ others | failed → enabled, stopped, `lastError`, `health=error` | ✅ `rule.error` (existing) | ✅ via existing status/health | **None** (existing fields) | Unit + integration + parity |
| 7. Future/unknown version | Recovery mode, **do not overwrite**, clear message | ✅ leave intact | ✅ | ❌ | n/a | ✅ recovery event | ✅ recovery block + advisory | New (recovery block + version) — **deferred** | Unit (once versioning exists) |

**Explicitly not salvaged:** classes 3 and 4 — a malformed file or a schema-invalid file is
**not** partially repaired or partially loaded at startup. The whole file is quarantined and
the runtime starts empty in recovery mode. Salvage is only ever an explicit, operator-visible
import-repair action, deferred beyond this slice.

---

## Recovery state / observability

Recovery state has two granularities, and they map cleanly onto existing vs new surfaces:

- **Rule-level** (classes 5 and 6): already observable. `ForwardStatus.lastError` +
  `health` (`deriveRuleHealth`) and the `rule.error` activity event already express
  "enabled but not running, with a reason." **No new API surface is required** for these.
  This is why class 6 (the core R-1 autostart fix) can ship without an API contract change.

- **Service-level / file-level** (classes 2, 3, 4, 7): there is **no rule** to attach an
  error to (the config never loaded), so a new observable surface is needed. **Recommended
  direction (future contract change, documented now, not implemented in this slice):** add
  an additive, optional `recovery` block to `GET /api/runtime`, e.g.:

  ```jsonc
  // GET /api/runtime (illustrative shape — NOT implemented in this slice)
  {
    "name": "Portier", "version": "1.17.0", "runtime": "go", /* ...existing fields... */
    "recovery": {
      "active": true,
      "reason": "config-malformed",          // missing|unreadable|malformed|schema-invalid|duplicate-binding|unsupported-version
      "configPath": "C:\\ProgramData\\Portier\\rules.json",
      "quarantinePath": "C:\\ProgramData\\Portier\\rules.json.corrupt-2026-06-18T...Z",
      "writesBlocked": true,
      "message": "rules.json could not be parsed; started with no active rules."
    }
  }
  ```

  Design constraints for that future block:
  - **Additive and optional** — existing clients that ignore unknown fields keep working;
    `recovery` absent (or `active: false`) means normal operation.
  - **Parity-required** — if added, both runtimes emit the identical shape, guarded by
    `validate:contract`, and OpenAPI is updated (`validate:openapi:go`).
  - **No secret/topology leak beyond what `/api/runtime` already exposes** — it already
    returns `configPath`; the quarantine path is the same directory, so no new sensitivity
    class is introduced. The `message` names the condition, never file contents.

- **Activity log:** emit a recovery event at startup for file-level recovery (a new event
  type, or reuse a generic config event — to be decided in the surfacing slice). Rule-level
  failures already emit `rule.error`. Any new activity event **type** is contract-guarded
  (the activity type/severity sets are validated), so this is an API-visible change to
  schedule with the surfacing slice, not the foundation slice.

- **CLI doctor / support bundle:** `portier doctor` already aggregates rule health and reads
  runtime info read-only; once the `recovery` block exists it should add a deterministic
  `recovery.active` / `recovery.clear` check. The support bundle should include the recovery
  state and the quarantine path (not the quarantined file contents). No mutation, no probing
  — consistent with the doctor/support-bundle read-only guarantees.

**Minimum viable observability** for the first implementation slices: rule-level
`lastError`/`health` (already present) for classes 5–6; a **loud, structured startup log
line** for file-level recovery (classes 2–4) until the `recovery` block lands. This lets the
R-1 autostart fix ship with **zero contract change**, deferring the `recovery` block to the
surfacing slice.

---

## Quarantine and preservation policy

Applies to classes 3 and 4 (malformed / schema-invalid). Classes 2 and 7 **preserve in
place without quarantine** (the bytes may be valid). Class 5 needs no preservation (the file
is valid).

- **Location:** the **same directory** as `rules.json`, matching the existing same-directory
  atomic-write convention (`config.Store.Save` / `ConfigStore.save`). Same-directory keeps it
  on the same filesystem (atomic rename) and keeps the artifact next to the config the
  operator already knows about. No new configurable location in v1.17 (avoid overengineering);
  revisit only if a real need appears.
- **Naming:** timestamped, derived from the original name, e.g.
  `rules.json.corrupt-<UTC-ISO-timestamp>` (colons normalized to be filename-safe on
  Windows, e.g. `rules.json.corrupt-2026-06-18T142530Z`). The timestamp guarantees we
  **never overwrite a previous quarantine**.
- **Mechanism:** prefer an atomic **rename** of the corrupt file to the quarantine name when
  we intend to start fresh; use a **copy** when we want to leave the original path populated
  for the operator to inspect. The recommended flow: quarantine by **rename** (so the
  corrupt file leaves `rules.json`), then **block auto-overwrite** of `rules.json` until the
  operator acts — i.e. the path is empty-but-protected, not silently re-created.
- **No timestamped rolling backups in v1.17** beyond the single quarantine-on-corruption
  copy. Config export/import already provides the deliberate backup/restore story; adding a
  rotating backup scheme is out of scope here.
- **If quarantine itself fails** (e.g. unreadable directory, permission denied): **do not
  fall back to deleting or overwriting** the original. Stay in recovery mode with writes
  blocked, surface that quarantine failed, and keep the original bytes exactly as found. A
  failed quarantine must never escalate to data loss.
- **Sensitivity:** quarantined files contain rule topology (hosts/ports), which the project
  already treats as locally sensitive but not secret. They stay local, are never uploaded,
  and are never included verbatim in diagnostics/support bundles (only their path is
  referenced).

---

## Migration / version policy

- **"Migration" in v1.17** means two distinct, bounded things: (a) the **recovery policy**
  in this document, and (b) a future, explicit, operator-initiated config migration command
  with dry-run/write modes (roadmap scope), backed up before writing and reversible on
  failure. This slice delivers (a) and the **policy** for (b); it does **not** build a
  general migration framework.
- **Forward compatibility (older Portier, newer config):** if a versioned config envelope is
  ever introduced, an older Portier that reads a **newer/unknown version must not overwrite
  or downgrade it.** It enters recovery mode, leaves the file intact, and reports a clear
  "written by a newer Portier" message. This is the safety guarantee class 7 protects.
- **Backward migration (older config → current):** when a migration command exists, it must
  **back up the original first, leave the original intact on failure**, and support
  **dry-run**. These mirror the existing config plan/apply and import safety guarantees;
  reuse them rather than inventing a parallel mechanism.
- **Current reality to preserve:** `rules.json` is an unversioned bare array (Go also accepts
  `{ "rules": [...] }`). Any future versioning must stay backward-compatible with the
  unversioned array, or provide a transparent one-way upgrade that is backed up first.
- **Deferred:** the migration command implementation, the versioned envelope format, and any
  schema-version negotiation are **not** part of the recovery foundation slices.

---

## Implementation slices

1. **Recovery policy / design document** — *this slice.* No code/API/version change.
2. **Go startup recovery foundation (done):** config-load failures (classes 2–4) are
   non-fatal at boot; the management API starts with zero active rules; recovery is carried
   by an internal `recovery.State`; the bad file is quarantined where safe; startup logs a
   loud `Warn`; writes are blocked while recovery is active. Tests. No API contract change.
   Implemented as:
   - **Package `service/sources/recovery`** owns `State` (reason, message, configPath,
     quarantinePath, writesBlocked, detectedAt) and `LoadConfig` (never returns a fatal
     error; classifies and returns empty rules + `*State`).
   - **Classification uses typed errors, not string matching:** `config.Parse` (extracted
     from `config.Store.Load`) wraps `config.ErrMalformed` / `config.ErrSchemaInvalid`, and
     the loader distinguishes unreadable (read error) vs malformed vs schema-invalid.
   - **Quarantine** = atomic same-directory rename to `rules.json.corrupt-<UTC>` (with a
     `-N` suffix on collision so a prior quarantine is never overwritten). Unreadable files
     are **not** quarantined (preserved in place); a failed quarantine leaves the original
     untouched and keeps recovery active.
   - **Write-block** is enforced at the single `manager.persist()` choke point, so
     create/update/delete/reorder/import are all refused with a `manager.RecoveryError`
     while recovery is active — preventing a fresh empty config from overwriting the bad
     one. The error flows through the existing error envelope (generic 500) for now; a
     dedicated status/surface is Slice 5.
   - **Deferred to Slice 3:** the persisted duplicate-binding guard (class 5) and per-rule
     autostart failures (class 6) are still fatal at boot; this slice covers config-load only.
3. **Go autostart recovery — core R-1 fix (done):** autostart is non-fatal. `StartEnabled`
   now returns an internal `StartEnabledResult` (attempted/started/failed/skipped) instead of
   aborting on the first failure, so one rule's bind failure (class 6) leaves it
   enabled-but-stopped with `lastError`/`health=error` (via `StartRule`, which already emits
   `rule.error`) while every other enabled rule still starts and the API always binds.
   Persisted duplicate listen bindings (class 5) are soft-recovered: `NewWithStore` no longer
   rejects them at construction (rules load), and `StartEnabled` skips every enabled rule that
   shares a binding with another enabled rule (deterministic, rule-order reporting), marking
   each stopped/error with a conflict `lastError` — it does not pick an arbitrary winner,
   mutate `enabled`, delete, or rewrite the config. An enabled rule that shares a binding only
   with *disabled* rules still autostarts (no runtime conflict). `main.go` logs a per-rule
   `Warn` for failures/skips. Create/update/import duplicate validation stays strict. No API
   contract change; reuses `rule.error` activity. (Reserved `recovery.State` reasons
   `duplicate-binding`/`unsupported-version` remain unused — duplicate conflicts are surfaced
   per-rule via `lastError`, not as a file-level recovery state.)
4. **TypeScript / NestJS parity recovery (done):** the reference runtime now follows the same
   operator policy as Go (Slices 2+3). Implemented as:
   - **Config-load recovery:** `parseConfig` extracted to `persistence/config-parse.ts` with
     typed errors (`MalformedConfigError`/`SchemaInvalidConfigError`); new
     `recovery/config-recovery.ts` (`loadConfigWithRecovery`) classifies unreadable/malformed/
     schema-invalid, quarantines the bad file to the same-directory `rules.json.corrupt-<UTC>`
     (with `-N` collision suffix; rename seam for the failure path), and returns empty rules +
     a `RecoveryState`. `ConfigStore.loadWithRecovery()` delegates to it.
   - **Non-fatal autostart:** `ForwardManager.loadAndStartEnabled()` returns an internal
     `LoadAndStartResult` (attempted/started/failed/skipped) instead of throwing; persisted
     duplicate bindings load (the load-time `ensureNoDuplicate` guard was removed; create/
     update/import stay strict) and conflicting enabled rules are skipped via
     `conflictingEnabledBindings` (same deterministic, rule-order, names-listed message as Go).
   - **Status parity fix:** the manager now tracks a `lastErrors` map so a failed/skipped rule
     reports `lastError` + `health:error` even with no live forwarder. Previously TS dropped the
     forwarder on a failed start and surfaced no `lastError` (it showed `health:warning`) — this
     aligns TS with Go's behavior.
   - **Write-block:** `persist()` throws `RecoveryError` while recovery is active; `mapManagerError`
     re-throws it to the generic-500 envelope (parity with Go; no schema change).
   - **Bootstrap:** `index.ts` consumes the summary, logs recovery + per-rule warnings, and no
     longer exits on these conditions. No public API/OpenAPI change.
5. **API / diagnostics / UI / CLI surfacing (done):** the recovery state is now observable.
   - **API:** `GET /api/runtime` carries an additive, always-present `recovery` block
     (`{ active: false }` normally; full block when active) in both runtimes, with identical
     shape (kebab `reason`, RFC3339 `detectedAt` matching `startedAt`). Shared type
     `RuntimeRecovery` / `RuntimeRecoveryReason`; mapped by Go `recoveryResponse` and TS
     `toRuntimeRecovery`. OpenAPI documents it (`RuntimeRecoveryDto`); `validate:contract`
     asserts the inactive shape + cross-runtime parity (active shape covered by unit tests).
   - **CLI:** `portier doctor` emits a `config.recovery_active` **warning** check only when
     recovery is active (with the safe reason + quarantine path in details); inactive adds no
     check. The support bundle already includes `runtime.json` (now with the recovery block)
     and the doctor report.
   - **UI:** an amber `RecoveryBanner` renders at the top of the app only when
     `runtime.recovery.active` — showing the message, the write-block note, and the quarantine
     path. Per-rule autostart/duplicate failures do **not** trigger it (they stay in rule status).
   - No recovery **activity event** was added (deferred — logging + the runtime block suffice;
     a new activity type is a contract-guarded taxonomy change not needed here).
6. **Docs / checklist / changelog:** finalize operator docs, add validation checklist
   entries, record the v1.17 changelog entry on release.
7. **Full validation and v1.17 release prep:** run the complete matrix; only then consider a
   version bump (out of scope for the design slice).

Sequencing rationale: slices 2–4 deliver the R-1 fix (no lockout) with **no contract
change**, so the highest-value safety improvement can ship and be validated independently of
the observability surface added in slice 5.

---

## Testing plan

Designed before implementation; classified by level.

**Unit**
- Config-store load: missing → empty (regression, both runtimes); malformed JSON →
  classified malformed (not fatal); not-an-array / wrong top-level type → malformed;
  schema-invalid rule → classified schema-invalid, **whole-file rejected, no partial salvage**.
- Quarantine helper: produces a timestamped same-directory name; never overwrites an
  existing quarantine; rename vs copy behavior; quarantine-failure path does **not** delete
  or overwrite the original.
- Recovery-state value: correct `reason`/`writesBlocked` per class; write-block prevents
  auto-overwrite.

**Integration**
- Malformed `rules.json` → service boots, management API reachable, zero active rules,
  recovery state set, original quarantined, no silent overwrite.
- Unreadable config (where testable per-OS) → boots in recovery mode, file untouched, writes
  blocked.
- Duplicate-binding persisted config → boots, rules loaded, conflicting rules not started
  and marked error/health, non-conflicting rules start.
- Autostart bind failure (port in use) → service boots, API reachable, failed rule
  enabled+stopped+`lastError`+`health=error`, **other enabled rules still start**, startup
  reports success with `started < enabled`.
- Future/unknown config version (once versioning exists) → recovery mode, file not
  overwritten, clear message.

**Contract (parity)** — for slice 5 / API-visible behavior only
- `recovery` block shape identical across Go and TS (`validate:contract`).
- Recovery activity event type/severity present in both runtimes' contract guards.
- Rule-level `lastError`/`health` parity for classes 5–6.

**Runtime smoke** — after the surfacing slice
- Boot from a release artifact with a corrupt `rules.json`; assert `/api/health` and
  `/api/runtime` (with `recovery.active`) respond.

**E2E / UI** — after the surfacing slice
- Recovery banner/message renders when the runtime reports recovery; operator can import a
  config to clear it.

---

## Validation plan

The implementation slices (not this design slice) should run, scaled to what each touches:

- `npm run lint`
- `npm run typecheck`
- `npm run test` (and `npm run test:cli` if the CLI is touched)
- `go build ./...`, `go vet ./...`, `go test ./...`
- `npm run validate:coverage:service`
- `npm run validate:contract` — when API-visible behavior changes (slice 5)
- `npm run validate:openapi:go` — when routes/OpenAPI change (slice 5)
- `npm run validate:runtime:smoke` — after startup/runtime changes
- `npm run validate:config` — if migration/config fixtures are added

Do not lower coverage gates. Raise a gate only if it is safely above threshold after
implementation (e.g. once the long-missing `config` error-path tests land).

**This design slice** runs only `npm run lint` and `npm run typecheck` (docs-only change).

---

## Deferred items

- The additive `recovery` block on `GET /api/runtime`, recovery activity event(s), doctor
  check, support-bundle field, and UI banner — **slice 5** (carries the contract/OpenAPI
  changes).
- The explicit, operator-initiated config migration command (dry-run/write, backup-first,
  reversible) and any versioned config envelope — later v1.17 / future.
- Explicit, operator-visible **import-repair / partial salvage** of a schema-invalid config —
  deferred; never automatic.
- Configurable quarantine location and rotating backups — out of scope unless a real need
  appears.
- O-1 (unwired UDP `PruneExpired` / one-way session registry growth) — tracked separately;
  not a startup-recovery concern.

---

## Open questions

1. **Go vs TS malformed-tolerance asymmetry:** Go's `decodeRuleItems` accepts both a bare
   array and a `{ "rules": [...] }` object; TS accepts only a bare array. Should the
   recovery work converge these (e.g. TS also accepts the object form), or is the object
   form deprecated? This affects which inputs are classified "malformed" per runtime.
2. **Recovery activity event:** new dedicated event type(s) vs reusing a generic config
   event. New types are contract-guarded; decide in the surfacing slice.
3. **Write-block UX:** while in file-level recovery with writes blocked, should the UI/CLI
   *create* operations be rejected with a clear "resolve recovery first" error, or should the
   first successful save/import be the explicit action that clears recovery and replaces the
   quarantined file? (Leaning toward the latter.)
4. **Unreadable-on-some-OS testability:** simulating permission-denied/locked files
   deterministically in CI differs by platform; decide how much to cover with a file-ops
   seam vs accept as a documented manual check.
