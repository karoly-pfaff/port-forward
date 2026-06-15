import type { ConfigPlanResponse } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toConfigPlanResponseDto } from "./config-plan.response.dto.js";

const PLAN: ConfigPlanResponse = {
  generatedAt: "2026-06-15T00:00:00.000Z",
  mode: "plan",
  summary: { add: 1, update: 1, remove: 0, unchanged: 0, destructive: 1, hasDrift: true, hasErrors: false },
  operations: [
    {
      type: "add",
      ruleName: "New",
      protocol: "tcp",
      desired: { name: "New", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 49000, targetHost: "127.0.0.1", targetPort: 9000, enabled: false },
      destructive: false,
    },
    {
      type: "update",
      ruleId: "r1",
      ruleName: "Web",
      protocol: "tcp",
      current: { id: "r1", name: "Web", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48010, targetHost: "127.0.0.1", targetPort: 8080, enabled: false },
      desired: { id: "r1", name: "Web", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48011, targetHost: "127.0.0.1", targetPort: 8080, enabled: false },
      changes: [{ field: "listenPort", before: 48010, after: 48011 }],
      destructive: true,
    },
  ],
  errors: [{ code: "X", message: "x", field: "rules[0]" }],
  warnings: [{ code: "LAN_EXPOSURE", message: "w" }],
};

describe("toConfigPlanResponseDto", () => {
  it("preserves the plan shape byte-for-byte without mutating the source", () => {
    const snapshot = structuredClone(PLAN);

    const dto = toConfigPlanResponseDto(PLAN);

    expect(dto).toEqual(PLAN);
    expect(PLAN).toEqual(snapshot); // source untouched
  });

  it("returns fresh nested objects (summary, operations, snapshots, changes, errors, warnings)", () => {
    const dto = toConfigPlanResponseDto(PLAN);

    expect(dto).not.toBe(PLAN);
    expect(dto.summary).not.toBe(PLAN.summary);
    expect(dto.operations).not.toBe(PLAN.operations);
    expect(dto.operations[1]).not.toBe(PLAN.operations[1]);
    expect(dto.operations[1].current).not.toBe(PLAN.operations[1].current);
    expect(dto.operations[1].changes).not.toBe(PLAN.operations[1].changes);
    expect(dto.errors).not.toBe(PLAN.errors);
    expect(dto.warnings).not.toBe(PLAN.warnings);
  });
});
