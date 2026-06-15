import type { GroupActionResponse } from "@portier/shared";
import { GroupActionResponseDto } from "../../common/api-schemas.js";

export { GroupActionResponseDto } from "../../common/api-schemas.js";

/**
 * Maps a `GroupActionResponse` to the `GroupActionResponseDto` shape at the HTTP
 * boundary — a fresh object with a fresh `results` array of fresh result objects,
 * so the controller never returns the domain summary and the mapper cannot mutate
 * it. The DTO class is the OpenAPI schema (in `common/api-schemas.ts`); this mapper
 * is the covered logic. Byte-for-byte equal to the Express group-action response.
 */
export function toGroupActionResponseDto(response: GroupActionResponse): GroupActionResponseDto {
  return {
    group: response.group,
    action: response.action,
    total: response.total,
    succeeded: response.succeeded,
    skipped: response.skipped,
    failed: response.failed,
    results: response.results.map((result) => ({ ...result })),
  };
}
