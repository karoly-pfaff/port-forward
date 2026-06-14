# Portier Workflow — AI Handoff Prompt

A reusable, copy-paste prompt for asking an AI assistant to help interpret a
Portier **workflow report bundle**. **Portier itself never sends anything
anywhere** — there is no AI integration, no upload, and no telemetry. This is
plain text *you* paste into an AI assistant of your choice, along with files you
generated locally.

Generate the bundle with:

```
portier workflow run --file <workflow.json> --json --out run.json
portier workflow report --from run.json --out ./workflow-report
```

(You can also package a `workflow plan --json --out` report the same way.) The
bundle is a small local directory containing `manifest.json`, `summary.txt`,
`report.json`, and `explanations.json`. Paste the last three when asking for help.

A workflow report describes the outcome of a **read-only** workflow: which steps
passed, failed, or were skipped, and the codes that explain the failures. A failed
`policy.check`/`policy.review`/`policy.baseline.compare` reflects a **policy
choice** (an operator-defined guardrail or an accepted baseline), not necessarily
a Portier defect.

## Before you paste — safety

- The bundle may include **workflow/step names, policy finding codes, and
  explanation text**. It does **not** contain raw configs, secrets, tokens,
  private keys, environment data, process lists, or logs by design — but review
  the files before sharing them with a third-party service.
- **Do not paste** secrets, tokens, API keys, private keys, environment-variable
  dumps, process lists, or unrelated logs. The workflow report tool never collects
  those, and the assistant does not need them.
- Pasting into a third-party AI service sends that data to them; that is your
  choice and happens outside Portier.

## Prompt

Copy everything in the block below, then paste the bundle files where marked.

```
You are helping me interpret a Portier "workflow report" bundle. Portier is a
local TCP/UDP port-forwarding manager. A workflow is an ordered sequence of
existing safe, read-only operations (policy.check, policy.review,
policy.baseline.compare). The bundle was generated from an existing report — no
workflow was re-run, no runtime was contacted, and no referenced files were read.

I will paste three files:
- summary.txt: a human summary (source, workflow, result, steps, explained codes).
- report.json: the normalized report (each step's id/type/status/message and the
  explainable codes for that step).
- explanations.json: a map of code -> { title, meaning, action, severity } for the
  emitted codes (workflow.step.* for an invalid plan, workflow.run.* for failed or
  skipped run steps, policy.* for policy findings).

Treat these files as the single source of truth. Do not invent steps, codes, or
runtime/config state that are not present. Prefer report.json over summary.txt if
both are provided, and tell me if they disagree.

Important framing:
- A failed policy/review/baseline step reflects a policy choice or an accepted
  baseline, not necessarily a Portier defect.
- workflow.run.dependency_failed means a step was skipped because a step it depends
  on (via reportFrom) produced no report. workflow.run.runtime_unreachable means a
  runtime policy.check could not reach the runtime. workflow.run.input_failed means
  a referenced file (or the runtime config) could not be read or parsed.
- "result": "passed" means every step passed — say so plainly and stop.

Please give me:
1. A one-line verdict (passed, or N failed / M skipped).
2. The failed/skipped steps in order, each with a plain-language explanation of
   why it failed or was skipped (using explanations.json), and what the underlying
   codes mean.
3. For each failure, suggested SAFE next actions to investigate or resolve it
   (e.g. which file or policy guardrail to review). Separate changes that are SAFE
   TO DO NOW from changes that need ADMIN, NETWORK, OR SECURITY REVIEW.
4. Anything ambiguous or missing I should double-check in my workflow, config, or
   policy files.

Do not ask me for secrets, tokens, keys, environment dumps, process lists, or
logs. Do NOT invent specific config changes (exact ports, hosts, rules) and tell
me to apply them blindly — propose what to review and let me decide and edit the
files myself. Do not claim Portier enforces anything or fixes failures
automatically — workflow run is read-only and this is an after-the-fact report.

Here is my Portier workflow report:

--- summary.txt ---
<PASTE summary.txt HERE>

--- report.json ---
<PASTE report.json HERE>

--- explanations.json ---
<PASTE explanations.json HERE>
```

## Notes

- This prompt is **docs-only**. There is no `--prompt` CLI helper; Portier does
  not contact any AI service.
- It reuses the framing of [`prompts/policy.md`](policy.md) and
  [`prompts/doctor.md`](doctor.md): when a workflow failure is a **policy**
  finding, the "change the rule vs. adjust the policy" guidance from the policy
  prompt applies — point the assistant at the policy finding codes in
  `explanations.json`.
