import type { GroupActionResult } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ApiBadRequestException, ApiNotFoundException } from "../common/api-errors.js";
import { StopForwardGroupService } from "./stop-forward-group.service.js";
import type { ForwardGroupStopper } from "./forwards.writer.js";

function service(stopper: ForwardGroupStopper): StopForwardGroupService {
  return new StopForwardGroupService(stopper);
}

describe("StopForwardGroupService.stop", () => {
  it("stops the group and summarizes the per-rule results", async () => {
    let receivedGroup: string | undefined;
    const results: GroupActionResult[] = [
      { ruleId: "r1", ruleName: "One", status: "stopped" },
      { ruleId: "r2", ruleName: "Two", status: "skipped", reason: "not_running" },
    ];
    const response = await service({
      stopGroup: async (group) => {
        receivedGroup = group;
        return results;
      },
    }).stop("web");

    expect(receivedGroup).toBe("web"); // normalized group passed through
    expect(response).toEqual({
      group: "web",
      action: "stop",
      total: 2,
      succeeded: 1,
      skipped: 1,
      failed: 0,
      results,
    });
  });

  it("normalizes the group via decodeURIComponent + trim (matching Express)", async () => {
    let receivedGroup: string | undefined;
    await service({
      stopGroup: async (group) => {
        receivedGroup = group;
        return [{ ruleId: "r1", ruleName: "One", status: "stopped" }];
      },
    }).stop("%20web%20team%20"); // "  web team " → trimmed → "web team"

    expect(receivedGroup).toBe("web team");
  });

  it("throws a 400 ApiBadRequestException for an empty/whitespace group (delegated to validateGroupName)", async () => {
    let called = false;
    const stopper: ForwardGroupStopper = {
      stopGroup: async () => {
        called = true;
        return [];
      },
    };
    await expect(service(stopper).stop("%20%20")).rejects.toBeInstanceOf(ApiBadRequestException);
    expect(called).toBe(false); // never reaches the stopper
  });

  it("throws a 404 ApiNotFoundException when the group has no rules", async () => {
    const stopper: ForwardGroupStopper = { stopGroup: async () => [] };
    await expect(service(stopper).stop("ghost")).rejects.toBeInstanceOf(ApiNotFoundException);
  });
});
