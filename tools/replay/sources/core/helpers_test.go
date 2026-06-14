package core

import (
	"os"
	"testing"
)

// writeFile writes content to path, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// itemByID returns the plan item with the given id, or fails the test.
func itemByID(t *testing.T, p Plan, id string) PlanItem {
	t.Helper()
	for _, it := range p.Items {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("no plan item with id %q", id)
	return PlanItem{}
}
