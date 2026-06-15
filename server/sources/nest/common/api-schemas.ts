import { ApiProperty, ApiPropertyOptional } from "@nestjs/swagger";
import type {
  ActivityEvent,
  ExportedConfig,
  ForwardRule,
  ForwardRuleInput,
  ForwardRuleResponse,
  ForwardStatus,
  LiveConnectionsResponse,
  PortAdvisory,
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
