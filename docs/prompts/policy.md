# Portier Policy — AI Handoff Prompt

A reusable, copy-paste prompt for asking an AI assistant to help interpret a
Portier **policy check** report. **Portier itself never sends anything anywhere**
— there is no AI integration, no upload, and no telemetry. This is plain text
*you* paste into an AI assistant of your choice, along with output you generated
locally.

Use it with:

- `portier policy check --config <file> --policy <file> --json --explain` (recommended — includes machine-readable findings and explanations)
- or the human output of `portier policy check ... --explain`

A policy report describes whether a config **complies with an operator-defined
policy** (a set of guardrails). A violation means the config breaks a guardrail
*you or your team chose* — it is a **policy choice**, not necessarily a Portier
defect or a security incident.

## Before you paste — safety

- Policy reports may include **rule names, hosts, ports, and group names**
  (Portier configuration data). They do **not** contain secrets, tokens, or
  private keys by design — but review the content before sharing it with a
  third-party service.
- **Do not paste** secrets, tokens, API keys, private keys, environment-variable
  dumps, process lists, or unrelated logs. The policy tool never collects those,
  and the assistant does not need them.
- Pasting into a third-party AI service sends that data to them; that is your
  choice and happens outside Portier.

## Prompt

Copy everything in the block below, then paste your policy output where marked.

```
You are helping me interpret a Portier "policy check" report. Portier is a local
TCP/UDP port-forwarding manager. A policy is a small set of operator-defined
guardrails (e.g. requireGroup, allowLanExposure, allowPrivilegedPorts,
allowAutostart, forbidDuplicateBindings). The report lists findings; each finding
has a stable code (e.g. policy.lan_exposure_forbidden), a severity, and — with
--explain — a meaning and a suggested action.

Treat the pasted report as the single source of truth. Do not invent rules,
findings, or runtime state that are not present in the output. Prefer the JSON
output over human text if both are provided, and tell me if they disagree.

Important framing:
- A violation means the config breaks a guardrail the operator chose. It is a
  POLICY CHOICE, not necessarily a Portier defect or an active security problem.
- For each violation there are usually two valid resolutions: (a) change the
  rule to comply, or (b) relax the policy guardrail if the behavior is
  intentional. Present both, and say which is safer by default.
- "result": "passed" with a single policy.valid finding means the config
  complies with every enabled guardrail — say so plainly and stop.

Please give me:
1. A one-line verdict (compliant, or N violations).
2. The findings grouped by guardrail, in priority order (errors first), each with
   a plain-language explanation of why it was flagged.
3. For each violation, the two resolution options (change the rule vs. adjust the
   policy) and which you recommend by default.
4. A short risk note: separate changes that are SAFE TO DO NOW from changes that
   need ADMIN, NETWORK, OR SECURITY REVIEW (for example, exposing a port on the
   LAN, or binding a privileged port).
5. Anything ambiguous or missing that I should double-check in the config or
   policy file.

Do not ask me for secrets, tokens, keys, environment dumps, process lists, or
logs. Do not recommend mutating my config or policy beyond what the pasted
findings justify. Do not claim Portier enforces policies or fixes violations
automatically — it does not; this is a dry-run report.

Here is my Portier policy output:

<PASTE `portier policy check --json --explain` OUTPUT HERE>
```

## Notes

- This prompt is **docs-only**. There is no `--prompt` CLI helper; Portier does
  not contact any AI service.
- It is intentionally distinct from [`docs/prompts/doctor.md`](doctor.md): a
  doctor report diagnoses runtime/config state, while a policy report evaluates
  compliance with operator-chosen guardrails — the framing and the "change the
  rule vs. adjust the policy" choice are policy-specific.
