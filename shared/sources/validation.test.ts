import { describe, expect, it } from "vitest";
import {
  getCommonPortInfo,
  getPortAdvisories,
  isRecommendedForwardPort,
  listenKey,
  validateForwardRule,
  validateForwardRulePatch,
  validateGroup
} from "./index.js";

const baseTcpRule = {
  name: "Grouped",
  protocol: "tcp" as const,
  listenHost: "127.0.0.1",
  listenPort: 48001,
  targetHost: "127.0.0.1",
  targetPort: 3000,
  enabled: true
};

describe("ForwardRule validation", () => {
  it("accepts a valid TCP rule", () => {
    const result = validateForwardRule({
      name: "Local app",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 8080,
      targetHost: "example.test",
      targetPort: 80,
      enabled: true
    });

    expect(result.valid).toBe(true);
    expect(result.value?.protocol).toBe("tcp");
  });

  it("defaults UDP mode to one-way", () => {
    const result = validateForwardRule({
      name: "Stats",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: 9000,
      targetHost: "127.0.0.1",
      targetPort: 9001,
      enabled: false
    });

    expect(result.valid).toBe(true);
    expect(result.value?.udpMode).toBe("one-way");
  });

  it("rejects invalid ports and protocol", () => {
    const result = validateForwardRule({
      name: "",
      protocol: "http",
      listenHost: "127.0.0.1",
      listenPort: 70000,
      targetHost: "",
      targetPort: 0,
      enabled: "yes"
    });

    expect(result.valid).toBe(false);
    expect(result.errors).toEqual(
      expect.arrayContaining([
        "name is required.",
        "protocol must be tcp or udp.",
        "listenPort must be an integer from 1 to 65535.",
        "targetHost is required.",
        "targetPort must be an integer from 1 to 65535.",
        "enabled must be a boolean."
      ])
    );
  });

  it("validates partial patches", () => {
    expect(validateForwardRulePatch({ listenPort: 3000 }).valid).toBe(true);
    expect(validateForwardRulePatch({ listenPort: -1 }).valid).toBe(false);
  });
});

describe("listenKey", () => {
  it("formats protocol:host:port", () => {
    expect(listenKey({ protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48001 })).toBe("tcp:127.0.0.1:48001");
    expect(listenKey({ protocol: "udp", listenHost: "0.0.0.0", listenPort: 9000 })).toBe("udp:0.0.0.0:9000");
  });
});

describe("validateForwardRulePatch trimming and optional fields", () => {
  it("trims string fields when provided", () => {
    const result = validateForwardRulePatch({
      name: "  My Rule  ",
      listenHost: "  127.0.0.1  ",
      targetHost: "  example.com  "
    });
    expect(result.valid).toBe(true);
    expect(result.value?.name).toBe("My Rule");
    expect(result.value?.listenHost).toBe("127.0.0.1");
    expect(result.value?.targetHost).toBe("example.com");
  });

  it("accepts optional id when provided as non-empty string", () => {
    const result = validateForwardRulePatch({ id: "rule-abc" });
    expect(result.valid).toBe(true);
    expect(result.value?.id).toBe("rule-abc");
  });

  it("rejects empty string id", () => {
    expect(validateForwardRulePatch({ id: "  " }).valid).toBe(false);
  });

  it("accepts absent fields without error", () => {
    const result = validateForwardRulePatch({ listenPort: 48001 });
    expect(result.valid).toBe(true);
    expect(result.value?.name).toBeUndefined();
    expect(result.value?.listenHost).toBeUndefined();
    expect(result.value?.targetHost).toBeUndefined();
  });

  it("rejects enabled non-boolean", () => {
    const result = validateForwardRulePatch({ enabled: "yes" as unknown as boolean });
    expect(result.valid).toBe(false);
    expect(result.errors).toContain("enabled must be a boolean.");
  });

  it("rejects empty string targetHost", () => {
    expect(validateForwardRulePatch({ targetHost: "" }).valid).toBe(false);
    expect(validateForwardRulePatch({ targetHost: "" }).errors).toContain(
      "targetHost must be a non-empty string."
    );
  });

  it("rejects invalid targetPort", () => {
    expect(validateForwardRulePatch({ targetPort: 0 }).valid).toBe(false);
    expect(validateForwardRulePatch({ targetPort: 0 }).errors).toContain(
      "targetPort must be an integer from 1 to 65535."
    );
  });

  it("rejects empty string listenHost", () => {
    expect(validateForwardRulePatch({ listenHost: "" }).valid).toBe(false);
    expect(validateForwardRulePatch({ listenHost: "" }).errors).toContain(
      "listenHost must be a non-empty string."
    );
  });

  it("rejects non-object input", () => {
    expect(validateForwardRulePatch(null).valid).toBe(false);
    expect(validateForwardRulePatch("string").valid).toBe(false);
  });

  it("rejects empty string name", () => {
    expect(validateForwardRulePatch({ name: "" }).valid).toBe(false);
    expect(validateForwardRulePatch({ name: "" }).errors).toContain(
      "name must be a non-empty string."
    );
  });

  it("rejects invalid protocol", () => {
    expect(validateForwardRulePatch({ protocol: "http" as never }).valid).toBe(false);
    expect(validateForwardRulePatch({ protocol: "http" as never }).errors).toContain(
      "protocol must be tcp or udp."
    );
  });

  it("accepts valid udpMode in patch", () => {
    const result = validateForwardRulePatch({ udpMode: "bidirectional-last-client" });
    expect(result.valid).toBe(true);
    expect(result.value?.udpMode).toBe("bidirectional-last-client");
  });

  it("rejects invalid udpMode in patch", () => {
    const result = validateForwardRulePatch({ udpMode: "invalid-mode" as never });
    expect(result.valid).toBe(false);
    expect(result.errors).toContain(
      "udpMode must be one-way, bidirectional-last-client, or bidirectional-multi-client."
    );
  });
});

describe("validateForwardRule id and optional-id edge cases", () => {
  it("rejects empty-string id", () => {
    const result = validateForwardRule({
      id: "  ",
      name: "Test",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48001,
      targetHost: "127.0.0.1",
      targetPort: 3000,
      enabled: true
    });
    expect(result.valid).toBe(false);
    expect(result.errors).toContain("id must be a non-empty string when provided.");
  });

  it("accepts rule without id (generates uuid externally)", () => {
    const result = validateForwardRule({
      name: "No ID",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48001,
      targetHost: "127.0.0.1",
      targetPort: 3000,
      enabled: true
    });
    expect(result.valid).toBe(true);
    expect(result.value?.id).toBeUndefined();
  });

  it("rejects non-object input", () => {
    expect(validateForwardRule(null).valid).toBe(false);
    expect(validateForwardRule("string").valid).toBe(false);
    expect(validateForwardRule(42).valid).toBe(false);
  });

  it("rejects empty or missing listenHost", () => {
    const result = validateForwardRule({
      name: "Test",
      protocol: "tcp",
      listenHost: "",
      listenPort: 48001,
      targetHost: "127.0.0.1",
      targetPort: 3000,
      enabled: true
    });
    expect(result.valid).toBe(false);
    expect(result.errors).toContain("listenHost is required.");
  });

  it("rejects udpMode on TCP rule", () => {
    const result = validateForwardRule({
      name: "Test",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48001,
      targetHost: "127.0.0.1",
      targetPort: 3000,
      enabled: true,
      udpMode: "one-way"
    });
    expect(result.valid).toBe(false);
    expect(result.errors).toContain("udpMode is only valid for UDP rules.");
  });

  it("accepts explicit udpMode on UDP rule", () => {
    const result = validateForwardRule({
      name: "Stats",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: 9000,
      targetHost: "127.0.0.1",
      targetPort: 9001,
      enabled: false,
      udpMode: "bidirectional-multi-client"
    });
    expect(result.valid).toBe(true);
    expect(result.value?.udpMode).toBe("bidirectional-multi-client");
  });

  it("rejects invalid udpMode on UDP rule", () => {
    const result = validateForwardRule({
      name: "Stats",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: 9000,
      targetHost: "127.0.0.1",
      targetPort: 9001,
      enabled: false,
      udpMode: "chatty" as never
    });
    expect(result.valid).toBe(false);
  });

  it("trims whitespace from string fields on success", () => {
    const result = validateForwardRule({
      name: "  My Rule  ",
      protocol: "tcp",
      listenHost: "  127.0.0.1  ",
      listenPort: 48001,
      targetHost: "  example.com  ",
      targetPort: 80,
      enabled: true
    });
    expect(result.valid).toBe(true);
    expect(result.value?.name).toBe("My Rule");
    expect(result.value?.listenHost).toBe("127.0.0.1");
    expect(result.value?.targetHost).toBe("example.com");
  });
});

describe("rule group metadata", () => {
  it("omits group when absent", () => {
    const result = validateForwardRule(baseTcpRule);
    expect(result.valid).toBe(true);
    expect("group" in (result.value ?? {})).toBe(false);
  });

  it("trims and stores a valid group", () => {
    const result = validateForwardRule({ ...baseTcpRule, group: "  web-team  " });
    expect(result.valid).toBe(true);
    expect(result.value?.group).toBe("web-team");
  });

  it("normalizes an empty/whitespace group to absent", () => {
    for (const group of ["", "   "]) {
      const result = validateForwardRule({ ...baseTcpRule, group });
      expect(result.valid).toBe(true);
      expect("group" in (result.value ?? {})).toBe(false);
    }
  });

  it("rejects a group longer than 64 characters", () => {
    const result = validateForwardRule({ ...baseTcpRule, group: "x".repeat(65) });
    expect(result.valid).toBe(false);
    expect(result.errors).toContain("group must be 64 characters or fewer.");
  });

  it("accepts a group of exactly 64 characters", () => {
    const result = validateForwardRule({ ...baseTcpRule, group: "y".repeat(64) });
    expect(result.valid).toBe(true);
    expect(result.value?.group).toBe("y".repeat(64));
  });

  it("rejects a group with control characters", () => {
    const result = validateForwardRule({ ...baseTcpRule, group: "bad\u0001group" });
    expect(result.valid).toBe(false);
    expect(result.errors).toContain("group must not contain control characters.");
  });

  it("rejects a non-string group", () => {
    const result = validateForwardRule({ ...baseTcpRule, group: 42 as unknown as string });
    expect(result.valid).toBe(false);
    expect(result.errors).toContain("group must be a string.");
  });

  it("patch: setting a non-empty group stores the trimmed value", () => {
    const result = validateForwardRulePatch({ group: "  api-team  " });
    expect(result.valid).toBe(true);
    expect(result.value?.group).toBe("api-team");
  });

  it("patch: an empty-string group clears it (group key present, undefined)", () => {
    const result = validateForwardRulePatch({ group: "" });
    expect(result.valid).toBe(true);
    expect("group" in (result.value ?? {})).toBe(true);
    expect(result.value?.group).toBeUndefined();
  });

  it("patch: an absent group leaves the field untouched (key omitted)", () => {
    const result = validateForwardRulePatch({ name: "Renamed" });
    expect(result.valid).toBe(true);
    expect("group" in (result.value ?? {})).toBe(false);
  });

  it("patch: a null group is treated as unchanged (key omitted)", () => {
    const result = validateForwardRulePatch({ group: null as unknown as string });
    expect(result.valid).toBe(true);
    expect("group" in (result.value ?? {})).toBe(false);
  });

  it("patch: rejects an invalid group", () => {
    const result = validateForwardRulePatch({ group: "z".repeat(65) });
    expect(result.valid).toBe(false);
    expect(result.errors).toContain("group must be 64 characters or fewer.");
  });

  it("validateGroup returns no errors for accepted values", () => {
    expect(validateGroup(undefined)).toEqual([]);
    expect(validateGroup(null)).toEqual([]);
    expect(validateGroup("")).toEqual([]);
    expect(validateGroup("   ")).toEqual([]);
    expect(validateGroup("ops")).toEqual([]);
  });

  it("validateGroup reports each violation", () => {
    expect(validateGroup("x".repeat(65))).toContain("group must be 64 characters or fewer.");
    expect(validateGroup("a\u0007b")).toContain("group must not contain control characters.");
    expect(validateGroup(123)).toContain("group must be a string.");
  });
});

describe("Port advisories", () => {
  it("looks up common port metadata", () => {
    expect(getCommonPortInfo(5173)).toMatchObject({
      port: 5173,
      label: "Vite dev server",
      category: "dev",
      severity: "warning"
    });
    expect(getCommonPortInfo(48001)).toBeUndefined();
  });

  it("checks the recommended forwarding range", () => {
    expect(isRecommendedForwardPort(48000)).toBe(true);
    expect(isRecommendedForwardPort(48999)).toBe(true);
    expect(isRecommendedForwardPort(47999)).toBe(false);
    expect(isRecommendedForwardPort(49000)).toBe(false);
  });

  it("returns a privileged port advisory", () => {
    expect(getPortAdvisories({ port: 80, purpose: "forward" })).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          code: "PRIVILEGED_PORT",
          severity: "danger"
        })
      ])
    );
  });

  it("returns a LAN exposure advisory for forwarded ports", () => {
    expect(getPortAdvisories({ port: 48001, listenHost: "0.0.0.0", purpose: "forward" })).toEqual([
      {
        code: "LAN_EXPOSURE",
        severity: "warning",
        message: "Listening on 0.0.0.0 exposes this forwarded port on all interfaces. Other LAN devices may be able to connect if firewall settings allow it."
      }
    ]);
  });

  it("returns a danger advisory for management LAN exposure", () => {
    expect(getPortAdvisories({ port: 47831, listenHost: "0.0.0.0", purpose: "management" })).toEqual([
      {
        code: "MANAGEMENT_LAN_EXPOSURE",
        severity: "danger",
        message: "Listening on 0.0.0.0 exposes the Portier management UI/API on the LAN."
      }
    ]);
  });

  it("returns an info advisory outside the recommended forwarding range", () => {
    expect(getPortAdvisories({ port: 3000, purpose: "forward" })).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          code: "OUTSIDE_RECOMMENDED_RANGE",
          severity: "info"
        })
      ])
    );
  });

  it("returns both COMMON_PORT and PRIVILEGED_PORT for port 80 forwarding", () => {
    const advisories = getPortAdvisories({ port: 80, purpose: "forward" });
    const codes = advisories.map((a) => a.code);
    expect(codes).toContain("COMMON_PORT");
    expect(codes).toContain("PRIVILEGED_PORT");
    expect(codes).toContain("OUTSIDE_RECOMMENDED_RANGE");
  });

  it("does not return OUTSIDE_RECOMMENDED_RANGE for management purpose", () => {
    const advisories = getPortAdvisories({ port: 9999, purpose: "management" });
    expect(advisories.every((a) => a.code !== "OUTSIDE_RECOMMENDED_RANGE")).toBe(true);
  });

  it("does not return LAN_EXPOSURE for 127.0.0.1 forward port", () => {
    const advisories = getPortAdvisories({ port: 48001, listenHost: "127.0.0.1", purpose: "forward" });
    expect(advisories.every((a) => a.code !== "LAN_EXPOSURE")).toBe(true);
  });

  it("returns no advisories for a clean recommended forwarding port", () => {
    const advisories = getPortAdvisories({ port: 48500, listenHost: "127.0.0.1", purpose: "forward" });
    expect(advisories).toHaveLength(0);
  });

  it("returns LAN_EXPOSURE and MANAGEMENT_LAN_EXPOSURE only for their respective purposes", () => {
    const forward = getPortAdvisories({ port: 48001, listenHost: "0.0.0.0", purpose: "forward" });
    expect(forward.some((a) => a.code === "LAN_EXPOSURE")).toBe(true);
    expect(forward.every((a) => a.code !== "MANAGEMENT_LAN_EXPOSURE")).toBe(true);

    const management = getPortAdvisories({ port: 47831, listenHost: "0.0.0.0", purpose: "management" });
    expect(management.some((a) => a.code === "MANAGEMENT_LAN_EXPOSURE")).toBe(true);
    expect(management.every((a) => a.code !== "LAN_EXPOSURE")).toBe(true);
  });
});
