package commands

import (
	"flag"
	"fmt"
	"io"

	"portier/cli/sources/client"
	"portier/cli/sources/doctor"
)

const doctorRuntimeHelp = `Usage: portier doctor

Run deterministic diagnostic checks against the live Portier runtime.
Read-only: never mutates the runtime or its config, and never probes targets.

Checks (each produces a stable check code):
  runtime.reachable / runtime.unreachable   Can the runtime be reached?
  runtime.version                           Runtime version (warns on CLI/runtime mismatch).
  runtime.status_read / runtime.status_failed   Could rule status be read?
  rules.none / rules.present                Are any forwarding rules configured?
  rules.health_ok / rules.health_warning / rules.health_error
                                            Aggregated rule health (from the API's health field).
  config.export_read / config.export_failed   Can the current config be read (read-only)?

Options:
  --strict       Treat warnings as failures (a warning-only report exits 1).
  --explain      Show an explanation (meaning + next action) for each emitted check.
  --out <file>   Also write the JSON report to <file> (same shape as --json).

Output:
  Human report by default; --json emits the full doctor report
  (checks + summary + strict + result, plus an additive explanations map with
  --explain). --out writes that same JSON to a file regardless of --json; with
  --json the JSON also prints to stdout.

Exit codes:
  0  Doctor completed; no error-severity checks (warnings alone exit 0 unless --strict)
  1  Doctor completed; one or more error-severity checks (incl. unreachable runtime),
     any warning when --strict is set, or the --out file could not be written
  2  Usage error (unexpected argument)

Note: an unreachable runtime is reported as a doctor check and exits 1 (the
doctor ran and found a problem), NOT the usual connection exit code 3.

Examples:
  portier doctor
  portier doctor --strict
  portier doctor --explain
  portier doctor --out doctor-report.json
  portier --json doctor
  portier --host 127.0.0.1 --port 47831 doctor
`

// RunDoctor runs deterministic diagnostic checks against the live Portier
// runtime and prints a doctor report. It is read-only: it never mutates the
// runtime or config and never probes forwarding targets. Exit codes follow the
// doctor policy: 0 = no error-severity checks, 1 = one or more error-severity
// checks (including an unreachable runtime), 2 = usage error.
func RunDoctor(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagStrict := fs.Bool("strict", false, "treat warnings as failures (exit 1)")
	flagExplain := fs.Bool("explain", false, "show an explanation for each emitted check")
	flagOut := fs.String("out", "", "also write the JSON report to this file")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, doctorRuntimeHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, doctorRuntimeHelp)
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "Error: doctor takes no arguments (got %q)\n\n", fs.Arg(0))
		fmt.Fprint(stderr, doctorRuntimeHelp)
		return 2
	}

	report := doctor.RunLiveChecks(c)
	return doctor.Emit("Portier Doctor", report, doctor.EmitOptions{
		Strict: *flagStrict, Explain: *flagExplain, JSON: jsonOutput, OutPath: *flagOut,
	}, stdout, stderr)
}
