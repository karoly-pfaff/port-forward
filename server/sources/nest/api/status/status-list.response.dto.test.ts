import type { ForwardStatus } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toStatusListResponseDto } from "./status-list.response.dto.js";

describe("toStatusListResponseDto", () => {
  it("maps an empty list to an empty array", () => {
    expect(toStatusListResponseDto([])).toEqual([]);
  });

  it("preserves the status shape byte-for-byte without mutating the source", () => {
    const source: ForwardStatus[] = [
      { ruleId: "r1", running: false, health: "healthy", bytesIn: 0, bytesOut: 0, activeConnections: 0 },
    ];
    const snapshot = structuredClone(source);

    const dto = toStatusListResponseDto(source);

    expect(dto).toEqual(source);
    expect(dto).not.toBe(source);
    expect(dto[0]).not.toBe(source[0]);
    expect(source).toEqual(snapshot);
  });
});
