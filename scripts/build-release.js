#!/usr/bin/env node
/* global console, process */
/**
 * Unified release artifact packaging for Portier.
 *
 * Produces portable archives (and, where tooling is available, installer
 * artifacts) under build/releases/<platform>/.
 *
 * Service binaries are platform-native. Only the current OS can produce
 * artifacts for its own platform. For a multi-platform release, run this
 * command separately on each target OS.
 *
 * Usage:
 *   node scripts/build-release.js [options]
 *
 * Options:
 *   --current-platform      Build artifacts for the current OS only (default).
 *   --windows               Build Windows artifacts. Must run on Windows.
 *   --macos                 Build macOS artifacts. Must run on macOS.
 *   --linux                 Build Linux artifacts. Must run on Linux.
 *   --portable-only         Produce portable archives only; skip installer builds.
 *   --skip-installers       Alias for --portable-only.
 *   --no-build              Skip npm run build:runtime; use existing build/portier/.
 *   --version <v>           Override version string (default: reads from package.json).
 */

import { existsSync, readFileSync, mkdirSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { generateChecksums, SHA256SUMS_NAME } from "./release-checksums.js";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const isWindows = process.platform === "win32";
const npmCmd = isWindows ? "npm.cmd" : "npm";

// ── Parse arguments ───────────────────────────────────────────────────────────

const rawArgs = process.argv.slice(2);
const hasFlag = (f) => rawArgs.includes(f);
const flagValue = (f) => {
  const i = rawArgs.indexOf(f);
  return i >= 0 && i + 1 < rawArgs.length ? rawArgs[i + 1] : null;
};

const noBuild = hasFlag("--no-build");
const portableOnly = hasFlag("--portable-only") || hasFlag("--skip-installers");
const versionOverride = flagValue("--version");
const explicitWindows = hasFlag("--windows");
const explicitMacos = hasFlag("--macos");
const explicitLinux = hasFlag("--linux");
const anyExplicit = explicitWindows || explicitMacos || explicitLinux;

// ── Resolve version ───────────────────────────────────────────────────────────

function readVersion() {
  const pkgPath = join(repoRoot, "package.json");
  if (!existsSync(pkgPath)) {
    console.error("[build-release] package.json not found.");
    process.exit(1);
  }
  const pkg = JSON.parse(readFileSync(pkgPath, "utf8"));
  if (!pkg.version) {
    console.error("[build-release] version field missing in package.json.");
    process.exit(1);
  }
  return pkg.version;
}

const version = versionOverride || readVersion();

// ── Determine platforms ───────────────────────────────────────────────────────

const targets = new Set();
if (anyExplicit) {
  if (explicitWindows) targets.add("win32");
  if (explicitMacos) targets.add("darwin");
  if (explicitLinux) targets.add("linux");
} else {
  targets.add(process.platform);
}

// Fail early if a non-current platform is explicitly requested
for (const target of targets) {
  if (target !== process.platform) {
    const name = target === "win32" ? "Windows" : target === "darwin" ? "macOS" : "Linux";
    console.error(`[build-release] Cannot build ${name} artifacts on ${process.platform}.`);
    console.error("  Portier service binaries are platform-native.");
    console.error("  Run this command on the target OS to build its artifacts.");
    process.exit(1);
  }
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function log(msg) { console.log(`[build-release] ${msg}`); }

function run(cmd, args, opts = {}) {
  const result = spawnSync(cmd, args, {
    stdio: "inherit",
    cwd: repoRoot,
    shell: isWindows,
    ...opts,
  });
  if ((result.status ?? 1) !== 0) {
    console.error(`[build-release] Command failed (exit ${result.status ?? 1}): ${cmd} ${args.join(" ")}`);
    process.exit(result.status ?? 1);
  }
}

// ── Build runtime package ─────────────────────────────────────────────────────

log(`Version      : ${version}`);
log(`Platform(s)  : ${[...targets].join(", ")}`);
log(`Portable only: ${portableOnly}`);
log(`Skip build   : ${noBuild}`);
log("");

if (!noBuild) {
  log("Running npm run build:runtime...");
  run(npmCmd, ["run", "build:runtime"]);
  log("");
} else {
  log("Skipping runtime build (--no-build).");
  log("");
}

const packageDir = join(repoRoot, "build", "portier");
if (!existsSync(packageDir)) {
  console.error("[build-release] build/portier/ not found. Run: npm run build:runtime");
  process.exit(1);
}

// ── Generate artifacts ────────────────────────────────────────────────────────

for (const target of targets) {
  const platformLabel =
    target === "win32" ? "windows" : target === "darwin" ? "macos" : "linux";
  const releasesDir = join(repoRoot, "build", "releases", platformLabel);
  mkdirSync(releasesDir, { recursive: true });

  log(`--- ${platformLabel} ---`);

  // Portable artifact: delegate to the single portable generator (build-portable.js)
  // so the zip/tar.gz is produced one way across every OS and release path.
  buildPortableArtifact(platformLabel, version);

  // Native installer (current platform only; not in --portable-only mode).
  if (!portableOnly) {
    if (target === "win32") {
      // The WiX MSI is the canonical Windows installer (Inno retired to legacy/).
      buildWindowsMsi(releasesDir, version);
    } else if (target === "darwin") {
      // Native macOS .pkg (built on macOS by pkgbuild; v1.18 installer track).
      buildMacosPkg(version);
    } else if (target === "linux") {
      // Native Linux installer: a file-install .deb (dpkg-deb). The portable tar.gz is
      // still produced above; .rpm is a planned later track.
      buildLinuxDeb(version);
    }
  }

  // Checksums: cover every release artifact present for this version (portable
  // archives and installer artifacts alike). Regenerated each run so it always
  // reflects the current artifacts on disk.
  const sums = generateChecksums(releasesDir, version);
  log(`  Checksums: wrote ${SHA256SUMS_NAME} (${sums.length} artifact${sums.length === 1 ? "" : "s"})`);
  for (const e of sums) log(`    ${e.hash.slice(0, 12)}…  ${e.name}`);

  log("");
}

log("Release packaging complete.");
log(`Artifacts: ${join(repoRoot, "build", "releases")}`);

// ── Portable artifact (one method for all OSes) ────────────────────────────────

function buildPortableArtifact(label, version) {
  log("  Portable: delegating to scripts/build-portable.js...");
  // build-portable.js cross-compiles the binaries (pure Go) and packages the full
  // runtime layout (Windows .zip / Unix .tar.gz with exec bits). It reuses the
  // neutral assets from build/portier/ and regenerates this platform's SHA256SUMS.
  run("node", ["scripts/build-portable.js", `--${label}`, "--version", version], { shell: isWindows });
}

// ── Windows ───────────────────────────────────────────────────────────────────

function buildWindowsMsi(releasesDir, version) {
  const msiScript = join(repoRoot, "scripts", "windows", "release", "build-release.ps1");
  log("  Installer: WiX MSI (canonical Windows installer)...");

  // build-release.ps1 resolves the wix tool (PATH or dotnet global tools) and exits
  // non-zero if WiX is unavailable. The MSI is the canonical Windows installer, so
  // a build failure is FATAL for a full Windows release. Use --portable-only to
  // build just the portable zip when WiX is not available.
  const result = spawnSync(
    "powershell",
    ["-NoProfile", "-ExecutionPolicy", "Bypass", "-File", msiScript, "-Version", version, "-OutputDir", releasesDir],
    { stdio: "inherit", cwd: repoRoot, shell: isWindows }
  );

  if ((result.status ?? 1) !== 0) {
    console.error(
      "[build-release]   ERROR: WiX MSI build failed (WiX Toolset unavailable or errored)."
    );
    console.error(
      "[build-release]   The MSI is the canonical Windows installer. Install WiX 7 (dotnet tool install --global wix),"
    );
    console.error(
      "[build-release]   or run with --portable-only to produce just the portable zip."
    );
    process.exit(result.status ?? 1);
  }
  log(`  Created : build/releases/windows/Portier-${version}.msi`);
}

// ── macOS ─────────────────────────────────────────────────────────────────────

function buildMacosPkg(version) {
  log("  Installer: macOS .pkg: delegating to scripts/macos/release/build-release.sh --pkg-only...");
  // The portable tar.gz is built by build-portable.js; here we build only the
  // native .pkg (pkgbuild; macOS only — non-fatal if pkgbuild is unavailable).
  run("bash", ["scripts/macos/release/build-release.sh", "--no-package", "--pkg-only", "--version", version], {
    shell: false,
  });
}

// ── Linux ─────────────────────────────────────────────────────────────────────

function buildLinuxDeb(version) {
  log("  Installer: Linux .deb: delegating to scripts/linux/release/build-release.sh...");
  // File-install .deb via dpkg-deb (Linux only). The script skips gracefully (exit 0)
  // on a host without dpkg-deb, so the portable tar.gz remains the baseline.
  run("bash", ["scripts/linux/release/build-release.sh", "--version", version], { shell: false });
}
