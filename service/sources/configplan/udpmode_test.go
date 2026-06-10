package configplan

// Coverage Slice D — udpMode diff branches. A change to a UDP rule's mode must
// be detected as a material (forwarding-field) update, exercising udpModeEqual
// and udpModeVal for both non-nil↔non-nil and nil↔non-nil comparisons.

import (
	"testing"

	"portier/service/sources/domain"
)

func udpRuleWithMode(mode *domain.UdpMode) domain.ForwardRule {
	return domain.ForwardRule{
		ID:         "u1",
		Name:       "U",
		Protocol:   domain.ProtocolUDP,
		ListenHost: "127.0.0.1",
		ListenPort: 48070,
		TargetHost: "127.0.0.1",
		TargetPort: 9000,
		Enabled:    false,
		UdpMode:    mode,
	}
}

func findUDPModeChange(ops []Operation) (Change, bool) {
	for _, op := range ops {
		if op.Type != "update" {
			continue
		}
		for _, ch := range op.Changes {
			if ch.Field == "udpMode" {
				return ch, true
			}
		}
	}
	return Change{}, false
}

func TestBuildConfigPlan_UdpModeChange_NonNilToNonNil(t *testing.T) {
	multi := domain.UdpModeBidirectionalMulti
	current := udpRuleWithMode(&multi)
	desired := `[{"id":"u1","name":"U","protocol":"udp","listenHost":"127.0.0.1","listenPort":48070,"targetHost":"127.0.0.1","targetPort":9000,"enabled":false,"udpMode":"one-way"}]`

	resp := buildPlan([]domain.ForwardRule{current}, desired)
	if resp.Summary.Update != 1 {
		t.Fatalf("update count = %d, want 1 (udpMode change)", resp.Summary.Update)
	}
	ch, ok := findUDPModeChange(resp.Operations)
	if !ok {
		t.Fatalf("expected a udpMode change in operations: %#v", resp.Operations)
	}
	if ch.Before != "bidirectional-multi-client" || ch.After != "one-way" {
		t.Fatalf("udpMode change = %v → %v, want bidirectional-multi-client → one-way", ch.Before, ch.After)
	}
}

func TestBuildConfigPlan_UdpModeChange_NilToNonNil(t *testing.T) {
	current := udpRuleWithMode(nil) // no mode set
	desired := `[{"id":"u1","name":"U","protocol":"udp","listenHost":"127.0.0.1","listenPort":48070,"targetHost":"127.0.0.1","targetPort":9000,"enabled":false,"udpMode":"one-way"}]`

	resp := buildPlan([]domain.ForwardRule{current}, desired)
	ch, ok := findUDPModeChange(resp.Operations)
	if !ok {
		t.Fatalf("expected a udpMode change for nil→one-way: %#v", resp.Operations)
	}
	if ch.Before != nil {
		t.Fatalf("udpMode before = %v, want nil", ch.Before)
	}
	if ch.After != "one-way" {
		t.Fatalf("udpMode after = %v, want one-way", ch.After)
	}
}

func TestBuildConfigPlan_UdpModeUnchanged_NoDrift(t *testing.T) {
	oneway := domain.UdpModeOneWay
	current := udpRuleWithMode(&oneway)
	desired := `[{"id":"u1","name":"U","protocol":"udp","listenHost":"127.0.0.1","listenPort":48070,"targetHost":"127.0.0.1","targetPort":9000,"enabled":false,"udpMode":"one-way"}]`

	resp := buildPlan([]domain.ForwardRule{current}, desired)
	if resp.Summary.HasDrift {
		t.Fatalf("identical udpMode must not produce drift: %#v", resp.Summary)
	}
}
