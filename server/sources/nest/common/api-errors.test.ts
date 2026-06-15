import { BadRequestException, ConflictException } from "@nestjs/common";
import { describe, expect, it } from "vitest";
import { ApiBadRequestException, ApiConflictException } from "./api-errors.js";

describe("ApiBadRequestException", () => {
  it("is a 400 carrying the errors as the envelope body", () => {
    const exception = new ApiBadRequestException(["a", "b"]);

    expect(exception).toBeInstanceOf(BadRequestException);
    expect(exception.getStatus()).toBe(400);
    expect(exception.getResponse()).toEqual({ errors: ["a", "b"] });
  });
});

describe("ApiConflictException", () => {
  it("is a 409 carrying the errors as the envelope body", () => {
    const exception = new ApiConflictException(["already listening"]);

    expect(exception).toBeInstanceOf(ConflictException);
    expect(exception.getStatus()).toBe(409);
    expect(exception.getResponse()).toEqual({ errors: ["already listening"] });
  });
});
