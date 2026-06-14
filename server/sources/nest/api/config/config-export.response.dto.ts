import type { ExportedConfig } from "@portier/shared";
import { ConfigExportResponseDto } from "../../common/api-schemas.js";

export { ConfigExportResponseDto } from "../../common/api-schemas.js";

/**
 * Maps the domain exported config to the `ConfigExportResponseDto` shape at the
 * HTTP boundary — a fresh object + fresh rule array (the DTO class is the OpenAPI
 * schema, defined in `common/api-schemas.ts`; this mapper is the covered logic).
 */
export function toConfigExportResponseDto(config: ExportedConfig): ConfigExportResponseDto {
  return {
    version: config.version,
    exportedAt: config.exportedAt,
    rules: config.rules.map((rule) => ({ ...rule })),
  };
}
