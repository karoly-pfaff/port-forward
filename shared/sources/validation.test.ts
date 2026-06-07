import { describe, expect, it } from "vitest";
import {
  getCommonPortInfo,
  getPortAdvisories,
  isRecommendedForwardPort,
  validateForwardRule,
  validateForwardRulePatch
} from "./index.js";

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
});
