#!/usr/bin/env node
/* global console, process */
/**
 * Validate the checked-in Portier Postman artifacts against the canonical OpenAPI contract
 * and a set of safety/privacy invariants.
 *
 *   node scripts/validate-postman.js
 *
 * Checks:
 *   - postman/collection.json and postman/environment.json exist and parse.
 *   - collection declares the Postman v2.1 schema.
 *   - environment has every required variable; baseUrl is variable-driven.
 *   - every public OpenAPI operation appears exactly once in Atomic Endpoint Tests.
 *   - no unknown/extra atomic operation; no duplicate atomic operation.
 *   - flow + negative requests reference valid OpenAPI operations.
 *   - every request URL is {{baseUrl}}-driven (no hard-coded http://… base URL).
 *   - no docs/private reference; no secret/token/password-looking values.
 *   - generated output is deterministic (regenerate in memory, compare to disk).
 *
 * On stale output it prints: run `npm run generate:postman`.
 */

import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import {
  ATOMIC_FOLDER_NAME,
  FLOW_FOLDER_NAME,
  NEGATIVE_FOLDER_NAME,
  buildCollection,
  buildEnvironment,
  collectRequests,
  findFolder,
  listOperations,
  loadOpenApi,
  operationKey,
  rawUrlOf,
  REQUIRED_ENV_KEYS,
  repoRoot,
  serialize,
  urlToPath,
} from "./library/postman.js";

const collectionPath = resolve(repoRoot, "postman", "collection.json");
const environmentPath = resolve(repoRoot, "postman", "environment.json");

let passed = 0;
let failed = 0;

function pass(msg) {
  console.log(`  ✓ ${msg}`);
  passed++;
}

function fail(msg) {
  console.error(`  ✗ ${msg}`);
  failed++;
}

function check(condition, okMsg, failMsg) {
  if (condition) pass(okMsg);
  else fail(failMsg || okMsg);
}

function readJson(path, label) {
  if (!existsSync(path)) {
    fail(`${label} not found at ${path}`);
    return null;
  }
  try {
    const parsed = JSON.parse(readFileSync(path, "utf8"));
    pass(`${label} exists and parses`);
    return parsed;
  } catch (error) {
    fail(`${label} is not valid JSON: ${error.message}`);
    return null;
  }
}

function keyFromRequest(item) {
  const method = item.request && item.request.method ? String(item.request.method) : "";
  return operationKey(method, urlToPath(rawUrlOf(item)));
}

// ── Safety / privacy scans ────────────────────────────────────────────────────

const SECRET_PATTERNS = [
  /bearer\s+[A-Za-z0-9._-]+/i,
  /authorization\s*[:=]/i,
  /\b(password|passwd|secret|api[_-]?key|apikey|access[_-]?token|client[_-]?secret)\b/i,
];

function scanForbiddenStrings(label, text) {
  check(!text.includes("docs/private"), `${label}: no docs/private reference`,
    `${label}: contains a docs/private reference`);
  let secretHit = null;
  for (const pattern of SECRET_PATTERNS) {
    if (pattern.test(text)) { secretHit = pattern; break; }
  }
  check(secretHit === null, `${label}: no secret/token/password-looking values`,
    `${label}: matched a secret-looking pattern ${secretHit}`);
}

// ── Main ──────────────────────────────────────────────────────────────────────

function main() {
  console.log("[validate:postman] Validating Postman collection + environment against OpenAPI\n");

  const collection = readJson(collectionPath, "postman/collection.json");
  const environment = readJson(environmentPath, "postman/environment.json");
  if (!collection || !environment) {
    finish();
    return;
  }

  // Schema version.
  const schema = collection.info && collection.info.schema ? String(collection.info.schema) : "";
  check(schema.includes("/v2.1.0/"), "collection declares Postman v2.1 schema",
    `collection schema is not v2.1: "${schema}"`);

  // Environment variables.
  const envKeys = new Set((environment.values || []).map((entry) => entry && entry.key));
  for (const key of REQUIRED_ENV_KEYS) {
    check(envKeys.has(key), `environment has "${key}"`, `environment is missing "${key}"`);
  }
  const baseUrl = (environment.values || []).find((entry) => entry && entry.key === "baseUrl");
  const baseUrlValue = baseUrl && typeof baseUrl.value === "string" ? baseUrl.value : "";
  check(baseUrlValue.includes("{{host}}") && baseUrlValue.includes("{{port}}"),
    "baseUrl is variable-driven ({{host}}/{{port}})",
    `baseUrl is not variable-driven: "${baseUrlValue}"`);

  // OpenAPI operation set.
  const openapi = loadOpenApi();
  const { operations } = listOperations(openapi);
  const contractKeys = new Set(operations.map((op) => operationKey(op.method, op.path)));

  // Atomic folder coverage.
  const atomic = findFolder(collection, ATOMIC_FOLDER_NAME);
  check(atomic !== null, `"${ATOMIC_FOLDER_NAME}" folder present`,
    `"${ATOMIC_FOLDER_NAME}" folder missing`);
  if (atomic) {
    const atomicRequests = collectRequests(atomic.item);
    const counts = new Map();
    for (const item of atomicRequests) {
      const key = keyFromRequest(item);
      counts.set(key, (counts.get(key) || 0) + 1);
    }
    for (const key of contractKeys) {
      const count = counts.get(key) || 0;
      if (count === 1) pass(`atomic covers ${key}`);
      else if (count === 0) fail(`atomic is missing ${key}`);
      else fail(`atomic has ${count} requests for ${key} (expected exactly 1)`);
    }
    for (const key of counts.keys()) {
      check(contractKeys.has(key), `atomic ${key} maps to a contract operation`,
        `atomic has an unknown operation not in the contract: ${key}`);
    }
    check(atomicRequests.length === contractKeys.size,
      `atomic request count (${atomicRequests.length}) equals contract operation count (${contractKeys.size})`,
      `atomic request count ${atomicRequests.length} != contract operation count ${contractKeys.size}`);
  }

  // Flow + negative requests reference valid operations.
  for (const folderName of [FLOW_FOLDER_NAME, NEGATIVE_FOLDER_NAME]) {
    const folder = findFolder(collection, folderName);
    check(folder !== null, `"${folderName}" folder present`, `"${folderName}" folder missing`);
    if (!folder) continue;
    const requests = collectRequests(folder.item);
    check(requests.length > 0, `"${folderName}" has requests`, `"${folderName}" has no requests`);
    for (const item of requests) {
      const key = keyFromRequest(item);
      check(contractKeys.has(key),
        `${folderName}: "${item.name}" maps to ${key}`,
        `${folderName}: "${item.name}" references a non-contract operation: ${key}`);
    }
  }

  // Every request URL is variable-driven.
  for (const item of collectRequests(collection.item)) {
    const raw = rawUrlOf(item);
    check(raw.startsWith("{{baseUrl}}"),
      `"${item.name}" URL is {{baseUrl}}-driven`,
      `"${item.name}" URL hard-codes a base URL: "${raw}"`);
  }

  // Safety / privacy scans over both serialized artifacts.
  scanForbiddenStrings("collection", JSON.stringify(collection));
  scanForbiddenStrings("environment", JSON.stringify(environment));

  // Determinism: regenerate in memory and compare to the checked-in files.
  const expectedCollection = serialize(buildCollection(openapi));
  const expectedEnvironment = serialize(buildEnvironment());
  check(readFileSync(collectionPath, "utf8") === expectedCollection,
    "postman/collection.json is up to date with the generator",
    "postman/collection.json is STALE — run `npm run generate:postman`");
  check(readFileSync(environmentPath, "utf8") === expectedEnvironment,
    "postman/environment.json is up to date with the generator",
    "postman/environment.json is STALE — run `npm run generate:postman`");

  finish();
}

function finish() {
  console.log(`\n[validate:postman] ${passed} passed, ${failed} failed.\n`);
  if (failed > 0) {
    console.error("[validate:postman] FAILED.\n");
    process.exit(1);
  }
  console.log("[validate:postman] All Postman checks passed.\n");
}

main();
