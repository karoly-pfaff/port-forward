import type { ForwardRuleResponse } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toForwardRuleResponseDto } from "./forward-rule.response.dto.js";

const RESPONSE: ForwardRuleResponse = {
  id: "r1",
  name: "Web",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48010,
  targetHost: "127.0.0.1",
  targetPort: 8080,
  enabled: false,
  advisories: [{ code: "COMMON_PORT", severity: "warning", message: "x" }],
};

describe("toForwardRuleResponseDto", () => {
  it("preserves the rule-response shape byte-for-byte without mutating the source", () => {
    const snapshot = structuredClone(RESPONSE);

    const dto = toForwardRuleResponseDto(RESPONSE);

    expect(dto).toEqual(RESPONSE);
    expect(dto).not.toBe(RESPONSE); // fresh object
    expect(RESPONSE).toEqual(snapshot); // source untouched
  });
});
