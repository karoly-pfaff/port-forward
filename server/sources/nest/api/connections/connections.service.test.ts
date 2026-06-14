import type { ForwardRule, TcpConnectionInfo, UdpSessionInfo } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ConnectionsService } from "./connections.service.js";
import type { ConnectionsReader } from "./connections.reader.js";
import type { ClockReader } from "../../common/clock.reader.js";

const NOW = new Date("2026-06-14T12:00:30.000Z");

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

const TCP: TcpConnectionInfo = {
  id: "c1",
  ruleId: "r1",
  ruleName: "Web",
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
};

function service(opts: {
  rules?: ForwardRule[];
  tcp?: TcpConnectionInfo[];
  udp?: UdpSessionInfo[];
  now?: Date;
}): ConnectionsService {
  const reader: ConnectionsReader = {
    listRules: () => opts.rules ?? [],
    getLiveTcpConnections: () => opts.tcp ?? [],
    getLiveUdpSessions: () => opts.udp ?? [],
  };
  const clock: ClockReader = { now: () => opts.now ?? NOW };
  return new ConnectionsService(reader, clock);
}

describe("ConnectionsService.get", () => {
  it("composes the reader + clock into the snapshot via the shared builder", () => {
    expect(service({ rules: [RULE], tcp: [TCP] }).get()).toEqual({
      generatedAt: "2026-06-14T12:00:30.000Z",
      tcpConnections: [TCP],
      udpSessions: [],
      ruleSummaries: [
        {
          ruleId: "r1",
          ruleName: "Web",
          protocol: "tcp",
          activeTcpConnections: 1,
          activeUdpSessions: 0,
          bytesIn: 10,
          bytesOut: 20,
          packetsIn: 0,
          packetsOut: 0,
          lastTrafficAt: "2026-06-14T11:00:00.000Z",
        },
      ],
    });
  });

  it("returns an empty snapshot when there is no runtime data", () => {
    expect(service({}).get()).toEqual({
      generatedAt: "2026-06-14T12:00:30.000Z",
      tcpConnections: [],
      udpSessions: [],
      ruleSummaries: [],
    });
  });
});
