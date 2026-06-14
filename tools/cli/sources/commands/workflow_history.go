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

const workflowHistoryHelp = `Usage: portier workflow history <list|stats|show|export|prune|clear> [options]

Inspect the local, opt-in workflow run history recorded by
'portier workflow run --record-history'. Fully offline: these commands read and
write a local history file only and never contact the runtime.

History stores compact metadata only — run id, time, workflow name, result,
summary counts, compact step metadata (id/type/status/exitCode), and the emitted
explanation/finding codes. It NEVER stores raw configs, policies, full reports,
file contents, logs, environment variables, process data, secrets, runtime URLs,
or tokens. History is bounded to the most recent 100 runs.

Subcommands:
  list [filters]       List recorded runs (newest first), optionally filtered.
  stats [filters]      Summarize the (optionally filtered) history.
  show <run-id>        Show one recorded run by id.
  export --out <file>  Write a compact JSON snapshot of the history to a file.
  prune <mode> --yes   Remove entries (by retention or filter; requires --yes).
  clear --yes          Delete the local history file (requires --yes).

List/stats filters (AND-combined; all operate only on local compact history):
  --result <passed|failed>  Keep only runs with this result.
  --workflow <name>         Keep only runs with this exact workflow name.
  --code <code>             Keep only runs whose codes contain this exact code.
  --limit <n>               Keep at most the newest N matching runs (n > 0).

Prune modes (exactly one; destructive, requires --yes; never accepts --limit):
  --keep <n>                Keep the newest N entries, remove older ones (n >= 0).
  --result/--workflow/--code  Remove entries matching these filters (AND-combined).

Options:
  --json           Output as machine-readable JSON (list/show/stats/prune; with
                   export, also print the snapshot to stdout, byte-identical to
                   the file).
  --out <file>     Output file for the export snapshot (export).
  --yes            Confirm the destructive operation (prune, clear).

Exit codes:
  0  Success (including 'list'/'stats' with no matches, 'list'/'stats'/'export'
     with no runs, 'prune' that removed 0 entries, and 'clear' when already empty)
  1  An unknown run id ('show'), or a history read/write failure
  2  Missing/invalid arguments (an invalid 'list'/'stats' filter value, a missing
     run id for 'show', a missing --out value for 'export', a missing/ambiguous
     'prune' mode or invalid --keep/filter value, or 'prune'/'clear' without
     --yes)

The export snapshot is compact metadata only — it never contains raw configs,
policies, full reports, logs, environment variables, process data, runtime URLs,
or tokens — and includes an explicit 'safety' object stating those exclusions.
Export reads the history file only; it never mutates it, contacts the runtime,
executes a workflow, or reads the files a workflow step refers to.

Examples:
  portier workflow run --file workflow.json --record-history
  portier workflow history list
  portier workflow history list --result failed --limit 10
  portier workflow history list --workflow policy-baseline-check
  portier --json workflow history list --code policy.lan_exposure_forbidden
  portier workflow history stats
  portier --json workflow history stats --result failed
  portier workflow history prune --keep 50 --yes
  portier workflow history prune --result passed --workflow nightly-check --yes
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
	case "stats":
		return runWorkflowHistoryStats(jsonOutput, args[1:], stdout, stderr)
	case "prune":
		return runWorkflowHistoryPrune(jsonOutput, args[1:], stdout, stderr)
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

// runWorkflowHistoryList prints recorded runs (newest first), optionally filtered
// by result/workflow/code/limit (AND-combined). Filtering operates only on the
// compact local history; it never contacts the runtime or reads workflow files.
// Exit codes: 0 success (including an empty match set), 1 a history read failure,
// 2 an invalid filter value.
func runWorkflowHistoryList(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow history list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	filters, code, done := parseHistoryFilters(fs, args, stdout, stderr)
	if done {
		return code
	}

	hist, code, done := loadHistoryForRead(stderr)
	if done {
		return code
	}

	view := workflow.BuildHistoryListView(hist, filters)
	if jsonOutput {
		if err := output.PrintJSON(stdout, view); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}
	printHistoryListHuman(view, stdout)
	return 0
}

// runWorkflowHistoryStats summarizes the (optionally filtered) local history by
// result, workflow, step status/type, and emitted code. It reuses the same
// filters as `list` (applied before computing stats) and reads only compact
// history metadata — it never contacts the runtime or reads workflow files. Exit
// codes: 0 success (including empty/missing history and empty match sets), 1 a
// history read failure, 2 an invalid filter value.
func runWorkflowHistoryStats(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow history stats", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	filters, code, done := parseHistoryFilters(fs, args, stdout, stderr)
	if done {
		return code
	}

	hist, code, done := loadHistoryForRead(stderr)
	if done {
		return code
	}

	stats := workflow.BuildHistoryStats(hist, filters)
	if jsonOutput {
		if err := output.PrintJSON(stdout, stats); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}
	printHistoryStatsHuman(stats, stdout)
	return 0
}

// runWorkflowHistoryPrune removes entries from the local history, either by
// retention (--keep N keeps the newest N) or by filter (--result/--workflow/--code
// remove matching entries). Exactly one mode is required, and the destructive
// operation requires --yes. It mutates ONLY the compact local history store — it
// never contacts the runtime, executes a workflow, reads workflow files, or
// touches configs/policies/reports. Exit codes: 0 success (including removed 0),
// 1 a history read/write failure, 2 a usage error (no/ambiguous mode, invalid
// keep/filter value, or missing --yes).
func runWorkflowHistoryPrune(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow history prune", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagKeep := fs.Int("keep", 0, "keep the newest N entries, removing older ones")
	flagResult := fs.String("result", "", "remove runs with this result (passed|failed)")
	flagWorkflow := fs.String("workflow", "", "remove runs with this exact workflow name")
	flagCode := fs.String("code", "", "remove runs whose codes contain this exact code")
	flagYes := fs.Bool("yes", false, "confirm the destructive prune")
	if code, done := parseHistoryFlags(fs, args, stdout, stderr); done {
		return code
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	keepMode := set["keep"]
	filterMode := set["result"] || set["workflow"] || set["code"]

	// Exactly one explicit mode; the two modes are mutually exclusive.
	switch {
	case !keepMode && !filterMode:
		return historyUsageError(stderr, "prune requires a mode: --keep <n> or at least one filter (--result/--workflow/--code)")
	case keepMode && filterMode:
		return historyUsageError(stderr, "--keep cannot be combined with filters")
	}

	var opts workflow.HistoryPruneOptions
	if keepMode {
		if *flagKeep < 0 {
			return historyUsageError(stderr, fmt.Sprintf("--keep must be a non-negative integer (got %d)", *flagKeep))
		}
		keep := *flagKeep
		opts.Keep = &keep
	} else {
		if set["result"] && !workflow.ValidHistoryResult(*flagResult) {
			return historyUsageError(stderr, fmt.Sprintf("invalid --result %q (expected one of: passed, failed)", *flagResult))
		}
		if set["workflow"] && *flagWorkflow == "" {
			return historyUsageError(stderr, "--workflow requires a non-empty value")
		}
		if set["code"] && *flagCode == "" {
			return historyUsageError(stderr, "--code requires a non-empty value")
		}
		opts.Filters = workflow.HistoryFilters{Result: *flagResult, Workflow: *flagWorkflow, Code: *flagCode}
	}

	// Destructive: require explicit confirmation BEFORE touching the store.
	if !*flagYes {
		return historyUsageError(stderr, "prune is destructive and requires --yes to confirm")
	}

	store, code, done := loadHistoryStore(stderr)
	if done {
		return code
	}
	hist, err := store.Load()
	if err != nil {
		fmt.Fprintf(stderr, "Error reading history %s: %v\n", store.Path(), err)
		return 1
	}

	kept, result := workflow.PruneHistory(hist, opts)
	if result.Removed > 0 {
		if err := store.Save(kept); err != nil {
			fmt.Fprintf(stderr, "Error writing history %s: %v\n", store.Path(), err)
			return 1
		}
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}
	printHistoryPruneHuman(result, stdout)
	return 0
}

// parseHistoryFilters defines and parses the shared list/stats filter flags
// (--result/--workflow/--code/--limit) on fs, then validates them. It returns the
// resolved filters, or (filters, exitCode, true) when the caller should return
// exitCode (help/parse error → from parseHistoryFlags; an invalid filter value →
// 2). fs.Visit distinguishes an explicitly-set flag from its default so an empty
// --workflow/--code or a non-positive --limit is rejected while an absent flag
// means "no filter". A non-numeric --limit already failed flag parsing → 2.
func parseHistoryFilters(fs *flag.FlagSet, args []string, stdout, stderr io.Writer) (workflow.HistoryFilters, int, bool) {
	flagResult := fs.String("result", "", "filter by run result (passed|failed)")
	flagWorkflow := fs.String("workflow", "", "filter by exact workflow name")
	flagCode := fs.String("code", "", "filter by an emitted code (exact match)")
	flagLimit := fs.Int("limit", 0, "keep at most the newest N matching runs")
	if code, done := parseHistoryFlags(fs, args, stdout, stderr); done {
		return workflow.HistoryFilters{}, code, true
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if set["result"] && !workflow.ValidHistoryResult(*flagResult) {
		return workflow.HistoryFilters{}, historyUsageError(stderr, fmt.Sprintf("invalid --result %q (expected one of: passed, failed)", *flagResult)), true
	}
	if set["workflow"] && *flagWorkflow == "" {
		return workflow.HistoryFilters{}, historyUsageError(stderr, "--workflow requires a non-empty value"), true
	}
	if set["code"] && *flagCode == "" {
		return workflow.HistoryFilters{}, historyUsageError(stderr, "--code requires a non-empty value"), true
	}
	if set["limit"] && *flagLimit <= 0 {
		return workflow.HistoryFilters{}, historyUsageError(stderr, fmt.Sprintf("--limit must be a positive integer (got %d)", *flagLimit)), true
	}
	return workflow.HistoryFilters{Result: *flagResult, Workflow: *flagWorkflow, Code: *flagCode, Limit: *flagLimit}, 0, false
}

// loadHistoryStore resolves the history store path. It returns (store, exitCode,
// true) when the caller should return exitCode (1 on a path-resolution failure).
func loadHistoryStore(stderr io.Writer) (*workflow.HistoryStore, int, bool) {
	store, err := historyStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not resolve history path: %v\n", err)
		return nil, 1, true
	}
	return store, 0, false
}

// loadHistoryForRead resolves the store path and loads the history for a
// read-only command. It returns (history, exitCode, true) when the caller should
// return exitCode (1 on a path-resolution or read failure).
func loadHistoryForRead(stderr io.Writer) (workflow.History, int, bool) {
	store, code, done := loadHistoryStore(stderr)
	if done {
		return workflow.History{}, code, true
	}
	hist, err := store.Load()
	if err != nil {
		fmt.Fprintf(stderr, "Error reading history %s: %v\n", store.Path(), err)
		return workflow.History{}, 1, true
	}
	return hist, 0, false
}

// historyUsageError prints a filter usage error + help and returns exit code 2.
func historyUsageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "Error: %s\n", msg)
	fmt.Fprint(stderr, workflowHistoryHelp)
	return 2
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

// printHistoryListHuman renders a (possibly filtered) history list: an optional
// Filters block, the matched runs, and a shown/total-stored summary. The pristine
// first-run case (no filters, nothing stored) keeps the original guidance message
// with no summary block.
func printHistoryListHuman(view workflow.HistoryListView, w io.Writer) {
	fmt.Fprintln(w, "Portier Workflow History")
	fmt.Fprintln(w)

	if view.Filters == nil && view.TotalStored == 0 {
		fmt.Fprintln(w, "No workflow runs recorded.")
		fmt.Fprintln(w, "Record a run with 'portier workflow run --file <workflow.json> --record-history'.")
		return
	}

	printHistoryFilterBlock(view.Filters, w)

	if len(view.Runs) == 0 {
		fmt.Fprintln(w, "No workflow history entries matched the filters.")
		fmt.Fprintln(w)
	} else {
		for _, r := range view.Runs {
			printHistoryRunHuman(r, w)
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "- %d shown\n", view.Shown)
	fmt.Fprintf(w, "- %d total stored\n", view.TotalStored)
}

// printHistoryFilterBlock prints the active-filters block (followed by a blank
// line) when any filter is set; it prints nothing when filters is nil.
func printHistoryFilterBlock(f *workflow.HistoryFilters, w io.Writer) {
	if f == nil {
		return
	}
	fmt.Fprintln(w, "Filters:")
	if f.Result != "" {
		fmt.Fprintf(w, "- Result: %s\n", f.Result)
	}
	if f.Workflow != "" {
		fmt.Fprintf(w, "- Workflow: %s\n", f.Workflow)
	}
	if f.Code != "" {
		fmt.Fprintf(w, "- Code: %s\n", f.Code)
	}
	if f.Limit > 0 {
		fmt.Fprintf(w, "- Limit: %d\n", f.Limit)
	}
	fmt.Fprintln(w)
}

// printHistoryStatsHuman renders the history stats: an optional Filters block, the
// run/step buckets, and the deterministically-sorted grouped tables. Empty/missing
// history (nothing stored) prints a short "no history" form. Empty grouped tables
// are omitted.
func printHistoryStatsHuman(stats workflow.HistoryStats, w io.Writer) {
	fmt.Fprintln(w, "Portier Workflow History Stats")
	fmt.Fprintln(w)

	printHistoryFilterBlock(stats.Filters, w)

	if stats.TotalStored == 0 {
		fmt.Fprintln(w, "Runs:")
		fmt.Fprintf(w, "- %d shown\n", stats.Shown)
		fmt.Fprintf(w, "- %d total stored\n", stats.TotalStored)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Result: no history")
		return
	}

	fmt.Fprintln(w, "Runs:")
	fmt.Fprintf(w, "- %d shown\n", stats.Shown)
	fmt.Fprintf(w, "- %d total stored\n", stats.TotalStored)
	fmt.Fprintf(w, "- %d passed\n", stats.Results.Passed)
	fmt.Fprintf(w, "- %d failed\n", stats.Results.Failed)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Steps:")
	fmt.Fprintf(w, "- %d passed\n", stats.Steps.Passed)
	fmt.Fprintf(w, "- %d failed\n", stats.Steps.Failed)
	fmt.Fprintf(w, "- %d skipped\n", stats.Steps.Skipped)

	if len(stats.Workflows) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Workflows:")
		for _, wc := range stats.Workflows {
			fmt.Fprintf(w, "- %s: %d\n", wc.Name, wc.Count)
		}
	}
	if len(stats.StepTypes) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Step types:")
		for _, st := range stats.StepTypes {
			fmt.Fprintf(w, "- %s: %d\n", st.Type, st.Count)
		}
	}
	if len(stats.Codes) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Codes:")
		for _, cc := range stats.Codes {
			fmt.Fprintf(w, "- %s: %d\n", cc.Code, cc.Count)
		}
	}
}

// printHistoryPruneHuman renders the prune outcome (removed/kept/before/after).
func printHistoryPruneHuman(result workflow.HistoryPruneResult, w io.Writer) {
	fmt.Fprintln(w, "Workflow history pruned.")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Removed: %d\n", result.Removed)
	fmt.Fprintf(w, "Kept: %d\n", result.Kept)
	fmt.Fprintf(w, "Total before: %d\n", result.TotalBefore)
	fmt.Fprintf(w, "Total after: %d\n", result.TotalAfter)
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
