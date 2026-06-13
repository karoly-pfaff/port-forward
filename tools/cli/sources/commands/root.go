package commands

import (
	"errors"
	"fmt"
	"io"

	"portier/cli/sources/client"
	"portier/cli/sources/version"
)

const helpText = `Portier CLI — manage the local Portier port forwarding service from the terminal.

Default management URL: http://127.0.0.1:47831

Global flags:
  --url string    Full management API URL (overrides --host and --port)
  --host string   Management host (default: 127.0.0.1)
  --port int      Management port (default: 47831)
  --json          Output as machine-readable JSON
  --version       Show CLI version and exit
  -h, --help      Show this help message

Environment:
  PORTIER_URL     Default management URL (overridden by --url / --host / --port)

Commands:
  runtime              Show runtime info for the running Portier service
  list                 List configured forwarding rules
  status               Show runtime status for forwarding rules
  activity             Show recent activity events (--limit, --rule, --type, --severity)
  start <id|name>      Start a forwarding rule
  stop <id|name>       Stop a forwarding rule
  diagnose <id|name>   Run diagnostics against a forwarding rule
  doctor               Run live runtime diagnostic checks (reachability, version, health)
  explain <code>       Explain a doctor/check code (offline; 'portier explain --list')
  policy               Evaluate a config against a policy file (offline; 'portier policy check')
  support-bundle       Collect doctor artifacts into a directory ('--out <dir>')
  config               Manage config: validate, doctor, export, import (run 'portier config help')
  group                Operate on rules by group: list, start, stop (run 'portier group help')
  diagnostics          Build diagnostics support bundles (run 'portier diagnostics help')
  version              Show CLI version
  help                 Show this help message

Rule identity:
  Commands that target a rule accept an exact rule ID or an exact rule name.
  If multiple rules share the same name, use the rule ID to avoid ambiguity.

Exit codes:
  0  Success
  1  General error or API error
  2  Invalid arguments or usage error
  3  Connection failure — Portier service unreachable
`

// PrintHelp writes the CLI help text to w.
func PrintHelp(w io.Writer) {
	fmt.Fprint(w, helpText)
}

// PrintVersion writes the CLI version line to w.
func PrintVersion(w io.Writer) {
	fmt.Fprintf(w, "Portier CLI %s\n", version.Version)
}

// exitWithError writes err to stderr and returns the appropriate exit code.
func exitWithError(err error, stderr io.Writer) int {
	var connErr *client.ConnectionError
	if errors.As(err, &connErr) {
		fmt.Fprintf(stderr, "Error: %s\n", connErr)
		fmt.Fprintln(stderr, "Hint: Is the Portier service running?")
		return 3
	}
	fmt.Fprintf(stderr, "Error: %v\n", err)
	return 1
}
