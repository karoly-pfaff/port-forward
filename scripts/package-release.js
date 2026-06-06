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
 *   node scripts/package-release.js [options]
 *
 * Options:
 *   --current-platform      Build artifacts for the current OS only (default).
 *   --windows               Build Windows artifacts. Must run on Windows.
 *   --macos                 Build macOS artifacts. Must run on macOS.
 *   --linux                 Build Linux artifacts. Must run on Linux.
 *   --portable-only         Produce portable archives only; skip installer builds.
 *   --skip-installers       Alias for --portable-only.
 *   --no-build              Skip npm run package:portier; use existing build/portier/.
 *   --version <v>           Override version string (default: reads from package.json).
 */

import { existsSync, readFileSync, mkdirSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

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
    console.error("[package-release] package.json not found.");
    process.exit(1);
  }
  const pkg = JSON.parse(readFileSync(pkgPath, "utf8"));
  if (!pkg.version) {
    console.error("[package-release] version field missing in package.json.");
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
    console.error(`[package-release] Cannot build ${name} artifacts on ${process.platform}.`);
    console.error("  Portier service binaries are platform-native.");
    console.error("  Run this command on the target OS to build its artifacts.");
    process.exit(1);
  }
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function log(msg) { console.log(`[package-release] ${msg}`); }

function run(cmd, args, opts = {}) {
  const result = spawnSync(cmd, args, {
    stdio: "inherit",
    cwd: repoRoot,
    shell: isWindows,
    ...opts,
  });
  if ((result.status ?? 1) !== 0) {
    console.error(`[package-release] Command failed (exit ${result.status ?? 1}): ${cmd} ${args.join(" ")}`);
    process.exit(result.status ?? 1);
  }
}

// ── Build base package ────────────────────────────────────────────────────────

log(`Version      : ${version}`);
log(`Platform(s)  : ${[...targets].join(", ")}`);
log(`Portable only: ${portableOnly}`);
log(`Skip build   : ${noBuild}`);
log("");

if (!noBuild) {
  log("Running npm run package:portier...");
  run(npmCmd, ["run", "package:portier"]);
  log("");
} else {
  log("Skipping package build (--no-build).");
  log("");
}

const packageDir = join(repoRoot, "build", "portier");
if (!existsSync(packageDir)) {
  console.error("[package-release] build/portier/ not found. Run: npm run package:portier");
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
    if (!portableOnly) buildWindowsInstaller(releasesDir, version);
  } else if (target === "darwin") {
    buildMacosPortable(version);
    if (!portableOnly) {
      log("  macOS .pkg: deferred (pkgbuild required on macOS). See docs/installer-strategy.md.");
    }
  } else if (target === "linux") {
    buildLinuxPortable(version);
    if (!portableOnly) {
      log("  Linux installer: no .deb/.rpm in v1.1. Portable tar.gz only.");
    }
  }

  log("");
}

log("Release packaging complete.");
log(`Artifacts: ${join(repoRoot, "build", "releases")}`);

// ── Windows ───────────────────────────────────────────────────────────────────

function buildWindowsPortable(releasesDir, version) {
  const zipName = `portier-${version}-windows-portable.zip`;
  const zipPath = join(releasesDir, zipName);
  log(`  Portable: ${zipName}`);

  // Compress-Archive -Path 'dir\*' includes all items from packageDir at the
  // zip root, preserving subdirectories (web\, etc.) without an outer wrapper dir.
  const src = packageDir + "\\*";
  const srcEsc = src.replace(/'/g, "''");
  const dstEsc = zipPath.replace(/'/g, "''");
  const psCmd = `Compress-Archive -Path '${srcEsc}' -DestinationPath '${dstEsc}' -Force`;

  run("powershell", ["-NoProfile", "-NonInteractive", "-Command", psCmd]);
  log(`  Created : ${zipPath}`);
}

function buildWindowsInstaller(releasesDir, version) {
  const installerScript = join(
    repoRoot, "scripts", "windows", "release", "build-release.ps1"
  );
  log("  Installer: Inno Setup...");

  // build-release.ps1 handles ISCC.exe detection and exits 1 if unavailable.
  // We treat installer failure as non-fatal: the portable zip is still valid.
  const result = spawnSync(
    "powershell",
    ["-NoProfile", "-ExecutionPolicy", "Bypass", "-File", installerScript, "-NoPackage", "-Version", version],
    { stdio: "inherit", cwd: repoRoot, shell: isWindows }
  );

  if ((result.status ?? 1) !== 0) {
    console.warn(
      "[package-release]   WARNING: Windows installer build failed (Inno Setup unavailable or errored)."
    );
    console.warn(
      "[package-release]   Portable zip is still valid. Install Inno Setup 6 to build the installer."
    );
  } else {
    log(`  Created : build/releases/windows/Portier-Setup-${version}.exe`);
  }
}

// ── macOS ─────────────────────────────────────────────────────────────────────

function buildMacosPortable(version) {
  log("  Portable: delegating to scripts/macos/release/build-release.sh...");
  run("bash", ["scripts/macos/release/build-release.sh", "--no-package", "--version", version], {
    shell: false,
  });
}

// ── Linux ─────────────────────────────────────────────────────────────────────

function buildLinuxPortable(version) {
  log("  Portable: delegating to scripts/linux/release/build-release.sh...");
  run("bash", ["scripts/linux/release/build-release.sh", "--no-package", "--version", version], {
    shell: false,
  });
}
