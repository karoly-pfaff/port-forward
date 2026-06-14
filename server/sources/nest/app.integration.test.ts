import "reflect-metadata";
import type { INestApplication } from "@nestjs/common";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { createNestApp } from "./app.factory.js";

/**
 * Boots the real NestJS scaffold (DI graph, routing, global filter) on an
 * ephemeral loopback port and exercises it over HTTP — proving the wiring, not
 * just the units. This is the only test that starts a listener.
 */
describe("NestJS scaffold app (integration)", () => {
  let app: INestApplication;
  let baseUrl: string;

  beforeAll(async () => {
    app = await createNestApp();
    await app.listen(0, "127.0.0.1");
    baseUrl = await app.getUrl();
  });

  afterAll(async () => {
    await app.close();
  });

  it("serves GET /health with the documented scaffold shape", async () => {
    const response = await fetch(`${baseUrl}/health`);

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ ok: true, server: "node", name: "Portier" });
  });

  it("returns the contract 404 envelope for an unknown /api route", async () => {
    // Use a path that is not (yet) migrated into the Nest scaffold.
    const response = await fetch(`${baseUrl}/api/not-migrated`);

    expect(response.status).toBe(404);
    expect(await response.json()).toEqual({ errors: ["API route was not found."] });
  });

  it("returns the contract 404 envelope for the bare /api namespace", async () => {
    const response = await fetch(`${baseUrl}/api`);

    expect(response.status).toBe(404);
    expect(await response.json()).toEqual({ errors: ["API route was not found."] });
  });

  it("does not leak the contract envelope onto non-API 404s", async () => {
    const response = await fetch(`${baseUrl}/not-a-real-page`);

    expect(response.status).toBe(404);
    const body = (await response.json()) as Record<string, unknown>;
    expect(body).not.toHaveProperty("errors");
  });

  it("does not bind 0.0.0.0 by default", () => {
    expect(baseUrl).toContain("127.0.0.1");
  });
});
