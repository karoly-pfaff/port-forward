#!/usr/bin/env node
/* global console, process */
/**
 * Go replay coverage reporter for tools/replay/.
 *
 * Runs the replay module's tests with module-wide cross-package coverage
 * instrumentation (equivalent to `go -C tools/replay test -coverpkg=./... ./...`),
 * prints the total statement coverage, and writes coverage/replay/coverage-summary.json
 * in the same json-summary format as scripts/coverage-tools-cli.js.
 *
 * The replay tool is a separate Go module (portier/replay), so its coverage is
 * gathered module-wide here; the module-wide total includes the thin main()/os.Exit
 * wrapper (0% by nature) with no per-function exception applied.
 *
 * Does not enforce a gate — use this for reporting only. The hard 95% replay gate
 * is enforced by scripts/validate-coverage.js (component "replay"), the same place
 * every other component gate lives. Does not depend on, run, or alter the CLI
 * module or its coverage gates.
 *
 * Usage:
 *   node scripts/coverage-tools-replay.js
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
const replayDir = join(repoRoot, "tools", "replay");

if (!existsSync(replayDir)) {
  console.error(`[coverage-tools-replay] replay directory not found: ${replayDir}`);
  process.exit(1);
}

const coverDir = join(repoRoot, "coverage");
if (!existsSync(coverDir)) mkdirSync(coverDir, { recursive: true });

const coverProfile = join(coverDir, `replay-coverage-${randomUUID()}.out`);

console.log("[coverage-tools-replay] Running replay tests with module-wide coverage...\n");

const testResult = spawnSync(
  "go",
  ["test", "-count=1", `-coverprofile=${coverProfile}`, "-coverpkg=./...", "./..."],
  { cwd: replayDir, stdio: "inherit", encoding: "utf8" }
);

if ((testResult.status ?? 1) !== 0) {
  try { rmSync(coverProfile); } catch { /* best-effort */ }
  console.error("\n[coverage-tools-replay] FAILED: go test returned non-zero exit code.");
  process.exit(1);
}

const { stmtTotal, stmtCovered } = parseProfile(coverProfile);

const coverResult = spawnSync(
  "go",
  ["tool", "cover", `-func=${coverProfile}`],
  { cwd: replayDir, stdio: ["inherit", "pipe", "inherit"], encoding: "utf8" }
);

try { rmSync(coverProfile); } catch { /* best-effort */ }

if ((coverResult.status ?? 1) !== 0) {
  console.error("[coverage-tools-replay] FAILED: go tool cover returned non-zero exit code.");
  process.exit(1);
}

const coverOutput = coverResult.stdout || "";
const totalLine = coverOutput.split("\n").find((l) => l.startsWith("total:"));
if (!totalLine) {
  console.error("[coverage-tools-replay] FAILED: could not find total coverage line.");
  process.exit(1);
}

const pctMatch = totalLine.match(/(\d+\.\d+)%/);
if (!pctMatch) {
  console.error(`[coverage-tools-replay] FAILED: could not parse total% from: ${totalLine}`);
  process.exit(1);
}

const stmtPct = parseFloat(pctMatch[1]);
const { funcTotal, funcCovered } = parseFunctions(coverOutput);

writeSummary(join(coverDir, "replay"), stmtTotal, stmtCovered, stmtPct, funcTotal, funcCovered);

console.log(`\n[coverage-tools-replay] Total replay coverage: ${stmtPct}% (statements, module-wide)\n`);

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
