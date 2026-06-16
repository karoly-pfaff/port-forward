import "reflect-metadata";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import type http from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { INestApplication } from "@nestjs/common";
import type { NestExpressApplication } from "@nestjs/platform-express";
import { Test } from "@nestjs/testing";
import type { ForwardRule } from "@portier/shared";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { createApp } from "../../legacy/api.js";
import { ForwardManager, type RuleStore } from "../../forward-manager.js";
import { AppModule } from "../app.module.js";
import { startHandlerServer, startNestServer, type ParityServer } from "../testing/api-parity.js";
import { STATIC_FALLBACK, configureStaticAssets, createStaticFallback } from "./static-serving.js";

const INDEX_HTML = "<html><body>Portier</body></html>";
const APP_JS = "console.log('portier');";

class MemoryStore implements RuleStore {
  constructor(private rules: ForwardRule[] = []) {}
  async load(): Promise<ForwardRule[]> {
    return this.rules;
  }
  async save(rules: ForwardRule[]): Promise<void> {
    this.rules = rules;
  }
}

interface RawResponse {
  status: number;
  text: string;
}

async function fetchRaw(baseUrl: string, path: string): Promise<RawResponse> {
  const response = await fetch(`${baseUrl}${path}`);
  return { status: response.status, text: await response.text() };
}

function makeClientDir(): string {
  const dir = mkdtempSync(join(tmpdir(), "portier-static-int-"));
  writeFileSync(join(dir, "index.html"), INDEX_HTML);
  mkdirSync(join(dir, "assets"));
  writeFileSync(join(dir, "assets", "app.js"), APP_JS);
  return dir;
}

/** Boots a Nest app with static assets + the SPA fallback wired to `staticDir`. */
async function nestWithStatic(staticDir: string): Promise<INestApplication> {
  const moduleRef = await Test.createTestingModule({ imports: [AppModule] })
    .overrideProvider(STATIC_FALLBACK)
    .useValue(createStaticFallback(staticDir))
    .compile();
  const app = moduleRef.createNestApplication<NestExpressApplication>({ logger: false });
  configureStaticAssets(app, staticDir);
  return app;
}

describe("static client serving (Nest) — parity with Express", () => {
  let staticDir: string;
  let nest: ParityServer;
  let express: ParityServer;

  beforeAll(async () => {
    staticDir = makeClientDir();
    const nestApp = await nestWithStatic(staticDir);
    nest = await startNestServer(nestApp);
    express = await startHandlerServer(
      createApp(new ForwardManager(new MemoryStore()), { staticClientDir: staticDir }) as unknown as http.RequestListener
    );
  });

  afterAll(async () => {
    await nest.close();
    await express.close();
    rmSync(staticDir, { recursive: true, force: true });
  });

  it.each([
    ["root /", "/"],
    ["a SPA route", "/some/spa/route"],
    ["a deep SPA route", "/forwards/123/edit"],
    ["a real asset", "/assets/app.js"],
    ["a missing asset (SPA fallback)", "/assets/missing.js"],
    ["an unmatched /api route (JSON envelope)", "/api/not-migrated"],
  ])("serves %s identically to Express", async (_label, path) => {
    const [e, n] = await Promise.all([fetchRaw(express.baseUrl, path), fetchRaw(nest.baseUrl, path)]);
    expect(n).toEqual(e);
  });

  it("serves the SPA index for the root and SPA routes", async () => {
    expect((await fetchRaw(nest.baseUrl, "/")).text).toContain("Portier");
    expect((await fetchRaw(nest.baseUrl, "/some/spa/route")).text).toContain("Portier");
  });

  it("serves real asset content", async () => {
    const asset = await fetchRaw(nest.baseUrl, "/assets/app.js");
    expect(asset.status).toBe(200);
    expect(asset.text).toContain("portier");
  });

  it("keeps the /api error envelope intact (no static interference)", async () => {
    const response = await fetch(`${nest.baseUrl}/api/not-migrated`);
    expect(response.status).toBe(404);
    expect(await response.json()).toEqual({ errors: ["API route was not found."] });
  });

  it("keeps the API usable while static serving is enabled", async () => {
    const response = await fetch(`${nest.baseUrl}/api/forwards`);
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual([]);
  });
});

describe("static client serving (Nest) — disabled (no static dir)", () => {
  let nest: ParityServer;
  let express: ParityServer;

  beforeAll(async () => {
    // Default: no static assets configured, STATIC_FALLBACK is the disabled default.
    const moduleRef = await Test.createTestingModule({ imports: [AppModule] }).compile();
    const nestApp = moduleRef.createNestApplication({ logger: false });
    nest = await startNestServer(nestApp);
    express = await startHandlerServer(
      createApp(new ForwardManager(new MemoryStore()), {
        staticClientDir: "/nonexistent-portier-static-dir",
      }) as unknown as http.RequestListener
    );
  });

  afterAll(async () => {
    await nest.close();
    await express.close();
  });

  it("keeps the API usable without a static client (both runtimes)", async () => {
    const [e, n] = await Promise.all([
      fetch(`${express.baseUrl}/api/forwards`),
      fetch(`${nest.baseUrl}/api/forwards`),
    ]);
    expect(e.status).toBe(200);
    expect(await e.json()).toEqual([]);
    expect(n.status).toBe(200);
    expect(await n.json()).toEqual([]);
  });

  it("does not serve the SPA index for a non-API route when static is disabled (both runtimes)", async () => {
    const [e, n] = await Promise.all([
      fetchRaw(express.baseUrl, "/some/spa/route"),
      fetchRaw(nest.baseUrl, "/some/spa/route"),
    ]);
    // Both 404; neither returns the SPA index. (The exact non-API 404 body shape
    // differs — Express's default HTML vs NestJS's default JSON — a documented
    // pre-existing documented boundary, not asserted here.)
    expect(e.status).toBe(404);
    expect(e.text).not.toContain("Portier");
    expect(n.status).toBe(404);
    expect(n.text).not.toContain("Portier");
  });
});
