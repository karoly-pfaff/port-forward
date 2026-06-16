import type { ConfigPlanResponse } from "@portier/shared";
import { ConfigPlanResponseDto } from "./config-plan.schema.js";

export { ConfigPlanResponseDto } from "./config-plan.schema.js";

/**
 * Maps a `ConfigPlanResponse` to the `ConfigPlanResponseDto` shape at the HTTP
 * boundary. The plan is deeply nested (operations → current/desired snapshots +
 * changes; errors; warnings), so a structural deep clone gives a fresh object with
 * no shared references that is byte-for-byte equal to the source (and preserves
 * property order). The DTO class is the OpenAPI schema (in `config-plan.schema.ts`);
 * this mapper is the covered logic, and it never mutates the source plan.
 */
export function toConfigPlanResponseDto(plan: ConfigPlanResponse): ConfigPlanResponseDto {
  return structuredClone(plan);
}
