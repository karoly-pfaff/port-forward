#!/usr/bin/env node
/* global console, process, Buffer */
/**
 * Structural validation for cross-built portable artifacts (Windows .zip + Linux/
 * macOS .tar.gz from build-portable.js), runnable on ANY host.
 *
 * Reads zips with adm-zip and tarballs with tar-stream (Node's direct `tar` spawn is
 * unreliable on Windows). Checks layout, platform binary naming + (for tar.gz) Unix
 * executable bits, api/openapi.json validity + version, forbidden content, and
 * checksums.sha256 coverage.
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
import { CHECKSUMS_NAME, parseSha256Sums, sha256File } from "./library/checksums.js";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");

// One structural-validation target per OS/arch. Names always carry the arch.
function mkTarget(platform, goarch, format, binExt) {
  const ext = format === "zip" ? "zip" : "tar.gz";
  return { platform, goarch, format, binExt, archiveName: (v) => `portier-${v}-${platform}-${goarch}.${ext}` };
}
const TARGETS = [
  mkTarget("windows", "amd64", "zip", ".exe"),
  mkTarget("linux", "amd64", "tar", ""),
  mkTarget("linux", "arm64", "tar", ""),
  mkTarget("macos", "amd64", "tar", ""),
  mkTarget("macos", "arm64", "tar", ""),
];

// ── Arguments ─────────────────────────────────────────────────────────────────

const rawArgs = process.argv.slice(2);
const hasFlag = (f) => rawArgs.includes(f);
const flagValue = (f) => {
  const i = rawArgs.indexOf(f);
  return i >= 0 && i + 1 < rawArgs.length ? rawArgs[i + 1] : null;
};

const all = hasFlag("--all");
const wantPlatforms = new Set();
if (all || hasFlag("--windows")) wantPlatforms.add("windows");
if (all || hasFlag("--linux")) wantPlatforms.add("linux");
if (all || hasFlag("--macos")) wantPlatforms.add("macos");
if (wantPlatforms.size === 0) ["windows", "linux", "macos"].forEach((p) => wantPlatforms.add(p));
const selectedTargets = TARGETS.filter((t) => wantPlatforms.has(t.platform));

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

function readTarGzBytes(archivePath, name) {
  return new Promise((resolvePromise, reject) => {
    let found = null;
    const extract = tar.extract();
    extract.on("entry", (header, stream, next) => {
      const entryName = header.name.replace(/\\/g, "/").replace(/^\.\//, "");
      if (entryName === name && found === null) {
        const chunks = [];
        stream.on("data", (c) => chunks.push(c));
        stream.on("end", () => { found = Buffer.concat(chunks); next(); });
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
    const find = (name) => entries.find((x) => x.entryName.replace(/\\/g, "/") === name);
    return {
      names,
      modeByName: null, // zip carries no Unix mode to assert
      readFile: async (name) => { const e = find(name); return e ? zip.readAsText(e) : null; },
      readBytes: async (name) => { const e = find(name); return e ? zip.readFile(e) : null; },
    };
  }
  const list = await readTarGz(archivePath);
  return {
    names: new Set(list.map((e) => e.name)),
    modeByName: new Map(list.map((e) => [e.name, e.mode])),
    readFile: (name) => readTarGzFile(archivePath, name),
    readBytes: (name) => readTarGzBytes(archivePath, name),
  };
}

async function safeRead(archive, name) {
  try { return await archive.readFile(name); } catch { return null; }
}

// machineArch reads an executable's header and returns "amd64"/"arm64" (or a debug token).
// Supports ELF (Linux), Mach-O 64-bit LE (macOS), and PE (Windows).
function machineArch(buf) {
  if (!buf || buf.length < 8) return "empty";
  // ELF: 0x7F 'E' 'L' 'F'; e_machine is a LE uint16 at offset 18.
  if (buf[0] === 0x7f && buf[1] === 0x45 && buf[2] === 0x4c && buf[3] === 0x46) {
    const m = buf.readUInt16LE(18);
    if (m === 0x3e) return "amd64";
    if (m === 0xb7) return "arm64";
    return `elf-0x${m.toString(16)}`;
  }
  // Mach-O 64-bit little-endian: magic CF FA ED FE; cputype is a LE uint32 at offset 4.
  if (buf[0] === 0xcf && buf[1] === 0xfa && buf[2] === 0xed && buf[3] === 0xfe) {
    const cpu = buf.readUInt32LE(4);
    if (cpu === 0x01000007) return "amd64"; // CPU_TYPE_X86_64
    if (cpu === 0x0100000c) return "arm64"; // CPU_TYPE_ARM64
    return `macho-0x${cpu.toString(16)}`;
  }
  // PE: 'MZ' … COFF machine is a LE uint16 at the PE header + 4.
  if (buf[0] === 0x4d && buf[1] === 0x5a && buf.length >= 0x40) {
    const peOff = buf.readUInt32LE(0x3c);
    if (buf.length >= peOff + 6 && buf.readUInt32LE(peOff) === 0x00004550) {
      const machine = buf.readUInt16LE(peOff + 4);
      if (machine === 0x8664) return "amd64";
      if (machine === 0xaa64) return "arm64";
      return `pe-0x${machine.toString(16)}`;
    }
  }
  return "unknown";
}

async function validateTarget(t) {
  console.log(`\n=== ${t.platform}/${t.goarch} (${t.format}) ===`);
  const releasesDir = join(repoRoot, "build", "releases", t.platform);
  const archiveName = t.archiveName(version);
  const archivePath = join(releasesDir, archiveName);

  if (!existsSync(archivePath)) {
    fail(`archive not found: ${archiveName} — build with: npm run build:release:${t.platform}`);
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

  // Binary architecture: prove the artifact actually contains the named arch (a wrong
  // GOARCH would otherwise pass every structural check above).
  console.log(`Binary architecture (expect ${t.goarch}):`);
  for (const bin of [service, portier]) {
    let bytes = null;
    try { bytes = await archive.readBytes(bin); } catch { bytes = null; }
    if (!bytes) { fail(`${bin}: could not read for arch check`); continue; }
    // Enough bytes to cover the PE header (located via the LE offset at 0x3c); ELF/Mach-O
    // machine fields are in the first 20 bytes.
    const arch = machineArch(bytes.subarray(0, 8192));
    if (arch === t.goarch) pass(`${bin} is ${arch}`);
    else fail(`${bin} is ${arch}, expected ${t.goarch}`);
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

// checkChecksum verifies the archive is listed in checksums.sha256 with a matching hash.
function checkChecksum(releasesDir, archiveName, archivePath) {
  console.log("Checksums (checksums.sha256):");
  const sumsPath = join(releasesDir, CHECKSUMS_NAME);
  if (!existsSync(sumsPath)) {
    fail(`${CHECKSUMS_NAME} not found`);
    return;
  }
  let sums;
  try {
    sums = parseSha256Sums(readFileSync(sumsPath, "utf8"));
  } catch (err) {
    fail(`${CHECKSUMS_NAME} ${err.message}`);
    return;
  }
  const entry = sums.find((s) => s.name === archiveName);
  if (!entry) fail(`${archiveName} not listed in ${CHECKSUMS_NAME}`);
  else if (sha256File(archivePath) === entry.hash) pass(`${archiveName} sha256 OK`);
  else fail(`${archiveName} sha256 mismatch`);
}

async function main() {
  console.log(`[validate-portable] Version : ${version}`);
  console.log(`[validate-portable] Targets : ${selectedTargets.map((t) => `${t.platform}/${t.goarch}`).join(", ")}`);
  console.log("[validate-portable] Structural only — native runtime smoke must run on each OS/arch.");

  for (const t of selectedTargets) await validateTarget(t);

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
