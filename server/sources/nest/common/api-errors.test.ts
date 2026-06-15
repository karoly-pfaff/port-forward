import { BadRequestException, ConflictException, NotFoundException } from "@nestjs/common";
import { describe, expect, it } from "vitest";
import { ApiBadRequestException, ApiConflictException, ApiNotFoundException } from "./api-errors.js";

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

describe("ApiNotFoundException", () => {
  it("is a 404 carrying the errors as the envelope body", () => {
    const exception = new ApiNotFoundException(["Forward rule abc was not found."]);

    expect(exception).toBeInstanceOf(NotFoundException);
    expect(exception.getStatus()).toBe(404);
    expect(exception.getResponse()).toEqual({ errors: ["Forward rule abc was not found."] });
  });
});
