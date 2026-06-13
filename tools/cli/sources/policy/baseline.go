package policy

// Policy baseline snapshots: a dry-run workflow for saving an accepted set of
// policy findings and later comparing a fresh policy report against that
// baseline (which findings are new / resolved / unchanged). Fully offline and
// deterministic — it operates only on policy report JSON files produced by
// `policy check`/`policy review` (--json / --out); it never contacts the runtime,
// stores no raw config or runtime host data, and mutates nothing. A baseline is
// an accepted snapshot of findings, NOT a config copy.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// baselineSchemaVersion is the only supported baseline file schema version.
const baselineSchemaVersion = 1

// BaselineFinding is the compact, accepted snapshot of one policy finding. It
// carries a stable fingerprint plus enough human context to read the baseline —
// no details, no explanations, no config data.
type BaselineFinding struct {
	Fingerprint string   `json:"fingerprint"`
	Code        string   `json:"code"`
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`
	Message     string   `json:"message"`
}

// Baseline is an accepted snapshot of policy findings at a point in time. It is a
// small deterministic JSON document — not a config copy, no secrets, no runtime
// host data.
type Baseline struct {
	SchemaVersion int               `json:"schemaVersion"`
	CreatedAt     string            `json:"createdAt"`
	Source        string            `json:"source,omitempty"`
	Result        string            `json:"result"`
	Findings      []BaselineFinding `json:"findings"`
}

// ReportSnapshot is the subset of a policy report (from `policy check` or
// `policy review`) that baselines care about: the result, optional source, and
// findings. Other fields (review, summary, explanations) are ignored.
type ReportSnapshot struct {
	Source   string    `json:"source"`
	Result   string    `json:"result"`
	Findings []Finding `json:"findings"`
}

// CompareSummary counts the comparison outcome.
type CompareSummary struct {
	New       int `json:"new"`
	Resolved  int `json:"resolved"`
	Unchanged int `json:"unchanged"`
}

// CompareReport is the deterministic result of comparing a fresh report against
// a baseline. Result is "failed" when any NEW findings exist (resolved-only
// changes do not fail), otherwise "passed".
type CompareReport struct {
	Summary   CompareSummary    `json:"summary"`
	Result    string            `json:"result"`
	New       []BaselineFinding `json:"new"`
	Resolved  []BaselineFinding `json:"resolved"`
	Unchanged []BaselineFinding `json:"unchanged"`
}

// ParseReportSnapshot decodes a policy report JSON file (from `policy check` or
// `policy review`, --json or --out) into the subset baselines use. It rejects
// malformed JSON.
func ParseReportSnapshot(data []byte) (ReportSnapshot, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return ReportSnapshot{}, fmt.Errorf("policy report is empty")
	}
	var snap ReportSnapshot
	if err := json.Unmarshal(trimmed, &snap); err != nil {
		return ReportSnapshot{}, fmt.Errorf("not a valid policy report: %w", err)
	}
	return snap, nil
}

// ParseBaseline decodes and validates a baseline JSON file.
func ParseBaseline(data []byte) (Baseline, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Baseline{}, fmt.Errorf("baseline file is empty")
	}
	var b Baseline
	if err := json.Unmarshal(trimmed, &b); err != nil {
		return Baseline{}, fmt.Errorf("not a valid baseline file: %w", err)
	}
	if b.SchemaVersion != baselineSchemaVersion {
		return Baseline{}, fmt.Errorf("unsupported baseline schemaVersion %d (expected %d)", b.SchemaVersion, baselineSchemaVersion)
	}
	return b, nil
}

// trackedFindings returns the findings worth baselining: every finding except the
// "policy.valid" no-violations marker (which is mutually exclusive with real
// findings and must never count as new/resolved/unchanged).
func trackedFindings(findings []Finding) []Finding {
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if f.Code != codeValid {
			out = append(out, f)
		}
	}
	return out
}

// snapshotFinding builds the compact baseline snapshot (with fingerprint) for a
// finding.
func snapshotFinding(f Finding) BaselineFinding {
	return BaselineFinding{
		Fingerprint: Fingerprint(f),
		Code:        f.Code,
		Severity:    f.Severity,
		Title:       f.Title,
		Message:     f.Message,
	}
}

// BuildBaseline builds a baseline from a report snapshot, stamping createdAt from
// now (UTC, RFC3339). The "policy.valid" marker is excluded.
func BuildBaseline(snap ReportSnapshot, now time.Time) Baseline {
	tracked := trackedFindings(snap.Findings)
	findings := make([]BaselineFinding, 0, len(tracked))
	for _, f := range tracked {
		findings = append(findings, snapshotFinding(f))
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Fingerprint < findings[j].Fingerprint })
	return Baseline{
		SchemaVersion: baselineSchemaVersion,
		CreatedAt:     now.UTC().Format(time.RFC3339),
		Source:        snap.Source,
		Result:        snap.Result,
		Findings:      findings,
	}
}

// Compare compares a fresh report snapshot against a baseline and classifies each
// finding as new (in the report, not the baseline), resolved (in the baseline,
// not the report), or unchanged (in both), keyed by fingerprint. Result is
// "failed" iff there are new findings. Deterministic: every list is sorted by
// fingerprint.
func Compare(baseline Baseline, snap ReportSnapshot) CompareReport {
	baselineByFP := map[string]BaselineFinding{}
	for _, bf := range baseline.Findings {
		baselineByFP[bf.Fingerprint] = bf
	}

	currentByFP := map[string]BaselineFinding{}
	for _, f := range trackedFindings(snap.Findings) {
		bf := snapshotFinding(f)
		currentByFP[bf.Fingerprint] = bf
	}

	var newFindings, unchanged, resolved []BaselineFinding
	for fp, bf := range currentByFP {
		if _, ok := baselineByFP[fp]; ok {
			unchanged = append(unchanged, bf)
		} else {
			newFindings = append(newFindings, bf)
		}
	}
	for fp, bf := range baselineByFP {
		if _, ok := currentByFP[fp]; !ok {
			resolved = append(resolved, bf)
		}
	}

	sortFindings(newFindings)
	sortFindings(resolved)
	sortFindings(unchanged)

	result := "passed"
	if len(newFindings) > 0 {
		result = "failed"
	}
	return CompareReport{
		Summary:   CompareSummary{New: len(newFindings), Resolved: len(resolved), Unchanged: len(unchanged)},
		Result:    result,
		New:       newFindings,
		Resolved:  resolved,
		Unchanged: unchanged,
	}
}

func sortFindings(fs []BaselineFinding) {
	sort.Slice(fs, func(i, j int) bool { return fs[i].Fingerprint < fs[j].Fingerprint })
}

// CompareExitCode returns 1 when the comparison found any NEW findings, else 0.
// Resolved-only changes do not fail.
func CompareExitCode(r CompareReport) int {
	if r.Summary.New > 0 {
		return 1
	}
	return 0
}

// PrintCompareHuman renders a comparison in deterministic human-readable form.
func PrintCompareHuman(r CompareReport, w io.Writer) {
	fmt.Fprintln(w, "Portier Policy Baseline Compare")
	fmt.Fprintln(w)
	printCompareSection(w, "New findings", r.New)
	fmt.Fprintln(w)
	printCompareSection(w, "Resolved findings", r.Resolved)
	fmt.Fprintln(w)
	printCompareSection(w, "Unchanged findings", r.Unchanged)
	fmt.Fprintf(w, "\nResult: %s\n", r.Result)
}

func printCompareSection(w io.Writer, heading string, findings []BaselineFinding) {
	fmt.Fprintf(w, "%s:\n", heading)
	if len(findings) == 0 {
		fmt.Fprintln(w, "- none")
		return
	}
	for _, f := range findings {
		fmt.Fprintf(w, "- %s: %s\n", f.Code, f.Message)
	}
}

// Fingerprint computes a deterministic, stable, readable fingerprint for a policy
// finding so it can be matched across reports. For the known rule-scoped findings
// it uses code + rule name/protocol/listen host/port; for duplicate bindings it
// uses code + binding protocol/host/port + the sorted affected rule names. When
// the expected details are absent it falls back to code + message. It reads the
// generic detail map (findings parsed from report JSON), so it is independent of
// the source (offline file vs runtime) and never depends on volatile rule IDs.
func Fingerprint(f Finding) string {
	switch f.Code {
	case codeGroupRequired, codeLanExposureForbidden, codePrivilegedPortForbidden, codeAutostartForbidden:
		if rule := nestedMap(f.Details, "rule"); rule != nil {
			return strings.Join([]string{
				f.Code,
				detailString(rule, "name"),
				detailString(rule, "protocol"),
				detailString(rule, "listenHost"),
				detailString(rule, "listenPort"),
			}, "|")
		}
	case codeDuplicateBindingForbidden:
		if binding := nestedMap(f.Details, "binding"); binding != nil {
			return strings.Join([]string{
				f.Code,
				detailString(binding, "protocol"),
				detailString(binding, "listenHost"),
				detailString(binding, "listenPort"),
				strings.Join(bindingRuleNames(binding), ","),
			}, "|")
		}
	}
	return f.Code + "|" + f.Message
}

// nestedMap returns details[key] as a map, or nil when absent / not an object.
func nestedMap(details map[string]any, key string) map[string]any {
	if details == nil {
		return nil
	}
	if m, ok := details[key].(map[string]any); ok {
		return m
	}
	return nil
}

// detailString reads a string/number/bool value from a generic detail map as a
// stable string (JSON numbers decode to float64; integers print without a
// decimal point).
func detailString(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return ""
	}
}

// bindingRuleNames returns the sorted rule names (falling back to id) of a
// duplicate-binding detail's "rules" array, for a deterministic fingerprint.
func bindingRuleNames(binding map[string]any) []string {
	raw, ok := binding["rules"].([]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(raw))
	for _, item := range raw {
		rule, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := detailString(rule, "name")
		if name == "" {
			name = detailString(rule, "id")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
