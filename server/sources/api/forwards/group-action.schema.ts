import { ApiProperty, ApiPropertyOptional } from "@nestjs/swagger";
import type { GroupActionResponse, GroupActionResult } from "@portier/shared";

/**
 * OpenAPI schemas for `POST /api/forwards/groups/:group/{start,stop}` — one
 * per-rule result and the overall response. Metadata-only decorated classes; each
 * `implements` its `@portier/shared` contract type.
 */
export class GroupActionResultDto implements GroupActionResult {
  @ApiProperty({ type: String, description: "Id of the affected rule." }) ruleId!: string;
  @ApiProperty({ type: String, description: "Name of the affected rule." }) ruleName!: string;
  @ApiProperty({
    enum: ["started", "stopped", "skipped", "failed"],
    description: "Per-rule outcome.",
  })
  status!: GroupActionResult["status"];
  @ApiPropertyOptional({
    type: String,
    description: "Skip token (already_running / not_running) or, for failed, the error message.",
  })
  reason?: string;
}

export class GroupActionResponseDto implements GroupActionResponse {
  @ApiProperty({ type: String, description: "The group label the action targeted." }) group!: string;
  @ApiProperty({ enum: ["start", "stop"], description: "The action that ran." })
  action!: GroupActionResponse["action"];
  @ApiProperty({ type: Number, description: "Total rules in the group." }) total!: number;
  @ApiProperty({ type: Number, description: "Rules started/stopped." }) succeeded!: number;
  @ApiProperty({ type: Number, description: "Rules skipped (already in the target state)." }) skipped!: number;
  @ApiProperty({ type: Number, description: "Rules that failed." }) failed!: number;
  @ApiProperty({ type: [GroupActionResultDto], description: "Per-rule results, in rule order." })
  results!: GroupActionResult[];
}
