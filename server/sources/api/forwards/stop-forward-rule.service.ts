import { Inject, Injectable } from "@nestjs/common";
import type { ForwardStatus } from "@portier/shared";
import { mapManagerError } from "./manager-error.js";
import { FORWARD_RULE_STOPPER, type ForwardRuleStopper } from "./forwards.writer.js";

/**
 * Behaviour for `POST /api/forwards/:id/stop`: stops a rule's forwarder via the
 * injected stopper (`ForwardManager.stopRule` — the SAME path Express uses, so the
 * not-found handling, idempotent already-stopped behaviour, listener close,
 * `rule.stopped` emission, and returned `ForwardStatus` are identical), and returns
 * the rule's current status. A domain `NotFoundError` (unknown id) is translated to
 * a `404` via `mapManagerError` so the shared error envelope produces the same
 * status + body as Express; any other error is re-thrown → the filter's generic
 * `500`.
 */
@Injectable()
export class StopForwardRuleService {
  constructor(@Inject(FORWARD_RULE_STOPPER) private readonly stopper: ForwardRuleStopper) {}

  async stop(ruleId: string): Promise<ForwardStatus> {
    try {
      return await this.stopper.stopRule(ruleId);
    } catch (error) {
      mapManagerError(error);
    }
  }
}
