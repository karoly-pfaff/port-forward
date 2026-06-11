import net from "node:net";
import dgram from "node:dgram";
import { afterEach, describe, expect, it } from "vitest";
import type { ForwardRule } from "@portier/shared";
import { diagnoseRule } from "./diagnose.js";
import { getFreeTcpPort, getFreeUdpPort } from "./test-helpers.js";

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

function tcpRule(overrides: Partial<ForwardRule> = {}): ForwardRule {
  return {
    id: "r1",
    name: "Test TCP",
    protocol: "tcp",
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

  it("returns pass udp-mode for one-way", async () => {
    const port = await getFreeUdpPort();
    const result = await diagnoseRule(udpRule({ listenPort: port, udpMode: "one-way" }), true);
    const check = result.checks.find((c) => c.id === "udp-mode");
    expect(check?.status).toBe("pass");
    expect(check?.message).toContain("one-way");
  });
});

describe("diagnoseRule TCP bind and target connection", () => {
  const cleanup: Array<() => void> = [];

  afterEach(() => {
    cleanup.splice(0).forEach((fn) => fn());
  });

  it("returns fail listen-bind when the TCP port is already in use", async () => {
    const port = await getFreeTcpPort();
    const occupying = net.createServer();
    await new Promise<void>((resolve) => occupying.listen(port, "127.0.0.1", resolve));
    cleanup.push(() => occupying.close());

    const result = await diagnoseRule(tcpRule({ listenPort: port }), false);
    const bind = result.checks.find((c) => c.id === "listen-bind");
    expect(bind?.status).toBe("fail");
    expect(bind?.message).toContain("failed");
  });

  it("returns pass target-connect when the target accepts a connection", async () => {
    const [listenPort, targetPort] = await Promise.all([getFreeTcpPort(), getFreeTcpPort()]);
    const target = net.createServer();
    await new Promise<void>((resolve) => target.listen(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => target.close());

    const result = await diagnoseRule(tcpRule({ listenPort, targetPort }), false);
    const host = result.checks.find((c) => c.id === "target-host");
    const connect = result.checks.find((c) => c.id === "target-connect");
    expect(host?.status).toBe("pass");
    expect(connect?.status).toBe("pass");
    expect(connect?.message).toContain("succeeded");
  });

  it("returns fail target-connect when the target refuses the connection", async () => {
    const [listenPort, targetPort] = await Promise.all([getFreeTcpPort(), getFreeTcpPort()]);
    // targetPort is allocated but nothing listens → connection refused.
    const result = await diagnoseRule(tcpRule({ listenPort, targetPort }), false);
    const connect = result.checks.find((c) => c.id === "target-connect");
    expect(connect?.status).toBe("fail");
    expect(connect?.message).toContain("failed");
  });

  it("fails target-host and skips target-connect when the host does not resolve", async () => {
    const listenPort = await getFreeTcpPort();
    const result = await diagnoseRule(
      tcpRule({ listenPort, targetHost: "portier-nonexistent.invalid" }),
      false
    );
    const host = result.checks.find((c) => c.id === "target-host");
    const connect = result.checks.find((c) => c.id === "target-connect");
    expect(host?.status).toBe("fail");
    expect(host?.message).toContain("could not be resolved");
    expect(connect?.status).toBe("skip");
    expect(result.summary.status).toBe("fail");
  });
});

describe("diagnoseRule advisory checks", () => {
  // UDP rules skip the target-connect check, isolating the advisory checks from
  // target reachability (so the summary reflects only the advisory warnings).
  it("warns on listen-host, lan-exposure for 0.0.0.0", async () => {
    const result = await diagnoseRule(udpRule({ listenHost: "0.0.0.0", listenPort: 49321 }), true);
    expect(result.checks.find((c) => c.id === "listen-host")?.status).toBe("warn");
    expect(result.checks.find((c) => c.id === "lan-exposure")?.status).toBe("warn");
    expect(result.summary.status).toBe("warn");
  });

  it("warns on privileged-port and common-port for port 80", async () => {
    const result = await diagnoseRule(udpRule({ listenPort: 80 }), true);
    expect(result.checks.find((c) => c.id === "privileged-port")?.status).toBe("warn");
    expect(result.checks.find((c) => c.id === "common-port")?.status).toBe("warn");
  });
});

describe("diagnoseRule check ordering", () => {
  // Locks the exact check order/set (previously unguarded — other tests find
  // checks by id). isRunning=true skips the listen-bind socket; the order is the
  // same regardless of individual check statuses.
  it("emits TCP checks in a fixed order (no udp-mode)", async () => {
    const result = await diagnoseRule(tcpRule(), true);
    expect(result.checks.map((c) => c.id)).toEqual([
      "listen-host",
      "lan-exposure",
      "privileged-port",
      "common-port",
      "listen-bind",
      "target-host",
      "target-connect"
    ]);
  });

  it("emits UDP checks in a fixed order (udp-mode last)", async () => {
    const result = await diagnoseRule(udpRule(), true);
    expect(result.checks.map((c) => c.id)).toEqual([
      "listen-host",
      "lan-exposure",
      "privileged-port",
      "common-port",
      "listen-bind",
      "target-host",
      "target-connect",
      "udp-mode"
    ]);
  });
});
