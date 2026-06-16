import type { ForwardRuleResponse } from "@portier/shared";
import { ForwardRuleResponseDto } from "./forward-rule.schema.js";

export { ForwardRuleResponseDto } from "./forward-rule.schema.js";

/**
 * Maps a single decorated rule (`ForwardRuleResponse`) to the `ForwardRuleResponseDto`
 * shape at the HTTP boundary — a fresh object (the DTO class is the OpenAPI
 * schema, defined in `forward-rule.schema.ts`; this mapper is the covered logic).
 * Used by the `POST /api/forwards` 201 response. Byte-for-byte equal to the
 * documented create response shape.
 */
export function toForwardRuleResponseDto(response: ForwardRuleResponse): ForwardRuleResponseDto {
  return { ...response };
}
