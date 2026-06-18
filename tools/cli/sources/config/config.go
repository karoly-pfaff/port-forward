// Package config holds the Portier CLI's local config-file domain model and its
// pure, offline logic: parsing a config file into rules, validating rule fields,
// detecting duplicate listen bindings, building a compact summary, and the
// rule-detail shapes shared by the doctor and policy tooling. It contains no
// command/dispatch code and never contacts the runtime — the CLI command
// handlers in package commands compose this with the API client.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// GroupMaxLength mirrors the server-side rule group label limit (characters).
// The CLI does a light local pre-check; the server is authoritative.
const GroupMaxLength = 64

// SupportedConfigVersion is the only config/export envelope version Portier
// understands today. A persisted/exported object carrying a different version is
// treated as unsupported (never auto-migrated or overwritten) — the offline
// `config migrate` classifier surfaces it as ErrUnsupportedVersion. The persisted
// `rules.json` remains an unversioned bare array; the version field only appears
// on the export/import envelope.
const SupportedConfigVersion = "1"

// ConfigFormat is the detected on-disk shape of a config file.
type ConfigFormat string

const (
	// FormatArray is the canonical persisted shape: a bare JSON array of rules.
	FormatArray ConfigFormat = "bare-array"
	// FormatWrapper is a { "rules": [...] } object with no version field.
	FormatWrapper ConfigFormat = "wrapper-object"
	// FormatExported is an exported envelope: { "version": "1", "exportedAt": ..., "rules": [...] }.
	FormatExported ConfigFormat = "exported"
)

// Classification describes the structural shape of a config file (independent of
// whether the rules inside are schema-valid — that is Validate's job).
type Classification struct {
	Format ConfigFormat `json:"format"`
	// SourceVersion is the envelope version: "" for a bare array or wrapper
	// object, "1" for an exported envelope.
	SourceVersion string `json:"sourceVersion"`
}

// Structural classification errors. Callers (e.g. `config migrate`) use
// errors.Is to branch and never write when one of these is returned.
var (
	// ErrEmptyConfig: the file held no JSON content.
	ErrEmptyConfig = errors.New("config file is empty")
	// ErrMalformedConfig: not valid JSON, or a JSON value that is neither a rules
	// array nor a { "rules": [...] } object.
	ErrMalformedConfig = errors.New("config is malformed")
	// ErrUnsupportedVersion: a versioned envelope whose version is not supported.
	// The detail (the actual version) is wrapped into the returned error.
	ErrUnsupportedVersion = errors.New("unsupported config version")
)

// Classify determines the structural shape (and envelope version) of a config
// file and returns the parsed rules. It is pure and offline. Structural problems
// return a wrapped ErrEmptyConfig / ErrMalformedConfig / ErrUnsupportedVersion;
// a future/unknown envelope version is NEVER treated as loadable (so migrate
// cannot overwrite it). Rule-field validity is intentionally NOT checked here —
// callers run Validate on the returned rules.
func Classify(data []byte) (Classification, []Rule, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Classification{}, nil, ErrEmptyConfig
	}

	if trimmed[0] == '[' {
		var rules []Rule
		if err := json.Unmarshal(trimmed, &rules); err != nil {
			return Classification{}, nil, fmt.Errorf("%w: not a valid JSON array: %v", ErrMalformedConfig, err)
		}
		if rules == nil {
			rules = []Rule{}
		}
		return Classification{Format: FormatArray, SourceVersion: ""}, rules, nil
	}

	if trimmed[0] == '{' {
		var obj struct {
			Version *string         `json:"version"`
			Rules   json.RawMessage `json:"rules"`
		}
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return Classification{}, nil, fmt.Errorf("%w: not a valid JSON object: %v", ErrMalformedConfig, err)
		}
		if obj.Rules == nil {
			return Classification{}, nil, fmt.Errorf("%w: config object is missing the required \"rules\" field", ErrMalformedConfig)
		}
		if obj.Version != nil && *obj.Version != SupportedConfigVersion {
			return Classification{}, nil, fmt.Errorf("%w: %q (this Portier supports version %q)", ErrUnsupportedVersion, *obj.Version, SupportedConfigVersion)
		}
		var rules []Rule
		if err := json.Unmarshal(obj.Rules, &rules); err != nil {
			return Classification{}, nil, fmt.Errorf("%w: \"rules\" field is not a valid JSON array: %v", ErrMalformedConfig, err)
		}
		if rules == nil {
			rules = []Rule{}
		}
		format := FormatWrapper
		version := ""
		if obj.Version != nil {
			format = FormatExported
			version = *obj.Version
		}
		return Classification{Format: format, SourceVersion: version}, rules, nil
	}

	return Classification{}, nil, fmt.Errorf("%w: expected a JSON array or an object with a \"rules\" field", ErrMalformedConfig)
}

// canonicalRule is the marshaling shape for the canonical persisted config: a
// bare array of rules with optional fields omitted when empty, matching a clean
// hand- or runtime-written rules.json. `enabled` is always emitted (a bool whose
// false value is meaningful).
type canonicalRule struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	ListenHost string `json:"listenHost"`
	ListenPort int    `json:"listenPort"`
	TargetHost string `json:"targetHost"`
	TargetPort int    `json:"targetPort"`
	Enabled    bool   `json:"enabled"`
	UDPMode    string `json:"udpMode,omitempty"`
	Group      string `json:"group,omitempty"`
}

// CanonicalRules serializes rules to the canonical persisted form: a 2-space
// indented bare JSON array with a trailing newline (matching the runtime store's
// output style), optional fields omitted when empty, and group labels trimmed.
// This is the normalization target of `config migrate`. It does not assign ids
// or mutate rule semantics — only the on-disk shape is canonicalized.
func CanonicalRules(rules []Rule) ([]byte, error) {
	out := make([]canonicalRule, len(rules))
	for i, r := range rules {
		out[i] = canonicalRule{
			ID:         r.ID,
			Name:       r.Name,
			Protocol:   r.Protocol,
			ListenHost: r.ListenHost,
			ListenPort: r.ListenPort,
			TargetHost: r.TargetHost,
			TargetPort: r.TargetPort,
			Enabled:    r.Enabled,
			UDPMode:    r.UDPMode,
			Group:      strings.TrimSpace(r.Group),
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// DuplicateBindingErrPrefix is the leading text of the duplicate listen-binding
// validation error. It is shared between Validate (the producer) and the config
// doctor (which partitions errors by it) so the two cannot drift.
const DuplicateBindingErrPrefix = "duplicate listen binding:"

// Rule is a forwarding rule parsed from a local config file.
type Rule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	ListenHost string `json:"listenHost"`
	ListenPort int    `json:"listenPort"`
	TargetHost string `json:"targetHost"`
	TargetPort int    `json:"targetPort"`
	Enabled    bool   `json:"enabled"`
	UDPMode    string `json:"udpMode"`
	Group      string `json:"group"`
}

// ValidationResult holds the outcome of local config validation.
type ValidationResult struct {
	Valid     bool     `json:"valid"`
	RuleCount int      `json:"ruleCount"`
	TCPCount  int      `json:"tcpCount"`
	UDPCount  int      `json:"udpCount"`
	Errors    []string `json:"errors"`
}

// ParseLocal extracts forwarding rules from a config file in any supported shape:
//   - raw JSON array: [...]
//   - wrapper object: { "rules": [...] }
//   - exported config: { "version": "1", "exportedAt": "...", "rules": [...] }
func ParseLocal(data []byte) ([]Rule, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("config file is empty")
	}

	if trimmed[0] == '[' {
		var rules []Rule
		if err := json.Unmarshal(trimmed, &rules); err != nil {
			return nil, fmt.Errorf("not a valid JSON array: %w", err)
		}
		if rules == nil {
			rules = []Rule{}
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
		var rules []Rule
		if err := json.Unmarshal(obj.Rules, &rules); err != nil {
			return nil, fmt.Errorf("\"rules\" field is not a valid JSON array: %w", err)
		}
		if rules == nil {
			rules = []Rule{}
		}
		return rules, nil
	}

	return nil, fmt.Errorf("not a valid Portier config: expected a JSON array or an object with a \"rules\" field")
}

// Validate checks rules for field validity and duplicate listen bindings.
func Validate(rules []Rule) ValidationResult {
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
		if g := strings.TrimSpace(r.Group); g != "" {
			if utf8.RuneCountInString(g) > GroupMaxLength {
				errs = append(errs, fmt.Sprintf("%s: group must be %d characters or fewer", prefix, GroupMaxLength))
			} else if HasControlChar(g) {
				errs = append(errs, fmt.Sprintf("%s: group must not contain control characters", prefix))
			}
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
				errs = append(errs, fmt.Sprintf("%s %s %s:%d (rules %d and %d)",
					DuplicateBindingErrPrefix, r.Protocol, r.ListenHost, r.ListenPort, prev, ruleNum))
			} else {
				seen[key] = ruleNum
			}
		}
	}

	return ValidationResult{
		Valid:     len(errs) == 0,
		RuleCount: len(rules),
		TCPCount:  tcpCount,
		UDPCount:  udpCount,
		Errors:    errs,
	}
}

// HasControlChar reports whether s contains a C0 control character (U+0000-
// U+001F) or DEL (U+007F). Used for the local group-label pre-check.
func HasControlChar(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// PartitionValidationErrors splits validation error messages into duplicate
// listen-binding errors and all other (field-level) errors, preserving order.
func PartitionValidationErrors(errs []string) (dup, field []string) {
	for _, e := range errs {
		if strings.HasPrefix(e, DuplicateBindingErrPrefix) {
			dup = append(dup, e)
		} else {
			field = append(field, e)
		}
	}
	return dup, field
}

// RuleLabel returns a stable human label for a rule in doctor/policy output:
// the quoted name when present, otherwise a positional-free generic label.
func RuleLabel(r Rule) string {
	return RuleLabelName(r.Name)
}

// RuleLabelName is RuleLabel by name (a rule has no positional index in details).
func RuleLabelName(name string) string {
	if name != "" {
		return fmt.Sprintf("Rule %q", name)
	}
	return "An unnamed rule"
}

// RuleDetail is the structured, JSON-serializable description of a rule used in
// doctor/policy finding details. Derived ONLY from the offline config — it
// carries no runtime, host-environment, process, log, or secret data. id/group
// are omitted when absent.
type RuleDetail struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	ListenHost string `json:"listenHost"`
	ListenPort int    `json:"listenPort"`
	Enabled    bool   `json:"enabled"`
	Group      string `json:"group,omitempty"`
}

// ToRuleDetail projects a Rule into its JSON-serializable RuleDetail.
func ToRuleDetail(r Rule) RuleDetail {
	return RuleDetail{
		ID:         r.ID,
		Name:       r.Name,
		Protocol:   r.Protocol,
		ListenHost: r.ListenHost,
		ListenPort: r.ListenPort,
		Enabled:    r.Enabled,
		Group:      strings.TrimSpace(r.Group),
	}
}

// BindingConflict describes one duplicated listen binding and the rules on it.
// The binding identity (protocol/host/port) is shared by all its rules.
type BindingConflict struct {
	Protocol   string        `json:"protocol"`
	ListenHost string        `json:"listenHost"`
	ListenPort int           `json:"listenPort"`
	Rules      []BindingRule `json:"rules"`
}

// BindingRule is the per-rule entry inside a BindingConflict (the binding
// already carries protocol/host/port, so only rule identity is repeated).
type BindingRule struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Group   string `json:"group,omitempty"`
}

// FindDuplicateBindings groups rules by listen binding (protocol + listenHost +
// listenPort, considering only rules whose basic fields are valid — mirroring
// Validate's duplicate detection) and returns one BindingConflict per binding
// shared by two or more rules. Rules within a binding stay in file order; the
// bindings list is sorted by protocol, host, then port. Fully deterministic and
// derived only from the offline config.
func FindDuplicateBindings(rules []Rule) []BindingConflict {
	type bindKey struct {
		proto string
		host  string
		port  int
	}
	validProtocols := map[string]bool{"tcp": true, "udp": true}
	groups := map[bindKey][]Rule{}
	order := []bindKey{}
	for _, r := range rules {
		if !validProtocols[r.Protocol] || r.ListenHost == "" || r.ListenPort < 1 || r.ListenPort > 65535 {
			continue
		}
		k := bindKey{r.Protocol, r.ListenHost, r.ListenPort}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], r)
	}

	conflicts := []BindingConflict{}
	for _, k := range order {
		rs := groups[k]
		if len(rs) < 2 {
			continue
		}
		bc := BindingConflict{Protocol: k.proto, ListenHost: k.host, ListenPort: k.port}
		for _, r := range rs {
			bc.Rules = append(bc.Rules, BindingRule{
				ID:      r.ID,
				Name:    r.Name,
				Enabled: r.Enabled,
				Group:   strings.TrimSpace(r.Group),
			})
		}
		conflicts = append(conflicts, bc)
	}
	sort.Slice(conflicts, func(i, j int) bool {
		a, b := conflicts[i], conflicts[j]
		if a.Protocol != b.Protocol {
			return a.Protocol < b.Protocol
		}
		if a.ListenHost != b.ListenHost {
			return a.ListenHost < b.ListenHost
		}
		return a.ListenPort < b.ListenPort
	})
	return conflicts
}

// Summary is a compact, deterministic, machine-readable description of a parsed
// config. It is derived ONLY from the offline config contents (no environment,
// runtime, or filesystem data) and intentionally does not repeat full rule data
// (doctor finding details already identify affected rules).
type Summary struct {
	RuleCount          int            `json:"ruleCount"`
	EnabledRuleCount   int            `json:"enabledRuleCount"`
	DisabledRuleCount  int            `json:"disabledRuleCount"`
	Protocols          ProtocolCounts `json:"protocols"`
	GroupCount         int            `json:"groupCount"`
	Groups             []GroupSummary `json:"groups"`
	UngroupedRuleCount int            `json:"ungroupedRuleCount"`
}

// ProtocolCounts counts rules per protocol in a stable field order.
type ProtocolCounts struct {
	TCP int `json:"tcp"`
	UDP int `json:"udp"`
}

// GroupSummary is the per-group rollup inside a Summary.
type GroupSummary struct {
	Name              string `json:"name"`
	RuleCount         int    `json:"ruleCount"`
	EnabledRuleCount  int    `json:"enabledRuleCount"`
	DisabledRuleCount int    `json:"disabledRuleCount"`
}

// BuildSummary computes the deterministic config summary from parsed rules.
// Group names are trimmed; empty/whitespace groups count as ungrouped; the
// groups list is sorted by name. Unknown protocols are simply not counted under
// tcp/udp (so ruleCount may exceed tcp+udp for an invalid config).
func BuildSummary(rules []Rule) Summary {
	s := Summary{RuleCount: len(rules)}
	groups := map[string]*GroupSummary{}
	for _, r := range rules {
		if r.Enabled {
			s.EnabledRuleCount++
		} else {
			s.DisabledRuleCount++
		}
		switch r.Protocol {
		case "tcp":
			s.Protocols.TCP++
		case "udp":
			s.Protocols.UDP++
		}

		g := strings.TrimSpace(r.Group)
		if g == "" {
			s.UngroupedRuleCount++
			continue
		}
		gs, ok := groups[g]
		if !ok {
			gs = &GroupSummary{Name: g}
			groups[g] = gs
		}
		gs.RuleCount++
		if r.Enabled {
			gs.EnabledRuleCount++
		} else {
			gs.DisabledRuleCount++
		}
	}

	s.GroupCount = len(groups)
	s.Groups = make([]GroupSummary, 0, len(groups))
	for _, gs := range groups {
		s.Groups = append(s.Groups, *gs)
	}
	sort.Slice(s.Groups, func(i, j int) bool { return s.Groups[i].Name < s.Groups[j].Name })
	return s
}
