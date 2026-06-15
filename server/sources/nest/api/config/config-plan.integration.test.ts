import "reflect-metadata";
import type http from "node:http";
import type { INestApplication } from "@nestjs/common";
import { Test } from "@nestjs/testing";
import type { ConfigPlanResponse, ForwardRule } from "@portier/shared";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { createApp } from "../../../api.js";
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
import { CONFIG_PLAN_READER, type ConfigPlanReader } from "./config-plan.reader.js";

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

const FIXED_NOW = new Date("2026-06-14T12:00:30.000Z");

function planReq(baseUrl: string, body: unknown): Promise<ApiResponse> {
  return fetchApi(baseUrl, "/api/config/plan", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** Nest app with a pinned clock + the (shared, seeded) manager as the plan reader. */
async function nestWithFixedClock(reader: ConfigPlanReader): Promise<INestApplication> {
  const fixedClock: ClockReader = { now: () => FIXED_NOW };
  const moduleRef = await Test.createTestingModule({ imports: [AppModule] })
    .overrideProvider(CLOCK_READER)
    .useValue(fixedClock)
    .overrideProvider(CONFIG_PLAN_READER)
    .useValue(reader)
    .compile();
  return moduleRef.createNestApplication({ logger: false });
}

/** Boots Nest (fixed clock + shared manager) and Express (same manager + same clock). Plan is non-mutating, so sharing is safe. */
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

const TCP_RULE = { id: "r1", name: "Web", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48010, targetHost: "127.0.0.1", targetPort: 8080, enabled: false } as const;

describe("POST /api/config/plan (Nest) — scaffold default", () => {
  let nestApp: INestApplication;
  let nest: ParityServer;

  beforeAll(async () => {
    nestApp = await createNestApp();
    nest = await startNestServer(nestApp);
  });

  afterAll(async () => {
    await nest.close();
  });

  it("returns 400 when the body has no `desired` key", async () => {
    expect(await planReq(nest.baseUrl, {})).toEqual({ status: 400, body: { errors: ["desired is required."] } });
  });

  it("plans an add against the empty default config (no runtime wired)", async () => {
    const response = await planReq(nest.baseUrl, { desired: { rules: [{ name: "New", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 49000, targetHost: "127.0.0.1", targetPort: 9000, enabled: false }] } });
    expect(response.status).toBe(200);
    const plan = response.body as ConfigPlanResponse;
    expect(plan.mode).toBe("plan");
    expect(plan.summary.add).toBe(1);
    expect(typeof plan.generatedAt).toBe("string");
  });
});

describe("POST /api/config/plan (Nest) — parity with Express", () => {
  it("returns the same 400 envelope as Express for a missing `desired` key", async () => {
    const { nest, express, close } = await bootPair(new ForwardManager(new MemoryStore()));
    try {
      const [e, n] = await Promise.all([planReq(express.baseUrl, {}), planReq(nest.baseUrl, {})]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n).toEqual({ status: 400, body: { errors: ["desired is required."] } });
    } finally {
      await close();
    }
  });

  it("plans byte-for-byte like Express for an empty desired config (no drift, pinned generatedAt)", async () => {
    const { nest, express, close } = await bootPair(new ForwardManager(new MemoryStore()));
    try {
      const [e, n] = await Promise.all([
        planReq(express.baseUrl, { desired: { rules: [] } }),
        planReq(nest.baseUrl, { desired: { rules: [] } }),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const plan = n.body as ConfigPlanResponse;
      expect(plan.generatedAt).toBe(FIXED_NOW.toISOString()); // pinned, not stripped
      expect(plan.summary.hasDrift).toBe(false);
      expect(plan.operations).toEqual([]);
    } finally {
      await close();
    }
  });

  it("plans add/update/unchanged byte-for-byte like Express against seeded current rules, WITHOUT mutating them", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule(TCP_RULE);
    await manager.addRule({ id: "r2", name: "Keep", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48012, targetHost: "127.0.0.1", targetPort: 8082, enabled: false });
    const before = structuredClone(manager.listRules());
    const { nest, express, close } = await bootPair(manager);
    try {
      const desired = {
        rules: [
          { id: "r1", name: "Web", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48099, targetHost: "127.0.0.1", targetPort: 8080, enabled: false }, // update (listenPort)
          { id: "r2", name: "Keep", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48012, targetHost: "127.0.0.1", targetPort: 8082, enabled: false }, // unchanged
          { name: "Added", protocol: "udp", listenHost: "127.0.0.1", listenPort: 49001, targetHost: "127.0.0.1", targetPort: 53, enabled: false, udpMode: "one-way" }, // add
        ],
      };
      const [e, n] = await Promise.all([planReq(express.baseUrl, { desired }), planReq(nest.baseUrl, { desired })]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const plan = n.body as ConfigPlanResponse;
      expect(plan.summary).toMatchObject({ add: 1, update: 1, unchanged: 1, hasDrift: true });
      // Non-mutating: the shared manager's rules are unchanged after the plan.
      expect(manager.listRules()).toEqual(before);
    } finally {
      await close();
    }
  });

  it("surfaces an invalid desired rule as a plan error (200) byte-for-byte like Express — NOT a 400", async () => {
    const { nest, express, close } = await bootPair(new ForwardManager(new MemoryStore()));
    try {
      const desired = { rules: [{ name: "Bad", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 70000, targetHost: "127.0.0.1", targetPort: 8080, enabled: false }] };
      const [e, n] = await Promise.all([planReq(express.baseUrl, { desired }), planReq(nest.baseUrl, { desired })]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200); // invalid rules are plan errors, not request errors
      const plan = n.body as ConfigPlanResponse;
      expect(plan.summary.hasErrors).toBe(true);
      expect(plan.errors.length).toBeGreaterThan(0);
    } finally {
      await close();
    }
  });

  it("allows `desired: null` (200 with a plan error) byte-for-byte like Express", async () => {
    const { nest, express, close } = await bootPair(new ForwardManager(new MemoryStore()));
    try {
      const [e, n] = await Promise.all([
        planReq(express.baseUrl, { desired: null }),
        planReq(nest.baseUrl, { desired: null }),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      expect((n.body as ConfigPlanResponse).summary.hasErrors).toBe(true);
    } finally {
      await close();
    }
  });
});
