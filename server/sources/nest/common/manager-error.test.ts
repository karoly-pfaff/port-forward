import { describe, expect, it } from "vitest";
import { ConflictError, NotFoundError, ValidationError } from "../../forward-manager.js";
import { ApiBadRequestException, ApiConflictException, ApiNotFoundException } from "./api-errors.js";
import { mapManagerError } from "./manager-error.js";

describe("mapManagerError", () => {
  it("translates a ValidationError to a 400 ApiBadRequestException carrying its errors", () => {
    try {
      mapManagerError(new ValidationError(["name is required.", "port is invalid."]));
      expect.unreachable("should have thrown");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiBadRequestException);
      expect((error as ApiBadRequestException).getStatus()).toBe(400);
      expect((error as ApiBadRequestException).getResponse()).toEqual({
        errors: ["name is required.", "port is invalid."],
      });
    }
  });

  it("translates a ConflictError to a 409 ApiConflictException carrying its message", () => {
    try {
      mapManagerError(new ConflictError("A TCP rule is already listening on 127.0.0.1:48010."));
      expect.unreachable("should have thrown");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiConflictException);
      expect((error as ApiConflictException).getStatus()).toBe(409);
      expect((error as ApiConflictException).getResponse()).toEqual({
        errors: ["A TCP rule is already listening on 127.0.0.1:48010."],
      });
    }
  });

  it("translates a NotFoundError to a 404 ApiNotFoundException carrying its message", () => {
    try {
      mapManagerError(new NotFoundError("Forward rule abc was not found."));
      expect.unreachable("should have thrown");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiNotFoundException);
      expect((error as ApiNotFoundException).getStatus()).toBe(404);
      expect((error as ApiNotFoundException).getResponse()).toEqual({
        errors: ["Forward rule abc was not found."],
      });
    }
  });

  it("re-throws any other error unchanged (e.g. a persist failure → generic 500 via the filter)", () => {
    const persistError = new Error("disk full");
    expect(() => mapManagerError(persistError)).toThrow(persistError);
  });
});
