package commands

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
)

const configHelp = `Usage: portier config <subcommand> [options]

Manage Portier forwarding rule configuration.

Subcommands:
  validate <file>                     Validate a local config file without importing it
  export --out <file>                 Export current rules to a file (omit --out to print to stdout)
  import <file> --mode merge|replace  Import rules from a file (--yes required for replace mode)

Run 'portier config <subcommand> --help' for subcommand options.
`

const exportHelp = `Usage: portier config export [--out <file>]

Export the current forwarding rules from the running Portier service to a JSON file.

Options:
  --out string   Output file path. If omitted, prints JSON to stdout.

Examples:
  portier config export --out rules-backup.json
  portier --json config export --out rules-backup.json
  portier --json config export            # print raw config JSON to stdout
`

const importHelp = `Usage: portier config import <file> --mode merge|replace [--yes]

Import forwarding rules from a local JSON config file.

The file is validated locally before any API call is made. Invalid files are rejected
without contacting the running service.

Options:
  --mode string   Required. Import mode: merge or replace.
  --yes           Required when --mode replace. Confirms destructive overwrite.

Modes:
  merge     Add rules from the file. Existing rules are preserved.
            ID conflicts generate new IDs. Listen-binding conflicts abort the import.
  replace   Remove all existing rules and replace with the imported set.
            Requires --yes to confirm.

Examples:
  portier config import rules-backup.json --mode merge
  portier config import rules-backup.json --mode replace --yes
`

const validateHelp = `Usage: portier config validate <file>

Validate a local Portier config file without importing it or contacting the service.

Accepted file shapes:
  Raw JSON array:  [{ "name": "...", ... }, ...]
  Wrapper object:  { "rules": [{ "name": "...", ... }, ...] }
  Exported config: { "version": "1", "exportedAt": "...", "rules": [...] }

Exit codes:
  0  Config is valid
  1  Config is invalid, unreadable, or the file does not exist

Examples:
  portier config validate rules-backup.json
  portier --json config validate rules-backup.json
`

// RunConfig dispatches portier config subcommands.
func RunConfig(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, configHelp)
		return 2
	}
	switch args[0] {
	case "export":
		return RunConfigExport(c, jsonOutput, args[1:], stdout, stderr)
	case "import":
		return RunConfigImport(c, jsonOutput, args[1:], stdout, stderr)
	case "validate":
		return RunConfigValidate(jsonOutput, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, configHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown config subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, configHelp)
		return 2
	}
}

// RunConfigExport exports the current service config to a file or stdout.
func RunConfigExport(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagOut := fs.String("out", "", "output file path")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, exportHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, exportHelp)
		return 2
	}

	cfg, err := c.ExportConfig()
	if err != nil {
		return exitWithError(err, stderr)
	}

	if *flagOut == "" {
		// No file specified — print JSON to stdout regardless of --json flag.
		// (The exported config IS the JSON; there is no alternative human representation.)
		if jsonOutput {
			if err := output.PrintJSON(stdout, cfg); err != nil {
				fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
				return 1
			}
			return 0
		}
		fmt.Fprintln(stderr, "Error: --out is required in human mode")
		fmt.Fprintln(stderr, "Use --out <file> to write to a file, or add --json to print to stdout.")
		return 2
	}

	if err := writePrettyJSON(*flagOut, cfg); err != nil {
		fmt.Fprintf(stderr, "Error writing %s: %v\n", *flagOut, err)
		return 1
	}

	ruleCount := len(cfg.Rules)
	if jsonOutput {
		if err := output.PrintJSON(stdout, map[string]any{
			"ok":        true,
			"path":      *flagOut,
			"ruleCount": ruleCount,
		}); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "Exported %d %s to %s\n",
		ruleCount, pluralRule(ruleCount), *flagOut)
	return 0
}

// RunConfigImport imports rules from a local config file.
func RunConfigImport(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagMode := fs.String("mode", "", "import mode: merge or replace")
	flagYes := fs.Bool("yes", false, "confirm replace mode")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, importHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, importHelp)
		return 2
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "Error: import requires a config file path")
		fmt.Fprint(stderr, importHelp)
		return 2
	}

	if *flagMode == "" {
		fmt.Fprintln(stderr, "Error: --mode is required (merge or replace)")
		fmt.Fprint(stderr, importHelp)
		return 2
	}
	if *flagMode != "merge" && *flagMode != "replace" {
		fmt.Fprintf(stderr, "Error: invalid --mode %q: must be \"merge\" or \"replace\"\n\n", *flagMode)
		fmt.Fprint(stderr, importHelp)
		return 2
	}
	if *flagMode == "replace" && !*flagYes {
		fmt.Fprintln(stderr, "Error: replace mode overwrites all existing rules.")
		fmt.Fprintln(stderr, "Re-run with --yes to confirm.")
		return 2
	}

	filePath := fs.Arg(0)
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading %s: %v\n", filePath, err)
		return 1
	}

	rules, parseErr := parseLocalConfig(data)
	if parseErr != nil {
		fmt.Fprintf(stderr, "Error: %v\n", parseErr)
		return 1
	}
	vr := validateLocalConfig(rules)
	if !vr.Valid {
		fmt.Fprintln(stderr, "Config is invalid — import aborted.")
		for _, e := range vr.Errors {
			fmt.Fprintf(stderr, "  %s\n", e)
		}
		return 1
	}

	configRules := make([]client.ConfigRule, len(rules))
	for i, r := range rules {
		configRules[i] = client.ConfigRule{
			ID:         r.ID,
			Name:       r.Name,
			Protocol:   r.Protocol,
			ListenHost: r.ListenHost,
			ListenPort: r.ListenPort,
			TargetHost: r.TargetHost,
			TargetPort: r.TargetPort,
			Enabled:    r.Enabled,
			UDPMode:    r.UDPMode,
		}
	}
	importReq := client.ConfigImportRequest{
		Mode: *flagMode,
		Config: client.ConfigExportResponse{
			Version: "1",
			Rules:   configRules,
		},
	}

	resp, err := c.ImportConfig(importReq)
	if err != nil {
		return exitWithError(err, stderr)
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, map[string]any{
			"ok":   true,
			"mode": *flagMode,
		}); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "Imported config using %s mode.\n", *flagMode)
	if resp.Result.Imported > 0 {
		fmt.Fprintf(stdout, "  %d %s imported.\n",
			resp.Result.Imported, pluralRule(resp.Result.Imported))
	}
	return 0
}

// RunConfigValidate validates a local config file without importing it.
func RunConfigValidate(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, validateHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, validateHelp)
		return 2
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "Error: validate requires a config file path")
		fmt.Fprint(stderr, validateHelp)
		return 2
	}

	filePath := fs.Arg(0)
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading %s: %v\n", filePath, err)
		return 1
	}

	rules, parseErr := parseLocalConfig(data)
	if parseErr != nil {
		if jsonOutput {
			if err := output.PrintJSON(stdout, configValidationResult{
				Valid:  false,
				Errors: []string{parseErr.Error()},
			}); err != nil {
				fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			}
			return 1
		}
		fmt.Fprintln(stdout, "Config is invalid.")
		fmt.Fprintf(stdout, "  %s\n", parseErr)
		return 1
	}

	vr := validateLocalConfig(rules)

	if jsonOutput {
		if err := output.PrintJSON(stdout, vr); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		if !vr.Valid {
			return 1
		}
		return 0
	}

	if vr.Valid {
		fmt.Fprintln(stdout, "Config is valid.")
		fmt.Fprintf(stdout, "  %d %s: %d TCP, %d UDP\n",
			vr.RuleCount, pluralRule(vr.RuleCount), vr.TCPCount, vr.UDPCount)
		return 0
	}

	fmt.Fprintln(stdout, "Config is invalid.")
	for _, e := range vr.Errors {
		fmt.Fprintf(stdout, "  %s\n", e)
	}
	return 1
}

// --- local config parsing and validation ---

// configValidationResult holds the outcome of local config validation.
type configValidationResult struct {
	Valid     bool     `json:"valid"`
	RuleCount int      `json:"ruleCount"`
	TCPCount  int      `json:"tcpCount"`
	UDPCount  int      `json:"udpCount"`
	Errors    []string `json:"errors"`
}

// rawConfigRule is a forwarding rule parsed from a local config file.
type rawConfigRule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	ListenHost string `json:"listenHost"`
	ListenPort int    `json:"listenPort"`
	TargetHost string `json:"targetHost"`
	TargetPort int    `json:"targetPort"`
	Enabled    bool   `json:"enabled"`
	UDPMode    string `json:"udpMode"`
}

// parseLocalConfig extracts forwarding rules from a config file in any supported shape:
//   - raw JSON array: [...]
//   - wrapper object: { "rules": [...] }
//   - exported config: { "version": "1", "exportedAt": "...", "rules": [...] }
func parseLocalConfig(data []byte) ([]rawConfigRule, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("config file is empty")
	}

	if trimmed[0] == '[' {
		var rules []rawConfigRule
		if err := json.Unmarshal(trimmed, &rules); err != nil {
			return nil, fmt.Errorf("not a valid JSON array: %w", err)
		}
		if rules == nil {
			rules = []rawConfigRule{}
		}
		return rules, nil
	}

	if trimmed[0] == '{' {
		var obj struct {
			Rules json.RawMessage `json:"rules"`
		}
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return nil, fmt.Errorf("not a valid JSON object: %w", err)
		}
		if obj.Rules == nil {
			return nil, fmt.Errorf("config object is missing the required \"rules\" field")
		}
		var rules []rawConfigRule
		if err := json.Unmarshal(obj.Rules, &rules); err != nil {
			return nil, fmt.Errorf("\"rules\" field is not a valid JSON array: %w", err)
		}
		if rules == nil {
			rules = []rawConfigRule{}
		}
		return rules, nil
	}

	return nil, fmt.Errorf("not a valid Portier config: expected a JSON array or an object with a \"rules\" field")
}

// validateLocalConfig checks rules for field validity and duplicate listen bindings.
func validateLocalConfig(rules []rawConfigRule) configValidationResult {
	errs := make([]string, 0)
	tcpCount := 0
	udpCount := 0

	validProtocols := map[string]bool{"tcp": true, "udp": true}
	validUDPModes := map[string]bool{
		"one-way":                    true,
		"bidirectional-last-client":  true,
		"bidirectional-multi-client": true,
	}

	type bindingKey struct {
		proto string
		host  string
		port  int
	}
	seen := map[bindingKey]int{} // key → 1-based rule index

	for i, r := range rules {
		ruleNum := i + 1
		prefix := fmt.Sprintf("rule %d", ruleNum)
		if r.Name != "" {
			prefix = fmt.Sprintf("rule %d %q", ruleNum, r.Name)
		}

		if r.Name == "" {
			errs = append(errs, fmt.Sprintf("%s: name is required", prefix))
		}
		if !validProtocols[r.Protocol] {
			errs = append(errs, fmt.Sprintf("%s: invalid protocol %q (must be \"tcp\" or \"udp\")", prefix, r.Protocol))
		}
		if r.ListenHost == "" {
			errs = append(errs, fmt.Sprintf("%s: listenHost is required", prefix))
		}
		if r.ListenPort < 1 || r.ListenPort > 65535 {
			errs = append(errs, fmt.Sprintf("%s: listenPort %d is out of range (must be 1–65535)", prefix, r.ListenPort))
		}
		if r.TargetHost == "" {
			errs = append(errs, fmt.Sprintf("%s: targetHost is required", prefix))
		}
		if r.TargetPort < 1 || r.TargetPort > 65535 {
			errs = append(errs, fmt.Sprintf("%s: targetPort %d is out of range (must be 1–65535)", prefix, r.TargetPort))
		}
		if r.Protocol == "udp" && r.UDPMode != "" && !validUDPModes[r.UDPMode] {
			errs = append(errs, fmt.Sprintf("%s: invalid udpMode %q", prefix, r.UDPMode))
		}

		if r.Protocol == "tcp" {
			tcpCount++
		} else if r.Protocol == "udp" {
			udpCount++
		}

		// Duplicate binding check (only when basic fields are valid)
		if validProtocols[r.Protocol] && r.ListenHost != "" && r.ListenPort >= 1 && r.ListenPort <= 65535 {
			key := bindingKey{r.Protocol, r.ListenHost, r.ListenPort}
			if prev, ok := seen[key]; ok {
				errs = append(errs, fmt.Sprintf("duplicate listen binding: %s %s:%d (rules %d and %d)",
					r.Protocol, r.ListenHost, r.ListenPort, prev, ruleNum))
			} else {
				seen[key] = ruleNum
			}
		}
	}

	return configValidationResult{
		Valid:     len(errs) == 0,
		RuleCount: len(rules),
		TCPCount:  tcpCount,
		UDPCount:  udpCount,
		Errors:    errs,
	}
}

// writePrettyJSON marshals v as indented JSON and writes it to path.
// The file is only written after a successful marshal, so no partial writes occur.
func writePrettyJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func pluralRule(n int) string {
	if n == 1 {
		return "rule"
	}
	return "rules"
}
