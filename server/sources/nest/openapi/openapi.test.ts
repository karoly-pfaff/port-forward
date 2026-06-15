import "reflect-metadata";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { OpenAPIObject } from "@nestjs/swagger";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { createNestApp } from "../app.factory.js";
import {
  buildOpenApiConfig,
  generateOpenApiDocument,
  OPENAPI_DOC_VERSION,
  OPENAPI_OUTPUT_PATH,
  serializeOpenApiDocument,
  writeOpenApiDocument,
} from "./openapi.js";

describe("buildOpenApiConfig", () => {
  it("sets the title, version, and the migrated-surface tags", () => {
    const config = buildOpenApiConfig();
    expect(config.info.title).toBe("Portier API");
    expect(config.info.version).toBe(OPENAPI_DOC_VERSION);
    const tags = (config.tags ?? []).map((t) => t.name);
    expect(tags).toEqual(["health", "ports", "activity", "status", "forwards", "runtime", "config", "connections"]);
  });
});

describe("generateOpenApiDocument", () => {
  let doc: OpenAPIObject;

  beforeAll(async () => {
    doc = await generateOpenApiDocument();
  });

  it("is a valid OpenAPI 3 document", () => {
    expect(doc.openapi).toMatch(/^3\./);
    expect(doc.info.title).toBe("Portier API");
    expect(doc.paths).toBeTypeOf("object");
    expect(doc.components?.schemas).toBeTypeOf("object");
  });

  it("documents every migrated route and method", () => {
    expect(doc.paths["/health"]?.get).toBeDefined();
    expect(doc.paths["/api/ports/advisory"]?.get).toBeDefined();
    expect(doc.paths["/api/activity"]?.get).toBeDefined();
    expect(doc.paths["/api/activity"]?.delete).toBeDefined();
    expect(doc.paths["/api/status"]?.get).toBeDefined();
    expect(doc.paths["/api/forwards"]?.get).toBeDefined();
    expect(doc.paths["/api/runtime"]?.get).toBeDefined();
    expect(doc.paths["/api/config/export"]?.get).toBeDefined();
    expect(doc.paths["/api/connections"]?.get).toBeDefined();
  });

  it("documents the error envelope schema (ApiErrorResponseDto = { errors: string[] })", () => {
    const schema = doc.components?.schemas?.ApiErrorResponseDto as Record<string, unknown> | undefined;
    expect(schema).toBeDefined();
    const props = (schema as { properties: Record<string, { type: string; items?: { type: string } }> }).properties;
    expect(props.errors.type).toBe("array");
    expect(props.errors.items?.type).toBe("string");
  });

  it("documents response schemas for each migrated endpoint", () => {
    const schemas = doc.components?.schemas ?? {};
    for (const name of [
      "ActivityListResponseDto",
      "ForwardStatusDto",
      "ForwardRuleResponseDto",
      "RuntimeInfoResponseDto",
      "ConfigExportResponseDto",
      "ConnectionsResponseDto",
      "PortAdvisoryDto",
      "TcpConnectionInfoDto",
      "UdpSessionInfoDto",
      "RuleLiveSummaryDto",
    ]) {
      expect(schemas[name], `schema ${name}`).toBeDefined();
    }
  });

  it("documents DELETE /api/activity as 204 with no body", () => {
    const del = doc.paths["/api/activity"]?.delete;
    expect(del?.responses?.["204"]).toBeDefined();
    // 204 must carry no content (no response body).
    expect((del?.responses?.["204"] as { content?: unknown }).content).toBeUndefined();
  });

  it("documents GET /api/activity query parameters (despite endpoint-local coercion)", () => {
    const get = doc.paths["/api/activity"]?.get;
    const params = (get?.parameters ?? []) as Array<{ name: string; in: string; required?: boolean }>;
    const byName = Object.fromEntries(params.map((p) => [p.name, p]));
    // The query has no validation DTO (silent coercion), so the params are documented
    // explicitly via @ApiQuery — all optional, never rejected.
    for (const name of ["limit", "ruleId", "type", "severity"]) {
      expect(byName[name]?.in, `param ${name}`).toBe("query");
      expect(byName[name]?.required, `param ${name} optional`).toBeFalsy();
    }
    // Response is the activity-list envelope.
    const ok = get?.responses?.["200"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(ok?.content?.["application/json"]?.schema?.$ref).toContain("ActivityListResponseDto");
  });

  it("documents the volatile read endpoints with their response schemas", () => {
    const refOf = (path: string): string | undefined =>
      (
        doc.paths[path]?.get?.responses?.["200"] as
          | { content?: Record<string, { schema?: { $ref?: string } }> }
          | undefined
      )?.content?.["application/json"]?.schema?.$ref;
    expect(refOf("/api/runtime")).toContain("RuntimeInfoResponseDto");
    expect(refOf("/api/config/export")).toContain("ConfigExportResponseDto");
    expect(refOf("/api/connections")).toContain("ConnectionsResponseDto");
  });

  it("documents GET /api/ports/advisory query parameters and the 400 error response", () => {
    const get = doc.paths["/api/ports/advisory"]?.get;
    const params = (get?.parameters ?? []) as Array<{ name: string; in: string; required?: boolean }>;
    const byName = Object.fromEntries(params.map((p) => [p.name, p]));
    expect(byName.port?.in).toBe("query");
    expect(byName.port?.required).toBe(true);
    expect(byName.purpose?.required).toBe(true);
    expect(byName.listenHost?.required).toBeFalsy();
    const badRequest = get?.responses?.["400"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(badRequest?.content?.["application/json"]?.schema?.$ref).toContain("ApiErrorResponseDto");
  });

  it("documents the GET /api/ports/advisory 200 response as an array of PortAdvisoryDto", () => {
    const ok = doc.paths["/api/ports/advisory"]?.get?.responses?.["200"] as {
      content?: Record<string, { schema?: { type?: string; items?: { $ref?: string } } }>;
    };
    const schema = ok?.content?.["application/json"]?.schema;
    expect(schema?.type).toBe("array");
    expect(schema?.items?.$ref).toContain("PortAdvisoryDto");
  });
});

describe("generateOpenApiDocument — lifecycle", () => {
  it("closes the Nest app cleanly after generating (no leaked listener)", async () => {
    let closeSpy: ReturnType<typeof vi.spyOn> | undefined;
    const factory = async () => {
      const app = await createNestApp();
      closeSpy = vi.spyOn(app, "close");
      return app;
    };

    const doc = await generateOpenApiDocument(factory);

    expect(doc.openapi).toMatch(/^3\./);
    expect(closeSpy).toHaveBeenCalledOnce();
  });
});

describe("serializeOpenApiDocument", () => {
  it("produces valid JSON with a trailing newline and is deterministic", async () => {
    const [a, b] = await Promise.all([generateOpenApiDocument(), generateOpenApiDocument()]);
    const sa = serializeOpenApiDocument(a);
    const sb = serializeOpenApiDocument(b);
    expect(sa.endsWith("\n")).toBe(true);
    expect(() => JSON.parse(sa) as unknown).not.toThrow();
    expect(sa).toBe(sb); // deterministic across runs
  });
});

describe("writeOpenApiDocument", () => {
  it("writes the serialized document to disk (creating parent dirs)", async () => {
    const dir = mkdtempSync(join(tmpdir(), "portier-openapi-"));
    const file = join(dir, "nested", "openapi.json");
    try {
      const doc = await generateOpenApiDocument();
      writeOpenApiDocument(file, doc);
      expect(readFileSync(file, "utf8")).toBe(serializeOpenApiDocument(doc));
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });
});

describe("tracked docs/api/openapi.json", () => {
  it("is up to date with generation (run `npm run generate:apidoc` if this fails)", async () => {
    const tracked = readFileSync(OPENAPI_OUTPUT_PATH, "utf8");
    const fresh = serializeOpenApiDocument(await generateOpenApiDocument());
    expect(tracked).toBe(fresh);
  });
});
