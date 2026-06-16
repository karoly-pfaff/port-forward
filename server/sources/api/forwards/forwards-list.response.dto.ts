import type { ForwardRuleResponse } from "@portier/shared";

/**
 * Response DTO for `GET /api/forwards` — the `ForwardRuleResponse[]` array
 * Express returns (each rule plus its port advisories — the shared REST-contract
 * shape). The mapper is a structural copy at the HTTP boundary.
 */
export type ForwardsListResponseDto = ForwardRuleResponse[];

/** Maps the rule-response list to the response DTO (a fresh array of fresh objects). */
export function toForwardsListResponseDto(responses: ForwardRuleResponse[]): ForwardsListResponseDto {
  return responses.map((response) => ({ ...response }));
}
