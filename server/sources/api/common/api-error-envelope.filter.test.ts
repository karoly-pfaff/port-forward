import { BadRequestException, HttpException, NotFoundException, type ArgumentsHost } from "@nestjs/common";
import { describe, expect, it, vi } from "vitest";
import type { StaticFallback } from "../../static/static-serving.js";
import { ApiErrorEnvelopeFilter } from "./api-error-envelope.filter.js";

interface CapturedResponse {
  status: ReturnType<typeof vi.fn>;
  json: ReturnType<typeof vi.fn>;
  statusCode?: number;
  body?: unknown;
}

function fakeHost(path: string, method = "GET"): { host: ArgumentsHost; response: CapturedResponse } {
  const response: CapturedResponse = {
    status: vi.fn(function (this: CapturedResponse, code: number) {
      this.statusCode = code;
      return this;
    }),
    json: vi.fn(function (this: CapturedResponse, body: unknown) {
      this.body = body;
      return this;
    }),
  };
  const host = {
    switchToHttp: () => ({
      getRequest: () => ({ path, method }),
      getResponse: () => response,
    }),
  } as unknown as ArgumentsHost;
  return { host, response };
}

describe("ApiErrorEnvelopeFilter", () => {
  const filter = new ApiErrorEnvelopeFilter();

  it("emits the envelope for a deliberate /api 400", () => {
    const { host, response } = fakeHost("/api/ports/advisory");
    filter.catch(new BadRequestException({ errors: ["purpose must be management or forward."] }), host);
    expect(response.statusCode).toBe(400);
    expect(response.body).toEqual({ errors: ["purpose must be management or forward."] });
  });

  it("emits the fixed 404 envelope for an unmatched /api route", () => {
    const { host, response } = fakeHost("/api/nope");
    filter.catch(new NotFoundException("Cannot GET /api/nope"), host);
    expect(response.statusCode).toBe(404);
    expect(response.body).toEqual({ errors: ["API route was not found."] });
  });

  it("maps an unknown error on /api to a generic 500 envelope", () => {
    const { host, response } = fakeHost("/api/boom");
    filter.catch(new Error("kaboom"), host);
    expect(response.statusCode).toBe(500);
    expect(response.body).toEqual({ errors: ["Internal server error."] });
  });

  it("keeps NestJS's default object body for a non-API NotFound", () => {
    const { host, response } = fakeHost("/not-a-page");
    const exception = new NotFoundException("Cannot GET /not-a-page");
    filter.catch(exception, host);
    expect(response.statusCode).toBe(404);
    expect(response.body).toEqual(exception.getResponse());
    expect(response.body).not.toHaveProperty("errors");
  });

  it("wraps a string-bodied HttpException on a non-API route in NestJS's default shape", () => {
    const { host, response } = fakeHost("/plain");
    filter.catch(new HttpException("nope", 400), host);
    expect(response.statusCode).toBe(400);
    expect(response.body).toEqual({ statusCode: 400, message: "nope" });
  });

  it("maps an unknown error on a non-API route to NestJS's default 500", () => {
    const { host, response } = fakeHost("/plain");
    filter.catch(new Error("kaboom"), host);
    expect(response.statusCode).toBe(500);
    expect(response.body).toEqual({ statusCode: 500, message: "Internal server error" });
  });
});

describe("ApiErrorEnvelopeFilter — SPA static fallback", () => {
  it("delegates an unmatched non-API NotFound to the static fallback and stops when handled", () => {
    const handle = vi.fn().mockReturnValue(true);
    const filter = new ApiErrorEnvelopeFilter({ handle } as StaticFallback);
    const { host, response } = fakeHost("/dashboard");

    filter.catch(new NotFoundException("Cannot GET /dashboard"), host);

    expect(handle).toHaveBeenCalledTimes(1);
    // The fallback owns the response, so the filter does not also write a 404.
    expect(response.status).not.toHaveBeenCalled();
    expect(response.json).not.toHaveBeenCalled();
  });

  it("keeps the default 404 when the static fallback declines (e.g. disabled or non-GET)", () => {
    const handle = vi.fn().mockReturnValue(false);
    const filter = new ApiErrorEnvelopeFilter({ handle } as StaticFallback);
    const { host, response } = fakeHost("/dashboard");
    const exception = new NotFoundException("Cannot GET /dashboard");

    filter.catch(exception, host);

    expect(handle).toHaveBeenCalledTimes(1);
    expect(response.statusCode).toBe(404);
    expect(response.body).toEqual(exception.getResponse());
  });

  it("does not consult the static fallback for a non-NotFound non-API error", () => {
    const handle = vi.fn().mockReturnValue(true);
    const filter = new ApiErrorEnvelopeFilter({ handle } as StaticFallback);
    const { host, response } = fakeHost("/plain");

    filter.catch(new HttpException("nope", 400), host);

    expect(handle).not.toHaveBeenCalled();
    expect(response.statusCode).toBe(400);
    expect(response.body).toEqual({ statusCode: 400, message: "nope" });
  });

  it("never consults the static fallback for an /api route (envelope wins)", () => {
    const handle = vi.fn().mockReturnValue(true);
    const filter = new ApiErrorEnvelopeFilter({ handle } as StaticFallback);
    const { host, response } = fakeHost("/api/nope");

    filter.catch(new NotFoundException("Cannot GET /api/nope"), host);

    expect(handle).not.toHaveBeenCalled();
    expect(response.statusCode).toBe(404);
    expect(response.body).toEqual({ errors: ["API route was not found."] });
  });
});
