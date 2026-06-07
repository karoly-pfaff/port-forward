#!/usr/bin/env node
/* global console, process, setTimeout */
/**
 * Runtime binary behavior tests for build/portier/service[.exe].
 *
 * Tests actual runtime behavior beyond the layout validation in validate-runtime.js:
 *   1. service starts on a free port and /api/health responds
 *   2. /  returns HTML when a static dir with index.html is present
 *   3. Missing/empty static dir does not prevent API startup; / returns 404
 *   4. Invalid config (bad JSON) → service exits within 5s without serving health
 *   5. Process terminates cleanly after a kill signal
 *
 * Usage:
 *   node scripts/test-runtime.js [--no-build]
 *
 *   --no-build   Use the existing build/portier/ without running build:runtime first.
 */

import { existsSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { createServer } from "node:net";
import { get } from "node:http";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const isWindows = process.platform === "win32";
const npmCommand = isWindows ? "npm.cmd" : "npm";

const packageDir = join(repoRoot, "build", "portier");
const serviceBinary = join(packageDir, isWindows ? "service.exe" : "service");

const args = process.argv.slice(2);
const noBuild = args.includes("--no-build");

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
      res.on("end", () => resolve({ status: res.statusCode, body }));
    });
    req.on("error", reject);
    req.setTimeout(2000, () => req.destroy(new Error("timeout")));
  });
}

async function waitForHealth(url, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const res = await httpGet(url);
      if (res.status === 200) return true;
    } catch {
      // not yet ready
    }
    await new Promise((r) => setTimeout(r, 300));
  }
  return false;
}

function makeTempDir(suffix) {
  const dir = join(tmpdir(), `portier-runtime-${suffix}-${Date.now()}-${process.pid}`);
  mkdirSync(dir, { recursive: true });
  return dir;
}

async function runTest(label, fn) {
  console.log(`\n  [${label}]`);
  try {
    await fn();
  } catch (err) {
    fail(`${label}: unexpected error: ${err.message}`);
  }
}

// ── Test 1: starts on free port, /api/health responds ───────────────────────

async function testStartsHealth() {
  const port = await getFreePort();
  const tempDir = makeTempDir("health");
  const configPath = join(tempDir, "rules.json");
  writeFileSync(configPath, "[]");

  const proc = spawn(serviceBinary, [
    "--service",
    "--config", configPath,
    "--host", "127.0.0.1",
    "--port", String(port),
  ], { stdio: ["ignore", "pipe", "pipe"], detached: false });

  const healthUrl = `http://127.0.0.1:${port}/api/health`;
  try {
    const ready = await waitForHealth(healthUrl, 15000);
    if (!ready) {
      fail("starts-health: /api/health did not respond within 15s");
      return;
    }
    pass("starts-health: /api/health responded 200");

    const res = await httpGet(healthUrl);
    let body;
    try { body = JSON.parse(res.body); } catch { body = {}; }
    if (body.ok === true) {
      pass("starts-health: /api/health body has ok:true");
    } else {
      fail(`starts-health: /api/health body missing ok:true — ${res.body}`);
    }
  } finally {
    proc.kill();
    rmSync(tempDir, { recursive: true, force: true });
  }
}

// ── Test 2: / returns HTML when static dir present ──────────────────────────

async function testStaticServed() {
  const port = await getFreePort();
  const tempDir = makeTempDir("static");
  const configPath = join(tempDir, "rules.json");
  writeFileSync(configPath, "[]");

  // Point to the packaged web/ dir which has real HTML
  const staticDir = join(packageDir, "web");
  if (!existsSync(join(staticDir, "index.html"))) {
    fail("static-served: build/portier/web/index.html not found — run npm run build:runtime first");
    rmSync(tempDir, { recursive: true, force: true });
    return;
  }

  const proc = spawn(serviceBinary, [
    "--service",
    "--config", configPath,
    "--host", "127.0.0.1",
    "--port", String(port),
    "--static-dir", staticDir,
  ], { stdio: ["ignore", "pipe", "pipe"], detached: false });

  const healthUrl = `http://127.0.0.1:${port}/api/health`;
  const rootUrl = `http://127.0.0.1:${port}/`;
  try {
    const ready = await waitForHealth(healthUrl, 15000);
    if (!ready) {
      fail("static-served: service did not start");
      return;
    }
    const res = await httpGet(rootUrl);
    if (res.status === 200 && res.body.toLowerCase().includes("<html")) {
      pass("static-served: / → 200 HTML");
    } else {
      fail(`static-served: / → status=${res.status}, body snippet: ${res.body.slice(0, 100)}`);
    }
  } finally {
    proc.kill();
    rmSync(tempDir, { recursive: true, force: true });
  }
}

// ── Test 3: missing static dir does not prevent API startup ─────────────────

async function testNoStaticDir() {
  const port = await getFreePort();
  const tempDir = makeTempDir("nostatic");
  const configPath = join(tempDir, "rules.json");
  writeFileSync(configPath, "[]");

  // Use a path that exists as a dir but has no index.html
  const emptyStaticDir = join(tempDir, "empty-web");
  mkdirSync(emptyStaticDir);

  const proc = spawn(serviceBinary, [
    "--service",
    "--config", configPath,
    "--host", "127.0.0.1",
    "--port", String(port),
    "--static-dir", emptyStaticDir,
  ], { stdio: ["ignore", "pipe", "pipe"], detached: false });

  const healthUrl = `http://127.0.0.1:${port}/api/health`;
  const rootUrl = `http://127.0.0.1:${port}/`;
  try {
    const ready = await waitForHealth(healthUrl, 15000);
    if (!ready) {
      fail("no-static-dir: service did not start without static dir");
      return;
    }
    pass("no-static-dir: /api/health responded despite missing static dir");

    const res = await httpGet(rootUrl);
    if (res.status !== 200) {
      pass(`no-static-dir: / → ${res.status} (not HTML) when static dir empty`);
    } else {
      fail(`no-static-dir: / → unexpected 200 when static dir has no index.html`);
    }
  } finally {
    proc.kill();
    rmSync(tempDir, { recursive: true, force: true });
  }
}

// ── Test 4: invalid JSON config → service exits within 5s ───────────────────

async function testInvalidConfig() {
  const port = await getFreePort();
  const tempDir = makeTempDir("badcfg");
  const configPath = join(tempDir, "rules.json");
  writeFileSync(configPath, "{this is not json}");

  const proc = spawn(serviceBinary, [
    "--service",
    "--config", configPath,
    "--host", "127.0.0.1",
    "--port", String(port),
  ], { stdio: ["ignore", "pipe", "pipe"], detached: false });

  let exited = false;
  let exitCode = null;
  proc.on("exit", (code) => { exited = true; exitCode = code; });

  const healthUrl = `http://127.0.0.1:${port}/api/health`;
  const healthResponded = await waitForHealth(healthUrl, 5000);

  if (!exited) proc.kill();
  try { rmSync(tempDir, { recursive: true, force: true }); } catch { /* best effort */ }

  if (healthResponded) {
    fail("invalid-config: /api/health responded — service should not start on unparseable config");
    return;
  }
  if (exited) {
    if (exitCode !== 0) {
      pass(`invalid-config: exited with non-zero code (${exitCode}) on invalid JSON config`);
    } else {
      fail("invalid-config: exited with code 0 on invalid JSON config (expected non-zero)");
    }
    return;
  }
  fail("invalid-config: process did not exit or respond within 5s (hung)");
}

// ── Test 5: clean shutdown (process terminates after kill) ───────────────────

async function testCleanShutdown() {
  const port = await getFreePort();
  const tempDir = makeTempDir("shutdown");
  const configPath = join(tempDir, "rules.json");
  writeFileSync(configPath, "[]");

  const proc = spawn(serviceBinary, [
    "--service",
    "--config", configPath,
    "--host", "127.0.0.1",
    "--port", String(port),
  ], { stdio: ["ignore", "pipe", "pipe"], detached: false });

  const healthUrl = `http://127.0.0.1:${port}/api/health`;
  let exited = false;
  proc.on("exit", () => { exited = true; });

  try {
    const ready = await waitForHealth(healthUrl, 15000);
    if (!ready) {
      fail("clean-shutdown: service did not start");
      return;
    }
    pass("clean-shutdown: service started and is healthy");

    proc.kill();

    const deadline = Date.now() + 5000;
    while (!exited && Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 200));
    }

    if (exited) {
      pass("clean-shutdown: process terminated within 5s after kill");
    } else {
      fail("clean-shutdown: process did not terminate within 5s after kill");
      proc.kill("SIGKILL");
    }
  } finally {
    if (!exited) { try { proc.kill("SIGKILL"); } catch { /* best effort */ } }
    try { rmSync(tempDir, { recursive: true, force: true }); } catch { /* best effort */ }
  }
}

// ── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  console.log("[validate:binary] Runtime binary behavior tests\n");

  if (!noBuild) {
    console.log("[validate:binary] Running npm run build:runtime...\n");
    const result = spawnSync(npmCommand, ["run", "build:runtime"], {
      stdio: "inherit",
      cwd: repoRoot,
      shell: isWindows,
    });
    if ((result.status ?? 1) !== 0) {
      console.error("[validate:binary] build:runtime failed.\n");
      process.exit(1);
    }
    console.log("");
  }

  if (!existsSync(serviceBinary)) {
    console.error(`[validate:binary] Binary not found: ${serviceBinary}`);
    console.error("[validate:binary] Run: npm run build:runtime\n");
    process.exit(1);
  }
  pass(`Binary found: ${serviceBinary}`);

  await runTest("starts-health", testStartsHealth);
  await runTest("static-served", testStaticServed);
  await runTest("no-static-dir", testNoStaticDir);
  await runTest("invalid-config", testInvalidConfig);
  await runTest("clean-shutdown", testCleanShutdown);

  console.log(`\n[validate:binary] ${passed} passed, ${failed} failed.\n`);
  if (failed > 0) {
    console.error("[validate:binary] FAILED.\n");
    process.exit(1);
  }
  console.log("[validate:binary] All runtime tests passed.\n");
}

main().catch((err) => {
  console.error("[validate:binary] Unexpected error:", err);
  process.exit(1);
});
