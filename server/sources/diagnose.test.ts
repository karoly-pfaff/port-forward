import dgram from "node:dgram";
import { afterEach, describe, expect, it } from "vitest";
import type { ForwardRule } from "@portier/shared";
import { diagnoseRule } from "./diagnose.js";
import { getFreeUdpPort } from "./test-helpers.js";

function udpRule(overrides: Partial<ForwardRule> = {}): ForwardRule {
  return {
    id: "r1",
    name: "Test UDP",
    protocol: "udp",
    listenHost: "127.0.0.1",
    listenPort: 48002,
    targetHost: "127.0.0.1",
    targetPort: 9000,
    enabled: true,
    ...overrides
  };
}

describe("diagnoseRule UDP bind error", () => {
  const cleanup: Array<() => void> = [];

  afterEach(() => {
    cleanup.splice(0).forEach((fn) => fn());
  });

  it("returns fail listen-bind when UDP port is already in use", async () => {
    const port = await getFreeUdpPort();
    const occupying = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => occupying.bind(port, "127.0.0.1", resolve));
    cleanup.push(() => { try { occupying.close(); } catch { /* ignore */ } });

    const result = await diagnoseRule(udpRule({ listenPort: port }), false);

    const bindCheck = result.checks.find((c) => c.id === "listen-bind");
    expect(bindCheck?.status).toBe("fail");
    expect(bindCheck?.message).toContain("failed");
  });

  it("returns warn udp-mode for bidirectional-last-client", async () => {
    const port = await getFreeUdpPort();
    const result = await diagnoseRule(
      udpRule({ listenPort: port, udpMode: "bidirectional-last-client" }),
      true
    );
    const check = result.checks.find((c) => c.id === "udp-mode");
    expect(check?.status).toBe("warn");
  });

  it("returns pass udp-mode for bidirectional-multi-client", async () => {
    const port = await getFreeUdpPort();
    const result = await diagnoseRule(
      udpRule({ listenPort: port, udpMode: "bidirectional-multi-client" }),
      true
    );
    const check = result.checks.find((c) => c.id === "udp-mode");
    expect(check?.status).toBe("pass");
    expect(check?.message).toContain("bidirectional-multi-client");
  });

  it("returns pass listen-bind when isRunning is true", async () => {
    const result = await diagnoseRule(udpRule({ listenPort: 48002 }), true);
    const check = result.checks.find((c) => c.id === "listen-bind");
    expect(check?.status).toBe("pass");
    expect(check?.message).toContain("already owned by Portier");
  });
});
