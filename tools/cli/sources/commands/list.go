package commands

import (
	"fmt"
	"io"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
)

// RunList executes the `portier list` command.
// Returns an exit code: 0 success, 1 API/general error, 3 connection failure.
func RunList(c *client.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	rules, err := c.GetForwards()
	if err != nil {
		return exitWithError(err, stderr)
	}
	if jsonOutput {
		if err := output.PrintJSON(stdout, rules); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}
	if len(rules) == 0 {
		fmt.Fprintln(stdout, "No forwarding rules configured.")
		return 0
	}
	printListHuman(rules, stdout)
	return 0
}

func printListHuman(rules []client.ForwardRuleResponse, w io.Writer) {
	headers := []string{"NAME", "PROTO", "LISTEN", "TARGET", "ENABLED"}
	rows := make([][]string, len(rules))
	for i, r := range rules {
		rows[i] = []string{
			r.Name,
			formatProto(r),
			fmt.Sprintf("%s:%d", r.ListenHost, r.ListenPort),
			fmt.Sprintf("%s:%d", r.TargetHost, r.TargetPort),
			output.FormatBool(r.Enabled),
		}
	}
	output.PrintTable(w, headers, rows)
}

// formatProto returns a compact protocol string including UDP mode when present.
func formatProto(r client.ForwardRuleResponse) string {
	if r.Protocol != "udp" {
		return "tcp"
	}
	switch r.UDPMode {
	case "one-way":
		return "udp/1way"
	case "bidirectional-last-client":
		return "udp/lc"
	case "bidirectional-multi-client":
		return "udp/mc"
	default:
		return "udp"
	}
}
