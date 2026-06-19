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
import AdmZip from "adm-zip";
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

  if (target === "win32") {
    buildWindowsPortable(releasesDir, version);
    // The WiX MSI is the canonical Windows installer. (Inno Setup has been retired
    // to scripts/windows/legacy/ and is not built by the release flow.)
    if (!portableOnly) buildWindowsMsi(releasesDir, version);
  } else if (target === "darwin") {
    // The macOS script builds the portable tar.gz and, on macOS with pkgbuild, the
    // native .pkg (the v1.18 macOS installer track). --portable-only skips the .pkg.
    buildMacosRelease(version, portableOnly);
  } else if (target === "linux") {
    buildLinuxPortable(version);
    if (!portableOnly) {
      // v1.18 Linux ships the portable tar.gz (with systemd scripts as the
      // canonical service layer). .deb/.rpm are a planned package track for a
      // later slice (built/validated on native runners). See docs/installer.md.
      log("  Linux: portable tar.gz is the v1.18 release artifact (.deb/.rpm: planned, later slice).");
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

// ── Windows ───────────────────────────────────────────────────────────────────

function buildWindowsPortable(releasesDir, version) {
  const zipName = `portier-${version}-windows-portable.zip`;
  const zipPath = join(releasesDir, zipName);
  log(`  Portable: ${zipName}`);

  // Add the contents of packageDir at the zip root, preserving subdirectories
  // (web/, api/, etc.) without an outer wrapper directory. writeZip overwrites
  // any existing archive. adm-zip is used instead of PowerShell Compress-Archive
  // so creation is OS-agnostic and matches the adm-zip reader in validate-release.
  const zip = new AdmZip();
  zip.addLocalFolder(packageDir);
  zip.writeZip(zipPath);
  log(`  Created : ${zipPath}`);
}

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

function buildMacosRelease(version, portableOnly) {
  log("  Portable tar.gz + .pkg: delegating to scripts/macos/release/build-release.sh...");
  const args = ["scripts/macos/release/build-release.sh", "--no-package", "--version", version];
  if (portableOnly) args.push("--portable-only");
  run("bash", args, { shell: false });
}

// ── Linux ─────────────────────────────────────────────────────────────────────

function buildLinuxPortable(version) {
  log("  Portable: delegating to scripts/linux/release/build-release.sh...");
  run("bash", ["scripts/linux/release/build-release.sh", "--no-package", "--version", version], {
    shell: false,
  });
}
