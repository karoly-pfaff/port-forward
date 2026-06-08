package commands

import (
	"fmt"
	"io"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
)

// ResolveRule resolves an id-or-name argument to a ForwardRuleResponse.
//
// Resolution order:
//  1. Exact ID match wins unconditionally.
//  2. Exact name match is used if exactly one rule has that name.
//  3. Multiple name matches: exit 2 (ambiguous — use ID).
//  4. No match: exit 1.
func ResolveRule(c *client.Client, idOrName string, stderr io.Writer) (*client.ForwardRuleResponse, int) {
	rules, err := c.GetForwards()
	if err != nil {
		return nil, exitWithError(err, stderr)
	}

	// 1. Exact ID match
	for i := range rules {
		if rules[i].ID == idOrName {
			return &rules[i], 0
		}
	}

	// 2. Exact name match — collect all
	var matches []client.ForwardRuleResponse
	for _, r := range rules {
		if r.Name == idOrName {
			matches = append(matches, r)
		}
	}

	switch len(matches) {
	case 0:
		fmt.Fprintf(stderr, "Error: no rule found matching %q\n", idOrName)
		return nil, 1
	case 1:
		return &matches[0], 0
	default:
		fmt.Fprintf(stderr, "Error: multiple rules match %q. Use an ID instead.\n\n", idOrName)
		headers := []string{"ID", "NAME", "PROTO", "LISTEN"}
		rows := make([][]string, len(matches))
		for i, r := range matches {
			rows[i] = []string{
				r.ID,
				r.Name,
				formatProto(r),
				fmt.Sprintf("%s:%d", r.ListenHost, r.ListenPort),
			}
		}
		output.PrintTable(stderr, headers, rows)
		return nil, 2
	}
}
