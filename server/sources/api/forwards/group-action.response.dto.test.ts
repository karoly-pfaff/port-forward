import type { GroupActionResponse } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toGroupActionResponseDto } from "./group-action.response.dto.js";

const RESPONSE: GroupActionResponse = {
  group: "web",
  action: "stop",
  total: 2,
  succeeded: 1,
  skipped: 1,
  failed: 0,
  results: [
    { ruleId: "r1", ruleName: "One", status: "stopped" },
    { ruleId: "r2", ruleName: "Two", status: "skipped", reason: "not_running" },
  ],
};

describe("toGroupActionResponseDto", () => {
  it("preserves the group-action response shape byte-for-byte without mutating the source", () => {
    const snapshot = structuredClone(RESPONSE);

    const dto = toGroupActionResponseDto(RESPONSE);

    expect(dto).toEqual(RESPONSE);
    expect(RESPONSE).toEqual(snapshot); // source untouched
  });

  it("returns fresh nested objects (results array + result objects) — not the source references", () => {
    const dto = toGroupActionResponseDto(RESPONSE);

    expect(dto).not.toBe(RESPONSE);
    expect(dto.results).not.toBe(RESPONSE.results);
    expect(dto.results[0]).not.toBe(RESPONSE.results[0]);
    // The result without a reason stays without it; the one with a reason keeps it.
    expect(dto.results[0]).not.toHaveProperty("reason");
    expect(dto.results[1].reason).toBe("not_running");
  });
});
