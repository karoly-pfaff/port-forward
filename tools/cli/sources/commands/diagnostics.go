package commands

import (
	"flag"
	"fmt"
	"io"
	"time"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
	"portier/cli/sources/version"
)

const diagnosticsHelp = `Usage: portier diagnostics <subcommand> [options]

Generate diagnostics support bundles from the running Portier service.

Subcommands:
  export --out <file>   Build and write a diagnostics bundle to a file

Run 'portier diagnostics export --help' for options.
`

const diagnosticsExportHelp = `Usage: portier diagnostics export [--out <file>] [options]

Build a JSON diagnostics bundle from the running Portier service.

The bundle includes: runtime info, forwarding rules, rule statuses, and recent activity.
It does not include logs, environment variables, OS usernames, or raw files from disk.

Options:
  --out string             Output file path. Required in human mode; omit with --json to print to stdout.
  --run-diagnostics        Run diagnostics for each rule and include results in the bundle.
  --activity-limit int     Maximum activity events to include (1–500, default: 100).

JSON behavior:
  With --out:    Prints a stable result object: { "ok": true, "path": "...", ... }
  Without --out: Prints the full diagnostics bundle to stdout.

Examples:
  portier diagnostics export --out bundle.json
  portier diagnostics export --out bundle.json --run-diagnostics
  portier diagnostics export --out bundle.json --activity-limit 50
  portier --json diagnostics export --out bundle.json
  portier --json diagnostics export                          # print bundle to stdout
`

// RunDiagnostics dispatches portier diagnostics subcommands.
func RunDiagnostics(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, diagnosticsHelp)
		return 2
	}
	switch args[0] {
	case "export":
		return RunDiagnosticsExport(c, jsonOutput, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, diagnosticsHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown diagnostics subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, diagnosticsHelp)
		return 2
	}
}

// RunDiagnosticsExport implements `portier diagnostics export`.
func RunDiagnosticsExport(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diagnostics export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagOut := fs.String("out", "", "output file path")
	flagRunDiagnostics := fs.Bool("run-diagnostics", false, "run diagnostics for each rule")
	flagActivityLimit := fs.Int("activity-limit", 100, "maximum activity events (1-500)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, diagnosticsExportHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, diagnosticsExportHelp)
		return 2
	}

	if *flagActivityLimit < 1 || *flagActivityLimit > 500 {
		fmt.Fprintln(stderr, "Error: --activity-limit must be between 1 and 500")
		fmt.Fprint(stderr, diagnosticsExportHelp)
		return 2
	}

	if *flagOut == "" && !jsonOutput {
		fmt.Fprintln(stderr, "Error: --out is required in human mode")
		fmt.Fprintln(stderr, "Use --out <file> to write to a file, or add --json to print to stdout.")
		return 2
	}

	bundle := buildDiagnosticsBundle(c, *flagRunDiagnostics, *flagActivityLimit)

	if *flagOut == "" {
		if err := output.PrintJSON(stdout, bundle); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	if err := writePrettyJSON(*flagOut, bundle); err != nil {
		fmt.Fprintf(stderr, "Error writing %s: %v\n", *flagOut, err)
		return 1
	}

	warningCount := len(bundle.Errors)

	if jsonOutput {
		result := map[string]any{
			"ok":              true,
			"path":            *flagOut,
			"ruleCount":       len(bundle.Rules),
			"statusCount":     len(bundle.Statuses),
			"activityCount":   len(bundle.Activity.Events),
			"diagnosticCount": len(bundle.Diagnostics),
		}
		if warningCount > 0 {
			result["warningCount"] = warningCount
		}
		if err := output.PrintJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	if warningCount > 0 {
		fmt.Fprintf(stdout, "Exported diagnostics with warnings to %s\n", *flagOut)
	} else {
		fmt.Fprintf(stdout, "Exported diagnostics to %s\n", *flagOut)
	}
	fmt.Fprintf(stdout, "  %d %s, %d statuses, %d activity events",
		len(bundle.Rules), pluralRule(len(bundle.Rules)),
		len(bundle.Statuses), len(bundle.Activity.Events))
	if *flagRunDiagnostics {
		fmt.Fprintf(stdout, ", %d diagnostics results", len(bundle.Diagnostics))
	}
	fmt.Fprintln(stdout)
	return 0
}

// --- bundle types ---

type diagnosticsBundle struct {
	SchemaVersion   string                                   `json:"schemaVersion"`
	ExportedAt      string                                   `json:"exportedAt"`
	App             bundleApp                                `json:"app"`
	Runtime         *client.RuntimeInfo                      `json:"runtime"`
	Rules           []client.ForwardRuleResponse             `json:"rules"`
	Statuses        []client.ForwardStatus                   `json:"statuses"`
	Diagnostics     map[string]*client.RuleDiagnosticsResult `json:"diagnostics"`
	DiagnosticsNote string                                   `json:"diagnosticsNote,omitempty"`
	Activity        bundleActivity                           `json:"activity"`
	Metadata        bundleMetadata                           `json:"metadata"`
	Errors          []bundleError                            `json:"errors"`
}

type bundleApp struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type bundleActivity struct {
	Included bool                   `json:"included"`
	Events   []client.ActivityEvent `json:"events"`
	Note     string                 `json:"note,omitempty"`
}

type bundleMetadata struct {
	ManagementURL string `json:"managementUrl"`
	Source        string `json:"source"`
	GeneratedBy   string `json:"generatedBy"`
}

type bundleError struct {
	Source  string `json:"source"`
	Message string `json:"message"`
}

// buildDiagnosticsBundle fetches data from the management API and assembles a diagnostics bundle.
// Individual source failures are recorded in bundle.Errors rather than aborting the build.
func buildDiagnosticsBundle(c *client.Client, runDiagnostics bool, activityLimit int) diagnosticsBundle {
	bundle := diagnosticsBundle{
		SchemaVersion: "1",
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		App:           bundleApp{Name: "Portier", Version: version.Version},
		Diagnostics:   map[string]*client.RuleDiagnosticsResult{},
		Errors:        make([]bundleError, 0),
		Metadata: bundleMetadata{
			ManagementURL: c.BaseURL(),
			Source:        "cli",
			GeneratedBy:   "portier diagnostics export",
		},
	}

	rt, err := c.GetRuntime()
	if err != nil {
		bundle.Errors = append(bundle.Errors, bundleError{Source: "runtime", Message: err.Error()})
	} else {
		bundle.Runtime = rt
	}

	rulesFetched := false
	rules, err := c.GetForwards()
	if err != nil {
		bundle.Errors = append(bundle.Errors, bundleError{Source: "rules", Message: err.Error()})
		bundle.Rules = make([]client.ForwardRuleResponse, 0)
	} else {
		if rules == nil {
			rules = make([]client.ForwardRuleResponse, 0)
		}
		bundle.Rules = rules
		rulesFetched = true
	}

	statuses, err := c.GetStatus()
	if err != nil {
		bundle.Errors = append(bundle.Errors, bundleError{Source: "statuses", Message: err.Error()})
		bundle.Statuses = make([]client.ForwardStatus, 0)
	} else {
		if statuses == nil {
			statuses = make([]client.ForwardStatus, 0)
		}
		bundle.Statuses = statuses
	}

	events, err := c.GetActivity(client.ActivityQuery{Limit: activityLimit})
	if err != nil {
		bundle.Errors = append(bundle.Errors, bundleError{Source: "activity", Message: err.Error()})
		bundle.Activity = bundleActivity{
			Included: false,
			Events:   make([]client.ActivityEvent, 0),
		}
	} else {
		if events == nil {
			events = make([]client.ActivityEvent, 0)
		}
		bundle.Activity = bundleActivity{
			Included: true,
			Events:   events,
			Note:     fmt.Sprintf("Up to %d most recent events included.", activityLimit),
		}
	}

	if !runDiagnostics {
		bundle.DiagnosticsNote = "Rule diagnostics were not run. Use --run-diagnostics to include current diagnostic results."
	} else if !rulesFetched {
		bundle.DiagnosticsNote = "Rule diagnostics were not run because the rules list could not be fetched."
	} else if len(bundle.Rules) == 0 {
		bundle.DiagnosticsNote = "No rules were available for diagnostics."
	} else {
		for _, rule := range bundle.Rules {
			result, err := c.DiagnoseForward(rule.ID)
			if err != nil {
				bundle.Errors = append(bundle.Errors, bundleError{
					Source:  "diagnostics:" + rule.ID,
					Message: err.Error(),
				})
			} else {
				bundle.Diagnostics[rule.ID] = result
			}
		}
	}

	return bundle
}
