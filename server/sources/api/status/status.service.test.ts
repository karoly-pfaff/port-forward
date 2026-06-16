import type { ForwardStatus } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { StatusService } from "./status.service.js";
import type { StatusReader } from "./status.reader.js";

function service(statuses: ForwardStatus[]): StatusService {
  const reader: StatusReader = { listStatus: () => statuses };
  return new StatusService(reader);
}

describe("StatusService.list", () => {
  it("returns the reader's statuses verbatim", () => {
    const statuses: ForwardStatus[] = [
      { ruleId: "r1", running: false, health: "healthy", bytesIn: 0, bytesOut: 0 },
    ];

    expect(service(statuses).list()).toBe(statuses);
  });

  it("returns an empty list when there are no rules", () => {
    expect(service([]).list()).toEqual([]);
  });
});
