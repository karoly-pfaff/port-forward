import { Inject, Injectable } from "@nestjs/common";
import type { ForwardRuleResponse } from "@portier/shared";
import { mapManagerError } from "./manager-error.js";
import { toForwardRuleResponse } from "./forward-rule-response.js";
import { FORWARD_RULE_UPDATER, type ForwardRuleUpdater } from "./forwards.writer.js";

/**
 * Behaviour for `PATCH /api/forwards/:id`: updates a rule via the injected
 * updater (`ForwardManager.updateRule` — the SAME path Express uses, so partial
 * validation, merge semantics (unspecified fields are NOT overwritten),
 * duplicate-binding rejection, not-found handling, persistence rollback, and the
 * restart-only-when-a-forwarding-field-changes-on-a-running-rule semantics are
 * identical), then decorates the updated rule with its port advisories (the same
 * `toForwardRuleResponse` the list/create use). Domain errors are translated to
 * the matching Nest API exception via `mapManagerError` so the shared error
 * envelope produces the same status + body as Express (`400`/`404`/`409`).
 */
@Injectable()
export class UpdateForwardRuleService {
  constructor(@Inject(FORWARD_RULE_UPDATER) private readonly updater: ForwardRuleUpdater) {}

  async update(ruleId: string, patch: unknown): Promise<ForwardRuleResponse> {
    try {
      const rule = await this.updater.updateRule(ruleId, patch);
      return toForwardRuleResponse(rule);
    } catch (error) {
      mapManagerError(error);
    }
  }
}
