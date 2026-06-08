#!/usr/bin/env node
/* global console, process */
/**
 * Go service coverage reporter for service/.
 *
 * Runs `go test` with cross-package coverage instrumentation using sequential
 * (-p 1) execution to avoid timing flakiness from parallel package runs, prints
 * the total statement coverage, and writes coverage/service/coverage-summary.json
 * in the same format as vitest's json-summary reporter.
 *
 * Statement counts are derived from the coverage profile. Function coverage
 * is derived from `go tool cover -func` per-function output (a function is
 * counted as covered if any of its statements were executed). Branch coverage
 * is not available from standard Go tooling and is left as "Unknown".
 *
 * Does not enforce a gate — use this for baseline reporting only.
 * To add a gate, add the component to the GATES table in scripts/validate-coverage.js.
 *
 * Usage:
 *   node scripts/coverage-service.js
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
const serviceDir = join(repoRoot, "service");

if (!existsSync(serviceDir)) {
  console.error(`[coverage-service] Service directory not found: ${serviceDir}`);
  process.exit(1);
}

const coverDir = join(repoRoot, "coverage");
if (!existsSync(coverDir)) mkdirSync(coverDir, { recursive: true });

const coverProfile = join(coverDir, `service-coverage-${randomUUID()}.out`);

console.log("[coverage-service] Running service tests with coverage (sequential, -p 1)...\n");

const testResult = spawnSync(
  "go",
  [
    "test", "-count=1", "-p", "1", "-timeout", "120s",
    `-coverprofile=${coverProfile}`, "-coverpkg=./sources/...", "./sources/...",
  ],
  { cwd: serviceDir, stdio: "inherit", encoding: "utf8" }
);

if ((testResult.status ?? 1) !== 0) {
  try { rmSync(coverProfile); } catch { /* best-effort */ }
  console.error("\n[coverage-service] FAILED: go test returned non-zero exit code.");
  process.exit(1);
}

const { stmtTotal, stmtCovered } = parseProfile(coverProfile);

const coverResult = spawnSync(
  "go",
  ["tool", "cover", `-func=${coverProfile}`],
  { cwd: serviceDir, stdio: ["inherit", "pipe", "inherit"], encoding: "utf8" }
);

try { rmSync(coverProfile); } catch { /* best-effort */ }

if ((coverResult.status ?? 1) !== 0) {
  console.error("[coverage-service] FAILED: go tool cover returned non-zero exit code.");
  process.exit(1);
}

const coverOutput = coverResult.stdout || "";
const totalLine = coverOutput.split("\n").find((l) => l.startsWith("total:"));
if (!totalLine) {
  console.error("[coverage-service] FAILED: could not find total coverage line.");
  process.exit(1);
}

const pctMatch = totalLine.match(/(\d+\.\d+)%/);
if (!pctMatch) {
  console.error(`[coverage-service] FAILED: could not parse total% from: ${totalLine}`);
  process.exit(1);
}

const stmtPct = parseFloat(pctMatch[1]);
const { funcTotal, funcCovered } = parseFunctions(coverOutput);

writeSummary(join(coverDir, "service"), stmtTotal, stmtCovered, stmtPct, funcTotal, funcCovered);

console.log(`\n[coverage-service] Total service coverage: ${stmtPct}% (statements, cross-package)\n`);

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
