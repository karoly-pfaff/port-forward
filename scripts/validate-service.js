#!/usr/bin/env node
/* global console, process */
/**
 * Cross-platform dispatcher for OS service install validation.
 *
 * Runs the appropriate platform validation script for the current OS:
 *   Windows → scripts/windows/service/validate-user-install.ps1 (no admin required)
 *   macOS   → scripts/macos/service/validate-launch-agent.sh
 *   Linux   → scripts/linux/service/validate-systemd-service.sh (requires root/sudo)
 *
 * All extra arguments are forwarded to the platform script.
 *
 * Usage:
 *   npm run validate:service:current [-- --no-build] [-- --keep-files] [-- --port PORT]
 */
import { spawnSync } from "node:child_process";

const platform = process.platform;
const extraArgs = process.argv.slice(2);

let label, cmd, cmdArgs;

if (platform === "win32") {
  label = "Windows user-scope (scheduled task) — no Administrator required";
  cmd = "powershell";
  cmdArgs = [
    "-ExecutionPolicy", "Bypass",
    "-File", "scripts\\windows\\service\\validate-user-install.ps1",
    ...extraArgs,
  ];
} else if (platform === "darwin") {
  label = "macOS LaunchAgent — no sudo required";
  cmd = "bash";
  cmdArgs = ["scripts/macos/service/validate-launch-agent.sh", ...extraArgs];
} else if (platform === "linux") {
  label = "Linux systemd — requires root/sudo";
  cmd = "bash";
  cmdArgs = ["scripts/linux/service/validate-systemd-service.sh", ...extraArgs];
} else {
  console.error(`[validate:service:current] Unsupported platform: ${platform}`);
  console.error("  Supported platforms: win32, darwin, linux");
  process.exit(1);
}

console.log(`[validate:service:current] Platform: ${platform}`);
console.log(`[validate:service:current] Running: ${label}`);

if (platform === "linux") {
  console.log("[validate:service:current] Note: Linux validation requires root.");
  console.log("  If not running as root, the script will fail with a clear error message.");
}

console.log("");

const result = spawnSync(cmd, cmdArgs, {
  stdio: "inherit",
  shell: platform === "win32",
});

process.exit(result.status ?? 1);
