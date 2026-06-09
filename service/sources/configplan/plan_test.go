package configplan

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"portier/service/sources/domain"
)

var fixedNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func buildPlan(currentRules []domain.ForwardRule, desired string) Response {
	return BuildConfigPlan(Input{
		CurrentRules: currentRules,
		DesiredRaw:   json.RawMessage(desired),
		Now:          &fixedNow,
	})
}

func tcpRule(id, name string, listenPort, targetPort int) domain.ForwardRule {
	return domain.ForwardRule{
		ID:         id,
		Name:       name,
		Protocol:   domain.ProtocolTCP,
		ListenHost: "127.0.0.1",
		ListenPort: listenPort,
		TargetHost: "127.0.0.1",
		TargetPort: targetPort,
		Enabled:    true,
	}
}

func udpRule(id, name string, listenPort, targetPort int, mode domain.UdpMode) domain.ForwardRule {
	m := mode
	return domain.ForwardRule{
		ID:         id,
		Name:       name,
		Protocol:   domain.ProtocolUDP,
		ListenHost: "127.0.0.1",
		ListenPort: listenPort,
		TargetHost: "127.0.0.1",
		TargetPort: targetPort,
		Enabled:    true,
		UdpMode:    &m,
	}
}

// ── generatedAt ──────────────────────────────────────────────────────────────

func TestGeneratedAtIsSet(t *testing.T) {
	resp := buildPlan(nil, "[]")
	if resp.GeneratedAt == "" {
		t.Fatal("generatedAt must not be empty")
	}
	want := "2026-01-01T00:00:00.000Z"
	if resp.GeneratedAt != want {
		t.Fatalf("generatedAt = %q, want %q", resp.GeneratedAt, want)
	}
}

func TestModeIsPlan(t *testing.T) {
	resp := buildPlan(nil, "[]")
	if resp.Mode != "plan" {
		t.Fatalf("mode = %q, want plan", resp.Mode)
	}
}

// ── Empty inputs ─────────────────────────────────────────────────────────────

func TestEmptyCurrentEmptyDesiredNoDrift(t *testing.T) {
	resp := buildPlan(nil, "[]")
	if resp.Summary.HasDrift {
		t.Fatal("expected no drift for empty current + empty desired")
	}
	if resp.Summary.HasErrors {
		t.Fatal("expected no errors")
	}
	if len(resp.Operations) != 0 {
		t.Fatalf("expected 0 operations, got %d", len(resp.Operations))
	}
	if resp.Errors == nil {
		t.Fatal("errors must be array, not null")
	}
	if resp.Warnings == nil {
		t.Fatal("warnings must be array, not null")
	}
	if resp.Operations == nil {
		t.Fatal("operations must be array, not null")
	}
}

func TestEmptyCurrentWithObjectDesiredNoDrift(t *testing.T) {
	resp := buildPlan(nil, `{"rules":[]}`)
	if resp.Summary.HasDrift {
		t.Fatal("expected no drift for object form with empty rules")
	}
	if len(resp.Operations) != 0 {
		t.Fatalf("expected 0 operations, got %d", len(resp.Operations))
	}
}

// ── Add operation ─────────────────────────────────────────────────────────────

func TestAddOperation(t *testing.T) {
	resp := buildPlan(nil, `[{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if resp.Summary.Add != 1 {
		t.Fatalf("add = %d, want 1", resp.Summary.Add)
	}
	if !resp.Summary.HasDrift {
		t.Fatal("hasDrift must be true when there is an add")
	}
	if len(resp.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(resp.Operations))
	}
	op := resp.Operations[0]
	if op.Type != "add" {
		t.Fatalf("type = %q, want add", op.Type)
	}
	if op.Destructive {
		t.Fatal("add must not be destructive")
	}
	if op.RuleID != nil {
		t.Fatal("add must not have ruleId")
	}
	if op.Current != nil {
		t.Fatal("add must not have current snapshot")
	}
	if op.Desired == nil {
		t.Fatal("add must have desired snapshot")
	}
	if op.RuleName != "App" {
		t.Fatalf("ruleName = %q, want App", op.RuleName)
	}
	if op.Protocol != domain.ProtocolTCP {
		t.Fatalf("protocol = %q, want tcp", op.Protocol)
	}
}

// ── Remove operation ─────────────────────────────────────────────────────────

func TestRemoveOperation(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, "[]")
	if resp.Summary.Remove != 1 {
		t.Fatalf("remove = %d, want 1", resp.Summary.Remove)
	}
	if !resp.Summary.HasDrift {
		t.Fatal("hasDrift must be true when there is a remove")
	}
	op := resp.Operations[0]
	if op.Type != "remove" {
		t.Fatalf("type = %q, want remove", op.Type)
	}
	if !op.Destructive {
		t.Fatal("remove must be destructive")
	}
	if op.RuleID == nil || *op.RuleID != "r1" {
		t.Fatalf("ruleId = %v, want r1", op.RuleID)
	}
	if op.Current == nil {
		t.Fatal("remove must have current snapshot")
	}
	if op.Desired != nil {
		t.Fatal("remove must not have desired snapshot")
	}
}

func TestRemoveProducesRemoveExistingWarning(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, "[]")
	if len(resp.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(resp.Warnings))
	}
	if resp.Warnings[0].Code != "REMOVE_EXISTING" {
		t.Fatalf("warning code = %q, want REMOVE_EXISTING", resp.Warnings[0].Code)
	}
}

// ── Unchanged operation ──────────────────────────────────────────────────────

func TestUnchangedOperation(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, `[{"id":"r1","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if resp.Summary.Unchanged != 1 {
		t.Fatalf("unchanged = %d, want 1", resp.Summary.Unchanged)
	}
	if resp.Summary.HasDrift {
		t.Fatal("hasDrift must be false for unchanged only")
	}
	op := resp.Operations[0]
	if op.Type != "unchanged" {
		t.Fatalf("type = %q, want unchanged", op.Type)
	}
	if op.Destructive {
		t.Fatal("unchanged must not be destructive")
	}
	if op.Changes != nil {
		t.Fatal("unchanged must not have changes")
	}
	if op.Current == nil || op.Desired == nil {
		t.Fatal("unchanged must have both current and desired snapshots")
	}
}

// ── Update operation ─────────────────────────────────────────────────────────

func TestUpdateNameOnlyIsNonDestructive(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, `[{"id":"r1","name":"Renamed","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if resp.Summary.Update != 1 {
		t.Fatalf("update = %d, want 1", resp.Summary.Update)
	}
	op := resp.Operations[0]
	if op.Type != "update" {
		t.Fatalf("type = %q, want update", op.Type)
	}
	if op.Destructive {
		t.Fatal("name-only update must not be destructive")
	}
	if len(op.Changes) != 1 || op.Changes[0].Field != "name" {
		t.Fatalf("changes = %v, want [{name ...}]", op.Changes)
	}
	if op.Changes[0].Before != "App" || op.Changes[0].After != "Renamed" {
		t.Fatalf("change before/after = %v / %v, want App / Renamed", op.Changes[0].Before, op.Changes[0].After)
	}
}

func TestUpdateEnabledOnlyIsNonDestructive(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, `[{"id":"r1","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":false}]`)
	op := resp.Operations[0]
	if op.Destructive {
		t.Fatal("enabled-only update must not be destructive")
	}
	if len(op.Changes) != 1 || op.Changes[0].Field != "enabled" {
		t.Fatalf("changes = %v", op.Changes)
	}
}

func TestUpdateForwardingFieldIsDestructive(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	// change targetPort — a forwarding field
	resp := buildPlan(current, `[{"id":"r1","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9999,"enabled":true}]`)
	op := resp.Operations[0]
	if !op.Destructive {
		t.Fatal("targetPort change must be destructive")
	}
	if resp.Summary.Destructive != 1 {
		t.Fatalf("destructive = %d, want 1", resp.Summary.Destructive)
	}
}

func TestUpdateListenPortIsDestructive(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, `[{"id":"r1","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9999,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	op := resp.Operations[0]
	if !op.Destructive {
		t.Fatal("listenPort change must be destructive")
	}
}

// ── Material fields ──────────────────────────────────────────────────────────

func TestUpdateChangesContainFieldBeforeAfter(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, `[{"id":"r1","name":"Renamed","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	changes := resp.Operations[0].Changes
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Field != "name" {
		t.Fatalf("field = %q, want name", changes[0].Field)
	}
	if changes[0].Before != "App" {
		t.Fatalf("before = %v, want App", changes[0].Before)
	}
	if changes[0].After != "Renamed" {
		t.Fatalf("after = %v, want Renamed", changes[0].After)
	}
}

func TestMultipleMaterialFieldChanges(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, `[{"id":"r1","name":"New","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"10.0.0.1","targetPort":8080,"enabled":false}]`)
	changes := resp.Operations[0].Changes
	fields := make(map[string]bool)
	for _, c := range changes {
		fields[c.Field] = true
	}
	for _, expected := range []string{"name", "targetHost", "targetPort", "enabled"} {
		if !fields[expected] {
			t.Fatalf("expected change for field %q", expected)
		}
	}
}

func TestListenPortChangeValue(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, `[{"id":"r1","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9999,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	var listenPortChange *Change
	for i, c := range resp.Operations[0].Changes {
		if c.Field == "listenPort" {
			listenPortChange = &resp.Operations[0].Changes[i]
		}
	}
	if listenPortChange == nil {
		t.Fatal("expected listenPort change")
	}
	if listenPortChange.Before != 9001 {
		t.Fatalf("listenPort before = %v, want 9001", listenPortChange.Before)
	}
	if listenPortChange.After != 9999 {
		t.Fatalf("listenPort after = %v, want 9999", listenPortChange.After)
	}
}

// ── Matching semantics ────────────────────────────────────────────────────────

func TestMatchByIDWhenProvided(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	// desired has same id, different name — should be "update", not "add"
	resp := buildPlan(current, `[{"id":"r1","name":"Changed","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if resp.Summary.Add != 0 || resp.Summary.Update != 1 {
		t.Fatalf("expected update, got add=%d update=%d", resp.Summary.Add, resp.Summary.Update)
	}
}

func TestMatchByIdentityKeyWhenNoID(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	// desired has no id, same protocol+listenHost+listenPort → identity key match
	resp := buildPlan(current, `[{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if resp.Summary.Unchanged != 1 {
		t.Fatalf("expected identity key match → unchanged, got: add=%d update=%d unchanged=%d", resp.Summary.Add, resp.Summary.Update, resp.Summary.Unchanged)
	}
}

func TestIDMatchWinsOverIdentityKey(t *testing.T) {
	// Two current rules with same id as desired, different identity key
	r1 := tcpRule("r1", "App", 9001, 9002)
	r2 := tcpRule("r2", "Other", 9003, 9004)
	current := []domain.ForwardRule{r1, r2}
	// desired: id=r1, but identity key matches r2 (listenPort=9003)
	// id-first matching should match r1, not r2
	resp := buildPlan(current, `[{"id":"r1","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9003,"targetHost":"127.0.0.1","targetPort":9004,"enabled":true}]`)
	// r1 is matched and updated (listenPort changed), r2 is removed
	if resp.Summary.Update != 1 || resp.Summary.Remove != 1 {
		t.Fatalf("expected update+remove, got: %+v", resp.Summary)
	}
	updateOp := resp.Operations[0]
	if updateOp.RuleID == nil || *updateOp.RuleID != "r1" {
		t.Fatalf("update ruleId = %v, want r1", updateOp.RuleID)
	}
}

func TestNoFuzzyNameMatching(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	// desired has same name but different identity key and no id — no match
	resp := buildPlan(current, `[{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9999,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	// r1 unmatched (remove), desired is add
	if resp.Summary.Add != 1 || resp.Summary.Remove != 1 {
		t.Fatalf("expected add+remove when name matches but key doesn't, got: %+v", resp.Summary)
	}
}

func TestUnknownIDProducesAdd(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, `[{"id":"unknown","name":"New","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9003,"targetHost":"127.0.0.1","targetPort":9004,"enabled":true}]`)
	// unknown id → add; r1 unmatched → remove
	if resp.Summary.Add != 1 || resp.Summary.Remove != 1 {
		t.Fatalf("expected add+remove, got: %+v", resp.Summary)
	}
}

// ── Duplicate detection ───────────────────────────────────────────────────────

func TestDuplicateDesiredIDError(t *testing.T) {
	resp := buildPlan(nil, `[
		{"id":"r1","name":"A","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true},
		{"id":"r1","name":"B","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9003,"targetHost":"127.0.0.1","targetPort":9004,"enabled":true}
	]`)
	if !resp.Summary.HasErrors {
		t.Fatal("expected hasErrors for duplicate id")
	}
	found := false
	for _, e := range resp.Errors {
		if e.Code == "DUPLICATE_DESIRED_ID" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected DUPLICATE_DESIRED_ID error, got: %+v", resp.Errors)
	}
}

func TestDuplicateDesiredIdentityKeyError(t *testing.T) {
	resp := buildPlan(nil, `[
		{"name":"A","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true},
		{"name":"B","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9004,"enabled":true}
	]`)
	if !resp.Summary.HasErrors {
		t.Fatal("expected hasErrors for duplicate identity key")
	}
	found := false
	for _, e := range resp.Errors {
		if e.Code == "DUPLICATE_DESIRED_IDENTITY_KEY" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected DUPLICATE_DESIRED_IDENTITY_KEY error, got: %+v", resp.Errors)
	}
}

func TestAmbiguousCurrentMatchError(t *testing.T) {
	r1 := tcpRule("r1", "App1", 9001, 9002)
	r2 := tcpRule("r2", "App2", 9001, 9003)
	// Two current rules with the same identity key (shouldn't happen in practice, but defensively handled)
	current := []domain.ForwardRule{r1, r2}
	// Force identical keys by overriding ID-based match: desired has no id
	resp := buildPlan(current, `[{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if !resp.Summary.HasErrors {
		t.Fatal("expected hasErrors for ambiguous match")
	}
	found := false
	for _, e := range resp.Errors {
		if e.Code == "AMBIGUOUS_CURRENT_MATCH" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected AMBIGUOUS_CURRENT_MATCH error, got: %+v", resp.Errors)
	}
}

func TestAmbiguousMatchDoesNotAddLANExposureWarning(t *testing.T) {
	r1 := tcpRule("r1", "App1", 9001, 9002)
	r1.ListenHost = "0.0.0.0"
	r2 := tcpRule("r2", "App2", 9001, 9003)
	r2.ListenHost = "0.0.0.0"
	current := []domain.ForwardRule{r1, r2}
	resp := buildPlan(current, `[{"name":"App","protocol":"tcp","listenHost":"0.0.0.0","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	// Should have AMBIGUOUS_CURRENT_MATCH error but NOT LAN_EXPOSURE (continue skips it)
	for _, w := range resp.Warnings {
		if w.Code == "LAN_EXPOSURE" {
			t.Fatal("LAN_EXPOSURE must not be emitted when AMBIGUOUS_CURRENT_MATCH causes continue")
		}
	}
}

// ── Invalid desired config ────────────────────────────────────────────────────

func TestInvalidDesiredConfigShape(t *testing.T) {
	for _, raw := range []string{"null", "42", `"hello"`, "{}"} {
		resp := buildPlan(nil, raw)
		if !resp.Summary.HasErrors {
			t.Fatalf("input %q: expected hasErrors for invalid config shape", raw)
		}
		if resp.Errors[0].Code != "INVALID_DESIRED_CONFIG" {
			t.Fatalf("input %q: expected INVALID_DESIRED_CONFIG, got %q", raw, resp.Errors[0].Code)
		}
	}
}

func TestInvalidDesiredRuleMissingName(t *testing.T) {
	resp := buildPlan(nil, `[{"protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if !resp.Summary.HasErrors {
		t.Fatal("expected hasErrors for rule missing name")
	}
	if resp.Errors[0].Code != "INVALID_DESIRED_RULE" {
		t.Fatalf("expected INVALID_DESIRED_RULE, got %q", resp.Errors[0].Code)
	}
}

func TestInvalidDesiredRuleInvalidProtocol(t *testing.T) {
	resp := buildPlan(nil, `[{"name":"App","protocol":"ftp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if !resp.Summary.HasErrors {
		t.Fatal("expected hasErrors for invalid protocol")
	}
	if resp.Errors[0].Code != "INVALID_DESIRED_RULE" {
		t.Fatalf("expected INVALID_DESIRED_RULE, got %q", resp.Errors[0].Code)
	}
}

func TestInvalidDesiredRuleInvalidPort(t *testing.T) {
	resp := buildPlan(nil, `[{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":99999,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if !resp.Summary.HasErrors {
		t.Fatal("expected hasErrors for invalid port")
	}
}

func TestInvalidDesiredRuleInvalidUDPMode(t *testing.T) {
	resp := buildPlan(nil, `[{"name":"App","protocol":"udp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true,"udpMode":"invalid-mode"}]`)
	if !resp.Summary.HasErrors {
		t.Fatal("expected hasErrors for invalid udpMode")
	}
}

func TestInvalidDesiredRuleUDPModeOnTCP(t *testing.T) {
	resp := buildPlan(nil, `[{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true,"udpMode":"one-way"}]`)
	if !resp.Summary.HasErrors {
		t.Fatal("expected hasErrors for udpMode on TCP rule")
	}
}

func TestInvalidDesiredRuleIndexInErrorField(t *testing.T) {
	resp := buildPlan(nil, `[{"protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if len(resp.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if resp.Errors[0].Field == nil || *resp.Errors[0].Field != "rules[0]" {
		t.Fatalf("field = %v, want rules[0]", resp.Errors[0].Field)
	}
}

// ── Warnings ──────────────────────────────────────────────────────────────────

func TestLANExposureWarningForZeroZeroZeroZero(t *testing.T) {
	resp := buildPlan(nil, `[{"name":"App","protocol":"tcp","listenHost":"0.0.0.0","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	found := false
	for _, w := range resp.Warnings {
		if w.Code == "LAN_EXPOSURE" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected LAN_EXPOSURE warning for 0.0.0.0 listenHost")
	}
}

func TestNoLANExposureWarningForLoopback(t *testing.T) {
	resp := buildPlan(nil, `[{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	for _, w := range resp.Warnings {
		if w.Code == "LAN_EXPOSURE" {
			t.Fatal("unexpected LAN_EXPOSURE for loopback listenHost")
		}
	}
}

// ── Summary fields ────────────────────────────────────────────────────────────

func TestSummaryCounts(t *testing.T) {
	r1 := tcpRule("r1", "Unchanged", 9001, 9002)
	r2 := tcpRule("r2", "Removed", 9003, 9004)
	current := []domain.ForwardRule{r1, r2}
	// r1 unchanged, r2 removed, one new rule added, one updated by name
	resp := buildPlan(current, `[
		{"id":"r1","name":"Unchanged","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true},
		{"name":"NewRule","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9005,"targetHost":"127.0.0.1","targetPort":9006,"enabled":true}
	]`)
	if resp.Summary.Add != 1 {
		t.Fatalf("add = %d, want 1", resp.Summary.Add)
	}
	if resp.Summary.Remove != 1 {
		t.Fatalf("remove = %d, want 1", resp.Summary.Remove)
	}
	if resp.Summary.Unchanged != 1 {
		t.Fatalf("unchanged = %d, want 1", resp.Summary.Unchanged)
	}
	if resp.Summary.Update != 0 {
		t.Fatalf("update = %d, want 0", resp.Summary.Update)
	}
}

func TestHasDriftFalseWhenAllUnchanged(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, `[{"id":"r1","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if resp.Summary.HasDrift {
		t.Fatal("hasDrift must be false when all rules are unchanged")
	}
}

func TestDestructiveCountIncludesRemoves(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, "[]")
	if resp.Summary.Destructive != 1 {
		t.Fatalf("destructive = %d, want 1", resp.Summary.Destructive)
	}
}

func TestDestructiveCountIncludesForwardingUpdates(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, `[{"id":"r1","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9999,"enabled":true}]`)
	if resp.Summary.Destructive != 1 {
		t.Fatalf("destructive = %d, want 1", resp.Summary.Destructive)
	}
}

// ── UDP rules ─────────────────────────────────────────────────────────────────

func TestUDPRuleUnchanged(t *testing.T) {
	current := []domain.ForwardRule{udpRule("u1", "UDP App", 9001, 9002, domain.UdpModeOneWay)}
	resp := buildPlan(current, `[{"id":"u1","name":"UDP App","protocol":"udp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true,"udpMode":"one-way"}]`)
	if resp.Summary.Unchanged != 1 {
		t.Fatalf("expected unchanged UDP rule, got: %+v", resp.Summary)
	}
}

func TestUDPModeChangeIsDestructive(t *testing.T) {
	current := []domain.ForwardRule{udpRule("u1", "UDP App", 9001, 9002, domain.UdpModeOneWay)}
	resp := buildPlan(current, `[{"id":"u1","name":"UDP App","protocol":"udp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true,"udpMode":"bidirectional-last-client"}]`)
	if resp.Summary.Update != 1 {
		t.Fatalf("expected update, got: %+v", resp.Summary)
	}
	if !resp.Operations[0].Destructive {
		t.Fatal("udpMode change must be destructive")
	}
}

func TestUDPDefaultModeIsOneWay(t *testing.T) {
	// Desired UDP rule without explicit udpMode → defaults to one-way
	current := []domain.ForwardRule{udpRule("u1", "UDP App", 9001, 9002, domain.UdpModeOneWay)}
	resp := buildPlan(current, `[{"id":"u1","name":"UDP App","protocol":"udp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if resp.Summary.Unchanged != 1 {
		t.Fatalf("UDP rule with no udpMode should default to one-way and be unchanged, got: %+v", resp.Summary)
	}
}

// ── Desired with object rules array ──────────────────────────────────────────

func TestDesiredAsObjectWithRulesArray(t *testing.T) {
	resp := buildPlan(nil, `{"rules":[{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]}`)
	if resp.Summary.Add != 1 {
		t.Fatalf("expected 1 add from object form, got: %+v", resp.Summary)
	}
}

// ── Transient fields are ignored ──────────────────────────────────────────────

func TestTransientFieldsInDesiredAreIgnored(t *testing.T) {
	// Desired rule with extra unknown fields — should be tolerated, not error
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	resp := buildPlan(current, `[{"id":"r1","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true,"running":true,"lastError":"ignored"}]`)
	if resp.Summary.HasErrors {
		t.Fatalf("unexpected errors for rule with extra fields: %v", resp.Errors)
	}
	if resp.Summary.Unchanged != 1 {
		t.Fatalf("expected unchanged, got: %+v", resp.Summary)
	}
}

// ── extractRulesRaw ──────────────────────────────────────────────────────────

func TestExtractRulesRawArray(t *testing.T) {
	rules, ok := ExtractRulesRaw(json.RawMessage(`[{"name":"A"},{"name":"B"}]`))
	if !ok {
		t.Fatal("expected ok=true for array input")
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestExtractRulesRawEmptyArray(t *testing.T) {
	rules, ok := ExtractRulesRaw(json.RawMessage(`[]`))
	if !ok {
		t.Fatal("expected ok=true for empty array")
	}
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(rules))
	}
}

func TestExtractRulesRawObjectWithRules(t *testing.T) {
	rules, ok := ExtractRulesRaw(json.RawMessage(`{"rules":[{"name":"A"}]}`))
	if !ok {
		t.Fatal("expected ok=true for object with rules array")
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

func TestExtractRulesRawNull(t *testing.T) {
	_, ok := ExtractRulesRaw(json.RawMessage(`null`))
	if ok {
		t.Fatal("expected ok=false for null")
	}
}

func TestExtractRulesRawNumber(t *testing.T) {
	_, ok := ExtractRulesRaw(json.RawMessage(`42`))
	if ok {
		t.Fatal("expected ok=false for number input")
	}
}

func TestExtractRulesRawEmptyObject(t *testing.T) {
	_, ok := ExtractRulesRaw(json.RawMessage(`{}`))
	if ok {
		t.Fatal("expected ok=false for object without rules")
	}
}

func TestExtractRulesRawNullRules(t *testing.T) {
	_, ok := ExtractRulesRaw(json.RawMessage(`{"rules":null}`))
	if ok {
		t.Fatal("expected ok=false for object with null rules")
	}
}

// ── BuildApplyImportFromPlan — apply orchestration ────────────────────────────

// seqIDGen returns a deterministic id generator and a pointer to its call count.
func seqIDGen() (func() string, *int) {
	count := 0
	gen := func() string {
		count++
		return "gen-" + strconv.Itoa(count)
	}
	return gen, &count
}

func assertApplied(t *testing.T, applied map[string]int, add, update, remove, unchanged int) {
	t.Helper()
	want := map[string]int{"add": add, "update": update, "remove": remove, "unchanged": unchanged}
	for k, v := range want {
		if applied[k] != v {
			t.Errorf("applied[%q] = %d, want %d (full: %#v)", k, applied[k], v, applied)
		}
	}
}

func TestBuildApplyImportNoDriftPreservesCurrentID(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	// Desired matches by identity key (same protocol+host+port), no explicit id.
	plan := buildPlan(current, `[{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if plan.Summary.HasDrift {
		t.Fatalf("expected no drift, got summary %#v", plan.Summary)
	}

	gen, count := seqIDGen()
	result := BuildApplyImportFromPlan(plan, gen)
	if len(result.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(result.Rules))
	}
	if result.Rules[0].ID != "r1" {
		t.Errorf("id = %q, want current id %q", result.Rules[0].ID, "r1")
	}
	if *count != 0 {
		t.Errorf("newID called %d times for an unchanged match, want 0", *count)
	}
	assertApplied(t, result.Applied, 0, 0, 0, 1)
}

func TestBuildApplyImportAddInjectsGeneratedID(t *testing.T) {
	plan := buildPlan(nil, `[{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if plan.Operations[0].Type != "add" {
		t.Fatalf("expected add op, got %q", plan.Operations[0].Type)
	}

	gen, _ := seqIDGen()
	result := BuildApplyImportFromPlan(plan, gen)
	if len(result.Rules) != 1 || result.Rules[0].ID != "gen-1" {
		t.Fatalf("expected one rule with id gen-1, got %#v", result.Rules)
	}
	assertApplied(t, result.Applied, 1, 0, 0, 0)
}

func TestBuildApplyImportAddWithExplicitIDPreservesIt(t *testing.T) {
	plan := buildPlan(nil, `[{"id":"explicit-id","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9002,"enabled":true}]`)
	if plan.Operations[0].Type != "add" {
		t.Fatalf("expected add op, got %q", plan.Operations[0].Type)
	}

	gen, count := seqIDGen()
	result := BuildApplyImportFromPlan(plan, gen)
	if result.Rules[0].ID != "explicit-id" {
		t.Errorf("id = %q, want %q", result.Rules[0].ID, "explicit-id")
	}
	if *count != 0 {
		t.Errorf("newID called %d times when explicit id present, want 0", *count)
	}
}

func TestBuildApplyImportUpdatePreservesCurrentID(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	// Same identity key, changed forwarding field (targetPort) → update.
	plan := buildPlan(current, `[{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9999,"enabled":true}]`)
	if plan.Operations[0].Type != "update" {
		t.Fatalf("expected update op, got %q", plan.Operations[0].Type)
	}

	gen, _ := seqIDGen()
	result := BuildApplyImportFromPlan(plan, gen)
	if result.Rules[0].ID != "r1" {
		t.Errorf("id = %q, want preserved current id %q", result.Rules[0].ID, "r1")
	}
	if result.Rules[0].TargetPort != 9999 {
		t.Errorf("targetPort = %d, want 9999", result.Rules[0].TargetPort)
	}
	assertApplied(t, result.Applied, 0, 1, 0, 0)
}

func TestBuildApplyImportRemoveOmitted(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	plan := buildPlan(current, `[]`)
	if plan.Operations[0].Type != "remove" {
		t.Fatalf("expected remove op, got %q", plan.Operations[0].Type)
	}

	gen, _ := seqIDGen()
	result := BuildApplyImportFromPlan(plan, gen)
	if len(result.Rules) != 0 {
		t.Fatalf("expected 0 rules (remove omitted), got %#v", result.Rules)
	}
	assertApplied(t, result.Applied, 0, 0, 1, 0)
}

func TestBuildApplyImportMixedDeterministicOrder(t *testing.T) {
	current := []domain.ForwardRule{
		tcpRule("r1", "App", 9001, 9002),
		udpRule("r2", "Udp", 9003, 9004, domain.UdpModeOneWay),
	}
	// r1 updated, r2 unchanged, one new rule added — nothing removed.
	desired := `[` +
		`{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9999,"enabled":true},` +
		`{"name":"Udp","protocol":"udp","listenHost":"127.0.0.1","listenPort":9003,"targetHost":"127.0.0.1","targetPort":9004,"enabled":true,"udpMode":"one-way"},` +
		`{"name":"New","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9100,"targetHost":"127.0.0.1","targetPort":9200,"enabled":true}` +
		`]`
	plan := buildPlan(current, desired)

	gen, _ := seqIDGen()
	result := BuildApplyImportFromPlan(plan, gen)
	want := []string{"r1", "r2", "gen-1"}
	if len(result.Rules) != len(want) {
		t.Fatalf("got %d rules, want %d (%#v)", len(result.Rules), len(want), result.Rules)
	}
	for i := range want {
		if result.Rules[i].ID != want[i] {
			gotIDs := make([]string, len(result.Rules))
			for j, r := range result.Rules {
				gotIDs[j] = r.ID
			}
			t.Fatalf("ids = %#v, want %#v", gotIDs, want)
		}
	}
	assertApplied(t, result.Applied, 1, 1, 0, 1)
}

func TestBuildApplyImportDoesNotMutatePlan(t *testing.T) {
	current := []domain.ForwardRule{tcpRule("r1", "App", 9001, 9002)}
	plan := buildPlan(current, `[{"name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":9001,"targetHost":"127.0.0.1","targetPort":9999,"enabled":true}]`)
	before, _ := json.Marshal(plan)

	gen, _ := seqIDGen()
	_ = BuildApplyImportFromPlan(plan, gen)
	after, _ := json.Marshal(plan)
	if string(before) != string(after) {
		t.Errorf("plan mutated:\n before: %s\n after:  %s", before, after)
	}
}

func TestBuildApplyImportPreservesUdpMode(t *testing.T) {
	current := []domain.ForwardRule{udpRule("r2", "Udp", 9003, 9004, domain.UdpModeOneWay)}
	plan := buildPlan(current, `[{"name":"Udp","protocol":"udp","listenHost":"127.0.0.1","listenPort":9003,"targetHost":"127.0.0.1","targetPort":9004,"enabled":false,"udpMode":"bidirectional-multi-client"}]`)
	gen, _ := seqIDGen()
	result := BuildApplyImportFromPlan(plan, gen)
	if result.Rules[0].UdpMode == nil || *result.Rules[0].UdpMode != domain.UdpModeBidirectionalMulti {
		t.Errorf("udpMode = %v, want bidirectional-multi-client", result.Rules[0].UdpMode)
	}
}
