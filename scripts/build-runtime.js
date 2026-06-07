#!/usr/bin/env node
/* global console, process */
import { spawnSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");

const outputDir = "build/portier";

// Call the platform script directly with the shared output directory so that
// `npm run build:runtime` always produces build/portier/ on every platform.
// The platform-specific scripts (build:runtime:windows, build:runtime:macos, build:runtime:linux)
// still output to their own build/{platform}/ directories when called directly.
const platformConfig = {
  win32: {
    cmd: "powershell",
    args: [
      "-ExecutionPolicy", "Bypass",
      "-File", "scripts\\windows\\build-runtime.ps1",
      "-OutputDir", `.\\${outputDir.replace(/\//g, "\\")}`,
    ],
    shell: true,
  },
  darwin: {
    cmd: "bash",
    args: ["scripts/macos/build-runtime.sh", `./${outputDir}`],
    shell: false,
  },
  linux: {
    cmd: "bash",
    args: ["scripts/linux/build-runtime.sh", `./${outputDir}`],
    shell: false,
  },
};

const config = platformConfig[process.platform];
if (!config) {
  console.error(`[build:runtime] Unsupported platform: ${process.platform}`);
  process.exit(1);
}

console.log(`[build:runtime] Building ${outputDir}/ on ${process.platform}...`);
const result = spawnSync(config.cmd, config.args, {
  stdio: "inherit",
  cwd: repoRoot,
  shell: config.shell,
});

process.exit(result.status ?? 1);
