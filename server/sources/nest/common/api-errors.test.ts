import { BadRequestException } from "@nestjs/common";
import { describe, expect, it } from "vitest";
import { ApiBadRequestException } from "./api-errors.js";

describe("ApiBadRequestException", () => {
  it("is a 400 carrying the errors as the envelope body", () => {
    const exception = new ApiBadRequestException(["a", "b"]);

    expect(exception).toBeInstanceOf(BadRequestException);
    expect(exception.getStatus()).toBe(400);
    expect(exception.getResponse()).toEqual({ errors: ["a", "b"] });
  });
});
