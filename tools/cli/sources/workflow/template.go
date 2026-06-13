package workflow

// Built-in workflow templates: a small static registry of common starter
// workflow files so operators don't have to invent the workflow JSON schema by
// hand. Each template renders to a complete workflow file in exactly the schema
// Parse accepts, so a rendered template (or a file written from one) can be
// passed straight to `workflow plan --file`. Templates are dry-run conveniences —
// they only compose the existing validated step types (policy.check /
// policy.review / policy.baseline.compare) and never introduce execution,
// scheduler, or mutation/apply/import fields.

import (
	"fmt"
	"io"
	"sort"

	"portier/cli/sources/output"
)

// templateStep is the serializable form of one workflow step in a template. Only
// the fields a step actually uses are emitted (omitempty), so the generated file
// is minimal and directly editable. The field names match stepRaw exactly, so a
// marshaled template round-trips through Parse (which rejects unknown fields).
type templateStep struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Config     string `json:"config,omitempty"`
	Runtime    bool   `json:"runtime,omitempty"`
	Policy     string `json:"policy,omitempty"`
	Current    string `json:"current,omitempty"`
	Candidate  string `json:"candidate,omitempty"`
	Baseline   string `json:"baseline,omitempty"`
	Report     string `json:"report,omitempty"`
	ReportFrom string `json:"reportFrom,omitempty"`
}

// templateWorkflowFile is the serializable workflow file a template renders to.
// Its JSON is exactly the schema Parse accepts (schemaVersion + name + steps).
type templateWorkflowFile struct {
	SchemaVersion int            `json:"schemaVersion"`
	Name          string         `json:"name"`
	Steps         []templateStep `json:"steps"`
}

// Template is a built-in workflow template: operator-facing metadata plus a
// complete workflow file. The workflow file is unexported; access it via
// WorkflowValue (for the bare workflow JSON) or DetailValue (for the --json
// metadata wrapper).
type Template struct {
	Name        string
	Title       string
	Description string
	file        templateWorkflowFile
}

// templateSummaryJSON is the per-template entry in the --list --json output.
type templateSummaryJSON struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// templateListJSON is the --list --json envelope.
type templateListJSON struct {
	Templates []templateSummaryJSON `json:"templates"`
}

// templateDetailJSON is the single-template --json envelope (metadata + the full
// workflow file under "workflow").
type templateDetailJSON struct {
	Name        string               `json:"name"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Workflow    templateWorkflowFile `json:"workflow"`
}

// builtinTemplates is the static template registry. Definitions are kept in name
// order for readability; Templates() returns a sorted copy regardless, so output
// is deterministic even if this list is reordered. Every template uses
// schemaVersion 1 and only the existing validated step types — no execution,
// scheduler, or mutation/apply/import fields.
var builtinTemplates = []Template{
	{
		Name:        "policy-check-local",
		Title:       "Policy check local config",
		Description: "Checks a local config file against a local policy file.",
		file: templateWorkflowFile{
			SchemaVersion: schemaVersion,
			Name:          "policy-check-local",
			Steps: []templateStep{
				{
					ID:     "check-config",
					Type:   stepPolicyCheck,
					Config: "portier.json",
					Policy: "local-safe.policy.json",
				},
			},
		},
	},
	{
		Name:        "policy-check-runtime",
		Title:       "Policy check runtime config",
		Description: "Checks the live runtime config against a local policy file.",
		file: templateWorkflowFile{
			SchemaVersion: schemaVersion,
			Name:          "policy-check-runtime",
			Steps: []templateStep{
				{
					ID:      "check-runtime",
					Type:    stepPolicyCheck,
					Runtime: true,
					Policy:  "local-safe.policy.json",
				},
			},
		},
	},
	{
		Name:        "policy-review",
		Title:       "Policy review",
		Description: "Compares a current config with a candidate config and evaluates the candidate against a policy.",
		file: templateWorkflowFile{
			SchemaVersion: schemaVersion,
			Name:          "policy-review",
			Steps: []templateStep{
				{
					ID:        "review-candidate",
					Type:      stepPolicyReview,
					Current:   "current-portier.json",
					Candidate: "candidate-portier.json",
					Policy:    "local-safe.policy.json",
				},
			},
		},
	},
	{
		Name:        "policy-baseline-check",
		Title:       "Policy baseline check",
		Description: "Checks a config against a policy, then compares the resulting report against a policy baseline.",
		file: templateWorkflowFile{
			SchemaVersion: schemaVersion,
			Name:          "policy-baseline-check",
			Steps: []templateStep{
				{
					ID:     "check-config",
					Type:   stepPolicyCheck,
					Config: "portier.json",
					Policy: "local-safe.policy.json",
				},
				{
					ID:         "compare-baseline",
					Type:       stepPolicyBaselineCompare,
					Baseline:   "policy-baseline.json",
					ReportFrom: "check-config",
				},
			},
		},
	},
}

// Templates returns the built-in workflow templates, sorted by name (deterministic).
func Templates() []Template {
	out := make([]Template, len(builtinTemplates))
	copy(out, builtinTemplates)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// FindTemplate returns the built-in template with the given (exact) name.
func FindTemplate(name string) (Template, bool) {
	for _, t := range builtinTemplates {
		if t.Name == name {
			return t, true
		}
	}
	return Template{}, false
}

// WorkflowValue returns the JSON-marshalable workflow file for the template — the
// exact schema Parse accepts. Used for the bare workflow JSON on stdout and for
// --out, so a written file can be passed straight to `workflow plan --file`.
func (t Template) WorkflowValue() any { return t.file }

// DetailValue returns the JSON-marshalable metadata+workflow wrapper used for a
// single template under --json.
func (t Template) DetailValue() any {
	return templateDetailJSON{
		Name:        t.Name,
		Title:       t.Title,
		Description: t.Description,
		Workflow:    t.file,
	}
}

// PrintTemplateList writes the deterministic template list to w. With jsonOutput
// it emits {"templates":[{name,title,description}]}; otherwise a compact human
// list (name column, title, then an indented description). Returns any
// JSON-encode error (the human path never fails).
func PrintTemplateList(jsonOutput bool, w io.Writer) error {
	templates := Templates()

	if jsonOutput {
		list := templateListJSON{Templates: make([]templateSummaryJSON, len(templates))}
		for i, t := range templates {
			list.Templates[i] = templateSummaryJSON{Name: t.Name, Title: t.Title, Description: t.Description}
		}
		return output.PrintJSON(w, list)
	}

	fmt.Fprintln(w, "Workflow templates")
	fmt.Fprintln(w)

	width := 0
	for _, t := range templates {
		if len(t.Name) > width {
			width = len(t.Name)
		}
	}
	for i, t := range templates {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%-*s   %s\n", width, t.Name, t.Title)
		fmt.Fprintf(w, "  %s\n", t.Description)
	}
	return nil
}

// TemplateNames returns the built-in template names, sorted — for error messages
// that suggest the available templates.
func TemplateNames() []string {
	templates := Templates()
	names := make([]string, len(templates))
	for i, t := range templates {
		names[i] = t.Name
	}
	return names
}
