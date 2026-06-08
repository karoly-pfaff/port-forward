#!/usr/bin/env node
/* global console, process */
/**
 * CLI test coverage gate for tools/cli/.
 *
 * Runs `go test` with cross-package coverage instrumentation, then checks
 * that the total statement coverage meets the minimum threshold.
 *
 * Usage:
 *   node scripts/validate-cli-coverage.js [--threshold <pct>]
 *
 * Options:
 *   --threshold <pct>   Minimum coverage percentage (default: 88).
 *
 * Exit codes:
 *   0  Coverage meets the threshold.
 *   1  Coverage is below the threshold or the test run failed.
 *
 * Known untestable branches (documented here, not counted against coverage):
 *   - main() entry point calls os.Exit() — cannot be tested without spawning a
 *     subprocess; the run() function it delegates to IS tested directly.
 *   - http.NewRequest() error in client.do() / client.doWithBody() — requires an
 *     invalid method string containing a newline, which the Go stdlib rejects at
 *     compile time for string literals.
 *   - json.Marshal() error in client.doWithBody() — requires a value that cannot
 *     be marshalled (e.g. a channel); none of the API request types contain such
 *     values.
 *   - json.MarshalIndent() error in configcmd.writePrettyJSON() — same reason as
 *     above; the config struct is fully marshallable.
 *   - json.NewEncoder(stdout).Encode() errors in RunRuntime, RunList, RunStart,
 *     RunStop, and related commands — require a broken stdout writer; not possible
 *     in unit tests without replacing os.Stdout.
 *   - Repeated validateURL-error branches in run() for list/status/activity/start/
 *     stop/diagnose/config/diagnostics — structurally identical to the runtime
 *     branch that IS tested; exercising all 9 copies adds noise without insight.
 */

import { existsSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { randomUUID } from "node:crypto";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const cliDir = join(repoRoot, "tools", "cli");
const isWindows = process.platform === "win32";

// ── Parse arguments ───────────────────────────────────────────────────────────

const rawArgs = process.argv.slice(2);
const flagValue = (f) => {
  const i = rawArgs.indexOf(f);
  return i >= 0 && i + 1 < rawArgs.length ? rawArgs[i + 1] : null;
};

const thresholdArg = flagValue("--threshold");
const threshold = thresholdArg !== null ? parseFloat(thresholdArg) : 92;

if (isNaN(threshold) || threshold < 0 || threshold > 100) {
  console.error(`[validate-cli-coverage] Invalid --threshold value: ${thresholdArg}`);
  process.exit(1);
}

// ── Paths ─────────────────────────────────────────────────────────────────────

if (!existsSync(cliDir)) {
  console.error(`[validate-cli-coverage] CLI directory not found: ${cliDir}`);
  process.exit(1);
}

const buildDir = join(cliDir, "build");
const coverProfile = join(buildDir, `coverage-${randomUUID()}.out`);

// Ensure build dir exists (go build creates it, but coverage might run before build)
spawnSync(isWindows ? "cmd" : "sh",
  isWindows
    ? ["/c", `if not exist "${buildDir}" mkdir "${buildDir}"`]
    : ["-c", `mkdir -p "${buildDir}"`],
  { stdio: "inherit" }
);

// ── Package list ──────────────────────────────────────────────────────────────

// Enumerate Go packages to instrument. Excludes version/ (no Go files to cover).
const coverpkg = [
  "portier/cli/sources",
  "portier/cli/sources/client",
  "portier/cli/sources/commands",
  "portier/cli/sources/output",
].join(",");

// ── Run tests with coverage ───────────────────────────────────────────────────

console.log(`[validate-cli-coverage] Running CLI tests with coverage (threshold: ${threshold}%)...\n`);

const testResult = spawnSync(
  "go",
  [
    "test",
    "-count=1",
    `-coverprofile=${coverProfile}`,
    `-coverpkg=${coverpkg}`,
    "./sources/...",
  ],
  {
    cwd: cliDir,
    stdio: "inherit",
    encoding: "utf8",
  }
);

if ((testResult.status ?? 1) !== 0) {
  console.error("\n[validate-cli-coverage] FAILED: go test returned non-zero exit code.");
  process.exit(1);
}

// ── Parse coverage ────────────────────────────────────────────────────────────

const coverResult = spawnSync(
  "go",
  ["tool", "cover", `-func=${coverProfile}`],
  {
    cwd: cliDir,
    stdio: ["inherit", "pipe", "inherit"],
    encoding: "utf8",
  }
);

if ((coverResult.status ?? 1) !== 0) {
  console.error("[validate-cli-coverage] FAILED: go tool cover returned non-zero exit code.");
  process.exit(1);
}

const coverOutput = coverResult.stdout || "";
const totalLine = coverOutput
  .split("\n")
  .find((l) => l.startsWith("total:"));

if (!totalLine) {
  console.error("[validate-cli-coverage] FAILED: could not find total coverage line.");
  console.error("Coverage output:\n", coverOutput);
  process.exit(1);
}

// Format: "total:                                (statements)    90.1%"
const match = totalLine.match(/(\d+\.\d+)%/);
if (!match) {
  console.error(`[validate-cli-coverage] FAILED: could not parse total% from: ${totalLine}`);
  process.exit(1);
}

const actual = parseFloat(match[1]);

console.log(`\n[validate-cli-coverage] Total coverage: ${actual}% (threshold: ${threshold}%)`);

if (actual < threshold) {
  console.error(
    `[validate-cli-coverage] FAILED: ${actual}% < ${threshold}% threshold.\n` +
    `  Add tests to cover the missing branches.\n` +
    `  See comments in this script for genuinely untestable branches.`
  );
  process.exit(1);
}

console.log(`[validate-cli-coverage] Coverage gate passed (${actual}% >= ${threshold}%).\n`);
