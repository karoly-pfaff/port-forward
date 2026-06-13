package policy

// White-box tests for the baseline model: deterministic finding fingerprints
// (rule-scoped, duplicate-binding, and fallback), tracked-finding filtering,
// baseline building (injected clock), and the new/resolved/unchanged comparison.

import (
	"encoding/json"
	"testing"
	"time"

	"portier/cli/sources/config"
)

// snapshotFromEval evaluates rules against a policy, marshals the report the way
// `policy check` does, and parses it back into a ReportSnapshot — so finding
// details are the generic map form the fingerprinter sees in practice.
func snapshotFromEval(t *testing.T, rules []config.Rule, pol Policy) ReportSnapshot {
	t.Helper()
	report := Evaluate(rules, pol)
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshaling report: %v", err)
	}
	snap, err := ParseReportSnapshot(data)
	if err != nil {
		t.Fatalf("parsing snapshot: %v", err)
	}
	return snap
}

func TestFingerprint_RuleScopedStable(t *testing.T) {
	rules := []config.Rule{
		{Name: "Admin UI", Protocol: "tcp", ListenHost: "0.0.0.0", ListenPort: 48080, TargetHost: "h", TargetPort: 1, Enabled: false, Group: "admin"},
	}
	pol, _ := Parse([]byte(`{"schemaVersion":1,"rules":{"allowLanExposure":false}}`))
	snap := snapshotFromEval(t, rules, pol)
	if len(snap.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(snap.Findings))
	}
	got := Fingerprint(snap.Findings[0])
	want := "policy.lan_exposure_forbidden|Admin UI|tcp|0.0.0.0|48080"
	if got != want {
		t.Errorf("fingerprint = %q, want %q", got, want)
	}
	// Deterministic: recomputing yields the same value.
	if again := Fingerprint(snap.Findings[0]); again != got {
		t.Errorf("fingerprint not deterministic: %q vs %q", got, again)
	}
}

func TestFingerprint_DuplicateBindingDeterministic(t *testing.T) {
	rules := []config.Rule{
		{Name: "Bravo", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000, TargetHost: "h", TargetPort: 1, Enabled: false, Group: "g"},
		{Name: "Alpha", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000, TargetHost: "h", TargetPort: 2, Enabled: false, Group: "g"},
	}
	pol, _ := Parse([]byte(`{"schemaVersion":1,"rules":{"forbidDuplicateBindings":true}}`))
	snap := snapshotFromEval(t, rules, pol)
	// Find the duplicate-binding finding.
	var dup Finding
	for _, f := range snap.Findings {
		if f.Code == codeDuplicateBindingForbidden {
			dup = f
		}
	}
	if dup.Code == "" {
		t.Fatalf("no duplicate-binding finding in %v", snap.Findings)
	}
	got := Fingerprint(dup)
	// Rule names are sorted, so order in the config does not change the fingerprint.
	want := "policy.duplicate_binding_forbidden|tcp|127.0.0.1|48000|Alpha,Bravo"
	if got != want {
		t.Errorf("fingerprint = %q, want %q", got, want)
	}
}

func TestFingerprint_FallbackToMessage(t *testing.T) {
	f := Finding{Code: "policy.lan_exposure_forbidden", Message: "msg without details"}
	got := Fingerprint(f)
	want := "policy.lan_exposure_forbidden|msg without details"
	if got != want {
		t.Errorf("fallback fingerprint = %q, want %q", got, want)
	}
}

func TestTrackedFindings_ExcludesValid(t *testing.T) {
	findings := []Finding{
		{Code: codeValid, Severity: Info},
		{Code: codeLanExposureForbidden, Severity: Error},
	}
	tracked := trackedFindings(findings)
	if len(tracked) != 1 || tracked[0].Code != codeLanExposureForbidden {
		t.Errorf("trackedFindings = %v, want only the violation", tracked)
	}
}

func TestBuildBaseline_DeterministicClockAndExclusion(t *testing.T) {
	snap := ReportSnapshot{
		Source: "runtime",
		Result: "failed",
		Findings: []Finding{
			{Code: codeValid, Severity: Info, Message: "ok"},
			{Code: "policy.lan_exposure_forbidden", Severity: Error, Message: "b"},
			{Code: "policy.autostart_forbidden", Severity: Error, Message: "a"},
		},
	}
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	b := BuildBaseline(snap, now)
	if b.SchemaVersion != baselineSchemaVersion {
		t.Errorf("schemaVersion = %d, want %d", b.SchemaVersion, baselineSchemaVersion)
	}
	if b.CreatedAt != "2026-06-13T12:00:00Z" {
		t.Errorf("createdAt = %q, want 2026-06-13T12:00:00Z", b.CreatedAt)
	}
	if b.Source != "runtime" || b.Result != "failed" {
		t.Errorf("source/result = %q/%q", b.Source, b.Result)
	}
	if len(b.Findings) != 2 {
		t.Fatalf("expected 2 findings (valid excluded), got %d", len(b.Findings))
	}
	// Sorted by fingerprint: "policy.autostart...|a" < "policy.lan...|b".
	if b.Findings[0].Code != "policy.autostart_forbidden" {
		t.Errorf("findings not sorted by fingerprint: %v", b.Findings)
	}
}

func TestBuildBaseline_CreatedAtIsUTC(t *testing.T) {
	loc := time.FixedZone("X", 5*3600)
	now := time.Date(2026, 6, 13, 17, 0, 0, 0, loc) // 12:00 UTC
	b := BuildBaseline(ReportSnapshot{Result: "passed"}, now)
	if b.CreatedAt != "2026-06-13T12:00:00Z" {
		t.Errorf("createdAt = %q, want UTC 2026-06-13T12:00:00Z", b.CreatedAt)
	}
}

// finding builds a detail-less finding whose fallback fingerprint is code|message.
func finding(code, message string) Finding {
	return Finding{Code: code, Severity: Error, Title: code, Message: message}
}

func TestCompare_NewResolvedUnchanged(t *testing.T) {
	base := BuildBaseline(ReportSnapshot{Result: "failed", Findings: []Finding{
		finding("policy.lan_exposure_forbidden", "lan"),
		finding("policy.autostart_forbidden", "auto"),
	}}, time.Now())

	// Current: lan unchanged, auto resolved, privileged new.
	snap := ReportSnapshot{Result: "failed", Findings: []Finding{
		finding("policy.lan_exposure_forbidden", "lan"),
		finding("policy.privileged_port_forbidden", "priv"),
	}}
	r := Compare(base, snap)
	if r.Summary.New != 1 || r.Summary.Resolved != 1 || r.Summary.Unchanged != 1 {
		t.Errorf("summary = %+v, want new1 resolved1 unchanged1", r.Summary)
	}
	if r.Result != "failed" || CompareExitCode(r) != 1 {
		t.Errorf("result/exit = %q/%d, want failed/1", r.Result, CompareExitCode(r))
	}
	if len(r.New) != 1 || r.New[0].Code != "policy.privileged_port_forbidden" {
		t.Errorf("new = %v", r.New)
	}
	if len(r.Resolved) != 1 || r.Resolved[0].Code != "policy.autostart_forbidden" {
		t.Errorf("resolved = %v", r.Resolved)
	}
}

func TestCompare_NoNewPasses(t *testing.T) {
	base := BuildBaseline(ReportSnapshot{Result: "failed", Findings: []Finding{
		finding("policy.lan_exposure_forbidden", "lan"),
	}}, time.Now())
	// Current resolves the only finding (a passing report carries policy.valid).
	snap := ReportSnapshot{Result: "passed", Findings: []Finding{
		{Code: codeValid, Severity: Info, Message: "ok"},
	}}
	r := Compare(base, snap)
	if r.Summary.New != 0 || r.Summary.Resolved != 1 {
		t.Errorf("summary = %+v, want new0 resolved1", r.Summary)
	}
	if r.Result != "passed" || CompareExitCode(r) != 0 {
		t.Errorf("resolved-only must pass: result %q exit %d", r.Result, CompareExitCode(r))
	}
}

func TestParseBaseline_RejectsBadSchema(t *testing.T) {
	if _, err := ParseBaseline([]byte(`{"schemaVersion":99}`)); err == nil {
		t.Error("expected error for unsupported schemaVersion")
	}
	if _, err := ParseBaseline([]byte(`not json`)); err == nil {
		t.Error("expected error for malformed baseline")
	}
	if _, err := ParseBaseline(nil); err == nil {
		t.Error("expected error for empty baseline")
	}
}

func TestParseReportSnapshot_RejectsMalformed(t *testing.T) {
	if _, err := ParseReportSnapshot([]byte(`not json`)); err == nil {
		t.Error("expected error for malformed report")
	}
	if _, err := ParseReportSnapshot(nil); err == nil {
		t.Error("expected error for empty report")
	}
}

func TestCompare_SortsMultipleNew(t *testing.T) {
	// Empty baseline + two findings → both are new and must be sorted by
	// fingerprint (exercises the sort comparator).
	base := BuildBaseline(ReportSnapshot{Result: "passed"}, time.Now())
	snap := ReportSnapshot{Result: "failed", Findings: []Finding{
		finding("policy.privileged_port_forbidden", "z"),
		finding("policy.autostart_forbidden", "a"),
	}}
	r := Compare(base, snap)
	if r.Summary.New != 2 {
		t.Fatalf("new = %d, want 2", r.Summary.New)
	}
	// "policy.autostart...|a" sorts before "policy.privileged...|z".
	if r.New[0].Code != "policy.autostart_forbidden" || r.New[1].Code != "policy.privileged_port_forbidden" {
		t.Errorf("new findings not sorted by fingerprint: %v", r.New)
	}
}

func TestDetailString(t *testing.T) {
	m := map[string]any{"s": "x", "n": float64(48080), "b": true}
	if got := detailString(m, "s"); got != "x" {
		t.Errorf("string = %q", got)
	}
	if got := detailString(m, "n"); got != "48080" {
		t.Errorf("number = %q, want 48080", got)
	}
	if got := detailString(m, "b"); got != "true" {
		t.Errorf("bool = %q, want true", got)
	}
	if got := detailString(m, "missing"); got != "" {
		t.Errorf("missing = %q, want empty", got)
	}
}

func TestNestedMap(t *testing.T) {
	if nestedMap(nil, "k") != nil {
		t.Error("nil details should yield nil")
	}
	if nestedMap(map[string]any{"k": "notamap"}, "k") != nil {
		t.Error("non-map value should yield nil")
	}
	if nestedMap(map[string]any{}, "missing") != nil {
		t.Error("missing key should yield nil")
	}
	if m := nestedMap(map[string]any{"k": map[string]any{"a": "b"}}, "k"); m == nil {
		t.Error("map value should be returned")
	}
}

func TestBindingRuleNames_Fallbacks(t *testing.T) {
	// No rules array → nil.
	if names := bindingRuleNames(map[string]any{}); names != nil {
		t.Errorf("missing rules = %v, want nil", names)
	}
	// Non-map item skipped; name falls back to id; result sorted.
	binding := map[string]any{"rules": []any{
		"not-a-map",
		map[string]any{"id": "id-2"},    // no name → id
		map[string]any{"name": "Alpha"}, // name
	}}
	got := bindingRuleNames(binding)
	if len(got) != 2 || got[0] != "Alpha" || got[1] != "id-2" {
		t.Errorf("names = %v, want [Alpha id-2]", got)
	}
}
