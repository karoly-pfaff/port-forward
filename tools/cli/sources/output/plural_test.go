package output

import "testing"

func TestPluralWord(t *testing.T) {
	if got := PluralWord(1, "warning", "warnings"); got != "warning" {
		t.Errorf("PluralWord(1) = %q, want warning", got)
	}
	if got := PluralWord(0, "warning", "warnings"); got != "warnings" {
		t.Errorf("PluralWord(0) = %q, want warnings", got)
	}
	if got := PluralWord(2, "warning", "warnings"); got != "warnings" {
		t.Errorf("PluralWord(2) = %q, want warnings", got)
	}
}

func TestPluralRule(t *testing.T) {
	if got := PluralRule(1); got != "rule" {
		t.Errorf("PluralRule(1) = %q, want rule", got)
	}
	if got := PluralRule(0); got != "rules" {
		t.Errorf("PluralRule(0) = %q, want rules", got)
	}
	if got := PluralRule(2); got != "rules" {
		t.Errorf("PluralRule(2) = %q, want rules", got)
	}
}
