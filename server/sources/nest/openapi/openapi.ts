import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { DocumentBuilder, SwaggerModule, type OpenAPIObject } from "@nestjs/swagger";
import { createNestApp } from "../app.factory.js";

/**
 * Offline OpenAPI generation for the NestJS scaffold.
 *
 * The document is generated from Nest controller/DTO metadata (`@ApiTags`,
 * `@ApiOperation`, `@Api*Response`, `@ApiProperty`) — not hand-written — so the
 * migrated `/api` surface is the source of truth. Generation never listens on a
 * socket and never changes Express (the default runtime); it only inspects the
 * Nest app's metadata. Output is written to a tracked file
 * (`docs/api/openapi.json`) so it can be reviewed/versioned; regenerate it with
 * `npm run generate:apidoc` whenever a migrated endpoint or its DTOs change.
 */

/** Version stamped into the generated document's `info.version` (kept stable for deterministic output). */
export const OPENAPI_DOC_VERSION = "1.14";

/** Tracked output path for the generated document, resolved relative to the repo root. */
export const OPENAPI_OUTPUT_PATH = join(
  dirname(fileURLToPath(import.meta.url)),
  "../../../../docs/api/openapi.json"
);

/** Builds the OpenAPI document metadata (title/description/version/tags). Deterministic. */
export function buildOpenApiConfig(): Omit<OpenAPIObject, "paths"> {
  return new DocumentBuilder()
    .setTitle("Portier API")
    .setDescription(
      "Portier management API — NestJS-generated documentation for the migrated read surface. " +
        "The Express server remains the default active runtime; these routes are also served by " +
        "the NestJS scaffold under `npm run start:nest`."
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
 * app without listening and closes it afterwards (no sockets, no lifecycle).
 */
export async function generateOpenApiDocument(): Promise<OpenAPIObject> {
  const app = await createNestApp();
  try {
    return SwaggerModule.createDocument(app, buildOpenApiConfig());
  } finally {
    await app.close();
  }
}

/** Serializes the document deterministically (stable JSON + trailing newline) for CI/review. */
export function serializeOpenApiDocument(doc: OpenAPIObject): string {
  return `${JSON.stringify(doc, null, 2)}\n`;
}

/** Writes the serialized document to `filePath`, creating parent directories as needed. */
export function writeOpenApiDocument(filePath: string, doc: OpenAPIObject): void {
  mkdirSync(dirname(filePath), { recursive: true });
  writeFileSync(filePath, serializeOpenApiDocument(doc));
}
