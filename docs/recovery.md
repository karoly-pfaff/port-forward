# Portier Startup & Config Recovery

Portier keeps its **management API reachable whenever a startup problem is recoverable**, so you can
see and fix the problem through the UI, CLI, or REST API instead of being locked out. A bad or
unbindable configuration no longer kills the service.

This behavior is shipped in both runtimes (the Go service and the TypeScript/NestJS server) with
parity-aligned, API-visible behavior. For the release history, see
[changelog.md](changelog.md); for canonical terms (recovery mode, quarantine, config migrate), see
[glossary.md](glossary.md).

## Principles

1. **API availability over fatal startup** whenever config/rule recovery is possible.
2. **Never silently discard user rules** — no partial salvage without an explicit operator action.
3. **Never overwrite a malformed or unreadable config** without first preserving it.
4. **Never report a rule as running** unless its forwarder actually started.
5. **One rule's autostart failure must not** prevent other rules or the management API from starting.
6. **Recovery state is observable** through the runtime API, diagnostics, the CLI, and the UI.
7. **Recovery is deterministic and parity-aligned** between the Go and TypeScript runtimes.
8. **Messages name the rule/binding/path, never file contents** — recovery leaks no secrets and adds
   no telemetry.

## Failure Classes And Current Handling

| Condition | Behavior | Original config | Active rules |
| --- | --- | --- | --- |
| Config file missing | Normal startup, empty rule set (normal first-run state) | n/a | none |
| Config file unreadable | Recovery mode, writes blocked | Preserved in place (not quarantined) | none |
| Config JSON malformed | Recovery mode, writes blocked | Quarantined | none |
| Config schema invalid | Recovery mode, writes blocked (whole file rejected, no partial salvage) | Quarantined | none |
| Persisted duplicate binding | Normal startup; conflicting rules load but do not autostart | Unchanged (file is valid) | non-conflicting rules |
| Enabled rule autostart fails | Non-fatal; other rules still start | Unchanged | all other enabled rules |

Two granularities:

- **File-level recovery** (unreadable / malformed / schema-invalid config): the config never
  loaded, so the runtime starts with **zero active rules in recovery mode**, blocks writes, and
  surfaces a service-level `recovery` block.
- **Rule-level recovery** (duplicate binding / autostart failure): the config is valid and loads
  normally. The affected rule stays **enabled but stopped** with `lastError` set and
  `health: "error"`; other rules and the API start normally. These do not enter the service-level
  recovery block — they are visible in the rule's status.

A malformed or schema-invalid file is **never partially repaired or partially loaded** at startup.
The whole file is quarantined and the runtime starts empty in recovery mode. Salvage is only ever an
explicit, operator-visible import-repair action.

## Quarantine & Preservation

Applies to malformed and schema-invalid configs. Unreadable files are preserved in place (the bytes
may be valid — they just could not be read), and a valid-but-conflicting config needs no
preservation.

- **Location:** the same directory as `rules.json`, on the same filesystem (atomic rename).
- **Naming:** a timestamped sibling, `rules.json.corrupt-<UTC>`, with a numeric suffix on collision
  so a previous quarantine is never overwritten.
- **Write-block:** while file-level recovery is active, create/update/delete/reorder/import are
  refused, so a fresh empty config can never silently overwrite the bad one. The first successful
  operator action (import a known-good config, or save) clears recovery and replaces the quarantined
  file.
- **Quarantine failure is never escalated to data loss:** if the quarantine rename itself fails, the
  original bytes are left exactly as found and recovery stays active.

Quarantined files contain rule topology (hosts/ports), which Portier treats as locally sensitive but
not secret. They stay local, are never uploaded, and are referenced by **path only** in
diagnostics/support bundles.

## Observability

Recovery state is visible everywhere an operator looks:

- **REST API:** `GET /api/runtime` always carries a `recovery` block — `{ active: false }` during
  normal operation, and a full block (`reason`, `message`, `configPath`, `quarantinePath`,
  `writesBlocked`, `detectedAt`) when active. The shape is identical across both runtimes and
  documented in OpenAPI. Rule-level failures are visible through each rule's `lastError` and
  `health` instead.
- **CLI:** `portier doctor` emits a `config.recovery_active` warning check when recovery is active,
  with the reason and quarantine path; the support bundle includes the recovery state via
  `runtime.json`.
- **UI:** an amber recovery banner renders at the top of the app only when the runtime reports an
  active recovery. Per-rule autostart/duplicate failures stay in the rule's status, not the banner.
- **Logs:** a loud, structured startup log line records file-level recovery.

## No Automatic Migration

Portier does **not** auto-migrate the persisted config at startup. `rules.json` stays a
backward-compatible unversioned JSON array, and the runtime never rewrites it to a new shape on
boot.

Config normalization is an explicit, offline, operator-driven step:

- **`portier config migrate <file>`** classifies a config file (bare array, wrapper object, or
  exported envelope), validates it, and normalizes a valid config to the canonical bare-array
  shape.
- It is **dry-run by default**; `--write` is **backup-first** (`<file>.bak-<UTC>`, never overwriting
  a prior backup) and atomic.
- A malformed, schema-invalid, or unsupported-version config is **never written or overwritten** —
  migrate reports and exits non-zero.

If a versioned persisted config format is ever introduced, an older Portier that reads a
newer/unknown version must not overwrite or downgrade it — it enters recovery mode, leaves the file
intact, and reports a clear "written by a newer Portier" message. This `unsupported-version` recovery
reason is reserved until such a format exists.

## Validation

A packaged-runtime recovery smoke boots the Go service against a corrupt `rules.json` and asserts
`GET /api/runtime` reports `recovery.active`:

```powershell
npm run validate:runtime:recovery-smoke   # recovery scenario only
npm run validate:runtime:smoke            # normal and recovery startup
```

`npm run validate:config` also asserts recovery-mode startup: a malformed or duplicate-binding
config makes the runtime start in recovery mode rather than abort. Cross-runtime parity of the
`recovery` block is guarded by `npm run validate:contract`.
