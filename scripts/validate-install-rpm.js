#!/usr/bin/env node
/* global console, process */
/**
 * Linux .rpm install/remove smoke for the Portier RPM (mirrors validate-install-deb.js).
 *
 * Proves the file-install .rpm installs the expected layout under /opt/portier with a
 * DISABLED systemd unit, never enables/starts the service, never overwrites user config
 * (/etc/portier/rules.json), and removes cleanly while preserving user config.
 *
 * Linux-only and needs the `rpm` CLI + sudo (passwordless on GitHub-hosted runners). Uses
 * `rpm -i` / `rpm -e` (the rpm database is independent of dpkg, so this is safe to run on an
 * Ubuntu runner that has systemd as PID 1, giving real disabled/inactive assertions). On
 * other platforms it exits 0 with a skip notice. All mutated paths are exact, Portier-specific
 * constants; package files are removed by rpm, and the only file this script deletes is the
 * config sentinel it created (guarded).
 *
 * Usage:
 *   node scripts/validate-install-rpm.js [--rpm <path>] [--keep]
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
const SENTINEL = "portier-rpm-smoke-sentinel";

const rawArgs = process.argv.slice(2);
const flagValue = (f) => {
  const i = rawArgs.indexOf(f);
  return i >= 0 && i + 1 < rawArgs.length ? rawArgs[i + 1] : null;
};
const rpmArg = flagValue("--rpm");
const keep = rawArgs.includes("--keep");

let passed = 0;
let failed = 0;
const pass = (m) => { console.log(`  ✓ ${m}`); passed++; };
const fail = (m) => { console.error(`  ✗ ${m}`); failed++; };

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
    console.error(`  [rpm-smoke] ${label} exited ${r.status}`);
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
function assertExact(actual, expected) {
  if (actual !== expected) {
    throw new Error(`refusing to operate on unexpected path: ${actual} (expected ${expected})`);
  }
}

async function main() {
  if (!isLinux) {
    console.log(`[rpm-smoke] Linux-only; skipping on ${process.platform}.`);
    return;
  }
  if ((sh("rpm", ["--version"]).status ?? 1) !== 0) {
    fail("rpm CLI not available — install the 'rpm' package (Debian/Ubuntu) or use Fedora/RHEL");
    process.exit(1);
  }

  const version = packageVersion();
  const rpmPath = rpmArg
    ? resolve(rpmArg)
    : join(repoRoot, "build", "releases", "linux", `portier-${version}-1.x86_64.rpm`);

  console.log(`[rpm-smoke] Package : ${rpmPath}`);
  if (!existsSync(rpmPath)) {
    fail(`.rpm not found: ${rpmPath}`);
    console.error("[rpm-smoke] Hint: run `npm run build:release:current` (Linux, needs rpmbuild).\n");
    process.exit(1);
  }
  pass(`.rpm present (${(statSync(rpmPath).size / 1024 / 1024).toFixed(1)} MB)`);

  const haveSystemctl = (sh("systemctl", ["--version"]).status ?? 1) === 0;
  console.log(`[rpm-smoke] systemctl available: ${haveSystemctl}`);

  let createdConfig = false;
  try {
    // Seed an external config sentinel to prove it is preserved.
    if (!existsSync(CONFIG_FILE)) {
      assertExact(CONFIG_DIR, "/etc/portier");
      sudo(["mkdir", "-p", CONFIG_DIR], "seed config dir");
      const r = spawnSync("sudo", ["tee", CONFIG_FILE], { input: SENTINEL + "\n", encoding: "utf8" });
      if ((r.status ?? 1) !== 0) fail("could not seed config sentinel");
      else { createdConfig = true; pass(`seeded config sentinel at ${CONFIG_FILE}`); }
    } else {
      console.log(`[rpm-smoke] ${CONFIG_FILE} already exists — leaving it untouched (not seeding).`);
    }

    // ── Install ──
    console.log("\nInstall (rpm -i):");
    const inst = sudo(["rpm", "-i", rpmPath], "rpm -i");
    if (!inst || (inst.status ?? 1) !== 0) {
      fail("rpm -i failed");
      return;
    }
    pass("package installed via rpm -i");

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
    const v = sh(join(INSTALL_DIR, "portier"), ["version"]);
    const out = (v.stdout || "").trim();
    if ((v.status ?? 1) === 0 && out.includes(version)) pass(`portier reports ${version} ("${out}")`);
    else fail(`version mismatch: status=${v.status} out="${out}" expected to include ${version}`);

    // ── Service must be installed DISABLED and NOT started ──
    console.log("\nService state (must be disabled + inactive):");
    if (haveSystemctl) {
      const enabledOut = (sh("systemctl", ["is-enabled", "portier"]).stdout || sh("systemctl", ["is-enabled", "portier"]).stderr || "").trim();
      if (/\benabled\b/.test(enabledOut)) fail(`service is ENABLED ("${enabledOut}") — package must not enable it`);
      else pass(`service not enabled ("${enabledOut}")`);
      const activeOut = (sh("systemctl", ["is-active", "portier"]).stdout || sh("systemctl", ["is-active", "portier"]).stderr || "").trim();
      if (activeOut === "active") fail("service is ACTIVE — package must not start it");
      else pass(`service not active ("${activeOut}")`);
    } else {
      pass("systemctl unavailable — unit file presence already asserted (no start possible)");
    }

    // ── Config preserved after install ──
    console.log("\nConfig preservation (after install):");
    if (createdConfig) {
      if (readFileSync(CONFIG_FILE, "utf8").includes(SENTINEL)) pass("config sentinel intact after install");
      else fail("config sentinel changed by install");
    } else {
      console.log("  - not seeded (pre-existing config left untouched) [skip]");
    }

    // ── Remove ──
    console.log("\nRemove (rpm -e):");
    const rem = sudo(["rpm", "-e", PKG_NAME], "rpm -e");
    if (!rem || (rem.status ?? 1) !== 0) {
      fail("rpm -e failed");
      return;
    }
    pass("package removed via rpm -e");

    console.log("\nPost-remove assertions:");
    if (!existsSync(join(INSTALL_DIR, "portier")) && !existsSync(join(INSTALL_DIR, "service")))
      pass("runtime binaries removed from /opt/portier");
    else fail("runtime binaries remain in /opt/portier after remove");

    if (!existsSync(UNIT_PATH)) pass("systemd unit removed");
    else fail(`systemd unit still present after remove: ${UNIT_PATH}`);

    if (haveSystemctl) {
      const activeOut = (sh("systemctl", ["is-active", "portier"]).stdout || sh("systemctl", ["is-active", "portier"]).stderr || "").trim();
      if (activeOut === "active") fail("service still ACTIVE after remove");
      else pass(`service not active after remove ("${activeOut}")`);
    }

    // ── Config preserved after remove ──
    console.log("\nConfig preservation (after remove):");
    if (createdConfig) {
      if (existsSync(CONFIG_FILE) && readFileSync(CONFIG_FILE, "utf8").includes(SENTINEL))
        pass("config sentinel preserved after remove (rpm erase kept user config)");
      else fail("config sentinel removed/changed by package removal");
    } else {
      console.log("  - not seeded [skip]");
    }
  } finally {
    if (createdConfig && !keep) {
      try {
        assertExact(CONFIG_FILE, "/etc/portier/rules.json");
        sudo(["rm", "-f", CONFIG_FILE], "cleanup config sentinel");
        sudo(["rmdir", "--ignore-fail-on-non-empty", CONFIG_DIR], "cleanup config dir");
      } catch (err) {
        console.error(`[rpm-smoke] cleanup skipped: ${err.message}`);
      }
    }
  }

  console.log(`\n[rpm-smoke] ${passed} passed, ${failed} failed.`);
  if (failed > 0) {
    console.error("[rpm-smoke] .rpm install/remove smoke FAILED.\n");
    process.exit(1);
  }
  console.log("[rpm-smoke] .rpm install/remove smoke passed.\n");
}

main().catch((err) => {
  console.error("[rpm-smoke] Unexpected error:", err);
  process.exit(1);
});
