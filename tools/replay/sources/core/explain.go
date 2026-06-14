package core

// Offline explanation of the emitted codes found in a saved Portier artifact.
// `explain` collects the codes an artifact records, maps the known ones to local
// offline explanations, and preserves unknown codes (marked unknown rather than
// dropped).
//
// Like the rest of the replay tool it is strictly offline and read-only: it parses
// only the supplied `--from` JSON file (or, for a support-report bundle directory,
// the bundle's own manifest.json / report.json) and NEVER executes workflows,
// contacts the runtime, reads the config/policy/baseline/report files an artifact
// refers to, mutates inputs, applies/imports, enforces policy, runs shell commands,
// uploads, or collects logs/environment/process data. The explanation registry is a
// small LOCAL registry — it does not import the CLI explain registry or any runtime
// package.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// ExplainSchemaVersion is the schema version of the replay explanation report. It
// is a local tool schema (not a REST/API contract), distinct from the plan/
// analysis/timeline/compare schemas.
const ExplainSchemaVersion = 1

// Source labels for replay tool output artifacts (workflow artifacts reuse the
// SourceWorkflow* constants).
const (
	SourceReplayAnalysis = "replay-analysis"
	SourceReplayTimeline = "replay-timeline"
	SourceReplayCompare  = "replay-compare"
	sourceCode           = "code"
	sourceRegistry       = "registry"
)

// Explanation severities.
const (
	explainInfo    = "info"
	explainWarning = "warning"
	explainError   = "error"
	explainUnknown = "unknown"
)

// Explanation is one code's offline explanation. Known is false for codes this
// replay tool version does not recognize.
type Explanation struct {
	Code       string `json:"code"`
	Known      bool   `json:"known"`
	Title      string `json:"title"`
	Severity   string `json:"severity"`
	Meaning    string `json:"meaning"`
	Suggestion string `json:"suggestion"`
}

// Explain is the deterministic explanation report for one artifact (or for a single
// `--code`, or the whole registry via `--list`).
type Explain struct {
	SchemaVersion int           `json:"schemaVersion"`
	Source        string        `json:"source"`
	CodeCount     int           `json:"codeCount"`
	Known         int           `json:"known"`
	Unknown       int           `json:"unknown"`
	Explanations  []Explanation `json:"explanations"`
}

// ExplainInput resolves an artifact path (a file or a bundle directory), collects
// its emitted codes, and builds their explanations. It reads only that artifact
// (and, for a bundle directory, only the bundle's own manifest/report files).
func ExplainInput(path string) (Explain, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Explain{}, err
	}
	if info.IsDir() {
		f, err := bundleFacts(path)
		if err != nil {
			return Explain{}, err
		}
		return BuildExplain(f.Source, f.Codes), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Explain{}, err
	}
	return ExplainJSON(data)
}

// ExplainJSON collects codes from a single-file artifact (a workflow artifact or a
// replay tool output) and builds their explanations. It is pure so it is
// independently testable.
func ExplainJSON(data []byte) (Explain, error) {
	source, codes, err := explainCodesFromJSON(data)
	if err != nil {
		return Explain{}, err
	}
	return BuildExplain(source, codes), nil
}

// ExplainCode builds a one-code explanation report (for `--code`).
func ExplainCode(code string) Explain {
	return BuildExplain(sourceCode, []string{code})
}

// ExplainRegistry builds a report listing every known code (for `--list`).
func ExplainRegistry() Explain {
	return BuildExplain(sourceRegistry, knownCodes())
}

// BuildExplain builds the explanation report for a (deduped, sorted) code list.
func BuildExplain(source string, codes []string) Explain {
	e := Explain{
		SchemaVersion: ExplainSchemaVersion,
		Source:        source,
		Explanations:  []Explanation{},
	}
	for _, code := range codes {
		if ex, ok := explainOne(code); ok {
			e.Explanations = append(e.Explanations, ex)
			e.Known++
		} else {
			e.Explanations = append(e.Explanations, unknownExplanation(code))
			e.Unknown++
		}
	}
	e.CodeCount = len(e.Explanations)
	return e
}

// explainCodesFromJSON detects a single-file artifact (a workflow run/plan report or
// history export, or a replay analysis/timeline/compare output) and returns its
// source label plus the deduped, sorted emitted codes.
func explainCodesFromJSON(data []byte) (string, []string, error) {
	// A workflow artifact (reuses detection + the compare facts extractor).
	if f, err := factsFromJSON(data); err == nil {
		return f.Source, f.Codes, nil
	}
	// A replay tool output artifact (analysis/timeline/compare JSON).
	if source, codes, ok := replayOutputCodes(data); ok {
		return source, codes, nil
	}
	return "", nil, fmt.Errorf("unsupported artifact for explain")
}

// replayProbe is the lenient shape used to detect a replay tool output and pull its
// codes. Each schema populates only its own fields.
type replayProbe struct {
	Source  string          `json:"source"`  // analysis/timeline
	Summary json.RawMessage `json:"summary"` // analysis (and others)
	Codes   []struct {
		Code string `json:"code"`
	} `json:"codes"` // analysis: [{code,count}]
	Events []struct {
		Codes []string `json:"codes"`
	} `json:"events"` // timeline
	Left  json.RawMessage      `json:"left"`  // compare
	Right json.RawMessage      `json:"right"` // compare
	Sets  map[string]replaySet `json:"sets"`  // compare
}

type replaySet struct {
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Unchanged []string `json:"unchanged"`
}

// replayOutputCodes detects a replay analysis/timeline/compare JSON and returns its
// source label + deduped/sorted codes. ok is false when the data is not a
// recognizable replay output.
func replayOutputCodes(data []byte) (string, []string, bool) {
	var p replayProbe
	if json.Unmarshal(data, &p) != nil {
		return "", nil, false
	}
	switch {
	case len(p.Left) > 0 && len(p.Right) > 0:
		set := p.Sets["codes"]
		codes := append(append(append([]string{}, set.Added...), set.Removed...), set.Unchanged...)
		return SourceReplayCompare, sortedDedupeStrings(codes), true
	case len(p.Events) > 0:
		var codes []string
		for _, ev := range p.Events {
			codes = append(codes, ev.Codes...)
		}
		return SourceReplayTimeline, sortedDedupeStrings(codes), true
	case p.Source != "" && len(p.Summary) > 0:
		var codes []string
		for _, c := range p.Codes {
			if c.Code != "" {
				codes = append(codes, c.Code)
			}
		}
		return SourceReplayAnalysis, sortedDedupeStrings(codes), true
	}
	return "", nil, false
}

// sortedDedupeStrings returns the unique values of in, sorted ascending.
func sortedDedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// unknownExplanation is the explanation for a code this replay tool version does not
// recognize. The code is preserved, never dropped.
func unknownExplanation(code string) Explanation {
	return Explanation{
		Code:       code,
		Known:      false,
		Title:      "Unknown code",
		Severity:   explainUnknown,
		Meaning:    "This code is not known by this replay tool version.",
		Suggestion: "Use replay analyze or inspect the original artifact for surrounding context.",
	}
}

// explainOne returns the registry explanation for a code, if known.
func explainOne(code string) (Explanation, bool) {
	ex, ok := replayExplanations[code]
	return ex, ok
}

// knownCodes returns every known code, sorted ascending.
func knownCodes() []string {
	out := make([]string, 0, len(replayExplanations))
	for code := range replayExplanations {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

// replayExplanations is the local offline explanation registry. It covers the
// stable code families emitted in Portier workflow artifacts: workflow step
// validation (`workflow.step.*`), workflow run outcomes (`workflow.run.*`), and
// policy findings (`policy.*`). Doctor/runtime codes are out of scope (they never
// appear in workflow artifacts) and are reported as unknown if encountered.
var replayExplanations = map[string]Explanation{
	// workflow.step.* — plan validation outcomes.
	"workflow.step.valid":                      {Title: "Step is valid", Severity: explainInfo, Meaning: "The workflow step passed schema validation.", Suggestion: "No action needed for this step."},
	"workflow.step.duplicate_id":               {Title: "Duplicate step id", Severity: explainError, Meaning: "Two workflow steps share the same id.", Suggestion: "Give each step a unique id in the workflow file."},
	"workflow.step.unknown_type":               {Title: "Unknown step type", Severity: explainError, Meaning: "The step type is not a supported workflow operation.", Suggestion: "Use a supported step type (policy.check, policy.review, policy.baseline.compare)."},
	"workflow.step.missing_id":                 {Title: "Missing step id", Severity: explainError, Meaning: "The workflow step has no id.", Suggestion: "Add a unique id to the step in the workflow file."},
	"workflow.step.missing_type":               {Title: "Missing step type", Severity: explainError, Meaning: "The workflow step has no type.", Suggestion: "Add a supported type to the step in the workflow file."},
	"workflow.step.missing_config_source":      {Title: "Missing config source", Severity: explainError, Meaning: "A policy.check step needs exactly one of config or runtime.", Suggestion: "Set either config or runtime on the step."},
	"workflow.step.conflicting_config_sources": {Title: "Conflicting config sources", Severity: explainError, Meaning: "A policy.check step set both config and runtime.", Suggestion: "Set only one of config or runtime on the step."},
	"workflow.step.missing_policy":             {Title: "Missing policy", Severity: explainError, Meaning: "The step requires a policy file.", Suggestion: "Add a policy path to the step."},
	"workflow.step.missing_current":            {Title: "Missing current config", Severity: explainError, Meaning: "A policy.review step requires a current config.", Suggestion: "Add a current config path to the step."},
	"workflow.step.missing_candidate":          {Title: "Missing candidate config", Severity: explainError, Meaning: "A policy.review step requires a candidate config.", Suggestion: "Add a candidate config path to the step."},
	"workflow.step.missing_baseline":           {Title: "Missing baseline", Severity: explainError, Meaning: "A policy.baseline.compare step requires a baseline.", Suggestion: "Add a baseline path to the step."},
	"workflow.step.missing_report_source":      {Title: "Missing report source", Severity: explainError, Meaning: "A baseline.compare step needs exactly one of report or reportFrom.", Suggestion: "Set either report or reportFrom on the step."},
	"workflow.step.conflicting_report_sources": {Title: "Conflicting report sources", Severity: explainError, Meaning: "A baseline.compare step set both report and reportFrom.", Suggestion: "Set only one of report or reportFrom on the step."},
	"workflow.step.unknown_report_from":        {Title: "Unknown reportFrom", Severity: explainError, Meaning: "reportFrom does not reference a known earlier step.", Suggestion: "Point reportFrom at the id of an earlier step that produces a report."},
	"workflow.step.future_report_from":         {Title: "Forward reportFrom", Severity: explainError, Meaning: "reportFrom references a later step.", Suggestion: "Reorder the workflow so the referenced step comes first."},

	// workflow.run.* — run outcomes.
	"workflow.run.dependency_failed":   {Title: "Dependency failed", Severity: explainError, Meaning: "A step was skipped because a step it depended on produced no report.", Suggestion: "Inspect the saved report for the failed upstream step. Replay will not rerun it."},
	"workflow.run.runtime_unreachable": {Title: "Runtime unreachable", Severity: explainError, Meaning: "A runtime policy check could not reach the Portier runtime.", Suggestion: "Confirm the runtime was reachable when the run was captured. Replay never contacts the runtime."},
	"workflow.run.input_failed":        {Title: "Workflow input failed", Severity: explainError, Meaning: "The workflow run could not read or process an expected input artifact.", Suggestion: "Inspect the saved workflow report and referenced step message. Replay will not reread referenced files."},

	// policy.* — policy findings.
	"policy.valid":                       {Title: "Policy satisfied", Severity: explainInfo, Meaning: "The configuration satisfied the policy.", Suggestion: "No action needed."},
	"policy.group_required":              {Title: "Group required", Severity: explainWarning, Meaning: "The policy requires every rule to declare a group.", Suggestion: "Add a group to the rule, or use a policy that does not require groups."},
	"policy.lan_exposure_forbidden":      {Title: "LAN exposure is forbidden", Severity: explainWarning, Meaning: "The policy does not allow rules listening on LAN-exposed interfaces.", Suggestion: "Review the rule's listen host or use a policy that explicitly allows LAN exposure."},
	"policy.privileged_port_forbidden":   {Title: "Privileged port forbidden", Severity: explainWarning, Meaning: "The policy does not allow rules listening on privileged ports (below 1024).", Suggestion: "Use a non-privileged listen port or a policy that allows privileged ports."},
	"policy.autostart_forbidden":         {Title: "Autostart forbidden", Severity: explainWarning, Meaning: "The policy does not allow rules with autostart enabled.", Suggestion: "Disable autostart on the rule or use a policy that allows it."},
	"policy.duplicate_binding_forbidden": {Title: "Duplicate binding forbidden", Severity: explainWarning, Meaning: "The policy forbids duplicate listen bindings (protocol + host + port).", Suggestion: "Resolve the duplicate listen binding between the rules."},
}

func init() {
	// Keep each registry entry's Code field in sync with its map key.
	for code, ex := range replayExplanations {
		ex.Code = code
		ex.Known = true
		replayExplanations[code] = ex
	}
}
