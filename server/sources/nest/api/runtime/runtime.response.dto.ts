import type { RuntimeInfo } from "@portier/shared";
import { RuntimeInfoResponseDto } from "../../common/api-schemas.js";

export { RuntimeInfoResponseDto } from "../../common/api-schemas.js";

/**
 * Maps the domain runtime info to the `RuntimeInfoResponseDto` shape at the HTTP
 * boundary — a fresh object (the DTO class is the OpenAPI schema, defined in
 * `common/api-schemas.ts`; this mapper is the covered logic). Byte-for-byte equal
 * to the Express route output.
 */
export function toRuntimeInfoResponseDto(info: RuntimeInfo): RuntimeInfoResponseDto {
  return { ...info };
}
