package workflow

// White-box tests for the workflow package internals (v1.11 Slice 1): the
// schema parser, the per-step validator, the plan model, and the deterministic
// human renderer. Black-box command behavior is covered in package commands_test.

import (
	"strings"
	"testing"
)

// --- Parse ---

func TestParse_ValidMinimal(t *testing.T) {
	f, err := Parse([]byte(`{"schemaVersion":1,"name":"w","steps":[{"id":"a","type":"policy.check","config":"c.json","policy":"p.json"}]}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if f.SchemaVersion != 1 || f.Name != "w" || len(f.Steps) != 1 {
		t.Fatalf("unexpected parse result: %+v", f)
	}
	if f.Steps[0].ID != "a" || f.Steps[0].Type != "policy.check" || f.Steps[0].Config != "c.json" || f.Steps[0].Policy != "p.json" {
		t.Errorf("step fields not preserved: %+v", f.Steps[0])
	}
}

func TestParse_Empty(t *testing.T) {
	if _, err := Parse([]byte("   ")); err == nil {
		t.Fatal("Parse(empty) should error")
	}
}

func TestParse_MalformedJSON(t *testing.T) {
	if _, err := Parse([]byte("{not json")); err == nil {
		t.Fatal("Parse(malformed) should error")
	}
}

func TestParse_TrailingData(t *testing.T) {
	_, err := Parse([]byte(`{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]} extra`))
	if err == nil || !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("Parse(trailing) err = %v, want trailing-data error", err)
	}
}

func TestParse_UnknownFieldRejected(t *testing.T) {
	_, err := Parse([]byte(`{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json","bogus":1}]}`))
	if err == nil {
		t.Fatal("Parse should reject unknown step field")
	}
}

func TestParse_UnknownTopLevelFieldRejected(t *testing.T) {
	_, err := Parse([]byte(`{"schemaVersion":1,"oops":true,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`))
	if err == nil {
		t.Fatal("Parse should reject unknown top-level field")
	}
}

func TestParse_MissingSchemaVersion(t *testing.T) {
	_, err := Parse([]byte(`{"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`))
	if err == nil || !strings.Contains(err.Error(), "schemaVersion") {
		t.Fatalf("Parse err = %v, want missing schemaVersion", err)
	}
}

func TestParse_UnsupportedSchemaVersion(t *testing.T) {
	_, err := Parse([]byte(`{"schemaVersion":2,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("Parse err = %v, want unsupported schemaVersion", err)
	}
}

func TestParse_MissingSteps(t *testing.T) {
	_, err := Parse([]byte(`{"schemaVersion":1}`))
	if err == nil || !strings.Contains(err.Error(), "at least one step") {
		t.Fatalf("Parse err = %v, want at-least-one-step", err)
	}
}

func TestParse_EmptySteps(t *testing.T) {
	_, err := Parse([]byte(`{"schemaVersion":1,"steps":[]}`))
	if err == nil || !strings.Contains(err.Error(), "at least one step") {
		t.Fatalf("Parse err = %v, want at-least-one-step", err)
	}
}

// --- BuildPlan: common validation ---

func mustParse(t *testing.T, src string) File {
	t.Helper()
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return f
}

func TestBuildPlan_ValidPolicyCheckConfig(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"name":"w","steps":[{"id":"a","type":"policy.check","config":"c.json","policy":"p.json"}]}`))
	if p.Result != statusValid || p.Summary.Valid != 1 || p.Summary.Invalid != 0 {
		t.Fatalf("unexpected plan: %+v", p)
	}
	sp := p.Steps[0]
	if sp.Inputs["config"] != "c.json" || sp.Inputs["policy"] != "p.json" {
		t.Errorf("inputs = %v", sp.Inputs)
	}
	if len(sp.DependsOn) != 0 {
		t.Errorf("dependsOn = %v, want empty", sp.DependsOn)
	}
}

func TestBuildPlan_ValidPolicyCheckRuntime(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`))
	if p.Result != statusValid {
		t.Fatalf("result = %s, want valid", p.Result)
	}
	if p.Steps[0].Inputs["runtime"] != "true" {
		t.Errorf("runtime marker missing: %v", p.Steps[0].Inputs)
	}
	if _, ok := p.Steps[0].Inputs["config"]; ok {
		t.Errorf("runtime step should not carry a config input")
	}
}

func TestBuildPlan_PolicyCheckBothSourcesInvalid(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","config":"c.json","runtime":true,"policy":"p.json"}]}`))
	assertInvalid(t, p, "both")
}

func TestBuildPlan_PolicyCheckNeitherSourceInvalid(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","policy":"p.json"}]}`))
	assertInvalid(t, p, "one of")
}

func TestBuildPlan_PolicyCheckMissingPolicyInvalid(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","config":"c.json"}]}`))
	assertInvalid(t, p, "policy")
}

func TestBuildPlan_ValidPolicyReview(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","current":"cur.json","candidate":"cand.json","policy":"p.json"}]}`))
	if p.Result != statusValid {
		t.Fatalf("result = %s, want valid", p.Result)
	}
	in := p.Steps[0].Inputs
	if in["current"] != "cur.json" || in["candidate"] != "cand.json" || in["policy"] != "p.json" {
		t.Errorf("inputs = %v", in)
	}
}

func TestBuildPlan_PolicyReviewMissingFields(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"no current", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","candidate":"x","policy":"p"}]}`, "current"},
		{"no candidate", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","current":"x","policy":"p"}]}`, "candidate"},
		{"no policy", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","current":"x","candidate":"y"}]}`, "policy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertInvalid(t, BuildPlan(mustParse(t, tc.src)), tc.want)
		})
	}
}

func TestBuildPlan_ValidBaselineCompareReport(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.baseline.compare","baseline":"b.json","report":"r.json"}]}`))
	if p.Result != statusValid {
		t.Fatalf("result = %s, want valid", p.Result)
	}
	if p.Steps[0].Inputs["report"] != "r.json" || p.Steps[0].Inputs["baseline"] != "b.json" {
		t.Errorf("inputs = %v", p.Steps[0].Inputs)
	}
	if len(p.Steps[0].DependsOn) != 0 {
		t.Errorf("report-file compare should have no dependency: %v", p.Steps[0].DependsOn)
	}
}

func TestBuildPlan_ValidBaselineCompareReportFrom(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[
		{"id":"chk","type":"policy.check","runtime":true,"policy":"p.json"},
		{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"chk"}
	]}`))
	if p.Result != statusValid || p.Summary.Valid != 2 {
		t.Fatalf("unexpected plan: %+v", p)
	}
	cmp := p.Steps[1]
	if cmp.Inputs["reportFrom"] != "chk" {
		t.Errorf("reportFrom input = %v", cmp.Inputs)
	}
	if len(cmp.DependsOn) != 1 || cmp.DependsOn[0] != "chk" {
		t.Errorf("dependsOn = %v, want [chk]", cmp.DependsOn)
	}
}

func TestBuildPlan_BaselineCompareMissingBaseline(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.baseline.compare","report":"r.json"}]}`))
	assertInvalid(t, p, "baseline")
}

func TestBuildPlan_BaselineCompareBothReportSourcesInvalid(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[
		{"id":"chk","type":"policy.check","runtime":true,"policy":"p.json"},
		{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","report":"r.json","reportFrom":"chk"}
	]}`))
	assertInvalid(t, p, "both")
}

func TestBuildPlan_BaselineCompareNeitherReportSourceInvalid(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.baseline.compare","baseline":"b.json"}]}`))
	assertInvalid(t, p, "one of")
}

func TestBuildPlan_ReportFromUnknown(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"nope"}]}`))
	assertInvalid(t, p, "unknown step id")
}

func TestBuildPlan_ReportFromLaterStep(t *testing.T) {
	// cmp references "chk", which is defined AFTER it → invalid (not earlier).
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[
		{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"chk"},
		{"id":"chk","type":"policy.check","runtime":true,"policy":"p.json"}
	]}`))
	if p.Result != statusInvalid {
		t.Fatalf("result = %s, want invalid", p.Result)
	}
	if !strings.Contains(p.Steps[0].Message, "does not appear before") {
		t.Errorf("message = %q, want later-step wording", p.Steps[0].Message)
	}
}

func TestBuildPlan_ReportFromSelfInvalid(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"a"}]}`))
	if p.Result != statusInvalid {
		t.Fatalf("result = %s, want invalid", p.Result)
	}
	if !strings.Contains(p.Steps[0].Message, "does not appear before") {
		t.Errorf("self-reference message = %q", p.Steps[0].Message)
	}
}

func TestBuildPlan_DuplicateIDInvalid(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[
		{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"},
		{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}
	]}`))
	if p.Result != statusInvalid || p.Summary.Invalid != 1 || p.Summary.Valid != 1 {
		t.Fatalf("unexpected plan: %+v", p)
	}
	// First occurrence stays valid; the duplicate (second) is invalid.
	if p.Steps[0].Status != statusValid {
		t.Errorf("first step should be valid, got %s", p.Steps[0].Status)
	}
	if p.Steps[1].Status != statusInvalid || !strings.Contains(p.Steps[1].Message, "Duplicate step id") {
		t.Errorf("second step = %+v, want duplicate invalid", p.Steps[1])
	}
}

func TestBuildPlan_MissingIDInvalid(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"type":"policy.check","runtime":true,"policy":"p.json"}]}`))
	assertInvalid(t, p, "missing a required id")
}

func TestBuildPlan_UnknownTypeInvalid(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.apply"}]}`))
	assertInvalid(t, p, "Unsupported step type")
}

func TestBuildPlan_MissingTypeInvalid(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a"}]}`))
	assertInvalid(t, p, "missing a required type")
}

func TestBuildPlan_MixedResultCounts(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[
		{"id":"ok","type":"policy.check","runtime":true,"policy":"p.json"},
		{"id":"bad","type":"policy.review","current":"x"}
	]}`))
	if p.Summary.Total != 2 || p.Summary.Valid != 1 || p.Summary.Invalid != 1 {
		t.Fatalf("summary = %+v", p.Summary)
	}
	if p.Result != statusInvalid {
		t.Errorf("result = %s, want invalid", p.Result)
	}
}

// --- exit code ---

func TestPlanExitCode(t *testing.T) {
	if PlanExitCode(Plan{Result: statusValid}) != 0 {
		t.Error("valid plan should exit 0")
	}
	if PlanExitCode(Plan{Result: statusInvalid}) != 1 {
		t.Error("invalid plan should exit 1")
	}
}

// --- human rendering ---

func TestPrintHuman_ValidShape(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"name":"local-policy-check","steps":[
		{"id":"check-current","type":"policy.check","config":"portier.json","policy":"local-safe.policy.json"},
		{"id":"compare-baseline","type":"policy.baseline.compare","baseline":"policy-baseline.json","reportFrom":"check-current"}
	]}`))
	var sb strings.Builder
	PrintHuman(p, false, &sb)
	out := sb.String()
	for _, want := range []string{
		"Portier Workflow Plan",
		"Workflow: local-policy-check",
		"[VALID]",
		"check-current",
		"policy.check",
		"Uses config file: portier.json",
		"Uses policy file: local-safe.policy.json",
		"Uses baseline: policy-baseline.json",
		"Uses report from: check-current",
		"Summary:",
		"2 total",
		"2 valid",
		"0 invalid",
		"Result: valid",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintHuman_InvalidShowsReason(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","current":"x"}]}`))
	var sb strings.Builder
	PrintHuman(p, false, &sb)
	out := sb.String()
	if !strings.Contains(out, "[INVALID]") || !strings.Contains(out, "candidate") {
		t.Errorf("invalid human output should show reason:\n%s", out)
	}
	if !strings.Contains(out, "Result: invalid") {
		t.Errorf("missing invalid result line:\n%s", out)
	}
}

func TestPrintHuman_RuntimeMarkerNoValue(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`))
	var sb strings.Builder
	PrintHuman(p, false, &sb)
	out := sb.String()
	if !strings.Contains(out, "Uses runtime config") {
		t.Errorf("runtime marker line missing:\n%s", out)
	}
	if strings.Contains(out, "Uses runtime config:") {
		t.Errorf("runtime marker should not print a value:\n%s", out)
	}
}

func TestPrintHuman_UnnamedWorkflow(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`))
	var sb strings.Builder
	PrintHuman(p, false, &sb)
	if !strings.Contains(sb.String(), "Workflow: (unnamed)") {
		t.Errorf("unnamed workflow placeholder missing:\n%s", sb.String())
	}
}

func TestPrintHuman_EmptyIDAndTypePlaceholders(t *testing.T) {
	// A step missing both id and type renders with the "(step N)" and "(no type)"
	// placeholders rather than blanks.
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{}]}`))
	var sb strings.Builder
	PrintHuman(p, false, &sb)
	out := sb.String()
	if !strings.Contains(out, "(step 1)") {
		t.Errorf("missing (step N) placeholder:\n%s", out)
	}
	if !strings.Contains(out, "(no type)") {
		t.Errorf("missing (no type) placeholder:\n%s", out)
	}
}

func TestStatusTag(t *testing.T) {
	if statusTag(statusValid) != "[VALID]" {
		t.Errorf("statusTag(valid) = %q", statusTag(statusValid))
	}
	if statusTag(statusInvalid) != "[INVALID]" {
		t.Errorf("statusTag(invalid) = %q", statusTag(statusInvalid))
	}
}

// --- v1.11 Slice 2: step validation codes ---

// TestBuildPlan_StepCodes maps each Slice 1 validation outcome to its stable
// code, so a code regression is caught. A single-step workflow per case.
func TestBuildPlan_StepCodes(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"valid", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p"}]}`, codeStepValid},
		{"missing id", `{"schemaVersion":1,"steps":[{"type":"policy.check","runtime":true,"policy":"p"}]}`, codeStepMissingID},
		{"missing type", `{"schemaVersion":1,"steps":[{"id":"a"}]}`, codeStepMissingType},
		{"unknown type", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.apply"}]}`, codeStepUnknownType},
		{"check missing source", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","policy":"p"}]}`, codeStepMissingConfigSource},
		{"check conflicting source", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","config":"c","runtime":true,"policy":"p"}]}`, codeStepConflictingConfigSources},
		{"check missing policy", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","config":"c"}]}`, codeStepMissingPolicy},
		{"review missing current", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","candidate":"x","policy":"p"}]}`, codeStepMissingCurrent},
		{"review missing candidate", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","current":"x","policy":"p"}]}`, codeStepMissingCandidate},
		{"review missing policy", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","current":"x","candidate":"y"}]}`, codeStepMissingPolicy},
		{"compare missing baseline", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.baseline.compare","report":"r"}]}`, codeStepMissingBaseline},
		{"compare missing report source", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.baseline.compare","baseline":"b"}]}`, codeStepMissingReportSource},
		{"compare conflicting report source", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.baseline.compare","baseline":"b","report":"r","reportFrom":"x"}]}`, codeStepConflictingReportSources},
		{"compare unknown reportFrom", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.baseline.compare","baseline":"b","reportFrom":"nope"}]}`, codeStepUnknownReportFrom},
		{"compare future reportFrom", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.baseline.compare","baseline":"b","reportFrom":"later"},{"id":"later","type":"policy.check","runtime":true,"policy":"p"}]}`, codeStepFutureReportFrom},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := BuildPlan(mustParse(t, tc.src))
			if p.Steps[0].Code != tc.want {
				t.Errorf("step code = %q, want %q (message: %q)", p.Steps[0].Code, tc.want, p.Steps[0].Message)
			}
		})
	}
}

func TestBuildPlan_DuplicateIDCode(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[
		{"id":"a","type":"policy.check","runtime":true,"policy":"p"},
		{"id":"a","type":"policy.check","runtime":true,"policy":"p"}
	]}`))
	if p.Steps[0].Code != codeStepValid {
		t.Errorf("first step code = %q, want valid", p.Steps[0].Code)
	}
	if p.Steps[1].Code != codeStepDuplicateID {
		t.Errorf("second step code = %q, want %q", p.Steps[1].Code, codeStepDuplicateID)
	}
}

func TestInvalidCodesOf(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[
		{"id":"ok","type":"policy.check","runtime":true,"policy":"p"},
		{"id":"bad","type":"policy.review","current":"x"},
		{"id":"bad2","type":"policy.apply"}
	]}`))
	codes := invalidCodesOf(p)
	if len(codes) != 2 {
		t.Fatalf("invalidCodesOf = %v, want 2 codes (valid step excluded)", codes)
	}
	if codes[0] != codeStepMissingCandidate || codes[1] != codeStepUnknownType {
		t.Errorf("invalidCodesOf = %v, want [missing_candidate unknown_type] in order", codes)
	}
}

func TestInvalidCodesOf_AllValid(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p"}]}`))
	if len(invalidCodesOf(p)) != 0 {
		t.Errorf("all-valid plan should have no invalid codes")
	}
}

func TestPrintHuman_ExplainInvalidStepOnly(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[
		{"id":"ok","type":"policy.check","runtime":true,"policy":"p"},
		{"id":"bad","type":"policy.baseline.compare","baseline":"b","reportFrom":"nope"}
	]}`))
	var sb strings.Builder
	PrintHuman(p, true, &sb)
	out := sb.String()
	// Invalid step gets an inline explanation block for its code.
	if !strings.Contains(out, "Code: "+codeStepUnknownReportFrom) {
		t.Errorf("missing inline explanation for invalid step:\n%s", out)
	}
	if !strings.Contains(out, "Meaning:") || !strings.Contains(out, "What to do:") {
		t.Errorf("inline block incomplete:\n%s", out)
	}
	// Valid step must NOT get an explanation block.
	if strings.Contains(out, "Code: "+codeStepValid) {
		t.Errorf("valid step should not be explained:\n%s", out)
	}
}

func TestPrintHuman_NoExplainOmitsBlocks(t *testing.T) {
	p := BuildPlan(mustParse(t, `{"schemaVersion":1,"steps":[{"id":"bad","type":"policy.apply"}]}`))
	var sb strings.Builder
	PrintHuman(p, false, &sb)
	if strings.Contains(sb.String(), "Code:") || strings.Contains(sb.String(), "Meaning:") {
		t.Errorf("non-explain human output should have no explanation block:\n%s", sb.String())
	}
}

// assertInvalid asserts a single-step plan is invalid and the (first) step's
// message contains want.
func assertInvalid(t *testing.T, p Plan, want string) {
	t.Helper()
	if p.Result != statusInvalid {
		t.Fatalf("result = %s, want invalid (%+v)", p.Result, p)
	}
	if len(p.Steps) == 0 {
		t.Fatal("plan has no steps")
	}
	// Find the first invalid step.
	for _, sp := range p.Steps {
		if sp.Status == statusInvalid {
			if !strings.Contains(sp.Message, want) {
				t.Errorf("invalid step message = %q, want substring %q", sp.Message, want)
			}
			return
		}
	}
	t.Errorf("no invalid step found in %+v", p)
}
