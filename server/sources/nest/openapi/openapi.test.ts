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

  it("documents POST /api/forwards with its body, 201 response, and 400/409 errors", () => {
    const post = doc.paths["/api/forwards"]?.post;
    expect(post).toBeDefined();
    const body = (post as { requestBody?: { content?: Record<string, { schema?: { $ref?: string } }> } })
      .requestBody;
    expect(body?.content?.["application/json"]?.schema?.$ref).toContain("CreateForwardRuleBodyDto");
    const responses = post?.responses ?? {};
    const created = responses["201"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(created?.content?.["application/json"]?.schema?.$ref).toContain("ForwardRuleResponseDto");
    const badRequest = responses["400"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(badRequest?.content?.["application/json"]?.schema?.$ref).toContain("ApiErrorResponseDto");
    const conflict = responses["409"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(conflict?.content?.["application/json"]?.schema?.$ref).toContain("ApiErrorResponseDto");
    expect(doc.components?.schemas?.CreateForwardRuleBodyDto).toBeDefined();
  });

  it("documents PATCH /api/forwards/{id} with its path param, body, 200 response, and 400/404/409 errors", () => {
    const patch = doc.paths["/api/forwards/{id}"]?.patch;
    expect(patch).toBeDefined();
    const params = (patch?.parameters ?? []) as Array<{ name: string; in: string }>;
    expect(params.some((p) => p.name === "id" && p.in === "path")).toBe(true);
    const body = (patch as { requestBody?: { content?: Record<string, { schema?: { $ref?: string } }> } })
      .requestBody;
    expect(body?.content?.["application/json"]?.schema?.$ref).toContain("UpdateForwardRuleBodyDto");
    const responses = patch?.responses ?? {};
    const ok = responses["200"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(ok?.content?.["application/json"]?.schema?.$ref).toContain("ForwardRuleResponseDto");
    for (const status of ["400", "404", "409"]) {
      const r = responses[status] as { content?: Record<string, { schema?: { $ref?: string } }> };
      expect(r?.content?.["application/json"]?.schema?.$ref, `status ${status}`).toContain("ApiErrorResponseDto");
    }
    expect(doc.components?.schemas?.UpdateForwardRuleBodyDto).toBeDefined();
  });

  it("documents POST /api/forwards/groups/{group}/stop with its path param, 200 summary, and 400/404 errors", () => {
    const stopGroup = doc.paths["/api/forwards/groups/{group}/stop"]?.post;
    expect(stopGroup).toBeDefined();
    const params = (stopGroup?.parameters ?? []) as Array<{ name: string; in: string }>;
    expect(params.some((p) => p.name === "group" && p.in === "path")).toBe(true);
    const responses = stopGroup?.responses ?? {};
    const ok = responses["200"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(ok?.content?.["application/json"]?.schema?.$ref).toContain("GroupActionResponseDto");
    for (const status of ["400", "404"]) {
      const r = responses[status] as { content?: Record<string, { schema?: { $ref?: string } }> };
      expect(r?.content?.["application/json"]?.schema?.$ref, `status ${status}`).toContain("ApiErrorResponseDto");
    }
    // The summary schema references the per-rule result item schema.
    const schemas = doc.components?.schemas ?? {};
    expect(schemas.GroupActionResponseDto).toBeDefined();
    expect(schemas.GroupActionResultDto).toBeDefined();
  });

  it("documents POST /api/forwards/{id}/diagnose with its path param, 200 result response, and 404 error", () => {
    const diagnose = doc.paths["/api/forwards/{id}/diagnose"]?.post;
    expect(diagnose).toBeDefined();
    const params = (diagnose?.parameters ?? []) as Array<{ name: string; in: string }>;
    expect(params.some((p) => p.name === "id" && p.in === "path")).toBe(true);
    const responses = diagnose?.responses ?? {};
    const ok = responses["200"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(ok?.content?.["application/json"]?.schema?.$ref).toContain("RuleDiagnosticsResultDto");
    const notFound = responses["404"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(notFound?.content?.["application/json"]?.schema?.$ref).toContain("ApiErrorResponseDto");
    // The result schema references the check + summary item schemas.
    const schemas = doc.components?.schemas ?? {};
    expect(schemas.RuleDiagnosticsResultDto).toBeDefined();
    expect(schemas.DiagnosticCheckDto).toBeDefined();
    expect(schemas.DiagnosticSummaryDto).toBeDefined();
  });

  it("documents POST /api/forwards/reorder with its body, 200 list response, and 400/404 errors", () => {
    const reorder = doc.paths["/api/forwards/reorder"]?.post;
    expect(reorder).toBeDefined();
    const body = (reorder as { requestBody?: { content?: Record<string, { schema?: { $ref?: string } }> } })
      .requestBody;
    expect(body?.content?.["application/json"]?.schema?.$ref).toContain("ReorderForwardRulesBodyDto");
    const responses = reorder?.responses ?? {};
    const ok = responses["200"] as { content?: Record<string, { schema?: { type?: string; items?: { $ref?: string } } }> };
    const okSchema = ok?.content?.["application/json"]?.schema;
    expect(okSchema?.type).toBe("array");
    expect(okSchema?.items?.$ref).toContain("ForwardRuleResponseDto");
    for (const status of ["400", "404"]) {
      const r = responses[status] as { content?: Record<string, { schema?: { $ref?: string } }> };
      expect(r?.content?.["application/json"]?.schema?.$ref, `status ${status}`).toContain("ApiErrorResponseDto");
    }
    // The reorder body DTO is a real validated schema (ids: string[]).
    const schema = doc.components?.schemas?.ReorderForwardRulesBodyDto as
      | { properties?: Record<string, { type?: string; items?: { type?: string } }> }
      | undefined;
    expect(schema?.properties?.ids?.type).toBe("array");
    expect(schema?.properties?.ids?.items?.type).toBe("string");
  });

  it("documents POST /api/forwards/{id}/start with its path param, 200 status response, and 404 error", () => {
    const start = doc.paths["/api/forwards/{id}/start"]?.post;
    expect(start).toBeDefined();
    const params = (start?.parameters ?? []) as Array<{ name: string; in: string }>;
    expect(params.some((p) => p.name === "id" && p.in === "path")).toBe(true);
    const responses = start?.responses ?? {};
    const ok = responses["200"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(ok?.content?.["application/json"]?.schema?.$ref).toContain("ForwardStatusDto");
    const notFound = responses["404"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(notFound?.content?.["application/json"]?.schema?.$ref).toContain("ApiErrorResponseDto");
  });

  it("documents POST /api/forwards/{id}/stop with its path param, 200 status response, and 404 error", () => {
    const stop = doc.paths["/api/forwards/{id}/stop"]?.post;
    expect(stop).toBeDefined();
    const params = (stop?.parameters ?? []) as Array<{ name: string; in: string }>;
    expect(params.some((p) => p.name === "id" && p.in === "path")).toBe(true);
    const responses = stop?.responses ?? {};
    const ok = responses["200"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(ok?.content?.["application/json"]?.schema?.$ref).toContain("ForwardStatusDto");
    const notFound = responses["404"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(notFound?.content?.["application/json"]?.schema?.$ref).toContain("ApiErrorResponseDto");
  });

  it("documents DELETE /api/forwards/{id} with its path param, 204 no-body, and 404 error", () => {
    const del = doc.paths["/api/forwards/{id}"]?.delete;
    expect(del).toBeDefined();
    const params = (del?.parameters ?? []) as Array<{ name: string; in: string }>;
    expect(params.some((p) => p.name === "id" && p.in === "path")).toBe(true);
    const responses = del?.responses ?? {};
    expect(responses["204"]).toBeDefined();
    expect((responses["204"] as { content?: unknown }).content).toBeUndefined(); // no body
    const notFound = responses["404"] as { content?: Record<string, { schema?: { $ref?: string } }> };
    expect(notFound?.content?.["application/json"]?.schema?.$ref).toContain("ApiErrorResponseDto");
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
