#!/usr/bin/env node
/* global console, process */
/**
 * Windows MSI install/layout smoke for the Portier WiX MSI.
 *
 * Proves the MSI installs the expected Portier file layout, keeps user config/data
 * outside the install dir, and (when elevated) installs + uninstalls cleanly via
 * msiexec — without service custom actions (the current MSI is a file-install spike).
 *
 * Two modes, auto-selected by privilege:
 *   - Elevated (or --full-install): real per-machine install to a temp INSTALLFOLDER
 *     via `msiexec /i ... /qn`, layout assertions, then `msiexec /x ... /qn`
 *     uninstall, asserting the install dir is removed. This is the full install smoke.
 *   - Non-elevated (default here): `msiexec /a ... /qn TARGETDIR=<temp>`
 *     (administrative-install extraction) lays out the exact payload WITHOUT admin
 *     and without touching the system. Validates the installed layout. The
 *     register/uninstall half is reported as an honest skip (needs admin).
 *
 * Windows-only. On other platforms it exits 0 with a skip notice so cross-platform
 * release validation is unaffected. Never required by the normal release matrix.
 *
 * Usage:
 *   node scripts/validate-msi-install.js [--msi <path>] [--data-dir <dir>]
 *                                        [--full-install] [--keep-temp]
 */

import { existsSync, mkdirSync, writeFileSync, readFileSync, rmSync, readdirSync, statSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { tmpdir } from "node:os";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const isWindows = process.platform === "win32";

// ── Arguments ─────────────────────────────────────────────────────────────────

const rawArgs = process.argv.slice(2);
const hasFlag = (f) => rawArgs.includes(f);
const flagValue = (f) => {
  const i = rawArgs.indexOf(f);
  return i >= 0 && i + 1 < rawArgs.length ? rawArgs[i + 1] : null;
};
const msiArg = flagValue("--msi");
const dataDirArg = flagValue("--data-dir");
const forceFullInstall = hasFlag("--full-install");
const keepTemp = hasFlag("--keep-temp");

// ── Pure helpers (exported for tests) ─────────────────────────────────────────

function readPackageVersion() {
  return JSON.parse(readFileSync(join(repoRoot, "package.json"), "utf8")).version;
}

// msiName is the MSI filename for a version (matches build-msi.ps1 output).
export function msiName(version) {
  return `Portier-${version}.msi`;
}

// openApiMajorMinor mirrors the OPENAPI_DOC_VERSION convention (1.18.0 -> "1.18").
export function openApiMajorMinor(version) {
  const p = version.split(".");
  return p.length >= 2 ? `${p[0]}.${p[1]}` : version;
}

// Expected file path suffixes (forward-slash) the installed layout must contain.
export const EXPECTED_INSTALLED_SUFFIXES = [
  "portier.exe",
  "service.exe",
  "server.js",
  "readme.txt",
  "web/index.html",
  "api/openapi.json",
  "service/install-service.ps1",
  "service/uninstall-service.ps1",
];

// ── Result tracking ───────────────────────────────────────────────────────────

let passed = 0;
let failed = 0;
let skipped = 0;
const pass = (m) => { console.log(`  ✓ ${m}`); passed++; };
const fail = (m) => { console.error(`  ✗ ${m}`); failed++; };
const skip = (m) => { console.log(`  - ${m} [skip]`); skipped++; };

// ── Filesystem helpers ────────────────────────────────────────────────────────

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

const norm = (p) => p.replace(/\\/g, "/").toLowerCase();

// ── msiexec ───────────────────────────────────────────────────────────────────

function isElevated() {
  const r = spawnSync("whoami", ["/groups"], { encoding: "utf8" });
  return r.status === 0 && /S-1-16-12288|S-1-16-16384/.test(r.stdout || "");
}

function runMsiexec(args, label) {
  const r = spawnSync("msiexec", args, { encoding: "utf8" });
  if (r.error) {
    fail(`${label}: msiexec could not be launched (${r.error.message})`);
    return null;
  }
  return r.status ?? 1;
}

// ── Layout assertions (shared by both modes) ──────────────────────────────────

function assertInstalledLayout(rootDir, version, dataDir) {
  const files = listFilesRecursive(rootDir).map(norm);

  console.log("\nInstalled layout:");
  for (const suffix of EXPECTED_INSTALLED_SUFFIXES) {
    if (files.some((f) => f.endsWith(suffix))) pass(suffix);
    else fail(`missing from installed layout: ${suffix}`);
  }

  console.log("\nConfig/data boundary:");
  if (files.some((f) => f.endsWith("rules.json"))) fail("rules.json must NOT be inside the install dir");
  else pass("no rules.json inside the install dir");

  // OpenAPI content + version.
  console.log("\nOpenAPI artifact:");
  const apiFull = listFilesRecursive(rootDir).find((f) => norm(f).endsWith("api/openapi.json"));
  if (!apiFull) {
    fail("api/openapi.json not found to validate");
  } else {
    let doc;
    try { doc = JSON.parse(readFileSync(apiFull, "utf8").replace(/^\uFEFF/, "")); } catch { doc = null; }
    if (!doc) {
      fail("api/openapi.json is not valid JSON");
    } else {
      if (typeof doc.openapi === "string" && doc.openapi) pass(`openapi = ${doc.openapi}`);
      else fail("api/openapi.json missing openapi field");
      const expected = openApiMajorMinor(version);
      if (doc.info && doc.info.version === expected) pass(`info.version = ${doc.info.version} (matches package ${version})`);
      else fail(`info.version = ${JSON.stringify(doc.info && doc.info.version)}, expected ${expected}`);
    }
  }

  // CLI version from the installed binary.
  console.log("\nInstalled CLI version:");
  const cliFull = listFilesRecursive(rootDir).find((f) => norm(f).endsWith("portier.exe"));
  if (!cliFull) {
    fail("portier.exe not found to check version");
  } else {
    const r = spawnSync(cliFull, ["version"], { encoding: "utf8" });
    const out = (r.stdout || "").trim();
    if ((r.status ?? 1) === 0 && out.includes(version)) pass(`portier.exe reports ${version} ("${out}")`);
    else fail(`portier.exe version mismatch: status=${r.status} out="${out}" expected to include ${version}`);
  }

  // External data dir is untouched by install.
  if (dataDir) {
    console.log("\nExternal data preservation:");
    assertExternalDataIntact(dataDir);
  }
}

function seedExternalData(dataDir) {
  mkdirSync(dataDir, { recursive: true });
  const files = {
    "rules.json": "[]",
    "rules.json.backup-sentinel": "backup-sentinel",
    "rules.json.quarantine-sentinel": "quarantine-sentinel",
  };
  for (const [name, content] of Object.entries(files)) writeFileSync(join(dataDir, name), content);
  return files;
}

function assertExternalDataIntact(dataDir) {
  const expected = {
    "rules.json": "[]",
    "rules.json.backup-sentinel": "backup-sentinel",
    "rules.json.quarantine-sentinel": "quarantine-sentinel",
  };
  for (const [name, content] of Object.entries(expected)) {
    const p = join(dataDir, name);
    if (existsSync(p) && readFileSync(p, "utf8") === content) pass(`external ${name} preserved (untouched by MSI)`);
    else fail(`external ${name} changed/removed by MSI`);
  }
}

// ── Main ──────────────────────────────────────────────────────────────────────

async function main() {
  if (!isWindows) {
    console.log("[validate-msi-install] Windows-only; skipping on " + process.platform + ".");
    return;
  }

  const version = readPackageVersion();
  const releasesDir = join(repoRoot, "build", "releases", "windows");
  const msiPath = msiArg ? resolve(msiArg) : join(releasesDir, msiName(version));

  console.log(`[validate-msi-install] MSI : ${msiPath}`);
  if (!existsSync(msiPath)) {
    fail(`MSI not found: ${msiPath}`);
    console.error("[validate-msi-install] Hint: run `npm run build:release:current` (requires WiX 7).\n");
    process.exit(1);
  }
  pass(`MSI present (${(statSync(msiPath).size / 1024 / 1024).toFixed(1)} MB)`);

  const elevated = isElevated();
  const fullInstall = forceFullInstall || elevated;
  console.log(`[validate-msi-install] Mode: ${fullInstall ? "full install (/i + /x)" : "administrative-install extraction (/a)"}${elevated ? " [elevated]" : " [non-elevated]"}`);

  const tempRoot = join(tmpdir(), `portier-msi-${process.pid}-${Date.now()}`);
  const installRoot = join(tempRoot, "install");
  const dataDir = dataDirArg ? resolve(dataDirArg) : join(tempRoot, "data");
  const logDir = join(tempRoot, "logs");
  mkdirSync(logDir, { recursive: true });

  try {
    // Seed an external data dir so we can prove the MSI never touches it.
    seedExternalData(dataDir);

    if (fullInstall) {
      mkdirSync(installRoot, { recursive: true });
      console.log("\nInstall (msiexec /i):");
      const code = runMsiexec(
        ["/i", msiPath, `INSTALLFOLDER=${installRoot}`, "/qn", "/l*v", join(logDir, "install.log")],
        "install"
      );
      if (code === null) return;
      if (code !== 0) {
        fail(`msiexec /i exited ${code} (see ${join(logDir, "install.log")})`);
        return;
      }
      pass("MSI installed silently (msiexec /i)");

      assertInstalledLayout(installRoot, version, dataDir);

      console.log("\nUninstall (msiexec /x):");
      const xcode = runMsiexec(["/x", msiPath, "/qn", "/l*v", join(logDir, "uninstall.log")], "uninstall");
      if (xcode === 0) {
        pass("MSI uninstalled silently (msiexec /x)");
        const leftover = listFilesRecursive(installRoot);
        if (leftover.length === 0) pass("install dir removed/empty after uninstall");
        else fail(`install dir not clean after uninstall: ${leftover.length} file(s) left`);
      } else {
        fail(`msiexec /x exited ${xcode} (see ${join(logDir, "uninstall.log")})`);
      }
    } else {
      // Non-admin: administrative-install extraction validates the install LAYOUT
      // without registering the product. The register/uninstall half needs admin.
      console.log("\nExtract (msiexec /a):");
      const code = runMsiexec(
        ["/a", msiPath, "/qn", `TARGETDIR=${installRoot}`, "/l*v", join(logDir, "extract.log")],
        "extract"
      );
      if (code === null) return;
      if (code !== 0) {
        fail(`msiexec /a exited ${code} (see ${join(logDir, "extract.log")})`);
        return;
      }
      pass("MSI payload extracted via administrative install (msiexec /a)");

      assertInstalledLayout(installRoot, version, dataDir);

      skip("full per-machine install + uninstall (msiexec /i, /x) — requires admin; re-run elevated or with --full-install");
    }
  } finally {
    if (keepTemp) {
      console.log(`\n[validate-msi-install] --keep-temp: left ${tempRoot}`);
    } else {
      try { rmSync(tempRoot, { recursive: true, force: true }); } catch { /* best effort */ }
    }
  }

  console.log(`\n[validate-msi-install] ${passed} passed, ${skipped} skipped, ${failed} failed.`);
  if (failed > 0) {
    console.error("[validate-msi-install] MSI install smoke FAILED.\n");
    process.exit(1);
  }
  console.log("[validate-msi-install] MSI install smoke passed.\n");
}

const invokedDirectly =
  process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (invokedDirectly) {
  main().catch((err) => {
    console.error("[validate-msi-install] Unexpected error:", err);
    process.exit(1);
  });
}
