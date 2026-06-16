import type { ForwardRule } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toForwardRuleResponse } from "./forward-rule-response.js";

const RULE: ForwardRule = {
  id: "r1",
  name: "Web",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48010,
  targetHost: "127.0.0.1",
  targetPort: 8080,
  enabled: false,
};

describe("toForwardRuleResponse", () => {
  it("decorates a rule with its forward-purpose port advisories", () => {
    const response = toForwardRuleResponse(RULE);
    expect(response).toMatchObject({ id: "r1", name: "Web", protocol: "tcp" });
    expect(Array.isArray(response.advisories)).toBe(true);
  });

  it("attaches a LAN_EXPOSURE advisory for a 0.0.0.0 listen host", () => {
    const response = toForwardRuleResponse({ ...RULE, listenHost: "0.0.0.0" });
    expect(response.advisories.some((a) => a.code === "LAN_EXPOSURE")).toBe(true);
  });

  it("attaches no advisories for a benign loopback binding", () => {
    const response = toForwardRuleResponse({ ...RULE, listenHost: "127.0.0.1", listenPort: 48010 });
    expect(response.advisories).toEqual([]);
  });
});
