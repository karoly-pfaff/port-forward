package commands

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
	"portier/cli/sources/version"
)

// bundleSchemaVersion is the schema version of the support-bundle manifest.
const bundleSchemaVersion = 1

const supportBundleHelp = `Usage: portier support-bundle --out <directory>

Collect deterministic doctor artifacts into a local directory for CI artifacts,
bug reports, and future tooling. Read-only: contacts the live runtime but never
mutates the runtime or its config.

The bundle directory contains:
  manifest.json        Bundle metadata (schema version, time, CLI version, artifacts, warnings).
  doctor.json          The live doctor report (same shape as 'portier doctor --json').
  doctor.txt           The human-readable doctor report.
  explanations.json    All known doctor/check code explanations.
  runtime.json         Runtime metadata (omitted with a manifest warning if unreachable).
  config-export.json   Read-only config export (omitted with a manifest warning if unavailable).

Safety: the bundle deliberately excludes OS environment variables, process
lists, logs, tokens, and arbitrary filesystem scans. It MAY include rule names,
hosts, and ports, because those are part of Portier configuration/diagnostics.

Options:
  --out <dir>   Required. Output directory. Created if missing; an existing
                directory must be empty (a non-empty directory is refused).
  --strict      Apply strict interpretation to the doctor report (warnings fail).
  --json        Print a machine-readable bundle summary to stdout.

Exit codes:
  0  Bundle written; doctor report has no error-severity checks (warnings exit 0
     unless --strict)
  1  Bundle written but the doctor report failed (errors, unreachable runtime, or
     a warning under --strict), OR a filesystem/write failure
  2  Missing --out, or a usage error

Examples:
  portier support-bundle --out ./portier-support-bundle
  portier support-bundle --out ./bundle --strict
  portier --json support-bundle --out ./bundle
`

// bundleManifest is the manifest.json describing a support bundle. It contains
// no secrets, environment data, or filesystem scans.
type bundleManifest struct {
	SchemaVersion int      `json:"schemaVersion"`
	GeneratedAt   string   `json:"generatedAt"`
	CLIVersion    string   `json:"cliVersion"`
	Source        string   `json:"source"`
	GeneratedBy   string   `json:"generatedBy"`
	RuntimeURL    string   `json:"runtimeUrl,omitempty"`
	Strict        bool     `json:"strict"`
	Result        string   `json:"result"`
	Artifacts     []string `json:"artifacts"`
	Warnings      []string `json:"warnings"`
}

// bundleArtifact is one file to write into the bundle directory: its filename
// and a function that writes it to the given path.
type bundleArtifact struct {
	name  string
	write func(path string) error
}

// bundleSummary is the --json stdout summary of a written support bundle.
type bundleSummary struct {
	Out       string   `json:"out"`
	Artifacts []string `json:"artifacts"`
	Strict    bool     `json:"strict"`
	Result    string   `json:"result"`
	Warnings  []string `json:"warnings"`
}

// RunSupportBundle collects deterministic doctor artifacts into the --out
// directory. It is read-only (never mutates the runtime or config). Exit codes:
// 0 = bundle written and doctor passed, 1 = filesystem/write failure or the
// doctor report failed (errors / unreachable / strict warning), 2 = usage error.
func RunSupportBundle(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("support-bundle", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagOut := fs.String("out", "", "output directory for the support bundle")
	flagStrict := fs.Bool("strict", false, "treat warnings as failures in the doctor report")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, supportBundleHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, supportBundleHelp)
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "Error: support-bundle takes no positional arguments (got %q)\n\n", fs.Arg(0))
		fmt.Fprint(stderr, supportBundleHelp)
		return 2
	}
	if *flagOut == "" {
		fmt.Fprintln(stderr, "Error: support-bundle requires --out <directory>")
		fmt.Fprint(stderr, supportBundleHelp)
		return 2
	}

	outDir := *flagOut
	if err := prepareBundleDir(outDir); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	strict := *flagStrict
	report := runLiveDoctorChecks(c)
	result := doctorResultLabel(report, strict)

	var warnings []string

	// Assemble the artifact list in deterministic order. doctor.json/.txt and
	// explanations.json are always present; runtime.json and config-export.json
	// are added only when the runtime provides them (otherwise a manifest
	// warning records the omission). manifest.json is written separately, last.
	var human bytes.Buffer
	printDoctorHuman("Portier Doctor", report, strict, &human)

	items := []bundleArtifact{
		{"doctor.json", func(p string) error {
			return writePrettyJSON(p, doctorReportJSON{DoctorReport: report, Strict: strict, Result: result})
		}},
		{"doctor.txt", func(p string) error { return os.WriteFile(p, human.Bytes(), 0o644) }},
		{"explanations.json", func(p string) error { return writePrettyJSON(p, sortedExplanations()) }},
	}
	if info, err := c.GetRuntime(); err != nil {
		warnings = append(warnings, fmt.Sprintf("runtime.json omitted: %v", err))
	} else {
		items = append(items, bundleArtifact{"runtime.json", func(p string) error { return writePrettyJSON(p, info) }})
	}
	if cfg, err := c.ExportConfig(); err != nil {
		warnings = append(warnings, fmt.Sprintf("config-export.json omitted: %v", err))
	} else {
		items = append(items, bundleArtifact{"config-export.json", func(p string) error { return writePrettyJSON(p, cfg) }})
	}

	artifacts := make([]string, 0, len(items))
	for _, it := range items {
		path := filepath.Join(outDir, it.name)
		if err := it.write(path); err != nil {
			fmt.Fprintf(stderr, "Error writing %s: %v\n", path, err)
			return 1
		}
		artifacts = append(artifacts, it.name)
	}

	// manifest.json — written last so its presence marks a complete bundle.
	manifest := bundleManifest{
		SchemaVersion: bundleSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		CLIVersion:    version.Version,
		Source:        "cli",
		GeneratedBy:   "portier support-bundle",
		RuntimeURL:    c.BaseURL(),
		Strict:        strict,
		Result:        result,
		Artifacts:     artifacts,
		Warnings:      warnings,
	}
	if err := writePrettyJSON(filepath.Join(outDir, "manifest.json"), manifest); err != nil {
		fmt.Fprintf(stderr, "Error writing %s: %v\n", filepath.Join(outDir, "manifest.json"), err)
		return 1
	}

	allFiles := append(append([]string{}, artifacts...), "manifest.json")
	if jsonOutput {
		if err := output.PrintJSON(stdout, bundleSummary{
			Out:       outDir,
			Artifacts: allFiles,
			Strict:    strict,
			Result:    result,
			Warnings:  warnings,
		}); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintf(stdout, "Support bundle written to %s\n", outDir)
		fmt.Fprintf(stdout, "  %d %s\n", len(allFiles), pluralWord(len(allFiles), "artifact", "artifacts"))
		for _, w := range warnings {
			fmt.Fprintf(stdout, "  warning: %s\n", w)
		}
		fmt.Fprintf(stdout, "  Result: %s\n", result)
	}

	return doctorExitCode(report, strict)
}

// prepareBundleDir ensures outDir is a usable, empty bundle directory: it
// creates the directory (and parents) if missing, accepts an existing empty
// directory, and refuses a non-empty directory or a non-directory path so an
// existing bundle is never silently overwritten.
func prepareBundleDir(outDir string) error {
	info, err := os.Stat(outDir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(outDir, 0o755)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("output path %s exists and is not a directory", outDir)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("output directory %s is not empty (refusing to overwrite)", outDir)
	}
	return nil
}
