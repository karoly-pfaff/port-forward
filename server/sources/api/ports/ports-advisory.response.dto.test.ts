import type { PortAdvisory } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toPortsAdvisoryResponseDto } from "./ports-advisory.response.dto.js";

describe("toPortsAdvisoryResponseDto", () => {
  it("maps an empty list to an empty array", () => {
    expect(toPortsAdvisoryResponseDto([])).toEqual([]);
  });

  it("preserves the advisory shape byte-for-byte without mutating the source", () => {
    const source: PortAdvisory[] = [{ code: "LAN_EXPOSURE", severity: "warning", message: "m" }];
    const snapshot = structuredClone(source);

    const dto = toPortsAdvisoryResponseDto(source);

    expect(dto).toEqual(source);
    expect(dto).not.toBe(source);
    expect(dto[0]).not.toBe(source[0]);
    expect(source).toEqual(snapshot);
  });
});
