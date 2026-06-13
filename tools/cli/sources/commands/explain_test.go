package commands_test

import (
	"encoding/json"
	"strings"
	"testing"

	"portier/cli/sources/commands"
	"portier/cli/sources/explain"
)

func TestExplain_ConfigCode(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(false, []string{"config.duplicate_binding"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	s := out.String()
	for _, want := range []string{"config.duplicate_binding", "Meaning:", "What to do:", "Related:", "config.validation_failed"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q\noutput:\n%s", want, s)
		}
	}
}

func TestExplain_RuntimeCode(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(false, []string{"rules.health_error"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "portier diagnose") {
		t.Errorf("rules.health_error explanation should mention 'portier diagnose'\noutput:\n%s", out.String())
	}
}

func TestExplain_JSON(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(true, []string{"config.duplicate_binding"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var exp explain.Explanation
	if err := json.Unmarshal([]byte(out.String()), &exp); err != nil {
		t.Fatalf("decoding JSON: %v\noutput:\n%s", err, out.String())
	}
	if exp.Code != "config.duplicate_binding" {
		t.Errorf("code = %q, want config.duplicate_binding", exp.Code)
	}
	for _, field := range []struct{ name, val string }{
		{"title", exp.Title}, {"meaning", exp.Meaning}, {"action", exp.Action},
	} {
		if field.val == "" {
			t.Errorf("JSON field %q is empty", field.name)
		}
	}
	if len(exp.Related) == 0 || exp.Related[0] != "config.validation_failed" {
		t.Errorf("related = %v, want [config.validation_failed]", exp.Related)
	}
}

func TestExplain_JSONShape_Keys(t *testing.T) {
	var out, errBuf strings.Builder
	commands.RunExplain(true, []string{"runtime.unreachable"}, &out, &errBuf)
	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON: %v", err)
	}
	for _, key := range []string{"code", "title", "meaning", "action"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("JSON missing %q: %v", key, raw)
		}
	}
}

func TestExplain_UnknownCode_Exit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(false, []string{"nope.not_a_code"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	es := errBuf.String()
	if !strings.Contains(es, "unknown code") || !strings.Contains(es, "--list") {
		t.Errorf("stderr should report unknown code and suggest --list: %q", es)
	}
	if out.String() != "" {
		t.Errorf("unknown code must not print a generic explanation; got stdout: %q", out.String())
	}
}

func TestExplain_MissingCode_Exit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(false, []string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "requires a code") {
		t.Errorf("stderr missing usage message: %q", errBuf.String())
	}
}

func TestExplain_TooManyArgs_Exit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(false, []string{"config.valid", "config.empty"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestExplain_List(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(false, []string{"--list"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	s := out.String()
	for _, want := range []string{"config.valid", "runtime.reachable", "rules.health_error", "config.export_read", "policy.lan_exposure_forbidden", "policy.group_required"} {
		if !strings.Contains(s, want) {
			t.Errorf("--list output missing %q", want)
		}
	}
	// Sorted alphabetically by code: config.* < policy.* < runtime.*.
	if strings.Index(s, "runtime.reachable") < strings.Index(s, "config.valid") {
		t.Errorf("--list should be sorted (config.* before runtime.*)\noutput:\n%s", s)
	}
	if strings.Index(s, "policy.group_required") > strings.Index(s, "runtime.reachable") {
		t.Errorf("--list should be sorted (policy.* before runtime.*)\noutput:\n%s", s)
	}
}

func TestExplain_PolicyCode(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(false, []string{"policy.lan_exposure_forbidden"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	s := out.String()
	for _, want := range []string{"policy.lan_exposure_forbidden", "Meaning:", "What to do:"} {
		if !strings.Contains(s, want) {
			t.Errorf("policy explain output missing %q\noutput:\n%s", want, s)
		}
	}
}

func TestExplain_PolicyCodeJSON(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(true, []string{"policy.group_required"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var exp explain.Explanation
	if err := json.Unmarshal([]byte(out.String()), &exp); err != nil {
		t.Fatalf("decoding JSON: %v\n%s", err, out.String())
	}
	if exp.Code != "policy.group_required" || exp.Severity != "error" {
		t.Errorf("policy explanation = %+v", exp)
	}
}

func TestExplain_ListJSON(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(true, []string{"--list"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var list []explain.Explanation
	if err := json.Unmarshal([]byte(out.String()), &list); err != nil {
		t.Fatalf("decoding JSON list: %v\noutput:\n%s", err, out.String())
	}
	if len(list) != 26 {
		t.Errorf("list length = %d, want 26 (8 config doctor + 12 live doctor + 6 policy)", len(list))
	}
	// Sorted by code.
	for i := 1; i < len(list); i++ {
		if list[i-1].Code >= list[i].Code {
			t.Errorf("list not sorted: %q before %q", list[i-1].Code, list[i].Code)
		}
	}
}

func TestExplain_JSONEncodeFailure_Exit1(t *testing.T) {
	var errBuf strings.Builder
	code := commands.RunExplain(true, []string{"config.valid"}, failingWriter{}, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Error encoding JSON") {
		t.Errorf("stderr missing encode-error message: %q", errBuf.String())
	}
}

func TestExplain_ListJSONEncodeFailure_Exit1(t *testing.T) {
	var errBuf strings.Builder
	code := commands.RunExplain(true, []string{"--list"}, failingWriter{}, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Error encoding JSON") {
		t.Errorf("stderr missing encode-error message: %q", errBuf.String())
	}
}

func TestExplain_Help_Exit0(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "portier explain") {
		t.Errorf("help output missing usage: %q", out.String())
	}
}

func TestExplain_BadFlag_Exit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(false, []string{"--nope"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}
