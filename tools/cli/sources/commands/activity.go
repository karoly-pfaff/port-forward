package commands

import (
	"flag"
	"fmt"
	"io"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
)

const activityHelp = `Usage: portier activity [options]

Show recent activity events (rule lifecycle, TCP connections, UDP packets).

Options:
  --limit int       Number of events to return, 1–500 (default 50)
  --rule string     Filter by rule ID
  --type string     Filter by event type (e.g. rule.started, tcp.connection.opened)
  --severity string Filter by severity: info, success, warning, error
`

// RunActivity executes the `portier activity` command.
// Returns an exit code: 0 success, 1 API/general error, 2 invalid args, 3 connection failure.
func RunActivity(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("activity", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	flagLimit := fs.Int("limit", 50, "number of events to return")
	flagRule := fs.String("rule", "", "filter by rule ID")
	flagType := fs.String("type", "", "filter by event type")
	flagSeverity := fs.String("severity", "", "filter by severity")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, activityHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, activityHelp)
		return 2
	}

	if *flagLimit < 1 || *flagLimit > 500 {
		fmt.Fprintf(stderr, "Error: --limit must be between 1 and 500\n\n")
		fmt.Fprint(stderr, activityHelp)
		return 2
	}

	events, err := c.GetActivity(client.ActivityQuery{
		Limit:    *flagLimit,
		RuleID:   *flagRule,
		Type:     *flagType,
		Severity: *flagSeverity,
	})
	if err != nil {
		return exitWithError(err, stderr)
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, events); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	if len(events) == 0 {
		fmt.Fprintln(stdout, "No activity events found.")
		return 0
	}

	printActivityHuman(events, stdout)
	return 0
}

func printActivityHuman(events []client.ActivityEvent, w io.Writer) {
	headers := []string{"TIME", "SEV", "TYPE", "RULE", "MESSAGE"}
	rows := make([][]string, len(events))
	for i, e := range events {
		rule := e.RuleName
		if rule == "" {
			rule = e.RuleID
		}
		rows[i] = []string{
			output.FormatTimestamp(e.Timestamp),
			e.Severity,
			e.Type,
			rule,
			e.Message,
		}
	}
	output.PrintTable(w, headers, rows)
}
