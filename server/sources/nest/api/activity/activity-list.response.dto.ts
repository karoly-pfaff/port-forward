import type { ActivityEvent } from "@portier/shared";
import { ActivityListResponseDto } from "../../common/api-schemas.js";

export { ActivityListResponseDto } from "../../common/api-schemas.js";

/**
 * Wraps the event list in the `{ events: [...] }` response envelope — a fresh
 * object + fresh elements (the DTO class is the OpenAPI schema, defined in
 * `common/api-schemas.ts`; this mapper is the covered logic).
 */
export function toActivityListResponseDto(events: ActivityEvent[]): ActivityListResponseDto {
  return { events: events.map((event) => ({ ...event })) };
}
