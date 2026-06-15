import type { ExportedConfig } from "@portier/shared";
import { ConfigExportResponseDto } from "./config-export.schema.js";

export { ConfigExportResponseDto } from "./config-export.schema.js";

/**
 * Maps the domain exported config to the `ConfigExportResponseDto` shape at the
 * HTTP boundary — a fresh object + fresh rule array (the DTO class is the OpenAPI
 * schema, defined in `config-export.schema.ts`; this mapper is the covered logic).
 */
export function toConfigExportResponseDto(config: ExportedConfig): ConfigExportResponseDto {
  return {
    version: config.version,
    exportedAt: config.exportedAt,
    rules: config.rules.map((rule) => ({ ...rule })),
  };
}
