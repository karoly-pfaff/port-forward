import { Transform } from "class-transformer";
import { IsIn, IsInt, IsOptional, Max, Min } from "class-validator";

/**
 * Query DTO for `GET /api/ports/advisory`, validated by `ApiValidationPipe`.
 *
 * Coercion is intentionally Express-faithful so the migrated route stays
 * byte-for-byte parity-identical:
 * - `port` is coerced with `Number(...)` (NaN for missing/non-numeric) and must
 *   be an integer in `[1, 65535]`. All three constraints share the exact Express
 *   message so whichever fails, the surfaced error is identical.
 * - `purpose` must be exactly `management` or `forward`.
 * - `listenHost` is an optional string; a non-string (e.g. a repeated query key
 *   parsed as an array) is coerced to `undefined` and never errors, matching the
 *   Express `typeof value === "string" ? value : undefined` rule.
 */
const PORT_MESSAGE = "port must be an integer from 1 to 65535.";

export class PortsAdvisoryQueryDto {
  @Transform(({ value }) => Number(value))
  @IsInt({ message: PORT_MESSAGE })
  @Min(1, { message: PORT_MESSAGE })
  @Max(65535, { message: PORT_MESSAGE })
  port!: number;

  @IsIn(["management", "forward"], { message: "purpose must be management or forward." })
  purpose!: "management" | "forward";

  @IsOptional()
  @Transform(({ value }) => (typeof value === "string" ? value : undefined))
  listenHost?: string;
}
