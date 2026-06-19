#!/usr/bin/env node
/* global console, process */
/**
 * macOS .pkg install/uninstall smoke for the Portier native package.
 *
 * Proves the unsigned file-install .pkg installs the expected layout under
 * /usr/local/portier (including the bundled, NOT-loaded LaunchAgent scripts), reports the
 * right version, never loads/starts a LaunchAgent, never creates or overwrites user config
 * (~/Library/Application Support/Portier), and can be removed cleanly while preserving user
 * config.
 *
 * macOS-only. On other platforms it exits 0 with a skip notice so cross-platform release
 * validation is unaffected. Requires root for `installer` — uses `sudo` (passwordless on
 * GitHub-hosted macOS runners). pkgbuild packages carry no uninstaller, so removal is the
 * documented manual cleanup: delete the exact install dir + `pkgutil --forget`. The only
 * paths this script deletes are the exact Portier install dir and the user-data sentinel it
 * created (both guarded).
 *
 * Usage:
 *   node scripts/validate-pkg-install.js [--pkg <path>] [--keep]
 */

import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { homedir } from "node:os";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const isMac = process.platform === "darwin";

// Exact, Portier-specific paths. Never derived from variables.
const INSTALL_DIR = "/usr/local/portier";
const PKG_IDENTIFIER = "com.portier.portier";
const USER_DATA_DIR = join(homedir(), "Library", "Application Support", "Portier");
const USER_DATA_FILE = join(USER_DATA_DIR, "rules.json");
const SENTINEL = "portier-pkg-smoke-sentinel";

// ── Arguments ─────────────────────────────────────────────────────────────────
const rawArgs = process.argv.slice(2);
const flagValue = (f) => {
  const i = rawArgs.indexOf(f);
  return i >= 0 && i + 1 < rawArgs.length ? rawArgs[i + 1] : null;
};
const pkgArg = flagValue("--pkg");
const keep = rawArgs.includes("--keep");

// ── Result tracking ───────────────────────────────────────────────────────────
let passed = 0;
let failed = 0;
const pass = (m) => { console.log(`  ✓ ${m}`); passed++; };
const fail = (m) => { console.error(`  ✗ ${m}`); failed++; };

function sh(cmd, args, opts = {}) {
  return spawnSync(cmd, args, { encoding: "utf8", ...opts });
}
function sudo(args, label) {
  const r = spawnSync("sudo", args, { encoding: "utf8" });
  if (r.error) {
    fail(`${label}: could not run sudo ${args.join(" ")} (${r.error.message})`);
    return null;
  }
  if ((r.status ?? 1) !== 0) {
    console.error(`  [pkg-smoke] ${label} exited ${r.status}`);
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
function launchAgentLoaded() {
  const r = sh("launchctl", ["list"]);
  const out = (r.stdout || "") + (r.stderr || "");
  return /portier/i.test(out);
}

// ── Main ──────────────────────────────────────────────────────────────────────
async function main() {
  if (!isMac) {
    console.log(`[pkg-smoke] macOS-only; skipping on ${process.platform}.`);
    return;
  }

  const version = packageVersion();
  const pkgPath = pkgArg
    ? resolve(pkgArg)
    : join(repoRoot, "build", "releases", "macos", `Portier-${version}.pkg`);

  console.log(`[pkg-smoke] Package : ${pkgPath}`);
  if (!existsSync(pkgPath)) {
    fail(`.pkg not found: ${pkgPath}`);
    console.error("[pkg-smoke] Hint: run `npm run build:release:current` (macOS, needs pkgbuild).\n");
    process.exit(1);
  }
  pass(`.pkg present (${(statSync(pkgPath).size / 1024 / 1024).toFixed(1)} MB)`);

  let createdUserData = false;
  try {
    // ── Seed an external user-data sentinel to prove it is preserved ──
    if (!existsSync(USER_DATA_FILE)) {
      sh("mkdir", ["-p", USER_DATA_DIR]); // user-owned; no sudo needed
      const w = sh("tee", [USER_DATA_FILE], { input: SENTINEL + "\n" });
      if ((w.status ?? 1) !== 0) fail("could not seed user-data sentinel");
      else { createdUserData = true; pass(`seeded user-data sentinel at ${USER_DATA_FILE}`); }
    } else {
      console.log(`[pkg-smoke] ${USER_DATA_FILE} already exists — leaving it untouched (not seeding).`);
    }

    // LaunchAgent must not be loaded before install (sanity).
    if (launchAgentLoaded()) fail("a portier LaunchAgent is already loaded before install");
    else pass("no portier LaunchAgent loaded before install");

    // ── Install ──
    console.log("\nInstall (installer -pkg -target /):");
    const inst = sudo(["installer", "-pkg", pkgPath, "-target", "/"], "installer");
    if (!inst || (inst.status ?? 1) !== 0) {
      fail("installer failed");
      return;
    }
    pass("package installed via installer(8)");

    // ── Installed layout ──
    console.log("\nInstalled layout:");
    for (const rel of ["portier", "service", "server.js", "api/openapi.json", "web/index.html", "readme.txt"]) {
      if (existsSync(join(INSTALL_DIR, rel))) pass(`/usr/local/portier/${rel}`);
      else fail(`missing: /usr/local/portier/${rel}`);
    }
    // Bundled LaunchAgent scripts (installed but NOT loaded).
    for (const rel of ["service-scripts/com.portier.plist.example", "service-scripts/install-launch-agent.sh"]) {
      if (existsSync(join(INSTALL_DIR, rel))) pass(`/usr/local/portier/${rel}`);
      else fail(`missing bundled LaunchAgent file: /usr/local/portier/${rel}`);
    }

    // ── No user config inside the install dir ──
    console.log("\nConfig/data boundary:");
    if (listFilesRecursive(INSTALL_DIR).some((f) => f.endsWith("rules.json")))
      fail("rules.json must NOT be inside /usr/local/portier");
    else pass("no rules.json inside /usr/local/portier");

    // ── Installed CLI version ──
    console.log("\nInstalled CLI version:");
    const cli = join(INSTALL_DIR, "portier");
    const v = sh(cli, ["version"]);
    const out = (v.stdout || "").trim();
    if ((v.status ?? 1) === 0 && out.includes(version)) pass(`portier reports ${version} ("${out}")`);
    else fail(`version mismatch: status=${v.status} out="${out}" expected to include ${version}`);

    // ── LaunchAgent must NOT be loaded/started by install ──
    console.log("\nLaunchAgent state (must not be loaded):");
    if (launchAgentLoaded()) fail("a portier LaunchAgent is loaded after install — package must not load it");
    else pass("no portier LaunchAgent loaded after install");

    // ── User data preserved after install ──
    console.log("\nUser-data preservation (after install):");
    if (createdUserData) {
      if (readFileSync(USER_DATA_FILE, "utf8").includes(SENTINEL)) pass("user-data sentinel intact after install");
      else fail("user-data sentinel changed by install");
    } else {
      console.log("  - not seeded (pre-existing user data left untouched) [skip]");
    }

    // ── Uninstall: pkgbuild has no uninstaller — documented manual cleanup ──
    console.log("\nUninstall (remove install dir + pkgutil --forget):");
    assertExact(INSTALL_DIR, "/usr/local/portier");
    const rm = sudo(["rm", "-rf", INSTALL_DIR], "remove install dir");
    sudo(["pkgutil", "--forget", PKG_IDENTIFIER], "pkgutil --forget");
    if (rm && (rm.status ?? 1) === 0 && !existsSync(INSTALL_DIR)) pass("install dir removed");
    else fail("install dir not removed cleanly");

    console.log("\nPost-uninstall assertions:");
    if (launchAgentLoaded()) fail("a portier LaunchAgent is loaded after uninstall");
    else pass("no portier LaunchAgent loaded after uninstall");

    if (createdUserData) {
      if (existsSync(USER_DATA_FILE) && readFileSync(USER_DATA_FILE, "utf8").includes(SENTINEL))
        pass("user-data sentinel preserved after uninstall");
      else fail("user-data sentinel removed/changed by uninstall");
    }
  } finally {
    // Cleanup ONLY the sentinel we created.
    if (createdUserData && !keep) {
      try { sh("rm", ["-f", USER_DATA_FILE]); } catch { /* best effort */ }
    }
  }

  console.log(`\n[pkg-smoke] ${passed} passed, ${failed} failed.`);
  if (failed > 0) {
    console.error("[pkg-smoke] .pkg install/uninstall smoke FAILED.\n");
    process.exit(1);
  }
  console.log("[pkg-smoke] .pkg install/uninstall smoke passed.\n");
}

main().catch((err) => {
  console.error("[pkg-smoke] Unexpected error:", err);
  process.exit(1);
});
