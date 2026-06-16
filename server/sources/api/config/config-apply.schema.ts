import { ApiProperty, ApiPropertyOptional } from "@nestjs/swagger";
import type { ConfigAppliedCounts, ConfigApplyResponse, DesiredConfig } from "@portier/shared";
import { ConfigPlanResponseDto, DesiredConfigDto } from "./config-plan.schema.js";

/**
 * OpenAPI schemas for `POST /api/config/apply` — the applied counts, the request
 * body, and the response (which embeds the plan). Metadata-only decorated classes.
 */

/** The applied operation counts of a config apply. */
export class ConfigAppliedCountsDto implements ConfigAppliedCounts {
  @ApiProperty({ type: Number, description: "Rules added." }) add!: number;
  @ApiProperty({ type: Number, description: "Rules updated." }) update!: number;
  @ApiProperty({ type: Number, description: "Rules removed." }) remove!: number;
  @ApiProperty({ type: Number, description: "Rules unchanged." }) unchanged!: number;
}

/**
 * Request body for `POST /api/config/apply` (documentation/typing only — validation
 * is the inline `desired`-key-presence check in the service, matching the documented
 * `"desired" in body`; `yes`/`dryRun` are read as strict `=== true`).
 */
export class ConfigApplyBodyDto {
  @ApiProperty({ type: DesiredConfigDto, description: "The desired config (object with a rules array). Required." })
  desired!: DesiredConfig;
  @ApiPropertyOptional({ type: Boolean, description: "Confirm destructive operations. Required (true) when the plan is destructive." })
  yes?: boolean;
  @ApiPropertyOptional({ type: Boolean, description: "Preview only — return the plan + applied counts without mutating." })
  dryRun?: boolean;
}

/** Response body for `POST /api/config/apply`. */
export class ConfigApplyResponseDto implements ConfigApplyResponse {
  @ApiProperty({ type: Boolean, description: "Whether the apply succeeded (false when the plan has errors)." })
  ok!: boolean;
  @ApiProperty({ type: Boolean, description: "Whether this was a dry-run (no mutation)." }) dryRun!: boolean;
  @ApiProperty({ type: String, format: "date-time", description: "When the apply ran (ISO 8601)." })
  appliedAt!: string;
  @ApiProperty({ type: ConfigPlanResponseDto, description: "The plan that was applied (or would be)." })
  plan!: ConfigApplyResponse["plan"];
  @ApiProperty({ type: ConfigAppliedCountsDto, description: "The applied operation counts." })
  applied!: ConfigAppliedCounts;
}
