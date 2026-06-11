package validation

import (
	"strings"
	"testing"

	"portier/service/sources/domain"
)

func TestValidTCPRule(t *testing.T) {
	rule, errors := ValidateForwardRuleInput(validInput(domain.ProtocolTCP))
	if len(errors) > 0 {
		t.Fatalf("expected valid rule, got errors: %#v", errors)
	}
	if rule.Protocol != domain.ProtocolTCP {
		t.Fatalf("protocol = %q, want tcp", rule.Protocol)
	}
	if rule.UdpMode != nil {
		t.Fatalf("tcp rule udpMode = %v, want nil", *rule.UdpMode)
	}
}

func TestValidUDPRuleDefaultsMode(t *testing.T) {
	input := validInput(domain.ProtocolUDP)
	rule, errors := ValidateForwardRuleInput(input)
	if len(errors) > 0 {
		t.Fatalf("expected valid rule, got errors: %#v", errors)
	}
	if rule.UdpMode == nil || *rule.UdpMode != domain.UdpModeOneWay {
		t.Fatalf("udpMode = %v, want one-way", rule.UdpMode)
	}
}

func TestInvalidProtocol(t *testing.T) {
	input := validInput(domain.ForwardProtocol("http"))
	_, errors := ValidateForwardRuleInput(input)
	if !hasError(errors, "protocol must be tcp or udp.") {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestInvalidPort(t *testing.T) {
	input := validInput(domain.ProtocolTCP)
	badPort := 70000
	input.ListenPort = &badPort
	_, errors := ValidateForwardRuleInput(input)
	if !hasError(errors, "listenPort must be an integer from 1 to 65535.") {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestMissingRequiredFields(t *testing.T) {
	_, errors := ValidateForwardRuleInput(ForwardRuleInput{})
	expected := []string{
		"id is required.",
		"name is required.",
		"protocol must be tcp or udp.",
		"listenHost is required.",
		"listenPort must be an integer from 1 to 65535.",
		"targetHost is required.",
		"targetPort must be an integer from 1 to 65535.",
		"enabled must be a boolean.",
	}
	for _, message := range expected {
		if !hasError(errors, message) {
			t.Fatalf("expected error %q in %#v", message, errors)
		}
	}
}

func TestUDPModeValidation(t *testing.T) {
	input := validInput(domain.ProtocolUDP)
	badMode := domain.UdpMode("chatty")
	input.UdpMode = &badMode
	_, errors := ValidateForwardRuleInput(input)
	if !hasError(errors, "udpMode must be one-way, bidirectional-last-client, or bidirectional-multi-client.") {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestTCPRejectsUDPMode(t *testing.T) {
	input := validInput(domain.ProtocolTCP)
	mode := domain.UdpModeOneWay
	input.UdpMode = &mode
	_, errors := ValidateForwardRuleInput(input)
	if !hasError(errors, "udpMode is only valid for UDP rules.") {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestPatchDoesNotOverwriteAbsentStringFields(t *testing.T) {
	existing := ruleFromValidInput(t, validInput(domain.ProtocolTCP))
	enabled := false
	patch, errors := ValidateForwardRulePatch(ForwardRuleInput{Enabled: &enabled})
	if len(errors) > 0 {
		t.Fatalf("patch errors: %#v", errors)
	}

	next, errors := ApplyPatch(existing, patch)
	if len(errors) > 0 {
		t.Fatalf("apply errors: %#v", errors)
	}
	if next.Name != existing.Name || next.ListenHost != existing.ListenHost || next.TargetHost != existing.TargetHost {
		t.Fatalf("string fields were overwritten: %#v", next)
	}
	if next.Enabled {
		t.Fatal("enabled false patch was not applied")
	}
}

func TestPatchEnabledFalseIsDistinguishedFromAbsent(t *testing.T) {
	enabled := false
	patch, errors := ValidateForwardRulePatch(ForwardRuleInput{Enabled: &enabled})
	if len(errors) > 0 {
		t.Fatalf("patch errors: %#v", errors)
	}
	if patch.Enabled == nil {
		t.Fatal("enabled=false should be present in patch")
	}
	if *patch.Enabled {
		t.Fatal("enabled patch = true, want false")
	}
}

func TestPatchRejectsInvalidPresentValues(t *testing.T) {
	zero := 0
	empty := ""
	badProtocol := domain.ForwardProtocol("http")
	badMode := domain.UdpMode("chatty")
	_, errors := ValidateForwardRulePatch(ForwardRuleInput{
		Name:       &empty,
		Protocol:   &badProtocol,
		ListenHost: &empty,
		ListenPort: &zero,
		TargetHost: &empty,
		TargetPort: &zero,
		UdpMode:    &badMode,
	})
	if !hasError(errors, "name must be a non-empty string.") {
		t.Fatalf("errors = %#v", errors)
	}
	if !hasError(errors, "protocol must be tcp or udp.") {
		t.Fatalf("errors = %#v", errors)
	}
	if !hasError(errors, "listenHost must be a non-empty string.") {
		t.Fatalf("errors = %#v", errors)
	}
	if !hasError(errors, "listenPort must be an integer from 1 to 65535.") {
		t.Fatalf("errors = %#v", errors)
	}
	if !hasError(errors, "targetHost must be a non-empty string.") {
		t.Fatalf("errors = %#v", errors)
	}
	if !hasError(errors, "targetPort must be an integer from 1 to 65535.") {
		t.Fatalf("errors = %#v", errors)
	}
	if !hasError(errors, "udpMode must be one-way, bidirectional-last-client, or bidirectional-multi-client.") {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestValidateForwardRuleInputWithOptionalIDNoID(t *testing.T) {
	input := validInput(domain.ProtocolTCP)
	input.ID = nil
	rule, errors := ValidateForwardRuleInputWithOptionalID(input)
	if len(errors) > 0 {
		t.Fatalf("expected valid rule, got errors: %#v", errors)
	}
	if rule.ID != "" {
		t.Fatalf("expected empty ID, got %q", rule.ID)
	}
}

func TestValidateForwardRuleInputWithOptionalIDWithID(t *testing.T) {
	input := validInput(domain.ProtocolTCP)
	id := "rule-abc"
	input.ID = &id
	rule, errors := ValidateForwardRuleInputWithOptionalID(input)
	if len(errors) > 0 {
		t.Fatalf("expected valid rule, got errors: %#v", errors)
	}
	if rule.ID != "rule-abc" {
		t.Fatalf("ID = %q, want rule-abc", rule.ID)
	}
}

func TestValidateForwardRuleInputWithOptionalIDEmptyID(t *testing.T) {
	input := validInput(domain.ProtocolTCP)
	empty := "  "
	input.ID = &empty
	_, errors := ValidateForwardRuleInputWithOptionalID(input)
	if !hasError(errors, "id must be a non-empty string when provided.") {
		t.Fatalf("expected empty-id error, got: %#v", errors)
	}
}

func TestDecodeAndValidateForwardRule(t *testing.T) {
	raw := []byte(`{"id":"r1","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48001,"targetHost":"127.0.0.1","targetPort":3000,"enabled":true}`)
	rule, errors := DecodeAndValidateForwardRule(raw)
	if len(errors) > 0 {
		t.Fatalf("expected valid rule, got errors: %#v", errors)
	}
	if rule.ID != "r1" || rule.Protocol != domain.ProtocolTCP {
		t.Fatalf("unexpected rule: %#v", rule)
	}
}

func TestDecodeAndValidateForwardRuleInvalidJSON(t *testing.T) {
	_, errors := DecodeAndValidateForwardRule([]byte(`not json`))
	if len(errors) == 0 {
		t.Fatal("expected decode error, got none")
	}
}

func TestDecodeAndValidateForwardRulePatch(t *testing.T) {
	raw := []byte(`{"name":"New Name","enabled":false}`)
	patch, errors := DecodeAndValidateForwardRulePatch(raw)
	if len(errors) > 0 {
		t.Fatalf("expected valid patch, got errors: %#v", errors)
	}
	if patch.Name == nil || *patch.Name != "New Name" {
		t.Fatalf("patch.Name = %v, want New Name", patch.Name)
	}
	if patch.Enabled == nil || *patch.Enabled {
		t.Fatalf("patch.Enabled = %v, want false", patch.Enabled)
	}
}

func TestDecodeAndValidateForwardRulePatchInvalidJSON(t *testing.T) {
	_, errors := DecodeAndValidateForwardRulePatch([]byte(`not json`))
	if len(errors) == 0 {
		t.Fatal("expected decode error, got none")
	}
}

func TestInputFromRuleTCP(t *testing.T) {
	rule := ruleFromValidInput(t, validInput(domain.ProtocolTCP))
	input := InputFromRule(rule)
	if input.ID == nil || *input.ID != rule.ID {
		t.Fatalf("ID mismatch: input=%v rule=%q", input.ID, rule.ID)
	}
	if input.Protocol == nil || *input.Protocol != domain.ProtocolTCP {
		t.Fatalf("Protocol = %v, want tcp", input.Protocol)
	}
	if input.UdpMode != nil {
		t.Fatalf("TCP rule UdpMode should be nil in input, got %v", input.UdpMode)
	}
}

func TestInputFromRuleUDP(t *testing.T) {
	base := validInput(domain.ProtocolUDP)
	rule := ruleFromValidInput(t, base)
	input := InputFromRule(rule)
	if input.UdpMode == nil || *input.UdpMode != domain.UdpModeOneWay {
		t.Fatalf("UdpMode = %v, want one-way", input.UdpMode)
	}
}

func TestApplyPatchProtocolChangeTCPClearsUdpMode(t *testing.T) {
	udpInput := validInput(domain.ProtocolUDP)
	udpRule := ruleFromValidInput(t, udpInput)
	if udpRule.UdpMode == nil {
		t.Fatal("expected udpMode on UDP rule before patch")
	}

	tcp := domain.ProtocolTCP
	patch, errors := ValidateForwardRulePatch(ForwardRuleInput{Protocol: &tcp})
	if len(errors) > 0 {
		t.Fatalf("patch errors: %#v", errors)
	}

	next, errors := ApplyPatch(udpRule, patch)
	if len(errors) > 0 {
		t.Fatalf("apply errors: %#v", errors)
	}
	if next.Protocol != domain.ProtocolTCP {
		t.Fatalf("protocol = %q, want tcp", next.Protocol)
	}
	if next.UdpMode != nil {
		t.Fatalf("expected nil UdpMode after TCP patch, got %v", *next.UdpMode)
	}
}

func TestApplyPatchAllFields(t *testing.T) {
	rule := ruleFromValidInput(t, validInput(domain.ProtocolTCP))
	newPort := 48002
	newTargetPort := 9000
	newHost := "192.168.1.1"
	newTarget := "192.168.1.2"
	enabled := false
	patch, errors := ValidateForwardRulePatch(ForwardRuleInput{
		ListenHost: &newHost,
		ListenPort: &newPort,
		TargetHost: &newTarget,
		TargetPort: &newTargetPort,
		Enabled:    &enabled,
	})
	if len(errors) > 0 {
		t.Fatalf("patch errors: %#v", errors)
	}
	next, errors := ApplyPatch(rule, patch)
	if len(errors) > 0 {
		t.Fatalf("apply errors: %#v", errors)
	}
	if next.ListenHost != newHost || next.ListenPort != newPort {
		t.Fatalf("listen mismatch: %v %v", next.ListenHost, next.ListenPort)
	}
	if next.TargetHost != newTarget || next.TargetPort != newTargetPort {
		t.Fatalf("target mismatch: %v %v", next.TargetHost, next.TargetPort)
	}
	if next.Enabled {
		t.Fatal("expected Enabled=false after patch")
	}
}

func TestApplyPatchUdpMode(t *testing.T) {
	rule := ruleFromValidInput(t, validInput(domain.ProtocolUDP))
	mode := domain.UdpModeBidirectionalLast
	patch, errors := ValidateForwardRulePatch(ForwardRuleInput{UdpMode: &mode})
	if len(errors) > 0 {
		t.Fatalf("patch errors: %#v", errors)
	}
	next, errors := ApplyPatch(rule, patch)
	if len(errors) > 0 {
		t.Fatalf("apply errors: %#v", errors)
	}
	if next.UdpMode == nil || *next.UdpMode != domain.UdpModeBidirectionalLast {
		t.Fatalf("UdpMode = %v, want bidirectional-last-client", next.UdpMode)
	}
}

func TestGroupAcceptedAndTrimmed(t *testing.T) {
	input := validInput(domain.ProtocolTCP)
	group := "  web-team  "
	input.Group = &group
	rule, errors := ValidateForwardRuleInput(input)
	if len(errors) > 0 {
		t.Fatalf("expected valid rule, got errors: %#v", errors)
	}
	if rule.Group == nil || *rule.Group != "web-team" {
		t.Fatalf("group = %v, want web-team", rule.Group)
	}
}

func TestGroupEmptyNormalizesToNil(t *testing.T) {
	for _, g := range []string{"", "   "} {
		input := validInput(domain.ProtocolTCP)
		group := g
		input.Group = &group
		rule, errors := ValidateForwardRuleInput(input)
		if len(errors) > 0 {
			t.Fatalf("group %q expected valid, got errors: %#v", g, errors)
		}
		if rule.Group != nil {
			t.Fatalf("group %q normalized to %v, want nil", g, *rule.Group)
		}
	}
}

func TestGroupAbsentIsNil(t *testing.T) {
	rule := ruleFromValidInput(t, validInput(domain.ProtocolTCP))
	if rule.Group != nil {
		t.Fatalf("group = %v, want nil", *rule.Group)
	}
}

func TestGroupTooLongRejected(t *testing.T) {
	input := validInput(domain.ProtocolTCP)
	group := strings.Repeat("x", domain.GroupMaxLength+1)
	input.Group = &group
	_, errors := ValidateForwardRuleInput(input)
	if !hasError(errors, "group must be 64 characters or fewer.") {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestGroupExactlyMaxAccepted(t *testing.T) {
	input := validInput(domain.ProtocolTCP)
	group := strings.Repeat("y", domain.GroupMaxLength)
	input.Group = &group
	rule, errors := ValidateForwardRuleInput(input)
	if len(errors) > 0 {
		t.Fatalf("expected valid, got errors: %#v", errors)
	}
	if rule.Group == nil || *rule.Group != group {
		t.Fatalf("group = %v, want %q", rule.Group, group)
	}
}

func TestGroupControlCharRejected(t *testing.T) {
	input := validInput(domain.ProtocolTCP)
	group := "bad" + string(rune(1)) + "group"
	input.Group = &group
	_, errors := ValidateForwardRuleInput(input)
	if !hasError(errors, "group must not contain control characters.") {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestPatchSetsGroup(t *testing.T) {
	rule := ruleFromValidInput(t, validInput(domain.ProtocolTCP))
	group := "  api-team  "
	patch := ForwardRulePatch{Group: &group}
	next, errors := ApplyPatch(rule, patch)
	if len(errors) > 0 {
		t.Fatalf("ApplyPatch errors: %#v", errors)
	}
	if next.Group == nil || *next.Group != "api-team" {
		t.Fatalf("group = %v, want api-team", next.Group)
	}
}

func TestPatchEmptyGroupClears(t *testing.T) {
	input := validInput(domain.ProtocolTCP)
	g := "web-team"
	input.Group = &g
	rule := ruleFromValidInput(t, input)
	empty := ""
	next, errors := ApplyPatch(rule, ForwardRulePatch{Group: &empty})
	if len(errors) > 0 {
		t.Fatalf("ApplyPatch errors: %#v", errors)
	}
	if next.Group != nil {
		t.Fatalf("group = %v, want nil (cleared)", *next.Group)
	}
}

func TestPatchAbsentGroupLeavesUnchanged(t *testing.T) {
	input := validInput(domain.ProtocolTCP)
	g := "web-team"
	input.Group = &g
	rule := ruleFromValidInput(t, input)
	name := "Renamed"
	next, errors := ApplyPatch(rule, ForwardRulePatch{Name: &name})
	if len(errors) > 0 {
		t.Fatalf("ApplyPatch errors: %#v", errors)
	}
	if next.Group == nil || *next.Group != "web-team" {
		t.Fatalf("group = %v, want web-team (unchanged)", next.Group)
	}
}

func TestPatchRejectsInvalidGroup(t *testing.T) {
	group := strings.Repeat("z", domain.GroupMaxLength+1)
	_, errors := ValidateForwardRulePatch(ForwardRuleInput{Group: &group})
	if !hasError(errors, "group must be 64 characters or fewer.") {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestValidateGroupName(t *testing.T) {
	if got, errs := ValidateGroupName("  web  "); got != "web" || len(errs) > 0 {
		t.Fatalf("ValidateGroupName(web) = %q, %#v", got, errs)
	}
	if _, errs := ValidateGroupName("   "); !hasError(errs, "group is required.") {
		t.Fatalf("whitespace group errors = %#v", errs)
	}
	if _, errs := ValidateGroupName(""); !hasError(errs, "group is required.") {
		t.Fatalf("empty group errors = %#v", errs)
	}
	if _, errs := ValidateGroupName(strings.Repeat("x", domain.GroupMaxLength+1)); !hasError(errs, "group must be 64 characters or fewer.") {
		t.Fatalf("too-long group errors = %#v", errs)
	}
	if _, errs := ValidateGroupName("bad" + string(rune(1)) + "grp"); !hasError(errs, "group must not contain control characters.") {
		t.Fatalf("control-char group errors = %#v", errs)
	}
}

func validInput(protocol domain.ForwardProtocol) ForwardRuleInput {
	id := "rule-1"
	name := "Local app"
	listenHost := "127.0.0.1"
	listenPort := 48001
	targetHost := "127.0.0.1"
	targetPort := 3000
	enabled := true
	return ForwardRuleInput{
		ID:         &id,
		Name:       &name,
		Protocol:   &protocol,
		ListenHost: &listenHost,
		ListenPort: &listenPort,
		TargetHost: &targetHost,
		TargetPort: &targetPort,
		Enabled:    &enabled,
	}
}

func ruleFromValidInput(t *testing.T, input ForwardRuleInput) domain.ForwardRule {
	t.Helper()
	rule, errors := ValidateForwardRuleInput(input)
	if len(errors) > 0 {
		t.Fatalf("valid input produced errors: %#v", errors)
	}
	return rule
}

func hasError(errors []string, expected string) bool {
	for _, actual := range errors {
		if actual == expected {
			return true
		}
	}
	return false
}
