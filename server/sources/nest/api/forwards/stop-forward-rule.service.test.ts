import type { ForwardStatus } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { NotFoundError } from "../../../forward-manager.js";
import { ApiNotFoundException } from "../../common/api-errors.js";
import { StopForwardRuleService } from "./stop-forward-rule.service.js";
import type { ForwardRuleStopper } from "./forwards.writer.js";

const STATUS: ForwardStatus = {
  ruleId: "r1",
  running: false,
  health: "healthy",
  bytesIn: 0,
  bytesOut: 0,
};

function service(stopper: ForwardRuleStopper): StopForwardRuleService {
  return new StopForwardRuleService(stopper);
}

describe("StopForwardRuleService.stop", () => {
  it("stops the rule and returns its status", async () => {
    let receivedId: string | undefined;
    const result = await service({
      stopRule: async (id) => {
        receivedId = id;
        return STATUS;
      },
    }).stop("r1");

    expect(receivedId).toBe("r1");
    expect(result).toBe(STATUS);
  });

  it("translates a manager NotFoundError to a 404 ApiNotFoundException", async () => {
    const stopper: ForwardRuleStopper = {
      stopRule: async () => {
        throw new NotFoundError("Forward rule nope was not found.");
      },
    };
    await expect(service(stopper).stop("nope")).rejects.toBeInstanceOf(ApiNotFoundException);
  });

  it("re-throws an unexpected stop error unchanged → generic 500", async () => {
    const stopError = new Error("socket close failed");
    const stopper: ForwardRuleStopper = {
      stopRule: async () => {
        throw stopError;
      },
    };
    await expect(service(stopper).stop("r1")).rejects.toBe(stopError);
  });
});
