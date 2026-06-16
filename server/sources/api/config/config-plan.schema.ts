import { ApiProperty, ApiPropertyOptional, type ApiPropertyOptions } from "@nestjs/swagger";
import type {
  ConfigPlanChange,
  ConfigPlanError,
  ConfigPlanOperation,
  ConfigPlanRequest,
  ConfigPlanResponse,
  ConfigPlanRuleSnapshot,
  ConfigPlanSummary,
  ConfigPlanWarning,
  DesiredConfig,
} from "@portier/shared";

/**
 * OpenAPI schemas for the config plan (`POST /api/config/plan`) and the desired
 * config shared by plan/apply requests. Metadata-only decorated classes; each
 * `implements` its `@portier/shared` contract type.
 */

// A material rule field's value is a JSON scalar (string/number/boolean). An
// explicit object schema with no constraints documents it as a free-form value and
// avoids the swagger generator's circular-type reflection on a bare `unknown`
// property (the same generator-safe pattern as the activity-event `details`).
const ruleFieldValueSchema = (description: string): ApiPropertyOptions =>
  ({ type: "object", additionalProperties: true, description }) as ApiPropertyOptions;

/** A single before→after field change within a config-plan update operation. */
export class ConfigPlanChangeDto implements ConfigPlanChange {
  @ApiProperty({ type: String, description: "The changed material field." }) field!: string;
  @ApiProperty(ruleFieldValueSchema("Value before the change (a JSON scalar).")) before!: unknown;
  @ApiProperty(ruleFieldValueSchema("Value after the change (a JSON scalar).")) after!: unknown;
}

/** A desired/current rule snapshot inside a config-plan operation. */
export class ConfigPlanRuleSnapshotDto implements ConfigPlanRuleSnapshot {
  @ApiPropertyOptional({ type: String, description: "Rule id (absent for an added rule)." }) id?: string;
  @ApiProperty({ type: String }) name!: string;
  @ApiProperty({ enum: ["tcp", "udp"] }) protocol!: ConfigPlanRuleSnapshot["protocol"];
  @ApiProperty({ type: String }) listenHost!: string;
  @ApiProperty({ type: Number }) listenPort!: number;
  @ApiProperty({ type: String }) targetHost!: string;
  @ApiProperty({ type: Number }) targetPort!: number;
  @ApiProperty({ type: Boolean }) enabled!: boolean;
  @ApiPropertyOptional({ enum: ["one-way", "bidirectional-last-client", "bidirectional-multi-client"] })
  udpMode?: ConfigPlanRuleSnapshot["udpMode"];
  @ApiPropertyOptional({ type: String, description: "Optional group label." }) group?: string;
}

/** One operation (add/update/remove/unchanged) in a config plan. */
export class ConfigPlanOperationDto implements ConfigPlanOperation {
  @ApiProperty({ enum: ["add", "update", "remove", "unchanged"], description: "Operation type." })
  type!: ConfigPlanOperation["type"];
  @ApiPropertyOptional({ type: String, description: "Affected rule id, if known." }) ruleId?: string;
  @ApiProperty({ type: String }) ruleName!: string;
  @ApiProperty({ enum: ["tcp", "udp"] }) protocol!: ConfigPlanOperation["protocol"];
  @ApiPropertyOptional({ type: ConfigPlanRuleSnapshotDto, description: "Current rule snapshot." })
  current?: ConfigPlanRuleSnapshot;
  @ApiPropertyOptional({ type: ConfigPlanRuleSnapshotDto, description: "Desired rule snapshot." })
  desired?: ConfigPlanRuleSnapshot;
  @ApiPropertyOptional({ type: [ConfigPlanChangeDto], description: "Field-level changes (update only)." })
  changes?: ConfigPlanChange[];
  @ApiProperty({ type: Boolean, description: "Whether the operation restarts/removes a forwarder." })
  destructive!: boolean;
}

/** The counts/flags summary of a config plan. */
export class ConfigPlanSummaryDto implements ConfigPlanSummary {
  @ApiProperty({ type: Number }) add!: number;
  @ApiProperty({ type: Number }) update!: number;
  @ApiProperty({ type: Number }) remove!: number;
  @ApiProperty({ type: Number }) unchanged!: number;
  @ApiProperty({ type: Number, description: "Number of destructive operations." }) destructive!: number;
  @ApiProperty({ type: Boolean, description: "Whether the desired config differs from current." }) hasDrift!: boolean;
  @ApiProperty({ type: Boolean, description: "Whether the plan has validation errors." }) hasErrors!: boolean;
}

/** A plan-level validation error. */
export class ConfigPlanErrorDto implements ConfigPlanError {
  @ApiProperty({ type: String }) code!: string;
  @ApiProperty({ type: String }) message!: string;
  @ApiPropertyOptional({ type: String }) field?: string;
}

/** A plan-level advisory warning. */
export class ConfigPlanWarningDto implements ConfigPlanWarning {
  @ApiProperty({ type: String }) code!: string;
  @ApiProperty({ type: String }) message!: string;
}

/** Response body for `POST /api/config/plan`. */
export class ConfigPlanResponseDto implements ConfigPlanResponse {
  @ApiProperty({ type: String, format: "date-time", description: "When the plan was generated (ISO 8601)." })
  generatedAt!: string;
  @ApiProperty({ enum: ["plan"], description: "Discriminator." }) mode!: "plan";
  @ApiProperty({ type: ConfigPlanSummaryDto }) summary!: ConfigPlanSummary;
  @ApiProperty({ type: [ConfigPlanOperationDto], description: "Per-rule operations." })
  operations!: ConfigPlanOperation[];
  @ApiProperty({ type: [ConfigPlanErrorDto], description: "Plan validation errors." })
  errors!: ConfigPlanError[];
  @ApiProperty({ type: [ConfigPlanWarningDto], description: "Plan advisory warnings." })
  warnings!: ConfigPlanWarning[];
}

/** The desired config inside a plan/apply request — an object with a `rules` array. */
export class DesiredConfigDto implements DesiredConfig {
  @ApiProperty({ type: [ConfigPlanRuleSnapshotDto], description: "Desired rule snapshots." })
  rules!: ConfigPlanRuleSnapshot[];
}

/**
 * Request body for `POST /api/config/plan` (documentation/typing only — validation
 * is the inline `desired`-key-presence check in the service, matching Express's
 * `"desired" in body`; a bare `rules` array is also accepted at runtime).
 */
export class ConfigPlanBodyDto implements ConfigPlanRequest {
  @ApiProperty({ type: DesiredConfigDto, description: "The desired config (object with a rules array). Required." })
  desired!: DesiredConfig;
}
