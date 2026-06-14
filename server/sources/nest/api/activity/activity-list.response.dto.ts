import type { ActivityEvent } from "@portier/shared";

/**
 * Response DTO for `GET /api/activity` — the `{ events: [...] }` object Express
 * returns (the shared REST-contract shape). The mapper is a structural copy at
 * the HTTP boundary.
 */
export interface ActivityListResponseDto {
  events: ActivityEvent[];
}

/** Wraps the event list in the response envelope (a fresh object + fresh elements). */
export function toActivityListResponseDto(events: ActivityEvent[]): ActivityListResponseDto {
  return { events: events.map((event) => ({ ...event })) };
}
