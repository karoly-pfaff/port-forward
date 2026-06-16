import type { ForwardRule } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ConflictError, NotFoundError, ValidationError } from "../../forwarders/forward-manager.js";
import {
  ApiBadRequestException,
  ApiConflictException,
  ApiNotFoundException,
} from "../common/api-errors.js";
import { UpdateForwardRuleService } from "./update-forward-rule.service.js";
import type { ForwardRuleUpdater } from "./forwards.writer.js";

const RULE: ForwardRule = {
  id: "r1",
  name: "Renamed",
  protocol: "tcp",
  listenHost: "0.0.0.0",
  listenPort: 48010,
  targetHost: "127.0.0.1",
  targetPort: 8080,
  enabled: false,
};

function service(updater: ForwardRuleUpdater): UpdateForwardRuleService {
  return new UpdateForwardRuleService(updater);
}

describe("UpdateForwardRuleService.update", () => {
  it("updates the rule and decorates it with port advisories", async () => {
    let receivedId: string | undefined;
    let receivedPatch: unknown;
    const response = await service({
      updateRule: async (id, patch) => {
        receivedId = id;
        receivedPatch = patch;
        return RULE;
      },
    }).update("r1", { name: "Renamed" });

    expect(receivedId).toBe("r1");
    expect(receivedPatch).toEqual({ name: "Renamed" });
    expect(response).toMatchObject({ id: "r1", name: "Renamed" });
    expect(response.advisories.some((a) => a.code === "LAN_EXPOSURE")).toBe(true);
  });

  it("translates a manager ValidationError to a 400 ApiBadRequestException", async () => {
    const updater: ForwardRuleUpdater = {
      updateRule: async () => {
        throw new ValidationError(["listenPort must be an integer from 1 to 65535."]);
      },
    };
    await expect(service(updater).update("r1", {})).rejects.toBeInstanceOf(ApiBadRequestException);
  });

  it("translates a manager NotFoundError to a 404 ApiNotFoundException", async () => {
    const updater: ForwardRuleUpdater = {
      updateRule: async () => {
        throw new NotFoundError("Forward rule nope was not found.");
      },
    };
    await expect(service(updater).update("nope", {})).rejects.toBeInstanceOf(ApiNotFoundException);
  });

  it("translates a manager ConflictError to a 409 ApiConflictException", async () => {
    const updater: ForwardRuleUpdater = {
      updateRule: async () => {
        throw new ConflictError("A TCP rule is already listening on 0.0.0.0:48010.");
      },
    };
    await expect(service(updater).update("r1", {})).rejects.toBeInstanceOf(ApiConflictException);
  });

  it("re-throws an unexpected error (e.g. persist failure) unchanged", async () => {
    const persistError = new Error("disk full");
    const updater: ForwardRuleUpdater = {
      updateRule: async () => {
        throw persistError;
      },
    };
    await expect(service(updater).update("r1", {})).rejects.toBe(persistError);
  });
});
