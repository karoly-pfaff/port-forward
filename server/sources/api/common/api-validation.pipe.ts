import type { PipeTransform } from "@nestjs/common";
import { plainToInstance } from "class-transformer";
import { validate, type ValidationError } from "class-validator";
import { ApiBadRequestException } from "./api-errors.js";

/**
 * Flattens class-validator errors into the contract's flat `string[]` (in
 * property order), hiding class-validator internals. With `stopAtFirstError`
 * each field contributes at most one message.
 */
export function flattenValidationErrors(errors: ValidationError[]): string[] {
  return errors.flatMap((error) => Object.values(error.constraints ?? {}));
}

/**
 * Validates a request DTO with class-transformer + class-validator and converts
 * failures into the shared API error envelope (`ApiBadRequestException` →
 * `400 { errors: [...] }`).
 *
 * The DTO class is passed EXPLICITLY (not inferred from `design:paramtypes`)
 * because the esbuild-based Vitest transform does not emit decorator metadata —
 * the same reason DI uses explicit `@Inject` tokens. This keeps validation
 * behaviour identical between `tsc` builds and tests, and confines the pipe to
 * the routes that opt in (no global pipe, so non-API routes are unaffected).
 *
 * `whitelist` strips unknown query keys without rejecting them (the route ignores
 * extra params); `stopAtFirstError` yields one message per field, matching the
 * the documented `/api` routes.
 */
export class ApiValidationPipe<T extends object> implements PipeTransform<unknown, Promise<T>> {
  constructor(private readonly dtoClass: new () => T) {}

  async transform(value: unknown): Promise<T> {
    const instance = plainToInstance(this.dtoClass, value ?? {});
    const errors = await validate(instance, { whitelist: true, stopAtFirstError: true });
    if (errors.length > 0) {
      throw new ApiBadRequestException(flattenValidationErrors(errors));
    }
    return instance;
  }
}
