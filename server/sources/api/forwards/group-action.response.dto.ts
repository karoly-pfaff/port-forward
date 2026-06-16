import type { GroupActionResponse } from "@portier/shared";
import { GroupActionResponseDto } from "./group-action.schema.js";

export { GroupActionResponseDto } from "./group-action.schema.js";

/**
 * Maps a `GroupActionResponse` to the `GroupActionResponseDto` shape at the HTTP
 * boundary — a fresh object with a fresh `results` array of fresh result objects,
 * so the controller never returns the domain summary and the mapper cannot mutate
 * it. The DTO class is the OpenAPI schema (in `group-action.schema.ts`); this mapper
 * is the covered logic. Matches the documented group-action response shape.
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
