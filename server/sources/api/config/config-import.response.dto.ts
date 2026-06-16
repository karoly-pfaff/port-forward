import { ConfigImportErrorResponseDto, ConfigImportResponseDto } from "./config-import.schema.js";
import type { ConfigImportErrorBody, ConfigImportSuccessBody } from "./config-import.service.js";

export { ConfigImportErrorResponseDto, ConfigImportResponseDto } from "./config-import.schema.js";

/**
 * Maps the import success body (`200 {result, rules}`) to its response DTO at the
 * HTTP boundary — a structural deep clone giving a fresh, non-aliasing, byte-for-
 * byte-equal copy (the `rules` are already advisory-decorated by the service).
 */
export function toConfigImportResponseDto(body: ConfigImportSuccessBody): ConfigImportResponseDto {
  return structuredClone(body);
}

/**
 * Maps the import error body (`422 {errors, result}`) to its response DTO at the
 * HTTP boundary — a structural deep clone (this `422` body carries `result`
 * ALONGSIDE `errors`, so it is NOT the plain `{errors}` error envelope and is
 * returned directly rather than thrown through the shared error filter).
 */
export function toConfigImportErrorResponseDto(body: ConfigImportErrorBody): ConfigImportErrorResponseDto {
  return structuredClone(body);
}
