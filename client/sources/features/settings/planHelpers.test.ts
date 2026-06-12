import { describe, expect, it } from "vitest";
import type { ConfigPlanOperation, ConfigPlanResponse } from "@portier/shared";
import {
  changeImpact,
  describeOperationImpact,
  formatChangeValue,
  formatFieldLabel,
  formatOperationType,
  hasPlanErrors,
  isDestructivePlan,
  isMetadataOnlyUpdate
} from "./planHelpers.js";

function makeOp(overrides: Partial<ConfigPlanOperation> = {}): ConfigPlanOperation {
  return {
    type: "update",
    ruleName: "Rule",
    protocol: "tcp",
    destructive: false,
    ...overrides
  };
}

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

describe("formatFieldLabel", () => {
  it("maps known snapshot fields to friendly labels", () => {
    expect(formatFieldLabel("listenPort")).toBe("Listen port");
    expect(formatFieldLabel("targetHost")).toBe("Target host");
    expect(formatFieldLabel("enabled")).toBe("Autostart");
    expect(formatFieldLabel("group")).toBe("Group");
    expect(formatFieldLabel("udpMode")).toBe("UDP mode");
  });
  it("falls back to the raw field name for unknown fields", () => {
    expect(formatFieldLabel("somethingNew")).toBe("somethingNew");
  });
});

describe("changeImpact", () => {
  it("classifies forwarding fields", () => {
    for (const f of ["protocol", "listenHost", "listenPort", "targetHost", "targetPort", "udpMode"]) {
      expect(changeImpact(f)).toBe("forwarding");
    }
  });
  it("classifies metadata fields (including group, name, enabled)", () => {
    for (const f of ["group", "name", "enabled"]) {
      expect(changeImpact(f)).toBe("metadata");
    }
  });
});

describe("isMetadataOnlyUpdate", () => {
  it("is true for a group-only update", () => {
    expect(
      isMetadataOnlyUpdate(makeOp({ changes: [{ field: "group", before: undefined, after: "web" }] }))
    ).toBe(true);
  });
  it("is true when every change is metadata (group + name + enabled)", () => {
    expect(
      isMetadataOnlyUpdate(
        makeOp({
          changes: [
            { field: "group", before: "a", after: "b" },
            { field: "name", before: "x", after: "y" },
            { field: "enabled", before: false, after: true }
          ]
        })
      )
    ).toBe(true);
  });
  it("is false when any change is a forwarding field", () => {
    expect(
      isMetadataOnlyUpdate(
        makeOp({
          changes: [
            { field: "group", before: "a", after: "b" },
            { field: "listenPort", before: 1, after: 2 }
          ]
        })
      )
    ).toBe(false);
  });
  it("is false for non-update operations", () => {
    expect(isMetadataOnlyUpdate(makeOp({ type: "add", changes: undefined }))).toBe(false);
    expect(isMetadataOnlyUpdate(makeOp({ type: "remove", destructive: true }))).toBe(false);
  });
  it("is false for an update with no changes", () => {
    expect(isMetadataOnlyUpdate(makeOp({ changes: [] }))).toBe(false);
    expect(isMetadataOnlyUpdate(makeOp({ changes: undefined }))).toBe(false);
  });
});

describe("describeOperationImpact", () => {
  it("describes add/remove/unchanged", () => {
    expect(describeOperationImpact(makeOp({ type: "add" }))).toMatch(/Creates a new rule/);
    expect(describeOperationImpact(makeOp({ type: "remove", destructive: true }))).toMatch(/Removes/);
    expect(describeOperationImpact(makeOp({ type: "unchanged" }))).toMatch(/No changes/);
  });
  it("describes a metadata-only update as not restarting the forwarder", () => {
    const op = makeOp({ destructive: false, changes: [{ field: "group", before: undefined, after: "web" }] });
    expect(describeOperationImpact(op)).toMatch(/Metadata only/);
    expect(describeOperationImpact(op)).toMatch(/not restarted/);
    expect(describeOperationImpact(op)).not.toMatch(/will restart/);
  });
  it("describes a forwarding update as restarting the forwarder", () => {
    const op = makeOp({ destructive: true, changes: [{ field: "listenPort", before: 1, after: 2 }] });
    expect(describeOperationImpact(op)).toMatch(/forwarder will restart/);
  });
});
