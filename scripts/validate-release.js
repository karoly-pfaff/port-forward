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
 *   node scripts/validate-release.js [options]
 *
 * Options:
 *   --version <v>                   Version string (default: reads from package.json).
 *   --platform current|windows|macos|linux   Target platform (default: current).
 *   --portable-only                 Only check the portable archive; skip the installer check.
 *   --checksums-only                Only verify checksums.sha256 (skip archive/installer checks).
 *
 * The canonical installer (the WiX MSI on Windows) is required unless --portable-only.
 */

import { existsSync, readFileSync, statSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import AdmZip from "adm-zip";
import {
  CHECKSUMS_NAME,
  parseSha256Sums,
  sha256File,
  findReleaseArtifacts,
} from "./library/checksums.js";

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
const checksumsOnly = hasFlag("--checksums-only");

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

// The packaged OpenAPI doc version follows the major.minor of the package version
// (e.g. package 1.18.0 -> OpenAPI "1.18"). Mirrors OPENAPI_DOC_VERSION in
// server/sources/openapi/openapi.ts.
function openApiVersionForPackage(ver) {
  const parts = ver.split(".");
  return parts.length >= 2 ? `${parts[0]}.${parts[1]}` : ver;
}
const expectedOpenApi = openApiVersionForPackage(version);

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

// Host architecture as a Go GOARCH label (artifact names carry the arch).
function hostGoarch() {
  return process.arch === "arm64" ? "arm64" : "amd64";
}

// The portable archive deep-checked here is the HOST-arch one (the artifact that can be
// runtime-smoked natively). Every produced arch portable is still covered by the
// checksums.sha256 verification below, and structurally by validate-portable.js.
function getArchiveName(platform, ver) {
  const ext = platform === "windows" ? "zip" : "tar.gz";
  return `portier-${ver}-${platform}-${hostGoarch()}.${ext}`;
}

function getInstallerName(platform, ver) {
  // The WiX MSI is the canonical Windows installer (Inno Setup retired).
  if (platform === "windows") return `Portier-${ver}.msi`;
  // The macOS native .pkg installer track is in progress (built on macOS).
  if (platform === "macos") return `Portier-${ver}.pkg`;
  // The Linux native .deb is built by dpkg-deb on Linux (.rpm is a later track).
  if (platform === "linux") return `portier_${ver}_amd64.deb`;
  return null;
}

// Whether a missing installer is a hard failure. The Windows MSI is the canonical
// installer (required). The macOS .pkg track is still being introduced, so a
// missing .pkg is reported but not fatal yet.
function installerIsRequired(platform) {
  return platform === "windows";
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
  try {
    return new AdmZip(archivePath).getEntries().map((e) => e.entryName);
  } catch {
    return null;
  }
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

// ── Generic archive entry extraction (by normalized relative path) ────────────

function normalizeRel(entry) {
  return entry.replace(/\\/g, "/").replace(/^\.\//, "").replace(/\/$/, "");
}

function extractTarGzEntry(archivePath, rawEntries, relPath) {
  const entry = rawEntries.find((e) => normalizeRel(e) === relPath);
  if (!entry) return null;
  const r = spawnSync("tar", ["-xOzf", archivePath, entry], { encoding: "utf8" });
  return r.status === 0 && r.stdout ? r.stdout : null;
}

function extractZipEntry(archivePath, relPath) {
  try {
    const zip = new AdmZip(archivePath);
    const entry = zip
      .getEntries()
      .find((e) => normalizeRel(e.entryName) === relPath);
    return entry ? zip.readAsText(entry) : null;
  } catch {
    return null;
  }
}

function extractArchiveEntry(archivePath, rawEntries, relPath) {
  return archivePath.endsWith(".zip")
    ? extractZipEntry(archivePath, relPath)
    : extractTarGzEntry(archivePath, rawEntries, relPath);
}

// ── api/openapi.json content check ────────────────────────────────────────────

function checkOpenApiContent(archivePath, rawEntries) {
  const content = extractArchiveEntry(archivePath, rawEntries, "api/openapi.json");
  if (!content || content.trim() === "") {
    fail("api/openapi.json is missing or empty in the archive");
    return;
  }
  pass("api/openapi.json is non-empty");

  // Defensively strip a leading UTF-8 BOM and surrounding whitespace before parsing.
  const jsonText = content.replace(/^\uFEFF/, "").trim();

  let doc;
  try {
    doc = JSON.parse(jsonText);
  } catch {
    fail("api/openapi.json is not valid JSON");
    return;
  }
  pass("api/openapi.json is valid JSON");

  if (typeof doc.openapi === "string" && doc.openapi.length > 0) {
    pass(`api/openapi.json has openapi field (${doc.openapi})`);
  } else {
    fail("api/openapi.json missing openapi field");
  }

  const infoVer = doc.info && doc.info.version;
  if (typeof infoVer !== "string" || infoVer.length === 0) {
    fail("api/openapi.json missing info.version");
  } else if (infoVer === expectedOpenApi) {
    pass(`api/openapi.json info.version = ${infoVer} (matches package ${version})`);
  } else {
    fail(
      `api/openapi.json info.version = ${infoVer}, expected ${expectedOpenApi} (from package ${version})`
    );
  }
}

// ── readme.txt content extraction ─────────────────────────────────────────────

function checkReadmeContent(archivePath, rawEntries) {
  const content = extractArchiveEntry(archivePath, rawEntries, "readme.txt");

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
  if (/portier(\.exe)?\s+runtime/i.test(content)) {
    pass("readme.txt mentions CLI command: portier runtime");
  } else {
    fail("readme.txt does not mention CLI command: portier runtime");
  }
  if (/portier(\.exe)?\s+list/i.test(content)) {
    pass("readme.txt mentions CLI command: portier list");
  } else {
    fail("readme.txt does not mention CLI command: portier list");
  }
  if (/portier(\.exe)?\s+diagnostics export/i.test(content)) {
    pass("readme.txt mentions CLI command: portier diagnostics export");
  } else {
    fail("readme.txt does not mention CLI command: portier diagnostics export");
  }
}

// ── checksums.sha256 verification ───────────────────────────────────────────────────

function checkChecksums(dir) {
  const sumsPath = join(dir, CHECKSUMS_NAME);
  if (!existsSync(sumsPath)) {
    fail(`${CHECKSUMS_NAME} not found — run: npm run build:release:current`);
    return;
  }
  pass(`${CHECKSUMS_NAME} present`);

  let entries;
  try {
    entries = parseSha256Sums(readFileSync(sumsPath, "utf8"));
  } catch (err) {
    fail(`${CHECKSUMS_NAME} ${err.message}`);
    return;
  }
  if (entries.length === 0) {
    fail(`${CHECKSUMS_NAME} contains no entries`);
    return;
  }

  // Every produced (version-scoped) artifact on disk must be listed.
  const listed = new Set(entries.map((e) => e.name));
  const produced = findReleaseArtifacts(dir, version);
  if (produced.length === 0) {
    warn("No release artifacts found on disk to cross-check against checksums.sha256");
  }
  for (const name of produced) {
    if (!listed.has(name)) {
      fail(`Produced artifact not listed in ${CHECKSUMS_NAME}: ${name}`);
    }
  }

  // Every listed entry must exist and match its recorded hash.
  for (const e of entries) {
    const full = join(dir, e.name);
    if (!existsSync(full)) {
      fail(`${CHECKSUMS_NAME} references a missing file: ${e.name}`);
      continue;
    }
    const actual = sha256File(full);
    if (actual === e.hash) {
      pass(`${e.name} sha256 OK`);
    } else {
      fail(
        `${e.name} sha256 mismatch (expected ${e.hash.slice(0, 12)}…, got ${actual.slice(0, 12)}…)`
      );
    }
  }
}

// ── Portable archive + installer checks ───────────────────────────────────────

function checkArchiveAndInstaller() {
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
    const cliBin = platformLabel === "windows" ? "portier.exe" : "portier";

    checkRequired(entries, cliBin, `${cliBin} (CLI binary)`);
    checkRequired(entries, serviceBin, `${serviceBin} (native Go service binary)`);
    checkRequired(entries, "server.js", "server.js (Node/TypeScript fallback)");
    checkRequired(entries, "readme.txt", "readme.txt");
    checkRequired(entries, "api/openapi.json", "api/openapi.json (OpenAPI document)");
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

    console.log("\napi/openapi.json content:");
    checkOpenApiContent(archivePath, rawEntries);

    console.log("\nreadme.txt content:");
    checkReadmeContent(archivePath, rawEntries);
  }

  // Native installer artifact. The Windows WiX MSI is canonical (required); the
  // macOS .pkg track is in progress (reported, not fatal yet). When present, the
  // installer's integrity is also verified by the checksums.sha256 check. Platforms
  // without a native installer (Linux today) have no installer expectation.
  if (!portableOnly && installerName) {
    const required = installerIsRequired(platformLabel);
    console.log(`\nInstaller artifact (${required ? "canonical" : "native, in progress"}):`);
    if (!existsSync(installerPath)) {
      const msg = `Installer not found: ${installerName} — build with: npm run build:release:current`;
      const hint =
        platformLabel === "windows" ? "Windows requires WiX 7"
        : platformLabel === "linux" ? "Linux .deb requires dpkg-deb (Debian/Ubuntu)"
        : "macOS .pkg requires pkgbuild; built on macOS";
      if (required) fail(`${msg} (${hint})`);
      else warn(`${msg} (${hint})`);
    } else {
      const iStat = statSync(installerPath);
      pass(`${installerName} (${(iStat.size / 1024 / 1024).toFixed(1)} MB)`);
    }
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
  fail(`build/releases/${platformLabel}/ not found — run: npm run build:release:current`);
  console.error("[validate-release] FAILED: release directory missing.\n");
  process.exit(1);
}
pass(`build/releases/${platformLabel}/ exists`);

if (checksumsOnly) {
  console.log("\nArtifacts found:");
  const found = findReleaseArtifacts(releasesDir, version);
  if (found.length === 0) {
    warn("No release artifacts found for this version");
  } else {
    for (const name of found) pass(name);
  }
} else {
  checkArchiveAndInstaller();
}

// Checksums (checksums.sha256) — verified in every mode
console.log("\nChecksums (checksums.sha256):");
checkChecksums(releasesDir);

// Summary
console.log(
  `\n[validate-release] ${passed} passed, ${warned} warned, ${failed} failed.`
);
if (failed > 0) {
  console.error("[validate-release] FAILED.\n");
  process.exit(1);
}
console.log("[validate-release] Release artifacts validated.\n");
