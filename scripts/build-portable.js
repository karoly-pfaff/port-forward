#!/usr/bin/env node
/* global console, process */
/**
 * Cross-platform portable artifact generation.
 *
 * Produces the Windows (.zip) and Linux/macOS (.tar.gz) portable artifacts from any host
 * by cross-compiling the Go CLI/service binaries (pure Go, CGO_ENABLED=0) and packaging
 * them with the platform-neutral runtime assets (server.js, web/, api/openapi.json) plus a
 * generated readme. Unix tarballs get correct exec bits (0755 binaries); the Windows zip
 * uses .exe binaries.
 *
 * Targets (amd64 + arm64 where useful):
 *   windows/amd64 → portier-<v>-windows-amd64.zip
 *   linux/amd64   → portier-<v>-linux-amd64.tar.gz
 *   linux/arm64   → portier-<v>-linux-arm64.tar.gz
 *   macos/amd64   → portier-<v>-macos-amd64.tar.gz
 *   macos/arm64   → portier-<v>-macos-arm64.tar.gz
 * Artifact names always carry the arch (no ambiguous single-name portables).
 *
 * This produces RELEASE-READY portable artifacts. It does NOT run a runtime smoke against
 * foreign binaries — native runtime validation must run on each OS/arch. The neutral assets
 * come from build/portier/, so `npm run build:runtime` (current platform) must have produced
 * it; this script never rebuilds the web/server assets.
 *
 * Usage:
 *   node scripts/build-portable.js [--windows] [--linux] [--macos] [--all] [--version <v>]
 */

import {
  existsSync, mkdirSync, rmSync, cpSync, readFileSync, writeFileSync,
  createWriteStream, readdirSync, statSync,
} from "node:fs";
import { join, resolve, dirname, relative, sep } from "node:path";
import { spawnSync } from "node:child_process";
import { createGzip } from "node:zlib";
import { fileURLToPath } from "node:url";
import AdmZip from "adm-zip";
import tar from "tar-stream";
import { generateChecksums, CHECKSUMS_NAME } from "./library/checksums.js";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const packageDir = join(repoRoot, "build", "portier");

const WIN_HINT =
  "  Use the Windows MSI installer, or:\n" +
  "  powershell -ExecutionPolicy Bypass -File scripts\\windows\\service\\install-service.ps1";
const LINUX_HINT =
  "  sudo bash scripts/linux/service/install-service.sh --source-dir <install-dir>";
const MAC_HINT =
  "  bash scripts/macos/service/install-launch-agent.sh --source-dir <install-dir>";

// One target per OS/arch. `platform` is the build/releases/<platform>/ directory; `goarch`
// is the artifact's architecture suffix.
function mkTarget(platform, goos, goarch, title, format, binExt, installHint) {
  const ext = format === "zip" ? "zip" : "tar.gz";
  return {
    platform, goos, goarch, title, format, binExt, installHint,
    archiveName: (v) => `portier-${v}-${platform}-${goarch}.${ext}`,
  };
}

const TARGETS = [
  mkTarget("windows", "windows", "amd64", "Windows", "zip", ".exe", WIN_HINT),
  mkTarget("linux", "linux", "amd64", "Linux", "tar", "", LINUX_HINT),
  mkTarget("linux", "linux", "arm64", "Linux", "tar", "", LINUX_HINT),
  mkTarget("macos", "darwin", "amd64", "macOS", "tar", "", MAC_HINT),
  mkTarget("macos", "darwin", "arm64", "macOS", "tar", "", MAC_HINT),
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

function readVersion() {
  return flagValue("--version") || JSON.parse(readFileSync(join(repoRoot, "package.json"), "utf8")).version;
}

const log = (m) => console.log(`[build-portable] ${m}`);

// ── Cross-compile a Go binary ─────────────────────────────────────────────────

function crossBuildGo(subdir, goos, goarch, outFile) {
  const r = spawnSync("go", ["build", "-o", outFile, "./sources"], {
    cwd: join(repoRoot, subdir),
    env: { ...process.env, GOOS: goos, GOARCH: goarch, CGO_ENABLED: "0" },
    stdio: "inherit",
  });
  if ((r.status ?? 1) !== 0) {
    console.error(`[build-portable] go build failed: ${subdir} (${goos}/${goarch})`);
    process.exit(r.status ?? 1);
  }
}

// ── Generated portable readme (platform/arch-aware binary names + install hint) ─

function generateReadme(t) {
  const portier = `portier${t.binExt}`;
  const service = `service${t.binExt}`;
  const run = t.format === "zip" ? "" : "./";
  return `Portier ${t.title} (${t.goarch}) Portable Package
================================

This portable archive contains the Portier runtime files only (${t.title}/${t.goarch}). It is
portable and does not install OS services. Use the install option (below) to set up a service.

Files:
  ${portier}${" ".repeat(Math.max(1, 12 - portier.length))}CLI — control a running Portier service from the terminal
  ${service}${" ".repeat(Math.max(1, 12 - service.length))}Native Go runtime (preferred; run as a service)
  server.js     Node.js fallback runtime (requires Node.js)
  web/          Built management UI
  api/openapi.json  OpenAPI 3 description of the /api surface
  readme.txt    This file

Native service (preferred):
  ${run}${service} --service --config <path-to-rules.json> --host 127.0.0.1 --port 47831 --static-dir ${run}web

CLI (requires a running Portier service):
  ${run}${portier} runtime
  ${run}${portier} list
  ${run}${portier} status
  ${run}${portier} diagnostics export --out diagnostics.json

Default management URL: http://127.0.0.1:47831

Config (rules.json) is external and is NOT bundled in this archive. Pass its path
with --config; a new empty rules.json is created when the service first starts if
the path does not exist.

Install as a service (from the Portier repository):
${t.installHint}
`;
}

// ── gzipped-tar creation (Unix exec bits set explicitly; NTFS has none) ────────

function walkFiles(dir, base = dir, out = []) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) walkFiles(full, base, out);
    else out.push({ full, rel: relative(base, full).split(sep).join("/") });
  }
  return out;
}

// createTarGz packs every file under stageDir into a gzipped tar at outPath, sorted
// by name with a fixed mtime. isExec(relPath) decides the mode: 0755 vs 0644.
function createTarGz(stageDir, outPath, isExec) {
  return new Promise((resolvePromise, reject) => {
    const pack = tar.pack();
    const out = createWriteStream(outPath);
    out.on("error", reject);
    out.on("close", resolvePromise);
    pack.on("error", reject);

    const gzip = createGzip({ level: 9 });
    gzip.on("error", reject);
    pack.pipe(gzip).pipe(out);

    const files = walkFiles(stageDir).sort((a, b) =>
      a.rel < b.rel ? -1 : a.rel > b.rel ? 1 : 0
    );
    const writeNext = (i) => {
      if (i >= files.length) {
        pack.finalize();
        return;
      }
      const f = files[i];
      const data = readFileSync(f.full);
      const mode = isExec(f.rel) ? 0o755 : 0o644;
      pack.entry({ name: f.rel, mode, size: data.length, mtime: new Date(0) }, data, (err) => {
        if (err) return reject(err);
        writeNext(i + 1);
      });
    };
    writeNext(0);
  });
}

// ── Build one target ──────────────────────────────────────────────────────────

async function buildTarget(t, version) {
  log(`--- ${t.platform}/${t.goarch} (${t.goos}/${t.goarch}) ---`);

  const stage = join(repoRoot, "build", `_portable-${t.platform}-${t.goarch}-${process.pid}`);
  rmSync(stage, { recursive: true, force: true });
  mkdirSync(stage, { recursive: true });

  try {
    // Cross-compile the binaries (with the platform extension).
    crossBuildGo("service", t.goos, t.goarch, join(stage, `service${t.binExt}`));
    crossBuildGo("tools/cli", t.goos, t.goarch, join(stage, `portier${t.binExt}`));

    // Copy the platform-neutral assets from build/portier/.
    cpSync(join(packageDir, "server.js"), join(stage, "server.js"));
    cpSync(join(packageDir, "web"), join(stage, "web"), { recursive: true });
    mkdirSync(join(stage, "api"), { recursive: true });
    cpSync(join(packageDir, "api", "openapi.json"), join(stage, "api", "openapi.json"));

    writeFileSync(join(stage, "readme.txt"), generateReadme(t));

    const releasesDir = join(repoRoot, "build", "releases", t.platform);
    mkdirSync(releasesDir, { recursive: true });
    const outPath = join(releasesDir, t.archiveName(version));

    if (t.format === "zip") {
      // Windows zip: no Unix exec bits. addLocalFolder packs stage contents at root.
      const zip = new AdmZip();
      zip.addLocalFolder(stage);
      zip.writeZip(outPath);
    } else {
      // Unix tar.gz: 0755 on the binaries, 0644 otherwise.
      const isExec = (rel) => rel === "portier" || rel === "service";
      await createTarGz(stage, outPath, isExec);
    }
    log(`  Created : ${outPath}`);

    // Regenerate this platform's checksums.sha256 (covers all version artifacts present in
    // the platform dir — the native installer plus every arch portable).
    const sums = generateChecksums(releasesDir, version);
    log(`  Checksums: wrote ${CHECKSUMS_NAME} (${sums.length} artifact${sums.length === 1 ? "" : "s"})`);
  } finally {
    rmSync(stage, { recursive: true, force: true });
  }
}

// ── Main ──────────────────────────────────────────────────────────────────────

async function main() {
  const version = readVersion();

  if (!existsSync(packageDir)) {
    console.error("[build-portable] build/portier/ not found. Run: npm run build:runtime");
    process.exit(1);
  }
  for (const req of ["server.js", "web", join("api", "openapi.json")]) {
    if (!existsSync(join(packageDir, req))) {
      console.error(`[build-portable] build/portier/${req} missing. Run: npm run build:runtime`);
      process.exit(1);
    }
  }

  log(`Version : ${version}`);
  log(`Targets : ${selectedTargets.map((t) => `${t.platform}/${t.goarch}`).join(", ")}`);
  log("");

  for (const t of selectedTargets) {
    await buildTarget(t, version);
    log("");
  }

  log("Cross-platform portable build complete.");
  log("Note: native runtime smoke for each OS/arch must run on that OS/arch.");
}

main().catch((err) => {
  console.error("[build-portable] Unexpected error:", err);
  process.exit(1);
});
