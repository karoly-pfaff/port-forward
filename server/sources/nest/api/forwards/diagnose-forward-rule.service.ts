import { Inject, Injectable } from "@nestjs/common";
import type { RuleDiagnosticsResult } from "@portier/shared";
import { diagnoseRule } from "../../../diagnose.js";
import { ApiNotFoundException } from "../../common/api-errors.js";
import { CLOCK_READER, type ClockReader } from "../../common/clock.reader.js";
import { DIAGNOSTIC_READER, type DiagnosticReader } from "./forwards-diagnostics.reader.js";

/**
 * Behaviour for `POST /api/forwards/:id/diagnose`: looks up the rule (an unknown
 * id → `404 { errors }`, matching Express's inline check), reads its running state,
 * and runs the shared `diagnoseRule` (the SAME function Express calls, so the
 * checks, ordering, messages, and summary are identical). The volatile
 * `diagnosedAt` timestamp comes from the injected `ClockReader` (the shared
 * `CLOCK_READER`) so it can be pinned in parity tests — exactly as Express threads
 * its `AppOptions.now` seam into `diagnoseRule`. `diagnoseRule` catches all probe
 * errors internally (each check resolves), so it does not reject; an unexpected
 * throw would propagate to the filter's generic `500`, matching Express's
 * `next(error)`.
 */
@Injectable()
export class DiagnoseForwardRuleService {
  constructor(
    @Inject(DIAGNOSTIC_READER) private readonly reader: DiagnosticReader,
    @Inject(CLOCK_READER) private readonly clock: ClockReader
  ) {}

  async diagnose(ruleId: string): Promise<RuleDiagnosticsResult> {
    const rule = this.reader.getRule(ruleId);
    if (!rule) {
      throw new ApiNotFoundException([`Forward rule ${ruleId} was not found.`]);
    }
    const isRunning = this.reader.getStatus(rule.id).running;
    return diagnoseRule(rule, isRunning, this.clock.now());
  }
}
