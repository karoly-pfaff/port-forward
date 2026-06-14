import { NotFoundException, type ArgumentsHost } from "@nestjs/common";
import { describe, expect, it, vi } from "vitest";
import { ApiNotFoundFilter, isApiPath } from "./api-not-found.filter.js";

interface CapturedResponse {
  status: ReturnType<typeof vi.fn>;
  json: ReturnType<typeof vi.fn>;
  statusCode?: number;
  body?: unknown;
}

function fakeHost(path: string): { host: ArgumentsHost; response: CapturedResponse } {
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
      getRequest: () => ({ path }),
      getResponse: () => response,
    }),
  } as unknown as ArgumentsHost;

  return { host, response };
}

describe("isApiPath", () => {
  it.each([
    ["/api", true],
    ["/api/", true],
    ["/api/forwards", true],
    ["/api/forwards/abc/start", true],
    ["/", false],
    ["/health", false],
    ["/apidocs", false],
    ["/apiary", false],
  ])("classifies %s as %s", (path, expected) => {
    expect(isApiPath(path)).toBe(expected);
  });
});

describe("ApiNotFoundFilter", () => {
  it("returns the contract 404 envelope for an unknown /api route", () => {
    const filter = new ApiNotFoundFilter();
    const { host, response } = fakeHost("/api/does-not-exist");

    filter.catch(new NotFoundException("Cannot GET /api/does-not-exist"), host);

    expect(response.statusCode).toBe(404);
    expect(response.body).toEqual({ errors: ["API route was not found."] });
  });

  it("handles the bare /api namespace", () => {
    const filter = new ApiNotFoundFilter();
    const { host, response } = fakeHost("/api");

    filter.catch(new NotFoundException(), host);

    expect(response.statusCode).toBe(404);
    expect(response.body).toEqual({ errors: ["API route was not found."] });
  });

  it("preserves the default NestJS 404 body (object form) for non-API routes", () => {
    const filter = new ApiNotFoundFilter();
    const { host, response } = fakeHost("/somewhere-else");
    const exception = new NotFoundException("Cannot GET /somewhere-else");

    filter.catch(exception, host);

    expect(response.statusCode).toBe(404);
    expect(response.body).toEqual(exception.getResponse());
  });

  it("wraps a string exception response in a message object for non-API routes", () => {
    const filter = new ApiNotFoundFilter();
    const { host, response } = fakeHost("/plain");
    // A NotFoundException constructed with no message still yields an object
    // response, so force the string branch explicitly.
    const exception = new NotFoundException();
    vi.spyOn(exception, "getResponse").mockReturnValue("Not Found");

    filter.catch(exception, host);

    expect(response.statusCode).toBe(404);
    expect(response.body).toEqual({ message: "Not Found" });
  });
});
