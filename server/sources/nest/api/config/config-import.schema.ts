import { ApiProperty } from "@nestjs/swagger";
import type { ExportedConfig, ForwardRuleResponse, ImportMode, ImportResult } from "@portier/shared";
import { ForwardRuleResponseDto } from "../forwards/forward-rule.schema.js";
import { ConfigExportResponseDto } from "./config-export.schema.js";

/**
 * OpenAPI schemas for `POST /api/config/import` — the import outcome counts, the
 * request body, and the `200`/`422` responses. Metadata-only decorated classes.
 */

/** The outcome counts of a config import. */
export class ImportResultDto implements ImportResult {
  @ApiProperty({ type: Number, description: "Number of rules imported." }) imported!: number;
  @ApiProperty({ type: Number, description: "Number of rules skipped (merge conflicts)." }) skipped!: number;
  @ApiProperty({ type: [String], description: "Import error messages (empty on success)." }) errors!: string[];
}

/** Request body for `POST /api/config/import` (documentation/typing only — validation is the inline mode/config checks). */
export class ConfigImportBodyDto {
  @ApiProperty({ enum: ["replace", "merge"], description: "Import mode: replace all rules, or merge non-conflicting rules." })
  mode!: ImportMode;
  @ApiProperty({ type: ConfigExportResponseDto, description: "The Portier config to import (version 1 + a rules array)." })
  config!: ExportedConfig;
}

/** Response body for a successful `POST /api/config/import` (`200`). */
export class ConfigImportResponseDto {
  @ApiProperty({ type: ImportResultDto, description: "The import outcome counts." }) result!: ImportResult;
  @ApiProperty({ type: [ForwardRuleResponseDto], description: "The full rule list after import (with advisories)." })
  rules!: ForwardRuleResponse[];
}

/** Response body for a `POST /api/config/import` that reports import errors (`422`). Carries `result` ALONGSIDE `errors`. */
export class ConfigImportErrorResponseDto {
  @ApiProperty({ type: [String], description: "Import error messages." }) errors!: string[];
  @ApiProperty({ type: ImportResultDto, description: "The import outcome (zero imported on error)." }) result!: ImportResult;
}
