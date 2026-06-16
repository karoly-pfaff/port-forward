import type { ValidationError } from "class-validator";
import { describe, expect, it } from "vitest";
import { PortsAdvisoryQueryDto } from "../ports/ports-advisory.query.dto.js";
import { ApiBadRequestException } from "./api-errors.js";
import { ApiValidationPipe, flattenValidationErrors } from "./api-validation.pipe.js";

describe("flattenValidationErrors", () => {
  it("flattens constraint messages in order and tolerates missing constraints", () => {
    const errors = [
      { property: "port", constraints: { isInt: "port must be an integer from 1 to 65535." } },
      { property: "nested", constraints: undefined },
      { property: "purpose", constraints: { isIn: "purpose must be management or forward." } },
    ] as unknown as ValidationError[];

    expect(flattenValidationErrors(errors)).toEqual([
      "port must be an integer from 1 to 65535.",
      "purpose must be management or forward.",
    ]);
  });
});

describe("ApiValidationPipe (with PortsAdvisoryQueryDto)", () => {
  const pipe = new ApiValidationPipe(PortsAdvisoryQueryDto);

  it("coerces and returns a valid DTO instance (listenHost present)", async () => {
    const dto = await pipe.transform({ port: "48001", purpose: "forward", listenHost: "0.0.0.0" });

    expect(dto).toMatchObject({ port: 48001, purpose: "forward", listenHost: "0.0.0.0" });
    expect(dto).toBeInstanceOf(PortsAdvisoryQueryDto);
  });

  it("coerces a valid DTO with no listenHost (optional → undefined)", async () => {
    const dto = await pipe.transform({ port: "47831", purpose: "management" });

    expect(dto).toMatchObject({ port: 47831, purpose: "management" });
    expect(dto.listenHost).toBeUndefined();
  });

  it("coerces a non-string listenHost to undefined (coerced, no error)", async () => {
    const dto = await pipe.transform({ port: "48001", purpose: "forward", listenHost: ["a", "b"] });

    expect(dto.listenHost).toBeUndefined();
  });

  it.each([
    ["70000", "forward", "port must be an integer from 1 to 65535."],
    ["1.5", "forward", "port must be an integer from 1 to 65535."],
    ["abc", "forward", "port must be an integer from 1 to 65535."],
    ["0", "forward", "port must be an integer from 1 to 65535."],
    ["48001", "bogus", "purpose must be management or forward."],
  ])("rejects port=%j purpose=%j with a single contract message", async (port, purpose, message) => {
    await expect(pipe.transform({ port, purpose })).rejects.toMatchObject({});
    try {
      await pipe.transform({ port, purpose });
      expect.unreachable("expected an ApiBadRequestException");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiBadRequestException);
      expect((error as ApiBadRequestException).getResponse()).toEqual({ errors: [message] });
    }
  });

  it("rejects a missing query entirely — both required fields fail, in property order", async () => {
    // `transform(undefined)` exercises the `value ?? {}` fallback. With both port
    // and purpose missing, errors accumulate (idiomatic), in DTO property order.
    try {
      await pipe.transform(undefined);
      expect.unreachable("expected an ApiBadRequestException");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiBadRequestException);
      expect((error as ApiBadRequestException).getResponse()).toEqual({
        errors: ["port must be an integer from 1 to 65535.", "purpose must be management or forward."],
      });
    }
  });
});
