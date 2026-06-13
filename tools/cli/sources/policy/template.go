package policy

// Built-in policy templates: a small static registry of common safe-operations
// policy files so operators don't have to invent the JSON schema by hand. Each
// template renders to a complete policy file in exactly the schema Parse accepts,
// so a rendered template (or a file written from one) can be passed straight to
// `policy check`. Templates are metadata-only conveniences — they never introduce
// new guardrails, protocol restrictions, or an allowUdp rule (UDP is first-class).

import (
	"fmt"
	"io"
	"sort"

	"portier/cli/sources/output"
)

// templatePolicyRules is the serializable `rules` object of a template policy
// file. Unlike rulesRaw (which uses pointers to distinguish omitted fields), a
// template emits all five guardrails explicitly so the generated file is a
// complete, self-documenting, directly-editable policy.
type templatePolicyRules struct {
	RequireGroup            bool `json:"requireGroup"`
	AllowLanExposure        bool `json:"allowLanExposure"`
	AllowPrivilegedPorts    bool `json:"allowPrivilegedPorts"`
	AllowAutostart          bool `json:"allowAutostart"`
	ForbidDuplicateBindings bool `json:"forbidDuplicateBindings"`
}

// templatePolicyFile is the serializable policy file a template renders to. Its
// JSON is exactly the schema Parse accepts (schemaVersion + rules object).
type templatePolicyFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Rules         templatePolicyRules `json:"rules"`
}

// Template is a built-in policy template: operator-facing metadata plus a
// complete policy file. The policy file is unexported; access it via PolicyValue
// (for the bare policy JSON) or DetailValue (for the --json metadata wrapper).
type Template struct {
	Name        string
	Title       string
	Description string
	file        templatePolicyFile
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

// templateDetailJSON is the single-template --json envelope (metadata + the
// full policy file under "policy").
type templateDetailJSON struct {
	Name        string             `json:"name"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Policy      templatePolicyFile `json:"policy"`
}

// builtinTemplates is the static template registry. Definitions are kept in name
// order for readability; Templates() returns a sorted copy regardless, so output
// is deterministic even if this list is reordered. Every template uses
// schemaVersion 1 and only the five standard boolean guardrails — no protocol
// allowlist/denylist and no allowUdp rule.
var builtinTemplates = []Template{
	{
		Name:        "local-safe",
		Title:       "Local safe",
		Description: "For local workstation use; forbids LAN exposure and privileged ports, keeps autostart allowed.",
		file: templatePolicyFile{
			SchemaVersion: schemaVersion,
			Rules: templatePolicyRules{
				RequireGroup:            false,
				AllowLanExposure:        false,
				AllowPrivilegedPorts:    false,
				AllowAutostart:          true,
				ForbidDuplicateBindings: true,
			},
		},
	},
	{
		Name:        "managed",
		Title:       "Managed",
		Description: "Stricter managed baseline; requires groups and forbids LAN exposure, privileged ports, and autostart.",
		file: templatePolicyFile{
			SchemaVersion: schemaVersion,
			Rules: templatePolicyRules{
				RequireGroup:            true,
				AllowLanExposure:        false,
				AllowPrivilegedPorts:    false,
				AllowAutostart:          false,
				ForbidDuplicateBindings: true,
			},
		},
	},
	{
		Name:        "permissive",
		Title:       "Permissive",
		Description: "Mirrors Portier's default permissive policy; only duplicate bindings are forbidden.",
		file: templatePolicyFile{
			SchemaVersion: schemaVersion,
			Rules: templatePolicyRules{
				RequireGroup:            false,
				AllowLanExposure:        true,
				AllowPrivilegedPorts:    true,
				AllowAutostart:          true,
				ForbidDuplicateBindings: true,
			},
		},
	},
}

// Templates returns the built-in policy templates, sorted by name (deterministic).
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

// PolicyValue returns the JSON-marshalable policy file for the template — the
// exact schema Parse accepts. Used for the bare policy JSON on stdout and for
// --out, so a written file can be passed straight to `policy check`.
func (t Template) PolicyValue() any { return t.file }

// DetailValue returns the JSON-marshalable metadata+policy wrapper used for a
// single template under --json.
func (t Template) DetailValue() any {
	return templateDetailJSON{
		Name:        t.Name,
		Title:       t.Title,
		Description: t.Description,
		Policy:      t.file,
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

	fmt.Fprintln(w, "Policy templates")
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
