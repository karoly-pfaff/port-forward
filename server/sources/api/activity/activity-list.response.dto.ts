import type { ActivityEvent } from "@portier/shared";
import { ActivityListResponseDto } from "./activity.schema.js";

export { ActivityListResponseDto } from "./activity.schema.js";

/**
 * Wraps the event list in the `{ events: [...] }` response envelope — a fresh
 * object + fresh elements (the DTO class is the OpenAPI schema in
 * `activity.schema.ts`; this mapper is the covered logic).
 */
export function toActivityListResponseDto(events: ActivityEvent[]): ActivityListResponseDto {
  return { events: events.map((event) => ({ ...event })) };
}
