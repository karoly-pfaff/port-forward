#!/usr/bin/env node
/* global console, process */
/**
 * Installer script dry-run and static analysis tests.
 *
 * Static checks (all platforms):
 *   - Validation scripts use test-specific names, not production service names.
 *   - Validation scripts do not hard-code the production port (47831).
 *   - Validation scripts use temp/isolated paths, not production install dirs.
 *   - Install scripts have quoting helpers for paths with spaces (Windows).
 *   - macOS plist generation uses absolute expanded paths, not shell ~ shortcuts.
 *   - Linux systemd defaults to /opt/portier/service and /etc/portier/rules.json.
 *   - No install script silently runs firewall commands (netsh, iptables, ufw, firewall-cmd).
 *
 * Dynamic dry-run (current platform only):
 *   - Windows: run install-service.ps1 -DryRun and verify planned output.
 *   - macOS:   run install-launch-agent.sh --dry-run and verify planned output.
 *   - Linux:   run install-service.sh --dry-run and verify planned output.
 *
 * Usage:
 *   node scripts/test-scripts.js
 */

import { existsSync, readFileSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const platform = process.platform;

const WIN_SCRIPTS  = join(repoRoot, "scripts", "windows", "service");
const MACOS_SCRIPTS = join(repoRoot, "scripts", "macos",   "service");
const LINUX_SCRIPTS = join(repoRoot, "scripts", "linux",   "service");

let passed = 0;
let failed = 0;
let skipped = 0;

function pass(msg) {
  console.log(`  ✓ ${msg}`);
  passed++;
}

function fail(msg) {
  console.error(`  ✗ ${msg}`);
  failed++;
}

function skip(msg) {
  console.log(`  - ${msg} [skip]`);
  skipped++;
}

function readScript(filePath) {
  if (!existsSync(filePath)) return null;
  return readFileSync(filePath, "utf8");
}

// ── Static analysis helpers ──────────────────────────────────────────────────

function assertContains(label, content, pattern, description) {
  const re = pattern instanceof RegExp ? pattern : new RegExp(pattern);
  if (re.test(content)) {
    pass(`${label}: ${description}`);
  } else {
    fail(`${label}: expected to find ${description} — pattern: ${re}`);
  }
}

function assertAbsent(label, content, pattern, description) {
  const re = pattern instanceof RegExp ? pattern : new RegExp(pattern);
  if (!re.test(content)) {
    pass(`${label}: absent (correct) — ${description}`);
  } else {
    fail(`${label}: must NOT contain ${description} — pattern: ${re}`);
  }
}

// ── Firewall command check ────────────────────────────────────────────────────
// None of the install scripts should silently run firewall commands.

const FIREWALL_PATTERNS = [
  /netsh\s+advfirewall/i,
  /netsh\s+firewall/i,
  /iptables\s/,
  /ip6tables\s/,
  /ufw\s/,
  /firewall-cmd\s/,
];

function checkNoFirewallCommands(label, content) {
  for (const pat of FIREWALL_PATTERNS) {
    if (pat.test(content)) {
      fail(`${label}: contains firewall command — ${pat}`);
      return;
    }
  }
  pass(`${label}: no firewall commands`);
}

// ── Windows static checks ────────────────────────────────────────────────────

function checkWindowsInstallScript() {
  console.log("\n  [Windows install-service.ps1]");
  const path = join(WIN_SCRIPTS, "install-service.ps1");
  const content = readScript(path);
  if (!content) { fail("install-service.ps1 not found"); return; }

  assertContains("install-service.ps1", content,
    /function\s+Format-Argument/i,
    "Format-Argument function present (path quoting helper)");

  assertContains("install-service.ps1", content,
    /Format-Argument.*\$.*InstallDir|Format-Argument.*\$.*ServiceArgs|Format-Argument.*paths\./i,
    "paths passed through Format-Argument (quoted correctly)");

  assertContains("install-service.ps1", content,
    /\[switch\]\$DryRun/i,
    "-DryRun parameter declared");

  assertContains("install-service.ps1", content,
    /if.*DryRun/i,
    "DryRun early-exit block present");

  checkNoFirewallCommands("install-service.ps1", content);
}

function checkWindowsValidateUserScript() {
  console.log("\n  [Windows validate-user-install.ps1]");
  const path = join(WIN_SCRIPTS, "validate-user-install.ps1");
  const content = readScript(path);
  if (!content) { fail("validate-user-install.ps1 not found"); return; }

  assertContains("validate-user-install.ps1", content,
    /TEST_TASK_NAME\s*=\s*["']PortierTest/i,
    "uses test-specific task name (PortierTest*)");

  assertAbsent("validate-user-install.ps1", content,
    /TEST_TASK_NAME\s*=\s*["']Portier["']/i,
    "production task name \"Portier\" as TEST_TASK_NAME");

  assertAbsent("validate-user-install.ps1", content,
    /[^A-Za-z0-9]47831[^0-9]/,
    "hard-coded production port 47831");

  assertContains("validate-user-install.ps1", content,
    /\$env:TEMP|\$env:LocalAppData|\$env:APPDATA/i,
    "uses temp/user-scoped path (not production install dir)");

  assertAbsent("validate-user-install.ps1", content,
    /ProgramFiles.*Portier['"]/i,
    "production install dir %ProgramFiles%\\Portier");

  checkNoFirewallCommands("validate-user-install.ps1", content);
}

function checkWindowsValidateMachineScript() {
  console.log("\n  [Windows validate-machine-service.ps1]");
  const path = join(WIN_SCRIPTS, "validate-machine-service.ps1");
  const content = readScript(path);
  if (!content) { fail("validate-machine-service.ps1 not found"); return; }

  assertContains("validate-machine-service.ps1", content,
    /TEST_SERVICE_NAME\s*=\s*["']PortierTest/i,
    "uses test-specific service name (PortierTest*)");

  assertAbsent("validate-machine-service.ps1", content,
    /TEST_SERVICE_NAME\s*=\s*["']Portier["']/i,
    "production service name \"Portier\" as TEST_SERVICE_NAME");

  assertAbsent("validate-machine-service.ps1", content,
    /[^A-Za-z0-9]47831[^0-9]/,
    "hard-coded production port 47831");

  assertContains("validate-machine-service.ps1", content,
    /\$env:TEMP/i,
    "uses temp path (not production install dir)");

  checkNoFirewallCommands("validate-machine-service.ps1", content);
}

// ── macOS static checks ──────────────────────────────────────────────────────

function checkMacOSInstallScript() {
  console.log("\n  [macOS install-launch-agent.sh]");
  const path = join(MACOS_SCRIPTS, "install-launch-agent.sh");
  const content = readScript(path);
  if (!content) { fail("install-launch-agent.sh not found"); return; }

  assertContains("install-launch-agent.sh", content,
    /--dry-run\)/,
    "--dry-run argument handler present");

  assertContains("install-launch-agent.sh", content,
    /DRY_RUN/,
    "DRY_RUN variable used");

  // Plist generation must not embed bare ~ (launchd doesn't expand ~)
  // Check the plist generation section uses expanded vars not literal ~
  const plistSection = content.slice(content.indexOf("Generate plist") > -1
    ? content.indexOf("Generate plist")
    : content.indexOf("echo '<plist"));
  if (plistSection) {
    assertAbsent("install-launch-agent.sh (plist)", plistSection,
      /<string>~/,
      "bare ~ in plist <string> values (launchd requires absolute paths)");
  }

  assertContains("install-launch-agent.sh", content,
    /\$INSTALL_DIR|\$CONFIG_PATH|\$EXECUTABLE/,
    "uses resolved variable paths (not bare ~)");

  checkNoFirewallCommands("install-launch-agent.sh", content);
}

function checkMacOSValidateScript() {
  console.log("\n  [macOS validate-launch-agent.sh]");
  const path = join(MACOS_SCRIPTS, "validate-launch-agent.sh");
  const content = readScript(path);
  if (!content) { fail("validate-launch-agent.sh not found"); return; }

  assertContains("validate-launch-agent.sh", content,
    /TEST_LABEL=["']com\.portier\.test/,
    "uses test-specific label (com.portier.test)");

  assertAbsent("validate-launch-agent.sh", content,
    /TEST_LABEL=["']com\.portier\.port-forwarding/,
    "production label as TEST_LABEL");

  assertAbsent("validate-launch-agent.sh", content,
    /[^0-9]47831[^0-9]/,
    "hard-coded production port 47831");

  checkNoFirewallCommands("validate-launch-agent.sh", content);
}

// ── Linux static checks ──────────────────────────────────────────────────────

function checkLinuxInstallScript() {
  console.log("\n  [Linux install-service.sh]");
  const path = join(LINUX_SCRIPTS, "install-service.sh");
  const content = readScript(path);
  if (!content) { fail("install-service.sh not found"); return; }

  assertContains("install-service.sh", content,
    /--dry-run\)/,
    "--dry-run argument handler present");

  assertContains("install-service.sh", content,
    /DRY_RUN/,
    "DRY_RUN variable used");

  assertContains("install-service.sh", content,
    /INSTALL_DIR=["']\/opt\/portier["']/,
    "production default INSTALL_DIR=/opt/portier");

  assertContains("install-service.sh", content,
    /CONFIG_PATH=["']\/etc\/portier\/rules\.json["']/,
    "production default CONFIG_PATH=/etc/portier/rules.json");

  assertContains("install-service.sh", content,
    /\$INSTALL_DIR\/service/,
    "ExecStart references $INSTALL_DIR/service binary");

  checkNoFirewallCommands("install-service.sh", content);
}

function checkLinuxValidateScript() {
  console.log("\n  [Linux validate-systemd-service.sh]");
  const path = join(LINUX_SCRIPTS, "validate-systemd-service.sh");
  const content = readScript(path);
  if (!content) { fail("validate-systemd-service.sh not found"); return; }

  assertContains("validate-systemd-service.sh", content,
    /TEST_SERVICE_NAME=["']portier-test["']/,
    "uses test-specific service name (portier-test)");

  assertAbsent("validate-systemd-service.sh", content,
    /TEST_SERVICE_NAME=["']portier["']/,
    "production service name \"portier\" as TEST_SERVICE_NAME");

  assertAbsent("validate-systemd-service.sh", content,
    /[^0-9]47831[^0-9]/,
    "hard-coded production port 47831");

  checkNoFirewallCommands("validate-systemd-service.sh", content);
}

// ── Dynamic dry-run: Windows ─────────────────────────────────────────────────

function runWindowsDryRun() {
  console.log("\n  [Windows -DryRun execution]");
  const scriptPath = join(WIN_SCRIPTS, "install-service.ps1");
  if (!existsSync(scriptPath)) {
    fail("install-service.ps1 not found for dry-run execution");
    return;
  }

  const result = spawnSync(
    "powershell",
    ["-ExecutionPolicy", "Bypass", "-File", scriptPath, "-DryRun"],
    { encoding: "utf8", timeout: 10000 }
  );

  if (result.error) {
    fail(`-DryRun: PowerShell spawn error: ${result.error.message}`);
    return;
  }
  if (result.status !== 0) {
    fail(`-DryRun: exited with code ${result.status}. stderr: ${result.stderr}`);
    return;
  }

  const output = result.stdout;

  function assertOutputContains(pattern, description) {
    const re = pattern instanceof RegExp ? pattern : new RegExp(pattern);
    if (re.test(output)) {
      pass(`-DryRun output: ${description}`);
    } else {
      fail(`-DryRun output: expected ${description} — pattern: ${re}`);
    }
  }

  function assertOutputAbsent(pattern, description) {
    const re = pattern instanceof RegExp ? pattern : new RegExp(pattern);
    if (!re.test(output)) {
      pass(`-DryRun output absent (correct): ${description}`);
    } else {
      fail(`-DryRun output: must NOT contain ${description} — pattern: ${re}`);
    }
  }

  assertOutputContains(/DryRun/i, "DryRun header in output");
  assertOutputContains(/ServiceName\s*:/i, "ServiceName field in output");
  assertOutputContains(/InstallDir\s*:/i, "InstallDir field in output");
  assertOutputContains(/ConfigPath\s*:/i, "ConfigPath field in output");
  assertOutputContains(/StaticDir\s*:/i, "StaticDir field in output");
  assertOutputContains(/Host\s*:/i, "Host field in output");
  assertOutputContains(/Port\s*:/i, "Port field in output");
  assertOutputContains(/Runtime\s*:/i, "Runtime field in output");
  assertOutputContains(/Command\s*:/i, "Command field in output");

  // Must NOT perform actual operations
  assertOutputAbsent(/New-Service/i, "New-Service (not executed in dry-run)");
  assertOutputAbsent(/Register-ScheduledTask/i, "Register-ScheduledTask (not executed in dry-run)");
  assertOutputAbsent(/New-Item/i, "New-Item (not executed in dry-run)");
  assertOutputAbsent(/Copy-Item/i, "Copy-Item (not executed in dry-run)");
}

// ── Dynamic dry-run: macOS ───────────────────────────────────────────────────

function runMacosDryRun() {
  console.log("\n  [macOS --dry-run execution]");
  const scriptPath = join(MACOS_SCRIPTS, "install-launch-agent.sh");
  if (!existsSync(scriptPath)) {
    fail("install-launch-agent.sh not found for dry-run execution");
    return;
  }

  const result = spawnSync("bash", [scriptPath, "--dry-run"], {
    encoding: "utf8",
    timeout: 10000,
  });

  if (result.error) {
    fail(`--dry-run: bash spawn error: ${result.error.message}`);
    return;
  }
  if (result.status !== 0) {
    fail(`--dry-run: exited with code ${result.status}. stderr: ${result.stderr}`);
    return;
  }

  const output = result.stdout;

  function assertOut(pattern, description) {
    const re = pattern instanceof RegExp ? pattern : new RegExp(pattern);
    if (re.test(output)) {
      pass(`--dry-run output: ${description}`);
    } else {
      fail(`--dry-run output: expected ${description} — pattern: ${re}`);
    }
  }

  assertOut(/DryRun/i, "DryRun header");
  assertOut(/Label\s*:/i, "Label field");
  assertOut(/PlistPath\s*:/i, "PlistPath field");
  assertOut(/InstallDir\s*:/i, "InstallDir field");
  assertOut(/ConfigPath\s*:/i, "ConfigPath field");
  assertOut(/Host\s*:/i, "Host field");
  assertOut(/Port\s*:/i, "Port field");
  assertOut(/ProgramArguments/i, "ProgramArguments section");
  // Paths must be absolute (no ~ in plist output)
  assertOut(/\/Users\/|\/home\/|InstallDir.*\/Applications/i, "absolute paths in dry-run output");
}

// ── Dynamic dry-run: Linux ───────────────────────────────────────────────────

function runLinuxDryRun() {
  console.log("\n  [Linux --dry-run execution]");
  const scriptPath = join(LINUX_SCRIPTS, "install-service.sh");
  if (!existsSync(scriptPath)) {
    fail("install-service.sh not found for dry-run execution");
    return;
  }

  const result = spawnSync("bash", [scriptPath, "--dry-run"], {
    encoding: "utf8",
    timeout: 10000,
  });

  if (result.error) {
    fail(`--dry-run: bash spawn error: ${result.error.message}`);
    return;
  }
  if (result.status !== 0) {
    fail(`--dry-run: exited with code ${result.status}. stderr: ${result.stderr}`);
    return;
  }

  const output = result.stdout;

  function assertOut(pattern, description) {
    const re = pattern instanceof RegExp ? pattern : new RegExp(pattern);
    if (re.test(output)) {
      pass(`--dry-run output: ${description}`);
    } else {
      fail(`--dry-run output: expected ${description} — pattern: ${re}`);
    }
  }

  assertOut(/DryRun/i, "DryRun header");
  assertOut(/InstallDir\s*:.*\/opt\/portier/i, "default InstallDir=/opt/portier");
  assertOut(/ConfigPath\s*:.*\/etc\/portier\/rules\.json/i, "default ConfigPath=/etc/portier/rules.json");
  assertOut(/ServiceUnit\s*:/i, "ServiceUnit field");
  assertOut(/ServiceName\s*:/i, "ServiceName field");
  assertOut(/ExecStart\s*:/i, "ExecStart field");
  assertOut(/\/opt\/portier\/service/i, "ExecStart references /opt/portier/service");
}

// ── Main ─────────────────────────────────────────────────────────────────────

function main() {
  console.log("[validate:scripts] Installer script static analysis and dry-run tests\n");

  // Static analysis — always run on all platforms
  console.log("Static analysis:\n");
  checkWindowsInstallScript();
  checkWindowsValidateUserScript();
  checkWindowsValidateMachineScript();
  checkMacOSInstallScript();
  checkMacOSValidateScript();
  checkLinuxInstallScript();
  checkLinuxValidateScript();

  // Dynamic dry-run — current platform only
  console.log("\nDry-run execution:\n");
  if (platform === "win32") {
    runWindowsDryRun();
    skip("macOS dry-run — only on macOS");
    skip("Linux dry-run — only on Linux");
  } else if (platform === "darwin") {
    skip("Windows dry-run — only on Windows");
    runMacosDryRun();
    skip("Linux dry-run — only on Linux");
  } else if (platform === "linux") {
    skip("Windows dry-run — only on Windows");
    skip("macOS dry-run — only on macOS");
    runLinuxDryRun();
  } else {
    skip(`Windows dry-run — only on Windows (current: ${platform})`);
    skip(`macOS dry-run — only on macOS (current: ${platform})`);
    skip(`Linux dry-run — only on Linux (current: ${platform})`);
  }

  console.log(`\n[validate:scripts] ${passed} passed, ${skipped} skipped, ${failed} failed.\n`);
  if (failed > 0) {
    console.error("[validate:scripts] FAILED.\n");
    process.exit(1);
  }
  console.log("[validate:scripts] All script tests passed.\n");
}

main();
