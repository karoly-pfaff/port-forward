#!/usr/bin/env node
/* global console, process */
import { existsSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import path from "node:path";

const ROOT = process.cwd();
const packageJsonPath = path.join(ROOT, "package.json");

// ── Read Stop hook event from stdin ─────────────────────────────────────────

function readHookEvent() {
  if (process.stdin.isTTY) return null;
  try {
    const raw = readFileSync(0, "utf8").trim();
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

const event = readHookEvent();

// Guard: if this run was itself triggered by a Stop hook, exit cleanly to
// prevent an infinite re-entry loop.
if (event?.stop_hook_active === true) {
  console.log("[portier hook] stop_hook_active — skipping re-validation to avoid loop.");
  process.exit(0);
}

// ── Detect which source areas changed ───────────────────────────────────────

function changedFiles() {
  for (const args of [
    ["diff", "--name-only", "HEAD", "--"],
    ["diff", "--name-only", "--cached", "--"],
  ]) {
    const r = spawnSync("git", args, { cwd: ROOT, encoding: "utf8" });
    if (r.status === 0 && r.stdout.trim()) {
      return r.stdout.split(/\r?\n/).filter(Boolean);
    }
  }
  // Unborn repo or nothing staged — parse git status --porcelain
  const r = spawnSync("git", ["status", "--porcelain"], { cwd: ROOT, encoding: "utf8" });
  if (r.status === 0 && r.stdout.trim()) {
    return r.stdout
      .split(/\r?\n/)
      .filter(Boolean)
      .map((line) => {
        // " M path" or "?? path" or "R  old -> new"
        const file = line.slice(3).trim().split(" -> ").pop();
        return file ?? null;
      })
      .filter(Boolean);
  }
  return null; // null means "could not detect — run everything"
}

const files = changedFiles();

function anyMatch(patterns) {
  if (files === null) return true; // no info → assume yes
  return files.some((f) => patterns.some((p) => f.includes(p)));
}

const tsChanged = anyMatch(["server/", "client/", "shared/", ".ts", ".tsx", ".js", ".jsx"]);
const goChanged = anyMatch(["service/sources/", ".go"]);

if (!tsChanged && !goChanged) {
  console.log("[portier hook] No source changes detected — skipping validation.");
  process.exit(0);
}

// ── Helpers ─────────────────────────────────────────────────────────────────

function readScripts() {
  if (!existsSync(packageJsonPath)) return {};
  const pkg = JSON.parse(readFileSync(packageJsonPath, "utf8"));
  return pkg.scripts ?? {};
}

function run(command, args, label) {
  console.log(`[portier hook] ${label}`);
  const result = spawnSync(command, args, { cwd: ROOT, stdio: "inherit", timeout: 120_000 });
  const code = result.status ?? 1;
  if (code !== 0) {
    console.error(`[portier hook] FAILED: ${label} (exit ${code})`);
  }
  return code;
}

function npmRun(scriptName, scripts) {
  if (!scripts[scriptName]) {
    console.log(`[portier hook] Skipping missing npm script: ${scriptName}`);
    return 0;
  }
  const cmd = process.platform === "win32" ? "cmd.exe" : "npm";
  const args =
    process.platform === "win32"
      ? ["/d", "/s", "/c", "npm.cmd", "run", scriptName]
      : ["run", scriptName];
  return run(cmd, args, `npm run ${scriptName}`);
}

// ── Run validation ───────────────────────────────────────────────────────────

const scripts = readScripts();
const failures = [];

if (tsChanged) {
  for (const name of ["lint", "typecheck", "test:shared", "test:server", "test:client"]) {
    if (npmRun(name, scripts) !== 0) failures.push(`npm run ${name}`);
  }
}

if (goChanged) {
  const goAvailable = spawnSync("go", ["version"], { encoding: "utf8" }).status === 0;
  if (!goAvailable) {
    console.log("[portier hook] go not found in PATH — skipping Go validation.");
  } else {
    const goArgs = ["-C", "service"];
    if (run("go", [...goArgs, "vet", "./..."], "go vet ./...") !== 0) failures.push("go vet");
    if (run("go", [...goArgs, "test", "./..."], "go test ./...") !== 0) failures.push("go test");
  }
}

// ── Report ───────────────────────────────────────────────────────────────────

if (failures.length === 0) {
  console.log("[portier hook] All validation passed.");
  process.exit(0);
} else {
  console.error(`[portier hook] Validation failed: ${failures.join(", ")}`);
  // Exit 2 re-enters Claude with this output so it can attempt to fix the issues.
  process.exit(2);
}
