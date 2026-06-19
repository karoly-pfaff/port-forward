#!/usr/bin/env node
/* global console, process */
/**
 * Windows MSI install/layout smoke for the Portier WiX MSI.
 *
 * Proves the MSI installs the expected Portier file layout, keeps user config/data
 * outside the install dir, and (when elevated) installs + uninstalls cleanly via
 * msiexec — without service custom actions (the current MSI is a file-install spike).
 *
 * Modes (npm run validate:msi:install = extract; validate:msi:install:full = full):
 *   - extract (--extract): `msiexec /a ... /qn TARGETDIR=<temp>` administrative-install
 *     extraction lays out the exact payload WITHOUT admin and validates the installed
 *     layout. The register/uninstall half is an honest skip (needs admin).
 *   - full (--full / --full-install): real per-machine install to a temp INSTALLFOLDER via
 *     `msiexec /i ... /qn`, layout + version + config assertions, asserts NO Windows
 *     service / scheduled task is created and %ProgramData%\Portier\rules.json is
 *     preserved, then `msiexec /x ... /qn` uninstall asserting the install dir is removed
 *     and config still preserved. Needs an elevated shell — honestly SKIPS (exit 0) when
 *     not elevated. (No service custom actions exist; the MSI is file-install only.)
 *   - auto (no mode flag): full when elevated, else extract (back-compat).
 *
 * Windows-only. On other platforms it exits 0 with a skip notice so cross-platform
 * release validation is unaffected.
 *
 * Usage:
 *   node scripts/validate-install-msi.js [--extract | --full] [--msi <path>]
 *                                        [--data-dir <dir>] [--keep-temp]
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
const keepTemp = hasFlag("--keep-temp");

// Mode: "extract" forces the non-elevated administrative-install (/a) layout smoke;
// "full" forces the elevated per-machine install/uninstall (/i + /x) smoke (and honestly
// skips when not elevated). With no mode flag the behavior is auto (full when elevated,
// else extract) for back-compat. --full-install is kept as an alias for --full.
const mode =
  hasFlag("--extract") ? "extract"
  : (hasFlag("--full") || hasFlag("--full-install")) ? "full"
  : "auto";

// ── Pure helpers (exported for tests) ─────────────────────────────────────────

function readPackageVersion() {
  return JSON.parse(readFileSync(join(repoRoot, "package.json"), "utf8")).version;
}

// msiName is the MSI filename for a version (matches build-release.ps1 output).
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

// ── Service / scheduled-task non-creation (full install only) ──────────────────

// assertNoService passes when no Windows service of `name` exists. sc.exe returns 0 when
// the service exists and 1060 ("does not exist") otherwise.
function assertNoService(name) {
  const r = spawnSync("sc.exe", ["query", name], { encoding: "utf8" });
  if ((r.status ?? 1) === 0) fail(`a Windows service named "${name}" exists (the MSI must not create one)`);
  else pass(`no Windows service "${name}" (MSI did not create/start a service)`);
}

// assertNoTask passes when no scheduled task of `name` exists.
function assertNoTask(name) {
  const r = spawnSync("schtasks", ["/Query", "/TN", name], { encoding: "utf8" });
  if ((r.status ?? 1) === 0) fail(`a scheduled task "${name}" exists (the MSI must not create one)`);
  else pass(`no scheduled task "${name}" (MSI did not create one)`);
}

// ── ProgramData config preservation (full install only) ────────────────────────
// The canonical Windows config lives at %ProgramData%\Portier\rules.json. The MSI is a
// file-install and must never create/overwrite it. We seed a sentinel there (only if the
// dir does not already exist, so a real install's config is never touched) and assert it
// survives install + uninstall byte-for-byte.

const PROGRAM_DATA = process.env.ProgramData || "C:\\ProgramData";
const PORTIER_PROGRAM_DATA = join(PROGRAM_DATA, "Portier");
const PROGRAM_DATA_CONFIG = join(PORTIER_PROGRAM_DATA, "rules.json");
const PROGRAM_DATA_SENTINEL = "portier-msi-smoke-sentinel";

function seedProgramData() {
  if (existsSync(PROGRAM_DATA_CONFIG)) {
    // Real config already present — never overwrite it; preserve its current content.
    return { created: false, content: readFileSync(PROGRAM_DATA_CONFIG, "utf8") };
  }
  mkdirSync(PORTIER_PROGRAM_DATA, { recursive: true });
  writeFileSync(PROGRAM_DATA_CONFIG, PROGRAM_DATA_SENTINEL);
  return { created: true, content: PROGRAM_DATA_SENTINEL };
}

function assertProgramDataIntact(expectedContent, when) {
  if (existsSync(PROGRAM_DATA_CONFIG) && readFileSync(PROGRAM_DATA_CONFIG, "utf8") === expectedContent)
    pass(`%ProgramData%\\Portier\\rules.json preserved ${when}`);
  else fail(`%ProgramData%\\Portier\\rules.json changed/removed by MSI ${when}`);
}

function cleanupProgramData(seed) {
  // Only remove what we created, and only the exact Portier ProgramData path.
  if (!seed.created) return;
  if (!PORTIER_PROGRAM_DATA.replace(/\\/g, "/").toLowerCase().endsWith("/portier")) return;
  try { rmSync(PORTIER_PROGRAM_DATA, { recursive: true, force: true }); } catch { /* best effort */ }
}

// ── Main ──────────────────────────────────────────────────────────────────────

async function main() {
  if (!isWindows) {
    console.log("[validate-install-msi] Windows-only; skipping on " + process.platform + ".");
    return;
  }

  const version = readPackageVersion();
  const releasesDir = join(repoRoot, "build", "releases", "windows");
  const msiPath = msiArg ? resolve(msiArg) : join(releasesDir, msiName(version));

  console.log(`[validate-install-msi] MSI : ${msiPath}`);
  if (!existsSync(msiPath)) {
    fail(`MSI not found: ${msiPath}`);
    console.error("[validate-install-msi] Hint: run `npm run build:release:current` (requires WiX 7).\n");
    process.exit(1);
  }
  pass(`MSI present (${(statSync(msiPath).size / 1024 / 1024).toFixed(1)} MB)`);

  const elevated = isElevated();
  const effectiveMode = mode === "auto" ? (elevated ? "full" : "extract") : mode;
  console.log(`[validate-install-msi] Mode: ${effectiveMode} (requested ${mode}) ${elevated ? "[elevated]" : "[non-elevated]"}`);

  // The full /i + /x smoke needs an elevated shell. Skip honestly when not elevated rather
  // than fail or fake success — the Release Windows workflow runs it elevated.
  if (effectiveMode === "full" && !elevated) {
    skip("full per-machine MSI install/uninstall (/i + /x) requires an elevated shell — run in an elevated terminal or rely on the Release Windows workflow");
    console.log(`\n[validate-install-msi] ${passed} passed, ${skipped} skipped, ${failed} failed.`);
    console.log("[validate-install-msi] MSI install smoke skipped (needs elevation).\n");
    return;
  }

  const tempRoot = join(tmpdir(), `portier-msi-${process.pid}-${Date.now()}`);
  const installRoot = join(tempRoot, "install");
  const dataDir = dataDirArg ? resolve(dataDirArg) : join(tempRoot, "data");
  const logDir = join(tempRoot, "logs");
  mkdirSync(logDir, { recursive: true });

  let programDataSeed = null;
  try {
    // Seed an external data dir so we can prove the MSI never touches it.
    seedExternalData(dataDir);

    if (effectiveMode === "full") {
      // Seed the canonical Windows config so we can prove the MSI never touches it.
      programDataSeed = seedProgramData();
      console.log(programDataSeed.created
        ? `\nSeeded ProgramData sentinel at ${PROGRAM_DATA_CONFIG}`
        : `\nProgramData config already present — preserving it (not seeding)`);

      mkdirSync(installRoot, { recursive: true });
      console.log("\nInstall (msiexec /i):");
      const code = runMsiexec(
        ["/i", msiPath, `INSTALLFOLDER=${installRoot}`, "/qn", "/norestart", "/l*v", join(logDir, "install.log")],
        "install"
      );
      if (code === null) return;
      if (code !== 0) {
        fail(`msiexec /i exited ${code} (see ${join(logDir, "install.log")})`);
        return;
      }
      pass("MSI installed silently (msiexec /i)");

      assertInstalledLayout(installRoot, version, dataDir);

      console.log("\nConfig preservation (after install):");
      assertProgramDataIntact(programDataSeed.content, "after install");

      console.log("\nService / scheduled task (must NOT be created):");
      assertNoService("Portier");
      assertNoTask("\\Portier");

      console.log("\nUninstall (msiexec /x):");
      const xcode = runMsiexec(["/x", msiPath, "/qn", "/norestart", "/l*v", join(logDir, "uninstall.log")], "uninstall");
      if (xcode === 0) {
        pass("MSI uninstalled silently (msiexec /x)");
        const leftover = listFilesRecursive(installRoot);
        if (leftover.length === 0) pass("install dir removed/empty after uninstall");
        else fail(`install dir not clean after uninstall: ${leftover.length} file(s) left`);
      } else {
        fail(`msiexec /x exited ${xcode} (see ${join(logDir, "uninstall.log")})`);
      }

      console.log("\nPost-uninstall assertions:");
      assertProgramDataIntact(programDataSeed.content, "after uninstall");
      assertNoService("Portier");
      assertNoTask("\\Portier");
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

      skip("full per-machine install + uninstall (msiexec /i, /x) — extraction mode; run `npm run validate:msi:install:full` elevated");
    }
  } finally {
    if (programDataSeed) cleanupProgramData(programDataSeed);
    if (keepTemp) {
      console.log(`\n[validate-install-msi] --keep-temp: left ${tempRoot}`);
    } else {
      try { rmSync(tempRoot, { recursive: true, force: true }); } catch { /* best effort */ }
    }
  }

  console.log(`\n[validate-install-msi] ${passed} passed, ${skipped} skipped, ${failed} failed.`);
  if (failed > 0) {
    console.error("[validate-install-msi] MSI install smoke FAILED.\n");
    process.exit(1);
  }
  console.log("[validate-install-msi] MSI install smoke passed.\n");
}

const invokedDirectly =
  process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (invokedDirectly) {
  main().catch((err) => {
    console.error("[validate-install-msi] Unexpected error:", err);
    process.exit(1);
  });
}
