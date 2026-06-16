import "reflect-metadata";
import type http from "node:http";
import type { INestApplication } from "@nestjs/common";
import { Test } from "@nestjs/testing";
import type {
  ForwardRule,
  LiveConnectionsResponse,
  TcpConnectionInfo,
  UdpSessionInfo,
} from "@portier/shared";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { createApp } from "../../../legacy/api.js";
import { ForwardManager, type RuleStore } from "../../../forward-manager.js";
import { AppModule } from "../../app.module.js";
import { createNestApp } from "../../app.factory.js";
import { CLOCK_READER, type ClockReader } from "../../common/clock.reader.js";
import {
  diffApiResponses,
  fetchApi,
  startHandlerServer,
  startNestServer,
  type ParityServer,
} from "../../testing/api-parity.js";
import { CONNECTIONS_READER, type ConnectionsReader } from "./connections.reader.js";

class MemoryStore implements RuleStore {
  async load(): Promise<ForwardRule[]> {
    return [];
  }
  async save(): Promise<void> {
    /* no-op */
  }
}

// Fixed clock shared by both runtimes so `generatedAt` is deterministic.
const FIXED_NOW = new Date("2026-06-14T12:00:30.000Z");

// Deterministic seeded snapshot data — NO sockets/registries involved.
const RULES: ForwardRule[] = [
  { id: "r-tcp", name: "Web", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48010, targetHost: "127.0.0.1", targetPort: 8080, enabled: false },
  { id: "r-udp", name: "DNS", protocol: "udp", listenHost: "127.0.0.1", listenPort: 48011, targetHost: "127.0.0.1", targetPort: 53, enabled: false, udpMode: "one-way" },
];
const TCP: TcpConnectionInfo[] = [
  { id: "c1", ruleId: "r-tcp", ruleName: "Web", protocol: "tcp", clientAddress: "10.0.0.1", clientPort: 5000, targetAddress: "127.0.0.1", targetPort: 8080, startedAt: "2026-06-14T11:00:00.000Z", durationMs: 1000, bytesIn: 10, bytesOut: 20, status: "active" },
];
const UDP: UdpSessionInfo[] = [
  { id: "s1", ruleId: "r-udp", ruleName: "DNS", protocol: "udp", mode: "one-way", clientAddress: "10.0.0.2", clientPort: 6000, targetAddress: "127.0.0.1", targetPort: 53, startedAt: "2026-06-14T11:00:00.000Z", lastSeenAt: "2026-06-14T11:30:00.000Z", idleMs: 500, packetsIn: 3, packetsOut: 4, bytesIn: 5, bytesOut: 6, status: "active" },
];

const seededReader: ConnectionsReader = {
  listRules: () => RULES,
  getLiveTcpConnections: () => TCP,
  getLiveUdpSessions: () => UDP,
};

/**
 * Express-side seam: a ForwardManager whose connection-snapshot reads are the
 * seeded data (NO sockets/registries). Only the three methods the connections
 * route calls are overridden.
 */
class SeededManager extends ForwardManager {
  override listRules(): ForwardRule[] {
    return seededReader.listRules();
  }
  override getLiveTcpConnections(): TcpConnectionInfo[] {
    return seededReader.getLiveTcpConnections();
  }
  override getLiveUdpSessions(): UdpSessionInfo[] {
    return seededReader.getLiveUdpSessions();
  }
}

/** Builds a Nest app whose clock is pinned and whose connections reader is the given fake. */
async function nestWithFixedClock(reader: ConnectionsReader): Promise<INestApplication> {
  const fixedClock: ClockReader = { now: () => FIXED_NOW };
  const moduleRef = await Test.createTestingModule({ imports: [AppModule] })
    .overrideProvider(CLOCK_READER)
    .useValue(fixedClock)
    .overrideProvider(CONNECTIONS_READER)
    .useValue(reader)
    .compile();
  return moduleRef.createNestApplication({ logger: false });
}

describe("GET /api/connections (Nest) — default app", () => {
  let nestApp: INestApplication;
  let nest: ParityServer;

  beforeAll(async () => {
    nestApp = await createNestApp();
    nest = await startNestServer(nestApp);
  });

  afterAll(async () => {
    await nest.close();
  });

  it("returns an empty snapshot with a live ISO generatedAt (no runtime wired)", async () => {
    const response = await fetchApi(nest.baseUrl, "/api/connections");
    expect(response.status).toBe(200);

    const body = response.body as LiveConnectionsResponse;
    expect(body.tcpConnections).toEqual([]);
    expect(body.udpSessions).toEqual([]);
    expect(body.ruleSummaries).toEqual([]);
    expect(typeof body.generatedAt).toBe("string");
    expect(new Date(body.generatedAt).toISOString()).toBe(body.generatedAt);
  });

  it("keeps /health, the /api/* 404 envelope, and non-API 404 behavior intact", async () => {
    expect(await fetchApi(nest.baseUrl, "/health")).toEqual({
      status: 200,
      body: { ok: true, server: "node", name: "Portier" },
    });
    expect(await fetchApi(nest.baseUrl, "/api/not-migrated")).toEqual({
      status: 404,
      body: { errors: ["API route was not found."] },
    });
    const nonApi = await fetchApi(nest.baseUrl, "/not-a-page");
    expect(nonApi.status).toBe(404);
    expect(nonApi.body).not.toHaveProperty("errors");
  });
});

describe("GET /api/connections (Nest) — parity with Express", () => {
  it("matches Express byte-for-byte for an empty runtime", async () => {
    const emptyReader: ConnectionsReader = {
      listRules: () => [],
      getLiveTcpConnections: () => [],
      getLiveUdpSessions: () => [],
    };
    const nestApp = await nestWithFixedClock(emptyReader);
    const nest = await startNestServer(nestApp);
    const express = await startHandlerServer(
      createApp(new ForwardManager(new MemoryStore()), { now: () => FIXED_NOW }) as unknown as http.RequestListener
    );
    try {
      const [expressResponse, nestResponse] = await Promise.all([
        fetchApi(express.baseUrl, "/api/connections"),
        fetchApi(nest.baseUrl, "/api/connections"),
      ]);
      expect(diffApiResponses(expressResponse, nestResponse)).toEqual([]);
      expect(nestResponse.body).toEqual({
        generatedAt: "2026-06-14T12:00:30.000Z",
        tcpConnections: [],
        udpSessions: [],
        ruleSummaries: [],
      });
    } finally {
      await nest.close();
      await express.close();
    }
  });

  it("matches Express byte-for-byte for a seeded snapshot (no sockets)", async () => {
    const nestApp = await nestWithFixedClock(seededReader);
    const nest = await startNestServer(nestApp);
    const express = await startHandlerServer(
      createApp(new SeededManager(new MemoryStore()), { now: () => FIXED_NOW }) as unknown as http.RequestListener
    );
    try {
      const [expressResponse, nestResponse] = await Promise.all([
        fetchApi(express.baseUrl, "/api/connections"),
        fetchApi(nest.baseUrl, "/api/connections"),
      ]);
      expect(diffApiResponses(expressResponse, nestResponse)).toEqual([]);
      const body = nestResponse.body as LiveConnectionsResponse;
      expect(body.generatedAt).toBe("2026-06-14T12:00:30.000Z");
      expect(body.tcpConnections).toEqual(TCP);
      expect(body.udpSessions).toEqual(UDP);
      expect(body.ruleSummaries).toEqual([
        { ruleId: "r-tcp", ruleName: "Web", protocol: "tcp", activeTcpConnections: 1, activeUdpSessions: 0, bytesIn: 10, bytesOut: 20, packetsIn: 0, packetsOut: 0, lastTrafficAt: "2026-06-14T11:00:00.000Z" },
        { ruleId: "r-udp", ruleName: "DNS", protocol: "udp", activeTcpConnections: 0, activeUdpSessions: 1, bytesIn: 5, bytesOut: 6, packetsIn: 3, packetsOut: 4, lastTrafficAt: "2026-06-14T11:30:00.000Z" },
      ]);
    } finally {
      await nest.close();
      await express.close();
    }
  });
});
