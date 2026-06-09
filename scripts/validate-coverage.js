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
 *   Set at v1.4.0 (2026-06-09). Ratchet upward as coverage improves.
 *
 * Known untestable CLI branches (documented here, not counted against coverage):
 *   - main() calls os.Exit() — cannot be tested without spawning a subprocess;
 *     the run() function it delegates to IS tested directly.
 *   - http.NewRequest() error in client.do() / client.doWithBody() — requires an
 *     invalid method string containing a newline, which Go rejects at compile time
 *     for string literals.
 *   - json.Marshal() error in client.doWithBody() — requires a value that cannot
 *     be marshalled; none of the API request types contain such values.
 *   - json.MarshalIndent() error in configcmd.writePrettyJSON() — same reason.
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
// Set at v1.4.0 release (2026-06-09). Raise these when coverage improves.
const GATES = {
  shared:  { statements: 82, branches: 54, functions: 90 },
  server:  { statements: 82, branches: 86, functions: 97 },
  client:  { statements: 90, branches: 89, functions: 76 },
  service: { statements: 82 },
  cli:     { statements: 92 },
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
 * Read statements/branches/functions from a coverage-summary.json.
 * Works for both vitest (numbers) and Go (statements+functions are numbers,
 * branches may be "Unknown"). Returns null if the file is missing or unparseable.
 */
function readSummary(workspace) {
  const summaryPath = join(coverDir, workspace, "coverage-summary.json");
  if (!existsSync(summaryPath)) {
    console.error(`[validate-coverage] Summary not found: ${summaryPath}`);
    return null;
  }
  try {
    const data = JSON.parse(readFileSync(summaryPath, "utf8"));
    return {
      statements: toNum(data.total?.statements?.pct),
      branches:   toNum(data.total?.branches?.pct),
      functions:  toNum(data.total?.functions?.pct),
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
