import { ApiProperty } from "@nestjs/swagger";
import type { PortAdvisory } from "@portier/shared";

/**
 * OpenAPI schema for a single port advisory — the item shape of
 * `GET /api/ports/advisory` and of each rule's `advisories` in the forwards
 * responses. A metadata-only decorated class that `implements PortAdvisory` so a
 * drift from the `@portier/shared` contract is a compile error.
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
