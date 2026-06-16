import { ArgumentsHost, Catch, ExceptionFilter, HttpException, Inject, NotFoundException } from "@nestjs/common";
import type { Request, Response } from "express";
import { isApiPath, toApiError } from "./api-error-envelope.js";
import { STATIC_FALLBACK, disabledStaticFallback, type StaticFallback } from "../static/static-serving.js";

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
 *
 * Because Nest surfaces an unmatched route as `NotFoundException`, this filter is
 * also the single place that decides what an unmatched route returns — so it owns
 * the **non-API SPA fallback**: when static serving is enabled, an unmatched non-API
 * GET/HEAD route serves the SPA `index.html` (mirroring the Express catch-all index
 * route). The static decision is delegated to the injected `StaticFallback`
 * (`disabledStaticFallback` by default — no static dir, so the
 * default 404 behavior is unchanged). The `/api/*` branch runs first and is never
 * affected by static serving.
 */
@Catch()
export class ApiErrorEnvelopeFilter implements ExceptionFilter {
  constructor(
    @Inject(STATIC_FALLBACK) private readonly staticFallback: StaticFallback = disabledStaticFallback
  ) {}

  catch(exception: unknown, host: ArgumentsHost): void {
    const ctx = host.switchToHttp();
    const request = ctx.getRequest<Request>();
    const response = ctx.getResponse<Response>();

    if (isApiPath(request.path)) {
      const { status, errors } = toApiError(exception);
      response.status(status).json({ errors });
      return;
    }

    // Non-API SPA fallback: an unmatched non-API GET/HEAD route serves the SPA
    // index when static serving is enabled (mirrors Express's catch-all index route).
    if (exception instanceof NotFoundException && this.staticFallback.handle(request, response)) {
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
