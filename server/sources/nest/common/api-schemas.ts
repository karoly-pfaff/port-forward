import { ApiProperty, ApiPropertyOptional, type ApiPropertyOptions } from "@nestjs/swagger";
import type {
  ActivityEvent,
  ConfigPlanChange,
  ConfigPlanError,
  ConfigPlanOperation,
  ConfigPlanRequest,
  ConfigPlanResponse,
  ConfigPlanRuleSnapshot,
  ConfigPlanSummary,
  ConfigPlanWarning,
  DesiredConfig,
  DiagnosticCheck,
  DiagnosticSummary,
  ExportedConfig,
  ForwardRule,
  ForwardRuleInput,
  ForwardRuleResponse,
  ForwardStatus,
  GroupActionResponse,
  GroupActionResult,
  LiveConnectionsResponse,
  PortAdvisory,
  RuleDiagnosticsResult,
  RuleLiveSummary,
  RuntimeInfo,
  TcpConnectionInfo,
  UdpSessionInfo,
} from "@portier/shared";

/**
 * OpenAPI schema classes mirroring the `@portier/shared` REST types (the reusable
 * item shapes AND the per-endpoint response-body wrappers).
 *
 * `@nestjs/swagger` derives schemas from decorated CLASSES (TypeScript
 * interfaces/type aliases are erased), so each shape used in a response has a
 * decorated mirror here. Every class `implements` its shared interface, so the
 * TypeScript compiler fails if a field drifts from the contract. Explicit
 * `type`/`enum` are always given (the esbuild/tsx transform that runs generation
 * does not emit `design:type` reflection metadata — mirroring the Slice-7 pipe).
 *
 * These classes are **metadata-only**: they are never instantiated (controllers
 * reference them via `@ApiOkResponse({ type, isArray })`; the response mappers in
 * each feature's `*.response.dto.ts` return plain objects assignable to them). The
 * file is therefore coverage-excluded (documented in `vitest.config.ts`) — there
 * is no executable logic here, only decorator metadata. The response **mappers**
 * (the logic) stay in the feature files and are fully covered.
 */

export class PortAdvisoryDto implements PortAdvisory {
  @ApiProperty({
    enum: ["COMMON_PORT", "PRIVILEGED_PORT", "OUTSIDE_RECOMMENDED_RANGE", "LAN_EXPOSURE", "MANAGEMENT_LAN_EXPOSURE"],
    description: "Advisory code.",
  })
  code!: PortAdvisory["code"];

  @ApiProperty({ enum: ["info", "warning", "danger"], description: "Advisory severity." })
  severity!: PortAdvisory["severity"];

  @ApiProperty({ type: String, description: "Human-readable advisory message." })
  message!: string;
}

export class ForwardRuleDto implements ForwardRule {
  @ApiProperty({ type: String, description: "Stable rule id." })
  id!: string;

  @ApiProperty({ type: String, description: "Operator-facing rule name." })
  name!: string;

  @ApiProperty({ enum: ["tcp", "udp"], description: "Forwarding protocol." })
  protocol!: ForwardRule["protocol"];

  @ApiProperty({ type: String, description: "Local listen host.", example: "127.0.0.1" })
  listenHost!: string;

  @ApiProperty({ type: Number, description: "Local listen port.", example: 48010 })
  listenPort!: number;

  @ApiProperty({ type: String, description: "Forward target host." })
  targetHost!: string;

  @ApiProperty({ type: Number, description: "Forward target port." })
  targetPort!: number;

  @ApiProperty({ type: Boolean, description: "Whether the rule autostarts." })
  enabled!: boolean;

  @ApiPropertyOptional({
    enum: ["one-way", "bidirectional-last-client", "bidirectional-multi-client"],
    description: "UDP forwarding mode (UDP rules only).",
  })
  udpMode?: ForwardRule["udpMode"];

  @ApiPropertyOptional({ type: String, description: "Optional grouping label." })
  group?: string;
}

export class ForwardRuleResponseDto extends ForwardRuleDto implements ForwardRuleResponse {
  @ApiProperty({ type: [PortAdvisoryDto], description: "Port advisories for the rule's listen binding." })
  advisories!: PortAdvisory[];
}

export class ForwardStatusDto implements ForwardStatus {
  @ApiProperty({ type: String, description: "Id of the rule this status describes." })
  ruleId!: string;

  @ApiProperty({ type: Boolean, description: "Whether the forwarder is running." })
  running!: boolean;

  @ApiProperty({ enum: ["healthy", "warning", "error"], description: "Derived operator health." })
  health!: ForwardStatus["health"];

  @ApiPropertyOptional({ type: Number, description: "Active TCP connections." })
  activeConnections?: number;

  @ApiProperty({ type: Number, description: "Total bytes received." })
  bytesIn!: number;

  @ApiProperty({ type: Number, description: "Total bytes sent." })
  bytesOut!: number;

  @ApiPropertyOptional({ type: Number, description: "Total UDP packets received." })
  packetsIn?: number;

  @ApiPropertyOptional({ type: Number, description: "Total UDP packets sent." })
  packetsOut?: number;

  @ApiPropertyOptional({ type: Number, description: "Active UDP sessions." })
  activeUdpSessions?: number;

  @ApiPropertyOptional({ type: String, description: "Last error message, if any." })
  lastError?: string;

  @ApiPropertyOptional({ type: String, format: "date-time", description: "When the forwarder started." })
  startedAt?: string;
}

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

export class TcpConnectionInfoDto implements TcpConnectionInfo {
  @ApiProperty({ type: String }) id!: string;
  @ApiProperty({ type: String }) ruleId!: string;
  @ApiProperty({ type: String }) ruleName!: string;
  @ApiProperty({ enum: ["tcp"] }) protocol!: "tcp";
  @ApiProperty({ type: String }) clientAddress!: string;
  @ApiProperty({ type: Number }) clientPort!: number;
  @ApiProperty({ type: String }) targetAddress!: string;
  @ApiProperty({ type: Number }) targetPort!: number;
  @ApiProperty({ type: String, format: "date-time" }) startedAt!: string;
  @ApiProperty({ type: Number, description: "Connection duration in milliseconds." }) durationMs!: number;
  @ApiProperty({ type: Number }) bytesIn!: number;
  @ApiProperty({ type: Number }) bytesOut!: number;
  @ApiProperty({ enum: ["active"] }) status!: "active";
}

export class UdpSessionInfoDto implements UdpSessionInfo {
  @ApiProperty({ type: String }) id!: string;
  @ApiProperty({ type: String }) ruleId!: string;
  @ApiProperty({ type: String }) ruleName!: string;
  @ApiProperty({ enum: ["udp"] }) protocol!: "udp";
  @ApiProperty({ enum: ["one-way", "bidirectional-last-client", "bidirectional-multi-client"] })
  mode!: UdpSessionInfo["mode"];
  @ApiProperty({ type: String }) clientAddress!: string;
  @ApiProperty({ type: Number }) clientPort!: number;
  @ApiProperty({ type: String }) targetAddress!: string;
  @ApiProperty({ type: Number }) targetPort!: number;
  @ApiProperty({ type: String, format: "date-time" }) startedAt!: string;
  @ApiProperty({ type: String, format: "date-time" }) lastSeenAt!: string;
  @ApiProperty({ type: Number, description: "Idle time in milliseconds." }) idleMs!: number;
  @ApiProperty({ type: Number }) packetsIn!: number;
  @ApiProperty({ type: Number }) packetsOut!: number;
  @ApiProperty({ type: Number }) bytesIn!: number;
  @ApiProperty({ type: Number }) bytesOut!: number;
  @ApiProperty({ enum: ["active", "idle"] }) status!: UdpSessionInfo["status"];
}

export class RuleLiveSummaryDto implements RuleLiveSummary {
  @ApiProperty({ type: String }) ruleId!: string;
  @ApiProperty({ type: String }) ruleName!: string;
  @ApiProperty({ enum: ["tcp", "udp"] }) protocol!: RuleLiveSummary["protocol"];
  @ApiProperty({ type: Number }) activeTcpConnections!: number;
  @ApiProperty({ type: Number }) activeUdpSessions!: number;
  @ApiProperty({ type: Number }) bytesIn!: number;
  @ApiProperty({ type: Number }) bytesOut!: number;
  @ApiProperty({ type: Number }) packetsIn!: number;
  @ApiProperty({ type: Number }) packetsOut!: number;
  @ApiProperty({ type: String, format: "date-time", nullable: true, description: "Most recent traffic time, or null." })
  lastTrafficAt!: string | null;
}

// ── Request bodies ───────────────────────────────────────────────────────────

/**
 * Request body for `POST /api/forwards` (rule creation) — the `ForwardRuleInput`
 * shape (`Omit<ForwardRule, "id"> & { id?: string }`). This is a
 * **documentation/typing** DTO: validation is delegated to the shared
 * `validateForwardRule` (`@portier/shared`) inside `ForwardManager.addRule` — the
 * single contract validator — so no class-validator constraints are attached here
 * (re-expressing them would risk error-message/coercion/`id`-handling drift from
 * Express; a documented Express-parity exception). `implements ForwardRuleInput`
 * keeps the documented field set aligned with the contract.
 */
export class CreateForwardRuleBodyDto implements ForwardRuleInput {
  @ApiPropertyOptional({ type: String, description: "Optional client-supplied id (server generates a UUID when omitted)." })
  id?: string;
  @ApiProperty({ type: String, description: "Operator-facing rule name." }) name!: string;
  @ApiProperty({ enum: ["tcp", "udp"], description: "Forwarding protocol." }) protocol!: ForwardRule["protocol"];
  @ApiProperty({ type: String, example: "127.0.0.1", description: "Local listen host." }) listenHost!: string;
  @ApiProperty({ type: Number, example: 48010, description: "Local listen port." }) listenPort!: number;
  @ApiProperty({ type: String, description: "Forward target host." }) targetHost!: string;
  @ApiProperty({ type: Number, description: "Forward target port." }) targetPort!: number;
  @ApiProperty({ type: Boolean, description: "Whether the rule autostarts (a created enabled rule starts its forwarder)." })
  enabled!: boolean;
  @ApiPropertyOptional({
    enum: ["one-way", "bidirectional-last-client", "bidirectional-multi-client"],
    description: "UDP forwarding mode (UDP rules only).",
  })
  udpMode?: ForwardRule["udpMode"];
  @ApiPropertyOptional({ type: String, description: "Optional grouping label." }) group?: string;
}

/**
 * Request body for `PATCH /api/forwards/:id` (rule update) — a partial of the
 * rule definition (every field optional). Like the create body, this is a
 * **documentation/typing** DTO: validation is delegated to the shared
 * `validateForwardRulePatch` (`@portier/shared`) inside `ForwardManager.updateRule`
 * (the single contract validator, which also preserves the
 * absent-field-is-not-`undefined` merge semantics), so no class-validator
 * constraints are attached (a documented Express-parity exception).
 */
export class UpdateForwardRuleBodyDto implements Partial<ForwardRuleInput> {
  @ApiPropertyOptional({ type: String, description: "Operator-facing rule name." }) name?: string;
  @ApiPropertyOptional({ enum: ["tcp", "udp"], description: "Forwarding protocol (forwarding field — changing it restarts a running rule)." })
  protocol?: ForwardRule["protocol"];
  @ApiPropertyOptional({ type: String, description: "Local listen host (forwarding field)." }) listenHost?: string;
  @ApiPropertyOptional({ type: Number, description: "Local listen port (forwarding field)." }) listenPort?: number;
  @ApiPropertyOptional({ type: String, description: "Forward target host (forwarding field)." }) targetHost?: string;
  @ApiPropertyOptional({ type: Number, description: "Forward target port (forwarding field)." }) targetPort?: number;
  @ApiPropertyOptional({ type: Boolean, description: "Autostart flag (metadata — does not restart a running rule)." })
  enabled?: boolean;
  @ApiPropertyOptional({
    enum: ["one-way", "bidirectional-last-client", "bidirectional-multi-client"],
    description: "UDP forwarding mode (forwarding field).",
  })
  udpMode?: ForwardRule["udpMode"];
  @ApiPropertyOptional({ type: String, description: "Grouping label (metadata; empty/whitespace clears, null/absent leaves unchanged)." })
  group?: string;
}

// ── Per-endpoint response-body wrappers ──────────────────────────────────────

/** The Portier `/api` error envelope (`{ errors: [...] }`) — produced at runtime by `toApiError`. */
export class ApiErrorResponseDto {
  @ApiProperty({
    type: [String],
    description: "One or more human-readable error messages.",
    example: ["port must be an integer from 1 to 65535."],
  })
  errors!: string[];
}

/** Response body for `GET /api/activity`. */
export class ActivityListResponseDto {
  @ApiProperty({ type: [ActivityEventDto], description: "Matching activity events (newest first)." })
  events!: ActivityEvent[];
}

/** Response body for `GET /api/runtime`. */
export class RuntimeInfoResponseDto implements RuntimeInfo {
  @ApiProperty({ type: String, example: "Portier" }) name!: string;
  @ApiProperty({ type: String, example: "1.13.0" }) version!: string;
  @ApiProperty({ enum: ["node", "go"], description: "Runtime implementation serving the API." })
  runtime!: RuntimeInfo["runtime"];
  @ApiProperty({ enum: ["windows", "macos", "linux", "unknown"] }) platform!: RuntimeInfo["platform"];
  @ApiProperty({ enum: ["x64", "arm64", "unknown"] }) arch!: RuntimeInfo["arch"];
  @ApiProperty({ type: Number, description: "Seconds since the runtime started." }) uptimeSeconds!: number;
  @ApiProperty({ type: String, format: "date-time" }) startedAt!: string;
  @ApiProperty({ type: String }) managementHost!: string;
  @ApiProperty({ type: Number }) managementPort!: number;
  @ApiProperty({ type: String }) configPath!: string;
  @ApiProperty({ type: String }) staticDir!: string;
  @ApiProperty({ type: Boolean }) serviceMode!: boolean;
  @ApiProperty({ type: Number, description: "Process id." }) pid!: number;
}

/** Response body for `GET /api/config/export`. */
export class ConfigExportResponseDto implements ExportedConfig {
  @ApiProperty({ enum: ["1"], description: "Export schema version." }) version!: "1";
  @ApiProperty({ type: String, format: "date-time", description: "When the config was exported." })
  exportedAt!: string;
  @ApiProperty({ type: [ForwardRuleDto], description: "Exported forward rules." }) rules!: ForwardRule[];
}

/** Response body for `GET /api/connections`. */
export class ConnectionsResponseDto implements LiveConnectionsResponse {
  @ApiProperty({ type: String, format: "date-time", description: "When the snapshot was generated." })
  generatedAt!: string;
  @ApiProperty({ type: [TcpConnectionInfoDto], description: "Live TCP connections." })
  tcpConnections!: TcpConnectionInfo[];
  @ApiProperty({ type: [UdpSessionInfoDto], description: "Live UDP sessions." })
  udpSessions!: UdpSessionInfo[];
  @ApiProperty({ type: [RuleLiveSummaryDto], description: "Per-rule live traffic summary." })
  ruleSummaries!: RuleLiveSummary[];
}

/** One diagnostic check in a rule-diagnose result. */
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

/** The overall summary of a rule-diagnose result. */
export class DiagnosticSummaryDto implements DiagnosticSummary {
  @ApiProperty({ enum: ["pass", "warn", "fail"], description: "Overall diagnose outcome." })
  status!: DiagnosticSummary["status"];
  @ApiProperty({ type: String, description: "Human-readable summary message." }) message!: string;
}

/** Response body for `POST /api/forwards/:id/diagnose`. */
export class RuleDiagnosticsResultDto implements RuleDiagnosticsResult {
  @ApiProperty({ type: String, description: "Id of the diagnosed rule." }) ruleId!: string;
  @ApiProperty({ type: String, description: "Name of the diagnosed rule." }) ruleName!: string;
  @ApiProperty({ enum: ["tcp", "udp"], description: "Rule protocol." }) protocol!: RuleDiagnosticsResult["protocol"];
  @ApiProperty({ type: DiagnosticSummaryDto, description: "Overall summary." }) summary!: DiagnosticSummary;
  @ApiProperty({ type: [DiagnosticCheckDto], description: "Ordered diagnostic checks." }) checks!: DiagnosticCheck[];
  @ApiProperty({ type: String, format: "date-time", description: "When the diagnose ran (ISO 8601)." })
  diagnosedAt!: string;
}

/** One per-rule outcome of a group action. */
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

/** Response body for `POST /api/forwards/groups/:group/{start,stop}`. */
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

// A material rule field's value is always a string/number/boolean — an explicit
// schema both documents that accurately AND avoids the swagger generator's
// circular-type inference on a bare `unknown` property.
// A material rule field's value is a JSON scalar (string/number/boolean). An
// explicit object schema with no constraints documents it as a free-form value and
// avoids the swagger generator's circular-type reflection on a bare `unknown`
// property (the same generator-safe pattern as `ActivityEventDto.details`).
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
