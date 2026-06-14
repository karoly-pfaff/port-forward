import type { ForwardStatus } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { StatusController } from "./status.controller.js";
import type { StatusService } from "./status.service.js";

describe("StatusController.list", () => {
  it("delegates to the service and maps the result to the response DTO", () => {
    const statuses: ForwardStatus[] = [
      { ruleId: "r1", running: false, health: "healthy", bytesIn: 0, bytesOut: 0 },
    ];
    const controller = new StatusController({ list: () => statuses } as unknown as StatusService);

    const result = controller.list();

    expect(result).toEqual(statuses); // byte-for-byte
    expect(result).not.toBe(statuses); // mapped copy
  });
});
