package core

import "testing"

// safetyItemIDs are the four analyses replay never performs — present in every plan.
var safetyItemIDs = []string{"workflow-rerun", "runtime-probing", "config-policy-reread", "mutation-enforcement"}

func assertSafetyItemsUnavailable(t *testing.T, p Plan) {
	t.Helper()
	for _, id := range safetyItemIDs {
		it := itemByID(t, p, id)
		if it.Status != ItemUnavailable {
			t.Errorf("safety item %q status = %q, want unavailable", id, it.Status)
		}
		if it.Inputs == nil {
			t.Errorf("safety item %q Inputs is nil, want non-nil slice", id)
		}
	}
}

func assertSummaryConsistent(t *testing.T, p Plan) {
	t.Helper()
	var avail, unavail int
	for _, it := range p.Items {
		switch it.Status {
		case ItemAvailable:
			avail++
		case ItemUnavailable:
			unavail++
		default:
			t.Errorf("item %q has unexpected status %q", it.ID, it.Status)
		}
		if it.Inputs == nil {
			t.Errorf("item %q Inputs is nil, want non-nil slice (for stable [] JSON)", it.ID)
		}
	}
	if p.Summary.Available != avail {
		t.Errorf("Summary.Available = %d, counted %d", p.Summary.Available, avail)
	}
	if p.Summary.Unavailable != unavail {
		t.Errorf("Summary.Unavailable = %d, counted %d", p.Summary.Unavailable, unavail)
	}
}

func TestBuildPlan_PassedRunReport(t *testing.T) {
	p := BuildPlan(Input{Kind: SourceWorkflowRunReport, Workflow: "ok", Result: "passed", Steps: 2, HasCodes: false})
	if p.SchemaVersion != PlanSchemaVersion {
		t.Errorf("SchemaVersion = %d", p.SchemaVersion)
	}
	if p.Source != SourceWorkflowRunReport {
		t.Errorf("Source = %q", p.Source)
	}
	if itemByID(t, p, "step-timeline").Status != ItemAvailable {
		t.Error("step-timeline should be available when steps present")
	}
	if itemByID(t, p, "step-status-summary").Status != ItemAvailable {
		t.Error("step-status-summary should be available when steps present")
	}
	if itemByID(t, p, "explanation-lookup").Status != ItemUnavailable {
		t.Error("explanation-lookup should be unavailable when no codes")
	}
	assertSafetyItemsUnavailable(t, p)
	assertSummaryConsistent(t, p)
}

func TestBuildPlan_FailedRunReport(t *testing.T) {
	p := BuildPlan(Input{Kind: SourceWorkflowRunReport, Workflow: "bad", Result: "failed", Steps: 3, HasCodes: true})
	if p.Result != "failed" {
		t.Errorf("Result = %q", p.Result)
	}
	if itemByID(t, p, "explanation-lookup").Status != ItemAvailable {
		t.Error("explanation-lookup should be available when codes present")
	}
	assertSafetyItemsUnavailable(t, p)
	assertSummaryConsistent(t, p)
}

func TestBuildPlan_RunReportNoSteps(t *testing.T) {
	p := BuildPlan(Input{Kind: SourceWorkflowRunReport, Workflow: "x", Result: "passed", Steps: 0})
	if itemByID(t, p, "step-timeline").Status != ItemUnavailable {
		t.Error("step-timeline should be unavailable with no steps")
	}
	assertSummaryConsistent(t, p)
}

func TestBuildPlan_PlanReport(t *testing.T) {
	p := BuildPlan(Input{Kind: SourceWorkflowPlanReport, Workflow: "rev", Result: "valid", Steps: 1, HasCodes: false})
	if itemByID(t, p, "step-validation-summary").Status != ItemAvailable {
		t.Error("step-validation-summary should be available with steps")
	}
	if itemByID(t, p, "explanation-lookup").Status != ItemUnavailable {
		t.Error("explanation-lookup should be unavailable when no codes")
	}
	assertSafetyItemsUnavailable(t, p)
	assertSummaryConsistent(t, p)
}

func TestBuildPlan_HistoryExportWithRuns(t *testing.T) {
	p := BuildPlan(Input{Kind: SourceWorkflowHistoryExport, Runs: 5})
	if p.Workflow != "" || p.Result != "" {
		t.Errorf("history export plan should omit workflow/result, got %q/%q", p.Workflow, p.Result)
	}
	for _, id := range []string{"result-distribution", "workflow-distribution", "step-status-distribution", "step-type-distribution", "code-distribution", "failed-run-shortlist"} {
		if itemByID(t, p, id).Status != ItemAvailable {
			t.Errorf("%q should be available with runs", id)
		}
	}
	assertSafetyItemsUnavailable(t, p)
	assertSummaryConsistent(t, p)
}

func TestBuildPlan_EmptyHistoryExport(t *testing.T) {
	p := BuildPlan(Input{Kind: SourceWorkflowHistoryExport, Runs: 0})
	for _, id := range []string{"result-distribution", "workflow-distribution", "code-distribution", "failed-run-shortlist"} {
		if itemByID(t, p, id).Status != ItemUnavailable {
			t.Errorf("%q should be unavailable with no runs", id)
		}
	}
	if p.Summary.Available != 0 {
		t.Errorf("empty history export should have 0 available items, got %d", p.Summary.Available)
	}
	assertSafetyItemsUnavailable(t, p)
	assertSummaryConsistent(t, p)
}

func TestBuildPlan_ReportBundle(t *testing.T) {
	p := BuildPlan(Input{Kind: SourceWorkflowReportBundle, Workflow: "w", Result: "failed"})
	for _, id := range []string{"summary-reconstruction", "normalized-report-inspection", "explanation-lookup"} {
		if itemByID(t, p, id).Status != ItemAvailable {
			t.Errorf("%q should be available for a bundle", id)
		}
	}
	assertSafetyItemsUnavailable(t, p)
	assertSummaryConsistent(t, p)
}

func TestBuildPlan_Deterministic(t *testing.T) {
	in := Input{Kind: SourceWorkflowRunReport, Workflow: "w", Result: "failed", Steps: 2, HasCodes: true}
	a, err := MarshalPlan(BuildPlan(in))
	if err != nil {
		t.Fatal(err)
	}
	b, err := MarshalPlan(BuildPlan(in))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Error("BuildPlan is not deterministic across calls")
	}
}
