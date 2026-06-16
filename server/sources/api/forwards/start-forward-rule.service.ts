import { Inject, Injectable } from "@nestjs/common";
import type { ForwardStatus } from "@portier/shared";
import { mapManagerError } from "./manager-error.js";
import { FORWARD_RULE_STARTER, type ForwardRuleStarter } from "./forwards.writer.js";

/**
 * Behaviour for `POST /api/forwards/:id/start`: starts a rule's forwarder via the
 * injected starter (`ForwardManager.startRule` — the SAME path Express uses, so
 * the not-found handling, idempotent already-running behaviour, listener start,
 * `rule.started` emission, and returned `ForwardStatus` are identical), and
 * returns the rule's current status. A domain `NotFoundError` (unknown id) is
 * translated to a `404` via `mapManagerError` so the shared error envelope
 * produces the same status + body as Express; any other error (e.g. the listen
 * port is in use) is re-thrown → the filter's generic `500`.
 */
@Injectable()
export class StartForwardRuleService {
  constructor(@Inject(FORWARD_RULE_STARTER) private readonly starter: ForwardRuleStarter) {}

  async start(ruleId: string): Promise<ForwardStatus> {
    try {
      return await this.starter.startRule(ruleId);
    } catch (error) {
      mapManagerError(error);
    }
  }
}
