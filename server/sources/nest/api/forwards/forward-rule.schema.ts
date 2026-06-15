import { ApiProperty, ApiPropertyOptional } from "@nestjs/swagger";
import type { ForwardRule, ForwardRuleResponse, PortAdvisory } from "@portier/shared";
import { PortAdvisoryDto } from "../ports/port-advisory.schema.js";

/**
 * OpenAPI schemas for a forward rule and its API response (the rule plus its port
 * advisories). Metadata-only decorated classes; each `implements` its
 * `@portier/shared` contract type. `ForwardRuleDto` is the bare rule shape (also
 * used by the config-export response); `ForwardRuleResponseDto` adds `advisories`
 * (also used by the config-import response).
 */
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
