import { generateOpenApiDocument, resolveOpenApiPaths, writeOpenApiArtifacts } from "./openapi.js";

/**
 * Process entry for `npm run generate:apidoc` — generates the OpenAPI document
 * from the Nest controller/DTO metadata, writes the primary server-owned artifact
 * (`server/build/api/openapi.json`), and syncs the tracked docs copy
 * (`docs/api/openapi.json`) from it. All logic lives in the unit-covered
 * `openapi.ts`; this logic-free process entry is coverage-excluded.
 */
void generateOpenApiDocument()
  .then((doc) => {
    const paths = writeOpenApiArtifacts(doc, resolveOpenApiPaths());
    console.log(`Wrote OpenAPI document to ${paths.primary}`);
    console.log(`Synced docs copy to ${paths.docs}`);
  })
  .catch((error: unknown) => {
    console.error(error);
    process.exitCode = 1;
  });
