package configplan

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"portier/service/sources/domain"
	"portier/service/sources/validation"
)

// RuleSnapshot is a point-in-time snapshot of a forward rule's material fields.
type RuleSnapshot struct {
	ID         *string                `json:"id,omitempty"`
	Name       string                 `json:"name"`
	Protocol   domain.ForwardProtocol `json:"protocol"`
	ListenHost string                 `json:"listenHost"`
	ListenPort int                    `json:"listenPort"`
	TargetHost string                 `json:"targetHost"`
	TargetPort int                    `json:"targetPort"`
	Enabled    bool                   `json:"enabled"`
	UdpMode    *domain.UdpMode        `json:"udpMode,omitempty"`
}

// Change records a single material field difference between current and desired.
type Change struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

// Operation describes what the plan would do for one rule.
type Operation struct {
	Type        string                 `json:"type"`
	RuleID      *string                `json:"ruleId,omitempty"`
	RuleName    string                 `json:"ruleName"`
	Protocol    domain.ForwardProtocol `json:"protocol"`
	Current     *RuleSnapshot          `json:"current,omitempty"`
	Desired     *RuleSnapshot          `json:"desired,omitempty"`
	Changes     []Change               `json:"changes,omitempty"`
	Destructive bool                   `json:"destructive"`
}

// Summary aggregates counts for the plan response.
type Summary struct {
	Add         int  `json:"add"`
	Update      int  `json:"update"`
	Remove      int  `json:"remove"`
	Unchanged   int  `json:"unchanged"`
	Destructive int  `json:"destructive"`
	HasDrift    bool `json:"hasDrift"`
	HasErrors   bool `json:"hasErrors"`
}

// PlanError is a structured error produced by the plan engine.
type PlanError struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Field   *string `json:"field,omitempty"`
}

// PlanWarning is a structured warning produced by the plan engine.
type PlanWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Response is the full output of BuildConfigPlan.
type Response struct {
	GeneratedAt string        `json:"generatedAt"`
	Mode        string        `json:"mode"`
	Summary     Summary       `json:"summary"`
	Operations  []Operation   `json:"operations"`
	Errors      []PlanError   `json:"errors"`
	Warnings    []PlanWarning `json:"warnings"`
}

// Input is the input to BuildConfigPlan.
type Input struct {
	CurrentRules []domain.ForwardRule
	DesiredRaw   json.RawMessage
	Now          *time.Time
}

var forwardingFields = map[string]bool{
	"protocol":   true,
	"listenHost": true,
	"listenPort": true,
	"targetHost": true,
	"targetPort": true,
	"udpMode":    true,
}

// BuildConfigPlan computes the difference between CurrentRules and the desired
// config encoded in DesiredRaw. It returns a structured plan without modifying
// any state or writing any files.
func BuildConfigPlan(input Input) Response {
	now := time.Now()
	if input.Now != nil {
		now = *input.Now
	}
	generatedAt := now.UTC().Format("2006-01-02T15:04:05.000Z")

	planErrors := make([]PlanError, 0)
	planWarnings := make([]PlanWarning, 0)
	operations := make([]Operation, 0)

	rawRules, ok := extractRulesRaw(input.DesiredRaw)
	if !ok {
		planErrors = append(planErrors, PlanError{
			Code:    "INVALID_DESIRED_CONFIG",
			Message: "desired must be an array of rules or an object with a rules array.",
		})
		return makeResponse(generatedAt, operations, planErrors, planWarnings)
	}

	validDesired := make([]RuleSnapshot, 0, len(rawRules))
	for i, rawRule := range rawRules {
		var ruleInput validation.ForwardRuleInput
		if err := json.Unmarshal(rawRule, &ruleInput); err != nil {
			planErrors = append(planErrors, PlanError{
				Code:    "INVALID_DESIRED_RULE",
				Message: fmt.Sprintf("Rule at index %d: Rule must be an object with valid field types: %v", i, err),
				Field:   strPtr(fmt.Sprintf("rules[%d]", i)),
			})
			continue
		}
		rule, errs := validation.ValidateForwardRuleInputWithOptionalID(ruleInput)
		if len(errs) > 0 {
			planErrors = append(planErrors, PlanError{
				Code:    "INVALID_DESIRED_RULE",
				Message: fmt.Sprintf("Rule at index %d: %s", i, strings.Join(errs, " ")),
				Field:   strPtr(fmt.Sprintf("rules[%d]", i)),
			})
			continue
		}
		validDesired = append(validDesired, ruleInputToSnapshot(ruleInput, rule))
	}

	if len(planErrors) > 0 {
		return makeResponse(generatedAt, operations, planErrors, planWarnings)
	}

	detectDuplicateIDs(validDesired, &planErrors)
	detectDuplicateKeys(validDesired, &planErrors)

	if len(planErrors) > 0 {
		return makeResponse(generatedAt, operations, planErrors, planWarnings)
	}

	currentByID := make(map[string]domain.ForwardRule, len(input.CurrentRules))
	currentByKey := make(map[string][]domain.ForwardRule)
	for _, rule := range input.CurrentRules {
		currentByID[rule.ID] = rule
		key := listenKeyFromRule(rule)
		currentByKey[key] = append(currentByKey[key], rule)
	}

	matchedCurrentIDs := make(map[string]bool)

	for _, desired := range validDesired {
		var matched *domain.ForwardRule

		if desired.ID != nil {
			if r, found := currentByID[*desired.ID]; found {
				r := r
				matched = &r
			}
		} else {
			key := listenKey(desired)
			candidates := currentByKey[key]
			if len(candidates) == 1 {
				r := candidates[0]
				matched = &r
			} else if len(candidates) > 1 {
				field := "id"
				planErrors = append(planErrors, PlanError{
					Code:    "AMBIGUOUS_CURRENT_MATCH",
					Message: fmt.Sprintf("Multiple current rules match identity key %q for desired rule %q. Use an explicit rule id to disambiguate.", key, desired.Name),
					Field:   &field,
				})
				continue
			}
		}

		if matched != nil {
			matchedCurrentIDs[matched.ID] = true
			currentSnap := ruleToSnapshot(*matched)
			changes := diffMaterialFields(currentSnap, desired)

			if len(changes) == 0 {
				operations = append(operations, Operation{
					Type:        "unchanged",
					RuleID:      strPtr(matched.ID),
					RuleName:    matched.Name,
					Protocol:    matched.Protocol,
					Current:     &currentSnap,
					Desired:     desiredPtr(desired),
					Destructive: false,
				})
			} else {
				operations = append(operations, Operation{
					Type:        "update",
					RuleID:      strPtr(matched.ID),
					RuleName:    desired.Name,
					Protocol:    desired.Protocol,
					Current:     &currentSnap,
					Desired:     desiredPtr(desired),
					Changes:     changes,
					Destructive: isDestructiveUpdate(changes),
				})
			}
		} else {
			operations = append(operations, Operation{
				Type:        "add",
				RuleName:    desired.Name,
				Protocol:    desired.Protocol,
				Desired:     desiredPtr(desired),
				Destructive: false,
			})
		}

		if desired.ListenHost == "0.0.0.0" {
			planWarnings = append(planWarnings, PlanWarning{
				Code:    "LAN_EXPOSURE",
				Message: fmt.Sprintf("Rule %q listens on 0.0.0.0 and will expose the port on all interfaces.", desired.Name),
			})
		}
	}

	if len(planErrors) > 0 {
		return makeResponse(generatedAt, operations, planErrors, planWarnings)
	}

	for _, rule := range input.CurrentRules {
		if !matchedCurrentIDs[rule.ID] {
			currentSnap := ruleToSnapshot(rule)
			operations = append(operations, Operation{
				Type:        "remove",
				RuleID:      strPtr(rule.ID),
				RuleName:    rule.Name,
				Protocol:    rule.Protocol,
				Current:     &currentSnap,
				Destructive: true,
			})
			planWarnings = append(planWarnings, PlanWarning{
				Code:    "REMOVE_EXISTING",
				Message: fmt.Sprintf("Rule %q (%s) will be removed.", rule.Name, rule.ID),
			})
		}
	}

	return makeResponse(generatedAt, operations, planErrors, planWarnings)
}

// ExtractRulesRaw parses desired JSON as either an array or an object with a
// rules array, matching TypeScript extractRulesArray semantics. Returns nil,
// false when the shape is unrecognized.
func ExtractRulesRaw(raw json.RawMessage) ([]json.RawMessage, bool) {
	return extractRulesRaw(raw)
}

func extractRulesRaw(raw json.RawMessage) ([]json.RawMessage, bool) {
	var asArray []json.RawMessage
	if err := json.Unmarshal(raw, &asArray); err == nil && asArray != nil {
		return asArray, true
	}
	// handle empty array []
	if strings.TrimSpace(string(raw)) == "[]" {
		return []json.RawMessage{}, true
	}
	var asObj struct {
		Rules []json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal(raw, &asObj); err == nil && asObj.Rules != nil {
		return asObj.Rules, true
	}
	return nil, false
}

func ruleInputToSnapshot(input validation.ForwardRuleInput, rule domain.ForwardRule) RuleSnapshot {
	snap := RuleSnapshot{
		Name:       rule.Name,
		Protocol:   rule.Protocol,
		ListenHost: rule.ListenHost,
		ListenPort: rule.ListenPort,
		TargetHost: rule.TargetHost,
		TargetPort: rule.TargetPort,
		Enabled:    rule.Enabled,
		UdpMode:    rule.UdpMode,
	}
	if input.ID != nil {
		id := strings.TrimSpace(*input.ID)
		snap.ID = &id
	}
	return snap
}

func ruleToSnapshot(rule domain.ForwardRule) RuleSnapshot {
	snap := RuleSnapshot{
		Name:       rule.Name,
		Protocol:   rule.Protocol,
		ListenHost: rule.ListenHost,
		ListenPort: rule.ListenPort,
		TargetHost: rule.TargetHost,
		TargetPort: rule.TargetPort,
		Enabled:    rule.Enabled,
		UdpMode:    rule.UdpMode,
	}
	snap.ID = strPtr(rule.ID)
	return snap
}

func listenKey(s RuleSnapshot) string {
	return fmt.Sprintf("%s:%s:%d", s.Protocol, s.ListenHost, s.ListenPort)
}

func listenKeyFromRule(rule domain.ForwardRule) string {
	return fmt.Sprintf("%s:%s:%d", rule.Protocol, rule.ListenHost, rule.ListenPort)
}

func diffMaterialFields(current, desired RuleSnapshot) []Change {
	changes := make([]Change, 0)

	if current.Name != desired.Name {
		changes = append(changes, Change{Field: "name", Before: current.Name, After: desired.Name})
	}
	if current.Protocol != desired.Protocol {
		changes = append(changes, Change{Field: "protocol", Before: string(current.Protocol), After: string(desired.Protocol)})
	}
	if current.ListenHost != desired.ListenHost {
		changes = append(changes, Change{Field: "listenHost", Before: current.ListenHost, After: desired.ListenHost})
	}
	if current.ListenPort != desired.ListenPort {
		changes = append(changes, Change{Field: "listenPort", Before: current.ListenPort, After: desired.ListenPort})
	}
	if current.TargetHost != desired.TargetHost {
		changes = append(changes, Change{Field: "targetHost", Before: current.TargetHost, After: desired.TargetHost})
	}
	if current.TargetPort != desired.TargetPort {
		changes = append(changes, Change{Field: "targetPort", Before: current.TargetPort, After: desired.TargetPort})
	}
	if current.Enabled != desired.Enabled {
		changes = append(changes, Change{Field: "enabled", Before: current.Enabled, After: desired.Enabled})
	}
	if !udpModeEqual(current.UdpMode, desired.UdpMode) {
		changes = append(changes, Change{Field: "udpMode", Before: udpModeVal(current.UdpMode), After: udpModeVal(desired.UdpMode)})
	}

	return changes
}

func udpModeEqual(a, b *domain.UdpMode) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func udpModeVal(m *domain.UdpMode) any {
	if m == nil {
		return nil
	}
	return string(*m)
}

func isDestructiveUpdate(changes []Change) bool {
	for _, c := range changes {
		if forwardingFields[c.Field] {
			return true
		}
	}
	return false
}

func detectDuplicateIDs(rules []RuleSnapshot, planErrors *[]PlanError) {
	seen := make(map[string]int)
	for _, r := range rules {
		if r.ID != nil {
			seen[*r.ID]++
		}
	}
	for id, count := range seen {
		if count > 1 {
			field := "id"
			*planErrors = append(*planErrors, PlanError{
				Code:    "DUPLICATE_DESIRED_ID",
				Message: fmt.Sprintf("Desired config has %d rules with the same id %q.", count, id),
				Field:   &field,
			})
		}
	}
}

func detectDuplicateKeys(rules []RuleSnapshot, planErrors *[]PlanError) {
	seen := make(map[string]int)
	for _, r := range rules {
		key := listenKey(r)
		seen[key]++
	}
	for key, count := range seen {
		if count > 1 {
			field := "listenPort"
			*planErrors = append(*planErrors, PlanError{
				Code:    "DUPLICATE_DESIRED_IDENTITY_KEY",
				Message: fmt.Sprintf("Desired config has %d rules with identity key %q.", count, key),
				Field:   &field,
			})
		}
	}
}

func makeResponse(generatedAt string, operations []Operation, planErrors []PlanError, planWarnings []PlanWarning) Response {
	add, update, remove, unchanged, destructive := 0, 0, 0, 0, 0
	for _, op := range operations {
		switch op.Type {
		case "add":
			add++
		case "update":
			update++
		case "remove":
			remove++
		case "unchanged":
			unchanged++
		}
		if op.Destructive {
			destructive++
		}
	}
	return Response{
		GeneratedAt: generatedAt,
		Mode:        "plan",
		Summary: Summary{
			Add:         add,
			Update:      update,
			Remove:      remove,
			Unchanged:   unchanged,
			Destructive: destructive,
			HasDrift:    add+update+remove > 0,
			HasErrors:   len(planErrors) > 0,
		},
		Operations: operations,
		Errors:     planErrors,
		Warnings:   planWarnings,
	}
}

func strPtr(s string) *string { return &s }

func desiredPtr(s RuleSnapshot) *RuleSnapshot { return &s }

// ── Apply orchestration ──────────────────────────────────────────────────────
// Apply transformation logic lives here, beside the plan engine — NOT in the
// HTTP handler. The handler (service/sources/api/api.go) owns request/response
// concerns (missing desired, yes/dryRun gating, status codes, calling the
// manager import). This helper owns the pure business transformation: deriving
// the desired-state replace rule list from a completed plan, injecting/preserving
// rule IDs, and computing applied counts. It does not mutate the plan, call the
// manager, write files, or start/stop rules. The TypeScript server mirrors this
// in server/sources/config-plan.ts (buildApplyImportFromPlan); validate:contract
// is the parity guard for the externally observable result.

// ApplyImportResult is the output of BuildApplyImportFromPlan.
type ApplyImportResult struct {
	// Rules is the desired-state rule list to pass to a "replace" import.
	// Remove operations are omitted.
	Rules []domain.ForwardRule
	// Applied holds the counts derived from the plan summary, returned verbatim
	// to the caller (keys: add, update, remove, unchanged).
	Applied map[string]int
}

// BuildApplyImportFromPlan derives the replace-import rule list and applied
// counts from a completed plan.
//
// ID rules (matching the prior inline handler behavior):
//   - "remove" operations are omitted from the result.
//   - When the desired snapshot carries an explicit id, it is preserved.
//   - For "unchanged"/"update" matches without an explicit desired id, the matched
//     current rule's id (op.RuleID) is preserved so identity is stable.
//   - Otherwise (a genuine add) a fresh id is generated via newID.
//
// newID is injected so tests can use a deterministic generator; production passes
// the shared domain.NewRuleID. The caller is responsible for only invoking the
// replace import when the plan actually has drift — this helper always returns the
// full desired list.
func BuildApplyImportFromPlan(plan Response, newID func() string) ApplyImportResult {
	rules := make([]domain.ForwardRule, 0, len(plan.Operations))
	for _, op := range plan.Operations {
		if op.Type == "remove" {
			continue
		}
		if op.Desired == nil {
			continue
		}
		snap := op.Desired

		var id string
		switch {
		case snap.ID != nil:
			id = *snap.ID
		case (op.Type == "unchanged" || op.Type == "update") && op.RuleID != nil:
			id = *op.RuleID
		default:
			id = newID()
		}

		rules = append(rules, domain.ForwardRule{
			ID:         id,
			Name:       snap.Name,
			Protocol:   snap.Protocol,
			ListenHost: snap.ListenHost,
			ListenPort: snap.ListenPort,
			TargetHost: snap.TargetHost,
			TargetPort: snap.TargetPort,
			Enabled:    snap.Enabled,
			UdpMode:    snap.UdpMode,
		})
	}

	return ApplyImportResult{
		Rules: rules,
		Applied: map[string]int{
			"add":       plan.Summary.Add,
			"update":    plan.Summary.Update,
			"remove":    plan.Summary.Remove,
			"unchanged": plan.Summary.Unchanged,
		},
	}
}
