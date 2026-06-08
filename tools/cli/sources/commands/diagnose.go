package commands

import (
	"fmt"
	"io"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
)

// RunDiagnose executes the `portier diagnose <id|name>` command.
// Returns an exit code: 0 success, 1 API/general error, 2 invalid args, 3 connection failure.
func RunDiagnose(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Error: diagnose requires a rule ID or name")
		fmt.Fprintln(stderr, "Usage: portier diagnose <id|name>")
		return 2
	}

	rule, code := ResolveRule(c, args[0], stderr)
	if code != 0 {
		return code
	}

	result, err := c.DiagnoseForward(rule.ID)
	if err != nil {
		return exitWithError(err, stderr)
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	printDiagnoseHuman(result, stdout)
	return 0
}

func printDiagnoseHuman(r *client.RuleDiagnosticsResult, w io.Writer) {
	fmt.Fprintf(w, "Rule: %s\n", r.RuleName)
	fmt.Fprintf(w, "Summary: %s — %s\n\n", r.Summary.Status, r.Summary.Message)

	if len(r.Checks) == 0 {
		return
	}

	headers := []string{"CHECK", "STATUS", "MESSAGE"}
	rows := make([][]string, len(r.Checks))
	for i, ch := range r.Checks {
		rows[i] = []string{ch.Label, ch.Status, ch.Message}
	}
	output.PrintTable(w, headers, rows)
}
