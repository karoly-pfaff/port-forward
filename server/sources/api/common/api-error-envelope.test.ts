import { BadRequestException, HttpException, NotFoundException } from "@nestjs/common";
import { describe, expect, it } from "vitest";
import { isApiPath, toApiError } from "./api-error-envelope.js";
import { ApiBadRequestException } from "./api-errors.js";

describe("isApiPath", () => {
  it.each([
    ["/api", true],
    ["/api/", true],
    ["/api/ports/advisory", true],
    ["/", false],
    ["/health", false],
    ["/apidocs", false],
    ["/apiary", false],
  ])("classifies %s as %s", (path, expected) => {
    expect(isApiPath(path)).toBe(expected);
  });
});

describe("toApiError", () => {
  it("keeps the explicit envelope from an ApiBadRequestException", () => {
    expect(toApiError(new ApiBadRequestException(["port must be an integer from 1 to 65535."]))).toEqual({
      status: 400,
      errors: ["port must be an integer from 1 to 65535."],
    });
  });

  it("keeps an explicit { errors } envelope from any HttpException (e.g. a deliberate 404)", () => {
    expect(toApiError(new NotFoundException({ errors: ["Forward rule x was not found."] }))).toEqual({
      status: 404,
      errors: ["Forward rule x was not found."],
    });
  });

  it("maps an unmatched-route NotFoundException to the fixed API message", () => {
    expect(toApiError(new NotFoundException("Cannot GET /api/x"))).toEqual({
      status: 404,
      errors: ["API route was not found."],
    });
  });

  it("wraps a BadRequestException string message", () => {
    expect(toApiError(new BadRequestException("bad input"))).toEqual({ status: 400, errors: ["bad input"] });
  });

  it("wraps a default BadRequestException message", () => {
    expect(toApiError(new BadRequestException())).toEqual({ status: 400, errors: ["Bad Request"] });
  });

  it("wraps a string-bodied HttpException with its own status", () => {
    expect(toApiError(new HttpException("teapot", 418))).toEqual({ status: 418, errors: ["teapot"] });
  });

  it("falls back when an HttpException body has neither errors nor a string message", () => {
    expect(toApiError(new BadRequestException({ foo: "bar" }))).toEqual({
      status: 400,
      errors: ["Request could not be processed."],
    });
  });

  it("falls back when an errors body is not an array of strings", () => {
    expect(toApiError(new BadRequestException({ errors: [1, 2] }))).toEqual({
      status: 400,
      errors: ["Request could not be processed."],
    });
  });

  it.each([new Error("boom"), "raw thrown string", undefined])(
    "maps a non-HTTP value to a generic 500 without leaking it (%#)",
    (value) => {
      expect(toApiError(value)).toEqual({ status: 500, errors: ["Internal server error."] });
    }
  );
});
