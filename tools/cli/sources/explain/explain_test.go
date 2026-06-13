package explain

import (
	"strings"
	"testing"
)

func sampleRegistry() map[string]Explanation {
	return map[string]Explanation{
		"a.one": {Code: "a.one", Title: "One", Meaning: "m1", Action: "do1", Severity: "info", Related: []string{"a.two"}},
		"a.two": {Code: "a.two", Title: "Two", Meaning: "m2", Action: "do2", Severity: "error"},
	}
}

func TestMerge(t *testing.T) {
	a := map[string]Explanation{"x": {Code: "x"}}
	b := map[string]Explanation{"y": {Code: "y"}}
	m := Merge(a, b)
	if len(m) != 2 || m["x"].Code != "x" || m["y"].Code != "y" {
		t.Errorf("merge = %v, want x and y", m)
	}
	// Later registry wins on collision.
	c := map[string]Explanation{"x": {Code: "x", Title: "first"}}
	d := map[string]Explanation{"x": {Code: "x", Title: "second"}}
	if Merge(c, d)["x"].Title != "second" {
		t.Errorf("merge collision should let the later registry win")
	}
}

func TestFor(t *testing.T) {
	reg := sampleRegistry()
	if e, ok := For(reg, "a.one"); !ok || e.Title != "One" {
		t.Errorf("For(a.one) = %v, %v", e, ok)
	}
	if _, ok := For(reg, "missing"); ok {
		t.Errorf("For(missing) should be false")
	}
}

func TestSortedCodesAndSorted(t *testing.T) {
	reg := sampleRegistry()
	codes := SortedCodes(reg)
	if len(codes) != 2 || codes[0] != "a.one" || codes[1] != "a.two" {
		t.Errorf("SortedCodes = %v, want [a.one a.two]", codes)
	}
	list := Sorted(reg)
	if len(list) != 2 || list[0].Code != "a.one" || list[1].Code != "a.two" {
		t.Errorf("Sorted = %v, want sorted by code", list)
	}
}

func TestForReport(t *testing.T) {
	reg := sampleRegistry()
	// Duplicate code collapses; an unknown code is omitted.
	m := ForReport(reg, []string{"a.one", "a.one", "missing"})
	if len(m) != 1 || m["a.one"].Code != "a.one" {
		t.Errorf("ForReport = %v, want only a.one", m)
	}
	if len(ForReport(reg, []string{"missing"})) != 0 {
		t.Errorf("unknown-only report should yield no explanations")
	}
}

func TestPrintInline_Known(t *testing.T) {
	var b strings.Builder
	PrintInline(sampleRegistry(), "a.one", &b)
	s := b.String()
	for _, want := range []string{"Code: a.one", "Meaning: m1", "What to do: do1", "Related: a.two"} {
		if !strings.Contains(s, want) {
			t.Errorf("inline missing %q:\n%s", want, s)
		}
	}
}

func TestPrintInline_NoRelated(t *testing.T) {
	var b strings.Builder
	PrintInline(sampleRegistry(), "a.two", &b)
	if strings.Contains(b.String(), "Related:") {
		t.Errorf("entry with no related codes should not print a Related line:\n%s", b.String())
	}
}

func TestPrintInline_UnknownDegrades(t *testing.T) {
	var b strings.Builder
	PrintInline(sampleRegistry(), "not.real", &b)
	s := b.String()
	if !strings.Contains(s, "Code: not.real") || !strings.Contains(s, "(no explanation available)") {
		t.Errorf("unknown code should degrade safely:\n%s", s)
	}
}
