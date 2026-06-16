import type { RuntimeInfo } from "@portier/shared";
import { RuntimeInfoResponseDto } from "./runtime.schema.js";

export { RuntimeInfoResponseDto } from "./runtime.schema.js";

/**
 * Maps the domain runtime info to the `RuntimeInfoResponseDto` shape at the HTTP
 * boundary — a fresh object (the DTO class is the OpenAPI schema, defined in
 * `runtime.schema.ts`; this mapper is the covered logic). Byte-for-byte equal
 * to the documented runtime response shape.
 */
export function toRuntimeInfoResponseDto(info: RuntimeInfo): RuntimeInfoResponseDto {
  return { ...info };
}
