import { BadRequestException, ConflictException } from "@nestjs/common";

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

/**
 * A `409` carrying Portier API envelope messages — the Nest equivalent of the
 * Express manager `ConflictError` (e.g. a duplicate listen binding on rule
 * create). Same `{ errors: [...] }` envelope, status `409`.
 */
export class ApiConflictException extends ConflictException {
  constructor(errors: string[]) {
    super({ errors });
  }
}
