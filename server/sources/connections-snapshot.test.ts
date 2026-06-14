import type { ForwardRule, TcpConnectionInfo, UdpSessionInfo } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { buildLiveConnections } from "./connections-snapshot.js";

const NOW = new Date("2026-06-14T12:00:00.000Z");

function rule(id: string, overrides: Partial<ForwardRule> = {}): ForwardRule {
  return {
    id,
    name: `Rule ${id}`,
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort: 48010,
    targetHost: "127.0.0.1",
    targetPort: 8080,
    enabled: false,
    ...overrides,
  };
}

function tcp(ruleId: string, overrides: Partial<TcpConnectionInfo> = {}): TcpConnectionInfo {
  return {
    id: `tcp-${ruleId}-${overrides.startedAt ?? "x"}`,
    ruleId,
    ruleName: `Rule ${ruleId}`,
    protocol: "tcp",
    clientAddress: "10.0.0.1",
    clientPort: 5000,
    targetAddress: "127.0.0.1",
    targetPort: 8080,
    startedAt: "2026-06-14T11:00:00.000Z",
    durationMs: 1000,
    bytesIn: 10,
    bytesOut: 20,
    status: "active",
    ...overrides,
  };
}

function udp(ruleId: string, overrides: Partial<UdpSessionInfo> = {}): UdpSessionInfo {
  return {
    id: `udp-${ruleId}-${overrides.lastSeenAt ?? "x"}`,
    ruleId,
    ruleName: `Rule ${ruleId}`,
    protocol: "udp",
    mode: "one-way",
    clientAddress: "10.0.0.2",
    clientPort: 6000,
    targetAddress: "127.0.0.1",
    targetPort: 53,
    startedAt: "2026-06-14T11:00:00.000Z",
    lastSeenAt: "2026-06-14T11:30:00.000Z",
    idleMs: 500,
    packetsIn: 3,
    packetsOut: 4,
    bytesIn: 5,
    bytesOut: 6,
    status: "active",
    ...overrides,
  };
}

describe("buildLiveConnections", () => {
  it("returns an empty snapshot (generatedAt from the clock; no rules/connections)", () => {
    expect(buildLiveConnections({ rules: [], tcpConnections: [], udpSessions: [], now: NOW })).toEqual({
      generatedAt: "2026-06-14T12:00:00.000Z",
      tcpConnections: [],
      udpSessions: [],
      ruleSummaries: [],
    });
  });

  it("passes the tcp/udp arrays straight through (historical Express behavior)", () => {
    const tcpConnections = [tcp("r1")];
    const udpSessions = [udp("r1")];
    const result = buildLiveConnections({ rules: [], tcpConnections, udpSessions, now: NOW });
    expect(result.tcpConnections).toBe(tcpConnections);
    expect(result.udpSessions).toBe(udpSessions);
  });

  it("summarizes a TCP-only rule, picking the latest startedAt as lastTrafficAt", () => {
    const result = buildLiveConnections({
      rules: [rule("r1")],
      tcpConnections: [
        tcp("r1", { startedAt: "2026-06-14T10:00:00.000Z", bytesIn: 1, bytesOut: 2 }),
        tcp("r1", { startedAt: "2026-06-14T11:00:00.000Z", bytesIn: 3, bytesOut: 4 }),
      ],
      udpSessions: [],
      now: NOW,
    });
    expect(result.ruleSummaries).toEqual([
      {
        ruleId: "r1",
        ruleName: "Rule r1",
        protocol: "tcp",
        activeTcpConnections: 2,
        activeUdpSessions: 0,
        bytesIn: 4,
        bytesOut: 6,
        packetsIn: 0,
        packetsOut: 0,
        lastTrafficAt: "2026-06-14T11:00:00.000Z",
      },
    ]);
  });

  it("summarizes a UDP-only rule (lastTrafficAt set from the latest lastSeenAt)", () => {
    const result = buildLiveConnections({
      rules: [rule("r1", { protocol: "udp" })],
      tcpConnections: [],
      udpSessions: [
        udp("r1", { lastSeenAt: "2026-06-14T11:10:00.000Z", packetsIn: 1, packetsOut: 1, bytesIn: 5, bytesOut: 6 }),
        udp("r1", { lastSeenAt: "2026-06-14T11:40:00.000Z", packetsIn: 2, packetsOut: 3, bytesIn: 7, bytesOut: 8 }),
      ],
      now: NOW,
    });
    expect(result.ruleSummaries[0]).toMatchObject({
      activeTcpConnections: 0,
      activeUdpSessions: 2,
      bytesIn: 12,
      bytesOut: 14,
      packetsIn: 3,
      packetsOut: 4,
      lastTrafficAt: "2026-06-14T11:40:00.000Z",
    });
  });

  it("prefers the later UDP lastSeenAt over the TCP startedAt when both exist", () => {
    const result = buildLiveConnections({
      rules: [rule("r1")],
      tcpConnections: [tcp("r1", { startedAt: "2026-06-14T11:00:00.000Z" })],
      udpSessions: [udp("r1", { lastSeenAt: "2026-06-14T11:45:00.000Z" })],
      now: NOW,
    });
    expect(result.ruleSummaries[0].lastTrafficAt).toBe("2026-06-14T11:45:00.000Z");
  });

  it("keeps the TCP startedAt when it is later than the UDP lastSeenAt", () => {
    const result = buildLiveConnections({
      rules: [rule("r1")],
      tcpConnections: [tcp("r1", { startedAt: "2026-06-14T11:50:00.000Z" })],
      udpSessions: [udp("r1", { lastSeenAt: "2026-06-14T11:20:00.000Z" })],
      now: NOW,
    });
    expect(result.ruleSummaries[0].lastTrafficAt).toBe("2026-06-14T11:50:00.000Z");
  });

  it("picks the latest timestamps regardless of input order (both comparator arms)", () => {
    // Descending input order — the sort comparators hit their other arm vs the
    // ascending cases above, so both ternary branches are covered.
    const result = buildLiveConnections({
      rules: [rule("r1")],
      tcpConnections: [
        tcp("r1", { startedAt: "2026-06-14T11:30:00.000Z" }),
        tcp("r1", { startedAt: "2026-06-14T11:00:00.000Z" }),
      ],
      udpSessions: [
        udp("r1", { lastSeenAt: "2026-06-14T11:40:00.000Z" }),
        udp("r1", { lastSeenAt: "2026-06-14T11:10:00.000Z" }),
      ],
      now: NOW,
    });
    // latest tcp startedAt = 11:30; latest udp lastSeenAt = 11:40 → udp wins.
    expect(result.ruleSummaries[0].lastTrafficAt).toBe("2026-06-14T11:40:00.000Z");
  });

  it("reports zeros and null lastTrafficAt for a rule with no traffic", () => {
    const result = buildLiveConnections({
      rules: [rule("idle")],
      tcpConnections: [tcp("other")],
      udpSessions: [udp("other")],
      now: NOW,
    });
    expect(result.ruleSummaries[0]).toEqual({
      ruleId: "idle",
      ruleName: "Rule idle",
      protocol: "tcp",
      activeTcpConnections: 0,
      activeUdpSessions: 0,
      bytesIn: 0,
      bytesOut: 0,
      packetsIn: 0,
      packetsOut: 0,
      lastTrafficAt: null,
    });
  });

  it("preserves the contract field order in JSON serialization", () => {
    const json = JSON.stringify(
      buildLiveConnections({
        rules: [rule("r1")],
        tcpConnections: [tcp("r1")],
        udpSessions: [],
        now: NOW,
      })
    );
    const parsed = JSON.parse(json) as Record<string, unknown>;
    expect(Object.keys(parsed)).toEqual([
      "generatedAt",
      "tcpConnections",
      "udpSessions",
      "ruleSummaries",
    ]);
    expect(Object.keys((parsed.ruleSummaries as Record<string, unknown>[])[0])).toEqual([
      "ruleId",
      "ruleName",
      "protocol",
      "activeTcpConnections",
      "activeUdpSessions",
      "bytesIn",
      "bytesOut",
      "packetsIn",
      "packetsOut",
      "lastTrafficAt",
    ]);
  });
});
