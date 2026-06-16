import "reflect-metadata";
import type http from "node:http";
import type { INestApplication } from "@nestjs/common";
import { Test } from "@nestjs/testing";
import type { ExportedConfig, ForwardRule, ForwardRuleResponse, ImportResult } from "@portier/shared";
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
  type ApiResponse,
  type ParityServer,
} from "../../testing/api-parity.js";
import { FORWARDS_READER } from "../forwards/forwards.reader.js";
import { CONFIG_EXPORT_READER } from "./config-export.reader.js";
import { CONFIG_IMPORTER } from "./config-import.writer.js";

class MemoryStore implements RuleStore {
  constructor(private rules: ForwardRule[] = []) {}
  async load(): Promise<ForwardRule[]> {
    return this.rules;
  }
  async save(rules: ForwardRule[]): Promise<void> {
    this.rules = rules;
  }
}

const FIXED_NOW = new Date("2026-06-14T12:00:30.000Z");

// Fixed-id, enabled:false rules → importConfig preserves the ids and starts no
// forwarder, so the import is deterministic and socket-free.
const RULE_A = { id: "a", name: "Web", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48010, targetHost: "127.0.0.1", targetPort: 8080, enabled: false } as const;
const RULE_B = { id: "b", name: "DNS", protocol: "udp", listenHost: "127.0.0.1", listenPort: 48011, targetHost: "127.0.0.1", targetPort: 53, enabled: false, udpMode: "one-way" } as const;

function config(rules: readonly ForwardRule[]): ExportedConfig {
  return { version: "1", exportedAt: "2026-06-14T00:00:00.000Z", rules: rules as ForwardRule[] };
}

function importReq(baseUrl: string, body: unknown): Promise<ApiResponse> {
  return fetchApi(baseUrl, "/api/config/import", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** Nest app: the (seeded) manager bound to the importer + forwards/export readers + a pinned clock. */
async function nestWithManager(manager: ForwardManager): Promise<INestApplication> {
  const fixedClock: ClockReader = { now: () => FIXED_NOW };
  const moduleRef = await Test.createTestingModule({ imports: [AppModule] })
    .overrideProvider(CLOCK_READER)
    .useValue(fixedClock)
    .overrideProvider(CONFIG_IMPORTER)
    .useValue(manager)
    .overrideProvider(FORWARDS_READER)
    .useValue(manager)
    .overrideProvider(CONFIG_EXPORT_READER)
    .useValue(manager)
    .compile();
  return moduleRef.createNestApplication({ logger: false });
}

/** Separate-but-equivalent managers per runtime (import mutates, so a shared manager would cross-contaminate). */
async function bootPair(
  seed?: (m: ForwardManager) => Promise<void>
): Promise<{ nest: ParityServer; express: ParityServer; close: () => Promise<void> }> {
  const expressManager = new ForwardManager(new MemoryStore());
  const nestManager = new ForwardManager(new MemoryStore());
  if (seed) {
    await seed(expressManager);
    await seed(nestManager);
  }
  const nestApp = await nestWithManager(nestManager);
  const nest = await startNestServer(nestApp);
  const express = await startHandlerServer(
    createApp(expressManager, { now: () => FIXED_NOW }) as unknown as http.RequestListener
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

describe("POST /api/config/import (Nest) — default app", () => {
  let nestApp: INestApplication;
  let nest: ParityServer;

  beforeAll(async () => {
    nestApp = await createNestApp();
    nest = await startNestServer(nestApp);
  });

  afterAll(async () => {
    await nest.close();
  });

  it("returns 400 for an invalid mode", async () => {
    expect(await importReq(nest.baseUrl, { mode: "wipe", config: config([]) })).toEqual({
      status: 400,
      body: { errors: ["mode must be replace or merge."] },
    });
  });

  it("imports into the empty default config (no runtime wired)", async () => {
    const response = await importReq(nest.baseUrl, { mode: "replace", config: config([RULE_A]) });
    expect(response.status).toBe(200);
    const body = response.body as { result: ImportResult; rules: ForwardRuleResponse[] };
    expect(body.result).toEqual({ imported: 1, skipped: 0, errors: [] });
    expect(body.rules.map((r) => r.id)).toEqual(["a"]);
  });
});

describe("POST /api/config/import (Nest) — parity with Express", () => {
  it("returns the same 400 envelope as Express for an invalid mode", async () => {
    const { nest, express, close } = await bootPair();
    try {
      const body = { mode: "wipe", config: config([]) };
      const [e, n] = await Promise.all([importReq(express.baseUrl, body), importReq(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n).toEqual({ status: 400, body: { errors: ["mode must be replace or merge."] } });
    } finally {
      await close();
    }
  });

  it("returns the same 400 envelope as Express for an invalid config (valid mode)", async () => {
    const { nest, express, close } = await bootPair();
    try {
      const body = { mode: "replace", config: { version: "2", rules: [] } };
      const [e, n] = await Promise.all([importReq(express.baseUrl, body), importReq(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(400);
      expect((n.body as { errors: string[] }).errors[0]).toContain("config must be a valid Portier config");
    } finally {
      await close();
    }
  });

  it("imports (replace) into empty state byte-for-byte like Express, and GET /api/forwards + export reflect it", async () => {
    const { nest, express, close } = await bootPair();
    try {
      const body = { mode: "replace", config: config([RULE_A, RULE_B]) };
      const [e, n] = await Promise.all([importReq(express.baseUrl, body), importReq(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const result = (n.body as { result: ImportResult }).result;
      expect(result).toEqual({ imported: 2, skipped: 0, errors: [] });
      // State after import matches byte-for-byte in both runtimes.
      const [ef, nf] = await Promise.all([fetchApi(express.baseUrl, "/api/forwards"), fetchApi(nest.baseUrl, "/api/forwards")]);
      expect(diffApiResponses(ef, nf)).toEqual([]);
      expect((nf.body as ForwardRuleResponse[]).map((r) => r.id)).toEqual(["a", "b"]);
      const [ex, nx] = await Promise.all([fetchApi(express.baseUrl, "/api/config/export"), fetchApi(nest.baseUrl, "/api/config/export")]);
      expect(diffApiResponses(ex, nx)).toEqual([]);
    } finally {
      await close();
    }
  });

  it("imports (replace) OVER seeded state byte-for-byte like Express", async () => {
    const seed = async (m: ForwardManager): Promise<void> => {
      await m.addRule({ id: "old", name: "Old", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48099, targetHost: "127.0.0.1", targetPort: 9000, enabled: false });
    };
    const { nest, express, close } = await bootPair(seed);
    try {
      const body = { mode: "replace", config: config([RULE_A]) };
      const [e, n] = await Promise.all([importReq(express.baseUrl, body), importReq(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      // The old rule is replaced.
      const [ef, nf] = await Promise.all([fetchApi(express.baseUrl, "/api/forwards"), fetchApi(nest.baseUrl, "/api/forwards")]);
      expect(diffApiResponses(ef, nf)).toEqual([]);
      expect((nf.body as ForwardRuleResponse[]).map((r) => r.id)).toEqual(["a"]);
    } finally {
      await close();
    }
  });

  it("returns the same 422 (errors + result) as Express for an invalid desired rule — NO mutation", async () => {
    const { nest, express, close } = await bootPair();
    try {
      const bad = { id: "bad", name: "Bad", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 70000, targetHost: "127.0.0.1", targetPort: 8080, enabled: false } as unknown as ForwardRule;
      const body = { mode: "replace", config: config([bad]) };
      const [e, n] = await Promise.all([importReq(express.baseUrl, body), importReq(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(422);
      const errBody = n.body as { errors: string[]; result: ImportResult };
      expect(errBody.errors.length).toBeGreaterThan(0);
      expect(errBody.result).toEqual({ imported: 0, skipped: 0, errors: errBody.errors });
      // No mutation — the rules are still empty in both runtimes.
      const [ef, nf] = await Promise.all([fetchApi(express.baseUrl, "/api/forwards"), fetchApi(nest.baseUrl, "/api/forwards")]);
      expect(diffApiResponses(ef, nf)).toEqual([]);
      expect(nf.body).toEqual([]);
    } finally {
      await close();
    }
  });

  it("returns the same 422 as Express for a duplicate listen binding within the imported set", async () => {
    const { nest, express, close } = await bootPair();
    try {
      const dup = { ...RULE_A, id: "a2", name: "Dup" }; // same protocol+host+port as RULE_A
      const body = { mode: "replace", config: config([RULE_A, dup]) };
      const [e, n] = await Promise.all([importReq(express.baseUrl, body), importReq(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(422);
      expect((n.body as { errors: string[] }).errors.length).toBeGreaterThan(0);
    } finally {
      await close();
    }
  });
});
