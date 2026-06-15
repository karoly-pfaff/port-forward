import type { ForwardStatus } from "@portier/shared";
import { ForwardStatusDto } from "./forward-status.schema.js";

export { ForwardStatusDto } from "./forward-status.schema.js";

/**
 * Maps a single rule status (`ForwardStatus`) to the `ForwardStatusDto` shape at
 * the HTTP boundary — a fresh object (the DTO class is the OpenAPI schema, reused
 * in `forward-status.schema.ts`; this mapper is the covered
 * logic). Used by the `POST /api/forwards/:id/start` 200 response. Byte-for-byte
 * equal to the Express start response.
 */
export function toForwardStatusResponseDto(status: ForwardStatus): ForwardStatusDto {
  return { ...status };
}
