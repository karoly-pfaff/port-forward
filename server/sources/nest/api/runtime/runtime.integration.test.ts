import "reflect-metadata";
import type http from "node:http";
import type { INestApplication } from "@nestjs/common";
import { Test } from "@nestjs/testing";
import type { ForwardRule, RuntimeInfo } from "@portier/shared";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { createApp } from "../../../legacy/api.js";
import { ForwardManager, type RuleStore } from "../../../forward-manager.js";
import { normalizeArch, normalizePlatform, type RuntimeInfoOptions } from "../../../runtime-info.js";
import { AppModule } from "../../app.module.js";
import { createNestApp } from "../../app.factory.js";
import {
  diffApiResponses,
  fetchApi,
  startHandlerServer,
  startNestServer,
  type ParityServer,
} from "../../testing/api-parity.js";
import {
  CLOCK_READER,
  RUNTIME_INFO_READER,
  type ClockReader,
  type RuntimeInfoReader,
} from "./runtime.reader.js";

/** Empty in-memory RuleStore so a ForwardManager can be constructed (no rules, no sockets). */
class MemoryStore implements RuleStore {
  async load(): Promise<ForwardRule[]> {
    return [];
  }
  async save(): Promise<void> {
    /* no-op */
  }
}

// Fixed volatile inputs shared by both runtimes for byte-for-byte parity.
const FIXED_STARTED_AT = new Date("2026-06-14T12:00:00.000Z");
const FIXED_NOW = new Date("2026-06-14T12:00:30.000Z"); // uptime = 30s

const FIXED_INFO: RuntimeInfoOptions = {
  version: "9.9.9-test",
  managementHost: "127.0.0.1",
  managementPort: 47831,
  configPath: "/test/data/forwards.json",
  staticDir: "/test/web",
  serviceMode: false,
  startedAt: FIXED_STARTED_AT,
};

/**
 * Builds a Nest app whose clock + runtime-info readers are fixed, but whose
 * PROCESS_READER stays the real default — so `pid`/`platform`/`arch` match the
 * Express app (same process) and the only otherwise-volatile field
 * (`uptimeSeconds`) is pinned by the shared fixed clock. No field is normalized
 * or stripped before comparison.
 */
async function nestWithFixedVolatiles(): Promise<INestApplication> {
  const fixedClock: ClockReader = { now: () => FIXED_NOW };
  const fixedRuntimeInfo: RuntimeInfoReader = {
    options: () => FIXED_INFO,
    startedAt: () => FIXED_STARTED_AT,
  };
  const moduleRef = await Test.createTestingModule({ imports: [AppModule] })
    .overrideProvider(CLOCK_READER)
    .useValue(fixedClock)
    .overrideProvider(RUNTIME_INFO_READER)
    .useValue(fixedRuntimeInfo)
    .compile();
  return moduleRef.createNestApplication({ logger: false });
}

describe("GET /api/runtime (Nest) — default app", () => {
  let nestApp: INestApplication;
  let nest: ParityServer;

  beforeAll(async () => {
    nestApp = await createNestApp();
    nest = await startNestServer(nestApp);
  });

  afterAll(async () => {
    await nest.close();
  });

  it("returns runtime info with default config and real process metadata", async () => {
    const response = await fetchApi(nest.baseUrl, "/api/runtime");
    expect(response.status).toBe(200);

    const body = response.body as RuntimeInfo;
    expect(body.name).toBe("Portier");
    expect(body.runtime).toBe("node");
    expect(body.version).toBe("unknown"); // no runtime info wired
    expect(body.managementHost).toBe("127.0.0.1");
    expect(body.managementPort).toBe(47831);
    expect(body.configPath).toBe("");
    expect(body.staticDir).toBe("");
    expect(body.serviceMode).toBe(false);
    // Real process metadata (normalized).
    expect(body.platform).toBe(normalizePlatform(process.platform));
    expect(body.arch).toBe(normalizeArch(process.arch));
    expect(body.pid).toBe(process.pid);
    // Live, non-negative uptime; parseable ISO start time.
    expect(typeof body.uptimeSeconds).toBe("number");
    expect(body.uptimeSeconds).toBeGreaterThanOrEqual(0);
    expect(new Date(body.startedAt).toISOString()).toBe(body.startedAt);
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

describe("GET /api/runtime (Nest) — parity with Express", () => {
  let nest: ParityServer;
  let express: ParityServer;

  beforeAll(async () => {
    const nestApp = await nestWithFixedVolatiles();
    nest = await startNestServer(nestApp);
    // Express uses the SAME fixed clock + runtime info; pid/platform/arch are the
    // real process values, identical to Nest's default PROCESS_READER.
    const app = createApp(new ForwardManager(new MemoryStore()), {
      runtimeInfo: FIXED_INFO,
      now: () => FIXED_NOW,
    });
    express = await startHandlerServer(app as unknown as http.RequestListener);
  });

  afterAll(async () => {
    await nest.close();
    await express.close();
  });

  it("matches Express byte-for-byte (no field normalization)", async () => {
    const [expressResponse, nestResponse] = await Promise.all([
      fetchApi(express.baseUrl, "/api/runtime"),
      fetchApi(nest.baseUrl, "/api/runtime"),
    ]);

    expect(diffApiResponses(expressResponse, nestResponse)).toEqual([]);
    expect(nestResponse.status).toBe(200);
    // Fully deterministic content (clock + info pinned; pid/platform/arch real).
    expect(nestResponse.body).toEqual({
      name: "Portier",
      version: "9.9.9-test",
      runtime: "node",
      platform: normalizePlatform(process.platform),
      arch: normalizeArch(process.arch),
      uptimeSeconds: 30,
      startedAt: "2026-06-14T12:00:00.000Z",
      managementHost: "127.0.0.1",
      managementPort: 47831,
      configPath: "/test/data/forwards.json",
      staticDir: "/test/web",
      serviceMode: false,
      pid: process.pid,
    });
  });
});
