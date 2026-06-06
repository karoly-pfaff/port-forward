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

func TestManagementLANExposureDanger(t *testing.T) {
	advisories := GetPortAdvisories(Input{Port: DefaultManagementPort, ListenHost: "0.0.0.0", Purpose: PurposeManagement})
	if len(advisories) != 1 || !containsAdvisory(advisories, "MANAGEMENT_LAN_EXPOSURE", "danger") {
		t.Fatalf("advisories = %#v", advisories)
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
