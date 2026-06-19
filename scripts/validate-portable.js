#!/usr/bin/env node
/* global console, process, Buffer */
/**
 * Structural validation for cross-built portable artifacts (Windows .zip + Linux/
 * macOS .tar.gz from build-portable.js), runnable on ANY host.
 *
 * Reads zips with adm-zip and tarballs with tar-stream (Node's direct `tar` spawn is
 * unreliable on Windows). Checks layout, platform binary naming + (for tar.gz) Unix
 * executable bits, api/openapi.json validity + version, forbidden content, and
 * SHA256SUMS coverage.
 *
 * STRUCTURAL only — it does NOT run a runtime smoke against foreign binaries. Native
 * runtime validation must run on each OS.
 *
 * Usage:
 *   node scripts/validate-portable.js [--windows] [--linux] [--macos] [--all] [--version <v>]
 */

import { existsSync, readFileSync, statSync, createReadStream } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { createGunzip } from "node:zlib";
import { fileURLToPath } from "node:url";
import AdmZip from "adm-zip";
import tar from "tar-stream";
import { SHA256SUMS_NAME, parseSha256Sums, sha256File } from "./release-checksums.js";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");

const TARGETS = {
  windows: { label: "windows", format: "zip", binExt: ".exe", archiveName: (v) => `portier-${v}-windows-portable.zip` },
  linux: { label: "linux", format: "tar", binExt: "", archiveName: (v) => `portier-${v}-linux.tar.gz` },
  macos: { label: "macos", format: "tar", binExt: "", archiveName: (v) => `portier-portable-macos-${v}.tar.gz` },
};

// ── Arguments ─────────────────────────────────────────────────────────────────

const rawArgs = process.argv.slice(2);
const hasFlag = (f) => rawArgs.includes(f);
const flagValue = (f) => {
  const i = rawArgs.indexOf(f);
  return i >= 0 && i + 1 < rawArgs.length ? rawArgs[i + 1] : null;
};

const all = hasFlag("--all");
const selected = [];
if (all || hasFlag("--windows")) selected.push("windows");
if (all || hasFlag("--linux")) selected.push("linux");
if (all || hasFlag("--macos")) selected.push("macos");
if (selected.length === 0) selected.push("windows", "linux", "macos");

const version =
  flagValue("--version") ||
  JSON.parse(readFileSync(join(repoRoot, "package.json"), "utf8")).version;

const expectedOpenApi = (() => {
  const p = version.split(".");
  return p.length >= 2 ? `${p[0]}.${p[1]}` : version;
})();

// ── Result tracking ───────────────────────────────────────────────────────────

let passed = 0;
let failed = 0;
const pass = (m) => { console.log(`  ✓ ${m}`); passed++; };
const fail = (m) => { console.error(`  ✗ ${m}`); failed++; };

const BASE_FORBIDDEN = ["rules.json", ".env", "node_modules", "sources", "client", "docs/private"];

// ── gzipped-tar reading (tar-stream; the `tar` CLI is unreliable on Windows) ────

function readTarGz(archivePath) {
  return new Promise((resolvePromise, reject) => {
    const entries = [];
    const extract = tar.extract();
    extract.on("entry", (header, stream, next) => {
      entries.push({
        name: header.name.replace(/\\/g, "/").replace(/^\.\//, ""),
        mode: header.mode,
      });
      stream.on("end", next);
      stream.resume();
    });
    extract.on("finish", () => resolvePromise(entries));
    extract.on("error", reject);
    const gunzip = createGunzip();
    gunzip.on("error", reject);
    createReadStream(archivePath).on("error", reject).pipe(gunzip).pipe(extract);
  });
}

function readTarGzFile(archivePath, name) {
  return new Promise((resolvePromise, reject) => {
    let found = null;
    const extract = tar.extract();
    extract.on("entry", (header, stream, next) => {
      const entryName = header.name.replace(/\\/g, "/").replace(/^\.\//, "");
      if (entryName === name && found === null) {
        const chunks = [];
        stream.on("data", (c) => chunks.push(c));
        stream.on("end", () => {
          found = Buffer.concat(chunks).toString("utf8");
          next();
        });
      } else {
        stream.on("end", next);
        stream.resume();
      }
    });
    extract.on("finish", () => resolvePromise(found));
    extract.on("error", reject);
    const gunzip = createGunzip();
    gunzip.on("error", reject);
    createReadStream(archivePath).on("error", reject).pipe(gunzip).pipe(extract);
  });
}

// ── Archive reading (zip via adm-zip, tar.gz via tar-stream) ───────────────────

async function readArchive(format, archivePath) {
  if (format === "zip") {
    const zip = new AdmZip(archivePath);
    const entries = zip.getEntries();
    const names = new Set(entries.map((e) => e.entryName.replace(/\\/g, "/")));
    return {
      names,
      modeByName: null, // zip carries no Unix mode to assert
      readFile: async (name) => {
        const e = entries.find((x) => x.entryName.replace(/\\/g, "/") === name);
        return e ? zip.readAsText(e) : null;
      },
    };
  }
  const list = await readTarGz(archivePath);
  return {
    names: new Set(list.map((e) => e.name)),
    modeByName: new Map(list.map((e) => [e.name, e.mode])),
    readFile: (name) => readTarGzFile(archivePath, name),
  };
}

async function safeRead(archive, name) {
  try { return await archive.readFile(name); } catch { return null; }
}

async function validateTarget(t) {
  console.log(`\n=== ${t.label} (${t.format}) ===`);
  const releasesDir = join(repoRoot, "build", "releases", t.label);
  const archiveName = t.archiveName(version);
  const archivePath = join(releasesDir, archiveName);

  if (!existsSync(archivePath)) {
    fail(`archive not found: ${archiveName} — build with: npm run build:release:${t.label}`);
    return;
  }
  pass(`${archiveName} (${(statSync(archivePath).size / 1024).toFixed(1)} KB)`);

  let archive;
  try {
    archive = await readArchive(t.format, archivePath);
  } catch (err) {
    fail(`could not read archive (corrupt?): ${err.message}`);
    checkChecksum(releasesDir, archiveName, archivePath);
    return;
  }
  const { names, modeByName } = archive;
  const portier = `portier${t.binExt}`;
  const service = `service${t.binExt}`;

  // Required layout (platform binary names).
  console.log("Layout:");
  for (const req of [portier, service, "server.js", "readme.txt", "api/openapi.json", "web/index.html"]) {
    if (names.has(req)) pass(req);
    else fail(`missing: ${req}`);
  }
  if ([...names].some((n) => n.startsWith("web/assets/"))) pass("web/assets/ (>=1 file)");
  else fail("missing: web/assets/");

  // Forbidden content (incl. the wrong-platform binary names).
  console.log("Forbidden content:");
  const wrongBins = t.binExt === ".exe" ? ["portier", "service"] : ["portier.exe", "service.exe"];
  for (const bad of [...BASE_FORBIDDEN, ...wrongBins]) {
    const present = [...names].some((n) => n === bad || n.startsWith(bad + "/"));
    if (present) fail(`must not contain: ${bad}`);
    else pass(`absent: ${bad}`);
  }

  // Executable bits (tar.gz only; zip carries no Unix mode).
  console.log("Executable bits:");
  if (!modeByName) {
    pass("n/a for zip (Windows binaries do not carry a Unix mode)");
  } else {
    for (const bin of [portier, service]) {
      const mode = modeByName.get(bin);
      if (mode === undefined) fail(`${bin}: no mode (missing)`);
      else if ((mode & 0o777) === 0o755) pass(`${bin} mode 0755 (executable)`);
      else fail(`${bin} mode ${(mode & 0o777).toString(8)} (expected 0755)`);
    }
  }

  // OpenAPI content + version.
  console.log("OpenAPI:");
  const openapiText = await safeRead(archive, "api/openapi.json");
  if (!openapiText) {
    fail("api/openapi.json could not be read");
  } else {
    let doc;
    try { doc = JSON.parse(openapiText.replace(/^\uFEFF/, "")); } catch { doc = null; }
    if (!doc) fail("api/openapi.json is not valid JSON");
    else {
      if (typeof doc.openapi === "string" && doc.openapi) pass(`openapi = ${doc.openapi}`);
      else fail("api/openapi.json missing openapi field");
      if (doc.info && doc.info.version === expectedOpenApi) pass(`info.version = ${doc.info.version} (matches package ${version})`);
      else fail(`info.version = ${JSON.stringify(doc.info && doc.info.version)}, expected ${expectedOpenApi}`);
    }
  }

  // readme content (allow optional .exe in CLI examples).
  const readme = await safeRead(archive, "readme.txt");
  console.log("readme.txt:");
  if (!readme) fail("readme.txt could not be read");
  else {
    const checks = [
      [/127\.0\.0\.1:47831/, "mentions management URL"],
      [/rules\.json|--config/i, "mentions --config / rules.json"],
      [/portable|does not install/i, "states it is portable / does not install"],
      [/portier(\.exe)?\s+runtime/i, "mentions CLI: portier runtime"],
      [/portier(\.exe)?\s+list/i, "mentions CLI: portier list"],
      [/portier(\.exe)?\s+diagnostics export/i, "mentions CLI: portier diagnostics export"],
    ];
    for (const [re, label] of checks) {
      if (re.test(readme)) pass(`readme.txt ${label}`);
      else fail(`readme.txt missing: ${label}`);
    }
  }

  checkChecksum(releasesDir, archiveName, archivePath);
}

// checkChecksum verifies the archive is listed in SHA256SUMS with a matching hash.
function checkChecksum(releasesDir, archiveName, archivePath) {
  console.log("Checksums (SHA256SUMS):");
  const sumsPath = join(releasesDir, SHA256SUMS_NAME);
  if (!existsSync(sumsPath)) {
    fail(`${SHA256SUMS_NAME} not found`);
    return;
  }
  let sums;
  try {
    sums = parseSha256Sums(readFileSync(sumsPath, "utf8"));
  } catch (err) {
    fail(`${SHA256SUMS_NAME} ${err.message}`);
    return;
  }
  const entry = sums.find((s) => s.name === archiveName);
  if (!entry) fail(`${archiveName} not listed in ${SHA256SUMS_NAME}`);
  else if (sha256File(archivePath) === entry.hash) pass(`${archiveName} sha256 OK`);
  else fail(`${archiveName} sha256 mismatch`);
}

async function main() {
  console.log(`[validate-portable] Version : ${version}`);
  console.log(`[validate-portable] Targets : ${selected.join(", ")}`);
  console.log("[validate-portable] Structural only — native runtime smoke must run on each OS.");

  for (const key of selected) await validateTarget(TARGETS[key]);

  console.log(`\n[validate-portable] ${passed} passed, ${failed} failed.`);
  if (failed > 0) {
    console.error("[validate-portable] FAILED.\n");
    process.exit(1);
  }
  console.log("[validate-portable] Portable artifacts structurally validated.\n");
}

main().catch((err) => {
  console.error("[validate-portable] Unexpected error:", err);
  process.exit(1);
});
