import { generateOpenApiDocument, OPENAPI_OUTPUT_PATH, writeOpenApiDocument } from "./openapi.js";

/**
 * Logic-free process entry for `npm run generate:apidoc` — generates the OpenAPI
 * document from the Nest scaffold metadata and writes it to the tracked file.
 * All logic lives in the unit-covered `openapi.ts`; this entry is coverage-
 * excluded (mirroring `main.ts`).
 */
void generateOpenApiDocument()
  .then((doc) => {
    writeOpenApiDocument(OPENAPI_OUTPUT_PATH, doc);
    console.log(`Wrote OpenAPI document to ${OPENAPI_OUTPUT_PATH}`);
  })
  .catch((error: unknown) => {
    console.error(error);
    process.exitCode = 1;
  });
