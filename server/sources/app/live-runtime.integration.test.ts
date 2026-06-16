import "reflect-metadata";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { INestApplication } from "@nestjs/common";
import type { ActivityEvent, ForwardRule, ForwardRuleResponse, ForwardStatus, RuntimeInfo } from "@portier/shared";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { ActivityStore } from "../activity/activity-store.js";
import { ForwardManager, type RuleStore } from "../forward-manager.js";
import type { RuntimeInfoOptions } from "../runtime-info.js";
import { createNestApp } from "./app.factory.js";
import type { AppRuntime } from "../common/runtime-context.js";
import { createStaticFallback } from "../static/static-serving.js";

/**
 * Proves the NestJS server is functionally live as the default runtime: built with
 * a real `ForwardManager`/`ActivityStore`/runtime info/static client via
 * `createNestApp(runtime)`, the API endpoints reflect the live state (not the
 * shadow-only empty defaults), writes mutate the live manager, activity reflects the
 * live store, and static + SPA + the `/api` envelope all work. No fixed ports; the
 * seeded rule is `enabled:false` (socket-free) and the manager is stopped in `finally`.
 */

class MemoryStore implements RuleStore {
  constructor(private rules: ForwardRule[] = []) {}
  async load(): Promise<ForwardRule[]> {
    return this.rules;
  }
  async save(rules: ForwardRule[]): Promise<void> {
    this.rules = rules;
  }
}

const SEED_RULE: ForwardRule = {
  id: "live-1",
  name: "Web",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48010,
  targetHost: "127.0.0.1",
  targetPort: 8080,
  enabled: false,
};

let app: INestApplication;
let manager: ForwardManager;
let activity: ActivityStore;
let staticDir: string;
let baseUrl: string;

async function get(path: string): Promise<{ status: number; text: string }> {
  const res = await fetch(`${baseUrl}${path}`);
  return { status: res.status, text: await res.text() };
}

async function getJson<T>(path: string): Promise<T> {
  return JSON.parse((await get(path)).text) as T;
}

beforeAll(async () => {
  staticDir = mkdtempSync(join(tmpdir(), "portier-live-static-"));
  writeFileSync(join(staticDir, "index.html"), "<html><body>Portier</body></html>");
  mkdirSync(join(staticDir, "assets"));
  writeFileSync(join(staticDir, "assets", "app.js"), "console.log('live');");

  activity = new ActivityStore();
  manager = new ForwardManager(new MemoryStore([SEED_RULE]), activity);
  await manager.loadAndStartEnabled(); // enabled:false → no sockets

  const startedAt = new Date("2026-06-16T08:00:00.000Z");
  const runtimeInfo: RuntimeInfoOptions = {
    version: "9.9.9-live",
    managementHost: "127.0.0.1",
    managementPort: 47831,
    configPath: "/tmp/live-rules.json",
    staticDir,
    serviceMode: false,
    startedAt,
  };
  const runtime: AppRuntime = {
    manager,
    activity,
    runtimeInfoReader: { options: () => runtimeInfo, startedAt: () => startedAt },
    staticFallback: createStaticFallback(staticDir),
    staticClientDir: staticDir,
  };

  app = await createNestApp(runtime);
  await app.listen(0, "127.0.0.1");
  baseUrl = await app.getUrl();
});

afterAll(async () => {
  await manager.stopAll();
  await app.close();
  rmSync(staticDir, { recursive: true, force: true });
});

describe("NestJS default runtime — live dependency wiring", () => {
  it("GET /api/forwards reflects the live manager (not the empty shadow default)", async () => {
    const rules = await getJson<ForwardRuleResponse[]>("/api/forwards");
    expect(rules.map((r) => r.id)).toEqual(["live-1"]);
    expect(rules[0].advisories).toBeDefined();
  });

  it("GET /api/status reflects the live manager", async () => {
    const statuses = await getJson<ForwardStatus[]>("/api/status");
    expect(statuses.map((s) => s.ruleId)).toEqual(["live-1"]);
    expect(statuses[0].running).toBe(false);
  });

  it("GET /api/config/export reflects the live manager", async () => {
    const config = await getJson<{ version: string; rules: ForwardRule[] }>("/api/config/export");
    expect(config.version).toBe("1");
    expect(config.rules.map((r) => r.id)).toEqual(["live-1"]);
  });

  it("GET /api/runtime reflects the live runtime info", async () => {
    const info = await getJson<RuntimeInfo>("/api/runtime");
    expect(info.version).toBe("9.9.9-live");
    expect(info.runtime).toBe("node");
    expect(info.configPath).toBe("/tmp/live-rules.json");
    expect(typeof info.uptimeSeconds).toBe("number");
  });

  it("GET /api/connections works against the live manager", async () => {
    const snapshot = await getJson<{ tcpConnections: unknown[]; udpSessions: unknown[] }>("/api/connections");
    expect(Array.isArray(snapshot.tcpConnections)).toBe(true);
    expect(Array.isArray(snapshot.udpSessions)).toBe(true);
  });

  it("GET /api/activity reflects the live activity store", async () => {
    // The endpoint reads the SAME store instance the runtime was built with: an
    // event added to it directly appears at the endpoint.
    activity.add({ type: "rule.created", severity: "info", message: "live wiring probe", ruleId: "live-1" });
    const { events } = await getJson<{ events: ActivityEvent[] }>("/api/activity");
    expect(events.some((e) => e.message === "live wiring probe" && e.ruleId === "live-1")).toBe(true);
  });

  it("a write (POST /api/forwards) mutates the live manager", async () => {
    const res = await fetch(`${baseUrl}/api/forwards`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        id: "live-2",
        name: "Api",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort: 48011,
        targetHost: "127.0.0.1",
        targetPort: 9090,
        enabled: false,
      }),
    });
    expect(res.status).toBe(201);
    // The live manager now holds both rules.
    expect(manager.listRules().map((r) => r.id).sort()).toEqual(["live-1", "live-2"]);
    const rules = await getJson<ForwardRuleResponse[]>("/api/forwards");
    expect(rules.map((r) => r.id).sort()).toEqual(["live-1", "live-2"]);
  });

  it("serves the static client root and SPA fallback", async () => {
    expect((await get("/")).text).toContain("Portier");
    expect((await get("/dashboard")).text).toContain("Portier");
    const asset = await get("/assets/app.js");
    expect(asset.status).toBe(200);
    expect(asset.text).toContain("live");
  });

  it("keeps the /api/* envelope for an unknown route", async () => {
    const res = await get("/api/not-migrated");
    expect(res.status).toBe(404);
    expect(JSON.parse(res.text)).toEqual({ errors: ["API route was not found."] });
  });

  it("serves the /health probe", async () => {
    expect(JSON.parse((await get("/health")).text)).toEqual({ ok: true, server: "node", name: "Portier" });
  });
});
