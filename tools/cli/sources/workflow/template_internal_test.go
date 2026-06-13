package workflow

// Internal (white-box) guard for the built-in workflow templates: every template
// must render to a workflow file that Parse accepts and BuildPlan reports VALID,
// the registry must be deterministic (sorted, unique), and no template may
// introduce execution/scheduler/mutation fields — templates only compose the
// existing validated step types.

import (
	"encoding/json"
	"testing"
)

// TestTemplates_SortedAndUnique locks the registry shape: Templates() is sorted
// by name with no duplicates, and the documented four templates are present.
func TestTemplates_SortedAndUnique(t *testing.T) {
	templates := Templates()
	if len(templates) != 4 {
		t.Fatalf("Templates() returned %d templates, want 4", len(templates))
	}
	seen := map[string]bool{}
	for i, tm := range templates {
		if tm.Name == "" || tm.Title == "" || tm.Description == "" {
			t.Errorf("template %d has empty metadata: %+v", i, tm)
		}
		if seen[tm.Name] {
			t.Errorf("duplicate template name %q", tm.Name)
		}
		seen[tm.Name] = true
		if i > 0 && templates[i-1].Name >= tm.Name {
			t.Errorf("templates not sorted: %q before %q", templates[i-1].Name, tm.Name)
		}
	}
	for _, want := range []string{"policy-check-local", "policy-check-runtime", "policy-review", "policy-baseline-check"} {
		if !seen[want] {
			t.Errorf("missing built-in template %q", want)
		}
	}
}

// TestTemplates_RenderToValidPlan renders every template, parses it, and builds a
// plan — each must parse cleanly and produce a VALID plan. This is the guarantee
// that a rendered/`--out` template is directly usable by `workflow plan --file`.
func TestTemplates_RenderToValidPlan(t *testing.T) {
	for _, tm := range Templates() {
		t.Run(tm.Name, func(t *testing.T) {
			data, err := json.Marshal(tm.WorkflowValue())
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			f, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse rejected rendered template: %v\n%s", err, data)
			}
			plan := BuildPlan(f)
			if plan.Result != statusValid {
				t.Errorf("template %q plans as %s, want valid:\n%+v", tm.Name, plan.Result, plan)
			}
			for _, sp := range plan.Steps {
				if sp.Status != statusValid {
					t.Errorf("template %q step %q invalid: %s", tm.Name, sp.ID, sp.Message)
				}
			}
		})
	}
}

// TestTemplates_NoForbiddenFields ensures no template introduces execution,
// scheduler, or mutation/apply/import fields — only the known dry-run step fields
// may appear. The rendered JSON keys are checked against an allowlist.
func TestTemplates_NoForbiddenFields(t *testing.T) {
	allowedTop := map[string]bool{"schemaVersion": true, "name": true, "steps": true}
	allowedStep := map[string]bool{
		"id": true, "type": true, "config": true, "runtime": true, "policy": true,
		"current": true, "candidate": true, "baseline": true, "report": true, "reportFrom": true,
	}
	allowedTypes := map[string]bool{stepPolicyCheck: true, stepPolicyReview: true, stepPolicyBaselineCompare: true}

	for _, tm := range Templates() {
		t.Run(tm.Name, func(t *testing.T) {
			data, err := json.Marshal(tm.WorkflowValue())
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			for k := range raw {
				if !allowedTop[k] {
					t.Errorf("template %q has unexpected top-level field %q", tm.Name, k)
				}
			}
			var steps []map[string]json.RawMessage
			if err := json.Unmarshal(raw["steps"], &steps); err != nil {
				t.Fatalf("unmarshal steps: %v", err)
			}
			for _, step := range steps {
				for k := range step {
					if !allowedStep[k] {
						t.Errorf("template %q step has forbidden/unknown field %q", tm.Name, k)
					}
				}
				var typ string
				_ = json.Unmarshal(step["type"], &typ)
				if !allowedTypes[typ] {
					t.Errorf("template %q step uses non-validated type %q", tm.Name, typ)
				}
			}
		})
	}
}

func TestFindTemplate(t *testing.T) {
	if tm, ok := FindTemplate("policy-review"); !ok || tm.Name != "policy-review" {
		t.Errorf("FindTemplate(policy-review) = %+v, %v", tm, ok)
	}
	if _, ok := FindTemplate("nope"); ok {
		t.Errorf("FindTemplate(nope) should be false")
	}
}

func TestTemplateNames_Sorted(t *testing.T) {
	names := TemplateNames()
	if len(names) != 4 {
		t.Fatalf("TemplateNames() = %v, want 4", names)
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Errorf("names not sorted: %v", names)
		}
	}
}

func TestDetailValue_Shape(t *testing.T) {
	tm, _ := FindTemplate("policy-check-local")
	data, err := json.Marshal(tm.DetailValue())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wrapper struct {
		Name        string          `json:"name"`
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Workflow    json.RawMessage `json:"workflow"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if wrapper.Name != "policy-check-local" || wrapper.Title == "" || wrapper.Description == "" {
		t.Errorf("detail metadata wrong: %+v", wrapper)
	}
	// The embedded workflow must itself parse + plan valid.
	f, err := Parse(wrapper.Workflow)
	if err != nil {
		t.Fatalf("embedded workflow does not parse: %v", err)
	}
	if BuildPlan(f).Result != statusValid {
		t.Errorf("embedded workflow does not plan valid")
	}
}
