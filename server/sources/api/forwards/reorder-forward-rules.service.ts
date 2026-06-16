import { Inject, Injectable } from "@nestjs/common";
import type { ForwardRuleResponse } from "@portier/shared";
import { mapManagerError } from "./manager-error.js";
import { toForwardRuleResponse } from "./forward-rule-response.js";
import { FORWARD_RULES_REORDERER, type ForwardRulesReorderer } from "./forwards.writer.js";

/**
 * Behaviour for `POST /api/forwards/reorder`: reorders the rules via the injected
 * reorderer (`ForwardManager.reorderRules` — the SAME path Express uses, so the
 * id-existence check (unknown → 404), partial-set/duplicate tolerance, no-op empty
 * list, and persistence rollback are identical), then returns the full reordered
 * rule list decorated with port advisories (the same `toForwardRuleResponse` the
 * list/create/update use), exactly as the Express route does
 * (`manager.listRules().map(toRuleResponse)`). A domain `NotFoundError` (unknown
 * id) is translated to a `404` via `mapManagerError`; any other error (e.g. a
 * persist failure) is re-thrown → the filter's generic `500`.
 */
@Injectable()
export class ReorderForwardRulesService {
  constructor(@Inject(FORWARD_RULES_REORDERER) private readonly reorderer: ForwardRulesReorderer) {}

  async reorder(ids: string[]): Promise<ForwardRuleResponse[]> {
    try {
      await this.reorderer.reorderRules(ids);
      return this.reorderer.listRules().map(toForwardRuleResponse);
    } catch (error) {
      mapManagerError(error);
    }
  }
}
