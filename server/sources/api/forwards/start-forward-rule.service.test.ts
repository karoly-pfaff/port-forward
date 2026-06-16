import type { ForwardStatus } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { NotFoundError } from "../../forward-manager.js";
import { ApiNotFoundException } from "../../common/api-errors.js";
import { StartForwardRuleService } from "./start-forward-rule.service.js";
import type { ForwardRuleStarter } from "./forwards.writer.js";

const STATUS: ForwardStatus = {
  ruleId: "r1",
  running: true,
  health: "healthy",
  bytesIn: 0,
  bytesOut: 0,
  startedAt: "2026-06-15T00:00:00.000Z",
};

function service(starter: ForwardRuleStarter): StartForwardRuleService {
  return new StartForwardRuleService(starter);
}

describe("StartForwardRuleService.start", () => {
  it("starts the rule and returns its status", async () => {
    let receivedId: string | undefined;
    const result = await service({
      startRule: async (id) => {
        receivedId = id;
        return STATUS;
      },
    }).start("r1");

    expect(receivedId).toBe("r1");
    expect(result).toBe(STATUS);
  });

  it("translates a manager NotFoundError to a 404 ApiNotFoundException", async () => {
    const starter: ForwardRuleStarter = {
      startRule: async () => {
        throw new NotFoundError("Forward rule nope was not found.");
      },
    };
    await expect(service(starter).start("nope")).rejects.toBeInstanceOf(ApiNotFoundException);
  });

  it("re-throws an unexpected start error (e.g. port in use) unchanged → generic 500", async () => {
    const startError = new Error("listen EADDRINUSE: address already in use 0.0.0.0:48010");
    const starter: ForwardRuleStarter = {
      startRule: async () => {
        throw startError;
      },
    };
    await expect(service(starter).start("r1")).rejects.toBe(startError);
  });
});
