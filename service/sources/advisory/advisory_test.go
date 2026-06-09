package advisory

import (
	"testing"

	"portier/service/sources/domain"
)

func TestCommonPortWarning(t *testing.T) {
	advisories := GetPortAdvisories(Input{Port: 5173, Purpose: PurposeForward})
	if !containsAdvisory(advisories, "COMMON_PORT", "warning") {
		t.Fatalf("advisories = %#v", advisories)
	}
}

func TestPrivilegedPortWarning(t *testing.T) {
	advisories := GetPortAdvisories(Input{Port: 80, Purpose: PurposeForward})
	if !containsAdvisory(advisories, "PRIVILEGED_PORT", "danger") {
		t.Fatalf("advisories = %#v", advisories)
	}
}

func TestOutsideRecommendedRangeInfo(t *testing.T) {
	advisories := GetPortAdvisories(Input{Port: 3000, Purpose: PurposeForward})
	if !containsAdvisory(advisories, "OUTSIDE_RECOMMENDED_RANGE", "info") {
		t.Fatalf("advisories = %#v", advisories)
	}
}

func TestLANExposureWarning(t *testing.T) {
	advisories := GetPortAdvisories(Input{Port: 48001, ListenHost: "0.0.0.0", Purpose: PurposeForward})
	if len(advisories) != 1 || !containsAdvisory(advisories, "LAN_EXPOSURE", "warning") {
		t.Fatalf("advisories = %#v", advisories)
	}
}

// TestLANExposureCanonicalContent locks the full LAN_EXPOSURE advisory content to
// the canonical TypeScript wording in shared/sources/index.ts. This is the
// runtime-parity guard for Arch-A: if either runtime's wording is edited without
// the other, this test (and validate:contract) must fail.
func TestLANExposureCanonicalContent(t *testing.T) {
	const canonicalMessage = "Listening on 0.0.0.0 exposes this forwarded port on all interfaces. Other LAN devices may be able to connect if firewall settings allow it."

	advisories := GetPortAdvisories(Input{Port: 48001, ListenHost: "0.0.0.0", Purpose: PurposeForward})
	if len(advisories) != 1 {
		t.Fatalf("expected exactly one advisory, got %#v", advisories)
	}
	got := advisories[0]
	if got.Code != "LAN_EXPOSURE" {
		t.Errorf("code = %q, want %q", got.Code, "LAN_EXPOSURE")
	}
	if got.Severity != "warning" {
		t.Errorf("severity = %q, want %q", got.Severity, "warning")
	}
	if got.Message != canonicalMessage {
		t.Errorf("message = %q, want %q", got.Message, canonicalMessage)
	}
}

func TestManagementLANExposureDanger(t *testing.T) {
	advisories := GetPortAdvisories(Input{Port: DefaultManagementPort, ListenHost: "0.0.0.0", Purpose: PurposeManagement})
	if len(advisories) != 1 || !containsAdvisory(advisories, "MANAGEMENT_LAN_EXPOSURE", "danger") {
		t.Fatalf("advisories = %#v", advisories)
	}
}

// TestManagementLANExposureCanonicalContent locks the MANAGEMENT_LAN_EXPOSURE
// advisory content to the canonical TypeScript wording in shared/sources/index.ts.
func TestManagementLANExposureCanonicalContent(t *testing.T) {
	const canonicalMessage = "Listening on 0.0.0.0 exposes the Portier management UI/API on the LAN."

	advisories := GetPortAdvisories(Input{Port: DefaultManagementPort, ListenHost: "0.0.0.0", Purpose: PurposeManagement})
	if len(advisories) != 1 {
		t.Fatalf("expected exactly one advisory, got %#v", advisories)
	}
	got := advisories[0]
	if got.Code != "MANAGEMENT_LAN_EXPOSURE" {
		t.Errorf("code = %q, want %q", got.Code, "MANAGEMENT_LAN_EXPOSURE")
	}
	if got.Severity != "danger" {
		t.Errorf("severity = %q, want %q", got.Severity, "danger")
	}
	if got.Message != canonicalMessage {
		t.Errorf("message = %q, want %q", got.Message, canonicalMessage)
	}
}

func containsAdvisory(advisories []domain.PortAdvisory, code string, severity string) bool {
	for _, advisory := range advisories {
		if advisory.Code == code && advisory.Severity == severity {
			return true
		}
	}
	return false
}
