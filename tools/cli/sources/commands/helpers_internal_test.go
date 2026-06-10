package commands

// Internal (white-box) tests for unexported diff/plan formatting helpers.
// These cover the value-formatting and endpoint branches that are awkward to
// reach through the full command path (a plan response would have to carry
// changes of every value kind). Behavior is asserted, not implementation.

import (
	"testing"

	"portier/cli/sources/client"
)

func TestFormatChangeValue_AllKinds(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, "(none)"},
		{"integer-valued float", float64(48080), "48080"},
		{"negative integer-valued float", float64(-5), "-5"},
		{"non-integer float", float64(2.5), "2.5"},
		{"string", "tcp", "tcp"},
		{"bool", true, "true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatChangeValue(tc.in); got != tc.want {
				t.Errorf("formatChangeValue(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestOpEndpoint_Variants(t *testing.T) {
	snap := &client.ConfigPlanRuleSnapshot{
		ListenHost: "127.0.0.1", ListenPort: 48080,
		TargetHost: "10.0.0.5", TargetPort: 9000,
	}
	const want = "127.0.0.1:48080 → 10.0.0.5:9000"

	// Current present → uses Current.
	if got := opEndpoint(client.ConfigPlanOperation{Current: snap}); got != want {
		t.Errorf("Current endpoint = %q, want %q", got, want)
	}
	// Current nil, Desired present → falls back to Desired (e.g. an add op).
	if got := opEndpoint(client.ConfigPlanOperation{Desired: snap}); got != want {
		t.Errorf("Desired endpoint = %q, want %q", got, want)
	}
	// Neither present → em-dash placeholder.
	if got := opEndpoint(client.ConfigPlanOperation{}); got != "—" {
		t.Errorf("empty endpoint = %q, want em dash", got)
	}
}
