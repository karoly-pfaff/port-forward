package validation

import (
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
