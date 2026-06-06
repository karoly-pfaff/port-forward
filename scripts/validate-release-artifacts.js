#!/usr/bin/env node
/* global console, process */
/**
 * Validates Portier release artifacts under build/releases/<platform>/.
 *
 * Checks that the portable archive exists, has a reasonable size, contains
 * the required runtime files, does not contain forbidden files, and that
 * readme.txt mentions the management URL and external config.
 *
 * Usage:
 *   node scripts/validate-release-artifacts.js [options]
 *
 * Options:
 *   --version <v>                   Version string (default: reads from package.json).
 *   --platform current|windows|macos|linux   Target platform (default: current).
 *   --portable-only                 Only check portable archive; skip installer check.
 *   --installer-required            Fail if the platform installer artifact is missing.
 */

import { existsSync, readFileSync, statSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");

// ── Parse arguments ───────────────────────────────────────────────────────────

const rawArgs = process.argv.slice(2);
const hasFlag = (f) => rawArgs.includes(f);
const flagValue = (f) => {
  const i = rawArgs.indexOf(f);
  return i >= 0 && i + 1 < rawArgs.length ? rawArgs[i + 1] : null;
};

const versionArg = flagValue("--version");
const platformArg = flagValue("--platform") || "current";
const portableOnly = hasFlag("--portable-only");
const installerRequired = hasFlag("--installer-required");

// ── Resolve version ───────────────────────────────────────────────────────────

function readVersion() {
  const pkgPath = join(repoRoot, "package.json");
  if (!existsSync(pkgPath)) {
    console.error("[validate-release] package.json not found.");
    process.exit(1);
  }
  const pkg = JSON.parse(readFileSync(pkgPath, "utf8"));
  if (!pkg.version) {
    console.error("[validate-release] version field missing in package.json.");
    process.exit(1);
  }
  return pkg.version;
}

const version = versionArg || readVersion();

// ── Resolve platform ──────────────────────────────────────────────────────────

function resolvePlatformLabel(arg) {
  const p = arg === "current" ? process.platform : arg;
  if (p === "win32"  || p === "windows") return "windows";
  if (p === "darwin" || p === "macos")   return "macos";
  if (p === "linux")                     return "linux";
  return null;
}

const platformLabel = resolvePlatformLabel(platformArg);
if (!platformLabel) {
  console.error(`[validate-release] Unknown platform: ${platformArg}`);
  console.error("  Supported values: current, windows, macos, linux");
  process.exit(1);
}

// ── Archive and installer name patterns ──────────────────────────────────────

function getArchiveName(platform, ver) {
  if (platform === "windows") return `portier-${ver}-windows-portable.zip`;
  if (platform === "macos")   return `portier-portable-macos-${ver}.tar.gz`;
  return                              `portier-${ver}-linux.tar.gz`;
}

function getInstallerName(platform, ver) {
  if (platform === "windows") return `Portier-Setup-${ver}.exe`;
  return null; // macOS .pkg and Linux .deb/.rpm not in v1.1
}

// ── Result tracking ───────────────────────────────────────────────────────────

let passed = 0;
let failed = 0;
let warned = 0;

function pass(msg) { console.log(`  ✓ ${msg}`); passed++; }
function fail(msg) { console.error(`  ✗ ${msg}`); failed++; }
function warn(msg) { console.warn(`  ! ${msg}`); warned++; }

// ── Archive content listing ───────────────────────────────────────────────────

function listTarGzContents(archivePath) {
  const r = spawnSync("tar", ["-tzf", archivePath], { encoding: "utf8" });
  if (r.status !== 0 || !r.stdout) return null;
  return r.stdout.split("\n").map((s) => s.trim()).filter(Boolean);
}

function listZipContents(archivePath) {
  const escaped = archivePath.replace(/'/g, "''");
  const psCmd = [
    `Add-Type -AssemblyName System.IO.Compression.FileSystem`,
    `$z = [System.IO.Compression.ZipFile]::OpenRead('${escaped}')`,
    `$z.Entries.FullName`,
    `$z.Dispose()`,
  ].join("; ");
  const r = spawnSync(
    "powershell",
    ["-NoProfile", "-NonInteractive", "-Command", psCmd],
    { encoding: "utf8" }
  );
  if (r.status !== 0 || !r.stdout) return null;
  return r.stdout.split("\n").map((s) => s.trim()).filter(Boolean);
}

function listArchiveContents(archivePath) {
  if (archivePath.endsWith(".zip")) return listZipContents(archivePath);
  return listTarGzContents(archivePath);
}

function normalizeEntries(rawEntries) {
  // Normalize to forward slashes, strip leading ./ and trailing / for consistent comparisons
  return new Set(
    rawEntries
      .map((e) => e.replace(/\\/g, "/").replace(/^\.\//, "").replace(/\/$/, ""))
      .filter(Boolean)
  );
}

// ── Content checks ────────────────────────────────────────────────────────────

function checkRequired(entries, path, label) {
  if (entries.has(path)) {
    pass(label);
  } else {
    fail(`Missing: ${label} (expected path: ${path})`);
  }
}

function checkHasPrefix(entries, prefix, label) {
  // entries are normalized to forward slashes
  const found = [...entries].some(
    (e) => e === prefix || e.startsWith(prefix + "/")
  );
  if (found) {
    pass(label);
  } else {
    fail(`Missing: ${label} (expected entries under: ${prefix}/)`);
  }
}

function checkAbsent(entries, name, label) {
  // entries are normalized to forward slashes
  const present = [...entries].some(
    (e) => e === name || e.startsWith(name + "/")
  );
  if (present) {
    fail(`Archive must not contain: ${label}`);
  } else {
    pass(`Absent (correct): ${label}`);
  }
}

// ── readme.txt content extraction ─────────────────────────────────────────────

function extractReadmeFromTarGz(archivePath, rawEntries) {
  const entry = rawEntries.find(
    (e) => e.replace(/^\.\//, "").replace(/\/$/, "") === "readme.txt"
  );
  if (!entry) return null;
  const r = spawnSync("tar", ["-xOzf", archivePath, entry], { encoding: "utf8" });
  return r.status === 0 && r.stdout ? r.stdout : null;
}

function extractReadmeFromZip(archivePath) {
  const escaped = archivePath.replace(/'/g, "''");
  const psCmd = [
    `Add-Type -AssemblyName System.IO.Compression.FileSystem`,
    `$z = [System.IO.Compression.ZipFile]::OpenRead('${escaped}')`,
    `$e = $z.Entries | Where-Object { $_.Name -eq 'readme.txt' } | Select-Object -First 1`,
    `if ($e) { $r = New-Object System.IO.StreamReader($e.Open()); $r.ReadToEnd(); $r.Dispose() }`,
    `$z.Dispose()`,
  ].join("; ");
  const r = spawnSync(
    "powershell",
    ["-NoProfile", "-NonInteractive", "-Command", psCmd],
    { encoding: "utf8" }
  );
  return r.status === 0 && r.stdout ? r.stdout : null;
}

function checkReadmeContent(archivePath, rawEntries) {
  const content = archivePath.endsWith(".zip")
    ? extractReadmeFromZip(archivePath)
    : extractReadmeFromTarGz(archivePath, rawEntries);

  if (!content) {
    warn("Could not extract readme.txt content for validation.");
    return;
  }

  if (content.includes("127.0.0.1:47831")) {
    pass("readme.txt mentions management URL (127.0.0.1:47831)");
  } else {
    fail("readme.txt does not mention management URL (127.0.0.1:47831)");
  }
  if (/rules\.json|--config/i.test(content)) {
    pass("readme.txt mentions --config / rules.json");
  } else {
    fail("readme.txt does not mention --config or rules.json");
  }
  if (/external|not bundled/i.test(content)) {
    pass("readme.txt states config is external");
  } else {
    warn("readme.txt does not explicitly state config is external");
  }
  if (/portable|does not install/i.test(content)) {
    pass("readme.txt notes portable archive does not install OS services");
  } else {
    warn("readme.txt does not mention that portable archive does not install OS services");
  }
}

// ── Main ──────────────────────────────────────────────────────────────────────

const releasesDir = join(repoRoot, "build", "releases", platformLabel);
const archiveName = getArchiveName(platformLabel, version);
const archivePath = join(releasesDir, archiveName);
const installerName = getInstallerName(platformLabel, version);
const installerPath = installerName ? join(releasesDir, installerName) : null;

console.log(`[validate-release] Platform : ${platformLabel}`);
console.log(`[validate-release] Version  : ${version}`);
console.log(`[validate-release] Dir      : ${releasesDir}`);
console.log("");

// Release directory
if (!existsSync(releasesDir)) {
  fail(`build/releases/${platformLabel}/ not found — run: npm run package:release:current`);
  console.error("[validate-release] FAILED: release directory missing.\n");
  process.exit(1);
}
pass(`build/releases/${platformLabel}/ exists`);

// Portable archive
console.log("\nPortable archive:");
if (!existsSync(archivePath)) {
  fail(`Archive not found: ${archiveName}`);
  console.error("[validate-release] FAILED: archive missing.\n");
  process.exit(1);
}
const archiveStat = statSync(archivePath);
if (archiveStat.size === 0) {
  fail(`Archive is empty: ${archiveName}`);
} else {
  pass(`${archiveName} (${(archiveStat.size / 1024).toFixed(1)} KB)`);
}

// Archive contents
console.log("\nArchive contents:");
const rawEntries = listArchiveContents(archivePath);
if (!rawEntries) {
  warn(`Could not list archive contents — skipping content checks.`);
} else {
  const entries = normalizeEntries(rawEntries);
  const serviceBin = platformLabel === "windows" ? "service.exe" : "service";

  checkRequired(entries, serviceBin, `${serviceBin} (native Go binary)`);
  checkRequired(entries, "server.js", "server.js (Node/TypeScript fallback)");
  checkRequired(entries, "readme.txt", "readme.txt");
  checkRequired(entries, "web/index.html", "web/index.html");
  checkHasPrefix(entries, "web/assets", "web/assets/ (at least one asset file)");

  console.log("\nForbidden content:");
  checkAbsent(entries, "node_modules", "node_modules/");
  checkAbsent(entries, "rules.json", "rules.json");
  checkAbsent(entries, ".env", ".env");
  checkAbsent(entries, "sources", "sources/");
  checkAbsent(entries, "client", "client/ directory");
  // Note: 'server' directory check — entries are normalized to forward slashes;
  // must not match 'server.js' (which doesn't start with "server/")
  const hasServerDir = [...entries].some(
    (e) => e === "server" || e.startsWith("server/")
  );
  if (hasServerDir) {
    fail("Archive must not contain: server/ directory");
  } else {
    pass("Absent (correct): server/ directory");
  }

  console.log("\nreadme.txt content:");
  checkReadmeContent(archivePath, rawEntries);
}

// Installer artifact (optional unless --installer-required)
if (!portableOnly && installerName) {
  console.log("\nInstaller artifact:");
  if (!existsSync(installerPath)) {
    if (installerRequired) {
      fail(`Installer not found: ${installerName} (--installer-required)`);
    } else {
      warn(`Installer not found: ${installerName} — build with: npm run installer:${platformLabel}`);
    }
  } else {
    const iStat = statSync(installerPath);
    pass(`${installerName} (${(iStat.size / 1024 / 1024).toFixed(1)} MB)`);
  }
}

// Summary
console.log(
  `\n[validate-release] ${passed} passed, ${warned} warned, ${failed} failed.`
);
if (failed > 0) {
  console.error("[validate-release] FAILED.\n");
  process.exit(1);
}
console.log("[validate-release] Release artifacts validated.\n");
