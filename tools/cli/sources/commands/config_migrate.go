package commands

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"portier/cli/sources/config"
	"portier/cli/sources/output"
)

// migrateReportSchemaVersion versions the migrate report artifact (CLI-local,
// not an API DTO) so future tooling can detect its shape.
const migrateReportSchemaVersion = 1

const migrateHelp = `Usage: portier config migrate <file> [--write] [--out <file>]

Inspect and normalize a local Portier config file to the canonical persisted
shape (a bare JSON array of rules). Offline: never contacts the running service.

Dry-run by default — it reports what it WOULD do and writes nothing. Pass --write
to apply the normalization; --write backs the original up first and writes
atomically. A malformed, schema-invalid, or unsupported-version config is never
written or overwritten.

What it does:
  - Detects the file format: bare-array, wrapper-object ({ "rules": [...] }),
    or exported envelope ({ "version": "1", ... }).
  - Refuses an unsupported/future envelope version (no overwrite).
  - Validates rule fields and duplicate listen bindings.
  - Normalizes a valid config to the canonical bare-array rules.json shape.

Options:
  --write        Apply the normalization (default is a dry run that writes nothing).
  --out <file>   Write the migrated config to <file> instead of in place.
                 Place flags before the config file (e.g. config migrate --write file).

Accepted file shapes:
  Raw JSON array:  [{ "name": "...", ... }, ...]
  Wrapper object:  { "rules": [{ "name": "...", ... }, ...] }
  Exported config: { "version": "1", "exportedAt": "...", "rules": [...] }

Exit codes:
  0  Config is valid (dry-run reported, or migration written)
  1  Config is malformed, schema-invalid, an unsupported version, unreadable,
     or a backup/write failure occurred
  2  Missing argument or usage error

Examples:
  portier config migrate rules.json
  portier config migrate rules.json --write
  portier config migrate exported.json --out rules.json --write
  portier --json config migrate rules.json
`

// migrateReport is the deterministic migrate result (human summary fields + the
// --json body). CLI-local artifact, versioned by schemaVersion.
type migrateReport struct {
	SchemaVersion int      `json:"schemaVersion"`
	File          string   `json:"file"`
	Format        string   `json:"format"`
	SourceVersion string   `json:"sourceVersion,omitempty"`
	TargetFormat  string   `json:"targetFormat"`
	RuleCount     int      `json:"ruleCount"`
	TCPCount      int      `json:"tcpCount"`
	UDPCount      int      `json:"udpCount"`
	Valid         bool     `json:"valid"`
	Changed       bool     `json:"changed"`
	DryRun        bool     `json:"dryRun"`
	Written       bool     `json:"written"`
	BackupPath    string   `json:"backupPath,omitempty"`
	OutputPath    string   `json:"outputPath,omitempty"`
	Errors        []string `json:"errors"`
}

// RunConfigMigrate inspects and (with --write) normalizes a local config file to
// the canonical bare-array shape. Offline and backup-first; never writes a
// malformed/schema-invalid/unsupported config. Exit codes: 0 valid (dry-run or
// written), 1 invalid/unsupported/unreadable/write failure, 2 usage error.
func RunConfigMigrate(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config migrate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagWrite := fs.Bool("write", false, "apply the normalization (default is a dry run)")
	flagOut := fs.String("out", "", "write the migrated config to this file instead of in place")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, migrateHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, migrateHelp)
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "Error: migrate requires a config file path")
		fmt.Fprint(stderr, migrateHelp)
		return 2
	}

	filePath := fs.Arg(0)
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading %s: %v\n", filePath, err)
		return 1
	}

	target := filePath
	if *flagOut != "" {
		target = *flagOut
	}

	// Structural classification first (malformed / unsupported version never write).
	cls, rules, cerr := config.Classify(data)
	if cerr != nil {
		report := migrateReport{
			SchemaVersion: migrateReportSchemaVersion,
			File:          filePath,
			TargetFormat:  string(config.FormatArray),
			DryRun:        !*flagWrite,
			Errors:        []string{cerr.Error()},
		}
		if errors.Is(cerr, config.ErrUnsupportedVersion) {
			report.Format = "unsupported"
		} else {
			report.Format = "unrecognized"
		}
		emitMigrateError(jsonOutput, report, "Config migration failed.", stdout, stderr)
		return 1
	}

	vr := config.Validate(rules)
	report := migrateReport{
		SchemaVersion: migrateReportSchemaVersion,
		File:          filePath,
		Format:        string(cls.Format),
		SourceVersion: cls.SourceVersion,
		TargetFormat:  string(config.FormatArray),
		RuleCount:     vr.RuleCount,
		TCPCount:      vr.TCPCount,
		UDPCount:      vr.UDPCount,
		Valid:         vr.Valid,
		DryRun:        !*flagWrite,
		Errors:        []string{},
	}

	if !vr.Valid {
		report.Errors = vr.Errors
		emitMigrateError(jsonOutput, report, "Config is invalid — migration aborted.", stdout, stderr)
		return 1
	}

	canonical, err := config.CanonicalRules(rules)
	if err != nil {
		fmt.Fprintf(stderr, "Error serializing canonical config: %v\n", err)
		return 1
	}
	report.Changed = !bytes.Equal(canonical, data)

	if !*flagWrite {
		// Dry run: never writes.
		report.OutputPath = target
		emitMigrate(jsonOutput, report, stdout, stderr)
		return 0
	}

	// Write mode: back up an existing target FIRST, then write atomically.
	if _, statErr := os.Stat(target); statErr == nil {
		backupPath, backupErr := backUpFile(target, time.Now())
		if backupErr != nil {
			fmt.Fprintf(stderr, "Error: could not back up %s before migrating: %v\n", target, backupErr)
			fmt.Fprintln(stderr, "Migration aborted — nothing was written.")
			return 1
		}
		report.BackupPath = backupPath
	}

	if err := output.WriteFileAtomic(target, canonical); err != nil {
		fmt.Fprintf(stderr, "Error writing %s: %v\n", target, err)
		return 1
	}
	report.Written = true
	report.OutputPath = target

	emitMigrate(jsonOutput, report, stdout, stderr)
	return 0
}

// backUpFile copies path to a unique, timestamped sibling (path.bak-<UTC>) so an
// existing config is preserved before migrate overwrites it. Never overwrites an
// existing backup (appends -N on collision). Returns the backup path.
func backUpFile(path string, now time.Time) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	base := fmt.Sprintf("%s.bak-%s", path, now.UTC().Format("2006-01-02T150405Z"))
	candidate := base
	for attempt := 1; ; attempt++ {
		if _, statErr := os.Stat(candidate); errors.Is(statErr, os.ErrNotExist) {
			break
		}
		candidate = fmt.Sprintf("%s-%d", base, attempt)
	}
	if err := os.WriteFile(candidate, data, 0o644); err != nil {
		return "", err
	}
	return candidate, nil
}

func emitMigrate(jsonOutput bool, report migrateReport, stdout, stderr io.Writer) {
	if jsonOutput {
		if err := output.PrintJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
		}
		return
	}
	printMigrateHuman(report, stdout)
}

func emitMigrateError(jsonOutput bool, report migrateReport, title string, stdout, stderr io.Writer) {
	if jsonOutput {
		if err := output.PrintJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
		}
		return
	}
	fmt.Fprintln(stdout, title)
	for _, e := range report.Errors {
		fmt.Fprintf(stdout, "  %s\n", e)
	}
}

func printMigrateHuman(report migrateReport, stdout io.Writer) {
	if report.DryRun {
		fmt.Fprintln(stdout, "Config migration (dry run)")
	} else {
		fmt.Fprintln(stdout, "Config migrated")
	}
	fmt.Fprintf(stdout, "  File: %s\n", report.File)
	if report.SourceVersion != "" {
		fmt.Fprintf(stdout, "  Format: %s (version %s)\n", report.Format, report.SourceVersion)
	} else {
		fmt.Fprintf(stdout, "  Format: %s\n", report.Format)
	}
	fmt.Fprintf(stdout, "  Rules: %d (%d TCP, %d UDP)\n", report.RuleCount, report.TCPCount, report.UDPCount)
	fmt.Fprintf(stdout, "  Target: canonical %s rules.json\n", report.TargetFormat)
	fmt.Fprintf(stdout, "  Normalization needed: %s\n", output.FormatBool(report.Changed))
	if report.Written {
		if report.BackupPath != "" {
			fmt.Fprintf(stdout, "  Backup: %s\n", report.BackupPath)
		}
		fmt.Fprintf(stdout, "  Written: %s\n", report.OutputPath)
	} else {
		fmt.Fprintln(stdout, "  No changes written (dry run). Re-run with --write to apply.")
	}
}
