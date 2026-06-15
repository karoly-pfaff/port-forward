import "reflect-metadata";
import type http from "node:http";
import type { INestApplication } from "@nestjs/common";
import { Test } from "@nestjs/testing";
import type { ConfigApplyResponse, ForwardRule, ForwardRuleResponse } from "@portier/shared";
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
import { FORWARDS_READER } from "../forwards/forwards.reader.js";
import { CONFIG_EXPORT_READER } from "./config-export.reader.js";
import { CONFIG_APPLIER } from "./config-apply.writer.js";

class MemoryStore implements RuleStore {
  constructor(private rules: ForwardRule[] = []) {}
  async load(): Promise<ForwardRule[]> {
    return this.rules;
  }
  async save(rules: ForwardRule[]): Promise<void> {
    this.rules = rules;
  }
}

const FIXED_NOW = new Date("2026-06-15T12:00:30.000Z");

// Fixed-id, enabled:false rules → the replace import preserves the ids and starts no
// forwarder, so apply is deterministic and socket-free.
const RULE_A = { id: "a", name: "Web", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48010, targetHost: "127.0.0.1", targetPort: 8080, enabled: false } as const;

function applyReq(baseUrl: string, body: unknown): Promise<ApiResponse> {
  return fetchApi(baseUrl, "/api/config/apply", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** Nest app: the (seeded) manager bound to the applier + forwards/export readers + a pinned clock. */
async function nestWithManager(manager: ForwardManager): Promise<INestApplication> {
  const fixedClock: ClockReader = { now: () => FIXED_NOW };
  const moduleRef = await Test.createTestingModule({ imports: [AppModule] })
    .overrideProvider(CLOCK_READER)
    .useValue(fixedClock)
    .overrideProvider(CONFIG_APPLIER)
    .useValue(manager)
    .overrideProvider(FORWARDS_READER)
    .useValue(manager)
    .overrideProvider(CONFIG_EXPORT_READER)
    .useValue(manager)
    .compile();
  return moduleRef.createNestApplication({ logger: false });
}

/** Separate-but-equivalent managers per runtime (apply mutates, so a shared manager would cross-contaminate). */
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

describe("POST /api/config/apply (Nest) — scaffold default", () => {
  let nestApp: INestApplication;
  let nest: ParityServer;

  beforeAll(async () => {
    nestApp = await createNestApp();
    nest = await startNestServer(nestApp);
  });

  afterAll(async () => {
    await nest.close();
  });

  it("returns 400 for a missing desired key", async () => {
    expect(await applyReq(nest.baseUrl, {})).toEqual({ status: 400, body: { errors: ["desired is required."] } });
  });

  it("dry-runs against the empty default config (no runtime wired)", async () => {
    const response = await applyReq(nest.baseUrl, { desired: [RULE_A], dryRun: true });
    expect(response.status).toBe(200);
    const body = response.body as ConfigApplyResponse;
    expect(body.ok).toBe(true);
    expect(body.dryRun).toBe(true);
    expect(body.applied).toEqual({ add: 1, update: 0, remove: 0, unchanged: 0 });
  });
});

describe("POST /api/config/apply (Nest) — parity with Express", () => {
  it("returns the same 400 envelope as Express for a missing desired key", async () => {
    const { nest, express, close } = await bootPair();
    try {
      const [e, n] = await Promise.all([applyReq(express.baseUrl, {}), applyReq(nest.baseUrl, {})]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n).toEqual({ status: 400, body: { errors: ["desired is required."] } });
    } finally {
      await close();
    }
  });

  it("dry-runs byte-for-byte like Express (pinned appliedAt + generatedAt) WITHOUT mutating", async () => {
    const { nest, express, close } = await bootPair();
    try {
      const body = { desired: [RULE_A], dryRun: true };
      const [e, n] = await Promise.all([applyReq(express.baseUrl, body), applyReq(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const applied = n.body as ConfigApplyResponse;
      expect(applied.ok).toBe(true);
      expect(applied.dryRun).toBe(true);
      expect(applied.appliedAt).toBe(FIXED_NOW.toISOString());
      expect(applied.plan.generatedAt).toBe(FIXED_NOW.toISOString());
      // No mutation — both runtimes still have an empty rule list.
      const [ef, nf] = await Promise.all([fetchApi(express.baseUrl, "/api/forwards"), fetchApi(nest.baseUrl, "/api/forwards")]);
      expect(diffApiResponses(ef, nf)).toEqual([]);
      expect(nf.body).toEqual([]);
    } finally {
      await close();
    }
  });

  it("applies (add) byte-for-byte like Express; GET /api/forwards + export reflect the new rule", async () => {
    const { nest, express, close } = await bootPair();
    try {
      const body = { desired: [RULE_A] };
      const [e, n] = await Promise.all([applyReq(express.baseUrl, body), applyReq(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const applied = n.body as ConfigApplyResponse;
      expect(applied.ok).toBe(true);
      expect(applied.dryRun).toBe(false);
      expect(applied.applied).toEqual({ add: 1, update: 0, remove: 0, unchanged: 0 });
      // State after apply matches byte-for-byte in both runtimes.
      const [ef, nf] = await Promise.all([fetchApi(express.baseUrl, "/api/forwards"), fetchApi(nest.baseUrl, "/api/forwards")]);
      expect(diffApiResponses(ef, nf)).toEqual([]);
      expect((nf.body as ForwardRuleResponse[]).map((r) => r.id)).toEqual(["a"]);
      const [ex, nx] = await Promise.all([fetchApi(express.baseUrl, "/api/config/export"), fetchApi(nest.baseUrl, "/api/config/export")]);
      expect(diffApiResponses(ex, nx)).toEqual([]);
    } finally {
      await close();
    }
  });

  it("blocks a destructive apply without yes (400) byte-for-byte, leaving state unchanged", async () => {
    const seed = async (m: ForwardManager): Promise<void> => {
      await m.addRule({ ...RULE_A });
    };
    const { nest, express, close } = await bootPair(seed);
    try {
      // Removing the only rule is destructive; without yes it must be blocked.
      const body = { desired: [] };
      const [e, n] = await Promise.all([applyReq(express.baseUrl, body), applyReq(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n).toEqual({
        status: 400,
        body: { errors: ["Apply requires yes: true when destructive operations are present."] },
      });
      // No mutation — the rule survives in both runtimes.
      const [ef, nf] = await Promise.all([fetchApi(express.baseUrl, "/api/forwards"), fetchApi(nest.baseUrl, "/api/forwards")]);
      expect(diffApiResponses(ef, nf)).toEqual([]);
      expect((nf.body as ForwardRuleResponse[]).map((r) => r.id)).toEqual(["a"]);
    } finally {
      await close();
    }
  });

  it("applies a destructive change with yes:true byte-for-byte (the rule is removed)", async () => {
    const seed = async (m: ForwardManager): Promise<void> => {
      await m.addRule({ ...RULE_A });
    };
    const { nest, express, close } = await bootPair(seed);
    try {
      const body = { desired: [], yes: true };
      const [e, n] = await Promise.all([applyReq(express.baseUrl, body), applyReq(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      expect((n.body as ConfigApplyResponse).applied).toEqual({ add: 0, update: 0, remove: 1, unchanged: 0 });
      const [ef, nf] = await Promise.all([fetchApi(express.baseUrl, "/api/forwards"), fetchApi(nest.baseUrl, "/api/forwards")]);
      expect(diffApiResponses(ef, nf)).toEqual([]);
      expect(nf.body).toEqual([]);
    } finally {
      await close();
    }
  });

  it("returns ok:false (200) byte-for-byte for an invalid desired rule — NO mutation", async () => {
    const { nest, express, close } = await bootPair();
    try {
      const bad = { id: "bad", name: "Bad", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 70000, targetHost: "127.0.0.1", targetPort: 8080, enabled: false };
      const body = { desired: [bad] };
      const [e, n] = await Promise.all([applyReq(express.baseUrl, body), applyReq(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const applied = n.body as ConfigApplyResponse;
      expect(applied.ok).toBe(false);
      expect(applied.plan.summary.hasErrors).toBe(true);
      // No mutation — both runtimes still have an empty rule list.
      const [ef, nf] = await Promise.all([fetchApi(express.baseUrl, "/api/forwards"), fetchApi(nest.baseUrl, "/api/forwards")]);
      expect(diffApiResponses(ef, nf)).toEqual([]);
      expect(nf.body).toEqual([]);
    } finally {
      await close();
    }
  });

  it("returns ok:true with no mutation when there is no drift (current === desired)", async () => {
    const seed = async (m: ForwardManager): Promise<void> => {
      await m.addRule({ ...RULE_A });
    };
    const { nest, express, close } = await bootPair(seed);
    try {
      const body = { desired: [RULE_A] };
      const [e, n] = await Promise.all([applyReq(express.baseUrl, body), applyReq(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const applied = n.body as ConfigApplyResponse;
      expect(applied.ok).toBe(true);
      expect(applied.plan.summary.hasDrift).toBe(false);
      expect(applied.applied).toEqual({ add: 0, update: 0, remove: 0, unchanged: 1 });
    } finally {
      await close();
    }
  });
});
