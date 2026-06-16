import { Inject, Injectable } from "@nestjs/common";
import type { ForwardRuleResponse } from "@portier/shared";
import { mapManagerError } from "./manager-error.js";
import { toForwardRuleResponse } from "./forward-rule-response.js";
import { FORWARD_RULE_CREATOR, type ForwardRuleCreator } from "./forwards.writer.js";

/**
 * Behaviour for `POST /api/forwards`: creates a rule via the injected creator
 * (`ForwardManager.addRule` — the SAME path Express uses, so validation /
 * duplicate-binding / persistence / autostart semantics are identical), then
 * decorates the created rule with its port advisories (the same `toForwardRuleResponse`
 * the list read uses). Domain errors are translated to the matching Nest API
 * exception via `mapManagerError` so the shared error envelope produces the same
 * status + body as Express (`400`/`409`).
 */
@Injectable()
export class CreateForwardRuleService {
  constructor(@Inject(FORWARD_RULE_CREATOR) private readonly creator: ForwardRuleCreator) {}

  async create(input: unknown): Promise<ForwardRuleResponse> {
    try {
      const rule = await this.creator.addRule(input);
      return toForwardRuleResponse(rule);
    } catch (error) {
      mapManagerError(error);
    }
  }
}
