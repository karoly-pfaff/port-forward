import type { LiveConnectionsResponse } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toConnectionsResponseDto } from "./connections.response.dto.js";

const SNAPSHOT: LiveConnectionsResponse = {
  generatedAt: "2026-06-14T12:00:30.000Z",
  tcpConnections: [
    {
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
    },
  ],
  udpSessions: [
    {
      id: "s1",
      ruleId: "r2",
      ruleName: "DNS",
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
    },
  ],
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
};

describe("toConnectionsResponseDto", () => {
  it("preserves the snapshot shape byte-for-byte without mutating the source", () => {
    const snapshot = structuredClone(SNAPSHOT);

    const dto = toConnectionsResponseDto(SNAPSHOT);

    expect(dto).toEqual(SNAPSHOT);
    expect(dto).not.toBe(SNAPSHOT); // fresh object
    expect(dto.tcpConnections).not.toBe(SNAPSHOT.tcpConnections); // fresh array
    expect(dto.tcpConnections[0]).not.toBe(SNAPSHOT.tcpConnections[0]); // fresh records
    expect(dto.udpSessions[0]).not.toBe(SNAPSHOT.udpSessions[0]);
    expect(dto.ruleSummaries[0]).not.toBe(SNAPSHOT.ruleSummaries[0]);
    expect(SNAPSHOT).toEqual(snapshot); // source untouched
  });

  it("maps an empty snapshot to empty arrays", () => {
    const empty: LiveConnectionsResponse = {
      generatedAt: "2026-06-14T12:00:30.000Z",
      tcpConnections: [],
      udpSessions: [],
      ruleSummaries: [],
    };
    expect(toConnectionsResponseDto(empty)).toEqual(empty);
  });
});
