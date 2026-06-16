import { getPortAdvisories, type ForwardRule, type ForwardRuleResponse } from "@portier/shared";

/**
 * Decorates a forward rule with its forward-purpose port advisories — the SAME
 * rule→response mapping performs, via the shared
 * `getPortAdvisories` so the runtimes cannot drift. Used by both the list read
 * (`GET /api/forwards`) and the create write (`POST /api/forwards`) so a single
 * rule and a listed rule are decorated identically.
 */
export function toForwardRuleResponse(rule: ForwardRule): ForwardRuleResponse {
  return {
    ...rule,
    advisories: getPortAdvisories({
      port: rule.listenPort,
      listenHost: rule.listenHost,
      purpose: "forward",
    }),
  };
}
