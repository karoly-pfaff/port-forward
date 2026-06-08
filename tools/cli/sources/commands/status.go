package commands

import (
	"fmt"
	"io"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
)

// RunStatus executes the `portier status` command.
// Returns an exit code: 0 success, 1 API/general error, 3 connection failure.
func RunStatus(c *client.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	statuses, err := c.GetStatus()
	if err != nil {
		return exitWithError(err, stderr)
	}
	if jsonOutput {
		if err := output.PrintJSON(stdout, statuses); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}
	if len(statuses) == 0 {
		fmt.Fprintln(stdout, "No rule status available.")
		return 0
	}
	infoMap := buildRuleInfo(c)
	printStatusHuman(statuses, infoMap, stdout)
	return 0
}

type ruleInfo struct {
	Name     string
	Protocol string
}

// buildRuleInfo fetches rule names and protocols for human display.
// Returns an empty map on error; callers fall back to ruleId.
func buildRuleInfo(c *client.Client) map[string]ruleInfo {
	rules, err := c.GetForwards()
	if err != nil {
		return map[string]ruleInfo{}
	}
	m := make(map[string]ruleInfo, len(rules))
	for _, r := range rules {
		m[r.ID] = ruleInfo{Name: r.Name, Protocol: r.Protocol}
	}
	return m
}

func printStatusHuman(statuses []client.ForwardStatus, infoMap map[string]ruleInfo, w io.Writer) {
	headers := []string{"NAME", "PROTO", "STATE", "CONNS", "BYTES IN", "BYTES OUT", "LAST ERROR"}
	rows := make([][]string, len(statuses))
	for i, s := range statuses {
		ri := infoMap[s.RuleID]
		name := ri.Name
		if name == "" {
			name = s.RuleID
		}
		proto := ri.Protocol
		if proto == "" {
			proto = "-"
		}
		state := "stopped"
		if s.Running {
			state = "running"
		}
		conns := "-"
		if s.ActiveConnections != nil {
			conns = fmt.Sprintf("%d", *s.ActiveConnections)
		} else if s.ActiveUDPSessions != nil {
			conns = fmt.Sprintf("%d", *s.ActiveUDPSessions)
		}
		rows[i] = []string{
			name,
			proto,
			state,
			conns,
			output.FormatBytes(s.BytesIn),
			output.FormatBytes(s.BytesOut),
			s.LastError,
		}
	}
	output.PrintTable(w, headers, rows)
}
