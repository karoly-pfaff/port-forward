import type { GroupActionResult } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ApiBadRequestException, ApiNotFoundException } from "../common/api-errors.js";
import { StartForwardGroupService } from "./start-forward-group.service.js";
import type { ForwardGroupStarter } from "./forwards.writer.js";

function service(starter: ForwardGroupStarter): StartForwardGroupService {
  return new StartForwardGroupService(starter);
}

describe("StartForwardGroupService.start", () => {
  it("starts the group and summarizes the per-rule results", async () => {
    let receivedGroup: string | undefined;
    const results: GroupActionResult[] = [
      { ruleId: "r1", ruleName: "One", status: "started" },
      { ruleId: "r2", ruleName: "Two", status: "skipped", reason: "already_running" },
    ];
    const response = await service({
      startGroup: async (group) => {
        receivedGroup = group;
        return results;
      },
    }).start("web");

    expect(receivedGroup).toBe("web"); // normalized group passed through
    expect(response).toEqual({
      group: "web",
      action: "start",
      total: 2,
      succeeded: 1,
      skipped: 1,
      failed: 0,
      results,
    });
  });

  it("normalizes the group via decodeURIComponent + trim", async () => {
    let receivedGroup: string | undefined;
    await service({
      startGroup: async (group) => {
        receivedGroup = group;
        return [{ ruleId: "r1", ruleName: "One", status: "started" }];
      },
    }).start("%20web%20team%20"); // "  web team " → trimmed → "web team"

    expect(receivedGroup).toBe("web team");
  });

  it("throws a 400 ApiBadRequestException for an empty/whitespace group (delegated to validateGroupName)", async () => {
    let called = false;
    const starter: ForwardGroupStarter = {
      startGroup: async () => {
        called = true;
        return [];
      },
    };
    await expect(service(starter).start("%20%20")).rejects.toBeInstanceOf(ApiBadRequestException);
    expect(called).toBe(false); // never reaches the starter
  });

  it("throws a 404 ApiNotFoundException when the group has no rules", async () => {
    const starter: ForwardGroupStarter = { startGroup: async () => [] };
    await expect(service(starter).start("ghost")).rejects.toBeInstanceOf(ApiNotFoundException);
  });
});
