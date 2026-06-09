import { describe, expect, it } from "vitest";
import type { ConfigPlanResponse } from "@portier/shared";
import { formatChangeValue, formatOperationType, hasPlanErrors, isDestructivePlan } from "./planHelpers.js";

function makePlan(overrides: Partial<ConfigPlanResponse["summary"]> = {}): ConfigPlanResponse {
  return {
    generatedAt: "2026-01-01T00:00:00.000Z",
    mode: "plan",
    summary: {
      add: 0, update: 0, remove: 0, unchanged: 1,
      destructive: 0, hasDrift: false, hasErrors: false,
      ...overrides
    },
    operations: [],
    errors: [],
    warnings: []
  };
}

describe("formatOperationType", () => {
  it("formats add", () => expect(formatOperationType("add")).toBe("Add"));
  it("formats update", () => expect(formatOperationType("update")).toBe("Update"));
  it("formats remove", () => expect(formatOperationType("remove")).toBe("Remove"));
  it("formats unchanged", () => expect(formatOperationType("unchanged")).toBe("Unchanged"));
});

describe("formatChangeValue", () => {
  it("formats null as (none)", () => expect(formatChangeValue(null)).toBe("(none)"));
  it("formats undefined as (none)", () => expect(formatChangeValue(undefined)).toBe("(none)"));
  it("formats true as yes", () => expect(formatChangeValue(true)).toBe("yes"));
  it("formats false as no", () => expect(formatChangeValue(false)).toBe("no"));
  it("formats numbers as strings", () => expect(formatChangeValue(48001)).toBe("48001"));
  it("formats strings as-is", () => expect(formatChangeValue("tcp")).toBe("tcp"));
  it("formats zero as string", () => expect(formatChangeValue(0)).toBe("0"));
  it("formats empty string", () => expect(formatChangeValue("")).toBe(""));
});

describe("isDestructivePlan", () => {
  it("returns false when destructive=0", () =>
    expect(isDestructivePlan(makePlan({ destructive: 0 }))).toBe(false));
  it("returns true when destructive=1", () =>
    expect(isDestructivePlan(makePlan({ destructive: 1 }))).toBe(true));
  it("returns true when destructive>1", () =>
    expect(isDestructivePlan(makePlan({ destructive: 3 }))).toBe(true));
});

describe("hasPlanErrors", () => {
  it("returns false when hasErrors=false", () =>
    expect(hasPlanErrors(makePlan({ hasErrors: false }))).toBe(false));
  it("returns true when hasErrors=true", () =>
    expect(hasPlanErrors(makePlan({ hasErrors: true }))).toBe(true));
});
