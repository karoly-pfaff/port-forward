import { describe, expect, it } from "vitest";
import type {
  LiveConnectionStatus,
  LiveConnectionsResponse,
  RuleLiveSummary,
  TcpConnectionInfo,
  UdpSessionInfo,
  UdpSessionStatus
} from "./connections.js";

describe("LiveConnectionsResponse shape", () => {
  it("accepts a minimal empty response", () => {
    const response: LiveConnectionsResponse = {
      generatedAt: "2026-06-08T12:00:00.000Z",
      tcpConnections: [],
      udpSessions: [],
      ruleSummaries: []
    };
    expect(response.generatedAt).toBe("2026-06-08T12:00:00.000Z");
    expect(response.tcpConnections).toEqual([]);
    expect(response.udpSessions).toEqual([]);
    expect(response.ruleSummaries).toEqual([]);
  });

  it("requires all four top-level fields", () => {
    const response: LiveConnectionsResponse = {
      generatedAt: "2026-06-08T12:00:00.000Z",
      tcpConnections: [],
      udpSessions: [],
      ruleSummaries: []
    };
    expect("generatedAt" in response).toBe(true);
    expect("tcpConnections" in response).toBe(true);
    expect("udpSessions" in response).toBe(true);
    expect("ruleSummaries" in response).toBe(true);
  });
});

describe("TcpConnectionInfo shape", () => {
  it("accepts a valid TCP connection record", () => {
    const conn: TcpConnectionInfo = {
      id: "tcp-rule-abc-1",
      ruleId: "rule-abc",
      ruleName: "Postgres forward",
      protocol: "tcp",
      clientAddress: "127.0.0.1",
      clientPort: 54321,
      targetAddress: "127.0.0.1",
      targetPort: 5432,
      startedAt: "2026-06-08T12:00:00.000Z",
      durationMs: 12000,
      bytesIn: 1024,
      bytesOut: 2048,
      status: "active"
    };
    expect(conn.protocol).toBe("tcp");
    expect(conn.status).toBe("active");
    expect(conn.bytesIn).toBe(1024);
    expect(conn.bytesOut).toBe(2048);
    expect(conn.durationMs).toBe(12000);
  });

  it("uses empty string for unknown ruleName", () => {
    const conn: TcpConnectionInfo = {
      id: "tcp-rule-xyz-1",
      ruleId: "rule-xyz",
      ruleName: "",
      protocol: "tcp",
      clientAddress: "192.168.1.5",
      clientPort: 49000,
      targetAddress: "10.0.0.1",
      targetPort: 8080,
      startedAt: "2026-06-08T12:00:00.000Z",
      durationMs: 0,
      bytesIn: 0,
      bytesOut: 0,
      status: "active"
    };
    expect(conn.ruleName).toBe("");
  });

  it("LiveConnectionStatus only allows 'active'", () => {
    const status: LiveConnectionStatus = "active";
    expect(status).toBe("active");
  });
});

describe("UdpSessionInfo shape", () => {
  it("accepts a valid UDP session record with one-way mode", () => {
    const session: UdpSessionInfo = {
      id: "udp-rule-def-127.0.0.1:53000",
      ruleId: "rule-def",
      ruleName: "DNS forward",
      protocol: "udp",
      mode: "one-way",
      clientAddress: "127.0.0.1",
      clientPort: 53000,
      targetAddress: "1.1.1.1",
      targetPort: 53,
      startedAt: "2026-06-08T12:00:00.000Z",
      lastSeenAt: "2026-06-08T12:00:05.000Z",
      idleMs: 5000,
      packetsIn: 10,
      packetsOut: 8,
      bytesIn: 1200,
      bytesOut: 900,
      status: "active"
    };
    expect(session.protocol).toBe("udp");
    expect(session.mode).toBe("one-way");
    expect(session.status).toBe("active");
    expect(session.packetsIn).toBe(10);
    expect(session.packetsOut).toBe(8);
  });

  it("accepts bidirectional-last-client mode", () => {
    const session: UdpSessionInfo = {
      id: "udp-rule-ghi-1",
      ruleId: "rule-ghi",
      ruleName: "Game server",
      protocol: "udp",
      mode: "bidirectional-last-client",
      clientAddress: "192.168.1.10",
      clientPort: 27015,
      targetAddress: "10.0.0.5",
      targetPort: 27015,
      startedAt: "2026-06-08T12:00:00.000Z",
      lastSeenAt: "2026-06-08T12:00:01.000Z",
      idleMs: 1000,
      packetsIn: 50,
      packetsOut: 48,
      bytesIn: 6000,
      bytesOut: 5800,
      status: "idle"
    };
    expect(session.mode).toBe("bidirectional-last-client");
    expect(session.status).toBe("idle");
  });

  it("accepts bidirectional-multi-client mode", () => {
    const session: UdpSessionInfo = {
      id: "udp-rule-jkl-1",
      ruleId: "rule-jkl",
      ruleName: "Multi game",
      protocol: "udp",
      mode: "bidirectional-multi-client",
      clientAddress: "192.168.1.20",
      clientPort: 19132,
      targetAddress: "10.0.0.6",
      targetPort: 19132,
      startedAt: "2026-06-08T12:00:00.000Z",
      lastSeenAt: "2026-06-08T12:00:02.000Z",
      idleMs: 2000,
      packetsIn: 100,
      packetsOut: 100,
      bytesIn: 10000,
      bytesOut: 10000,
      status: "active"
    };
    expect(session.mode).toBe("bidirectional-multi-client");
  });

  it("UdpSessionStatus allows 'active' and 'idle'", () => {
    const active: UdpSessionStatus = "active";
    const idle: UdpSessionStatus = "idle";
    expect(active).toBe("active");
    expect(idle).toBe("idle");
  });
});

describe("RuleLiveSummary shape", () => {
  it("accepts a TCP rule summary with active connections", () => {
    const summary: RuleLiveSummary = {
      ruleId: "rule-abc",
      ruleName: "Postgres forward",
      protocol: "tcp",
      activeTcpConnections: 2,
      activeUdpSessions: 0,
      bytesIn: 1024,
      bytesOut: 2048,
      packetsIn: 0,
      packetsOut: 0,
      lastTrafficAt: "2026-06-08T12:00:05.000Z"
    };
    expect(summary.protocol).toBe("tcp");
    expect(summary.activeTcpConnections).toBe(2);
    expect(summary.lastTrafficAt).not.toBeNull();
  });

  it("accepts a UDP rule summary with active sessions", () => {
    const summary: RuleLiveSummary = {
      ruleId: "rule-def",
      ruleName: "DNS forward",
      protocol: "udp",
      activeTcpConnections: 0,
      activeUdpSessions: 3,
      bytesIn: 1200,
      bytesOut: 900,
      packetsIn: 18,
      packetsOut: 16,
      lastTrafficAt: "2026-06-08T12:00:05.000Z"
    };
    expect(summary.protocol).toBe("udp");
    expect(summary.activeUdpSessions).toBe(3);
    expect(summary.packetsIn).toBe(18);
  });

  it("uses null for lastTrafficAt when no traffic since start", () => {
    const summary: RuleLiveSummary = {
      ruleId: "rule-new",
      ruleName: "New rule",
      protocol: "tcp",
      activeTcpConnections: 0,
      activeUdpSessions: 0,
      bytesIn: 0,
      bytesOut: 0,
      packetsIn: 0,
      packetsOut: 0,
      lastTrafficAt: null
    };
    expect(summary.lastTrafficAt).toBeNull();
  });

  it("protocol accepts tcp and udp", () => {
    const tcp: RuleLiveSummary = {
      ruleId: "r1",
      ruleName: "TCP rule",
      protocol: "tcp",
      activeTcpConnections: 0,
      activeUdpSessions: 0,
      bytesIn: 0,
      bytesOut: 0,
      packetsIn: 0,
      packetsOut: 0,
      lastTrafficAt: null
    };
    const udp: RuleLiveSummary = {
      ruleId: "r2",
      ruleName: "UDP rule",
      protocol: "udp",
      activeTcpConnections: 0,
      activeUdpSessions: 0,
      bytesIn: 0,
      bytesOut: 0,
      packetsIn: 0,
      packetsOut: 0,
      lastTrafficAt: null
    };
    expect(tcp.protocol).toBe("tcp");
    expect(udp.protocol).toBe("udp");
  });
});

describe("LiveConnectionsResponse with connections", () => {
  it("accepts a fully populated response", () => {
    const response: LiveConnectionsResponse = {
      generatedAt: "2026-06-08T12:00:00.000Z",
      tcpConnections: [
        {
          id: "tcp-r1-1",
          ruleId: "r1",
          ruleName: "DB tunnel",
          protocol: "tcp",
          clientAddress: "127.0.0.1",
          clientPort: 54321,
          targetAddress: "127.0.0.1",
          targetPort: 5432,
          startedAt: "2026-06-08T12:00:00.000Z",
          durationMs: 12000,
          bytesIn: 1024,
          bytesOut: 2048,
          status: "active"
        }
      ],
      udpSessions: [
        {
          id: "udp-r2-127.0.0.1:53000",
          ruleId: "r2",
          ruleName: "DNS",
          protocol: "udp",
          mode: "one-way",
          clientAddress: "127.0.0.1",
          clientPort: 53000,
          targetAddress: "1.1.1.1",
          targetPort: 53,
          startedAt: "2026-06-08T12:00:00.000Z",
          lastSeenAt: "2026-06-08T12:00:05.000Z",
          idleMs: 5000,
          packetsIn: 10,
          packetsOut: 8,
          bytesIn: 1200,
          bytesOut: 900,
          status: "active"
        }
      ],
      ruleSummaries: [
        {
          ruleId: "r1",
          ruleName: "DB tunnel",
          protocol: "tcp",
          activeTcpConnections: 1,
          activeUdpSessions: 0,
          bytesIn: 1024,
          bytesOut: 2048,
          packetsIn: 0,
          packetsOut: 0,
          lastTrafficAt: "2026-06-08T12:00:05.000Z"
        }
      ]
    };
    expect(response.tcpConnections).toHaveLength(1);
    expect(response.udpSessions).toHaveLength(1);
    expect(response.ruleSummaries).toHaveLength(1);
    expect(response.tcpConnections[0].status).toBe("active");
    expect(response.udpSessions[0].mode).toBe("one-way");
    expect(response.ruleSummaries[0].activeTcpConnections).toBe(1);
  });
});
