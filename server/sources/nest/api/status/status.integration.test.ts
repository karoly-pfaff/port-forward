import "reflect-metadata";
import type http from "node:http";
import type { INestApplication } from "@nestjs/common";
import { Test } from "@nestjs/testing";
import type { ForwardRule } from "@portier/shared";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { createApp } from "../../../legacy/api.js";
import { ForwardManager, type RuleStore } from "../../../forward-manager.js";
import { AppModule } from "../../app.module.js";
import { createNestApp } from "../../app.factory.js";
import {
  diffApiResponses,
  fetchApi,
  startHandlerServer,
  startNestServer,
  type ParityServer,
} from "../../testing/api-parity.js";
import { STATUS_READER, type StatusReader } from "./status.reader.js";

/** In-memory RuleStore for the seeded manager; save() is exercised by addRule. */
class MemoryStore implements RuleStore {
  constructor(private rules: ForwardRule[] = []) {}
  async load(): Promise<ForwardRule[]> {
    return this.rules;
  }
  async save(rules: ForwardRule[]): Promise<void> {
    this.rules = rules;
  }
}

const TCP_RULE = { name: "Web", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48010, targetHost: "127.0.0.1", targetPort: 8080, enabled: false } as const;
const UDP_RULE = { name: "DNS", protocol: "udp", listenHost: "127.0.0.1", listenPort: 48011, targetHost: "127.0.0.1", targetPort: 53, enabled: false, udpMode: "one-way" } as const;

/** Builds a Nest app whose STATUS_READER is the given (seeded) manager. */
async function nestWithReader(reader: StatusReader): Promise<INestApplication> {
  const moduleRef = await Test.createTestingModule({ imports: [AppModule] })
    .overrideProvider(STATUS_READER)
    .useValue(reader)
    .compile();
  return moduleRef.createNestApplication({ logger: false });
}

/** Boots Nest (with the shared manager) and Express (with the same manager) for parity. */
async function bootPair(
  manager: ForwardManager
): Promise<{ nest: ParityServer; express: ParityServer; close: () => Promise<void> }> {
  const nestApp = await nestWithReader(manager);
  const nest = await startNestServer(nestApp);
  const express = await startHandlerServer(createApp(manager) as unknown as http.RequestListener);
  return {
    nest,
    express,
    close: async () => {
      await nest.close();
      await express.close();
    },
  };
}

describe("GET /api/status (Nest) — default app", () => {
  let nestApp: INestApplication;
  let nest: ParityServer;

  beforeAll(async () => {
    nestApp = await createNestApp();
    nest = await startNestServer(nestApp);
  });

  afterAll(async () => {
    await nest.close();
  });

  it("returns an empty status list by default (no runtime wired)", async () => {
    expect(await fetchApi(nest.baseUrl, "/api/status")).toEqual({ status: 200, body: [] });
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

describe("GET /api/status (Nest) — parity with Express", () => {
  it("matches Express for an empty manager", async () => {
    const { nest, express, close } = await bootPair(new ForwardManager(new MemoryStore()));
    try {
      const [expressResponse, nestResponse] = await Promise.all([
        fetchApi(express.baseUrl, "/api/status"),
        fetchApi(nest.baseUrl, "/api/status"),
      ]);
      expect(diffApiResponses(expressResponse, nestResponse)).toEqual([]);
      expect(nestResponse.body).toEqual([]);
    } finally {
      await close();
    }
  });

  it("matches Express byte-for-byte for seeded (stopped) rules", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule(TCP_RULE);
    await manager.addRule(UDP_RULE);
    const { nest, express, close } = await bootPair(manager);
    try {
      const [expressResponse, nestResponse] = await Promise.all([
        fetchApi(express.baseUrl, "/api/status"),
        fetchApi(nest.baseUrl, "/api/status"),
      ]);
      expect(diffApiResponses(expressResponse, nestResponse)).toEqual([]);
      // Exact, deterministic content (rules are stopped — no startedAt/counters drift).
      expect(nestResponse.status).toBe(200);
      expect(nestResponse.body).toEqual(manager.listStatus());
      expect((nestResponse.body as unknown[]).length).toBe(2);
    } finally {
      await close();
    }
  });
});
