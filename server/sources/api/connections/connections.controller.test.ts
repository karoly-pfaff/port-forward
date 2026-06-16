import type { LiveConnectionsResponse } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ConnectionsController } from "./connections.controller.js";
import type { ConnectionsService } from "./connections.service.js";

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
  udpSessions: [],
  ruleSummaries: [],
};

describe("ConnectionsController.get", () => {
  it("delegates to the service and maps the result to the response DTO", () => {
    const controller = new ConnectionsController({
      get: () => SNAPSHOT,
    } as unknown as ConnectionsService);

    const result = controller.get();

    expect(result).toEqual(SNAPSHOT); // byte-for-byte
    expect(result).not.toBe(SNAPSHOT); // mapped copy
    expect(result.tcpConnections[0]).not.toBe(SNAPSHOT.tcpConnections[0]); // records freshly copied
  });
});
