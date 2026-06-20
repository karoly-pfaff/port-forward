/* global process */
/**
 * Shared builders for the Portier Postman collection + environment.
 *
 * The canonical API contract is docs/openapi.json (the single source of truth, validated
 * by `npm run validate:openapi:go`). Both scripts/generate-postman.js and
 * scripts/validate-postman.js import this module so the collection is *derived from* the
 * contract and cannot silently diverge: the generator writes the build output, the
 * validator regenerates it in memory and fails if the checked-in files are stale.
 *
 * Atomic requests are intentionally non-destructive: id/group path params point at
 * {{invalidRuleId}} / {{group}} (404, no mutation), config apply is dryRun, import is an
 * empty merge, and reorder is an empty no-op. The one unavoidable side effect is the
 * atomic "Create a forward rule" call, which creates a single stopped (enabled:false)
 * demo rule. The Happy Path flow creates and then deletes its own rule.
 */

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const libDir = dirname(fileURLToPath(import.meta.url));
export const repoRoot = resolve(libDir, "..", "..");
export const openApiPath = resolve(repoRoot, "docs", "openapi.json");

export const COLLECTION_SCHEMA =
  "https://schema.getpostman.com/json/collection/v2.1.0/collection.json";

export const ATOMIC_FOLDER_NAME = "Atomic Endpoint Tests";
export const FLOW_FOLDER_NAME = "Happy Path Rule Flow";
export const NEGATIVE_FOLDER_NAME = "Negative/Error Tests";

const HTTP_METHODS = ["get", "post", "put", "patch", "delete", "options", "head"];

// ── Environment ───────────────────────────────────────────────────────────────
// Ordered so the generated environment.json is deterministic. createdRuleId is filled
// in by the Happy Path flow at run time. No secrets, tokens, or machine-specific paths.
export const ENV_DEFAULTS = [
  ["host", "127.0.0.1"],
  ["port", "47831"],
  ["baseUrl", "http://{{host}}:{{port}}"],
  ["apiPrefix", "/api"],
  ["ruleName", "portier-postman-demo"],
  ["listenHost", "127.0.0.1"],
  ["listenPort", "48010"],
  ["targetHost", "127.0.0.1"],
  ["targetPort", "48011"],
  ["protocol", "tcp"],
  ["udpMode", "one-way"],
  ["group", "postman-demo-group"],
  ["advisoryPurpose", "forward"],
  ["advisoryPort", "8080"],
  ["createdRuleId", ""],
  ["invalidRuleId", "nonexistent-rule-00000000"],
];

export const REQUIRED_ENV_KEYS = ENV_DEFAULTS.map(([key]) => key);

export function buildEnvironment() {
  return {
    name: "Portier Local",
    values: ENV_DEFAULTS.map(([key, value]) => ({
      key,
      value,
      type: "default",
      enabled: true,
    })),
    _postman_variable_scope: "environment",
  };
}

// ── OpenAPI loading + operation listing ───────────────────────────────────────

export function loadOpenApi() {
  return JSON.parse(readFileSync(openApiPath, "utf8"));
}

// Stable key for an operation: "<METHOD> <path>" (path with {param} placeholders).
export function operationKey(method, path) {
  return `${method.toUpperCase()} ${path}`;
}

export function listOperations(openapi) {
  const paths = openapi && openapi.paths ? openapi.paths : {};
  const tagOrder = Array.isArray(openapi.tags)
    ? openapi.tags.map((tag) => tag && tag.name).filter((name) => typeof name === "string")
    : [];
  const operations = [];
  for (const path of Object.keys(paths)) {
    const item = paths[path] || {};
    for (const method of Object.keys(item)) {
      if (!HTTP_METHODS.includes(method)) continue;
      const op = item[method] || {};
      const tags = Array.isArray(op.tags) ? op.tags : [];
      operations.push({
        method: method.toUpperCase(),
        path,
        operationId: typeof op.operationId === "string" ? op.operationId : "",
        summary: typeof op.summary === "string" ? op.summary : `${method.toUpperCase()} ${path}`,
        description: typeof op.description === "string" ? op.description : "",
        tag: typeof tags[0] === "string" ? tags[0] : "other",
        statuses: Object.keys(op.responses || {}).map((code) => Number(code)),
        hasRequestBody: Boolean(op.requestBody),
      });
    }
  }
  operations.sort((a, b) => {
    const ra = tagRank(a.tag, tagOrder);
    const rb = tagRank(b.tag, tagOrder);
    if (ra !== rb) return ra - rb;
    if (a.path !== b.path) return a.path.localeCompare(b.path);
    return HTTP_METHODS.indexOf(a.method.toLowerCase()) - HTTP_METHODS.indexOf(b.method.toLowerCase());
  });
  return { operations, tagOrder };
}

function tagRank(name, tagOrder) {
  const index = tagOrder.indexOf(name);
  return index === -1 ? tagOrder.length : index;
}

// ── URL helpers ───────────────────────────────────────────────────────────────
// Every request URL is variable-driven: it starts with {{baseUrl}}, with {{apiPrefix}}
// standing in for "/api". Path params are written inline as {{invalidRuleId}} /
// {{createdRuleId}} / {{group}}. urlToPath() reverses that back to an OpenAPI path
// template so the validator can match a request to its contract operation.

const PATH_TOKEN_TO_TEMPLATE = [
  ["{{createdRuleId}}", "{id}"],
  ["{{invalidRuleId}}", "{id}"],
  ["{{group}}", "{group}"],
];

export function pathToUrl(path) {
  if (path.startsWith("/api/")) return `{{baseUrl}}{{apiPrefix}}${path.slice("/api".length)}`;
  if (path === "/api") return "{{baseUrl}}{{apiPrefix}}";
  return `{{baseUrl}}${path}`;
}

export function urlToPath(rawUrl) {
  let value = String(rawUrl).split("?")[0];
  value = value.replace("{{baseUrl}}", "").replace("{{apiPrefix}}", "/api");
  for (const [token, template] of PATH_TOKEN_TO_TEMPLATE) {
    value = value.split(token).join(template);
  }
  return value;
}

// ── Request item builders ─────────────────────────────────────────────────────

function jsonHeader() {
  return [{ key: "Content-Type", value: "application/json" }];
}

function rawBody(raw) {
  return { mode: "raw", raw, options: { raw: { language: "json" } } };
}

function testEvent(lines) {
  return [{ listen: "test", script: { type: "text/javascript", exec: lines } }];
}

function requestItem({ name, method, url, description, bodyRaw, testLines }) {
  const request = { method, header: bodyRaw ? jsonHeader() : [] };
  if (bodyRaw) request.body = rawBody(bodyRaw);
  request.url = url;
  if (description) request.description = description;
  const item = { name, request };
  if (testLines) item.event = testEvent(testLines);
  return item;
}

// Atomic id/group params resolve to values that never mutate a real rule.
function atomicUrl(path) {
  let url = pathToUrl(path);
  url = url.split("{id}").join("{{invalidRuleId}}").split("{group}").join("{{group}}");
  if (path === "/api/ports/advisory") {
    url += "?purpose={{advisoryPurpose}}&port={{advisoryPort}}";
  }
  return url;
}

// Curated, OpenAPI-shaped request bodies for the atomic catalog. Keyed by operationId.
const ATOMIC_BODIES = {
  ForwardsController_create: [
    "{",
    '  "name": "{{ruleName}}",',
    '  "protocol": "{{protocol}}",',
    '  "listenHost": "{{listenHost}}",',
    '  "listenPort": {{listenPort}},',
    '  "targetHost": "{{targetHost}}",',
    '  "targetPort": {{targetPort}},',
    '  "enabled": false',
    "}",
  ].join("\n"),
  ForwardsController_reorder: ['{', '  "ids": []', "}"].join("\n"),
  ForwardsController_update: ['{', '  "name": "{{ruleName}}"', "}"].join("\n"),
  ConfigPlanController_plan: ['{', '  "desired": {', '    "rules": []', "  }", "}"].join("\n"),
  ConfigImportController_import: [
    "{",
    '  "mode": "merge",',
    '  "config": {',
    '    "version": "1",',
    '    "exportedAt": "1970-01-01T00:00:00.000Z",',
    '    "rules": []',
    "  }",
    "}",
  ].join("\n"),
  ConfigApplyController_apply: [
    "{",
    '  "desired": {',
    '    "rules": []',
    "  },",
    '  "dryRun": true',
    "}",
  ].join("\n"),
};

function atomicTestLines(statuses) {
  const codes = JSON.stringify([...statuses].sort((a, b) => a - b));
  return [
    `const documented = ${codes};`,
    'pm.test("Status is documented by the API contract", function () {',
    "  pm.expect(documented).to.include(pm.response.code);",
    "});",
    'pm.test("Body is JSON when a body is present", function () {',
    "  if (pm.response.code !== 204 && pm.response.text()) {",
    "    pm.response.to.be.json;",
    "  }",
    "});",
  ];
}

function buildAtomicFolder(operations, tagOrder) {
  const byTag = new Map();
  for (const op of operations) {
    if (!byTag.has(op.tag)) byTag.set(op.tag, []);
    byTag.get(op.tag).push(op);
  }
  const tagNames = [...byTag.keys()].sort((a, b) => {
    const ra = tagRank(a, tagOrder);
    const rb = tagRank(b, tagOrder);
    return ra !== rb ? ra - rb : a.localeCompare(b);
  });
  const folders = tagNames.map((tag) => ({
    name: tag,
    item: byTag.get(tag).map((op) =>
      requestItem({
        name: op.summary,
        method: op.method,
        url: atomicUrl(op.path),
        description: op.description,
        bodyRaw: op.hasRequestBody ? ATOMIC_BODIES[op.operationId] || "{}" : undefined,
        testLines: atomicTestLines(op.statuses),
      })
    ),
  }));
  return {
    name: ATOMIC_FOLDER_NAME,
    description:
      "One request per public API operation, generated from docs/openapi.json. " +
      "Path params resolve to {{invalidRuleId}} / {{group}} so these calls do not mutate real rules.",
    item: folders,
  };
}

// ── Happy Path flow ───────────────────────────────────────────────────────────

function flowFolder() {
  const createBody = ATOMIC_BODIES.ForwardsController_create;
  const updateBody = ['{', '  "name": "{{ruleName}}-updated"', "}"].join("\n");
  const items = [
    requestItem({
      name: "Runtime check",
      method: "GET",
      url: pathToUrl("/api/runtime"),
      testLines: [
        'pm.test("Runtime responds 200", function () { pm.response.to.have.status(200); });',
        'pm.test("Reports a version", function () { pm.expect(pm.response.json().version).to.be.a("string"); });',
      ],
    }),
    requestItem({
      name: "Create demo rule",
      method: "POST",
      url: pathToUrl("/api/forwards"),
      bodyRaw: createBody,
      testLines: [
        'pm.test("Rule created", function () { pm.response.to.have.status(201); });',
        "const created = pm.response.json();",
        'pm.test("Created rule has an id", function () { pm.expect(created.id).to.be.a("string"); });',
        'pm.environment.set("createdRuleId", created.id);',
      ],
    }),
    requestItem({
      name: "List includes demo rule",
      method: "GET",
      url: pathToUrl("/api/forwards"),
      testLines: [
        'pm.test("List responds 200", function () { pm.response.to.have.status(200); });',
        "const ids = pm.response.json().map(function (rule) { return rule.id; });",
        'pm.test("List contains the created rule", function () {',
        '  pm.expect(ids).to.include(pm.environment.get("createdRuleId"));',
        "});",
      ],
    }),
    requestItem({
      name: "Status lists rules",
      method: "GET",
      url: pathToUrl("/api/status"),
      testLines: [
        'pm.test("Status responds 200", function () { pm.response.to.have.status(200); });',
        'pm.test("Status is an array", function () { pm.expect(pm.response.json()).to.be.an("array"); });',
      ],
    }),
    requestItem({
      name: "Rename demo rule",
      method: "PATCH",
      url: pathToUrl("/api/forwards/{id}").split("{id}").join("{{createdRuleId}}"),
      bodyRaw: updateBody,
      testLines: [
        'pm.test("Update responds 200", function () { pm.response.to.have.status(200); });',
        'pm.test("Name reflects the update", function () {',
        '  pm.expect(pm.response.json().name).to.eql(pm.environment.get("ruleName") + "-updated");',
        "});",
      ],
    }),
    requestItem({
      name: "Start demo rule",
      method: "POST",
      url: pathToUrl("/api/forwards/{id}/start").split("{id}").join("{{createdRuleId}}"),
      testLines: [
        'pm.test("Start responds 200", function () { pm.response.to.have.status(200); });',
        'pm.test("Rule is running", function () { pm.expect(pm.response.json().running).to.eql(true); });',
      ],
    }),
    requestItem({
      name: "Stop demo rule",
      method: "POST",
      url: pathToUrl("/api/forwards/{id}/stop").split("{id}").join("{{createdRuleId}}"),
      testLines: [
        'pm.test("Stop responds 200", function () { pm.response.to.have.status(200); });',
        'pm.test("Rule is stopped", function () { pm.expect(pm.response.json().running).to.eql(false); });',
      ],
    }),
    requestItem({
      name: "Export config",
      method: "GET",
      url: pathToUrl("/api/config/export"),
      testLines: [
        'pm.test("Export responds 200", function () { pm.response.to.have.status(200); });',
        'pm.test("Export is version 1", function () { pm.expect(pm.response.json().version).to.eql("1"); });',
      ],
    }),
    requestItem({
      name: "Delete demo rule",
      method: "DELETE",
      url: pathToUrl("/api/forwards/{id}").split("{id}").join("{{createdRuleId}}"),
      testLines: ['pm.test("Delete responds 204", function () { pm.response.to.have.status(204); });'],
    }),
    requestItem({
      name: "Confirm cleanup",
      method: "GET",
      url: pathToUrl("/api/forwards"),
      testLines: [
        'pm.test("List responds 200", function () { pm.response.to.have.status(200); });',
        "const ids = pm.response.json().map(function (rule) { return rule.id; });",
        'pm.test("Created rule is gone", function () {',
        '  pm.expect(ids).to.not.include(pm.environment.get("createdRuleId"));',
        "});",
        'pm.environment.unset("createdRuleId");',
      ],
    }),
  ];
  return {
    name: FLOW_FOLDER_NAME,
    description:
      "An ordered, self-cleaning flow: create a stopped demo rule, verify, rename, " +
      "start/stop it on a loopback high port, export, then delete and confirm removal.",
    item: items,
  };
}

// ── Negative / error tests ────────────────────────────────────────────────────

function errorEnvelopeLines(status) {
  return [
    `pm.test("Responds ${status}", function () { pm.response.to.have.status(${status}); });`,
    'pm.test("Error envelope has an errors array", function () {',
    '  pm.expect(pm.response.json().errors).to.be.an("array");',
    "});",
  ];
}

function negativeFolder() {
  const items = [
    requestItem({
      name: "Create rule with invalid port",
      method: "POST",
      url: pathToUrl("/api/forwards"),
      bodyRaw: [
        "{",
        '  "name": "invalid-port",',
        '  "protocol": "tcp",',
        '  "listenHost": "127.0.0.1",',
        '  "listenPort": 70000,',
        '  "targetHost": "127.0.0.1",',
        '  "targetPort": 48011,',
        '  "enabled": false',
        "}",
      ].join("\n"),
      testLines: errorEnvelopeLines(400),
    }),
    requestItem({
      name: "Start unknown rule",
      method: "POST",
      url: pathToUrl("/api/forwards/{id}/start").split("{id}").join("{{invalidRuleId}}"),
      testLines: errorEnvelopeLines(404),
    }),
    requestItem({
      name: "Delete unknown rule",
      method: "DELETE",
      url: pathToUrl("/api/forwards/{id}").split("{id}").join("{{invalidRuleId}}"),
      testLines: errorEnvelopeLines(404),
    }),
    requestItem({
      name: "Plan without desired config",
      method: "POST",
      url: pathToUrl("/api/config/plan"),
      bodyRaw: "{}",
      testLines: errorEnvelopeLines(400),
    }),
    requestItem({
      name: "Import with invalid mode",
      method: "POST",
      url: pathToUrl("/api/config/import"),
      bodyRaw: [
        "{",
        '  "mode": "bogus",',
        '  "config": {',
        '    "version": "1",',
        '    "exportedAt": "1970-01-01T00:00:00.000Z",',
        '    "rules": []',
        "  }",
        "}",
      ].join("\n"),
      testLines: errorEnvelopeLines(400),
    }),
    requestItem({
      name: "Reorder with unknown id",
      method: "POST",
      url: pathToUrl("/api/forwards/reorder"),
      bodyRaw: ['{', '  "ids": ["{{invalidRuleId}}"]', "}"].join("\n"),
      testLines: errorEnvelopeLines(404),
    }),
    requestItem({
      name: "Port advisory missing required params",
      method: "GET",
      url: pathToUrl("/api/ports/advisory"),
      testLines: errorEnvelopeLines(400),
    }),
  ];
  return {
    name: NEGATIVE_FOLDER_NAME,
    description:
      "Requests that the API is documented to reject, asserting the expected status and " +
      "the { errors: string[] } envelope.",
    item: items,
  };
}

// ── Collection ────────────────────────────────────────────────────────────────

export function buildCollection(openapi) {
  const { operations, tagOrder } = listOperations(openapi);
  const info = openapi && openapi.info ? openapi.info : {};
  const description =
    (typeof info.description === "string" ? info.description : "Portier management API.") +
    "\n\nGenerated from docs/openapi.json by `npm run generate:postman`. Edit postman/environment.json " +
    "to point host/port and test values at your runtime. Do not hand-edit postman/collection.json.";
  return {
    info: {
      name: typeof info.title === "string" ? info.title : "Portier API",
      description,
      schema: COLLECTION_SCHEMA,
    },
    item: [buildAtomicFolder(operations, tagOrder), flowFolder(), negativeFolder()],
  };
}

// ── Serialization ─────────────────────────────────────────────────────────────

export function serialize(value) {
  return `${JSON.stringify(value, null, 2)}\n`;
}

// Walk an item tree and yield every leaf request item (one with a `request` field).
export function collectRequests(items) {
  const out = [];
  const walk = (list) => {
    for (const entry of list || []) {
      if (entry && entry.request) out.push(entry);
      if (entry && Array.isArray(entry.item)) walk(entry.item);
    }
  };
  walk(items);
  return out;
}

export function rawUrlOf(item) {
  const url = item.request && item.request.url;
  if (typeof url === "string") return url;
  if (url && typeof url.raw === "string") return url.raw;
  return "";
}

export function findFolder(collection, name) {
  return (collection.item || []).find((entry) => entry && entry.name === name) || null;
}

// Fail fast if invoked directly.
if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  process.stderr.write("postman.js is a library; run generate-postman.js or validate-postman.js.\n");
  process.exit(1);
}
