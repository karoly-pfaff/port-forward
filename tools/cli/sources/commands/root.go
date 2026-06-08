package commands

import (
	"fmt"
	"io"

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
  runtime         Show runtime info for the running Portier service
  version         Show CLI version
  help            Show this help message

More commands are planned for v1.3:
  list, status, activity, start, stop, diagnose, config export/import,
  diagnostics export

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
