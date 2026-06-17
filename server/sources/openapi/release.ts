import { copyOpenApiToRelease, resolveOpenApiPaths } from "./openapi.js";

/**
 * Process entry for `npm run apidoc:release -- <releaseDir>` — copies the
 * already-generated primary OpenAPI artifact (`server/build/api/openapi.json`)
 * into a release/package directory as `<releaseDir>/api/openapi.json`. It never
 * regenerates the document; packaging runs `apidoc:generate` first so the primary
 * artifact exists. All logic lives in the unit-covered `openapi.ts`; this
 * logic-free process entry is coverage-excluded.
 */
const releaseDir = process.argv[2];
if (!releaseDir) {
  console.error("usage: release.ts <releaseDir>");
  process.exit(2);
}

try {
  const dest = copyOpenApiToRelease(releaseDir, resolveOpenApiPaths().primary);
  console.log(`Copied OpenAPI document to ${dest}`);
} catch (error) {
  console.error(error);
  process.exit(1);
}
