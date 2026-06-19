#!/usr/bin/env node
/* global console, process, setTimeout */
/**
 * Cross-platform runtime validation for the Portier build/portier/ output.
 *
 * Usage:
 *   node scripts/validate-runtime.js [--build] [--smoke] [--recovery-smoke]
 *
 *   --build           run `npm run build:runtime` before validating
 *   --smoke           start the packaged service and verify it responds; also
 *                     runs the configuration-recovery smoke scenario
 *   --recovery-smoke  run only the configuration-recovery smoke scenario (boot
 *                     with a corrupt rules.json and assert /api/runtime.recovery)
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
const cliBinary = isWindows ? "portier.exe" : "portier";
const npmCommand = isWindows ? "npm.cmd" : "npm";

const args = process.argv.slice(2);
const shouldBuild = args.includes("--build");
const shouldSmoke = args.includes("--smoke");
const shouldRecoverySmoke = args.includes("--recovery-smoke");

function readPackageVersion() {
  const pkg = JSON.parse(readFileSync(join(repoRoot, "package.json"), "utf8"));
  return pkg.version;
}
const packageVersion = readPackageVersion();
// The packaged OpenAPI doc version is the major.minor of the package version
// (e.g. 1.18.0 -> "1.18"). Mirrors OPENAPI_DOC_VERSION in server/sources/openapi/openapi.ts.
const expectedOpenApiVersion = (() => {
  const parts = packageVersion.split(".");
  return parts.length >= 2 ? `${parts[0]}.${parts[1]}` : packageVersion;
})();

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

// checkCliVersion runs the packaged CLI binary directly (offline, no network) and
// asserts it reports the root package.json version.
function checkCliVersion() {
  const cliPath = join(packageDir, cliBinary);
  if (!existsSync(cliPath)) {
    fail(`CLI version: binary not found: ${cliBinary}`);
    return;
  }
  const r = spawnSync(cliPath, ["version"], { encoding: "utf8" });
  if ((r.status ?? 1) !== 0) {
    fail(`CLI version: '${cliBinary} version' exited with ${r.status ?? "unknown"}`);
    return;
  }
  const out = (r.stdout || "").trim();
  if (out.includes(packageVersion)) {
    pass(`CLI reports package version ${packageVersion} ("${out}")`);
  } else {
    fail(`CLI version mismatch: got "${out}", expected to include ${packageVersion}`);
  }
}

// checkOpenApiVersion validates the bundled OpenAPI document's structure and that
// info.version matches the major.minor convention derived from the package version.
function checkOpenApiVersion() {
  const apiPath = join(packageDir, "api", "openapi.json");
  if (!existsSync(apiPath)) return; // already reported by checkFile
  let doc;
  try {
    doc = JSON.parse(readFileSync(apiPath, "utf8"));
  } catch {
    fail("api/openapi.json is not valid JSON");
    return;
  }
  if (typeof doc.openapi === "string" && doc.openapi.length > 0) {
    pass(`api/openapi.json openapi = ${doc.openapi}`);
  } else {
    fail("api/openapi.json missing openapi field");
  }
  const infoVer = doc.info && doc.info.version;
  if (infoVer === expectedOpenApiVersion) {
    pass(`api/openapi.json info.version = ${infoVer} (matches package ${packageVersion})`);
  } else {
    fail(
      `api/openapi.json info.version = ${JSON.stringify(infoVer)}, expected ${expectedOpenApiVersion}`
    );
  }
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

      // Packaged Go service version check (the canonical packaged runtime). The
      // TypeScript fallback (server.js) version is not started here; its version
      // surface is covered by the runtime schema test and by the OpenAPI
      // info.version assertion above (the OpenAPI doc is generated from the
      // TypeScript server).
      const rtRes = await httpGet(`http://${host}:${smokePort}/api/runtime`);
      if (rtRes.status !== 200) {
        fail(`Smoke test: GET /api/runtime expected 200, got ${rtRes.status}`);
      } else {
        let rt;
        try {
          rt = JSON.parse(rtRes.body);
        } catch {
          rt = null;
        }
        if (rt && rt.version === packageVersion) {
          pass(`Smoke test: /api/runtime reports version ${packageVersion}`);
        } else {
          fail(
            `Smoke test: /api/runtime version = ${JSON.stringify(rt && rt.version)}, expected ${packageVersion}`
          );
        }
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

// runRecoverySmoke proves the packaged Go runtime survives a corrupt rules.json
// (v1.17 R-1): the management API must still bind, and GET /api/runtime must
// report an active config-recovery block. Uses a free port + temp dir; cleans up
// the temp dir (including the quarantined file) and the process.
async function runRecoverySmoke() {
  console.log("\n[recovery-smoke] Starting configuration-recovery smoke test...");

  const servicePath = join(packageDir, serviceBinary);
  if (!existsSync(servicePath)) {
    fail(`Recovery smoke: service binary not found: ${serviceBinary}`);
    return;
  }

  const port = await getFreePort();
  const tempDir = join(tmpdir(), `portier-recovery-smoke-${process.pid}-${Date.now()}`);
  mkdirSync(tempDir, { recursive: true });
  const tempConfig = join(tempDir, "rules.json");
  // Deliberately corrupt config: must NOT prevent the service from starting.
  writeFileSync(tempConfig, "this is not valid json");

  const host = "127.0.0.1";
  const staticDir = join(packageDir, "web");
  const healthUrl = `http://${host}:${port}/api/health`;

  console.log(`[recovery-smoke]   service  : ${servicePath}`);
  console.log(`[recovery-smoke]   port     : ${port}`);
  console.log(`[recovery-smoke]   config   : ${tempConfig} (corrupt)`);

  const proc = spawn(
    servicePath,
    [
      "--service",
      "--config", tempConfig,
      "--host", host,
      "--port", String(port),
      "--static-dir", staticDir,
    ],
    { stdio: ["ignore", "pipe", "pipe"], detached: false }
  );

  try {
    const ready = await waitForHealth(healthUrl);
    if (!ready) {
      fail("Recovery smoke: service did not respond to /api/health (a corrupt config must NOT prevent startup)");
      return;
    }
    pass("Recovery smoke: service started despite a corrupt rules.json (/api/health OK)");

    const res = await httpGet(`http://${host}:${port}/api/runtime`);
    if (res.status !== 200) {
      fail(`Recovery smoke: GET /api/runtime expected 200, got ${res.status}`);
      return;
    }
    let data;
    try {
      data = JSON.parse(res.body);
    } catch {
      fail(`Recovery smoke: /api/runtime body is not JSON: ${res.body}`);
      return;
    }

    const r = data.recovery;
    if (!r || r.active !== true) {
      fail(`Recovery smoke: recovery.active is not true: ${JSON.stringify(r)}`);
      return;
    }
    pass("Recovery smoke: /api/runtime reports recovery.active = true");

    if (r.reason === "malformed") {
      pass('Recovery smoke: recovery.reason = "malformed"');
    } else {
      fail(`Recovery smoke: recovery.reason = ${JSON.stringify(r.reason)}, want "malformed"`);
    }
    if (r.writesBlocked === true) {
      pass("Recovery smoke: recovery.writesBlocked = true");
    } else {
      fail(`Recovery smoke: recovery.writesBlocked = ${JSON.stringify(r.writesBlocked)}, want true`);
    }
    if (r.configPath === tempConfig) {
      pass("Recovery smoke: recovery.configPath points at the temp config");
    } else {
      fail(`Recovery smoke: recovery.configPath = ${JSON.stringify(r.configPath)}, want ${tempConfig}`);
    }
    if (typeof r.quarantinePath === "string" && r.quarantinePath.length > 0) {
      pass("Recovery smoke: recovery.quarantinePath is present (bad config preserved)");
    } else {
      fail(`Recovery smoke: recovery.quarantinePath missing: ${JSON.stringify(r.quarantinePath)}`);
    }

    const uiRes = await httpGet(`http://${host}:${port}/`);
    if (uiRes.status === 200 && uiRes.body.toLowerCase().includes("<html")) {
      pass("Recovery smoke: web UI still served at / during recovery");
    } else {
      fail(`Recovery smoke: web UI not served at / during recovery (status=${uiRes.status})`);
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
  checkFile(cliBinary);
  checkFile(serviceBinary);
  checkFile("server.js");
  checkFile("readme.txt");

  console.log("\nBinary separation:");
  if (cliBinary !== serviceBinary) {
    pass(`CLI (${cliBinary}) and service (${serviceBinary}) are separate binaries`);
  } else {
    fail(`CLI and service binaries must have different names`);
  }

  console.log("\nRequired web UI:");
  checkDir("web");
  checkFile("web/index.html");
  checkDir("web/assets");

  console.log("\nOpenAPI artifact:");
  checkFile("api/openapi.json");
  checkOpenApiVersion();

  console.log("\nPackaged version reporting:");
  checkCliVersion();

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
    if (/portier(\.exe)?\s+runtime/i.test(text)) {
      pass("readme.txt mentions CLI command: portier runtime");
    } else {
      fail("readme.txt does not mention CLI command: portier runtime");
    }
    if (/portier(\.exe)?\s+list/i.test(text)) {
      pass("readme.txt mentions CLI command: portier list");
    } else {
      fail("readme.txt does not mention CLI command: portier list");
    }
    if (/portier(\.exe)?\s+diagnostics export/i.test(text)) {
      pass("readme.txt mentions CLI command: portier diagnostics export");
    } else {
      fail("readme.txt does not mention CLI command: portier diagnostics export");
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

  // `--smoke` runs the normal startup smoke AND the config-recovery smoke;
  // `--recovery-smoke` runs only the recovery scenario.
  if (shouldSmoke) {
    await runSmoke();
  }
  if (shouldSmoke || shouldRecoverySmoke) {
    await runRecoverySmoke();
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
