# portier-replay

`portier-replay` is a **separate, offline analysis tool beside the Portier CLI** —
not a `portier` subcommand. It reads existing Portier workflow artifacts and reports
what offline replay/analysis each saved artifact can support.

It is part of the v1.13 *Local Replay & Offline Analysis* work. This first slice is
a scaffold: input detection plus a deterministic **replay plan**.

## Safety boundary

The replay tool is strictly **offline and read-only**. It never:

- executes workflows
- contacts the Portier runtime
- reads the config/policy/baseline/report files an artifact refers to
- mutates runtime/config/history/report files
- applies or imports configs
- enforces policy
- schedules jobs
- runs shell commands
- uploads anything
- collects logs, environment, or process data

It analyzes **saved artifacts only**. For a support-report bundle directory it reads
**only** the bundle's own `manifest.json`.

## Supported inputs

Detection is based on JSON shape / manifest shape, **not the filename**:

| Source kind                | Produced by                                  |
| -------------------------- | -------------------------------------------- |
| `workflow-run-report`      | `portier workflow run --json --out`          |
| `workflow-plan-report`     | `portier workflow plan --json --out`         |
| `workflow-history-export`  | `portier workflow history export --out`      |
| `workflow-report-bundle`   | `portier workflow report --out` (a directory)|

## Usage

```text
portier-replay [--json] plan --from <file-or-dir> [--out <file>]
```

- `--from <path>` — a workflow run/plan report file, a history export file, or a
  support-report bundle **directory** (whose `manifest.json` is read).
- `--json` — emit the replay plan as JSON instead of human-readable text.
- `--out <file>` — also write the replay plan JSON to a file. Under `--json` the
  stdout bytes and the file bytes are identical.

### Example

```text
$ portier-replay plan --from run-report.json
Portier Replay Plan

Source: workflow-run-report
Workflow: policy-baseline-check
Result: failed

Available:
- Step timeline can be reconstructed from saved step records.
- Failed and skipped steps can be summarized from saved step records.
- Emitted codes can be looked up offline for explanations.

Unavailable:
- Replay analysis does not re-execute workflows.
- Replay analysis does not contact the runtime.
- Replay analysis does not re-read referenced config, policy, baseline, or report files.
- Replay analysis does not mutate inputs or enforce policy.

Summary:
- 3 available
- 4 unavailable
```

## Replay plan

A replay plan answers a single question: **"What offline replay/analysis can this
saved artifact support?"** It does not replay anything by re-executing it. The plan
is deterministic and lists, per artifact:

- **available** analyses (e.g. step timeline reconstruction, distributions,
  explanation lookup) — gated on the inputs the artifact actually contains, and
- **unavailable** analyses — always including the four the replay tool deliberately
  never performs: workflow re-execution, runtime probing, referenced-file reread,
  and mutation/enforcement.

The replay plan schema (`schemaVersion: 1`) is a **local tool schema** — it is not a
REST/API contract and is independent of the workflow artifact schemas it analyzes.

## Exit codes

- `0` — a replay plan was produced.
- `1` — an output-write or JSON-encode failure.
- `2` — a usage error, or an unreadable / malformed / unsupported input artifact.

## Build & test

```text
npm run build:replay      # builds tools/replay/build/portier-replay
npm run test:replay       # go test ./...
npm run validate:replay   # test + build
```

The tool is its own Go module (`portier/replay`) and depends only on the Go
standard library. It does not import `tools/cli` or any runtime client packages.
