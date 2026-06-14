import { ArgumentsHost, Catch, ExceptionFilter, NotFoundException } from "@nestjs/common";
import type { Request, Response } from "express";

/** True for the API namespace exactly (`/api`) or any sub-path (`/api/...`). */
export function isApiPath(path: string): boolean {
  return path === "/api" || path.startsWith("/api/");
}

/**
 * Reshapes unmatched `/api/*` routes to the frozen contract envelope
 * `{ "errors": ["API route was not found."] }` (matching the existing TypeScript
 * server and the Go service). Non-API 404s keep NestJS's default body.
 *
 * Because NestJS only raises `NotFoundException` when no route matches, this
 * filter never intercepts a real handler — future migrated `/api` controllers
 * respond normally and only genuinely-unknown API routes reach here.
 */
@Catch(NotFoundException)
export class ApiNotFoundFilter implements ExceptionFilter {
  catch(exception: NotFoundException, host: ArgumentsHost): void {
    const ctx = host.switchToHttp();
    const request = ctx.getRequest<Request>();
    const response = ctx.getResponse<Response>();

    if (isApiPath(request.path)) {
      response.status(404).json({ errors: ["API route was not found."] });
      return;
    }

    const body = exception.getResponse();
    response.status(exception.getStatus()).json(typeof body === "string" ? { message: body } : body);
  }
}
