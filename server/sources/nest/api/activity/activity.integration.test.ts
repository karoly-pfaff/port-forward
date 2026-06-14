import "reflect-metadata";
import type http from "node:http";
import type { INestApplication } from "@nestjs/common";
import type { ForwardRule } from "@portier/shared";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { ActivityStore } from "../../../activity/activity-store.js";
import { createApp } from "../../../api.js";
import { ForwardManager, type RuleStore } from "../../../forward-manager.js";
import { createNestApp } from "../../app.factory.js";
import {
  diffApiResponses,
  fetchApi,
  startHandlerServer,
  startNestServer,
  type ParityServer,
} from "../../testing/api-parity.js";
import { ACTIVITY_STORE } from "./activity.service.js";

/** Minimal RuleStore so the Express app can be built; the activity route never uses it. */
class EmptyStore implements RuleStore {
  async load(): Promise<ForwardRule[]> {
    return [];
  }
  async save(): Promise<void> {
    /* no-op */
  }
}

function seed(store: ActivityStore): void {
  store.add({ type: "rule.started", severity: "success", message: "started", ruleId: "rule-1", ruleName: "Web", protocol: "tcp" });
  store.add({ type: "rule.error", severity: "error", message: "failed", ruleId: "rule-2", ruleName: "DB", protocol: "udp" });
  store.add({ type: "tcp.connection.opened", severity: "info", message: "conn", ruleId: "rule-1", ruleName: "Web", protocol: "tcp" });
}

const BASE = "/api/activity";

describe("GET /api/activity (Nest)", () => {
  let nestApp: INestApplication;
  let nest: ParityServer;
  let express: ParityServer;
  let store: ActivityStore;

  beforeAll(async () => {
    nestApp = await createNestApp();
    // Seed the Nest app's own default store, then share that exact instance with
    // Express so identical data + identical list() logic give byte-for-byte parity.
    store = nestApp.get<ActivityStore>(ACTIVITY_STORE);
    seed(store);
    nest = await startNestServer(nestApp);

    const expressApp = createApp(new ForwardManager(new EmptyStore()), { activity: store });
    express = await startHandlerServer(expressApp as unknown as http.RequestListener);
  });

  afterAll(async () => {
    await nest.close();
    await express.close();
  });

  it.each([
    `${BASE}`,
    `${BASE}?limit=2`,
    `${BASE}?limit=0`, // invalid → default 100
    `${BASE}?limit=abc`, // invalid → default 100
    `${BASE}?limit=600`, // clamped to 500
    `${BASE}?ruleId=rule-1`,
    `${BASE}?type=rule.error`,
    `${BASE}?severity=error`,
    `${BASE}?ruleId=nope`, // no match → []
    `${BASE}?ruleId=rule-1&severity=info`, // combined filters
  ])("matches the existing Express route for %s", async (path) => {
    const [expressResponse, nestResponse] = await Promise.all([
      fetchApi(express.baseUrl, path),
      fetchApi(nest.baseUrl, path),
    ]);

    expect(diffApiResponses(expressResponse, nestResponse)).toEqual([]);
  });

  it("returns 200 with the newest-first events", async () => {
    const response = await fetchApi(nest.baseUrl, BASE);

    expect(response.status).toBe(200);
    expect(response.body).toEqual({ events: store.list({ limit: 100 }) });
  });

  it("filters by ruleId and returns an empty list for an unknown rule", async () => {
    const response = await fetchApi(nest.baseUrl, `${BASE}?ruleId=nope`);

    expect(response.status).toBe(200);
    expect(response.body).toEqual({ events: [] });
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
