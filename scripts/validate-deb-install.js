#!/usr/bin/env node
/* global console, process */
/**
 * Linux .deb install/remove smoke for the Portier Debian package.
 *
 * Proves the file-install .deb installs the expected layout under /opt/portier with a
 * DISABLED systemd unit, never enables/starts the service, never overwrites user config
 * (/etc/portier/rules.json), and removes cleanly while preserving user config.
 *
 * Linux-only. On other platforms it exits 0 with a skip notice so cross-platform release
 * validation is unaffected. Requires root for apt/dpkg — uses `sudo` (passwordless on
 * GitHub-hosted ubuntu runners). All mutated paths are exact, Portier-specific constants;
 * removal of runtime files is done by the package manager, and the only files this script
 * deletes are the config sentinel it created (guarded).
 *
 * Usage:
 *   node scripts/validate-deb-install.js [--deb <path>] [--keep]
 */

import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const isLinux = process.platform === "linux";

// Exact, Portier-specific paths. Never derived from variables.
const INSTALL_DIR = "/opt/portier";
const UNIT_PATH = "/lib/systemd/system/portier.service";
const CONFIG_DIR = "/etc/portier";
const CONFIG_FILE = "/etc/portier/rules.json";
const PKG_NAME = "portier";
const SENTINEL = "portier-deb-smoke-sentinel";

// ── Arguments ─────────────────────────────────────────────────────────────────
const rawArgs = process.argv.slice(2);
const flagValue = (f) => {
  const i = rawArgs.indexOf(f);
  return i >= 0 && i + 1 < rawArgs.length ? rawArgs[i + 1] : null;
};
const debArg = flagValue("--deb");
const keep = rawArgs.includes("--keep");

// ── Result tracking ───────────────────────────────────────────────────────────
let passed = 0;
let failed = 0;
const pass = (m) => { console.log(`  ✓ ${m}`); passed++; };
const fail = (m) => { console.error(`  ✗ ${m}`); failed++; };

// ── Helpers ───────────────────────────────────────────────────────────────────
function sh(cmd, args) {
  return spawnSync(cmd, args, { encoding: "utf8" });
}
function sudo(args, label) {
  const r = spawnSync("sudo", args, { encoding: "utf8" });
  if (r.error) {
    fail(`${label}: could not run sudo ${args.join(" ")} (${r.error.message})`);
    return null;
  }
  if ((r.status ?? 1) !== 0) {
    console.error(`  [deb-smoke] ${label} exited ${r.status}`);
    if (r.stdout) console.error(r.stdout.trim());
    if (r.stderr) console.error(r.stderr.trim());
  }
  return r;
}

function packageVersion() {
  return JSON.parse(readFileSync(join(repoRoot, "package.json"), "utf8")).version;
}

function listFilesRecursive(dir) {
  const out = [];
  const walk = (d) => {
    for (const name of readdirSync(d)) {
      const full = join(d, name);
      if (statSync(full).isDirectory()) walk(full);
      else out.push(full);
    }
  };
  if (existsSync(dir)) walk(dir);
  return out;
}

// Guard: refuse to touch anything that is not the exact expected path.
function assertExact(actual, expected) {
  if (actual !== expected) {
    throw new Error(`refusing to operate on unexpected path: ${actual} (expected ${expected})`);
  }
}

// ── Main ──────────────────────────────────────────────────────────────────────
async function main() {
  if (!isLinux) {
    console.log(`[deb-smoke] Linux-only; skipping on ${process.platform}.`);
    return;
  }

  const version = packageVersion();
  const debPath = debArg
    ? resolve(debArg)
    : join(repoRoot, "build", "releases", "linux", `portier_${version}_amd64.deb`);

  console.log(`[deb-smoke] Package : ${debPath}`);
  if (!existsSync(debPath)) {
    fail(`.deb not found: ${debPath}`);
    console.error("[deb-smoke] Hint: run `npm run build:release:current` (Linux).\n");
    process.exit(1);
  }
  pass(`.deb present (${(statSync(debPath).size / 1024 / 1024).toFixed(1)} MB)`);

  // systemctl availability (GitHub ubuntu runners run systemd as PID 1).
  const haveSystemctl = (sh("systemctl", ["--version"]).status ?? 1) === 0;
  console.log(`[deb-smoke] systemctl available: ${haveSystemctl}`);

  let createdConfig = false;
  try {
    // ── Seed an external config sentinel to prove it is preserved ──
    if (!existsSync(CONFIG_FILE)) {
      assertExact(CONFIG_DIR, "/etc/portier");
      sudo(["mkdir", "-p", CONFIG_DIR], "seed config dir");
      // write sentinel via tee (needs root)
      const r = spawnSync("sudo", ["tee", CONFIG_FILE], { input: SENTINEL + "\n", encoding: "utf8" });
      if ((r.status ?? 1) !== 0) fail("could not seed config sentinel");
      else { createdConfig = true; pass(`seeded config sentinel at ${CONFIG_FILE}`); }
    } else {
      console.log(`[deb-smoke] ${CONFIG_FILE} already exists — leaving it untouched (not seeding).`);
    }

    // ── Install ──
    console.log("\nInstall (apt-get install):");
    const inst = sudo(["apt-get", "install", "-y", debPath], "apt-get install");
    if (!inst || (inst.status ?? 1) !== 0) {
      fail("apt-get install failed");
      return;
    }
    pass("package installed via apt-get");

    // ── Installed layout ──
    console.log("\nInstalled layout:");
    for (const rel of ["portier", "service", "server.js", "api/openapi.json", "web/index.html"]) {
      if (existsSync(join(INSTALL_DIR, rel))) pass(`/opt/portier/${rel}`);
      else fail(`missing: /opt/portier/${rel}`);
    }
    if (existsSync(UNIT_PATH)) pass(`systemd unit present: ${UNIT_PATH}`);
    else fail(`missing systemd unit: ${UNIT_PATH}`);

    // ── No user config inside the install dir ──
    console.log("\nConfig/data boundary:");
    if (listFilesRecursive(INSTALL_DIR).some((f) => f.endsWith("rules.json")))
      fail("rules.json must NOT be inside /opt/portier");
    else pass("no rules.json inside /opt/portier");

    // ── Installed CLI version ──
    console.log("\nInstalled CLI version:");
    const cli = join(INSTALL_DIR, "portier");
    const v = sh(cli, ["version"]);
    const out = (v.stdout || "").trim();
    if ((v.status ?? 1) === 0 && out.includes(version)) pass(`portier reports ${version} ("${out}")`);
    else fail(`version mismatch: status=${v.status} out="${out}" expected to include ${version}`);

    // ── Service must be installed DISABLED and NOT started ──
    console.log("\nService state (must be disabled + inactive):");
    if (haveSystemctl) {
      const enabled = sh("systemctl", ["is-enabled", "portier"]);
      const enabledOut = (enabled.stdout || enabled.stderr || "").trim();
      if (/\benabled\b/.test(enabledOut)) fail(`service is ENABLED ("${enabledOut}") — package must not enable it`);
      else pass(`service not enabled ("${enabledOut}")`);

      const active = sh("systemctl", ["is-active", "portier"]);
      const activeOut = (active.stdout || active.stderr || "").trim();
      if (activeOut === "active") fail("service is ACTIVE — package must not start it");
      else pass(`service not active ("${activeOut}")`);
    } else {
      pass("systemctl unavailable — unit file presence already asserted (no start possible)");
    }

    // ── Config preserved after install ──
    console.log("\nConfig preservation (after install):");
    if (createdConfig) {
      const c = readFileSync(CONFIG_FILE, "utf8");
      if (c.includes(SENTINEL)) pass("config sentinel intact after install");
      else fail("config sentinel changed by install");
    } else {
      console.log("  - not seeded (pre-existing config left untouched) [skip]");
    }

    // ── Remove ──
    console.log("\nRemove (apt-get remove):");
    const rem = sudo(["apt-get", "remove", "-y", PKG_NAME], "apt-get remove");
    if (!rem || (rem.status ?? 1) !== 0) {
      fail("apt-get remove failed");
      return;
    }
    pass("package removed via apt-get");

    console.log("\nPost-remove assertions:");
    if (!existsSync(join(INSTALL_DIR, "portier")) && !existsSync(join(INSTALL_DIR, "service")))
      pass("runtime binaries removed from /opt/portier");
    else fail("runtime binaries remain in /opt/portier after remove");

    if (!existsSync(UNIT_PATH)) pass("systemd unit removed");
    else fail(`systemd unit still present after remove: ${UNIT_PATH}`);

    if (haveSystemctl) {
      const active = sh("systemctl", ["is-active", "portier"]);
      const activeOut = (active.stdout || active.stderr || "").trim();
      if (activeOut === "active") fail("service still ACTIVE after remove");
      else pass(`service not active after remove ("${activeOut}")`);
    }

    // ── Config preserved after remove ──
    console.log("\nConfig preservation (after remove):");
    if (createdConfig) {
      if (existsSync(CONFIG_FILE) && readFileSync(CONFIG_FILE, "utf8").includes(SENTINEL))
        pass("config sentinel preserved after remove (apt remove kept user config)");
      else fail("config sentinel removed/changed by package removal");
    } else {
      console.log("  - not seeded [skip]");
    }
  } finally {
    // Cleanup ONLY the sentinel we created, under the exact config path.
    if (createdConfig && !keep) {
      try {
        assertExact(CONFIG_FILE, "/etc/portier/rules.json");
        sudo(["rm", "-f", CONFIG_FILE], "cleanup config sentinel");
        sudo(["rmdir", "--ignore-fail-on-non-empty", CONFIG_DIR], "cleanup config dir");
      } catch (err) {
        console.error(`[deb-smoke] cleanup skipped: ${err.message}`);
      }
    }
  }

  console.log(`\n[deb-smoke] ${passed} passed, ${failed} failed.`);
  if (failed > 0) {
    console.error("[deb-smoke] .deb install/remove smoke FAILED.\n");
    process.exit(1);
  }
  console.log("[deb-smoke] .deb install/remove smoke passed.\n");
}

main().catch((err) => {
  console.error("[deb-smoke] Unexpected error:", err);
  process.exit(1);
});
