import type { ForwardRule } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ApiBadRequestException } from "../common/api-errors.js";
import type { ClockReader } from "../common/clock.reader.js";
import { ConfigPlanService } from "./config-plan.service.js";
import type { ConfigPlanReader } from "./config-plan.reader.js";

const FIXED = new Date("2026-06-15T00:00:00.000Z");
const fixedClock: ClockReader = { now: () => FIXED };

function rule(id: string, listenPort: number): ForwardRule {
  return {
    id,
    name: id,
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort,
    targetHost: "127.0.0.1",
    targetPort: 8080,
    enabled: false,
  };
}

function service(rules: ForwardRule[]): ConfigPlanService {
  const reader: ConfigPlanReader = { listRules: () => rules };
  return new ConfigPlanService(reader, fixedClock);
}

describe("ConfigPlanService.plan — request validation (`desired` key-presence check)", () => {
  it.each([
    ["undefined body", undefined],
    ["null body", null],
    ["a string body", "nope"],
    ["an array body", [] as unknown],
    ["an object without a desired key", { other: 1 }],
  ])("throws 400 ApiBadRequestException for %s", (_label, body) => {
    expect(() => service([]).plan(body)).toThrow(ApiBadRequestException);
    try {
      service([]).plan(body);
    } catch (error) {
      expect((error as ApiBadRequestException).getResponse()).toEqual({ errors: ["desired is required."] });
    }
  });

  it("allows `desired: null` (key present) — returns a plan with an INVALID_DESIRED_CONFIG error, NOT a 400", () => {
    const plan = service([]).plan({ desired: null });
    expect(plan.generatedAt).toBe(FIXED.toISOString());
    expect(plan.errors.some((e) => e.code === "INVALID_DESIRED_CONFIG")).toBe(true);
    expect(plan.summary.hasErrors).toBe(true);
  });
});

describe("ConfigPlanService.plan — planning over current rules with the pinned clock", () => {
  it("plans an add against an empty current config", () => {
    const desired = { rules: [{ name: "New", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 49000, targetHost: "127.0.0.1", targetPort: 9000, enabled: false }] };
    const plan = service([]).plan({ desired });
    expect(plan.generatedAt).toBe(FIXED.toISOString()); // clock pinned the volatile field
    expect(plan.mode).toBe("plan");
    expect(plan.summary.add).toBe(1);
    expect(plan.summary.hasDrift).toBe(true);
    expect(plan.operations.map((op) => op.type)).toContain("add");
  });

  it("plans `unchanged` when the desired config matches the current rules", () => {
    const current = rule("r1", 49001);
    const desired = { rules: [{ ...current }] };
    const plan = service([current]).plan({ desired });
    expect(plan.summary.hasDrift).toBe(false);
    expect(plan.summary.unchanged).toBe(1);
  });
});
