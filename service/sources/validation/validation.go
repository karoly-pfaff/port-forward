package validation

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"portier/service/sources/domain"
)

type ForwardRuleInput struct {
	ID         *string                 `json:"id"`
	Name       *string                 `json:"name"`
	Protocol   *domain.ForwardProtocol `json:"protocol"`
	ListenHost *string                 `json:"listenHost"`
	ListenPort *int                    `json:"listenPort"`
	TargetHost *string                 `json:"targetHost"`
	TargetPort *int                    `json:"targetPort"`
	Enabled    *bool                   `json:"enabled"`
	UdpMode    *domain.UdpMode         `json:"udpMode"`
	Group      *string                 `json:"group"`
}

type ForwardRulePatch struct {
	ID         *string
	Name       *string
	Protocol   *domain.ForwardProtocol
	ListenHost *string
	ListenPort *int
	TargetHost *string
	TargetPort *int
	Enabled    *bool
	UdpMode    *domain.UdpMode
	// Group carries the patch intent for the group label: nil = unchanged,
	// non-nil = set to the (trimmed) value, non-nil empty/whitespace = clear.
	Group *string
}

func DecodeAndValidateForwardRule(raw json.RawMessage) (domain.ForwardRule, []string) {
	var input ForwardRuleInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return domain.ForwardRule{}, []string{fmt.Sprintf("Rule must be an object with valid field types: %v", err)}
	}
	return ValidateForwardRuleInput(input)
}

func ValidateForwardRuleInput(input ForwardRuleInput) (domain.ForwardRule, []string) {
	return validateForwardRuleInput(input, true)
}

func ValidateForwardRuleInputWithOptionalID(input ForwardRuleInput) (domain.ForwardRule, []string) {
	return validateForwardRuleInput(input, false)
}

func DecodeAndValidateForwardRulePatch(raw json.RawMessage) (ForwardRulePatch, []string) {
	var input ForwardRuleInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return ForwardRulePatch{}, []string{fmt.Sprintf("Patch must be an object with valid field types: %v", err)}
	}
	return ValidateForwardRulePatch(input)
}

func ValidateForwardRulePatch(input ForwardRuleInput) (ForwardRulePatch, []string) {
	errors := make([]string, 0)

	if input.ID != nil && strings.TrimSpace(*input.ID) == "" {
		errors = append(errors, "id must be a non-empty string when provided.")
	}
	if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
		errors = append(errors, "name must be a non-empty string.")
	}
	if input.Protocol != nil && *input.Protocol != domain.ProtocolTCP && *input.Protocol != domain.ProtocolUDP {
		errors = append(errors, "protocol must be tcp or udp.")
	}
	if input.ListenHost != nil && strings.TrimSpace(*input.ListenHost) == "" {
		errors = append(errors, "listenHost must be a non-empty string.")
	}
	if input.ListenPort != nil && !isValidPort(*input.ListenPort) {
		errors = append(errors, "listenPort must be an integer from 1 to 65535.")
	}
	if input.TargetHost != nil && strings.TrimSpace(*input.TargetHost) == "" {
		errors = append(errors, "targetHost must be a non-empty string.")
	}
	if input.TargetPort != nil && !isValidPort(*input.TargetPort) {
		errors = append(errors, "targetPort must be an integer from 1 to 65535.")
	}
	if input.UdpMode != nil && !isValidUdpMode(*input.UdpMode) {
		errors = append(errors, "udpMode must be one-way, bidirectional-last-client, or bidirectional-multi-client.")
	}
	errors = append(errors, validateGroup(input.Group)...)

	if len(errors) > 0 {
		return ForwardRulePatch{}, errors
	}

	patch := ForwardRulePatch{
		ID:         trimOptional(input.ID),
		Name:       trimOptional(input.Name),
		Protocol:   input.Protocol,
		ListenHost: trimOptional(input.ListenHost),
		ListenPort: input.ListenPort,
		TargetHost: trimOptional(input.TargetHost),
		TargetPort: input.TargetPort,
		Enabled:    input.Enabled,
		UdpMode:    input.UdpMode,
		// Keep the raw pointer so ApplyPatch can distinguish unchanged (nil)
		// from clear (non-nil empty) from set (non-nil non-empty).
		Group: input.Group,
	}
	return patch, nil
}

func validateForwardRuleInput(input ForwardRuleInput, requireID bool) (domain.ForwardRule, []string) {
	errors := make([]string, 0)

	if requireID && (input.ID == nil || strings.TrimSpace(*input.ID) == "") {
		errors = append(errors, "id is required.")
	}
	if !requireID && input.ID != nil && strings.TrimSpace(*input.ID) == "" {
		errors = append(errors, "id must be a non-empty string when provided.")
	}
	if input.Name == nil || strings.TrimSpace(*input.Name) == "" {
		errors = append(errors, "name is required.")
	}
	if input.Protocol == nil || (*input.Protocol != domain.ProtocolTCP && *input.Protocol != domain.ProtocolUDP) {
		errors = append(errors, "protocol must be tcp or udp.")
	}
	if input.ListenHost == nil || strings.TrimSpace(*input.ListenHost) == "" {
		errors = append(errors, "listenHost is required.")
	}
	if input.ListenPort == nil || !isValidPort(*input.ListenPort) {
		errors = append(errors, "listenPort must be an integer from 1 to 65535.")
	}
	if input.TargetHost == nil || strings.TrimSpace(*input.TargetHost) == "" {
		errors = append(errors, "targetHost is required.")
	}
	if input.TargetPort == nil || !isValidPort(*input.TargetPort) {
		errors = append(errors, "targetPort must be an integer from 1 to 65535.")
	}
	if input.Enabled == nil {
		errors = append(errors, "enabled must be a boolean.")
	}
	if input.Protocol != nil && *input.Protocol == domain.ProtocolUDP && input.UdpMode != nil && !isValidUdpMode(*input.UdpMode) {
		errors = append(errors, "udpMode must be one-way, bidirectional-last-client, or bidirectional-multi-client.")
	}
	if input.Protocol != nil && *input.Protocol == domain.ProtocolTCP && input.UdpMode != nil {
		errors = append(errors, "udpMode is only valid for UDP rules.")
	}
	errors = append(errors, validateGroup(input.Group)...)

	if len(errors) > 0 {
		return domain.ForwardRule{}, errors
	}

	rule := domain.ForwardRule{
		Name:       strings.TrimSpace(*input.Name),
		Protocol:   *input.Protocol,
		ListenHost: strings.TrimSpace(*input.ListenHost),
		ListenPort: *input.ListenPort,
		TargetHost: strings.TrimSpace(*input.TargetHost),
		TargetPort: *input.TargetPort,
		Enabled:    *input.Enabled,
		Group:      normalizeGroup(input.Group),
	}

	if input.ID != nil {
		rule.ID = strings.TrimSpace(*input.ID)
	}

	if rule.Protocol == domain.ProtocolUDP {
		mode := domain.UdpModeOneWay
		if input.UdpMode != nil {
			mode = *input.UdpMode
		}
		rule.UdpMode = &mode
	}

	return rule, nil
}

func InputFromRule(rule domain.ForwardRule) ForwardRuleInput {
	return ForwardRuleInput{
		ID:         &rule.ID,
		Name:       &rule.Name,
		Protocol:   &rule.Protocol,
		ListenHost: &rule.ListenHost,
		ListenPort: &rule.ListenPort,
		TargetHost: &rule.TargetHost,
		TargetPort: &rule.TargetPort,
		Enabled:    &rule.Enabled,
		UdpMode:    rule.UdpMode,
		Group:      rule.Group,
	}
}

func ApplyPatch(rule domain.ForwardRule, patch ForwardRulePatch) (domain.ForwardRule, []string) {
	next := rule
	if patch.Name != nil {
		next.Name = *patch.Name
	}
	if patch.Protocol != nil {
		next.Protocol = *patch.Protocol
	}
	if patch.ListenHost != nil {
		next.ListenHost = *patch.ListenHost
	}
	if patch.ListenPort != nil {
		next.ListenPort = *patch.ListenPort
	}
	if patch.TargetHost != nil {
		next.TargetHost = *patch.TargetHost
	}
	if patch.TargetPort != nil {
		next.TargetPort = *patch.TargetPort
	}
	if patch.Enabled != nil {
		next.Enabled = *patch.Enabled
	}
	if patch.UdpMode != nil {
		next.UdpMode = patch.UdpMode
	}
	if next.Protocol == domain.ProtocolTCP {
		next.UdpMode = nil
	}
	if patch.Group != nil {
		// non-nil: set (or clear when empty/whitespace). normalizeGroup returns
		// nil for empty/whitespace, which clears the group.
		next.Group = normalizeGroup(patch.Group)
	}

	return ValidateForwardRuleInput(InputFromRule(next))
}

func isValidPort(port int) bool {
	return port >= 1 && port <= 65535
}

func isValidUdpMode(mode domain.UdpMode) bool {
	return mode == domain.UdpModeOneWay ||
		mode == domain.UdpModeBidirectionalLast ||
		mode == domain.UdpModeBidirectionalMulti
}

// validateGroup returns error messages for an invalid group label (mirrors the
// TypeScript collectGroupErrors). nil/empty/whitespace are accepted (they
// normalize to "no group"); a present non-empty value must be at most
// domain.GroupMaxLength characters with no control characters.
func validateGroup(group *string) []string {
	if group == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*group)
	if trimmed == "" {
		return nil
	}
	errs := make([]string, 0)
	if utf8.RuneCountInString(trimmed) > domain.GroupMaxLength {
		errs = append(errs, fmt.Sprintf("group must be %d characters or fewer.", domain.GroupMaxLength))
	}
	if hasControlChar(trimmed) {
		errs = append(errs, "group must not contain control characters.")
	}
	return errs
}

// ValidateGroupName validates a group label used as a group-operation target
// (e.g. the :group path of a group start/stop request) and returns the trimmed
// value plus any error messages. Unlike validateGroup, an empty/whitespace value
// is rejected ("group is required.") — you cannot act on "no group". Mirrors the
// TypeScript validateGroupName; same error wording (parity-tested).
func ValidateGroupName(raw string) (string, []string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", []string{"group is required."}
	}
	g := trimmed
	return trimmed, validateGroup(&g)
}

// normalizeGroup trims a group label and returns nil when the result is empty,
// so an ungrouped rule omits the field. Assumes the value passed validateGroup.
func normalizeGroup(group *string) *string {
	if group == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*group)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// hasControlChar reports whether s contains a C0 control character (U+0000-
// U+001F) or DEL (U+007F).
func hasControlChar(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}
