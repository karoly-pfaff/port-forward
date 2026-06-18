# Portier Glossary

Canonical terms for Portier docs, UI copy, CLI output, reviews, and API references.
This glossary names concepts; it does not rename frozen API fields, REST paths, or CLI
commands. Keep entries short. Put feature history in `docs/changelog.md` or
`docs/roadmap.md`.

## Core Terms

- **Forward rule**: A configured TCP or UDP port forward:
  `protocol + listenHost:listenPort -> targetHost:targetPort`, plus `enabled` and,
  for UDP, `udpMode`. "Rule" is the acceptable short form.
- **Group**: Optional rule metadata (`ForwardRule.group`) used to organize rules.
  It is behavior-neutral except for explicit group actions. It does not affect forwarding,
  duplicate-binding checks, status, or lifecycle.
- **Config**: Ambiguous by itself. Prefer a qualified term: exported config, desired
  config, config plan, config apply, config import, or config export.
- **Exported config**: The portable current-rule bundle:
  `{ version, exportedAt, rules }`.
- **Desired config**: The target rule set supplied to plan/apply.
- **Config plan**: A read-only computed diff between current rules and a desired config.
- **Config diff**: A human-readable presentation of a config plan.
- **Config apply**: Plan-based application of a desired config after required gates
  such as errors, dry-run, and destructive confirmation.
- **Config import**: Bulk loading of an exported config in `merge` or `replace` mode.
- **Config export**: Read-only dump of current rules as an exported config.
- **Drift**: The current config differs from the desired config.
- **Duplicate binding**: A conflicting listen tuple:
  `protocol + listenHost + listenPort`. TCP and UDP may use the same port number
  because they are different protocols.

## Runtime Terms

- **Runtime**: The generic implementation of the Portier service contract. Also the
  API field whose values are `"node"` or `"go"`.
- **TypeScript server** or **Node fallback**: The `server/` runtime and packaged
  `server.js` fallback. API runtime value: `"node"`.
- **Go service** or **native service**: The `service/` runtime and preferred packaged
  production service. API runtime value: `"go"`.
- **CLI**: The Go `portier` command under `tools/cli/`. It talks to the management API
  unless a command is explicitly offline.
- **Client** or **web UI**: The React app under `client/`, served from `web/`.
- **Replay tool**: The offline `tools/replay` tool. It reads saved artifacts and never
  contacts the runtime, executes workflows, mutates inputs, or uploads data.
- **Recovery mode** (or **configuration recovery mode**): A degraded startup state (v1.17)
  the service enters when the persisted config cannot be loaded (unreadable, malformed, or
  schema-invalid). The management API stays reachable with no active rules; the bad config
  is preserved/quarantined; rule writes are blocked until it is repaired. Surfaced on
  `GET /api/runtime` as the `recovery` block. See [recovery.md](recovery.md).
- **Quarantine**: The preserved copy of a bad config file moved aside during recovery, kept
  in the same directory with a timestamped `*.corrupt-<UTC>` name so the original data is
  never lost or silently overwritten.
- **Config migrate**: The offline `portier config migrate` command (v1.17). It inspects a
  local config file, classifies its format, and normalizes a valid config to the canonical
  persisted bare-array shape. Dry-run by default; `--write` is backup-first
  (`*.bak-<UTC>`) and atomic, and never writes a malformed/schema-invalid/unsupported config.
  Portier does not auto-migrate the persisted config at startup.

## Rule State

- **enabled**: Persisted desired state. Enabled rules start automatically at launch.
- **running**: Observed runtime state; the forwarder is bound and serving.
- **status**: Observed runtime view: running state, counters, active connections or
  sessions, `lastError`, and timestamps.
- **health**: Operator-facing interpretation of existing state:
  `healthy`, `warning`, or `error`. Derived from `enabled`, `running`, and `lastError`;
  it never implies target probing or background monitoring.
- **lastError**: Last observed forwarding error for a rule. Cleared on successful start.
- **Duplicate rule**: A client-only UI convenience that opens the create form prefilled
  from another rule. It creates through the normal API path and does not copy runtime state.

## Activity And Diagnostics

- **Activity event**: An in-memory event in `/api/activity`: rule lifecycle, TCP
  connection, UDP packet/session, or config event.
- **Activity type**: The event type value such as `rule.started` or
  `config.import.failed`. It is a contract value, not a cosmetic label.
- **Activity severity**: `info`, `success`, `warning`, or `error`.
- **Advisory**: A structured port-advisory object `{ code, severity, message }`.
- **Warning**: A severity/category. Do not use it as a synonym for the advisory object.
- **Diagnose**: A per-rule diagnostic action (`POST /api/forwards/:id/diagnose`,
  CLI `diagnose`).
- **Diagnostic check**: One check inside a diagnose result.
- **Diagnostics export**: A runtime-wide support export. Distinct from per-rule diagnose.
- **Doctor**: A deterministic multi-check operator diagnostic. `config doctor` analyzes
  a local config file offline; `doctor` reads the live runtime without mutating it.
- **Doctor check**: One finding in a doctor report. Check codes such as `config.valid`
  are stable operator-facing identifiers.
- **Support bundle**: A doctor-centered directory of support artifacts. It excludes
  secrets, environment dumps, process lists, and logs.
- **AI handoff prompt**: A local prompt template for asking an assistant to interpret
  Portier output. Portier does not upload anything.

## Policy And Workflow Terms

- **Policy**: Operator-defined safe-operation guardrails evaluated against a config.
  Policy evaluation is dry-run unless a future feature explicitly says otherwise.
- **Policy finding**: One result in a policy report. Policy codes are stable
  operator-facing identifiers.
- **Policy template**: A built-in starter policy file that renders to the same schema
  accepted by `policy check`.
- **Policy review**: Offline comparison of current vs candidate config that evaluates
  only the candidate against a policy and reports a compact summary.
- **Policy baseline**: A compact snapshot of accepted policy findings for later compare.
  It is not a config copy.
- **Finding fingerprint**: Stable identifier used to compare policy findings without
  depending on volatile rule IDs.
- **Explanation**: Static offline reference text for a stable code. Explanations do not
  imply probing, enforcement, or automatic remediation.
- **Workflow**: An ordered JSON-defined sequence of existing safe Portier operations.
  It is not a general scripting language.
- **Workflow plan**: Dry-run validation of a workflow definition.
- **Workflow runbook**: Command preview for a valid workflow; it never executes commands.
- **Workflow run**: Read-only execution of a valid workflow through existing evaluators.
  It never mutates config, enforces policy, schedules work, or runs shell commands.
- **Workflow support report**: Offline bundle made from an existing workflow report.
- **Workflow run history**: Opt-in, bounded, local metadata about workflow runs. It never
  records raw configs, policies, full reports, secrets, logs, environment, process data,
  runtime URLs, or tokens.

## Live Traffic Terms

- **Live connection**: A tracked TCP connection in `GET /api/connections` and the
  Live Connections view.
- **UDP session**: A tracked UDP flow. UDP has no connection, so Portier tracks sessions.

## Frozen Public Names

Do not rename these for cosmetic consistency:

- REST path `/api/forwards`.
- Rule fields `listenHost` and `targetHost`.
- Observed connection/session fields `clientAddress` and `targetAddress`.
- Runtime values `"node"` and `"go"`.
- Existing CLI command names.

## Writing Rules

- Prefer "forward rule" in prose; "rule" is fine when context is clear.
- Use "Go service" and "TypeScript server" when distinguishing implementations.
- Use "config apply" for plan-based apply and "config import" for import endpoints.
- Use "activity event" for `/api/activity`.
- Keep "advisory" and "warning" distinct.
- Use "live connection" for TCP and "UDP session" for UDP.
- Use "diagnose" for the per-rule action and "diagnostics export" for the bundle.
- Use "doctor" for graded diagnostic reports.
- Use "policy" for guardrails and "workflow" for ordered safe-operation definitions.

New public wording should use these terms or update this glossary when a genuinely new
concept is introduced.
