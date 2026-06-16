import "reflect-metadata";
import type http from "node:http";
import type { INestApplication } from "@nestjs/common";
import { Test } from "@nestjs/testing";
import type { ExportedConfig, ForwardRule } from "@portier/shared";
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
import { CONFIG_EXPORT_READER, type ConfigExportReader } from "./config-export.reader.js";

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

// Fixed clock shared by both runtimes so `exportedAt` is deterministic.
const FIXED_NOW = new Date("2026-06-14T12:00:30.000Z");

/**
 * Builds a Nest app whose clock is pinned and whose config-export reader is the
 * given (seeded) manager — so both volatile inputs (`exportedAt` clock, rules)
 * match the Express app exactly. No field is normalized before comparison.
 */
async function nestWithFixedClock(reader: ConfigExportReader): Promise<INestApplication> {
  const fixedClock: ClockReader = { now: () => FIXED_NOW };
  const moduleRef = await Test.createTestingModule({ imports: [AppModule] })
    .overrideProvider(CLOCK_READER)
    .useValue(fixedClock)
    .overrideProvider(CONFIG_EXPORT_READER)
    .useValue(reader)
    .compile();
  return moduleRef.createNestApplication({ logger: false });
}

/** Boots Nest (fixed clock + shared manager) and Express (same manager + same clock) for parity. */
async function bootPair(
  manager: ForwardManager
): Promise<{ nest: ParityServer; express: ParityServer; close: () => Promise<void> }> {
  const nestApp = await nestWithFixedClock(manager);
  const nest = await startNestServer(nestApp);
  const express = await startHandlerServer(
    createApp(manager, { now: () => FIXED_NOW }) as unknown as http.RequestListener
  );
  return {
    nest,
    express,
    close: async () => {
      await nest.close();
      await express.close();
    },
  };
}

describe("GET /api/config/export (Nest) — default app", () => {
  let nestApp: INestApplication;
  let nest: ParityServer;

  beforeAll(async () => {
    nestApp = await createNestApp();
    nest = await startNestServer(nestApp);
  });

  afterAll(async () => {
    await nest.close();
  });

  it("returns an empty export with a live ISO exportedAt (no runtime wired)", async () => {
    const response = await fetchApi(nest.baseUrl, "/api/config/export");
    expect(response.status).toBe(200);

    const body = response.body as ExportedConfig;
    expect(body.version).toBe("1");
    expect(body.rules).toEqual([]);
    expect(typeof body.exportedAt).toBe("string");
    expect(new Date(body.exportedAt).toISOString()).toBe(body.exportedAt);
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

describe("GET /api/config/export (Nest) — parity with Express", () => {
  it("matches Express byte-for-byte for an empty manager", async () => {
    const { nest, express, close } = await bootPair(new ForwardManager(new MemoryStore()));
    try {
      const [expressResponse, nestResponse] = await Promise.all([
        fetchApi(express.baseUrl, "/api/config/export"),
        fetchApi(nest.baseUrl, "/api/config/export"),
      ]);
      expect(diffApiResponses(expressResponse, nestResponse)).toEqual([]);
      expect(nestResponse.status).toBe(200);
      expect(nestResponse.body).toEqual({
        version: "1",
        exportedAt: "2026-06-14T12:00:30.000Z",
        rules: [],
      });
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
        fetchApi(express.baseUrl, "/api/config/export"),
        fetchApi(nest.baseUrl, "/api/config/export"),
      ]);
      expect(diffApiResponses(expressResponse, nestResponse)).toEqual([]);
      // Deterministic content: pinned clock + the manager's own rules.
      expect(nestResponse.status).toBe(200);
      expect(nestResponse.body).toEqual({
        version: "1",
        exportedAt: "2026-06-14T12:00:30.000Z",
        rules: manager.listRules(),
      });
      expect((nestResponse.body as ExportedConfig).rules.length).toBe(2);
    } finally {
      await close();
    }
  });
});
