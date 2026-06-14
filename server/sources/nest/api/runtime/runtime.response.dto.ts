import type { RuntimeInfo } from "@portier/shared";

/**
 * Response DTO for `GET /api/runtime` — the `RuntimeInfo` object Express returns
 * (the shared REST-contract shape). The mapper is a structural copy at the HTTP
 * boundary (it would become an explicit field pick only to hide a future
 * internal-only field).
 */
export type RuntimeInfoResponseDto = RuntimeInfo;

/** Maps the runtime info to the response DTO (a fresh object). */
export function toRuntimeInfoResponseDto(info: RuntimeInfo): RuntimeInfoResponseDto {
  return { ...info };
}
