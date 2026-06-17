package api

import (
	"portier/service/sources/advisory"
	"portier/service/sources/domain"
)

// rulesToResponses / toRuleResponse map domain rules to the API response shape,
// decorating each rule with port advisories. They are forwards-domain helpers
// but are shared: the forwards routes (list/create/update/reorder) and the
// legacy config import handler both use them, so they live in this small
// package-level file rather than in either feature module (v1.15 Slice 11).
func rulesToResponses(rules []domain.ForwardRule) []domain.ForwardRuleResponse {
	responses := make([]domain.ForwardRuleResponse, 0, len(rules))
	for _, rule := range rules {
		responses = append(responses, toRuleResponse(rule))
	}
	return responses
}

func toRuleResponse(rule domain.ForwardRule) domain.ForwardRuleResponse {
	return domain.ForwardRuleResponse{
		ID:         rule.ID,
		Name:       rule.Name,
		Protocol:   rule.Protocol,
		ListenHost: rule.ListenHost,
		ListenPort: rule.ListenPort,
		TargetHost: rule.TargetHost,
		TargetPort: rule.TargetPort,
		Enabled:    rule.Enabled,
		UdpMode:    rule.UdpMode,
		Group:      rule.Group,
		Advisories: advisory.GetPortAdvisories(advisory.Input{
			Port:       rule.ListenPort,
			ListenHost: rule.ListenHost,
			Purpose:    advisory.PurposeForward,
		}),
	}
}
