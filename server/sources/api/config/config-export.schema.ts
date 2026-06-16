import { ApiProperty } from "@nestjs/swagger";
import type { ExportedConfig, ForwardRule } from "@portier/shared";
import { ForwardRuleDto } from "../forwards/forward-rule.schema.js";

/** Response body for `GET /api/config/export` — metadata-only, `implements ExportedConfig`. */
export class ConfigExportResponseDto implements ExportedConfig {
  @ApiProperty({ enum: ["1"], description: "Export schema version." }) version!: "1";
  @ApiProperty({ type: String, format: "date-time", description: "When the config was exported." })
  exportedAt!: string;
  @ApiProperty({ type: [ForwardRuleDto], description: "Exported forward rules." }) rules!: ForwardRule[];
}
