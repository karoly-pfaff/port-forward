import { ApiProperty, ApiPropertyOptional } from "@nestjs/swagger";
import type { ForwardStatus } from "@portier/shared";

/**
 * OpenAPI schema for a forward rule's runtime status — the body of the single-rule
 * start/stop responses and the item shape of `GET /api/status`. A metadata-only
 * decorated class that `implements ForwardStatus`.
 */
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
