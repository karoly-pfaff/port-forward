import type { ForwardRule } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ConflictError, ValidationError } from "../../../forward-manager.js";
import { ApiBadRequestException, ApiConflictException } from "../../common/api-errors.js";
import { CreateForwardRuleService } from "./create-forward-rule.service.js";
import type { ForwardRuleCreator } from "./forwards.writer.js";

const RULE: ForwardRule = {
  id: "r1",
  name: "Web",
  protocol: "tcp",
  listenHost: "0.0.0.0",
  listenPort: 48010,
  targetHost: "127.0.0.1",
  targetPort: 8080,
  enabled: false,
};

function service(creator: ForwardRuleCreator): CreateForwardRuleService {
  return new CreateForwardRuleService(creator);
}

describe("CreateForwardRuleService.create", () => {
  it("creates the rule and decorates it with port advisories", async () => {
    const response = await service({ addRule: async () => RULE }).create(RULE);
    expect(response).toMatchObject({ id: "r1", name: "Web" });
    // 0.0.0.0 listen host → a LAN_EXPOSURE advisory is attached (same as the list read).
    expect(response.advisories.some((a) => a.code === "LAN_EXPOSURE")).toBe(true);
  });

  it("translates a manager ValidationError to a 400 ApiBadRequestException", async () => {
    const creator: ForwardRuleCreator = {
      addRule: async () => {
        throw new ValidationError(["name is required."]);
      },
    };
    await expect(service(creator).create({})).rejects.toBeInstanceOf(ApiBadRequestException);
  });

  it("translates a manager ConflictError to a 409 ApiConflictException", async () => {
    const creator: ForwardRuleCreator = {
      addRule: async () => {
        throw new ConflictError("A TCP rule is already listening on 0.0.0.0:48010.");
      },
    };
    await expect(service(creator).create(RULE)).rejects.toBeInstanceOf(ApiConflictException);
  });

  it("re-throws an unexpected error (e.g. persist failure) unchanged", async () => {
    const persistError = new Error("disk full");
    const creator: ForwardRuleCreator = {
      addRule: async () => {
        throw persistError;
      },
    };
    await expect(service(creator).create(RULE)).rejects.toBe(persistError);
  });
});
