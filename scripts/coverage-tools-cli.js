#!/usr/bin/env node
/* global console, process */
/**
 * Go CLI coverage reporter for tools/cli/.
 *
 * Runs `go test` with cross-package coverage instrumentation, prints the
 * total statement coverage, and writes coverage/cli/coverage-summary.json
 * in the same format as vitest's json-summary reporter.
 *
 * Statement counts are derived from the coverage profile. Function coverage
 * is derived from `go tool cover -func` per-function output (a function is
 * counted as covered if any of its statements were executed). Branch coverage
 * is not available from standard Go tooling and is left as "Unknown".
 *
 * Does not enforce a gate — use this for reporting only.
 * The gate is enforced by scripts/validate-coverage.js.
 *
 * Usage:
 *   node scripts/coverage-tools-cli.js
 *
 * Exit codes:
 *   0  Tests passed and coverage summary was written.
 *   1  Tests failed or coverage could not be parsed.
 */

import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { randomUUID } from "node:crypto";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const cliDir = join(repoRoot, "tools", "cli");

if (!existsSync(cliDir)) {
  console.error(`[coverage-tools-cli] CLI directory not found: ${cliDir}`);
  process.exit(1);
}

const coverDir = join(repoRoot, "coverage");
if (!existsSync(coverDir)) mkdirSync(coverDir, { recursive: true });

const coverProfile = join(coverDir, `cli-coverage-${randomUUID()}.out`);

// Packages to instrument (excludes version/ — no Go files to cover).
const coverpkg = [
  "portier/cli/sources",
  "portier/cli/sources/client",
  "portier/cli/sources/commands",
  "portier/cli/sources/output",
].join(",");

console.log("[coverage-tools-cli] Running CLI tests with coverage...\n");

const testResult = spawnSync(
  "go",
  ["test", "-count=1", `-coverprofile=${coverProfile}`, `-coverpkg=${coverpkg}`, "./sources/..."],
  { cwd: cliDir, stdio: "inherit", encoding: "utf8" }
);

if ((testResult.status ?? 1) !== 0) {
  try { rmSync(coverProfile); } catch { /* best-effort */ }
  console.error("\n[coverage-tools-cli] FAILED: go test returned non-zero exit code.");
  process.exit(1);
}

const { stmtTotal, stmtCovered } = parseProfile(coverProfile);

const coverResult = spawnSync(
  "go",
  ["tool", "cover", `-func=${coverProfile}`],
  { cwd: cliDir, stdio: ["inherit", "pipe", "inherit"], encoding: "utf8" }
);

try { rmSync(coverProfile); } catch { /* best-effort */ }

if ((coverResult.status ?? 1) !== 0) {
  console.error("[coverage-tools-cli] FAILED: go tool cover returned non-zero exit code.");
  process.exit(1);
}

const coverOutput = coverResult.stdout || "";
const totalLine = coverOutput.split("\n").find((l) => l.startsWith("total:"));
if (!totalLine) {
  console.error("[coverage-tools-cli] FAILED: could not find total coverage line.");
  process.exit(1);
}

const pctMatch = totalLine.match(/(\d+\.\d+)%/);
if (!pctMatch) {
  console.error(`[coverage-tools-cli] FAILED: could not parse total% from: ${totalLine}`);
  process.exit(1);
}

const stmtPct = parseFloat(pctMatch[1]);
const { funcTotal, funcCovered } = parseFunctions(coverOutput);

writeSummary(join(coverDir, "cli"), stmtTotal, stmtCovered, stmtPct, funcTotal, funcCovered);

console.log(`\n[coverage-tools-cli] Total CLI coverage: ${stmtPct}% (statements, cross-package)\n`);

// ── Helpers ───────────────────────────────────────────────────────────────────

function parseProfile(profilePath) {
  const lines = readFileSync(profilePath, "utf8").trim().split("\n").slice(1); // skip "mode:" line
  let total = 0, covered = 0;
  for (const line of lines) {
    const parts = line.split(" ");
    if (parts.length < 3) continue;
    const numStmts = parseInt(parts[1], 10);
    const count = parseInt(parts[2], 10);
    total += numStmts;
    if (count > 0) covered += numStmts;
  }
  return { stmtTotal: total, stmtCovered: covered };
}

function parseFunctions(funcOutput) {
  let total = 0, covered = 0;
  for (const line of funcOutput.split("\n")) {
    if (!line.trim() || line.startsWith("total:")) continue;
    const m = line.match(/(\d+\.\d+)%\s*$/);
    if (!m) continue;
    total++;
    if (parseFloat(m[1]) > 0) covered++;
  }
  return { funcTotal: total, funcCovered: covered };
}

function writeSummary(dir, stmtTotal, stmtCovered, stmtPct, funcTotal, funcCovered) {
  const funcPct = funcTotal > 0 ? Math.round((funcCovered / funcTotal) * 1000) / 10 : 0;
  const summary = {
    total: {
      lines:        { total: stmtTotal, covered: stmtCovered, skipped: 0, pct: stmtPct },
      statements:   { total: stmtTotal, covered: stmtCovered, skipped: 0, pct: stmtPct },
      functions:    { total: funcTotal,  covered: funcCovered,  skipped: 0, pct: funcPct },
      branches:     { total: 0, covered: 0, skipped: 0, pct: "Unknown" },
      branchesTrue: { total: 0, covered: 0, skipped: 0, pct: "Unknown" },
    },
  };
  if (!existsSync(dir)) mkdirSync(dir, { recursive: true });
  writeFileSync(join(dir, "coverage-summary.json"), JSON.stringify(summary, null, 2));
}
