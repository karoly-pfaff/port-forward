import { ConflictError, NotFoundError, ValidationError } from "../forward-manager.js";
import { ApiBadRequestException, ApiConflictException, ApiNotFoundException } from "./api-errors.js";

/**
 * Translates a domain `ForwardManager` error into the matching Nest API
 * exception so the shared error-envelope layer produces the SAME status + body
 * the Express error middleware does:
 *
 * - `ValidationError` → `400 { errors }` (the validator's message list).
 * - `ConflictError`   → `409 { errors: [message] }` (e.g. duplicate binding).
 * - `NotFoundError`   → `404 { errors: [message] }` (e.g. unknown rule id).
 * - anything else (e.g. a persist failure) is re-thrown unchanged → the filter
 *   maps it to `500` (generic, no leak — the established generic-500 behavior; the
 *   only documented divergence from Express's 500, which echoes the raw message).
 *
 * Always throws (return type `never`). The manager is domain logic (reusable,
 * like `ActivityStore`), not the Express app factory — importing its error
 * classes is allowed.
 */
export function mapManagerError(error: unknown): never {
  if (error instanceof ValidationError) {
    throw new ApiBadRequestException(error.errors);
  }
  if (error instanceof ConflictError) {
    throw new ApiConflictException([error.message]);
  }
  if (error instanceof NotFoundError) {
    throw new ApiNotFoundException([error.message]);
  }
  throw error;
}
