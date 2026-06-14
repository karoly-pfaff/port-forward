import { BadRequestException } from "@nestjs/common";

/**
 * A `400` carrying Portier API envelope messages. Controllers throw this with a
 * plain `string[]`; the `{ errors: [...] }` envelope shape is owned by the shared
 * error layer (`ApiErrorEnvelopeFilter` / `toApiError`), so controllers never
 * hand-roll it.
 */
export class ApiBadRequestException extends BadRequestException {
  constructor(errors: string[]) {
    super({ errors });
  }
}
