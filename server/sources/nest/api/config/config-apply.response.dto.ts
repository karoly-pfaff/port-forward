import type { ConfigApplyResponse } from "@portier/shared";
import { ConfigApplyResponseDto } from "../../common/api-schemas.js";

export { ConfigApplyResponseDto } from "../../common/api-schemas.js";

/**
 * Maps a `ConfigApplyResponse` to its response DTO at the HTTP boundary. The body
 * is deeply nested (the full plan: operations → current/desired snapshots + changes;
 * errors; warnings; plus the applied counts), so a structural deep clone gives a
 * fresh, non-aliasing copy that is byte-for-byte equal to the source (and preserves
 * property order). The DTO class is the OpenAPI schema (in `common/api-schemas.ts`);
 * this mapper is the covered logic, and it never mutates the source response.
 */
export function toConfigApplyResponseDto(response: ConfigApplyResponse): ConfigApplyResponseDto {
  return structuredClone(response);
}
