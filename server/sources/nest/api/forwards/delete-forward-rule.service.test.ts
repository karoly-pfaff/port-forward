import { describe, expect, it } from "vitest";
import { ConflictError, NotFoundError, ValidationError } from "../../../forward-manager.js";
import {
  ApiBadRequestException,
  ApiConflictException,
  ApiNotFoundException,
} from "../../common/api-errors.js";
import { DeleteForwardRuleService } from "./delete-forward-rule.service.js";
import type { ForwardRuleDeleter } from "./forwards.writer.js";

function service(deleter: ForwardRuleDeleter): DeleteForwardRuleService {
  return new DeleteForwardRuleService(deleter);
}

describe("DeleteForwardRuleService.delete", () => {
  it("deletes the rule via the deleter (no return value)", async () => {
    let receivedId: string | undefined;
    const result = await service({
      deleteRule: async (id) => {
        receivedId = id;
      },
    }).delete("r1");

    expect(receivedId).toBe("r1");
    expect(result).toBeUndefined();
  });

  it("translates a manager NotFoundError to a 404 ApiNotFoundException", async () => {
    const deleter: ForwardRuleDeleter = {
      deleteRule: async () => {
        throw new NotFoundError("Forward rule nope was not found.");
      },
    };
    await expect(service(deleter).delete("nope")).rejects.toBeInstanceOf(ApiNotFoundException);
  });

  it("translates a manager ValidationError to a 400 ApiBadRequestException", async () => {
    const deleter: ForwardRuleDeleter = {
      deleteRule: async () => {
        throw new ValidationError(["bad"]);
      },
    };
    await expect(service(deleter).delete("r1")).rejects.toBeInstanceOf(ApiBadRequestException);
  });

  it("translates a manager ConflictError to a 409 ApiConflictException", async () => {
    const deleter: ForwardRuleDeleter = {
      deleteRule: async () => {
        throw new ConflictError("conflict");
      },
    };
    await expect(service(deleter).delete("r1")).rejects.toBeInstanceOf(ApiConflictException);
  });

  it("re-throws an unexpected error (e.g. persist failure) unchanged", async () => {
    const persistError = new Error("disk full");
    const deleter: ForwardRuleDeleter = {
      deleteRule: async () => {
        throw persistError;
      },
    };
    await expect(service(deleter).delete("r1")).rejects.toBe(persistError);
  });
});
