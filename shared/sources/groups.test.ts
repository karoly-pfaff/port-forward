import { describe, expect, it } from "vitest";
import { summarizeGroupAction, validateGroupName } from "./index.js";
import type { GroupActionResult } from "./index.js";

describe("summarizeGroupAction", () => {
  it("counts started/stopped as succeeded, plus skipped and failed", () => {
    const results: GroupActionResult[] = [
      { ruleId: "1", ruleName: "a", status: "started" },
      { ruleId: "2", ruleName: "b", status: "skipped", reason: "already_running" },
      { ruleId: "3", ruleName: "c", status: "failed", reason: "boom" },
      { ruleId: "4", ruleName: "d", status: "started" }
    ];
    expect(summarizeGroupAction("web", "start", results)).toEqual({
      group: "web",
      action: "start",
      total: 4,
      succeeded: 2,
      skipped: 1,
      failed: 1,
      results
    });
  });

  it("counts stopped results as succeeded", () => {
    const results: GroupActionResult[] = [
      { ruleId: "1", ruleName: "a", status: "stopped" },
      { ruleId: "2", ruleName: "b", status: "skipped", reason: "not_running" }
    ];
    const summary = summarizeGroupAction("api", "stop", results);
    expect(summary).toMatchObject({ action: "stop", total: 2, succeeded: 1, skipped: 1, failed: 0 });
  });

  it("returns zeroed counts for an empty result list", () => {
    expect(summarizeGroupAction("web", "start", [])).toEqual({
      group: "web",
      action: "start",
      total: 0,
      succeeded: 0,
      skipped: 0,
      failed: 0,
      results: []
    });
  });
});

describe("validateGroupName", () => {
  it("accepts a valid non-empty group", () => {
    expect(validateGroupName("web")).toEqual([]);
    expect(validateGroupName("  web team  ")).toEqual([]);
  });

  it("rejects empty / whitespace / non-string with 'group is required.'", () => {
    expect(validateGroupName("")).toEqual(["group is required."]);
    expect(validateGroupName("   ")).toEqual(["group is required."]);
    expect(validateGroupName(undefined)).toEqual(["group is required."]);
    expect(validateGroupName(42)).toEqual(["group is required."]);
  });

  it("applies the length and control-character rules to a present value", () => {
    expect(validateGroupName("x".repeat(65))).toContain("group must be 64 characters or fewer.");
    expect(validateGroupName(`bad${String.fromCharCode(1)}grp`)).toContain(
      "group must not contain control characters."
    );
  });
});
