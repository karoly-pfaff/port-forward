import { ApiProperty } from "@nestjs/swagger";
import type {
  LiveConnectionsResponse,
  RuleLiveSummary,
  TcpConnectionInfo,
  UdpSessionInfo,
} from "@portier/shared";

/**
 * OpenAPI schemas for the live-connections feature — the TCP/UDP/summary item
 * shapes and the `GET /api/connections` response wrapper. Metadata-only decorated
 * classes; each `implements` its `@portier/shared` contract type.
 */
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
