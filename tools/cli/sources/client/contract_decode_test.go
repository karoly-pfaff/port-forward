package client_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"portier/cli/sources/client"
)

// TestCLIDTOContractParity is the Arch-D live-runtime parity guard. The CLI is a
// separate Go module whose DTOs (tools/cli/sources/client/client.go) are a third
// copy of the REST contract; its other tests use httptest mocks, which can mask
// drift between the CLI's assumptions and what a real runtime emits.
//
// scripts/validate-contract.js captures representative LIVE JSON responses from
// each running runtime into PORTIER_CLI_CONTRACT_FIXTURES/<runtime>/ and then runs
// this test. Each fixture is decoded STRICTLY (DisallowUnknownFields) into the
// matching CLI DTO so that any field a runtime adds/renames which the CLI does not
// model fails here — forcing the CLI DTOs to be kept in sync with the contract.
//
// When the env var is unset (e.g. plain `go test`/`npm run test:cli`) the test
// skips, so it never fails without live fixtures and the CLI stays decoupled.
func TestCLIDTOContractParity(t *testing.T) {
	root := os.Getenv("PORTIER_CLI_CONTRACT_FIXTURES")
	if root == "" {
		t.Skip("PORTIER_CLI_CONTRACT_FIXTURES not set; captured live fixtures are produced by `npm run validate:contract`")
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read fixtures dir %q: %v", root, err)
	}

	runtimes := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		runtimes++
		dir := filepath.Join(root, e.Name())
		t.Run(e.Name(), func(t *testing.T) {
			assertRuntimeFixtures(t, dir)
		})
	}
	if runtimes == 0 {
		t.Fatalf("no runtime fixture subdirectories under %q", root)
	}
}

// decodeStrict reads a fixture and decodes it into out with DisallowUnknownFields,
// so any unmodeled field is a hard failure (contract drift the CLI has not absorbed).
func decodeStrict(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %q: %v", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		t.Fatalf("strict decode %q into %T failed (CLI DTO out of sync with runtime contract): %v", filepath.Base(path), out, err)
	}
}

func assertRuntimeFixtures(t *testing.T, dir string) {
	t.Helper()
	p := func(name string) string { return filepath.Join(dir, name) }

	// GET /api/runtime → RuntimeInfo
	{
		var info client.RuntimeInfo
		decodeStrict(t, p("runtime.json"), &info)
		if info.Name != "Portier" {
			t.Errorf("runtime.name = %q, want %q", info.Name, "Portier")
		}
		if info.Runtime != "node" && info.Runtime != "go" {
			t.Errorf("runtime.runtime = %q, want node|go", info.Runtime)
		}
		if info.ManagementPort <= 0 {
			t.Errorf("runtime.managementPort = %d, want > 0", info.ManagementPort)
		}
		if info.PID <= 0 {
			t.Errorf("runtime.pid = %d, want > 0", info.PID)
		}
	}

	// POST /api/forwards (create) → ForwardRuleResponse
	{
		var rule client.ForwardRuleResponse
		decodeStrict(t, p("forwards-create.json"), &rule)
		if rule.ID == "" {
			t.Error("created rule has empty id")
		}
		if rule.Name != "CLI Contract" || rule.Protocol != "tcp" {
			t.Errorf("created rule = {name:%q protocol:%q}, want {CLI Contract tcp}", rule.Name, rule.Protocol)
		}
		if rule.Advisories == nil {
			t.Error("created rule advisories decoded as nil, want a slice")
		}
	}

	// GET /api/forwards → []ForwardRuleResponse
	{
		var rules []client.ForwardRuleResponse
		decodeStrict(t, p("forwards.json"), &rules)
		if len(rules) < 1 {
			t.Fatalf("forwards list has %d rules, want >= 1", len(rules))
		}
		if rules[0].ID == "" || rules[0].Protocol == "" {
			t.Errorf("forwards[0] missing id/protocol: %+v", rules[0])
		}
		for _, a := range rules[0].Advisories {
			if a.Code == "" || a.Severity == "" || a.Message == "" {
				t.Errorf("advisory missing fields: %+v", a)
			}
		}
	}

	// GET /api/status → []ForwardStatus
	{
		var statuses []client.ForwardStatus
		decodeStrict(t, p("status.json"), &statuses)
		if len(statuses) < 1 {
			t.Fatalf("status list has %d entries, want >= 1", len(statuses))
		}
		if statuses[0].RuleID == "" {
			t.Errorf("status[0] missing ruleId: %+v", statuses[0])
		}
	}

	// GET /api/activity → { events: [ActivityEvent] }
	{
		var resp struct {
			Events []client.ActivityEvent `json:"events"`
		}
		decodeStrict(t, p("activity.json"), &resp)
		if len(resp.Events) < 1 {
			t.Fatalf("activity has %d events, want >= 1 (rule.created emitted by capture)", len(resp.Events))
		}
		e := resp.Events[0]
		if e.Type == "" || e.Severity == "" || e.Message == "" {
			t.Errorf("activity event missing type/severity/message: %+v", e)
		}
	}

	// GET /api/config/export → ConfigExportResponse
	{
		var cfg client.ConfigExportResponse
		decodeStrict(t, p("export.json"), &cfg)
		if cfg.Version != "1" {
			t.Errorf("export.version = %q, want %q", cfg.Version, "1")
		}
		if len(cfg.Rules) < 1 {
			t.Errorf("export.rules has %d rules, want >= 1", len(cfg.Rules))
		}
	}

	// POST /api/config/plan → ConfigPlanResponse (desired == current → unchanged)
	{
		var plan client.ConfigPlanResponse
		decodeStrict(t, p("plan.json"), &plan)
		if plan.Mode != "plan" {
			t.Errorf("plan.mode = %q, want %q", plan.Mode, "plan")
		}
		if plan.Summary.HasDrift {
			t.Errorf("plan.summary.hasDrift = true, want false (desired equals current)")
		}
		if plan.Summary.Unchanged < 1 {
			t.Errorf("plan.summary.unchanged = %d, want >= 1", plan.Summary.Unchanged)
		}
		if len(plan.Operations) < 1 || plan.Operations[0].Type != "unchanged" {
			t.Errorf("plan.operations[0] = %+v, want type unchanged", plan.Operations)
		}
		if plan.Errors == nil || plan.Warnings == nil {
			t.Error("plan.errors/warnings decoded as nil, want slices")
		}
	}

	// POST /api/config/apply (dry-run) → ConfigApplyResponse
	{
		var apply client.ConfigApplyResponse
		decodeStrict(t, p("apply-dryrun.json"), &apply)
		if !apply.DryRun {
			t.Error("apply.dryRun = false, want true")
		}
		if !apply.Ok {
			t.Error("apply.ok = false, want true")
		}
		if apply.Plan.Mode != "plan" {
			t.Errorf("apply.plan.mode = %q, want %q", apply.Plan.Mode, "plan")
		}
		// applied counts decode into the typed struct (no drift → all zero/unchanged).
		if apply.Applied.Unchanged < 1 {
			t.Errorf("apply.applied.unchanged = %d, want >= 1", apply.Applied.Unchanged)
		}
	}

	// GET /api/ports/advisory (forward, 0.0.0.0) → []PortAdvisory incl. LAN_EXPOSURE
	{
		var advisories []client.PortAdvisory
		decodeStrict(t, p("advisory-lan.json"), &advisories)
		if !hasAdvisory(advisories, "LAN_EXPOSURE", "warning") {
			t.Errorf("advisory-lan missing LAN_EXPOSURE/warning: %+v", advisories)
		}
	}

	// GET /api/ports/advisory (management, 0.0.0.0) → []PortAdvisory incl. MANAGEMENT_LAN_EXPOSURE
	{
		var advisories []client.PortAdvisory
		decodeStrict(t, p("advisory-mgmt.json"), &advisories)
		if !hasAdvisory(advisories, "MANAGEMENT_LAN_EXPOSURE", "danger") {
			t.Errorf("advisory-mgmt missing MANAGEMENT_LAN_EXPOSURE/danger: %+v", advisories)
		}
	}

	// POST /api/forwards/groups/:group/start → GroupActionResponse
	{
		var ga client.GroupActionResponse
		decodeStrict(t, p("group-start.json"), &ga)
		if ga.Group != "cli-contract" || ga.Action != "start" {
			t.Errorf("group-start = {group:%q action:%q}, want {cli-contract start}", ga.Group, ga.Action)
		}
		if ga.Total < 1 || ga.Succeeded < 1 {
			t.Errorf("group-start counts = %+v, want total>=1 succeeded>=1", ga)
		}
		if len(ga.Results) < 1 || ga.Results[0].RuleID == "" || ga.Results[0].Status == "" {
			t.Errorf("group-start results missing fields: %+v", ga.Results)
		}
	}

	// POST /api/forwards/:id/diagnose → RuleDiagnosticsResult (captured only when present)
	if _, err := os.Stat(p("diagnose.json")); err == nil {
		var diag client.RuleDiagnosticsResult
		decodeStrict(t, p("diagnose.json"), &diag)
		if diag.RuleID == "" || diag.Protocol == "" {
			t.Errorf("diagnose missing ruleId/protocol: %+v", diag)
		}
		if diag.Summary.Status == "" {
			t.Error("diagnose.summary.status is empty")
		}
		if len(diag.Checks) < 1 {
			t.Error("diagnose.checks is empty, want >= 1 check")
		}
	}
}

func hasAdvisory(advisories []client.PortAdvisory, code, severity string) bool {
	for _, a := range advisories {
		if a.Code == code && a.Severity == severity && a.Message != "" {
			return true
		}
	}
	return false
}
