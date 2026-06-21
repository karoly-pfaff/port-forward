import { copyFileSync, mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import type { INestApplication } from "@nestjs/common";
import { DocumentBuilder, SwaggerModule, type OpenAPIObject } from "@nestjs/swagger";
import { createNestApp } from "../app/app.factory.js";

/**
 * Offline OpenAPI generation for the NestJS server.
 *
 * The document is generated from Nest controller/DTO metadata (`@ApiTags`,
 * `@ApiOperation`, `@Api*Response`, `@ApiProperty`) — not hand-written — so the
 * `/api` surface is the source of truth. Generation never listens on a socket and
 * never starts a listener or mutates runtime state; it only inspects the Nest app's metadata.
 *
 * Artifact placement: the **primary** artifact is server-owned generated output
 * (`server/build/api/openapi.json`), and `docs/openapi.json` is a tracked,
 * reviewable **copy** synced from it. A separate helper copies the primary artifact
 * into a release/package directory during packaging (no regeneration). Regenerate
 * with `npm run apidoc:generate` whenever an endpoint or its DTOs change.
 */

/** Version stamped into the generated document's `info.version` (kept stable for deterministic output). */
export const OPENAPI_DOC_VERSION = "1.19";

/** Path of the directory this module lives in (`server/sources/openapi`). */
const HERE = dirname(fileURLToPath(import.meta.url));

/** Relative path for generated/runtime OpenAPI artifacts under build and release roots. */
export const OPENAPI_RELATIVE_PATH = join("api", "openapi.json");

/** Relative path for the tracked docs copy under `docs/`. */
export const OPENAPI_DOCS_RELATIVE_PATH = "openapi.json";

/** Resolved output paths for the generated OpenAPI artifacts. */
export interface OpenApiPaths {
  /** Primary, server-owned generated artifact (under `server/build`, gitignored). */
  primary: string;
  /** Tracked, reviewable copy synced from the primary artifact. */
  docs: string;
}

/** Resolves the primary (server build) and docs-copy OpenAPI paths relative to the repo. */
export function resolveOpenApiPaths(): OpenApiPaths {
  return {
    // server/sources/openapi -> server/build/api/openapi.json
    primary: join(HERE, "../../build", OPENAPI_RELATIVE_PATH),
    // server/sources/openapi -> docs/openapi.json
    docs: join(HERE, "../../../docs", OPENAPI_DOCS_RELATIVE_PATH),
  };
}

/**
 * Tracked docs copy path — kept as a named export for the drift guard and any
 * tooling that references the reviewed artifact directly.
 */
export const OPENAPI_DOCS_PATH = resolveOpenApiPaths().docs;

/** Builds the OpenAPI document metadata (title/description/version/tags). Deterministic. */
export function buildOpenApiConfig(): Omit<OpenAPIObject, "paths"> {
  return new DocumentBuilder()
    .setTitle("Portier API")
    .setDescription(
      "Portier management API — generated from the NestJS controllers and DTOs that serve the `/api` surface."
    )
    .setVersion(OPENAPI_DOC_VERSION)
    .addTag("health")
    .addTag("ports")
    .addTag("activity")
    .addTag("status")
    .addTag("forwards")
    .addTag("runtime")
    .addTag("config")
    .addTag("connections")
    .build();
}

/**
 * Builds the OpenAPI document by inspecting the Nest app's metadata. Creates the
 * app without listening and **always closes it** afterwards (no sockets, no
 * lifecycle). `createApp` is injectable so a test can observe the clean close;
 * production calls it with the default `createNestApp`.
 */
export async function generateOpenApiDocument(
  createApp: () => Promise<INestApplication> = createNestApp
): Promise<OpenAPIObject> {
  const app = await createApp();
  try {
    return SwaggerModule.createDocument(app, buildOpenApiConfig());
  } finally {
    await app.close();
  }
}

/**
 * Returns a copy of the document with `components.schemas` sorted by name. The
 * schema classes live in per-feature files, so their registration order depends on
 * module-evaluation order, which differs between the `tsx` generator and the
 * esbuild/Vitest transform. Sorting the schema map makes the serialized output
 * deterministic regardless of how generation is invoked (the schema map order is
 * cosmetic — `$ref`s resolve by name).
 */
export function canonicalizeOpenApiDocument(doc: OpenAPIObject): OpenAPIObject {
  const schemas = doc.components?.schemas;
  if (!schemas) {
    return doc;
  }
  const sorted = Object.fromEntries(Object.keys(schemas).sort().map((name) => [name, schemas[name]]));
  return { ...doc, components: { ...doc.components, schemas: sorted } };
}

/** Serializes the document deterministically (sorted schemas, stable JSON + trailing newline) for CI/review. */
export function serializeOpenApiDocument(doc: OpenAPIObject): string {
  return `${JSON.stringify(canonicalizeOpenApiDocument(doc), null, 2)}\n`;
}

/** Writes the serialized document to `filePath`, creating parent directories as needed. */
export function writeOpenApiDocument(filePath: string, doc: OpenAPIObject): void {
  mkdirSync(dirname(filePath), { recursive: true });
  writeFileSync(filePath, serializeOpenApiDocument(doc));
}

/** Copies an existing OpenAPI artifact to `destPath`, creating parent directories as needed. */
function copyOpenApiArtifact(sourcePath: string, destPath: string): void {
  mkdirSync(dirname(destPath), { recursive: true });
  copyFileSync(sourcePath, destPath);
}

/**
 * Writes the OpenAPI document to the primary server-owned artifact, then syncs the
 * tracked docs copy from it (byte-for-byte). Returns the resolved paths. The caller
 * supplies the paths (`generate.ts` passes `resolveOpenApiPaths()`).
 */
export function writeOpenApiArtifacts(doc: OpenAPIObject, paths: OpenApiPaths): OpenApiPaths {
  writeOpenApiDocument(paths.primary, doc);
  copyOpenApiArtifact(paths.primary, paths.docs);
  return paths;
}

/**
 * Copies an already-generated OpenAPI artifact into a release/package directory as
 * `<releaseDir>/api/openapi.json` — an explicit packaging step that never
 * regenerates. Returns the written path. Packaging calls this with the primary
 * artifact path (`resolveOpenApiPaths().primary`) after generation has produced it.
 */
export function copyOpenApiToRelease(releaseDir: string, sourcePath: string): string {
  const destPath = join(releaseDir, OPENAPI_RELATIVE_PATH);
  copyOpenApiArtifact(sourcePath, destPath);
  return destPath;
}
