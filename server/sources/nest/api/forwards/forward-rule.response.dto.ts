import type { ForwardRuleResponse } from "@portier/shared";
import { ForwardRuleResponseDto } from "../../common/api-schemas.js";

export { ForwardRuleResponseDto } from "../../common/api-schemas.js";

/**
 * Maps a single decorated rule (`ForwardRuleResponse`) to the `ForwardRuleResponseDto`
 * shape at the HTTP boundary — a fresh object (the DTO class is the OpenAPI
 * schema, defined in `common/api-schemas.ts`; this mapper is the covered logic).
 * Used by the `POST /api/forwards` 201 response. Byte-for-byte equal to the
 * Express create response.
 */
export function toForwardRuleResponseDto(response: ForwardRuleResponse): ForwardRuleResponseDto {
  return { ...response };
}
