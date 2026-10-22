/* global console, fetch, structuredClone */
/**
 * Minimal in-repo runner for the generated Postman collection.
 *
 * Replaces Newman for `validate:postman:local`. Newman's dependency tree carried
 * advisories with no upstream fix (node-forge, @faker-js/faker, csv-parse), and the
 * collection only uses a small, generator-controlled slice of the Postman sandbox. This
 * module implements exactly that slice:
 *
 *   - `{{variable}}` resolution from the environment (recursive, e.g. baseUrl → host/port)
 *   - raw request bodies and per-request headers
 *   - `test` event scripts, run in a separate `node:vm` context with a `pm` object:
 *       pm.test, pm.expect(x).to[.not].include / .be.a / .be.an / .eql,
 *       pm.response.code / .text() / .json() / .to.be.json / .to.have.status(n),
 *       pm.environment.get / .set / .unset
 *
 * Anything outside that slice (another assertion, a pre-request script, a non-raw body,
 * an unresolved variable) fails loudly rather than passing silently. If the generator in
 * scripts/library/postman.js starts emitting something new, extend this runner with it.
 */

import { isDeepStrictEqual } from "node:util";
import { runInNewContext } from "node:vm";

const SCRIPT_TIMEOUT_MS = 5000;

class AssertionError extends Error {}

// Errors thrown inside the vm context are not instances of this realm's Error.
function errorMessage(error) {
  return error && typeof error.message === "string" ? error.message : String(error);
}

// Throw on any property the runner does not implement, so an unsupported getter-style
// assertion (e.g. `.to.be.ok`) cannot pass by evaluating to undefined.
function strict(target, label) {
  return new Proxy(target, {
    get(obj, prop) {
      if (typeof prop === "symbol" || prop in obj) return obj[prop];
      throw new Error(`Unsupported Postman API: ${label}.${String(prop)} (extend scripts/library/postman-runner.js)`);
    },
  });
}

function format(value) {
  return JSON.stringify(value) ?? String(value);
}

function typeOf(value) {
  if (value === null) return "null";
  if (Array.isArray(value)) return "array";
  return typeof value;
}

function expect(actual) {
  const chain = (negate) => {
    const check = (pass, message) => {
      if (pass === negate) throw new AssertionError(`expected ${format(actual)} to ${negate ? "not " : ""}${message}`);
    };
    const typeCheck = (type) => check(typeOf(actual) === type, `be a ${type}`);
    const be = strict({ a: typeCheck, an: typeCheck }, "pm.expect().to.be");
    return {
      be,
      include(expected) {
        const pass = typeof actual === "string" || Array.isArray(actual) ? actual.includes(expected) : false;
        check(pass, `include ${format(expected)}`);
      },
      eql(expected) {
        // Values built inside the vm context have that realm's prototypes; clone both sides
        // into this realm so isDeepStrictEqual compares structure, not realm identity.
        check(isDeepStrictEqual(structuredClone(actual), structuredClone(expected)), `deeply equal ${format(expected)}`);
      },
    };
  };
  const to = chain(false);
  to.not = strict(chain(true), "pm.expect().to.not");
  return strict({ to: strict(to, "pm.expect().to") }, "pm.expect()");
}

function createResponse(status, headers, body) {
  const json = () => JSON.parse(body);
  const to = {
    be: strict({
      get json() {
        const contentType = headers.get("content-type") ?? "";
        if (!contentType.includes("json")) throw new AssertionError(`expected response to be JSON but Content-Type is ${format(contentType)}`);
        try {
          json();
        } catch {
          throw new AssertionError("expected response body to be valid JSON");
        }
        return true;
      },
    }, "pm.response.to.be"),
    have: strict({
      status(expected) {
        if (status !== expected) throw new AssertionError(`expected response to have status ${expected} but got ${status}`);
      },
    }, "pm.response.to.have"),
  };
  return strict({ code: status, text: () => body, json, to: strict(to, "pm.response.to") }, "pm.response");
}

function resolveVariables(text, env, context) {
  let out = text;
  // Resolve nested references (baseUrl is itself `http://{{host}}:{{port}}`).
  for (let depth = 0; depth < 10 && out.includes("{{"); depth++) {
    out = out.replace(/\{\{([^{}]+)\}\}/g, (match, name) => (env.has(name) ? env.get(name) : match));
  }
  const unresolved = out.match(/\{\{[^{}]+\}\}/);
  if (unresolved) throw new Error(`Unresolved variable ${unresolved[0]} in ${context}`);
  return out;
}

function assertSupportedEvents(node, label) {
  for (const event of node.event ?? []) {
    if (event.listen !== "test") throw new Error(`Unsupported ${event.listen} script on ${label}`);
  }
}

function collectRequests(items, trail, out) {
  for (const item of items) {
    const label = [...trail, item.name].join(" / ");
    if (item.item) {
      assertSupportedEvents(item, `folder "${label}"`);
      collectRequests(item.item, [...trail, item.name], out);
    } else {
      out.push({ label, item });
    }
  }
  return out;
}

async function sendRequest(request, env, label) {
  if (typeof request.url !== "string") throw new Error(`Unsupported structured url on "${label}"`);
  const url = resolveVariables(request.url, env, `url of "${label}"`);
  const headers = {};
  for (const header of request.header ?? []) {
    if (header.disabled) continue;
    headers[header.key] = resolveVariables(header.value, env, `header ${header.key} of "${label}"`);
  }
  let body;
  if (request.body) {
    if (request.body.mode !== "raw") throw new Error(`Unsupported body mode ${request.body.mode} on "${label}"`);
    body = resolveVariables(request.body.raw, env, `body of "${label}"`);
  }
  const res = await fetch(url, { method: request.method, headers, body });
  return { url, status: res.status, headers: res.headers, text: await res.text() };
}

function runTestScripts(item, response, env, label) {
  const results = [];
  const pm = strict({
    test(name, fn) {
      try {
        fn();
        results.push({ name, ok: true });
      } catch (error) {
        results.push({ name, ok: false, message: errorMessage(error) });
      }
    },
    expect,
    response,
    environment: strict({
      get: (key) => env.get(key),
      set: (key, value) => { env.set(key, String(value)); },
      unset: (key) => { env.delete(key); },
    }, "pm.environment"),
  }, "pm");
  for (const event of item.event ?? []) {
    runInNewContext(event.script.exec.join("\n"), { pm }, { filename: label, timeout: SCRIPT_TIMEOUT_MS });
  }
  return results;
}

/**
 * Run one top-level folder of `collection` against the URLs its variables resolve to.
 *
 * @param {object} options
 * @param {object} options.collection   parsed postman/collection.json
 * @param {object} options.environment  parsed postman/environment.json
 * @param {string} options.folder       top-level folder name
 * @param {Record<string, string>} [options.overrides]  environment values to override
 * @param {boolean} [options.verbose]   print response bodies
 * @returns {Promise<{ requests: number, failedRequests: number, assertions: number, failedAssertions: number }>}
 */
export async function runFolder({ collection, environment, folder, overrides = {}, verbose = false }) {
  assertSupportedEvents(collection, "the collection");
  const root = collection.item.find((item) => item.name === folder && item.item);
  if (!root) throw new Error(`Folder not found in collection: ${JSON.stringify(folder)}`);
  assertSupportedEvents(root, `folder "${folder}"`);

  const env = new Map();
  for (const entry of environment.values) {
    if (entry.enabled !== false) env.set(entry.key, String(entry.value));
  }
  for (const [key, value] of Object.entries(overrides)) env.set(key, String(value));

  const summary = { requests: 0, failedRequests: 0, assertions: 0, failedAssertions: 0 };
  for (const { label, item } of collectRequests(root.item, [], [])) {
    summary.requests++;
    console.log(`\n→ ${label}`);
    let response;
    try {
      response = await sendRequest(item.request, env, label);
    } catch (error) {
      summary.failedRequests++;
      console.log(`  ✗ request failed: ${errorMessage(error)}`);
      continue;
    }
    console.log(`  ${item.request.method} ${response.url} [${response.status}]`);
    if (verbose && response.text) console.log(`  ${response.text}`);

    let results;
    try {
      results = runTestScripts(item, createResponse(response.status, response.headers, response.text), env, label);
    } catch (error) {
      summary.failedRequests++;
      console.log(`  ✗ test script error: ${errorMessage(error)}`);
      continue;
    }
    for (const result of results) {
      summary.assertions++;
      if (result.ok) {
        console.log(`  ✓ ${result.name}`);
      } else {
        summary.failedAssertions++;
        console.log(`  ✗ ${result.name}: ${result.message}`);
      }
    }
  }

  console.log(
    `\n  requests: ${summary.requests} executed, ${summary.failedRequests} failed` +
      ` | assertions: ${summary.assertions} executed, ${summary.failedAssertions} failed`
  );
  return summary;
}
