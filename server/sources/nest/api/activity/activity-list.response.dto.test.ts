import type { ActivityEvent } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toActivityListResponseDto } from "./activity-list.response.dto.js";

describe("toActivityListResponseDto", () => {
  it("wraps an empty list in { events: [] }", () => {
    expect(toActivityListResponseDto([])).toEqual({ events: [] });
  });

  it("wraps events byte-for-byte without mutating the source", () => {
    const source: ActivityEvent[] = [
      { id: "1", timestamp: "2026-06-14T00:00:00.000Z", type: "rule.error", severity: "error", ruleId: "r1", message: "boom" },
    ];
    const snapshot = structuredClone(source);

    const dto = toActivityListResponseDto(source);

    expect(dto).toEqual({ events: source });
    expect(dto.events).not.toBe(source);
    expect(dto.events[0]).not.toBe(source[0]);
    expect(source).toEqual(snapshot);
  });
});
