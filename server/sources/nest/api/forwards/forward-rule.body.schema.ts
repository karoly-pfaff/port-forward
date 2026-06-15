import { ApiProperty, ApiPropertyOptional } from "@nestjs/swagger";
import type { ForwardRule, ForwardRuleInput } from "@portier/shared";

/**
 * Request body for `POST /api/forwards` (rule creation) — the `ForwardRuleInput`
 * shape (`Omit<ForwardRule, "id"> & { id?: string }`). A documentation/typing-only
 * DTO: request validation is delegated to the shared `validateForwardRule`
 * (`@portier/shared`) inside `ForwardManager.addRule` (the single contract
 * validator), so no class-validator constraints are attached here — re-expressing
 * them would risk error-message/coercion/`id`-handling drift from the contract.
 * `implements ForwardRuleInput` keeps the documented field set aligned.
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
 * Request body for `PATCH /api/forwards/:id` (rule update) — a partial of the rule
 * definition (every field optional). Like the create body, this is a
 * documentation/typing-only DTO: validation is delegated to the shared
 * `validateForwardRulePatch` (`@portier/shared`) inside `ForwardManager.updateRule`
 * (which also preserves the absent-field-is-not-`undefined` merge semantics), so no
 * class-validator constraints are attached.
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
