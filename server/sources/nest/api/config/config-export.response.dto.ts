import type { ExportedConfig } from "@portier/shared";

/**
 * Response DTO for `GET /api/config/export` — the `ExportedConfig` object Express
 * returns (the shared REST-contract shape). The mapper is a structural copy at
 * the HTTP boundary: a fresh object preserving the `version`/`exportedAt`/`rules`
 * field order, with each rule freshly copied (it would become an explicit field
 * pick only to hide a future internal-only field).
 */
export type ConfigExportResponseDto = ExportedConfig;

/** Maps the exported config to the response DTO (a fresh object + fresh rule array). */
export function toConfigExportResponseDto(config: ExportedConfig): ConfigExportResponseDto {
  return {
    version: config.version,
    exportedAt: config.exportedAt,
    rules: config.rules.map((rule) => ({ ...rule })),
  };
}
