import { Inject, Injectable } from "@nestjs/common";
import { getPortAdvisories, type ForwardRuleResponse } from "@portier/shared";
import { FORWARDS_READER, type ForwardsReader } from "./forwards.reader.js";

/**
 * Behaviour for `GET /api/forwards`: returns each configured rule decorated with
 * its forward-purpose port advisories — the SAME mapping the Express route's
 * `toRuleResponse` performs, reusing the shared `getPortAdvisories` so the
 * runtimes cannot drift. Read-only; never throws.
 */
@Injectable()
export class ForwardsService {
  constructor(@Inject(FORWARDS_READER) private readonly reader: ForwardsReader) {}

  list(): ForwardRuleResponse[] {
    return this.reader.listRules().map((rule) => ({
      ...rule,
      advisories: getPortAdvisories({
        port: rule.listenPort,
        listenHost: rule.listenHost,
        purpose: "forward",
      }),
    }));
  }
}
