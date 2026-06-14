import { HttpException, NotFoundException } from "@nestjs/common";

/**
 * Shared `/api` error-envelope mapping for the NestJS migration.
 *
 * Portier's REST contract reports errors as `{ "errors": ["..."] }`. This module
 * is the single, pure place that converts a thrown exception into that envelope
 * (status + messages) so migrated controllers never hand-roll it. The
 * `ApiErrorEnvelopeFilter` applies this to `/api/*` routes.
 */

export interface ApiError {
  status: number;
  errors: string[];
}

/** True for the API namespace exactly (`/api`) or any sub-path (`/api/...`). */
export function isApiPath(path: string): boolean {
  return path === "/api" || path.startsWith("/api/");
}

/**
 * Maps any thrown value to the Portier API error envelope, deterministically:
 *
 * - An `HttpException` whose body is `{ errors: string[] }` (a controller that
 *   deliberately supplied envelope messages, e.g. `ApiBadRequestException`) keeps
 *   those messages.
 * - A `NotFoundException` without an explicit envelope → `["API route was not
 *   found."]` (unmatched `/api/*` route; preserves the prior behavior).
 * - Any other `HttpException` → its message text wrapped as a single error.
 * - Anything else (a non-HTTP/unknown error) → `500 ["Internal server error."]`,
 *   never leaking the exception or a stack trace.
 */
export function toApiError(exception: unknown): ApiError {
  if (exception instanceof HttpException) {
    const response = exception.getResponse();
    const envelope = errorsFromResponse(response);
    if (envelope) {
      return { status: exception.getStatus(), errors: envelope };
    }
    if (exception instanceof NotFoundException) {
      return { status: 404, errors: ["API route was not found."] };
    }
    return { status: exception.getStatus(), errors: messagesFromResponse(response) };
  }
  return { status: 500, errors: ["Internal server error."] };
}

/** Returns the `errors` array when the exception body already carries the envelope. */
function errorsFromResponse(response: string | object): string[] | undefined {
  if (typeof response === "string") {
    return undefined;
  }
  const errors = (response as { errors?: unknown }).errors;
  if (Array.isArray(errors) && errors.every((entry) => typeof entry === "string")) {
    return errors as string[];
  }
  return undefined;
}

/** Extracts a human message from an HttpException body (string body, `message`, or a generic fallback). */
function messagesFromResponse(response: string | object): string[] {
  if (typeof response === "string") {
    return [response];
  }
  const message = (response as { message?: unknown }).message;
  return typeof message === "string" ? [message] : ["Request could not be processed."];
}
