import { ApiProperty } from "@nestjs/swagger";

/**
 * The Portier `/api` error envelope (`{ errors: [...] }`) — the single error
 * response shape shared by every endpoint, produced at runtime by `toApiError`.
 *
 * This is a metadata-only OpenAPI schema class (decorated `@ApiProperty`, never
 * instantiated). It is genuinely cross-feature, so it lives in `common/`; all
 * feature-owned schema classes live in their feature folder. Such `*.schema.ts`
 * files are coverage-excluded — they hold decorator metadata, not logic.
 */
export class ApiErrorResponseDto {
  @ApiProperty({
    type: [String],
    description: "One or more human-readable error messages.",
    example: ["port must be an integer from 1 to 65535."],
  })
  errors!: string[];
}
