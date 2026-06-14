import type { ForwardStatus } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { StatusController } from "./status.controller.js";
import type { StatusService } from "./status.service.js";

describe("StatusController.list", () => {
  it("delegates to the service and returns its statuses", () => {
    const statuses: ForwardStatus[] = [
      { ruleId: "r1", running: false, health: "healthy", bytesIn: 0, bytesOut: 0 },
    ];
    const controller = new StatusController({ list: () => statuses } as unknown as StatusService);

    expect(controller.list()).toBe(statuses);
  });
});
