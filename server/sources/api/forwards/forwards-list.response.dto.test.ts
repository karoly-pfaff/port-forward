import type { ForwardRuleResponse } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toForwardsListResponseDto } from "./forwards-list.response.dto.js";

describe("toForwardsListResponseDto", () => {
  it("maps an empty list to an empty array", () => {
    expect(toForwardsListResponseDto([])).toEqual([]);
  });

  it("preserves the rule-response shape byte-for-byte without mutating the source", () => {
    const source: ForwardRuleResponse[] = [
      {
        id: "r1",
        name: "Web",
        protocol: "tcp",
        listenHost: "0.0.0.0",
        listenPort: 48010,
        targetHost: "127.0.0.1",
        targetPort: 8080,
        enabled: false,
        advisories: [{ code: "LAN_EXPOSURE", severity: "warning", message: "m" }],
      },
    ];
    const snapshot = structuredClone(source);

    const dto = toForwardsListResponseDto(source);

    expect(dto).toEqual(source);
    expect(dto).not.toBe(source);
    expect(dto[0]).not.toBe(source[0]);
    expect(source).toEqual(snapshot);
  });
});
