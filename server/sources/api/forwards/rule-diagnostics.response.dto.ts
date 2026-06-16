import type { RuleDiagnosticsResult } from "@portier/shared";
import { RuleDiagnosticsResultDto } from "./rule-diagnostics.schema.js";

export { RuleDiagnosticsResultDto } from "./rule-diagnostics.schema.js";

/**
 * Maps a `RuleDiagnosticsResult` to the `RuleDiagnosticsResultDto` shape at the
 * HTTP boundary — a fresh object with a fresh `summary`, a fresh `checks` array of
 * fresh check objects (and fresh `details` where present), so the controller never
 * returns the domain result object and the mapper cannot mutate it. The DTO class
 * is the OpenAPI schema (in `rule-diagnostics.schema.ts`); this mapper is the covered
 * logic. Matches the documented diagnose response shape.
 */
export function toRuleDiagnosticsResponseDto(result: RuleDiagnosticsResult): RuleDiagnosticsResultDto {
  return {
    ruleId: result.ruleId,
    ruleName: result.ruleName,
    protocol: result.protocol,
    summary: { ...result.summary },
    checks: result.checks.map((check) => ({
      ...check,
      ...(check.details === undefined ? {} : { details: { ...check.details } }),
    })),
    diagnosedAt: result.diagnosedAt,
  };
}
