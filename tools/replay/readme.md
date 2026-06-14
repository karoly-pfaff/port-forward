# replay

`replay` is a **separate, offline analysis tool beside the Portier CLI** —
not a `portier` subcommand. It reads existing Portier workflow artifacts and analyzes
them offline, without ever running a workflow or contacting the runtime.

It is part of the v1.13 *Local Replay & Offline Analysis* work and provides five
commands: `plan`, `analyze`, `timeline`, `compare`, and `explain`. The Portier CLI
creates/runs/exports/packages operational artifacts; `replay` analyzes those saved
artifacts after the fact.

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
**only** the bundle's own `manifest.json`, `report.json`, and `explanations.json` —
never any file referenced from inside an artifact. A missing or malformed bundle
`report.json` is tolerated (the manifest alone still yields a result).

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
replay [--json] plan     --from <file-or-dir> [--out <file>]
replay [--json] analyze  --from <file-or-dir> [--out <file>]
replay [--json] timeline --from <file-or-dir> [--out <file>]
replay [--json] compare  --left <file-or-dir> --right <file-or-dir> [--out <file>]
replay [--json] explain  --from <file-or-dir> [--out <file>]
replay [--json] explain  --code <code>
replay [--json] explain  --list
replay version          # or: replay --version
```

The version is exposed like the Portier CLI — `replay version` / `replay --version`
prints `Portier replay <version>` (the version constant lives in
`tools/replay/sources/version/version.go` and tracks the overall Portier release).

- `--from <path>` — a workflow run/plan report file, a history export file, or a
  support-report bundle **directory** (whose `manifest.json` — and, for `analyze`,
  `report.json`/`explanations.json` — are read; no other file). `explain` also
  accepts a replay analysis/timeline/compare JSON.
- `--left <path>` / `--right <path>` — the two artifacts for `compare` (same path
  forms as `--from`).
- `--code <code>` / `--list` — explain a single code, or list every known code
  (`explain` only; mutually exclusive with `--from`).
- `--json` — emit JSON instead of human-readable text.
- `--out <file>` — also write the JSON output to a file. Under `--json` the stdout
  bytes and the file bytes are identical.

### `plan` — capability preview

`plan` reports *what* offline analysis a saved artifact can support (it does not
run any analysis).

```text
$ replay plan --from run-report.json
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

### `analyze` — offline analysis

`analyze` produces deterministic analysis *from the artifact contents only* — step
or run tallies, an emitted-code distribution, findings, and plain-language
insights.

```text
$ replay analyze --from run-report.json
Portier Replay Analysis

Source: workflow-run-report
Workflow: policy-baseline-check
Result: failed

Step summary:
- 2 total
- 1 passed
- 1 failed
- 0 skipped

Codes:
- policy.lan_exposure_forbidden: 1

Findings:
- Workflow has failed steps.

Insights:
- Workflow failed because one or more steps failed.
```

For a history export `analyze` reports run tallies, a workflow distribution, a
code distribution (counting the **runs** that contain each code), a failed-run
shortlist, and the most recent run. The analysis schema (`schemaVersion: 1`) is a
**local tool schema**, separate from the replay-plan schema. Deterministic
ordering: codes and workflow counts by count descending then name/code ascending;
IDs ascending; insights in a stable order.

### `timeline` — ordered reconstruction

`timeline` reconstructs an ordered view of what the saved artifact says happened.
Each timeline brackets the artifact's own saved events with clearly-marked
**synthetic** lifecycle events (a `*-start` and a `*-result`/`*-end`). It never
infers timestamps the artifact does not contain — ordering is by saved order only.

```text
$ replay timeline --from run-report.json
Portier Replay Timeline

Source: workflow-run-report
Workflow: policy-baseline-check
Result: failed

1. workflow-start [started]
   Workflow replay timeline reconstructed from saved report.

2. check-config [passed] policy.check
   Exit code: 0

3. compare-baseline [failed] policy.baseline.compare
   Exit code: 1
   Codes: policy.lan_exposure_forbidden

4. workflow-result [failed]
   Workflow result: failed.

Summary:
- 4 events
- 1 passed
- 1 failed
- 0 skipped
- 2 synthetic
```

Per artifact: run reports build a step timeline; plan reports a validation
timeline (valid/invalid); history exports a run timeline in **newest-first** stored
order (each run event carries id, createdAt, workflow, result, per-run summary, and
codes); support bundles use the bundle's own `manifest.json`/`report.json`. Every
event carries a `synthetic` boolean and the summary reports a `synthetic` count, so
reconstruction markers are always distinguishable from saved events. The timeline
schema (`schemaVersion: 1`) is a **local tool schema**, separate from the plan and
analysis schemas.

### `compare` — offline diff of two artifacts

`compare` parses two saved artifacts and reports what changed between them using
only their offline contents.

```text
$ replay compare --left before.json --right after.json
Portier Replay Compare

Left:
- Source: workflow-run-report
- Workflow: policy-baseline-check
- Result: passed

Right:
- Source: workflow-run-report
- Workflow: policy-baseline-check
- Result: failed

Changes:
- Result changed from passed to failed.
- New emitted codes appeared.
  - policy.lan_exposure_forbidden

Insights:
- Result changed from passed to failed.
- New emitted codes appeared.
```

**Same-kind** comparisons produce a full diff: changed result/validity, step or run
count deltas, added/removed/unchanged code sets, changed failed/skipped/invalid
step sets, history workflow-distribution deltas, and changed failed-run sets.
**Mixed-kind** comparisons (e.g. a run report vs a history export) produce a
deterministic *limited* result — the two source kinds plus a clear
"detailed comparison is limited" marker — never a crash and never a false claim of
equivalence. Changes are emitted in a stable semantic order and set values are
sorted ascending. The compare schema (`schemaVersion: 1`) is a **local tool
schema**, separate from the plan, analysis, and timeline schemas.

### `explain` — offline code explanations

`explain` collects the emitted codes an artifact records and maps the known ones to
local offline explanations. Unknown codes are **preserved and marked unknown**,
never dropped.

```text
$ replay explain --from run-report.json
Portier Replay Explanations

Source: workflow-run-report
Codes: 2
Known: 1
Unknown: 1

policy.lan_exposure_forbidden [warning]
LAN exposure is forbidden
The policy does not allow rules listening on LAN-exposed interfaces.
Suggestion: Review the rule's listen host or use a policy that explicitly allows LAN exposure.

custom.future.code [unknown]
Unknown code
This code is not known by this replay tool version.
Suggestion: Use replay analyze or inspect the original artifact for surrounding context.
```

Codes are deduped and sorted ascending. An artifact with no codes is a success
(exit 0) with a clear notice. `explain` accepts the four workflow artifacts plus
replay analysis/timeline/compare JSON. The explanation registry is a **small local
registry** (workflow step/run + policy code families); it does not import the CLI
explain registry. `--code <code>` explains one code; `--list` lists every known
code. The explain schema (`schemaVersion: 1`) is a **local tool schema**, separate
from the plan, analysis, timeline, and compare schemas.

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

Every `replay` command uses the same exit codes:

- `0` — the requested output was produced (including when an artifact has no codes
  to explain, or two artifacts have no differences).
- `1` — an output-write or JSON-encode failure.
- `2` — a usage error, or an unreadable / malformed / unsupported input artifact.

There is no exit `3` — `replay` never contacts a runtime.

## Build & test

```text
npm run build:replay              # builds tools/replay/build/replay
npm run test:replay               # go test ./...
npm run validate:replay           # test + build
npm run validate:coverage:replay  # module-wide coverage gate (hard 95% minimum)
```

`tools/replay` has a **hard module-wide coverage gate of 95%**, enforced by
`scripts/validate-coverage.js` (component `replay`, run via
`npm run validate:coverage:replay`). Coverage is gathered module-wide by
`scripts/coverage-tools-replay.js` (the reporter, equivalent to
`go -C tools/replay test -coverpkg=./... ./...`, run as `npm run coverage:replay`),
and the gate is applied alongside the other component gates. It is a separate gate
from the CLI coverage gate and does not depend on the CLI module; existing CLI
coverage gates are unchanged.

The tool is its own Go module (`portier/replay`) and depends only on the Go
standard library. It does not import `tools/cli` or any runtime client packages.

## Structure

```text
tools/replay/sources/
  main.go        # thin entry point: os.Exit(commands.Run(...))
  commands/      # CLI dispatch, argument parsing, command runners
  core/          # artifact detection + plan/analysis/timeline/compare/explain
                 #   models, builders, and renderers
```

`commands` depends on `core`; `core` never depends on `commands` (domain logic
stays free of argument parsing). Neither package depends on `tools/cli`.
