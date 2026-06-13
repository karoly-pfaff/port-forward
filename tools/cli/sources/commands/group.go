package commands

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"portier/cli/sources/client"
	"portier/cli/sources/config"
	"portier/cli/sources/output"
)

const groupHelp = `Usage: portier group <subcommand> [args]

Operate on forwarding rules by their optional group label.

Subcommands:
  list                 List groups derived from current rules (with rule + running counts)
  start <group>        Start every rule in the group
  stop <group>         Stop every running rule in the group
  help                 Show this help message

Group operations act on existing rules only — they never create, delete, or
modify rule definitions. Rules are processed in rule order. "start" skips
already-running rules; "stop" skips rules that are not running.

Exit codes:
  0  Success (including rules skipped for already_running / not_running)
  1  API/runtime error, no matching group (404), or one or more rules failed
  2  Invalid arguments or invalid group value
  3  Connection failure — Portier service unreachable

Add --json for machine-readable output (full GroupActionResponse for start/stop).
`

// RunGroup dispatches the `portier group <subcommand>` commands.
func RunGroup(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, groupHelp)
		return 2
	}
	switch args[0] {
	case "list":
		return RunGroupList(c, jsonOutput, stdout, stderr)
	case "start":
		return runGroupAction(c, jsonOutput, "start", args[1:], stdout, stderr)
	case "stop":
		return runGroupAction(c, jsonOutput, "stop", args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, groupHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown group subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, groupHelp)
		return 2
	}
}

// RunGroupList lists groups derived from the live rules with rule/running counts.
func RunGroupList(c *client.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	rules, err := c.GetForwards()
	if err != nil {
		return exitWithError(err, stderr)
	}
	statuses, err := c.GetStatus()
	if err != nil {
		return exitWithError(err, stderr)
	}

	running := make(map[string]bool, len(statuses))
	for _, s := range statuses {
		running[s.RuleID] = s.Running
	}

	type counts struct{ total, run int }
	byGroup := map[string]*counts{}
	for _, r := range rules {
		g := strings.TrimSpace(r.Group)
		if g == "" {
			continue
		}
		c := byGroup[g]
		if c == nil {
			c = &counts{}
			byGroup[g] = c
		}
		c.total++
		if running[r.ID] {
			c.run++
		}
	}

	names := make([]string, 0, len(byGroup))
	for g := range byGroup {
		names = append(names, g)
	}
	sort.Strings(names)

	if jsonOutput {
		groups := make([]map[string]any, 0, len(names))
		for _, g := range names {
			groups = append(groups, map[string]any{"group": g, "total": byGroup[g].total, "running": byGroup[g].run})
		}
		if err := output.PrintJSON(stdout, map[string]any{"groups": groups}); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	if len(names) == 0 {
		fmt.Fprintln(stdout, "No rule groups configured.")
		return 0
	}

	headers := []string{"GROUP", "RULES", "RUNNING"}
	rows := make([][]string, len(names))
	for i, g := range names {
		rows[i] = []string{g, strconv.Itoa(byGroup[g].total), strconv.Itoa(byGroup[g].run)}
	}
	output.PrintTable(stdout, headers, rows)
	return 0
}

// runGroupAction backs `portier group start|stop <group>`.
func runGroupAction(c *client.Client, jsonOutput bool, action string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "Error: group %s requires a group name\n", action)
		fmt.Fprintf(stderr, "Usage: portier group %s <group>\n", action)
		return 2
	}

	group, errMsg, ok := validateGroupArg(args[0])
	if !ok {
		fmt.Fprintf(stderr, "Error: %s\n", errMsg)
		return 2
	}

	var resp *client.GroupActionResponse
	var err error
	if action == "start" {
		resp, err = c.StartGroup(group)
	} else {
		resp, err = c.StopGroup(group)
	}
	if err != nil {
		return exitWithError(err, stderr)
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, resp); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		if resp.Failed > 0 {
			return 1
		}
		return 0
	}

	printGroupActionHuman(resp, stdout)
	if resp.Failed > 0 {
		fmt.Fprintf(stderr, "Error: %d rule(s) failed to %s in group %q\n", resp.Failed, action, resp.Group)
		return 1
	}
	return 0
}

func printGroupActionHuman(resp *client.GroupActionResponse, w io.Writer) {
	fmt.Fprintf(w, "Group %q %s — %d succeeded, %d skipped, %d failed (%d total)\n",
		resp.Group, resp.Action, resp.Succeeded, resp.Skipped, resp.Failed, resp.Total)
	if len(resp.Results) == 0 {
		return
	}
	headers := []string{"RULE", "STATUS", "REASON"}
	rows := make([][]string, len(resp.Results))
	for i, r := range resp.Results {
		rows[i] = []string{r.RuleName, r.Status, r.Reason}
	}
	output.PrintTable(w, headers, rows)
}

// validateGroupArg trims and validates a group argument with the same rules the
// API enforces (non-empty, ≤ config.GroupMaxLength characters, no control characters).
// Local pre-check only; the server remains authoritative. Returns the trimmed
// group, an error message, and ok.
func validateGroupArg(raw string) (string, string, bool) {
	g := strings.TrimSpace(raw)
	if g == "" {
		return "", "group is required.", false
	}
	if utf8.RuneCountInString(g) > config.GroupMaxLength {
		return "", fmt.Sprintf("group must be %d characters or fewer.", config.GroupMaxLength), false
	}
	if config.HasControlChar(g) {
		return "", "group must not contain control characters.", false
	}
	return g, "", true
}
