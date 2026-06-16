import type { ForwardStatus } from "@portier/shared";

/**
 * Response DTO for `GET /api/status` — the `ForwardStatus[]` array the contract
 * returns (the shared REST-contract shape). The mapper is a structural copy at
 * the HTTP boundary.
 */
export type StatusListResponseDto = ForwardStatus[];

/** Maps the status list to the response DTO (a fresh array of fresh objects). */
export function toStatusListResponseDto(statuses: ForwardStatus[]): StatusListResponseDto {
  return statuses.map((status) => ({ ...status }));
}
