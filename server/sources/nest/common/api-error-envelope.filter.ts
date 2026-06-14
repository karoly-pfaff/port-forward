import { ArgumentsHost, Catch, ExceptionFilter, HttpException } from "@nestjs/common";
import type { Request, Response } from "express";
import { isApiPath, toApiError } from "./api-error-envelope.js";

/**
 * Global exception filter that gives every `/api/*` route the Portier error
 * envelope (`{ errors: [...] }`) via `toApiError`, so migrated controllers do not
 * duplicate envelope handling. Non-API routes keep NestJS's default error shape.
 *
 * It is a catch-all (`@Catch()`), so it also maps unknown/internal errors on
 * `/api/*` to `500 { errors: ["Internal server error."] }` without leaking the
 * exception. NestJS only raises `NotFoundException` for unmatched routes, so a
 * future migrated `/api` controller still responds normally — only genuinely
 * unhandled exceptions reach here.
 */
@Catch()
export class ApiErrorEnvelopeFilter implements ExceptionFilter {
  catch(exception: unknown, host: ArgumentsHost): void {
    const ctx = host.switchToHttp();
    const request = ctx.getRequest<Request>();
    const response = ctx.getResponse<Response>();

    if (isApiPath(request.path)) {
      const { status, errors } = toApiError(exception);
      response.status(status).json({ errors });
      return;
    }

    // Non-API routes keep NestJS's default error shape.
    if (exception instanceof HttpException) {
      const status = exception.getStatus();
      const body = exception.getResponse();
      response.status(status).json(typeof body === "string" ? { statusCode: status, message: body } : body);
      return;
    }
    response.status(500).json({ statusCode: 500, message: "Internal server error" });
  }
}
