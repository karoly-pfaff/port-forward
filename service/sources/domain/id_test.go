package domain

import (
	"regexp"
	"testing"
)

// uuidV4Pattern matches the UUID-v4-style format produced by NewRuleID in the
// normal (CSPRNG available) path: 8-4-4-4-12 lowercase hex with version nibble 4
// and variant nibble in [89ab].
var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewRuleIDIsNonEmpty(t *testing.T) {
	if NewRuleID() == "" {
		t.Fatal("NewRuleID returned an empty string")
	}
}

func TestNewRuleIDFormat(t *testing.T) {
	id := NewRuleID()
	if !uuidV4Pattern.MatchString(id) {
		t.Errorf("NewRuleID() = %q, does not match UUID-v4 format", id)
	}
}

func TestNewRuleIDIsUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := NewRuleID()
		if seen[id] {
			t.Fatalf("NewRuleID produced a duplicate id after %d calls: %q", i, id)
		}
		seen[id] = true
	}
}
