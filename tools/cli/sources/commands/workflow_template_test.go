package commands_test

// Black-box tests for `portier workflow template` (v1.11 Slice 3): built-in
// starter workflow templates. Fully offline; never contacts the runtime, never
// executes a step, and never modifies any file except the requested --out file.
// A rendered/`--out` template must be directly usable by `workflow plan --file`.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/commands"
	"portier/cli/sources/workflow"
)

// runWorkflowTemplate runs `workflow template <args...>` and returns stdout,
// stderr, and the exit code.
func runWorkflowTemplate(t *testing.T, jsonOutput bool, args ...string) (string, string, int) {
	t.Helper()
	var out, errBuf strings.Builder
	code := commands.RunWorkflowTemplate(jsonOutput, args, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

// --- list ---

func TestWorkflowTemplate_ListHuman(t *testing.T) {
	out, _, code := runWorkflowTemplate(t, false, "--list")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"Workflow templates",
		"policy-baseline-check",
		"policy-check-local",
		"policy-check-runtime",
		"policy-review",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("--list missing %q:\n%s", want, out)
		}
	}
}

func TestWorkflowTemplate_ListJSONSorted(t *testing.T) {
	out, _, code := runWorkflowTemplate(t, true, "--list")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var wrapper struct {
		Templates []struct {
			Name        string `json:"name"`
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"templates"`
	}
	if err := json.Unmarshal([]byte(out), &wrapper); err != nil {
		t.Fatalf("decoding list JSON: %v\n%s", err, out)
	}
	if len(wrapper.Templates) != 4 {
		t.Fatalf("list has %d templates, want 4", len(wrapper.Templates))
	}
	for i := 1; i < len(wrapper.Templates); i++ {
		if wrapper.Templates[i-1].Name >= wrapper.Templates[i].Name {
			t.Errorf("list not sorted: %q before %q", wrapper.Templates[i-1].Name, wrapper.Templates[i].Name)
		}
	}
	// List entries must NOT embed the workflow body.
	if strings.Contains(out, "\"steps\"") || strings.Contains(out, "\"workflow\"") {
		t.Errorf("--list --json should not embed workflow bodies:\n%s", out)
	}
}

// --- single template render ---

func TestWorkflowTemplate_RenderBareWorkflow(t *testing.T) {
	for _, name := range []string{"policy-check-local", "policy-check-runtime", "policy-review", "policy-baseline-check"} {
		t.Run(name, func(t *testing.T) {
			out, _, code := runWorkflowTemplate(t, false, name)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			// Bare workflow JSON (not the metadata wrapper).
			if strings.Contains(out, "\"workflow\"") || strings.Contains(out, "\"title\"") {
				t.Errorf("%s: human output should be the bare workflow, not the wrapper:\n%s", name, out)
			}
			// It must parse + plan valid.
			f, err := workflow.Parse([]byte(out))
			if err != nil {
				t.Fatalf("%s: rendered workflow does not parse: %v", name, err)
			}
			if workflow.BuildPlan(f).Result != "valid" {
				t.Errorf("%s: rendered workflow does not plan valid", name)
			}
		})
	}
}

func TestWorkflowTemplate_SingleJSONWrapper(t *testing.T) {
	out, _, code := runWorkflowTemplate(t, true, "policy-review")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var wrapper struct {
		Name        string          `json:"name"`
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Workflow    json.RawMessage `json:"workflow"`
	}
	if err := json.Unmarshal([]byte(out), &wrapper); err != nil {
		t.Fatalf("decoding wrapper JSON: %v\n%s", err, out)
	}
	if wrapper.Name != "policy-review" || wrapper.Title == "" || wrapper.Description == "" {
		t.Errorf("wrapper metadata wrong: %+v", wrapper)
	}
	f, err := workflow.Parse(wrapper.Workflow)
	if err != nil {
		t.Fatalf("embedded workflow does not parse: %v", err)
	}
	if workflow.BuildPlan(f).Result != "valid" {
		t.Errorf("embedded workflow does not plan valid")
	}
}

// --- --out ---

func TestWorkflowTemplate_OutWritesBareWorkflowUsableByPlan(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "workflow.json")
	stdout, _, code := runWorkflowTemplate(t, false, "policy-baseline-check", "--out", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Workflow written to "+outPath) {
		t.Errorf("missing write confirmation:\n%s", stdout)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading out file: %v", err)
	}
	// The file must be the bare workflow (not the wrapper) and parse + plan valid.
	if strings.Contains(string(data), "\"workflow\"") || strings.Contains(string(data), "\"title\"") {
		t.Errorf("out file should be the bare workflow, not the wrapper:\n%s", data)
	}
	f, err := workflow.Parse(data)
	if err != nil {
		t.Fatalf("out file does not parse: %v", err)
	}
	if workflow.BuildPlan(f).Result != "valid" {
		t.Errorf("out file does not plan valid")
	}

	// End-to-end: `workflow plan --file <outfile>` accepts it (exit 0).
	var planOut, planErr strings.Builder
	planCode := commands.RunWorkflowPlan(false, []string{"--file", outPath}, &planOut, &planErr)
	if planCode != 0 {
		t.Fatalf("workflow plan rejected the template file: exit %d\n%s\n%s", planCode, planOut.String(), planErr.String())
	}
}

func TestWorkflowTemplate_JSONOutWritesBareWorkflow(t *testing.T) {
	// Under --json, stdout is the wrapper but --out still writes the bare workflow.
	outPath := filepath.Join(t.TempDir(), "workflow.json")
	stdout, _, code := runWorkflowTemplate(t, true, "policy-check-local", "--out", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "\"workflow\"") {
		t.Errorf("--json stdout should be the wrapper:\n%s", stdout)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading out file: %v", err)
	}
	if strings.Contains(string(data), "\"workflow\"") {
		t.Errorf("out file should be the bare workflow, not the wrapper:\n%s", data)
	}
	if _, err := workflow.Parse(data); err != nil {
		t.Fatalf("out file does not parse: %v", err)
	}
}

func TestWorkflowTemplate_FlagAfterName(t *testing.T) {
	// `template <name> --out f` (flag after the positional name) must work.
	outPath := filepath.Join(t.TempDir(), "w.json")
	_, _, code := runWorkflowTemplate(t, false, "policy-review", "--out", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("out file not written: %v", err)
	}
}

// --- error paths ---

func TestWorkflowTemplate_UnknownExit2(t *testing.T) {
	_, errOut, code := runWorkflowTemplate(t, false, "does-not-exist")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "unknown template") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestWorkflowTemplate_MissingNameExit2(t *testing.T) {
	_, errOut, code := runWorkflowTemplate(t, false)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "template name is required") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestWorkflowTemplate_TooManyArgsExit2(t *testing.T) {
	_, _, code := runWorkflowTemplate(t, false, "policy-review", "policy-check-local")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowTemplate_MissingOutValueExit2(t *testing.T) {
	_, _, code := runWorkflowTemplate(t, false, "policy-review", "--out")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowTemplate_ListWithNameExit2(t *testing.T) {
	_, _, code := runWorkflowTemplate(t, false, "--list", "policy-review")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowTemplate_ListWithOutExit2(t *testing.T) {
	_, _, code := runWorkflowTemplate(t, false, "--list", "--out", filepath.Join(t.TempDir(), "x.json"))
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowTemplate_BadFlagExit2(t *testing.T) {
	_, _, code := runWorkflowTemplate(t, false, "policy-review", "--nope")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowTemplate_HelpExit0(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunWorkflowTemplate(false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: portier workflow template") {
		t.Errorf("help missing:\n%s", out.String())
	}
}

func TestWorkflowTemplate_OutWriteFailureExit1(t *testing.T) {
	// A path whose parent directory does not exist → write failure (not created).
	badOut := filepath.Join(t.TempDir(), "nope", "w.json")
	_, errOut, code := runWorkflowTemplate(t, false, "policy-review", "--out", badOut)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (write failure)", code)
	}
	if !strings.Contains(errOut, "Error writing") {
		t.Errorf("stderr = %q", errOut)
	}
}

// --- safety: no file mutation, offline ---

func TestWorkflowTemplate_DoesNotMutateUnrelatedFiles(t *testing.T) {
	// Rendering a template (no --out) must not write anything. We assert that a
	// pre-existing sentinel file in the temp dir is untouched and no new file is
	// created next to it.
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("untouched"), 0o644); err != nil {
		t.Fatalf("seeding sentinel: %v", err)
	}
	_, _, code := runWorkflowTemplate(t, false, "policy-review")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("template render created/removed files: %v", entries)
	}
	data, _ := os.ReadFile(sentinel)
	if string(data) != "untouched" {
		t.Errorf("sentinel mutated: %q", data)
	}
}

// --- dispatch ---

func TestRunWorkflow_TemplateDispatch(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunWorkflow(false, commands.ConnFlags{}, []string{"template", "--list"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Workflow templates") {
		t.Errorf("template dispatch output wrong:\n%s", out.String())
	}
}
