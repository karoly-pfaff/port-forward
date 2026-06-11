#!/usr/bin/env node
/* global console, process */
/**
 * Go service test + coverage runner for service/.
 *
 * Two modes:
 *   node scripts/coverage-service.js              coverage run + coverage/service/
 *                                                 coverage-summary.json (vitest json-summary shape)
 *   node scripts/coverage-service.js --test-only  plain test run, no coverage (used by `test:service`)
 *
 * Both modes COMPILE the test binaries to a STABLE path (build/tests/<pkg>.test.exe)
 * with `go test -c` and then RUN them from there, instead of letting `go test`
 * link a fresh binary into a per-invocation temp work dir and run it from there.
 *
 * WHY (Windows Firewall): some service tests genuinely bind 0.0.0.0 (e.g. the
 * diagnose lan-exposure test), which triggers a Windows Firewall prompt. The
 * firewall rule is keyed by EXECUTABLE PATH. Plain `go test [-o dir] ./...` runs
 * the test binary from `…\Temp\go-build<random>\b001\<pkg>.test.exe`, a path that
 * changes every invocation — so every run re-prompts and accumulates another
 * firewall rule. The historical `-o build/tests/` flag did NOT fix this: `-o`
 * only COPIES the linked binary afterward; the binary that actually runs (and
 * binds the socket) is still the temp one. `go test -c` compiles WITHOUT running,
 * so executing the compiled binary ourselves from build/tests/ gives each
 * package's test binary one stable path → one persistent "allow" decision.
 *
 * Running the binaries directly is safe because the service tests use t.TempDir()
 * exclusively (no testdata / no cwd-relative reads); we still set each binary's
 * cwd to its package source dir to match `go test` semantics. Binaries run
 * sequentially (one at a time), preserving the coverage path's prior `-p 1`
 * sequential intent and reducing EADDRINUSE risk for socket-binding tests.
 *
 * Statement counts are derived from the merged coverage profile. Function
 * coverage is derived from `go tool cover -func` per-function output (a function
 * is counted as covered if any of its statements were executed). Branch coverage
 * is not available from standard Go tooling and is left as "Unknown".
 *
 * Does not enforce a gate — use this for reporting only. To add a gate, add the
 * component to the GATES table in scripts/validate-coverage.js.
 *
 * Exit codes:
 *   0  Tests passed (and coverage summary written, in coverage mode).
 *   1  Compilation failed, a test package failed, or coverage could not be parsed.
 */

import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve, dirname, sep } from "node:path";
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

const testOnly = process.argv.includes("--test-only");
const testBinDir = join(repoRoot, "build", "tests");
const BIN_SUFFIX = process.platform === "win32" ? ".test.exe" : ".test";

// ── Compile (both modes) ────────────────────────────────────────────────────
let packages;
try {
  packages = listTestPackages(serviceDir);
  compileTestBinaries({ serviceDir, outDir: testBinDir, coverage: !testOnly });
} catch (err) {
  console.error(`\n[coverage-service] FAILED: ${err.message}`);
  process.exit(1);
}

// ── Plain test run ────────────────────────────────────────────────────────────
if (testOnly) {
  const status = runTestBinaries({
    packages,
    outDir: testBinDir,
    extraArgs: ["-test.timeout=120s"],
  });
  process.exit(status);
}

// ── Coverage run ────────────────────────────────────────────────────────────
const coverDir = join(repoRoot, "coverage");
if (!existsSync(coverDir)) mkdirSync(coverDir, { recursive: true });

const coverProfile = join(coverDir, `service-coverage-${randomUUID()}.out`);
const coverDataDir = join(coverDir, `service-covdata-${randomUUID()}`);
mkdirSync(coverDataDir, { recursive: true });

console.log("[coverage-service] Running service tests with coverage (sequential)...\n");

// Each binary writes binary coverage data to coverDataDir; `go tool covdata
// textfmt` merges it into the legacy profile the parser below expects.
const runStatus = runTestBinaries({
  packages,
  outDir: testBinDir,
  // -test.gocoverdir tells a directly-run `-cover` test binary to emit its raw
  // coverage data into coverDataDir (the GOCOVERDIR env var alone is honored by
  // `go build -cover` binaries, not by manually-run `go test -c` binaries).
  extraArgs: ["-test.timeout=120s", `-test.gocoverdir=${coverDataDir}`],
  env: { ...process.env, GOCOVERDIR: coverDataDir },
});

if (runStatus !== 0) {
  try { rmSync(coverDataDir, { recursive: true, force: true }); } catch { /* best-effort */ }
  console.error("\n[coverage-service] FAILED: one or more test packages failed.");
  process.exit(1);
}

// Merge the per-binary coverage data into a single legacy text profile.
const mergeResult = spawnSync(
  "go",
  ["tool", "covdata", "textfmt", `-i=${coverDataDir}`, `-o=${coverProfile}`],
  { cwd: serviceDir, stdio: ["inherit", "inherit", "inherit"], encoding: "utf8" }
);
try { rmSync(coverDataDir, { recursive: true, force: true }); } catch { /* best-effort */ }

if ((mergeResult.status ?? 1) !== 0) {
  console.error("\n[coverage-service] FAILED: go tool covdata textfmt returned non-zero exit code.");
  process.exit(1);
}

// covdata only knows about packages that were linked into a test binary, so
// packages with NO tests that are also un-imported (e.g. sources/logger,
// the sources main package, sources/platform) are dropped from the profile.
// The previous `go test -coverpkg=./sources/... ./sources/...` run counted them
// as 0% in the denominator, so the 90% gate is calibrated WITH them. Re-add
// their 0% blocks so the number stays comparable. Running `go test -run='^$'`
// on those (test-less) packages executes nothing — no socket bind, no firewall
// prompt — and emits exactly their statement blocks at count 0.
supplementMissingPackages(coverProfile, serviceDir);

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

// ── Test-binary helpers (compile to a stable path, run from there) ──────────────

/** Discover the service packages that have test files. */
function listTestPackages(svcDir) {
  const result = spawnSync(
    "go",
    [
      "list",
      "-f",
      "{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}|{{.Dir}}|{{.Name}}{{end}}",
      "./sources/...",
    ],
    { cwd: svcDir, encoding: "utf8" },
  );
  if ((result.status ?? 1) !== 0) {
    console.error(result.stderr || "[coverage-service] go list failed");
    throw new Error("go list failed");
  }
  return (result.stdout || "")
    .split("\n")
    .map((l) => l.trim())
    .filter(Boolean)
    .map((line) => {
      const [importPath, dir, name] = line.split("|");
      return { importPath, dir, name };
    });
}

/** Compile all service test binaries to outDir (no run). Throws on failure. */
function compileTestBinaries({ serviceDir: svcDir, outDir, coverage }) {
  mkdirSync(outDir, { recursive: true });
  const args = ["test", "-c"];
  if (coverage) args.push("-cover", "-coverpkg=./sources/...");
  // Trailing separator makes Go treat the path as a directory and emit one
  // <pkg>.test[.exe] per package.
  args.push("-o", outDir.endsWith(sep) ? outDir : outDir + sep, "./sources/...");

  console.log(
    `[coverage-service] Compiling test binaries -> ${outDir}` +
      (coverage ? " (coverage-instrumented)" : ""),
  );
  const result = spawnSync("go", args, { cwd: svcDir, stdio: "inherit", encoding: "utf8" });
  if ((result.status ?? 1) !== 0) {
    throw new Error("compile failed");
  }
}

/**
 * Run each compiled test binary from its stable path, with cwd set to the
 * package source dir. Returns 0 if all passed, 1 otherwise.
 */
function runTestBinaries({ packages: pkgs, outDir, env, extraArgs = [] }) {
  const failed = [];
  for (const pkg of pkgs) {
    const binPath = join(outDir, pkg.name + BIN_SUFFIX);
    if (!existsSync(binPath)) {
      console.warn(`[coverage-service] WARN: missing test binary for ${pkg.importPath}`);
      continue;
    }
    console.log(`\n=== ${pkg.importPath} ===`);
    console.log(`[run] ${binPath}`);
    const result = spawnSync(binPath, extraArgs, {
      cwd: pkg.dir,
      stdio: "inherit",
      env: env ?? process.env,
      encoding: "utf8",
    });
    if ((result.status ?? 1) !== 0) failed.push(pkg.importPath);
  }

  if (failed.length > 0) {
    console.error(`\n[coverage-service] FAILED packages:\n  ${failed.join("\n  ")}`);
    return 1;
  }
  console.log(`\n[coverage-service] All ${pkgs.length} test package(s) passed.`);
  return 0;
}

// ── Coverage helpers ────────────────────────────────────────────────────────

/**
 * Append 0%-coverage blocks for any ./sources/... package missing from the
 * merged profile, matching the denominator the old single-run profile produced.
 * Lines for files already present are skipped, so the append can never
 * double-count (the missing packages are disjoint from the covered ones).
 */
function supplementMissingPackages(profilePath, svcDir) {
  const profileText = readFileSync(profilePath, "utf8");
  const presentFiles = new Set();
  const presentPkgs = new Set();
  for (const line of profileText.split("\n")) {
    if (!line || line.startsWith("mode:")) continue;
    const file = line.split(":")[0];
    if (!file) continue;
    presentFiles.add(file);
    presentPkgs.add(file.slice(0, file.lastIndexOf("/"))); // dir == import path
  }

  const listed = spawnSync("go", ["list", "./sources/..."], { cwd: svcDir, encoding: "utf8" });
  if ((listed.status ?? 1) !== 0) {
    console.error("[coverage-service] WARN: go list failed; skipping denominator supplement.");
    return;
  }
  const missing = (listed.stdout || "")
    .split("\n")
    .map((l) => l.trim())
    .filter((p) => p && !presentPkgs.has(p));
  if (missing.length === 0) return;

  const suppProfile = join(coverDir, `service-supp-${randomUUID()}.out`);
  // -run='^$' matches no tests; test-less packages run nothing, so this neither
  // binds a socket nor prompts the firewall — it only emits their 0% blocks.
  const supp = spawnSync(
    "go",
    ["test", "-run=^$", "-coverpkg=./sources/...", `-coverprofile=${suppProfile}`, ...missing],
    { cwd: svcDir, stdio: ["inherit", "ignore", "inherit"], encoding: "utf8" }
  );
  if ((supp.status ?? 1) !== 0 || !existsSync(suppProfile)) {
    console.error("[coverage-service] WARN: could not generate missing-package blocks; skipping supplement.");
    try { rmSync(suppProfile); } catch { /* best-effort */ }
    return;
  }

  const extra = readFileSync(suppProfile, "utf8")
    .split("\n")
    .filter((line) => line && !line.startsWith("mode:") && !presentFiles.has(line.split(":")[0]));
  try { rmSync(suppProfile); } catch { /* best-effort */ }

  if (extra.length > 0) {
    const sepStr = profileText.endsWith("\n") ? "" : "\n";
    writeFileSync(profilePath, profileText + sepStr + extra.join("\n") + "\n");
  }
}

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
