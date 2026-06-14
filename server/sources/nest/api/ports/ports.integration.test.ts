import "reflect-metadata";
import type http from "node:http";
import type { INestApplication } from "@nestjs/common";
import type { ForwardRule } from "@portier/shared";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
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

/** Minimal RuleStore so the Express app can be built; the advisory route never uses it. */
class EmptyStore implements RuleStore {
  async load(): Promise<ForwardRule[]> {
    return [];
  }
  async save(): Promise<void> {
    /* no-op */
  }
}

const BASE = "/api/ports/advisory";

describe("GET /api/ports/advisory (Nest)", () => {
  let nestApp: INestApplication;
  let nest: ParityServer;
  let express: ParityServer;

  beforeAll(async () => {
    nestApp = await createNestApp();
    nest = await startNestServer(nestApp);
    const expressApp = createApp(new ForwardManager(new EmptyStore()));
    express = await startHandlerServer(expressApp as unknown as http.RequestListener);
  });

  afterAll(async () => {
    await nest.close();
    await express.close();
  });

  it.each([
    `${BASE}?port=48001&listenHost=0.0.0.0&purpose=forward`,
    `${BASE}?port=47831&purpose=management`,
    `${BASE}?port=80&purpose=forward`,
    `${BASE}?port=70000&purpose=forward`, // invalid port
    `${BASE}?port=1.5&purpose=forward`, // non-integer port
    `${BASE}?port=abc&purpose=forward`, // non-numeric port
    `${BASE}?port=48001&purpose=bogus`, // invalid purpose
    `${BASE}?purpose=forward`, // missing port
    `${BASE}?port=48001&purpose=forward&unexpected=1`, // extra query param is ignored, not rejected
  ])("matches the existing Express route for %s", async (path) => {
    const [expressResponse, nestResponse] = await Promise.all([
      fetchApi(express.baseUrl, path),
      fetchApi(nest.baseUrl, path),
    ]);

    expect(diffApiResponses(expressResponse, nestResponse)).toEqual([]);
  });

  it("returns 200 and a LAN exposure advisory for a 0.0.0.0 forward", async () => {
    const response = await fetchApi(nest.baseUrl, `${BASE}?port=48001&listenHost=0.0.0.0&purpose=forward`);

    expect(response.status).toBe(200);
    expect(response.body).toEqual([
      {
        code: "LAN_EXPOSURE",
        severity: "warning",
        message:
          "Listening on 0.0.0.0 exposes this forwarded port on all interfaces. Other LAN devices may be able to connect if firewall settings allow it.",
      },
    ]);
  });

  it("returns the 400 envelope for an invalid port", async () => {
    const response = await fetchApi(nest.baseUrl, `${BASE}?port=70000&purpose=forward`);

    expect(response.status).toBe(400);
    expect(response.body).toEqual({ errors: ["port must be an integer from 1 to 65535."] });
  });

  it("returns the 400 envelope for an invalid purpose", async () => {
    const response = await fetchApi(nest.baseUrl, `${BASE}?port=48001&purpose=bogus`);

    expect(response.status).toBe(400);
    expect(response.body).toEqual({ errors: ["purpose must be management or forward."] });
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
