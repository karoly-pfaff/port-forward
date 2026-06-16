import type { ConfigApplyResponse } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toConfigApplyResponseDto } from "./config-apply.response.dto.js";

function response(): ConfigApplyResponse {
  return {
    ok: true,
    dryRun: false,
    appliedAt: "2026-06-15T08:30:00.000Z",
    plan: {
      generatedAt: "2026-06-15T08:30:00.000Z",
      mode: "plan",
      summary: { add: 1, update: 0, remove: 0, unchanged: 0, destructive: 0, hasDrift: true, hasErrors: false },
      operations: [
        {
          type: "add",
          ruleName: "Web",
          protocol: "tcp",
          desired: { id: "a", name: "Web", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48010, targetHost: "127.0.0.1", targetPort: 8080, enabled: false },
          destructive: false,
        },
      ],
      errors: [],
      warnings: [],
    },
    applied: { add: 1, update: 0, remove: 0, unchanged: 0 },
  };
}

describe("toConfigApplyResponseDto", () => {
  it("returns a byte-for-byte equal copy", () => {
    const source = response();
    expect(toConfigApplyResponseDto(source)).toEqual(source);
  });

  it("returns a fresh, deeply-independent object (no shared references) and does not mutate the source", () => {
    const source = response();
    const snapshot = structuredClone(source);
    const dto = toConfigApplyResponseDto(source);

    expect(dto).not.toBe(source);
    expect(dto.plan).not.toBe(source.plan);
    expect(dto.plan.operations).not.toBe(source.plan.operations);
    expect(dto.applied).not.toBe(source.applied);

    // Mutating the copy leaves the source untouched.
    dto.ok = false;
    dto.plan.operations.push({ type: "remove", ruleName: "x", protocol: "tcp", destructive: true });
    expect(source).toEqual(snapshot);
  });
});
