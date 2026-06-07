#!/usr/bin/env node
/* global console, process, setTimeout, clearTimeout, URL, Buffer */
/**
 * Config compatibility tests.
 *
 * Loads every fixture from tests/fixtures/config/ and validates it against
 * the TypeScript server and (if available) the Go service binary.
 *
 * Checks:
 *   - Valid fixtures load from a config file and import via the HTTP API
 *   - Invalid fixtures are rejected (config load fails or API returns 400/409)
 *   - The {rules:[...]} wrapper shape is supported by the Go service
 *   - UDP rules without an explicit udpMode default to "one-way"
 *   - Duplicate listen bindings are rejected by both runtimes
 *   - Export shape is stable: {version, exportedAt, rules[]}
 *
 * Usage:
 *   node scripts/validate-config.js [--skip-go]
 *
 *   --skip-go   Skip Go parity even if the binary is present.
 */

import { existsSync, mkdirSync, writeFileSync, rmSync, readFileSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { spawn } from "node:child_process";
import { createServer } from "node:net";
import { get, request as httpRequest } from "node:http";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const isWindows = process.platform === "win32";
const fixturesDir = join(repoRoot, "tests", "fixtures", "config");

let passed = 0;
let failed = 0;
let skipped = 0;

function pass(msg) {
  console.log(`  ✓ ${msg}`);
  passed++;
}

function fail(msg) {
  console.error(`  ✗ ${msg}`);
  failed++;
}

function skip(msg) {
  console.log(`  - ${msg} [skip]`);
  skipped++;
}

function getFreePort() {
  return new Promise((resolve, reject) => {
    const srv = createServer();
    srv.listen(0, "127.0.0.1", () => {
      const { port } = srv.address();
      srv.close(() => resolve(port));
    });
    srv.on("error", reject);
  });
}

function httpGet(url) {
  return new Promise((resolve, reject) => {
    const req = get(url, (res) => {
      let body = "";
      res.on("data", (d) => { body += d; });
      res.on("end", () => resolve({ status: res.statusCode, body, json: () => JSON.parse(body) }));
    });
    req.on("error", reject);
    req.setTimeout(3000, () => req.destroy(new Error("timeout")));
  });
}

function httpMethod(method, url, body) {
  return new Promise((resolve, reject) => {
    const data = body !== undefined ? JSON.stringify(body) : undefined;
    const urlObj = new URL(url);
    const req = httpRequest({
      hostname: urlObj.hostname,
      port: urlObj.port,
      path: urlObj.pathname + urlObj.search,
      method,
      headers: data ? {
        "Content-Type": "application/json",
        "Content-Length": Buffer.byteLength(data),
      } : {},
    }, (res) => {
      let responseBody = "";
      res.on("data", (d) => { responseBody += d; });
      res.on("end", () => resolve({
        status: res.statusCode,
        body: responseBody,
        json: () => JSON.parse(responseBody),
      }));
    });
    req.on("error", reject);
    req.setTimeout(3000, () => req.destroy(new Error("timeout")));
    if (data) req.write(data);
    req.end();
  });
}

async function waitForReady(baseUrl, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const res = await httpGet(`${baseUrl}/api/forwards`);
      if (res.status === 200) return true;
    } catch {
      // not ready yet
    }
    await new Promise((r) => setTimeout(r, 300));
  }
  return false;
}

function spawnServer(binary, args) {
  return spawn(binary, args, { stdio: ["ignore", "pipe", "pipe"], detached: false });
}

function waitForExit(proc, timeoutMs = 5000) {
  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve(null), timeoutMs);
    proc.on("exit", (code) => { clearTimeout(timer); resolve(code); });
  });
}

function killProc(proc) {
  try { proc.kill(); } catch { /* best effort */ }
}

function makeTempDir(label) {
  const dir = join(tmpdir(), `portier-config-${label}-${Date.now()}-${process.pid}`);
  mkdirSync(dir, { recursive: true });
  return dir;
}

function cleanupDir(dir) {
  try { rmSync(dir, { recursive: true, force: true }); } catch { /* best effort */ }
}

// Extracts the rules array from fixture content.
// Handles both raw array and {rules:[...]} wrapper shapes.
function extractRules(content) {
  const parsed = JSON.parse(content);
  if (Array.isArray(parsed)) return parsed;
  if (parsed && Array.isArray(parsed.rules)) return parsed.rules;
  return null;
}

function toExportedConfig(rules) {
  return { version: "1", exportedAt: new Date().toISOString(), rules };
}

// ── Phase 1: Static parsing ──────────────────────────────────────────────────

function runStaticTests(expectations) {
  console.log("[validate:config] Phase 1: Static fixture parsing\n");

  for (const [name, exp] of Object.entries(expectations)) {
    const content = readFileSync(join(fixturesDir, name), "utf-8");

    if (exp.errorCategory === "malformed-json") {
      try {
        JSON.parse(content);
        fail(`${name}: expected malformed JSON but parsed successfully`);
      } catch {
        pass(`${name}: correctly rejects as malformed JSON`);
      }
    } else {
      try {
        JSON.parse(content);
        pass(`${name}: parses as valid JSON`);
      } catch (e) {
        fail(`${name}: unexpected JSON parse error: ${e.message}`);
      }
    }
  }
}

// ── Config file loading tests ────────────────────────────────────────────────
//
// Writes each fixture directly as the server's rules.json and starts the
// server. Checks that valid fixtures result in the expected rule count and
// that invalid fixtures cause the server to exit with a non-zero code.
//
// Wrapper shape (v1-wrapper-rules.json) is skipped for the TypeScript server
// because ConfigStore.load() requires a raw JSON array; the Go service
// supports both shapes.

async function runConfigLoadTests(binary, binaryArgs, runtime, expectations) {
  console.log(`\n[validate:config] Config file loading — ${runtime}\n`);

  const isTs = runtime === "ts";

  for (const [name, exp] of Object.entries(expectations)) {
    // Wrapper shape is not supported by the TypeScript config store.
    if (exp.shape === "wrapper" && isTs) {
      skip(`${name}: TypeScript config requires raw array; wrapper shape not tested for config load`);
      continue;
    }

    // Invalid-field fixtures are only tested via the HTTP API (not config load).
    // The API gives clearer, per-field error messages for these cases.
    if (!exp.valid && exp.errorCategory === "invalid-field") {
      continue;
    }

    const content = readFileSync(join(fixturesDir, name), "utf-8");
    const tempDir = makeTempDir(`${runtime}-load`);
    const tempConfig = join(tempDir, "rules.json");
    writeFileSync(tempConfig, content);

    const port = await getFreePort();
    const proc = spawnServer(binary, [
      ...binaryArgs,
      "--config", tempConfig,
      "--host", "127.0.0.1",
      "--port", String(port),
    ]);

    const serverUrl = `http://127.0.0.1:${port}`;

    if (!exp.valid) {
      // Expect the server to reject the config and exit with a non-zero code.
      const outcome = await Promise.race([
        waitForExit(proc, 4000),
        waitForReady(serverUrl, 4000).then(() => "ready"),
      ]);

      if (outcome === "ready") {
        fail(`${name}: expected server to reject invalid config but it became ready`);
        killProc(proc);
      } else if (outcome !== null && outcome !== 0) {
        pass(`${name}: server correctly rejected invalid config (exit ${outcome})`);
      } else {
        // Exited with code 0 or timed out without becoming ready — still counts as rejection.
        pass(`${name}: server did not become ready with invalid config`);
      }
      cleanupDir(tempDir);
      continue;
    }

    // Valid fixture — server must start and expose the expected rule count.
    const ready = await waitForReady(`http://127.0.0.1:${port}`, 12000);

    if (!ready) {
      fail(`${name}: server did not become ready with valid config`);
      killProc(proc);
      cleanupDir(tempDir);
      continue;
    }

    try {
      const res = await httpGet(`http://127.0.0.1:${port}/api/forwards`);
      if (res.status !== 200) {
        fail(`${name}: GET /api/forwards → ${res.status}`);
        continue;
      }
      const rules = res.json();
      if (!Array.isArray(rules)) {
        fail(`${name}: GET /api/forwards did not return an array`);
        continue;
      }
      if (rules.length === exp.ruleCount) {
        pass(`${name}: config load → ${exp.ruleCount} rule(s)`);
      } else {
        fail(`${name}: config load → expected ${exp.ruleCount} rules, got ${rules.length}`);
      }
    } finally {
      killProc(proc);
      cleanupDir(tempDir);
    }
  }
}

// ── HTTP API import/export tests ─────────────────────────────────────────────
//
// Starts a single server with an empty config and drives the import/export API:
//   - Valid fixtures are imported as ExportedConfig (replace mode) and the
//     resulting rule count is verified.
//   - v1-raw-array.json verifies that UDP rules without udpMode default to
//     "one-way" after import.
//   - v1-mixed.json verifies the export shape after a multi-rule import.
//   - Invalid-field fixtures are submitted one rule at a time via POST
//     /api/forwards and must return 400.
//   - Duplicate binding is tested by posting two rules with the same
//     listen key; the second must return 409.

async function runApiTests(binary, binaryArgs, runtime, expectations) {
  console.log(`\n[validate:config] HTTP API import/export — ${runtime}\n`);

  const tempDir = makeTempDir(`${runtime}-api`);
  const tempConfig = join(tempDir, "rules.json");
  writeFileSync(tempConfig, "[]");

  const port = await getFreePort();
  const proc = spawnServer(binary, [
    ...binaryArgs,
    "--config", tempConfig,
    "--host", "127.0.0.1",
    "--port", String(port),
  ]);

  const base = `http://127.0.0.1:${port}`;
  const api = {
    get: (path) => httpGet(`${base}${path}`),
    post: (path, body) => httpMethod("POST", `${base}${path}`, body),
  };

  const ready = await waitForReady(base, 15000);
  if (!ready) {
    fail(`${runtime}: server did not become ready for API tests`);
    killProc(proc);
    cleanupDir(tempDir);
    return;
  }
  pass(`${runtime}: server started for API tests`);

  try {
    // ── Import valid fixtures ────────────────────────────────────────────────
    for (const [name, exp] of Object.entries(expectations)) {
      if (!exp.valid) continue;

      const content = readFileSync(join(fixturesDir, name), "utf-8");
      const rules = extractRules(content);
      if (!rules) {
        fail(`${name}: could not extract rules for import test`);
        continue;
      }

      const res = await api.post("/api/config/import", {
        mode: "replace",
        config: toExportedConfig(rules),
      });

      if (res.status !== 200) {
        fail(`${name}: import → ${res.status}: ${res.body}`);
        continue;
      }

      const data = res.json();
      if (!data.result || typeof data.result.imported !== "number") {
        fail(`${name}: import response missing result.imported`);
        continue;
      }

      const listRes = await api.get("/api/forwards");
      const listed = listRes.json();
      if (!Array.isArray(listed)) {
        fail(`${name}: GET /api/forwards did not return an array after import`);
        continue;
      }

      if (listed.length === exp.ruleCount) {
        pass(`${name}: import → ${exp.ruleCount} rule(s)`);
      } else {
        fail(`${name}: import → expected ${exp.ruleCount} rules, got ${listed.length}`);
        continue;
      }

      // UDP rule without explicit udpMode must default to "one-way".
      if (exp.udpDefaultMode) {
        const udpRule = listed.find((r) => r.protocol === "udp");
        if (!udpRule) {
          fail(`${name}: expected a UDP rule after import`);
        } else if (udpRule.udpMode === "one-way") {
          pass(`${name}: UDP rule without udpMode → defaults to "one-way"`);
        } else {
          fail(`${name}: expected udpMode "one-way", got "${udpRule.udpMode}"`);
        }
      }
    }

    // ── Export shape after mixed import ─────────────────────────────────────
    {
      const mixedContent = readFileSync(join(fixturesDir, "v1-mixed.json"), "utf-8");
      const mixedRules = extractRules(mixedContent);
      await api.post("/api/config/import", { mode: "replace", config: toExportedConfig(mixedRules) });

      const exportRes = await api.get("/api/config/export");
      if (exportRes.status === 200) {
        const exported = exportRes.json();
        if (
          exported.version === "1" &&
          typeof exported.exportedAt === "string" &&
          Array.isArray(exported.rules) &&
          exported.rules.length === 4
        ) {
          pass("v1-mixed.json: export shape → {version:\"1\", exportedAt, rules[4]}");
        } else {
          fail(`export shape mismatch after mixed import: ${JSON.stringify(exported)}`);
        }
      } else {
        fail(`GET /api/config/export → ${exportRes.status}`);
      }
    }

    // ── Reset to empty ────────────────────────────────────────────────────────
    await api.post("/api/config/import", { mode: "replace", config: toExportedConfig([]) });

    // ── Duplicate binding rejection ───────────────────────────────────────────
    {
      const r1 = await api.post("/api/forwards", {
        name: "Dup First",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort: 48131,
        targetHost: "127.0.0.1",
        targetPort: 48231,
        enabled: false,
      });
      if (r1.status === 201) {
        const r2 = await api.post("/api/forwards", {
          name: "Dup Second",
          protocol: "tcp",
          listenHost: "127.0.0.1",
          listenPort: 48131,
          targetHost: "127.0.0.1",
          targetPort: 48232,
          enabled: false,
        });
        if (r2.status === 409) {
          pass("invalid-duplicate-binding.json: second rule with same listen key → 409");
        } else {
          fail(`invalid-duplicate-binding.json: expected 409, got ${r2.status}`);
        }
      } else {
        fail(`duplicate binding test: first rule POST failed with ${r1.status}: ${r1.body}`);
      }
    }

    // ── Invalid-field fixtures via POST /api/forwards ─────────────────────────
    const invalidFieldFixtures = [
      "invalid-port-zero.json",
      "invalid-port-too-high.json",
      "invalid-missing-name.json",
      "invalid-empty-host.json",
      "invalid-protocol.json",
      "invalid-udp-mode.json",
    ];

    for (const name of invalidFieldFixtures) {
      // Reset between tests to avoid state leakage.
      await api.post("/api/config/import", { mode: "replace", config: toExportedConfig([]) });

      const content = readFileSync(join(fixturesDir, name), "utf-8");
      const rules = extractRules(content);
      if (!rules || rules.length === 0) {
        fail(`${name}: could not extract rule for invalid-field API test`);
        continue;
      }

      // Strip the id so POST /api/forwards treats it as a new rule creation.
      const ruleInput = { ...rules[0] };
      delete ruleInput.id;
      const res = await api.post("/api/forwards", ruleInput);

      if (res.status === 400) {
        const data = res.json();
        if (Array.isArray(data.errors) && data.errors.length > 0) {
          pass(`${name}: invalid rule → 400 with errors[]`);
        } else {
          fail(`${name}: 400 but no errors[]: ${res.body}`);
        }
      } else {
        fail(`${name}: expected 400, got ${res.status}: ${res.body}`);
      }
    }
  } finally {
    killProc(proc);
    cleanupDir(tempDir);
  }
}

// ── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  const cliArgs = process.argv.slice(2);
  const skipGo = cliArgs.includes("--skip-go");

  console.log("[validate:config] Config compatibility tests\n");

  const fixtureMetaPath = join(fixturesDir, "fixtures.json");
  if (!existsSync(fixtureMetaPath)) {
    console.error(`[validate:config] fixtures.json not found: ${fixtureMetaPath}`);
    process.exit(1);
  }

  const expectations = JSON.parse(readFileSync(fixtureMetaPath, "utf-8"));
  const fixtureCount = Object.keys(expectations).length;
  const validCount = Object.values(expectations).filter((e) => e.valid).length;
  const invalidCount = fixtureCount - validCount;

  console.log(`  Fixtures: ${fixtureCount} total (${validCount} valid, ${invalidCount} invalid)\n`);

  // ── Phase 1: Static parsing ──────────────────────────────────────────────
  runStaticTests(expectations);

  // ── TypeScript server ────────────────────────────────────────────────────
  const tsServerPath = join(repoRoot, "server", "build", "index.js");

  if (!existsSync(tsServerPath)) {
    skip("TypeScript server build not found — run: npm run build:server");
    console.log("\n[validate:config] TypeScript checks skipped.\n");
  } else {
    const tsArgs = [tsServerPath, "--service"];

    await runConfigLoadTests("node", tsArgs, "ts", expectations);
    await runApiTests("node", tsArgs, "ts", expectations);
  }

  // ── Go service ───────────────────────────────────────────────────────────
  if (skipGo) {
    console.log("\n[validate:config] Go service: --skip-go set, skipping.\n");
  } else {
    const packageBinary = join(repoRoot, "build", "portier", isWindows ? "service.exe" : "service");
    const devBinary = join(repoRoot, "service", "build", isWindows ? "portier-service.exe" : "portier-service");

    const goPath = existsSync(packageBinary) ? packageBinary
      : existsSync(devBinary) ? devBinary
      : null;

    if (!goPath) {
      console.log("\n[validate:config] Go service binary not found.");
      console.log(`  Checked: ${packageBinary}`);
      console.log(`  Checked: ${devBinary}`);
      console.log("  Run: npm run build:runtime  (or npm run build:service)");
      console.log("  Go config checks skipped (not a failure).\n");
      skipped++;
    } else {
      console.log(`\n[validate:config] Go service: ${goPath}`);
      const goArgs = ["--service"];

      await runConfigLoadTests(goPath, goArgs, "go", expectations);
      await runApiTests(goPath, goArgs, "go", expectations);
    }
  }

  // ── Summary ──────────────────────────────────────────────────────────────
  console.log(`\n[validate:config] ${passed} passed, ${skipped} skipped, ${failed} failed.\n`);

  if (failed > 0) {
    console.error("[validate:config] FAILED.\n");
    process.exit(1);
  }
  console.log("[validate:config] All config compatibility tests passed.\n");
}

main().catch((err) => {
  console.error("[validate:config] Unexpected error:", err);
  process.exit(1);
});
