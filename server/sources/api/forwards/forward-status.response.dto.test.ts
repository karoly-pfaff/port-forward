import type { ForwardStatus } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toForwardStatusResponseDto } from "./forward-status.response.dto.js";

const STATUS: ForwardStatus = {
  ruleId: "r1",
  running: true,
  health: "healthy",
  activeConnections: 0,
  bytesIn: 10,
  bytesOut: 20,
  startedAt: "2026-06-15T00:00:00.000Z",
};

describe("toForwardStatusResponseDto", () => {
  it("preserves the status shape byte-for-byte without mutating the source", () => {
    const snapshot = structuredClone(STATUS);

    const dto = toForwardStatusResponseDto(STATUS);

    expect(dto).toEqual(STATUS);
    expect(dto).not.toBe(STATUS); // fresh object
    expect(STATUS).toEqual(snapshot); // source untouched
  });
});
