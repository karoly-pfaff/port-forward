import { ApiProperty, ApiPropertyOptional } from "@nestjs/swagger";
import type { DiagnosticCheck, DiagnosticSummary, RuleDiagnosticsResult } from "@portier/shared";

/**
 * OpenAPI schemas for `POST /api/forwards/:id/diagnose` — one check, the overall
 * summary, and the full result. Metadata-only decorated classes; each `implements`
 * its `@portier/shared` contract type.
 */
export class DiagnosticCheckDto implements DiagnosticCheck {
  @ApiProperty({ type: String, description: "Stable check id (e.g. listen-host, target-connect)." }) id!: string;
  @ApiProperty({ type: String, description: "Human-readable check label." }) label!: string;
  @ApiProperty({ enum: ["pass", "warn", "fail", "skip"], description: "Check outcome." })
  status!: DiagnosticCheck["status"];
  @ApiProperty({ type: String, description: "Human-readable check message." }) message!: string;
  @ApiPropertyOptional({
    type: "object",
    additionalProperties: true,
    description: "Optional structured details (string/number/boolean/null values).",
  })
  details?: Record<string, string | number | boolean | null>;
}

export class DiagnosticSummaryDto implements DiagnosticSummary {
  @ApiProperty({ enum: ["pass", "warn", "fail"], description: "Overall diagnose outcome." })
  status!: DiagnosticSummary["status"];
  @ApiProperty({ type: String, description: "Human-readable summary message." }) message!: string;
}

export class RuleDiagnosticsResultDto implements RuleDiagnosticsResult {
  @ApiProperty({ type: String, description: "Id of the diagnosed rule." }) ruleId!: string;
  @ApiProperty({ type: String, description: "Name of the diagnosed rule." }) ruleName!: string;
  @ApiProperty({ enum: ["tcp", "udp"], description: "Rule protocol." }) protocol!: RuleDiagnosticsResult["protocol"];
  @ApiProperty({ type: DiagnosticSummaryDto, description: "Overall summary." }) summary!: DiagnosticSummary;
  @ApiProperty({ type: [DiagnosticCheckDto], description: "Ordered diagnostic checks." }) checks!: DiagnosticCheck[];
  @ApiProperty({ type: String, format: "date-time", description: "When the diagnose ran (ISO 8601)." })
  diagnosedAt!: string;
}
