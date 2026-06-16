import { ApiProperty, ApiPropertyOptional } from "@nestjs/swagger";
import type { ActivityEvent } from "@portier/shared";

/**
 * OpenAPI schemas for the activity feature — a single activity event and the
 * `GET /api/activity` response wrapper. Metadata-only decorated classes; the event
 * `implements ActivityEvent` so a contract drift is a compile error.
 */
export class ActivityEventDto implements ActivityEvent {
  @ApiProperty({ type: String, description: "Stable event id." })
  id!: string;

  @ApiProperty({ type: String, format: "date-time", description: "Event timestamp (ISO 8601)." })
  timestamp!: string;

  @ApiProperty({ type: String, description: "Activity event type (e.g. rule.created, config.imported)." })
  type!: ActivityEvent["type"];

  @ApiProperty({ enum: ["info", "success", "warning", "error"], description: "Event severity." })
  severity!: ActivityEvent["severity"];

  @ApiPropertyOptional({ type: String, description: "Related rule id, if any." })
  ruleId?: string;

  @ApiPropertyOptional({ type: String, description: "Related rule name, if any." })
  ruleName?: string;

  @ApiPropertyOptional({ enum: ["tcp", "udp"], description: "Related protocol, if any." })
  protocol?: ActivityEvent["protocol"];

  @ApiProperty({ type: String, description: "Human-readable message." })
  message!: string;

  @ApiPropertyOptional({
    type: "object",
    additionalProperties: true,
    description: "Optional structured details (string/number/boolean/null values).",
  })
  details?: Record<string, string | number | boolean | null>;
}

/** Response body for `GET /api/activity`. */
export class ActivityListResponseDto {
  @ApiProperty({ type: [ActivityEventDto], description: "Matching activity events (newest first)." })
  events!: ActivityEvent[];
}
