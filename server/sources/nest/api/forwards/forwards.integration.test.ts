import "reflect-metadata";
import type http from "node:http";
import type { INestApplication } from "@nestjs/common";
import { Test } from "@nestjs/testing";
import { getPortAdvisories, type ForwardRule, type ForwardRuleResponse, type ForwardStatus } from "@portier/shared";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { createApp } from "../../../legacy/api.js";
import { ForwardManager, type RuleStore } from "../../../forward-manager.js";
import { getFreeTcpPort, getFreeUdpPort, startRuleStable } from "../../../test-helpers.js";
import { AppModule } from "../../app.module.js";
import { createNestApp } from "../../app.factory.js";
import {
  diffApiResponses,
  fetchApi,
  startHandlerServer,
  startNestServer,
  type ApiResponse,
  type ParityServer,
} from "../../testing/api-parity.js";
import { CLOCK_READER, type ClockReader } from "../../common/clock.reader.js";
import { STATUS_READER } from "../status/status.reader.js";
import { DIAGNOSTIC_READER } from "./forwards-diagnostics.reader.js";
import { FORWARDS_READER, type ForwardsReader } from "./forwards.reader.js";
import {
  FORWARD_GROUP_STARTER,
  FORWARD_GROUP_STOPPER,
  FORWARD_RULE_CREATOR,
  FORWARD_RULE_DELETER,
  FORWARD_RULES_REORDERER,
  FORWARD_RULE_STARTER,
  FORWARD_RULE_STOPPER,
  FORWARD_RULE_UPDATER,
} from "./forwards.writer.js";

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

// A 0.0.0.0 listen host guarantees a LAN_EXPOSURE advisory; both are stopped (enabled:false → no sockets).
const LAN_RULE = { name: "Web", protocol: "tcp", listenHost: "0.0.0.0", listenPort: 48010, targetHost: "127.0.0.1", targetPort: 8080, enabled: false } as const;
const UDP_RULE = { name: "DNS", protocol: "udp", listenHost: "127.0.0.1", listenPort: 48011, targetHost: "127.0.0.1", targetPort: 53, enabled: false, udpMode: "one-way" } as const;

/** The same mapping the Express route's toRuleResponse performs. */
function expectedResponses(manager: ForwardManager): ForwardRuleResponse[] {
  return manager.listRules().map((rule) => ({
    ...rule,
    advisories: getPortAdvisories({ port: rule.listenPort, listenHost: rule.listenHost, purpose: "forward" }),
  }));
}

async function nestWithReader(reader: ForwardsReader): Promise<INestApplication> {
  const moduleRef = await Test.createTestingModule({ imports: [AppModule] })
    .overrideProvider(FORWARDS_READER)
    .useValue(reader)
    .compile();
  return moduleRef.createNestApplication({ logger: false });
}

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

describe("GET /api/forwards (Nest) — default app", () => {
  let nestApp: INestApplication;
  let nest: ParityServer;

  beforeAll(async () => {
    nestApp = await createNestApp();
    nest = await startNestServer(nestApp);
  });

  afterAll(async () => {
    await nest.close();
  });

  it("returns an empty rule list by default (no runtime wired)", async () => {
    expect(await fetchApi(nest.baseUrl, "/api/forwards")).toEqual({ status: 200, body: [] });
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

describe("GET /api/forwards (Nest) — parity with Express", () => {
  it("matches Express for an empty manager", async () => {
    const { nest, express, close } = await bootPair(new ForwardManager(new MemoryStore()));
    try {
      const [expressResponse, nestResponse] = await Promise.all([
        fetchApi(express.baseUrl, "/api/forwards"),
        fetchApi(nest.baseUrl, "/api/forwards"),
      ]);
      expect(diffApiResponses(expressResponse, nestResponse)).toEqual([]);
      expect(nestResponse.body).toEqual([]);
    } finally {
      await close();
    }
  });

  it("matches Express byte-for-byte for seeded rules (with advisories)", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule(LAN_RULE);
    await manager.addRule(UDP_RULE);
    const { nest, express, close } = await bootPair(manager);
    try {
      const [expressResponse, nestResponse] = await Promise.all([
        fetchApi(express.baseUrl, "/api/forwards"),
        fetchApi(nest.baseUrl, "/api/forwards"),
      ]);
      expect(diffApiResponses(expressResponse, nestResponse)).toEqual([]);
      // Exact, deterministic content: rule fields + shared port advisories.
      expect(nestResponse.status).toBe(200);
      expect(nestResponse.body).toEqual(expectedResponses(manager));
      const body = nestResponse.body as ForwardRuleResponse[];
      expect(body).toHaveLength(2);
      expect(body[0].advisories.some((a) => a.code === "LAN_EXPOSURE")).toBe(true);
    } finally {
      await close();
    }
  });
});

// ── POST /api/forwards (create) ───────────────────────────────────────────────

// Fixed ids make the success responses deterministic for byte-for-byte parity
// (a created rule without an id gets a random UUID — covered separately). All
// enabled:false so no forwarder/socket starts.
const CREATE_TCP = { id: "fixed-tcp", name: "New Web", protocol: "tcp", listenHost: "0.0.0.0", listenPort: 48020, targetHost: "127.0.0.1", targetPort: 8080, enabled: false } as const;
const CREATE_UDP = { id: "fixed-udp", name: "New DNS", protocol: "udp", listenHost: "127.0.0.1", listenPort: 48021, targetHost: "127.0.0.1", targetPort: 53, enabled: false, udpMode: "one-way" } as const;

/**
 * Binds the (seeded) manager to the reader + status reader + diagnostic reader +
 * creator + updater + deleter + starter + stopper + reorderer so GET reflects
 * POST/PATCH/DELETE and `/start`/`/stop`/`/diagnose` + `/api/status` read the same
 * manager state. An optional fixed clock pins the diagnose `diagnosedAt` for
 * byte-for-byte parity.
 */
async function nestWithManager(manager: ForwardManager, clock?: ClockReader): Promise<INestApplication> {
  const builder = Test.createTestingModule({ imports: [AppModule] })
    .overrideProvider(FORWARDS_READER)
    .useValue(manager)
    .overrideProvider(STATUS_READER)
    .useValue(manager)
    .overrideProvider(DIAGNOSTIC_READER)
    .useValue(manager)
    .overrideProvider(FORWARD_RULE_CREATOR)
    .useValue(manager)
    .overrideProvider(FORWARD_RULE_UPDATER)
    .useValue(manager)
    .overrideProvider(FORWARD_RULE_DELETER)
    .useValue(manager)
    .overrideProvider(FORWARD_RULE_STARTER)
    .useValue(manager)
    .overrideProvider(FORWARD_RULE_STOPPER)
    .useValue(manager)
    .overrideProvider(FORWARD_RULES_REORDERER)
    .useValue(manager)
    .overrideProvider(FORWARD_GROUP_STOPPER)
    .useValue(manager)
    .overrideProvider(FORWARD_GROUP_STARTER)
    .useValue(manager);
  if (clock) {
    builder.overrideProvider(CLOCK_READER).useValue(clock);
  }
  const moduleRef = await builder.compile();
  return moduleRef.createNestApplication({ logger: false });
}

/** Express and Nest get SEPARATE but equivalent managers (a shared one would cross-contaminate on create). */
async function bootCreatePair(
  seed?: (manager: ForwardManager) => Promise<void>
): Promise<{ nest: ParityServer; express: ParityServer; close: () => Promise<void> }> {
  const expressManager = new ForwardManager(new MemoryStore());
  const nestManager = new ForwardManager(new MemoryStore());
  if (seed) {
    await seed(expressManager);
    await seed(nestManager);
  }
  const nestApp = await nestWithManager(nestManager);
  const nest = await startNestServer(nestApp);
  const express = await startHandlerServer(createApp(expressManager) as unknown as http.RequestListener);
  return {
    nest,
    express,
    close: async () => {
      await nest.close();
      await express.close();
    },
  };
}

function postRule(baseUrl: string, body: unknown): Promise<ApiResponse> {
  return fetchApi(baseUrl, "/api/forwards", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

describe("POST /api/forwards (Nest) — default app", () => {
  let nestApp: INestApplication;
  let nest: ParityServer;

  beforeAll(async () => {
    nestApp = await createNestApp();
    nest = await startNestServer(nestApp);
  });

  afterAll(async () => {
    await nest.close();
  });

  it("creates a rule via the default in-memory creator (201)", async () => {
    const response = await postRule(nest.baseUrl, CREATE_TCP);
    expect(response.status).toBe(201);
    const body = response.body as ForwardRuleResponse;
    expect(body.id).toBe("fixed-tcp");
    expect(body.name).toBe("New Web");
    expect(body.advisories.some((a) => a.code === "LAN_EXPOSURE")).toBe(true);
  });

  it("returns 404 for PATCH of an unknown id (default empty updater)", async () => {
    const response = await patchRule(nest.baseUrl, "nonexistent", { name: "X" });
    expect(response.status).toBe(404);
    expect((response.body as { errors: string[] }).errors[0]).toContain("not found");
  });

  it("returns 404 for DELETE of an unknown id (default empty deleter)", async () => {
    const response = await deleteRule(nest.baseUrl, "nonexistent");
    expect(response.status).toBe(404);
    expect((response.body as { errors: string[] }).errors[0]).toContain("not found");
  });

  it("returns 404 for START of an unknown id (default empty starter)", async () => {
    const response = await startRuleReq(nest.baseUrl, "nonexistent");
    expect(response.status).toBe(404);
    expect((response.body as { errors: string[] }).errors[0]).toContain("not found");
  });

  it("returns 404 for STOP of an unknown id (default empty stopper)", async () => {
    const response = await stopRuleReq(nest.baseUrl, "nonexistent");
    expect(response.status).toBe(404);
    expect((response.body as { errors: string[] }).errors[0]).toContain("not found");
  });

  it("returns 200 + [] for an empty reorder (default empty reorderer, no-op)", async () => {
    const response = await reorderReq(nest.baseUrl, { ids: [] });
    expect(response.status).toBe(200);
    expect(response.body).toEqual([]);
  });

  it("returns 404 for a reorder naming an unknown id (default empty reorderer)", async () => {
    const response = await reorderReq(nest.baseUrl, { ids: ["nonexistent"] });
    expect(response.status).toBe(404);
    expect((response.body as { errors: string[] }).errors[0]).toContain("not found");
  });

  it("returns 400 with the exact envelope for an invalid reorder body", async () => {
    const response = await reorderReq(nest.baseUrl, { ids: "notarray" });
    expect(response).toEqual({ status: 400, body: { errors: ["ids must be an array of strings."] } });
  });

  it("returns 404 for diagnose of an unknown id (default empty diagnostic reader)", async () => {
    const response = await diagnoseReq(nest.baseUrl, "nonexistent");
    expect(response.status).toBe(404);
    expect((response.body as { errors: string[] }).errors[0]).toContain("not found");
  });

  it("returns 404 for a group stop with no matching rules (default empty group stopper)", async () => {
    const response = await stopGroupReq(nest.baseUrl, "nonexistent");
    expect(response.status).toBe(404);
    expect((response.body as { errors: string[] }).errors[0]).toContain('No rules found in group "nonexistent"');
  });

  it("returns 400 for an empty/whitespace group name", async () => {
    const response = await stopGroupReq(nest.baseUrl, "%20%20");
    expect(response).toEqual({ status: 400, body: { errors: ["group is required."] } });
  });

  it("returns 404 for a group START with no matching rules (default empty group starter)", async () => {
    const response = await startGroupReq(nest.baseUrl, "nonexistent");
    expect(response.status).toBe(404);
    expect((response.body as { errors: string[] }).errors[0]).toContain('No rules found in group "nonexistent"');
  });

  it("returns 400 for a group START with an empty/whitespace group name", async () => {
    const response = await startGroupReq(nest.baseUrl, "%20%20");
    expect(response).toEqual({ status: 400, body: { errors: ["group is required."] } });
  });
});

describe("POST /api/forwards (Nest) — parity with Express", () => {
  it("creates a TCP rule byte-for-byte like Express (201 + advisories) and reflects it in GET", async () => {
    const { nest, express, close } = await bootCreatePair();
    try {
      const [e, n] = await Promise.all([postRule(express.baseUrl, CREATE_TCP), postRule(nest.baseUrl, CREATE_TCP)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(201);
      const body = n.body as ForwardRuleResponse;
      expect(body.id).toBe("fixed-tcp");
      expect(body.advisories.some((a) => a.code === "LAN_EXPOSURE")).toBe(true);
      // GET after create reflects the new rule in both runtimes.
      const [eg, ng] = await Promise.all([
        fetchApi(express.baseUrl, "/api/forwards"),
        fetchApi(nest.baseUrl, "/api/forwards"),
      ]);
      expect(diffApiResponses(eg, ng)).toEqual([]);
      expect((ng.body as ForwardRuleResponse[]).map((r) => r.id)).toEqual(["fixed-tcp"]);
    } finally {
      await close();
    }
  });

  it("creates a UDP rule byte-for-byte like Express (UDP first-class)", async () => {
    const { nest, express, close } = await bootCreatePair();
    try {
      const [e, n] = await Promise.all([postRule(express.baseUrl, CREATE_UDP), postRule(nest.baseUrl, CREATE_UDP)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(201);
      expect((n.body as ForwardRuleResponse).protocol).toBe("udp");
    } finally {
      await close();
    }
  });

  it("rejects a missing required field with the same 400 envelope as Express", async () => {
    const { nest, express, close } = await bootCreatePair();
    try {
      const [e, n] = await Promise.all([postRule(express.baseUrl, {}), postRule(nest.baseUrl, {})]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(400);
      expect((n.body as { errors: string[] }).errors.length).toBeGreaterThan(0);
    } finally {
      await close();
    }
  });

  it("rejects an out-of-range port with the same 400 envelope as Express", async () => {
    const { nest, express, close } = await bootCreatePair();
    try {
      const body = { ...CREATE_TCP, listenPort: 70000 };
      const [e, n] = await Promise.all([postRule(express.baseUrl, body), postRule(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(400);
    } finally {
      await close();
    }
  });

  it("rejects an invalid protocol with the same 400 envelope as Express", async () => {
    const { nest, express, close } = await bootCreatePair();
    try {
      const body = { ...CREATE_TCP, protocol: "icmp" };
      const [e, n] = await Promise.all([postRule(express.baseUrl, body), postRule(nest.baseUrl, body)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(400);
    } finally {
      await close();
    }
  });

  it("rejects a duplicate listen binding with the same 409 conflict envelope as Express", async () => {
    const { nest, express, close } = await bootCreatePair(async (manager) => {
      await manager.addRule({ name: "Existing", protocol: "tcp", listenHost: "0.0.0.0", listenPort: 48020, targetHost: "127.0.0.1", targetPort: 9000, enabled: false });
    });
    try {
      const [e, n] = await Promise.all([postRule(express.baseUrl, CREATE_TCP), postRule(nest.baseUrl, CREATE_TCP)]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(409);
      expect((n.body as { errors: string[] }).errors[0]).toContain("already listening");
    } finally {
      await close();
    }
  });

  it("generates a UUID id when the body omits one (no-id create path)", async () => {
    const { nest, close } = await bootCreatePair();
    try {
      const { id: _omit, ...withoutId } = CREATE_TCP;
      const response = await postRule(nest.baseUrl, withoutId);
      expect(response.status).toBe(201);
      const body = response.body as ForwardRuleResponse;
      expect(body.id).toBeTypeOf("string");
      expect(body.id.length).toBeGreaterThan(0);
      expect(body.id).not.toBe("fixed-tcp");
    } finally {
      await close();
    }
  });
});

// ── PATCH /api/forwards/:id (update) ──────────────────────────────────────────

function patchRule(baseUrl: string, id: string, body: unknown): Promise<ApiResponse> {
  return fetchApi(baseUrl, `/api/forwards/${id}`, {
    method: "PATCH",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

function deleteRule(baseUrl: string, id: string): Promise<ApiResponse> {
  return fetchApi(baseUrl, `/api/forwards/${id}`, { method: "DELETE" });
}

function startRuleReq(baseUrl: string, id: string): Promise<ApiResponse> {
  return fetchApi(baseUrl, `/api/forwards/${id}/start`, { method: "POST" });
}

function stopRuleReq(baseUrl: string, id: string): Promise<ApiResponse> {
  return fetchApi(baseUrl, `/api/forwards/${id}/stop`, { method: "POST" });
}

function reorderReq(baseUrl: string, body: unknown): Promise<ApiResponse> {
  return fetchApi(baseUrl, "/api/forwards/reorder", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

function diagnoseReq(baseUrl: string, id: string): Promise<ApiResponse> {
  return fetchApi(baseUrl, `/api/forwards/${id}/diagnose`, { method: "POST" });
}

function stopGroupReq(baseUrl: string, group: string): Promise<ApiResponse> {
  return fetchApi(baseUrl, `/api/forwards/groups/${group}/stop`, { method: "POST" });
}

function startGroupReq(baseUrl: string, group: string): Promise<ApiResponse> {
  return fetchApi(baseUrl, `/api/forwards/groups/${group}/start`, { method: "POST" });
}

// A stopped TCP + UDP pair so DELETE can be parity-tested per protocol without sockets.
const SEED_UDP = { id: "seed-udp", name: "SeedUdp", protocol: "udp", listenHost: "127.0.0.1", listenPort: 48032, targetHost: "127.0.0.1", targetPort: 53, enabled: false, udpMode: "one-way" } as const;

async function seedTcpUdp(manager: ForwardManager): Promise<void> {
  await manager.addRule(SEED_TCP);
  await manager.addRule(SEED_UDP);
}

// A stopped (enabled:false) rule with a known id, seeded into both managers so PATCH is deterministic.
const SEED_TCP = { id: "seed-1", name: "Seed", protocol: "tcp", listenHost: "0.0.0.0", listenPort: 48030, targetHost: "127.0.0.1", targetPort: 8080, enabled: false } as const;

async function seedOne(manager: ForwardManager): Promise<void> {
  await manager.addRule(SEED_TCP);
}

async function seedTwo(manager: ForwardManager): Promise<void> {
  await manager.addRule(SEED_TCP);
  await manager.addRule({ id: "seed-2", name: "Second", protocol: "tcp", listenHost: "0.0.0.0", listenPort: 48031, targetHost: "127.0.0.1", targetPort: 8081, enabled: false });
}

// Three stopped rules (distinct bindings) for reorder parity. enabled:false → no sockets.
async function seedThree(manager: ForwardManager): Promise<void> {
  await manager.addRule({ id: "r1", name: "One", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48040, targetHost: "127.0.0.1", targetPort: 8080, enabled: false });
  await manager.addRule({ id: "r2", name: "Two", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48041, targetHost: "127.0.0.1", targetPort: 8081, enabled: false });
  await manager.addRule({ id: "r3", name: "Three", protocol: "udp", listenHost: "127.0.0.1", listenPort: 48042, targetHost: "127.0.0.1", targetPort: 8082, enabled: false, udpMode: "one-way" });
}

describe("PATCH /api/forwards/:id (Nest) — parity with Express", () => {
  it("applies a metadata-only patch (no restart) and preserves unspecified fields, byte-for-byte like Express", async () => {
    const { nest, express, close } = await bootCreatePair(seedOne);
    try {
      const patch = { name: "Renamed", group: "team" };
      const [e, n] = await Promise.all([
        patchRule(express.baseUrl, "seed-1", patch),
        patchRule(nest.baseUrl, "seed-1", patch),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const body = n.body as ForwardRuleResponse;
      expect(body).toMatchObject({ id: "seed-1", name: "Renamed", group: "team" });
      // Unspecified fields are NOT overwritten with undefined.
      expect(body.listenPort).toBe(48030);
      expect(body.targetHost).toBe("127.0.0.1");
      expect(body.protocol).toBe("tcp");
      // GET after PATCH reflects the update in both runtimes.
      const [eg, ng] = await Promise.all([
        fetchApi(express.baseUrl, "/api/forwards"),
        fetchApi(nest.baseUrl, "/api/forwards"),
      ]);
      expect(diffApiResponses(eg, ng)).toEqual([]);
      expect((ng.body as ForwardRuleResponse[])[0].name).toBe("Renamed");
    } finally {
      await close();
    }
  });

  it("applies a forwarding-affecting patch on a stopped rule (no socket) byte-for-byte like Express", async () => {
    const { nest, express, close } = await bootCreatePair(seedOne);
    try {
      const patch = { targetPort: 9999 };
      const [e, n] = await Promise.all([
        patchRule(express.baseUrl, "seed-1", patch),
        patchRule(nest.baseUrl, "seed-1", patch),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      expect((n.body as ForwardRuleResponse).targetPort).toBe(9999);
    } finally {
      await close();
    }
  });

  it("returns the same 404 envelope as Express for an unknown id", async () => {
    const { nest, express, close } = await bootCreatePair(seedOne);
    try {
      const [e, n] = await Promise.all([
        patchRule(express.baseUrl, "ghost", { name: "X" }),
        patchRule(nest.baseUrl, "ghost", { name: "X" }),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(404);
    } finally {
      await close();
    }
  });

  it("rejects an out-of-range port with the same 400 envelope as Express", async () => {
    const { nest, express, close } = await bootCreatePair(seedOne);
    try {
      const patch = { listenPort: 70000 };
      const [e, n] = await Promise.all([
        patchRule(express.baseUrl, "seed-1", patch),
        patchRule(nest.baseUrl, "seed-1", patch),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(400);
    } finally {
      await close();
    }
  });

  it("rejects an invalid protocol with the same 400 envelope as Express", async () => {
    const { nest, express, close } = await bootCreatePair(seedOne);
    try {
      const patch = { protocol: "icmp" };
      const [e, n] = await Promise.all([
        patchRule(express.baseUrl, "seed-1", patch),
        patchRule(nest.baseUrl, "seed-1", patch),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(400);
    } finally {
      await close();
    }
  });

  it("rejects a patch that duplicates another rule's binding with the same 409 envelope as Express", async () => {
    const { nest, express, close } = await bootCreatePair(seedTwo);
    try {
      // Move seed-2 onto seed-1's binding (0.0.0.0:48030) → conflict.
      const patch = { listenPort: 48030 };
      const [e, n] = await Promise.all([
        patchRule(express.baseUrl, "seed-2", patch),
        patchRule(nest.baseUrl, "seed-2", patch),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(409);
      expect((n.body as { errors: string[] }).errors[0]).toContain("already listening");
    } finally {
      await close();
    }
  });

  it("ignores an unknown extra field in the patch, byte-for-byte like Express", async () => {
    const { nest, express, close } = await bootCreatePair(seedOne);
    try {
      const patch = { name: "WithExtra", bogusField: "ignored" };
      const [e, n] = await Promise.all([
        patchRule(express.baseUrl, "seed-1", patch),
        patchRule(nest.baseUrl, "seed-1", patch),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const body = n.body as ForwardRuleResponse & { bogusField?: unknown };
      expect(body.name).toBe("WithExtra");
      expect(body.bogusField).toBeUndefined(); // unknown field dropped by the validator
    } finally {
      await close();
    }
  });
});

// ── DELETE /api/forwards/:id (delete) ─────────────────────────────────────────

describe("DELETE /api/forwards/:id (Nest) — parity with Express", () => {
  it("deletes an existing stopped TCP rule (204, no body) and removes it from GET, byte-for-byte like Express", async () => {
    const { nest, express, close } = await bootCreatePair(seedTcpUdp);
    try {
      const [e, n] = await Promise.all([
        deleteRule(express.baseUrl, "seed-1"),
        deleteRule(nest.baseUrl, "seed-1"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(204);
      expect(n.body).toBeNull(); // 204, no response body
      // GET after DELETE reflects the removal in both runtimes.
      const [eg, ng] = await Promise.all([
        fetchApi(express.baseUrl, "/api/forwards"),
        fetchApi(nest.baseUrl, "/api/forwards"),
      ]);
      expect(diffApiResponses(eg, ng)).toEqual([]);
      expect((ng.body as ForwardRuleResponse[]).map((r) => r.id)).toEqual(["seed-udp"]);
    } finally {
      await close();
    }
  });

  it("deletes an existing stopped UDP rule (204, no body) byte-for-byte like Express (UDP first-class)", async () => {
    const { nest, express, close } = await bootCreatePair(seedTcpUdp);
    try {
      const [e, n] = await Promise.all([
        deleteRule(express.baseUrl, "seed-udp"),
        deleteRule(nest.baseUrl, "seed-udp"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(204);
      const [eg, ng] = await Promise.all([
        fetchApi(express.baseUrl, "/api/forwards"),
        fetchApi(nest.baseUrl, "/api/forwards"),
      ]);
      expect(diffApiResponses(eg, ng)).toEqual([]);
      expect((ng.body as ForwardRuleResponse[]).map((r) => r.id)).toEqual(["seed-1"]);
    } finally {
      await close();
    }
  });

  it("returns the same 404 envelope as Express for an unknown id", async () => {
    const { nest, express, close } = await bootCreatePair(seedOne);
    try {
      const [e, n] = await Promise.all([
        deleteRule(express.baseUrl, "ghost"),
        deleteRule(nest.baseUrl, "ghost"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(404);
      expect((n.body as { errors: string[] }).errors[0]).toContain("not found");
    } finally {
      await close();
    }
  });

  it("returns the same 404 for a repeated delete (Express has no idempotent no-op), byte-for-byte", async () => {
    const { nest, express, close } = await bootCreatePair(seedOne);
    try {
      // First delete succeeds in both runtimes.
      const [e1, n1] = await Promise.all([
        deleteRule(express.baseUrl, "seed-1"),
        deleteRule(nest.baseUrl, "seed-1"),
      ]);
      expect(diffApiResponses(e1, n1)).toEqual([]);
      expect(n1.status).toBe(204);
      // Second delete of the now-removed id → 404 in both runtimes.
      const [e2, n2] = await Promise.all([
        deleteRule(express.baseUrl, "seed-1"),
        deleteRule(nest.baseUrl, "seed-1"),
      ]);
      expect(diffApiResponses(e2, n2)).toEqual([]);
      expect(n2.status).toBe(404);
    } finally {
      await close();
    }
  });
});

// ── POST /api/forwards/:id/start (lifecycle start) ────────────────────────────

/**
 * Boots Express and Nest over the SAME manager instance (start is idempotent, so
 * sharing is safe and is what makes the started status — incl. the volatile
 * `startedAt` — deterministically equal across runtimes). Caller owns rule
 * lifecycle + socket cleanup (`manager.stopAll()`).
 */
async function bootSharedStartPair(
  manager: ForwardManager
): Promise<{ nest: ParityServer; express: ParityServer; close: () => Promise<void> }> {
  const nestApp = await nestWithManager(manager);
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

describe("POST /api/forwards/:id/start (Nest) — parity with Express", () => {
  it("returns the same 404 envelope as Express for an unknown id", async () => {
    const { nest, express, close } = await bootCreatePair(seedOne);
    try {
      const [e, n] = await Promise.all([
        startRuleReq(express.baseUrl, "ghost"),
        startRuleReq(nest.baseUrl, "ghost"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(404);
      expect((n.body as { errors: string[] }).errors[0]).toContain("not found");
    } finally {
      await close();
    }
  });

  it("returns the started rule's status (200) byte-for-byte like Express — idempotent already-running, no volatile drift", async () => {
    // Shared manager: open the listener ONCE (bind-retry helper) so `startedAt` is
    // pinned, then both runtimes hit the idempotent already-running path and return
    // the identical status. One real socket, cleaned up in finally.
    const manager = new ForwardManager(new MemoryStore());
    const listenPort = await getFreeTcpPort();
    await manager.addRule({ id: "run-1", name: "Runner", protocol: "tcp", listenHost: "127.0.0.1", listenPort, targetHost: "127.0.0.1", targetPort: 49990, enabled: false });
    const { nest, express, close } = await bootSharedStartPair(manager);
    try {
      await startRuleStable(manager, "run-1", getFreeTcpPort); // opens the listener once

      const [e, n] = await Promise.all([
        startRuleReq(express.baseUrl, "run-1"),
        startRuleReq(nest.baseUrl, "run-1"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const body = n.body as ForwardStatus;
      expect(body).toMatchObject({ ruleId: "run-1", running: true });
      expect(typeof body.startedAt).toBe("string");

      // GET /api/status after start reflects the running rule identically in both runtimes.
      const [es, ns] = await Promise.all([
        fetchApi(express.baseUrl, "/api/status"),
        fetchApi(nest.baseUrl, "/api/status"),
      ]);
      expect(diffApiResponses(es, ns)).toEqual([]);
      expect((ns.body as ForwardStatus[])[0]).toMatchObject({ ruleId: "run-1", running: true });
    } finally {
      await manager.stopAll();
      await close();
    }
  });
});

describe("POST /api/forwards/:id/start (Nest) — real cold start (single runtime, socket cleaned up)", () => {
  it("starts a stopped TCP rule end-to-end (200, running:true, ISO startedAt) and stops it", async () => {
    const manager = new ForwardManager(new MemoryStore());
    const listenPort = await getFreeTcpPort();
    await manager.addRule({ id: "cold-tcp", name: "ColdTcp", protocol: "tcp", listenHost: "127.0.0.1", listenPort, targetHost: "127.0.0.1", targetPort: 49991, enabled: false });
    const nestApp = await nestWithManager(manager);
    const nest = await startNestServer(nestApp);
    try {
      const response = await startRuleReq(nest.baseUrl, "cold-tcp");
      expect(response.status).toBe(200);
      const body = response.body as ForwardStatus;
      expect(body.ruleId).toBe("cold-tcp");
      expect(body.running).toBe(true);
      expect(body.startedAt).toBeTypeOf("string");
      expect(Number.isNaN(Date.parse(body.startedAt as string))).toBe(false);
    } finally {
      await manager.stopAll(); // close the real listener
      await nest.close();
    }
  });

  it("starts a stopped UDP rule end-to-end (200, running:true) — UDP first-class — and stops it", async () => {
    const manager = new ForwardManager(new MemoryStore());
    const listenPort = await getFreeUdpPort();
    await manager.addRule({ id: "cold-udp", name: "ColdUdp", protocol: "udp", listenHost: "127.0.0.1", listenPort, targetHost: "127.0.0.1", targetPort: 49992, enabled: false, udpMode: "one-way" });
    const nestApp = await nestWithManager(manager);
    const nest = await startNestServer(nestApp);
    try {
      const response = await startRuleReq(nest.baseUrl, "cold-udp");
      expect(response.status).toBe(200);
      const body = response.body as ForwardStatus;
      expect(body.ruleId).toBe("cold-udp");
      expect(body.running).toBe(true);
    } finally {
      await manager.stopAll(); // close the real socket
      await nest.close();
    }
  });
});

// ── POST /api/forwards/:id/stop (lifecycle stop) ──────────────────────────────

describe("POST /api/forwards/:id/stop (Nest) — parity with Express", () => {
  it("returns the same 404 envelope as Express for an unknown id", async () => {
    const { nest, express, close } = await bootCreatePair(seedOne);
    try {
      const [e, n] = await Promise.all([
        stopRuleReq(express.baseUrl, "ghost"),
        stopRuleReq(nest.baseUrl, "ghost"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(404);
      expect((n.body as { errors: string[] }).errors[0]).toContain("not found");
    } finally {
      await close();
    }
  });

  it("stopping an already-stopped rule returns the deterministic stopped status (200), byte-for-byte like Express — NO sockets", async () => {
    // seed-1 is a stopped TCP rule; never started, so stopRule is a pure no-op that
    // returns the synthetic stopped status. Deterministic (running:false, no
    // startedAt), so separate managers give byte-for-byte equality.
    const { nest, express, close } = await bootCreatePair(seedOne);
    try {
      const [e, n] = await Promise.all([
        stopRuleReq(express.baseUrl, "seed-1"),
        stopRuleReq(nest.baseUrl, "seed-1"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const body = n.body as ForwardStatus;
      expect(body).toMatchObject({ ruleId: "seed-1", running: false });
      expect(body.startedAt).toBeUndefined();
    } finally {
      await close();
    }
  });

  it("stops a RUNNING TCP rule (200, running:false) byte-for-byte like Express, and GET /api/status reflects it", async () => {
    // Separate-but-equivalent managers; each starts the rule on its own free port
    // (bind-retry helper), then stops it via the endpoint. The stopped status is
    // deterministic (no volatile field), so byte-for-byte parity holds.
    const expressManager = new ForwardManager(new MemoryStore());
    const nestManager = new ForwardManager(new MemoryStore());
    const seed = async (m: ForwardManager): Promise<void> => {
      await m.addRule({ id: "stop-tcp", name: "StopTcp", protocol: "tcp", listenHost: "127.0.0.1", listenPort: await getFreeTcpPort(), targetHost: "127.0.0.1", targetPort: 49993, enabled: false });
    };
    await seed(expressManager);
    await seed(nestManager);
    const nestApp = await nestWithManager(nestManager);
    const nest = await startNestServer(nestApp);
    const express = await startHandlerServer(createApp(expressManager) as unknown as http.RequestListener);
    try {
      await startRuleStable(expressManager, "stop-tcp", getFreeTcpPort);
      await startRuleStable(nestManager, "stop-tcp", getFreeTcpPort);

      const [e, n] = await Promise.all([
        stopRuleReq(express.baseUrl, "stop-tcp"),
        stopRuleReq(nest.baseUrl, "stop-tcp"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const body = n.body as ForwardStatus;
      expect(body).toMatchObject({ ruleId: "stop-tcp", running: false });
      expect(body.startedAt).toBeUndefined();

      // GET /api/status after stop reflects the stopped rule identically in both runtimes.
      const [es, ns] = await Promise.all([
        fetchApi(express.baseUrl, "/api/status"),
        fetchApi(nest.baseUrl, "/api/status"),
      ]);
      expect(diffApiResponses(es, ns)).toEqual([]);
      expect((ns.body as ForwardStatus[])[0]).toMatchObject({ ruleId: "stop-tcp", running: false });
    } finally {
      await expressManager.stopAll();
      await nestManager.stopAll();
      await nest.close();
      await express.close();
    }
  });
});

describe("POST /api/forwards/:id/stop (Nest) — real stop of a running UDP rule (single runtime, socket cleaned up)", () => {
  it("starts then stops a running UDP rule end-to-end (200, running:false) — UDP first-class", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule({ id: "stop-udp", name: "StopUdp", protocol: "udp", listenHost: "127.0.0.1", listenPort: await getFreeUdpPort(), targetHost: "127.0.0.1", targetPort: 49994, enabled: false, udpMode: "one-way" });
    const nestApp = await nestWithManager(manager);
    const nest = await startNestServer(nestApp);
    try {
      const started = await startRuleReq(nest.baseUrl, "stop-udp");
      expect(started.status).toBe(200);
      expect((started.body as ForwardStatus).running).toBe(true);

      const stopped = await stopRuleReq(nest.baseUrl, "stop-udp");
      expect(stopped.status).toBe(200);
      const body = stopped.body as ForwardStatus;
      expect(body.ruleId).toBe("stop-udp");
      expect(body.running).toBe(false);
    } finally {
      await manager.stopAll(); // idempotent — ensure no leaked socket
      await nest.close();
    }
  });
});

// ── POST /api/forwards/reorder (reorder) ──────────────────────────────────────

describe("POST /api/forwards/reorder (Nest) — parity with Express", () => {
  const ids = (body: unknown): string[] => (body as ForwardRuleResponse[]).map((r) => r.id);

  it("reorders all rules (full set, 200), byte-for-byte like Express, and GET reflects the new order", async () => {
    const { nest, express, close } = await bootCreatePair(seedThree);
    try {
      const [e, n] = await Promise.all([
        reorderReq(express.baseUrl, { ids: ["r3", "r1", "r2"] }),
        reorderReq(nest.baseUrl, { ids: ["r3", "r1", "r2"] }),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      expect(ids(n.body)).toEqual(["r3", "r1", "r2"]);
      // GET after reorder reflects the new order in both runtimes.
      const [eg, ng] = await Promise.all([
        fetchApi(express.baseUrl, "/api/forwards"),
        fetchApi(nest.baseUrl, "/api/forwards"),
      ]);
      expect(diffApiResponses(eg, ng)).toEqual([]);
      expect(ids(ng.body)).toEqual(["r3", "r1", "r2"]);
    } finally {
      await close();
    }
  });

  it("reorders a partial set — listed ids first, the rest keep their order at the end — byte-for-byte like Express", async () => {
    const { nest, express, close } = await bootCreatePair(seedThree);
    try {
      const [e, n] = await Promise.all([
        reorderReq(express.baseUrl, { ids: ["r2"] }),
        reorderReq(nest.baseUrl, { ids: ["r2"] }),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      expect(ids(n.body)).toEqual(["r2", "r1", "r3"]); // r2 first, then r1, r3 in prior order
    } finally {
      await close();
    }
  });

  it("tolerates a duplicate id (deduped, no error), byte-for-byte like Express", async () => {
    const { nest, express, close } = await bootCreatePair(seedThree);
    try {
      const [e, n] = await Promise.all([
        reorderReq(express.baseUrl, { ids: ["r2", "r2", "r1"] }),
        reorderReq(nest.baseUrl, { ids: ["r2", "r2", "r1"] }),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      expect(ids(n.body)).toEqual(["r2", "r1", "r3"]);
    } finally {
      await close();
    }
  });

  it("treats an empty ids list as a no-op (200, unchanged order), byte-for-byte like Express", async () => {
    const { nest, express, close } = await bootCreatePair(seedThree);
    try {
      const [e, n] = await Promise.all([
        reorderReq(express.baseUrl, { ids: [] }),
        reorderReq(nest.baseUrl, { ids: [] }),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      expect(ids(n.body)).toEqual(["r1", "r2", "r3"]); // unchanged
    } finally {
      await close();
    }
  });

  it("returns the same 404 envelope as Express for an unknown id (no reorder persisted)", async () => {
    const { nest, express, close } = await bootCreatePair(seedThree);
    try {
      const [e, n] = await Promise.all([
        reorderReq(express.baseUrl, { ids: ["r2", "ghost"] }),
        reorderReq(nest.baseUrl, { ids: ["r2", "ghost"] }),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(404);
      expect((n.body as { errors: string[] }).errors[0]).toContain("not found");
      // Order is unchanged in both runtimes (the unknown id aborts before persist).
      const [eg, ng] = await Promise.all([
        fetchApi(express.baseUrl, "/api/forwards"),
        fetchApi(nest.baseUrl, "/api/forwards"),
      ]);
      expect(diffApiResponses(eg, ng)).toEqual([]);
      expect(ids(ng.body)).toEqual(["r1", "r2", "r3"]);
    } finally {
      await close();
    }
  });

  it("rejects an invalid body shape with the same 400 envelope as Express", async () => {
    const { nest, express, close } = await bootCreatePair(seedThree);
    try {
      for (const bad of [{ ids: "notarray" }, { ids: ["ok", 1] }, {}]) {
        const [e, n] = await Promise.all([
          reorderReq(express.baseUrl, bad),
          reorderReq(nest.baseUrl, bad),
        ]);
        expect(diffApiResponses(e, n)).toEqual([]);
        expect(n).toEqual({ status: 400, body: { errors: ["ids must be an array of strings."] } });
      }
    } finally {
      await close();
    }
  });
});

// ── POST /api/forwards/:id/diagnose (diagnose) ────────────────────────────────

describe("POST /api/forwards/:id/diagnose (Nest) — parity with Express", () => {
  it("returns the same 404 envelope as Express for an unknown id", async () => {
    const { nest, express, close } = await bootCreatePair(seedOne);
    try {
      const [e, n] = await Promise.all([
        diagnoseReq(express.baseUrl, "ghost"),
        diagnoseReq(nest.baseUrl, "ghost"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(404);
      expect((n.body as { errors: string[] }).errors[0]).toContain("not found");
    } finally {
      await close();
    }
  });

  it("diagnoses a stopped UDP rule byte-for-byte like Express (pinned diagnosedAt, deterministic checks)", async () => {
    // A UDP rule makes diagnose deterministic: target-connect is always "skip" (no
    // TCP probe), 127.0.0.1 resolves instantly, and the only socket is a transient
    // UDP listen-bind. A SHARED manager + a pinned clock (both runtimes) make the
    // whole body — incl. the volatile diagnosedAt — byte-for-byte equal. Sequential
    // calls so the listen-bind probe never collides.
    const FIXED = new Date("2026-06-15T12:00:00.000Z");
    const fixedClock: ClockReader = { now: () => FIXED };
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule({ id: "diag-udp", name: "DiagUdp", protocol: "udp", listenHost: "127.0.0.1", listenPort: await getFreeUdpPort(), targetHost: "127.0.0.1", targetPort: await getFreeUdpPort(), enabled: false, udpMode: "one-way" });

    const nestApp = await nestWithManager(manager, fixedClock);
    const nest = await startNestServer(nestApp);
    const express = await startHandlerServer(createApp(manager, { now: () => FIXED }) as unknown as http.RequestListener);
    try {
      // Sequential (not Promise.all) so the two listen-bind probes never overlap.
      const e = await diagnoseReq(express.baseUrl, "diag-udp");
      const n = await diagnoseReq(nest.baseUrl, "diag-udp");
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      const body = n.body as {
        ruleId: string;
        protocol: string;
        diagnosedAt: string;
        checks: Array<{ id: string; status: string }>;
      };
      expect(body.ruleId).toBe("diag-udp");
      expect(body.protocol).toBe("udp");
      expect(body.diagnosedAt).toBe(FIXED.toISOString()); // pinned, not stripped
      expect(body.checks.find((c) => c.id === "target-connect")?.status).toBe("skip");
      expect(body.checks.find((c) => c.id === "udp-mode")?.status).toBe("pass");
    } finally {
      await nest.close();
      await express.close();
    }
  });
});

// ── POST /api/forwards/groups/:group/stop (group stop) ────────────────────────

// Two stopped rules sharing group "web" + one ungrouped rule. enabled:false → no
// sockets; stopping the group is a deterministic no-op (all not_running skips).
async function seedGroup(manager: ForwardManager): Promise<void> {
  await manager.addRule({ id: "g1", name: "Web One", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48050, targetHost: "127.0.0.1", targetPort: 8080, enabled: false, group: "web" });
  await manager.addRule({ id: "g2", name: "Web Two", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48051, targetHost: "127.0.0.1", targetPort: 8081, enabled: false, group: "web" });
  await manager.addRule({ id: "g3", name: "Other", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48052, targetHost: "127.0.0.1", targetPort: 8082, enabled: false });
}

describe("POST /api/forwards/groups/:group/stop (Nest) — parity with Express", () => {
  it("stops a group of stopped rules byte-for-byte like Express (all not_running skips, NO sockets) and leaves rules unchanged", async () => {
    const { nest, express, close } = await bootCreatePair(seedGroup);
    try {
      const [e, n] = await Promise.all([
        stopGroupReq(express.baseUrl, "web"),
        stopGroupReq(nest.baseUrl, "web"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      expect(n.body).toEqual({
        group: "web",
        action: "stop",
        total: 2,
        succeeded: 0,
        skipped: 2,
        failed: 0,
        results: [
          { ruleId: "g1", ruleName: "Web One", status: "skipped", reason: "not_running" },
          { ruleId: "g2", ruleName: "Web Two", status: "skipped", reason: "not_running" },
        ],
      });
      // The group action does not mutate rule definitions/order — GET is unchanged in both runtimes.
      const [eg, ng] = await Promise.all([
        fetchApi(express.baseUrl, "/api/forwards"),
        fetchApi(nest.baseUrl, "/api/forwards"),
      ]);
      expect(diffApiResponses(eg, ng)).toEqual([]);
      expect((ng.body as ForwardRuleResponse[]).map((r) => r.id)).toEqual(["g1", "g2", "g3"]);
    } finally {
      await close();
    }
  });

  it("returns the same 404 envelope as Express for a group with no rules", async () => {
    const { nest, express, close } = await bootCreatePair(seedGroup);
    try {
      const [e, n] = await Promise.all([
        stopGroupReq(express.baseUrl, "ghost"),
        stopGroupReq(nest.baseUrl, "ghost"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(404);
      expect((n.body as { errors: string[] }).errors[0]).toContain('No rules found in group "ghost"');
    } finally {
      await close();
    }
  });

  it("returns the same 400 envelope as Express for an empty/whitespace group name", async () => {
    const { nest, express, close } = await bootCreatePair(seedGroup);
    try {
      const [e, n] = await Promise.all([
        stopGroupReq(express.baseUrl, "%20%20"),
        stopGroupReq(nest.baseUrl, "%20%20"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n).toEqual({ status: 400, body: { errors: ["group is required."] } });
    } finally {
      await close();
    }
  });

  it("normalizes an encoded group name (spaces) the same way as Express", async () => {
    // Seed a group literally named "team space"; request it URL-encoded.
    const seed = async (manager: ForwardManager): Promise<void> => {
      await manager.addRule({ id: "s1", name: "Spaced", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48055, targetHost: "127.0.0.1", targetPort: 8080, enabled: false, group: "team space" });
    };
    const { nest, express, close } = await bootCreatePair(seed);
    try {
      const [e, n] = await Promise.all([
        stopGroupReq(express.baseUrl, "team%20space"),
        stopGroupReq(nest.baseUrl, "team%20space"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      expect((n.body as { group: string; total: number }).group).toBe("team space");
      expect((n.body as { total: number }).total).toBe(1);
    } finally {
      await close();
    }
  });
});

// ── POST /api/forwards/groups/:group/start (group start) ──────────────────────

describe("POST /api/forwards/groups/:group/start (Nest) — parity with Express", () => {
  it("returns the same 404 envelope as Express for a group with no rules", async () => {
    const { nest, express, close } = await bootCreatePair(seedGroup);
    try {
      const [e, n] = await Promise.all([
        startGroupReq(express.baseUrl, "ghost"),
        startGroupReq(nest.baseUrl, "ghost"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(404);
      expect((n.body as { errors: string[] }).errors[0]).toContain('No rules found in group "ghost"');
    } finally {
      await close();
    }
  });

  it("returns the same 400 envelope as Express for an empty/whitespace group name", async () => {
    const { nest, express, close } = await bootCreatePair(seedGroup);
    try {
      const [e, n] = await Promise.all([
        startGroupReq(express.baseUrl, "%20%20"),
        startGroupReq(nest.baseUrl, "%20%20"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n).toEqual({ status: 400, body: { errors: ["group is required."] } });
    } finally {
      await close();
    }
  });

  it("normalizes an encoded group name in the 404 message the same way as Express (no sockets)", async () => {
    // An encoded group that does not exist → decoded+trimmed → 404 whose message
    // carries the NORMALIZED group, proving normalization without starting anything.
    const { nest, express, close } = await bootCreatePair(seedGroup);
    try {
      const [e, n] = await Promise.all([
        startGroupReq(express.baseUrl, "team%20space"),
        startGroupReq(nest.baseUrl, "team%20space"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(404);
      expect((n.body as { errors: string[] }).errors[0]).toContain('No rules found in group "team space"');
    } finally {
      await close();
    }
  });

  it("starts a group byte-for-byte like Express via the idempotent already-running path (1 socket, shared manager)", async () => {
    // A started GroupActionResponse has NO volatile field, but starting opens a
    // socket. Open the listener ONCE up front (Test-A bind-retry), then both runtimes
    // hit the idempotent already_running branch — no new sockets, deterministic.
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule({ id: "gr1", name: "Runner", protocol: "tcp", listenHost: "127.0.0.1", listenPort: await getFreeTcpPort(), targetHost: "127.0.0.1", targetPort: 49990, enabled: false, group: "web" });
    const nestApp = await nestWithManager(manager);
    const nest = await startNestServer(nestApp);
    const express = await startHandlerServer(createApp(manager) as unknown as http.RequestListener);
    try {
      await startRuleStable(manager, "gr1", getFreeTcpPort); // open the listener once

      const [e, n] = await Promise.all([
        startGroupReq(express.baseUrl, "web"),
        startGroupReq(nest.baseUrl, "web"),
      ]);
      expect(diffApiResponses(e, n)).toEqual([]);
      expect(n.status).toBe(200);
      expect(n.body).toEqual({
        group: "web",
        action: "start",
        total: 1,
        succeeded: 0,
        skipped: 1,
        failed: 0,
        results: [{ ruleId: "gr1", ruleName: "Runner", status: "skipped", reason: "already_running" }],
      });
    } finally {
      await manager.stopAll();
      await nest.close();
      await express.close();
    }
  });
});

describe("POST /api/forwards/groups/:group/start (Nest) — real cold start (single runtime, sockets cleaned up)", () => {
  it("starts a stopped TCP rule in a group end-to-end (200, started) and GET /api/status reflects it running", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule({ id: "cg1", name: "ColdGroup", protocol: "tcp", listenHost: "127.0.0.1", listenPort: await getFreeTcpPort(), targetHost: "127.0.0.1", targetPort: 49991, enabled: false, group: "cold" });
    const nestApp = await nestWithManager(manager);
    const nest = await startNestServer(nestApp);
    try {
      const response = await startGroupReq(nest.baseUrl, "cold");
      expect(response.status).toBe(200);
      expect(response.body).toEqual({
        group: "cold",
        action: "start",
        total: 1,
        succeeded: 1,
        skipped: 0,
        failed: 0,
        results: [{ ruleId: "cg1", ruleName: "ColdGroup", status: "started" }],
      });
      // The rule is now actually running.
      const status = await fetchApi(nest.baseUrl, "/api/status");
      expect((status.body as Array<{ ruleId: string; running: boolean }>).find((s) => s.ruleId === "cg1")?.running).toBe(true);
    } finally {
      await manager.stopAll(); // close the real listener
      await nest.close();
    }
  });
});
