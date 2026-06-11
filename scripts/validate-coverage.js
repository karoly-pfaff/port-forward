#!/usr/bin/env node
/* global console, process */
/**
 * Aggregate coverage validator for all Portier components.
 *
 * Runs coverage for every component via npm scripts, then reads the
 * coverage-summary.json that each script writes to coverage/{component}/.
 * All five components — shared, server, client (TypeScript vitest) and
 * service, cli (Go) — produce the same json-summary format after running
 * their respective coverage scripts.
 *
 * Usage:
 *   node scripts/validate-coverage.js [--only <component>]
 *
 * Options:
 *   --only <component>   Run and check a single component only.
 *                        Valid values: shared, server, client, service, cli
 *
 * Exit codes:
 *   0  All gates passed.
 *   1  One or more gates failed, or a coverage run failed.
 *
 * Gates (update here when ratcheting):
 *   Each gate is an object: { statements?, branches?, functions? }
 *   All specified thresholds must pass for the component to PASS.
 *   Go components (service, cli) report statements and functions only;
 *   branches are always "Unknown" from standard Go tooling.
 *
 *   Ratcheted at v1.5.0 (2026-06-09). Ratchet upward as coverage improves.
 *
 * Known untestable CLI branches (documented here, not counted against coverage):
 *   - main() calls os.Exit() — cannot be tested without spawning a subprocess;
 *     the run() function it delegates to IS tested directly.
 *   - http.NewRequest() error in client.do() / client.doWithBody() — requires an
 *     invalid method string containing a newline, which Go rejects at compile time
 *     for string literals.
 *   - json.Marshal() error in client.doWithBody() — requires a value that cannot
 *     be marshalled; none of the API request types contain such values.
 *   - json.MarshalIndent() error in config.writePrettyJSON() — same reason.
 *   - json.NewEncoder(stdout).Encode() errors in RunRuntime, RunList, RunStart,
 *     RunStop, and related commands — require a broken stdout writer; not possible
 *     in unit tests without replacing os.Stdout.
 *   - Repeated validateURL-error branches across commands — structurally identical
 *     to the runtime branch that IS tested; exercising all copies adds noise.
 */

import { existsSync, mkdirSync, readFileSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const coverDir = join(repoRoot, "coverage");

// ── Gates ─────────────────────────────────────────────────────────────────────
// Object: { statements?, branches?, functions? } — per-metric gates.
// All specified thresholds must pass for the component to PASS.
// Go components (service, cli): branches are "Unknown"; only gate statements.
//
// Ratcheted at v1.6-pre (2026-06-09). Tooling stabilized at v1.6 Slice A (2026-06-09):
//   - Windows vitest/v8 drive-letter deduplication added to readSummary()
//   - Structural-zero files excluded from vitest coverage configs
//   - Client branch/funcs gates recalibrated from pre-Slice-A values (90/79) to
//     accurate post-deduplication actuals (89/78).  Previous gates were set from a
//     run where ghost entries were absent; now that deduplication is always applied,
//     the true values are consistently 89.6% branch and 78.6% funcs.
// Actuals after Slice A (accurate): shared 100/100/100, server 95.2/91.6/100,
//   client 95.6/89.6/78.6, service 87.7, cli 93.2
// Actuals after Slice B: service 88.6% (gate raised 87→88).
// Actuals after Coverage Slice C: cli 97.7% (gate raised 93→95; deterministic
//   cross-package Go number, repeated runs stable). See docs/coverage-baseline.md.
// Actuals after Coverage Slice D: service 90.3% (gate raised 88→90; two
//   consecutive coverage:service runs both 90.3%, modified packages stable at
//   -count=3). See docs/coverage-baseline.md.
// Actuals after Coverage Slice E: server 98.9/93.57/100 (gates raised
//   89/91/99 → 95/92/99; three consecutive coverage:server runs stable, the
//   ghost-entry dedup keeps the number deterministic). See docs/coverage-baseline.md.
const GATES = {
  shared:  { statements: 100, branches: 100, functions: 100 },
  server:  { statements: 95, branches: 92, functions: 99 },
  client:  { statements: 94, branches: 89, functions: 78 },
  service: { statements: 90 },
  cli:     { statements: 95 },
};

const ALL_COMPONENTS = Object.keys(GATES);

// ── Argument parsing ──────────────────────────────────────────────────────────

const args = process.argv.slice(2);
const onlyIdx = args.indexOf("--only");
const onlyComponent = onlyIdx >= 0 ? args[onlyIdx + 1] : null;

if (onlyComponent !== null && !ALL_COMPONENTS.includes(onlyComponent)) {
  console.error(
    `[validate-coverage] Unknown component: "${onlyComponent}". ` +
    `Valid values: ${ALL_COMPONENTS.join(", ")}`
  );
  process.exit(1);
}

const activeComponents = onlyComponent ? [onlyComponent] : ALL_COMPONENTS;

// ── Helpers ───────────────────────────────────────────────────────────────────

function ensureCoverDir() {
  if (!existsSync(coverDir)) mkdirSync(coverDir, { recursive: true });
}

/** Run an npm workspace coverage command; show output live; return ok/fail. */
function runNpmCoverage(workspace) {
  console.log(`\n${"─".repeat(60)}`);
  console.log(`[validate-coverage] Running coverage:${workspace}...`);
  console.log(`${"─".repeat(60)}\n`);

  const result = spawnSync(
    "npm",
    ["run", `coverage:${workspace}`],
    { cwd: repoRoot, stdio: "inherit", encoding: "utf8", shell: true }
  );

  return (result.status ?? 1) === 0;
}

/**
 * Normalize a Windows path's drive letter to lowercase.
 * "C:\Users\..." → "c:\Users\..." — no-op on non-Windows paths.
 *
 * Background: vitest + v8 coverage on Windows can write two entries for the same
 * physical file — one with the lowercase drive letter produced by Node.js file-URL
 * resolution (the real execution data) and one with an uppercase drive letter from
 * the coverage.include glob resolution (always 0%).  When both appear, the statement
 * denominator is doubled and aggregate coverage collapses by ~50%.  Normalizing
 * before aggregation lets us detect and deduplicate these ghost entries.
 */
function normalizePath(p) {
  return p.replace(/^([A-Z]):/, (_, d) => d.toLowerCase() + ":");
}

/**
 * Read statements/branches/functions from a coverage-summary.json.
 *
 * Go coverage scripts (service, cli) write only a "total" key — those are read
 * directly.  TypeScript workspaces (vitest + v8) write per-file entries alongside
 * "total".  For TypeScript workspaces we recompute totals from deduplicated per-file
 * entries to eliminate Windows drive-letter ghost duplicates.
 *
 * Returns null if the file is missing or unparseable.
 */
function readSummary(workspace) {
  const summaryPath = join(coverDir, workspace, "coverage-summary.json");
  if (!existsSync(summaryPath)) {
    console.error(`[validate-coverage] Summary not found: ${summaryPath}`);
    return null;
  }
  try {
    const raw = JSON.parse(readFileSync(summaryPath, "utf8"));

    // Go coverage scripts write only a "total" key; use it directly.
    const fileKeys = Object.keys(raw).filter(k => k !== "total");
    if (fileKeys.length === 0) {
      return {
        statements: toNum(raw.total?.statements?.pct),
        branches:   toNum(raw.total?.branches?.pct),
        functions:  toNum(raw.total?.functions?.pct),
      };
    }

    // TypeScript path: deduplicate entries that differ only by drive-letter case,
    // then recompute totals so ghost zero-coverage entries don't inflate the denominator.
    // When two entries map to the same normalized path, keep the one with more
    // covered statements — that is the real execution entry, not the ghost.
    const deduped = new Map();
    for (const key of fileKeys) {
      const entry = raw[key];
      const canonical = normalizePath(key);
      const existing = deduped.get(canonical);
      if (!existing) {
        deduped.set(canonical, entry);
      } else {
        const existingCovered = existing.statements?.covered ?? 0;
        const newCovered      = entry.statements?.covered    ?? 0;
        if (newCovered > existingCovered) {
          deduped.set(canonical, entry);
        }
      }
    }

    let stmtTotal = 0,   stmtCovered = 0;
    let branchTotal = 0, branchCovered = 0;
    let funcTotal = 0,   funcCovered = 0;

    for (const entry of deduped.values()) {
      stmtTotal     += entry.statements?.total   ?? 0;
      stmtCovered   += entry.statements?.covered ?? 0;
      branchTotal   += entry.branches?.total     ?? 0;
      branchCovered += entry.branches?.covered   ?? 0;
      funcTotal     += entry.functions?.total    ?? 0;
      funcCovered   += entry.functions?.covered  ?? 0;
    }

    return {
      statements: stmtTotal   > 0 ? (stmtCovered   / stmtTotal)   * 100 : 100,
      branches:   branchTotal > 0 ? (branchCovered / branchTotal) * 100 : null,
      functions:  funcTotal   > 0 ? (funcCovered   / funcTotal)   * 100 : null,
    };
  } catch (e) {
    console.error(`[validate-coverage] Failed to parse ${summaryPath}: ${e.message}`);
    return null;
  }
}

/** Return v if it is a finite number, otherwise null. */
function toNum(v) {
  return typeof v === "number" && isFinite(v) ? v : null;
}

/**
 * Check a gate object against measured data.
 * Returns { passed: bool, failures: string[] }.
 */
function checkGate(gate, data) {
  const failures = [];
  if (gate.statements != null && data.statements < gate.statements) {
    failures.push(`stmts(${data.statements.toFixed(1)}%<${gate.statements}%)`);
  }
  if (gate.branches != null && data.branches != null && data.branches < gate.branches) {
    failures.push(`branch(${data.branches.toFixed(1)}%<${gate.branches}%)`);
  }
  if (gate.functions != null && data.functions != null && data.functions < gate.functions) {
    failures.push(`funcs(${data.functions.toFixed(1)}%<${gate.functions}%)`);
  }
  return { passed: failures.length === 0, failures };
}

/**
 * Format a gate object for display in the summary table.
 * Single-metric gates (service/cli) show "82%" — multi-metric show "82/54/90".
 */
function fmtGate(gate) {
  const parts = [
    gate.statements != null ? String(gate.statements) : null,
    gate.branches   != null ? String(gate.branches)   : null,
    gate.functions  != null ? String(gate.functions)  : null,
  ].filter(Boolean);
  if (parts.length === 1) return `${parts[0]}%`;
  return parts.join("/");
}

// ── Run coverage for active components ───────────────────────────────────────

ensureCoverDir();

const results = {};
let anyRunFailed = false;

for (const component of activeComponents) {
  const ok = runNpmCoverage(component);
  if (!ok) {
    console.error(`[validate-coverage] Coverage run failed for ${component}.`);
    anyRunFailed = true;
    results[component] = null;
    continue;
  }
  const data = readSummary(component);
  if (!data) anyRunFailed = true;
  results[component] = data;
}

// ── Summary table ─────────────────────────────────────────────────────────────

const W = 72;
console.log(`\n${"═".repeat(W)}`);
console.log(" Coverage Summary");
console.log(`${"═".repeat(W)}`);
console.log(
  " Component".padEnd(12) +
  "Statements".padStart(12) +
  "  Branch".padStart(10) +
  "  Functions".padStart(12) +
  "  Gate".padStart(12) +
  "  Status".padStart(14)
);
console.log(`${"─".repeat(W)}`);

let anyGateFailed = false;

for (const [component, data] of Object.entries(results)) {
  const gate = GATES[component];
  let status = "—";

  if (data === null) {
    status = "FAILED";
    anyGateFailed = true;
  } else {
    const { passed, failures } = checkGate(gate, data);
    if (!passed) {
      status = `FAIL`;
      anyGateFailed = true;
      for (const f of failures) {
        console.error(`[validate-coverage]   ${component}: ${f}`);
      }
    } else {
      status = "PASS";
    }
  }

  const stmts    = data         ? `${data.statements.toFixed(1)}%` : "—";
  const branches = data?.branches  != null ? `${data.branches.toFixed(1)}%`  : "—";
  const funcs    = data?.functions != null ? `${data.functions.toFixed(1)}%` : "—";
  const gateStr  = fmtGate(gate);

  console.log(
    ` ${component}`.padEnd(12) +
    stmts.padStart(12) +
    branches.padStart(10) +
    funcs.padStart(12) +
    gateStr.padStart(12) +
    `  ${status}`.padStart(14)
  );
}

console.log(`${"─".repeat(W)}`);
console.log(" Gate format: s/b/f = statements / branches / functions thresholds");
console.log(`${"═".repeat(W)}\n`);

if (anyRunFailed || anyGateFailed) {
  if (anyRunFailed) {
    console.error("[validate-coverage] One or more coverage runs failed. See output above.");
  }
  if (anyGateFailed) {
    console.error("[validate-coverage] One or more coverage gates failed. See table above.");
  }
  process.exit(1);
}

console.log("[validate-coverage] All gates passed.\n");
