#!/usr/bin/env node
/* global console, process, setTimeout */
/**
 * Current-platform upgrade-preservation smoke for Portier release artifacts.
 *
 * Proves that replacing the packaged binaries/web assets (an "upgrade") does NOT
 * lose user configuration, recovery backup/quarantine side files, or basic
 * runtime health — the data dir lives outside the disposable install dir.
 *
 * This is a PORTABLE, SAME-VERSION replacement smoke today: it extracts the
 * current-platform portable archive into install dir A, runs the runtime against
 * an external temp data dir, stops it, extracts a fresh copy into install dir B
 * (simulating new binaries), and restarts against the SAME data dir. It does not
 * fake cross-version behavior. It is the safety gate that future WiX/MSI and
 * macOS .pkg installer upgrades must satisfy.
 *
 * Future cross-version extension points (already wired as flags):
 *   --from <archive>   portable archive to install first   (default: current)
 *   --to   <archive>   portable archive to upgrade to      (default: --from)
 *   --data-dir <dir>   reuse an existing data dir instead of a temp one
 *   --keep-temp        do not delete temp dirs (debugging)
 *
 * Current-platform safe: no admin, no OS service install, free ports, temp dirs,
 * deterministic cleanup. Exits non-zero on failure.
 *
 * Usage:
 *   node scripts/validate-upgrade.js [--from <archive>] [--to <archive>]
 *                                    [--data-dir <dir>] [--keep-temp]
 */

import { existsSync, mkdirSync, writeFileSync, readFileSync, rmSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { tmpdir } from "node:os";
import { spawn, spawnSync } from "node:child_process";
import { createServer } from "node:net";
import { get } from "node:http";
import { fileURLToPath } from "node:url";
import AdmZip from "adm-zip";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const isWindows = process.platform === "win32";
const serviceBinary = isWindows ? "service.exe" : "service";

const platformLabel =
  process.platform === "win32" ? "windows" : process.platform === "darwin" ? "macos" : "linux";

// ── Arguments ─────────────────────────────────────────────────────────────────

const rawArgs = process.argv.slice(2);
const hasFlag = (f) => rawArgs.includes(f);
const flagValue = (f) => {
  const i = rawArgs.indexOf(f);
  return i >= 0 && i + 1 < rawArgs.length ? rawArgs[i + 1] : null;
};
const keepTemp = hasFlag("--keep-temp");
const fromArg = flagValue("--from");
const toArg = flagValue("--to");
const dataDirArg = flagValue("--data-dir");

// ── Pure helpers (exported for tests) ─────────────────────────────────────────

function readPackageVersion() {
  return JSON.parse(readFileSync(join(repoRoot, "package.json"), "utf8")).version;
}

// portableArchiveName returns the portable archive filename for a platform/version.
export function portableArchiveName(label, version) {
  if (label === "windows") return `portier-${version}-windows-portable.zip`;
  if (label === "macos") return `portier-portable-macos-${version}.tar.gz`;
  return `portier-${version}-linux.tar.gz`;
}

// buildSentinelConfig returns a persisted rules array (bare array, current schema)
// with a single distinctive, disabled rule so no listen binding is attempted.
export function buildSentinelConfig(name) {
  return [
    {
      id: name,
      name,
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48999,
      targetHost: "127.0.0.1",
      targetPort: 49999,
      enabled: false,
    },
  ];
}

// findRule parses persisted config text and returns the rule with the given name,
// or null. Used to assert the sentinel survived an upgrade.
export function findRule(configText, name) {
  let rules;
  try {
    rules = JSON.parse(configText);
  } catch {
    return null;
  }
  if (!Array.isArray(rules)) return null;
  return rules.find((r) => r && r.name === name) ?? null;
}

// ── Result tracking ───────────────────────────────────────────────────────────

let passed = 0;
let failed = 0;
let warned = 0;
const pass = (m) => { console.log(`  ✓ ${m}`); passed++; };
const fail = (m) => { console.error(`  ✗ ${m}`); failed++; };

// ── Runtime helpers ───────────────────────────────────────────────────────────

function getFreePort() {
  return new Promise((res, rej) => {
    const srv = createServer();
    srv.listen(0, "127.0.0.1", () => {
      const { port } = srv.address();
      srv.close(() => res(port));
    });
    srv.on("error", rej);
  });
}

function httpGet(url) {
  return new Promise((res, rej) => {
    const req = get(url, (r) => {
      let body = "";
      r.on("data", (d) => { body += d; });
      r.on("end", () => res({ status: r.statusCode, body }));
    });
    req.on("error", rej);
    req.setTimeout(2000, () => req.destroy(new Error("timeout")));
  });
}

async function waitForHealth(url, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const r = await httpGet(url);
      if (r.status === 200) return true;
    } catch {
      // not ready yet
    }
    await new Promise((r) => setTimeout(r, 400));
  }
  return false;
}

function extractArchive(archivePath, destDir) {
  mkdirSync(destDir, { recursive: true });
  if (archivePath.endsWith(".zip")) {
    new AdmZip(archivePath).extractAllTo(destDir, true);
    return true;
  }
  const r = spawnSync("tar", ["-xzf", archivePath, "-C", destDir], { stdio: "inherit" });
  return (r.status ?? 1) === 0;
}

// startRuntime launches the packaged service against the given install + data dir
// and waits for health. Returns { proc, port } or { proc: null } on failure.
async function startRuntime(installDir, configPath) {
  const servicePath = join(installDir, serviceBinary);
  if (!existsSync(servicePath)) {
    fail(`service binary not found in install dir: ${servicePath}`);
    return { proc: null };
  }
  const port = await getFreePort();
  const staticDir = join(installDir, "web");
  const proc = spawn(
    servicePath,
    ["--service", "--config", configPath, "--host", "127.0.0.1", "--port", String(port), "--static-dir", staticDir],
    { stdio: ["ignore", "pipe", "pipe"], detached: false }
  );
  const ready = await waitForHealth(`http://127.0.0.1:${port}/api/health`);
  if (!ready) {
    fail(`runtime did not respond to /api/health within 15s (install: ${installDir})`);
    proc.kill();
    return { proc: null };
  }
  return { proc, port };
}

function stopRuntime(proc) {
  if (proc) proc.kill();
}

// ── Main ──────────────────────────────────────────────────────────────────────

async function main() {
  const version = readPackageVersion();
  const releasesDir = join(repoRoot, "build", "releases", platformLabel);

  const fromArchive = fromArg
    ? resolve(fromArg)
    : join(releasesDir, portableArchiveName(platformLabel, version));
  const toArchive = toArg ? resolve(toArg) : fromArchive;
  const sameVersion = !toArg && !fromArg;

  console.log(`[validate-upgrade] Platform : ${platformLabel}`);
  console.log(`[validate-upgrade] Version  : ${version}`);
  console.log(`[validate-upgrade] From     : ${fromArchive}`);
  console.log(`[validate-upgrade] To       : ${toArchive}${sameVersion ? " (same-version replacement smoke)" : ""}`);
  console.log("");

  if (!existsSync(fromArchive)) {
    fail(`Portable archive not found: ${fromArchive}`);
    console.error("[validate-upgrade] Hint: run `npm run build:release:current` first.\n");
    process.exit(1);
  }
  if (!existsSync(toArchive)) {
    fail(`Upgrade-to archive not found: ${toArchive}`);
    process.exit(1);
  }

  const tempRoot = join(tmpdir(), `portier-upgrade-${process.pid}-${Date.now()}`);
  const installA = join(tempRoot, "install-A");
  const installB = join(tempRoot, "install-B");
  const dataDir = dataDirArg ? resolve(dataDirArg) : join(tempRoot, "data");
  mkdirSync(dataDir, { recursive: true });
  const configPath = join(dataDir, "rules.json");

  let procA = null;
  let procB = null;
  let portB;
  try {
    // 1. Extract install A and seed the external data dir.
    console.log("Setup:");
    if (extractArchive(fromArchive, installA)) pass("extracted install dir A from portable archive");
    else { fail("could not extract install dir A"); return; }

    const sentinelName = `upgrade-sentinel-${Date.now()}`;
    const configText = JSON.stringify(buildSentinelConfig(sentinelName), null, 2);
    writeFileSync(configPath, configText);
    pass(`wrote sentinel rules.json (rule: ${sentinelName})`);

    // Recovery side files (backup/quarantine) live next to rules.json. They must
    // survive an upgrade. Seed simple sentinels and assert they are untouched.
    const backupPath = `${configPath}.backup-sentinel`;
    const quarantinePath = `${configPath}.quarantine-sentinel`;
    const backupContent = `backup-sentinel-${sentinelName}`;
    const quarantineContent = `quarantine-sentinel-${sentinelName}`;
    writeFileSync(backupPath, backupContent);
    writeFileSync(quarantinePath, quarantineContent);
    pass("seeded recovery backup/quarantine sentinel side files");

    // 2. Run the pre-upgrade runtime from install A.
    console.log("\nPre-upgrade runtime (install A):");
    ({ proc: procA } = await startRuntime(installA, configPath));
    if (!procA) return;
    pass("install A runtime started (/api/health OK)");
    stopRuntime(procA);
    procA = null;
    pass("install A runtime stopped");

    // 3. Simulate the upgrade: fresh binaries in install B, old install removed.
    console.log("\nUpgrade (replace install dir):");
    if (extractArchive(toArchive, installB)) pass("extracted install dir B (upgraded binaries)");
    else { fail("could not extract install dir B"); return; }
    rmSync(installA, { recursive: true, force: true });
    pass("removed install dir A (binaries replaced)");

    // 4. Run the post-upgrade runtime from install B against the SAME data dir.
    console.log("\nPost-upgrade runtime (install B):");
    ({ proc: procB, port: portB } = await startRuntime(installB, configPath));
    if (!procB) return;
    pass("install B runtime started (/api/health OK)");

    // 5. Preservation + health assertions.
    console.log("\nConfig & data preservation:");
    if (existsSync(configPath)) pass("rules.json still exists after upgrade");
    else { fail("rules.json missing after upgrade"); return; }

    const afterText = readFileSync(configPath, "utf8");
    if (afterText === configText) pass("rules.json content unchanged (not overwritten/migrated)");
    else fail("rules.json content changed during upgrade (unexpected rewrite)");

    const sentinel = findRule(afterText, sentinelName);
    if (sentinel) {
      pass(`sentinel rule preserved on disk (${sentinelName})`);
      const expected = buildSentinelConfig(sentinelName)[0];
      if (JSON.stringify(sentinel) === JSON.stringify(expected)) pass("sentinel rule fields preserved exactly");
      else fail(`sentinel rule fields changed: ${JSON.stringify(sentinel)}`);
    } else {
      fail("sentinel rule missing after upgrade");
    }

    if (readFileSync(backupPath, "utf8") === backupContent) pass("recovery backup side file preserved");
    else fail("recovery backup side file changed/removed");
    if (readFileSync(quarantinePath, "utf8") === quarantineContent) pass("recovery quarantine side file preserved");
    else fail("recovery quarantine side file changed/removed");

    console.log("\nPost-upgrade runtime health:");
    const rt = await httpGet(`http://127.0.0.1:${portB}/api/runtime`);
    if (rt.status === 200) {
      let data;
      try { data = JSON.parse(rt.body); } catch { data = null; }
      if (data && data.version === version) pass(`/api/runtime version = ${version}`);
      else fail(`/api/runtime version = ${JSON.stringify(data && data.version)}, expected ${version}`);
      const rec = data && data.recovery;
      if (rec && rec.active === false) pass("/api/runtime recovery.active = false (valid config, no recovery)");
      else fail(`/api/runtime recovery.active = ${JSON.stringify(rec && rec.active)}, expected false`);
    } else {
      fail(`GET /api/runtime expected 200, got ${rt.status}`);
    }

    // Prove the upgraded runtime actually LOADED the preserved config.
    const fwd = await httpGet(`http://127.0.0.1:${portB}/api/forwards`);
    if (fwd.status === 200) {
      let rules;
      try { rules = JSON.parse(fwd.body); } catch { rules = null; }
      const loaded = Array.isArray(rules) && rules.some((r) => r && r.name === sentinelName);
      if (loaded) pass("upgraded runtime loaded the preserved sentinel rule (GET /api/forwards)");
      else fail("upgraded runtime did not load the sentinel rule");
    } else {
      fail(`GET /api/forwards expected 200, got ${fwd.status}`);
    }

    const ui = await httpGet(`http://127.0.0.1:${portB}/`);
    if (ui.status === 200 && ui.body.toLowerCase().includes("<html")) pass("web UI served at / after upgrade");
    else fail(`web UI not served at / after upgrade (status=${ui.status})`);
  } finally {
    stopRuntime(procA);
    stopRuntime(procB);
    if (keepTemp) {
      console.log(`\n[validate-upgrade] --keep-temp: left temp dir ${tempRoot}`);
    } else {
      try { rmSync(tempRoot, { recursive: true, force: true }); } catch { /* best effort */ }
    }
  }

  console.log(`\n[validate-upgrade] ${passed} passed, ${warned} warned, ${failed} failed.`);
  if (failed > 0) {
    console.error("[validate-upgrade] Upgrade-preservation smoke FAILED.\n");
    process.exit(1);
  }
  console.log("[validate-upgrade] Upgrade-preservation smoke passed.\n");
}

// Run only when invoked directly so tests can import the pure helpers.
const invokedDirectly =
  process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (invokedDirectly) {
  main().catch((err) => {
    console.error("[validate-upgrade] Unexpected error:", err);
    process.exit(1);
  });
}
