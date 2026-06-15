import "reflect-metadata";
import type http from "node:http";
import type { INestApplication } from "@nestjs/common";
import { Test } from "@nestjs/testing";
import { getPortAdvisories, type ForwardRule, type ForwardRuleResponse } from "@portier/shared";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { createApp } from "../../../api.js";
import { ForwardManager, type RuleStore } from "../../../forward-manager.js";
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
import { FORWARDS_READER, type ForwardsReader } from "./forwards.reader.js";
import { FORWARD_RULE_CREATOR, FORWARD_RULE_DELETER, FORWARD_RULE_UPDATER } from "./forwards.writer.js";

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

describe("GET /api/forwards (Nest) — scaffold default", () => {
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

/** Binds the (seeded) manager to the reader + creator + updater + deleter so GET reflects POST/PATCH/DELETE. */
async function nestWithManager(manager: ForwardManager): Promise<INestApplication> {
  const moduleRef = await Test.createTestingModule({ imports: [AppModule] })
    .overrideProvider(FORWARDS_READER)
    .useValue(manager)
    .overrideProvider(FORWARD_RULE_CREATOR)
    .useValue(manager)
    .overrideProvider(FORWARD_RULE_UPDATER)
    .useValue(manager)
    .overrideProvider(FORWARD_RULE_DELETER)
    .useValue(manager)
    .compile();
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

describe("POST /api/forwards (Nest) — scaffold default", () => {
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
