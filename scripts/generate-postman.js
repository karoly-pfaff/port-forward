#!/usr/bin/env node
/* global console */
/**
 * Generate the Portier Postman collection + environment from the canonical OpenAPI
 * contract (docs/openapi.json).
 *
 *   node scripts/generate-postman.js
 *
 * Writes (deterministically):
 *   postman/collection.json   — Atomic Endpoint Tests + Happy Path Rule Flow + Negative/Error Tests
 *   postman/environment.json  — variable-driven local environment (host/port/test values)
 *
 * Re-run after any change to docs/openapi.json. `npm run validate:postman` regenerates in
 * memory and fails if the checked-in files are stale.
 */

import { mkdirSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

import {
  buildCollection,
  buildEnvironment,
  loadOpenApi,
  repoRoot,
  serialize,
} from "./library/postman.js";

function main() {
  const openapi = loadOpenApi();
  const postmanDir = resolve(repoRoot, "postman");
  mkdirSync(postmanDir, { recursive: true });

  const collectionPath = resolve(postmanDir, "collection.json");
  const environmentPath = resolve(postmanDir, "environment.json");

  writeFileSync(collectionPath, serialize(buildCollection(openapi)), "utf8");
  writeFileSync(environmentPath, serialize(buildEnvironment()), "utf8");

  console.log("[generate:postman] Wrote postman/collection.json and postman/environment.json");
}

main();
