import { Inject, Injectable } from "@nestjs/common";
import type { RuleDiagnosticsResult } from "@portier/shared";
import { diagnoseRule } from "../../diagnostics/diagnose.js";
import { ApiNotFoundException } from "../common/api-errors.js";
import { CLOCK_READER, type ClockReader } from "../common/clock.reader.js";
import { DIAGNOSTIC_READER, type DiagnosticReader } from "./forwards-diagnostics.reader.js";

/**
 * Behaviour for `POST /api/forwards/:id/diagnose`: looks up the rule (an unknown
 * id → `404 { errors }`, matching the documented inline check), reads its running state,
 * and runs the shared `diagnoseRule` (the single diagnostics function, so the
 * checks, ordering, messages, and summary are identical). The volatile
 * `diagnosedAt` timestamp comes from the injected `ClockReader` (the shared
 * `CLOCK_READER`) so it can be pinned in parity tests — via the optional `now` argument of `diagnoseRule`. `diagnoseRule` catches all probe
 * errors internally (each check resolves), so it does not reject; an unexpected
 * throw would propagate to the filter's generic `500` (no leak).
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
