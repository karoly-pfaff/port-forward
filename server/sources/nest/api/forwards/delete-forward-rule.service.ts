import { Inject, Injectable } from "@nestjs/common";
import { mapManagerError } from "../../common/manager-error.js";
import { FORWARD_RULE_DELETER, type ForwardRuleDeleter } from "./forwards.writer.js";

/**
 * Behaviour for `DELETE /api/forwards/:id`: deletes a rule via the injected
 * deleter (`ForwardManager.deleteRule` — the SAME path Express uses, so the
 * not-found handling, runtime cleanup (a running forwarder is stopped first),
 * persistence rollback, and `rule.deleted` activity emission are identical). The
 * route returns `204` with no body, so this resolves with no value. A domain
 * `NotFoundError` (unknown id) is translated to a `404` via `mapManagerError` so
 * the shared error envelope produces the same status + body as Express; any other
 * error (e.g. a persist failure) is re-thrown → the filter's generic `500`.
 */
@Injectable()
export class DeleteForwardRuleService {
  constructor(@Inject(FORWARD_RULE_DELETER) private readonly deleter: ForwardRuleDeleter) {}

  async delete(ruleId: string): Promise<void> {
    try {
      await this.deleter.deleteRule(ruleId);
    } catch (error) {
      mapManagerError(error);
    }
  }
}
