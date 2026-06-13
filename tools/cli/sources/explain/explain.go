// Package explain holds the Portier CLI's operator-facing code explanation
// model: the shared Explanation type plus small, deterministic helpers for
// looking up, sorting, filtering, and rendering explanations. It is a leaf
// package — it imports nothing internal and owns no explanation DATA. Each
// domain (doctor, policy) defines its own explanation registry keyed by its own
// stable code constants and reuses these helpers; the `explain` command merges
// those registries for lookup and listing. This keeps a single Explanation type
// and one rendering implementation without a central registry framework.
package explain

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Explanation is a static, deterministic description of one stable
// doctor/policy/check code: what it means and what an operator should do. It is
// offline reference data — there is no probing, remediation, or external lookup
// behind it, and it must not claim automatic enforcement or fixing.
type Explanation struct {
	Code     string   `json:"code"`
	Title    string   `json:"title"`
	Meaning  string   `json:"meaning"`
	Action   string   `json:"action"`
	Severity string   `json:"severity,omitempty"`
	Related  []string `json:"related,omitempty"`
}

// Merge combines explanation registries into one map. Keys are stable codes and
// must not collide across domains (doctor uses "config.*"/"runtime.*"/"rules.*",
// policy uses "policy.*"); on an unexpected collision the later registry wins.
func Merge(regs ...map[string]Explanation) map[string]Explanation {
	out := make(map[string]Explanation)
	for _, reg := range regs {
		for code, exp := range reg {
			out[code] = exp
		}
	}
	return out
}

// For returns the explanation for a code from reg and whether one exists.
func For(reg map[string]Explanation, code string) (Explanation, bool) {
	e, ok := reg[code]
	return e, ok
}

// SortedCodes returns all codes in reg in deterministic sorted order.
func SortedCodes(reg map[string]Explanation) []string {
	codes := make([]string, 0, len(reg))
	for c := range reg {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	return codes
}

// Sorted returns every explanation in reg in deterministic code-sorted order.
func Sorted(reg map[string]Explanation) []Explanation {
	codes := SortedCodes(reg)
	list := make([]Explanation, len(codes))
	for i, c := range codes {
		list[i] = reg[c]
	}
	return list
}

// ForReport returns the explanation for each code present in codes (deduplicated
// via the map; codes with no entry in reg are safely omitted). Used to attach an
// additive explanations map to a report under --explain.
func ForReport(reg map[string]Explanation, codes []string) map[string]Explanation {
	m := make(map[string]Explanation)
	for _, c := range codes {
		if exp, ok := reg[c]; ok {
			m[c] = exp
		}
	}
	return m
}

// PrintInline renders the indented inline explanation block for one code,
// aligned with an 8-space message indent (the doctor/policy human style). A code
// with no entry in reg degrades safely to a clear "(no explanation available)".
func PrintInline(reg map[string]Explanation, code string, w io.Writer) {
	fmt.Fprintf(w, "        Code: %s\n", code)
	exp, ok := reg[code]
	if !ok {
		fmt.Fprintln(w, "        (no explanation available)")
		return
	}
	fmt.Fprintf(w, "        Meaning: %s\n", exp.Meaning)
	fmt.Fprintf(w, "        What to do: %s\n", exp.Action)
	if len(exp.Related) > 0 {
		fmt.Fprintf(w, "        Related: %s\n", strings.Join(exp.Related, ", "))
	}
}
