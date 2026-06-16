import type {
  ForwardRule,
  LiveConnectionsResponse,
  RuleLiveSummary,
  TcpConnectionInfo,
  UdpSessionInfo,
} from "@portier/shared";

/**
 * Inputs for `buildLiveConnections`. The volatile timestamp source (`now`) is
 * passed in so the builder stays pure and deterministically parity-testable —
 * the caller owns the clock (e.g. `new Date()`; the NestJS service
 * passes its injected `ClockReader`). The connection/session records are
 * supplied pre-computed by the caller (the live registries), so the builder
 * never reads wall-clock-derived fields itself.
 */
export interface BuildLiveConnectionsInput {
  rules: ForwardRule[];
  tcpConnections: TcpConnectionInfo[];
  udpSessions: UdpSessionInfo[];
  now: Date;
}

/**
 * Builds the `GET /api/connections` snapshot: the raw TCP/UDP records plus a
 * per-rule live summary, stamped with `generatedAt`. Pure: identical inputs
 * always produce identical output (including field order, which JSON
 * serialization preserves), so it can be parity-tested deterministically. The
 * aggregation and field order match the documented `/api` contract;
 * the `tcpConnections`/`udpSessions` arrays are passed straight through (no copy),
 * as before. No side effects.
 */
export function buildLiveConnections(input: BuildLiveConnectionsInput): LiveConnectionsResponse {
  const { rules, tcpConnections, udpSessions, now } = input;

  const ruleSummaries: RuleLiveSummary[] = rules.map((rule) => {
    const tcpForRule = tcpConnections.filter((c) => c.ruleId === rule.id);
    const udpForRule = udpSessions.filter((s) => s.ruleId === rule.id);

    const activeTcpConnections = tcpForRule.length;
    const activeUdpSessions = udpForRule.length;
    const bytesIn = [...tcpForRule, ...udpForRule].reduce((sum, item) => sum + item.bytesIn, 0);
    const bytesOut = [...tcpForRule, ...udpForRule].reduce((sum, item) => sum + item.bytesOut, 0);
    const packetsIn = udpForRule.reduce((sum, s) => sum + s.packetsIn, 0);
    const packetsOut = udpForRule.reduce((sum, s) => sum + s.packetsOut, 0);

    let lastTrafficAt: string | null = null;
    if (tcpForRule.length > 0) {
      lastTrafficAt = [...tcpForRule].sort((a, b) => (a.startedAt < b.startedAt ? 1 : -1))[0].startedAt;
    }
    if (udpForRule.length > 0) {
      const udpLast = [...udpForRule].sort((a, b) => (a.lastSeenAt < b.lastSeenAt ? 1 : -1))[0].lastSeenAt;
      if (lastTrafficAt === null || udpLast > lastTrafficAt) {
        lastTrafficAt = udpLast;
      }
    }

    return {
      ruleId: rule.id,
      ruleName: rule.name,
      protocol: rule.protocol,
      activeTcpConnections,
      activeUdpSessions,
      bytesIn,
      bytesOut,
      packetsIn,
      packetsOut,
      lastTrafficAt,
    };
  });

  return {
    generatedAt: now.toISOString(),
    tcpConnections,
    udpSessions,
    ruleSummaries,
  };
}
