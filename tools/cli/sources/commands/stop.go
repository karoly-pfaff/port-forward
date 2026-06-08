package commands

import (
	"fmt"
	"io"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
)

// RunStop executes the `portier stop <id|name>` command.
// Returns an exit code: 0 success, 1 API/general error, 2 invalid args, 3 connection failure.
func RunStop(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Error: stop requires a rule ID or name")
		fmt.Fprintln(stderr, "Usage: portier stop <id|name>")
		return 2
	}

	rule, code := ResolveRule(c, args[0], stderr)
	if code != 0 {
		return code
	}

	if err := c.StopForward(rule.ID); err != nil {
		return exitWithError(err, stderr)
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, map[string]any{
			"ok":     true,
			"action": "stop",
			"ruleId": rule.ID,
		}); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "Stopped %s\n", rule.Name)
	return 0
}
