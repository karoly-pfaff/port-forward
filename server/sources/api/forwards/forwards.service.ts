import { Inject, Injectable } from "@nestjs/common";
import type { ForwardRuleResponse } from "@portier/shared";
import { FORWARDS_READER, type ForwardsReader } from "./forwards.reader.js";
import { toForwardRuleResponse } from "./forward-rule-response.js";

/**
 * Behaviour for `GET /api/forwards`: returns each configured rule decorated with
 * its forward-purpose port advisories (via the shared `toForwardRuleResponse`,
 * the same decoration the create write uses, so they cannot drift). Read-only;
 * never throws.
 */
@Injectable()
export class ForwardsService {
  constructor(@Inject(FORWARDS_READER) private readonly reader: ForwardsReader) {}

  list(): ForwardRuleResponse[] {
    return this.reader.listRules().map(toForwardRuleResponse);
  }
}
