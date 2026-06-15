import type { ForwardStatus } from "@portier/shared";
import { ForwardStatusDto } from "../../common/api-schemas.js";

export { ForwardStatusDto } from "../../common/api-schemas.js";

/**
 * Maps a single rule status (`ForwardStatus`) to the `ForwardStatusDto` shape at
 * the HTTP boundary — a fresh object (the DTO class is the OpenAPI schema, reused
 * from the status feature in `common/api-schemas.ts`; this mapper is the covered
 * logic). Used by the `POST /api/forwards/:id/start` 200 response. Byte-for-byte
 * equal to the Express start response.
 */
export function toForwardStatusResponseDto(status: ForwardStatus): ForwardStatusDto {
  return { ...status };
}
