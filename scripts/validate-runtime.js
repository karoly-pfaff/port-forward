#!/usr/bin/env node
/* global console, process, setTimeout */
/**
 * Cross-platform runtime validation for the Portier build/portier/ output.
 *
 * Usage:
 *   node scripts/validate-runtime.js [--build] [--smoke]
 *
 *   --build   run `npm run build:runtime` before validating
 *   --smoke   start the packaged service and verify it responds
 */

import { existsSync, statSync, readFileSync, writeFileSync, mkdirSync, rmSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { spawnSync, spawn } from "node:child_process";
import { createServer } from "node:net";
import { get } from "node:http";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const packageDir = join(repoRoot, "build", "portier");
const isWindows = process.platform === "win32";
const serviceBinary = isWindows ? "service.exe" : "service";
const npmCommand = isWindows ? "npm.cmd" : "npm";

const args = process.argv.slice(2);
const shouldBuild = args.includes("--build");
const shouldSmoke = args.includes("--smoke");

let passed = 0;
let failed = 0;
let warned = 0;

function pass(msg) {
  console.log(`  ✓ ${msg}`);
  passed++;
}

function fail(msg) {
  console.error(`  ✗ ${msg}`);
  failed++;
}

function warn(msg) {
  console.warn(`  ! ${msg}`);
  warned++;
}

function checkFile(rel) {
  const full = join(packageDir, rel);
  if (!existsSync(full)) {
    fail(`Missing required file: ${rel}`);
    return false;
  }
  const stat = statSync(full);
  if (!stat.isFile()) {
    fail(`Expected a file: ${rel}`);
    return false;
  }
  if (stat.size === 0) {
    fail(`Empty file: ${rel}`);
    return false;
  }
  pass(`${rel} (${stat.size.toLocaleString()} bytes)`);
  return true;
}

function checkDir(rel) {
  const full = join(packageDir, rel);
  if (!existsSync(full)) {
    fail(`Missing required directory: ${rel}/`);
    return false;
  }
  if (!statSync(full).isDirectory()) {
    fail(`Expected a directory: ${rel}`);
    return false;
  }
  pass(`${rel}/`);
  return true;
}

function checkAbsent(rel) {
  if (existsSync(join(packageDir, rel))) {
    fail(`Package must not contain: ${rel}`);
    return false;
  }
  pass(`Absent (correct): ${rel}`);
  return true;
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

async function waitForHealth(url, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const res = await httpGet(url);
      if (res.status === 200) return true;
    } catch {
      // not ready yet
    }
    await new Promise((r) => setTimeout(r, 400));
  }
  return false;
}

async function runSmoke() {
  console.log("\n[smoke] Starting smoke test...");

  const servicePath = join(packageDir, serviceBinary);
  if (!existsSync(servicePath)) {
    fail(`Smoke test: service binary not found: ${serviceBinary}`);
    return;
  }

  const smokePort = await getFreePort();
  const tempDir = join(tmpdir(), `portier-smoke-${process.pid}`);
  mkdirSync(tempDir, { recursive: true });
  const tempConfig = join(tempDir, "rules.json");
  writeFileSync(tempConfig, "[]");

  const host = "127.0.0.1";
  const staticDir = join(packageDir, "web");
  const healthUrl = `http://${host}:${smokePort}/api/health`;

  console.log(`[smoke]   service  : ${servicePath}`);
  console.log(`[smoke]   port     : ${smokePort}`);
  console.log(`[smoke]   config   : ${tempConfig}`);
  console.log(`[smoke]   static   : ${staticDir}`);

  const proc = spawn(
    servicePath,
    [
      "--service",
      "--config", tempConfig,
      "--host", host,
      "--port", String(smokePort),
      "--static-dir", staticDir,
    ],
    { stdio: ["ignore", "pipe", "pipe"], detached: false }
  );

  try {
    const ready = await waitForHealth(healthUrl);
    if (!ready) {
      fail("Smoke test: service did not respond to /api/health within 15s");
    } else {
      pass("Smoke test: /api/health responded OK");
      const uiRes = await httpGet(`http://${host}:${smokePort}/`);
      if (uiRes.status === 200 && uiRes.body.toLowerCase().includes("<html")) {
        pass("Smoke test: web UI served at /");
      } else {
        fail(`Smoke test: web UI not served at / (status=${uiRes.status})`);
      }
    }
  } finally {
    proc.kill();
    try {
      rmSync(tempDir, { recursive: true, force: true });
    } catch {
      // best effort
    }
  }
}

async function main() {
  if (shouldBuild) {
    console.log("[validate-runtime] Running npm run build:runtime...\n");
    const result = spawnSync(npmCommand, ["run", "build:runtime"], {
      stdio: "inherit",
      cwd: repoRoot,
      shell: isWindows,
    });
    if ((result.status ?? 1) !== 0) {
      console.error("[validate-runtime] build:runtime failed.");
      process.exit(1);
    }
    console.log("");
  }

  console.log(`[validate-runtime] Validating: ${packageDir}\n`);

  if (!existsSync(packageDir)) {
    fail("build/portier/ does not exist — run: npm run build:runtime");
    console.error(`\n[validate-runtime] Validation FAILED (runtime directory missing).\n`);
    process.exit(1);
  }
  if (!statSync(packageDir).isDirectory()) {
    fail("build/portier exists but is not a directory");
    process.exit(1);
  }
  pass("build/portier/ exists");

  console.log("\nRequired files:");
  checkFile(serviceBinary);
  checkFile("server.js");
  checkFile("readme.txt");

  console.log("\nRequired web UI:");
  checkDir("web");
  checkFile("web/index.html");
  checkDir("web/assets");

  console.log("\nreadme.txt content:");
  const readmePath = join(packageDir, "readme.txt");
  if (existsSync(readmePath)) {
    const text = readFileSync(readmePath, "utf8");
    if (text.includes("127.0.0.1:47831")) {
      pass("readme.txt mentions management URL (127.0.0.1:47831)");
    } else {
      fail("readme.txt does not mention management URL (127.0.0.1:47831)");
    }
    if (/rules\.json|config/i.test(text)) {
      pass("readme.txt mentions config path");
    } else {
      fail("readme.txt does not mention config path");
    }
    if (/install|scripts\//i.test(text)) {
      pass("readme.txt references install scripts or docs");
    } else {
      warn("readme.txt does not reference install scripts or docs");
    }
  }

  console.log("\nForbidden content (must not be present):");
  checkAbsent("node_modules");
  checkAbsent("rules.json");
  checkAbsent(".env");
  checkAbsent("sources");
  checkAbsent("client");
  // check 'server' dir (not server.js file — those are different paths)
  checkAbsent("server");

  console.log(
    `\n[validate-runtime] ${passed} passed, ${warned} warned, ${failed} failed.`
  );

  if (failed > 0) {
    console.error("[validate-runtime] Validation FAILED.\n");
    process.exit(1);
  }

  console.log("[validate-runtime] Runtime layout validated.\n");

  if (shouldSmoke) {
    await runSmoke();
    console.log(
      `\n[validate-runtime] ${passed} passed, ${warned} warned, ${failed} failed (including smoke).`
    );
    if (failed > 0) {
      console.error("[validate-runtime] Smoke test FAILED.\n");
      process.exit(1);
    }
    console.log("[validate-runtime] Smoke test passed.\n");
  }
}

main().catch((err) => {
  console.error("[validate-runtime] Unexpected error:", err);
  process.exit(1);
});
