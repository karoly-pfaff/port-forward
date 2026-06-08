package commands

import (
	"fmt"
	"io"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
)

// RunStart executes the `portier start <id|name>` command.
// Returns an exit code: 0 success, 1 API/general error, 2 invalid args, 3 connection failure.
func RunStart(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Error: start requires a rule ID or name")
		fmt.Fprintln(stderr, "Usage: portier start <id|name>")
		return 2
	}

	rule, code := ResolveRule(c, args[0], stderr)
	if code != 0 {
		return code
	}

	if err := c.StartForward(rule.ID); err != nil {
		return exitWithError(err, stderr)
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, map[string]any{
			"ok":     true,
			"action": "start",
			"ruleId": rule.ID,
		}); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "Started %s  (%s:%d → %s:%d)\n",
		rule.Name, rule.ListenHost, rule.ListenPort, rule.TargetHost, rule.TargetPort)
	return 0
}
