package commands

// `portier workflow history` handlers (v1.12 Slice 1): list/show/clear the local,
// opt-in workflow run history recorded by `workflow run --record-history`. These
// commands are fully offline — they read/write the local history file only and
// never contact the runtime. The history model, projection, and storage live in
// the `workflow` package; this file stays handlers-only (flag parsing, path
// resolution, and rendering).

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"portier/cli/sources/output"
	"portier/cli/sources/workflow"
)

const workflowHistoryHelp = `Usage: portier workflow history <list|show|clear> [options]

Inspect the local, opt-in workflow run history recorded by
'portier workflow run --record-history'. Fully offline: these commands read and
write a local history file only and never contact the runtime.

History stores compact metadata only — run id, time, workflow name, result,
summary counts, compact step metadata (id/type/status/exitCode), and the emitted
explanation/finding codes. It NEVER stores raw configs, policies, full reports,
file contents, logs, environment variables, process data, secrets, runtime URLs,
or tokens. History is bounded to the most recent 100 runs.

Subcommands:
  list                 List recorded runs (newest first).
  show <run-id>        Show one recorded run by id.
  export --out <file>  Write a compact JSON snapshot of the history to a file.
  clear --yes          Delete the local history file (requires --yes).

Options:
  --json           Output as machine-readable JSON (list/show; with export, also
                   print the snapshot to stdout, byte-identical to the file).
  --out <file>     Output file for the export snapshot (export).
  --yes            Confirm deletion (clear).

Exit codes:
  0  Success (including 'list'/'export' with no runs and 'clear' when already
     empty)
  1  An unknown run id ('show'), or a history read/write failure
  2  Missing/invalid arguments (a missing run id for 'show', a missing --out value
     for 'export', or 'clear' without --yes)

The export snapshot is compact metadata only — it never contains raw configs,
policies, full reports, logs, environment variables, process data, runtime URLs,
or tokens — and includes an explicit 'safety' object stating those exclusions.
Export reads the history file only; it never mutates it, contacts the runtime,
executes a workflow, or reads the files a workflow step refers to.

Examples:
  portier workflow run --file workflow.json --record-history
  portier workflow history list
  portier --json workflow history list
  portier workflow history show 20260614T101530Z-policy-baseline-check-a1b2c3d4
  portier workflow history export --out workflow-history-export.json
  portier --json workflow history export --out workflow-history-export.json
  portier workflow history clear --yes
`

// historyStorePath resolves the local workflow-history file path. It is a package
// variable so tests can point it at a temporary directory without touching the
// real per-user config location (and without an env override in a workflow
// handler). The default is <user-config-dir>/portier/workflow-history.json.
var historyStorePath = func() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "portier", workflow.HistoryFileName), nil
}

// newHistorySuffix produces the short random suffix for a run id. It is a package
// variable so tests can make ids deterministic.
var newHistorySuffix = func() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is extraordinary; fall back to a fixed marker rather
		// than failing the (already completed) run's history record.
		return "00000000"
	}
	return hex.EncodeToString(b[:])
}

// historyStore builds the history store at the resolved path.
func historyStore() (*workflow.HistoryStore, error) {
	path, err := historyStorePath()
	if err != nil {
		return nil, err
	}
	return workflow.NewHistoryStore(path), nil
}

// RunWorkflowHistory dispatches `portier workflow history <subcommand>`. It is
// fully offline. Exit codes follow the workflow-history policy documented in
// workflowHistoryHelp.
func RunWorkflowHistory(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, workflowHistoryHelp)
		return 2
	}
	switch args[0] {
	case "list":
		return runWorkflowHistoryList(jsonOutput, args[1:], stdout, stderr)
	case "show":
		return runWorkflowHistoryShow(jsonOutput, args[1:], stdout, stderr)
	case "export":
		return runWorkflowHistoryExport(jsonOutput, args[1:], stdout, stderr)
	case "clear":
		return runWorkflowHistoryClear(args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, workflowHistoryHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown workflow history subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, workflowHistoryHelp)
		return 2
	}
}

// runWorkflowHistoryList prints all recorded runs (newest first), or a clear empty
// message. Exit codes: 0 success (including empty), 1 a history read failure.
func runWorkflowHistoryList(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow history list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if code, done := parseHistoryFlags(fs, args, stdout, stderr); done {
		return code
	}

	store, err := historyStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not resolve history path: %v\n", err)
		return 1
	}
	hist, err := store.Load()
	if err != nil {
		fmt.Fprintf(stderr, "Error reading history %s: %v\n", store.Path(), err)
		return 1
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, hist); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintln(stdout, "Portier Workflow History")
	fmt.Fprintln(stdout)
	if len(hist.Runs) == 0 {
		fmt.Fprintln(stdout, "No workflow runs recorded.")
		fmt.Fprintln(stdout, "Record a run with 'portier workflow run --file <workflow.json> --record-history'.")
		return 0
	}
	for _, r := range hist.Runs {
		printHistoryRunHuman(r, stdout)
		fmt.Fprintln(stdout)
	}
	return 0
}

// runWorkflowHistoryShow prints one recorded run by id. Exit codes: 0 success, 1 an
// unknown run id or a history read failure, 2 a missing run id argument.
func runWorkflowHistoryShow(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow history show", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if code, done := parseHistoryFlags(fs, args, stdout, stderr); done {
		return code
	}

	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(stderr, "Error: a run id is required")
		fmt.Fprint(stderr, workflowHistoryHelp)
		return 2
	}
	if len(rest) > 1 {
		fmt.Fprintf(stderr, "Error: too many arguments (expected one run id, got %d)\n\n", len(rest))
		fmt.Fprint(stderr, workflowHistoryHelp)
		return 2
	}
	id := rest[0]

	store, err := historyStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not resolve history path: %v\n", err)
		return 1
	}
	hist, err := store.Load()
	if err != nil {
		fmt.Fprintf(stderr, "Error reading history %s: %v\n", store.Path(), err)
		return 1
	}

	for _, r := range hist.Runs {
		if r.ID == id {
			if jsonOutput {
				if err := output.PrintJSON(stdout, r); err != nil {
					fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
					return 1
				}
				return 0
			}
			printHistoryRunHuman(r, stdout)
			return 0
		}
	}

	fmt.Fprintf(stderr, "Error: no recorded run with id %q\n", id)
	return 1
}

// runWorkflowHistoryExport writes a compact JSON snapshot of the local history to
// the --out file. It is fully offline and read-only: it loads the history store,
// builds the deterministic export snapshot in the workflow package, and writes it
// — it never mutates the store, contacts the runtime, executes a workflow, or
// reads workflow-referenced files. A missing/empty history produces a valid empty
// snapshot (runCount 0). Exit codes: 0 success, 1 a history read failure / output
// write failure / JSON-encode failure, 2 a missing --out value.
func runWorkflowHistoryExport(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow history export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagOut := fs.String("out", "", "output file for the export snapshot")
	if code, done := parseHistoryFlags(fs, args, stdout, stderr); done {
		return code
	}
	if *flagOut == "" {
		fmt.Fprintln(stderr, "Error: --out is required")
		fmt.Fprint(stderr, workflowHistoryHelp)
		return 2
	}

	store, err := historyStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not resolve history path: %v\n", err)
		return 1
	}
	hist, err := store.Load()
	if err != nil {
		fmt.Fprintf(stderr, "Error reading history %s: %v\n", store.Path(), err)
		return 1
	}

	// Build the snapshot once so the --json stdout output and the file are
	// byte-identical (output.PrintJSON and output.WritePrettyJSON produce the same
	// indented bytes for the same value).
	export := workflow.BuildHistoryExport(hist, time.Now())

	if jsonOutput {
		if err := output.PrintJSON(stdout, export); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	}
	if err := output.WritePrettyJSON(*flagOut, export); err != nil {
		fmt.Fprintf(stderr, "Error writing %s: %v\n", *flagOut, err)
		return 1
	}
	if !jsonOutput {
		printHistoryExportHuman(*flagOut, export, stdout)
	}
	return 0
}

// runWorkflowHistoryClear deletes the local history file. It requires --yes to
// avoid accidental deletion. Exit codes: 0 cleared (or already empty/missing), 2
// without --yes, 1 a delete failure.
func runWorkflowHistoryClear(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow history clear", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagYes := fs.Bool("yes", false, "confirm deletion of the history file")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, workflowHistoryHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, workflowHistoryHelp)
		return 2
	}
	if !*flagYes {
		fmt.Fprintln(stderr, "Error: clear requires --yes to confirm deletion")
		fmt.Fprint(stderr, workflowHistoryHelp)
		return 2
	}

	store, err := historyStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not resolve history path: %v\n", err)
		return 1
	}
	if err := store.Clear(); err != nil {
		fmt.Fprintf(stderr, "Error clearing history %s: %v\n", store.Path(), err)
		return 1
	}
	fmt.Fprintln(stdout, "Workflow history cleared.")
	return 0
}

// parseHistoryFlags parses the shared list/show flag set (currently none beyond
// the global --json), handling -h/--help and parse errors. It returns (code, true)
// when the caller should return code, or (0, false) to continue.
func parseHistoryFlags(fs *flag.FlagSet, args []string, stdout, stderr io.Writer) (int, bool) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, workflowHistoryHelp)
			return 0, true
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, workflowHistoryHelp)
		return 2, true
	}
	return 0, false
}

// printHistoryRunHuman renders one recorded run in a compact, deterministic block.
func printHistoryRunHuman(r workflow.HistoryRun, w io.Writer) {
	fmt.Fprintln(w, r.ID)
	fmt.Fprintf(w, "  Workflow: %s\n", historyWorkflowName(r.Workflow))
	fmt.Fprintf(w, "  Result: %s\n", r.Result)
	fmt.Fprintf(w, "  Steps: %d total, %d passed, %d failed, %d skipped\n",
		r.Summary.Total, r.Summary.Passed, r.Summary.Failed, r.Summary.Skipped)
	fmt.Fprintf(w, "  Created: %s\n", r.CreatedAt)
	if len(r.Codes) > 0 {
		fmt.Fprintf(w, "  Codes: %s\n", strings.Join(r.Codes, ", "))
	}
}

// printHistoryExportHuman renders the export confirmation, including the run count
// and a fixed safety block restating what the snapshot excludes.
func printHistoryExportHuman(outPath string, export workflow.HistoryExport, w io.Writer) {
	fmt.Fprintln(w, "Workflow history export written.")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "File: %s\n", outPath)
	fmt.Fprintf(w, "Runs: %d\n", export.RunCount)
	fmt.Fprintln(w, "Safety:")
	fmt.Fprintln(w, "- Compact metadata only")
	fmt.Fprintln(w, "- No raw configs or policies")
	fmt.Fprintln(w, "- No full reports")
	fmt.Fprintln(w, "- No logs, environment variables, process data, runtime URLs, or tokens")
}

// historyWorkflowName returns the workflow name, or a placeholder when unnamed.
func historyWorkflowName(name string) string {
	if name == "" {
		return "(unnamed)"
	}
	return name
}
