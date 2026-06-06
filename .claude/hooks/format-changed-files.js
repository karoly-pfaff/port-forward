#!/usr/bin/env node
/* global console, process */
import { existsSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import path from "node:path";

const ROOT = process.cwd();
const BLOCKED_SEGMENTS = new Set(["node_modules", "build", "coverage", ".git"]);
const PRETTIER_EXTENSIONS = new Set([".ts", ".tsx", ".js", ".jsx", ".json", ".css", ".md"]);

function toRepoPath(filePath) {
  return filePath.replace(/\\/g, "/");
}

function isBlockedPath(filePath) {
  const segments = toRepoPath(filePath).split("/");
  return segments.some((s) => BLOCKED_SEGMENTS.has(s));
}

function prettierBin() {
  const localBin =
    process.platform === "win32"
      ? path.join(ROOT, "node_modules", ".bin", "prettier.cmd")
      : path.join(ROOT, "node_modules", ".bin", "prettier");
  return existsSync(localBin) ? localBin : null;
}

function gofmtAvailable() {
  const result = spawnSync("gofmt", ["-h"], { encoding: "utf8" });
  return result.status !== null && result.error == null;
}

/** Read the PostToolUse event from stdin (piped by Claude Code). Returns null on TTY. */
function readHookEvent() {
  if (process.stdin.isTTY) return null;
  try {
    const raw = readFileSync(0, "utf8").trim();
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

/** Extract the edited file path from a PostToolUse event. */
function fileFromEvent(event) {
  const input = event?.tool_input ?? {};
  const p = input.file_path ?? input.path ?? null;
  return p ? toRepoPath(p) : null;
}

/** Fall back to git-changed files when not triggered by a hook event. */
function gitChangedFiles() {
  for (const args of [
    ["diff", "--name-only", "HEAD", "--"],
    ["diff", "--name-only", "--cached", "--"],
    ["diff", "--name-only", "--"],
  ]) {
    const result = spawnSync("git", args, { cwd: ROOT, encoding: "utf8" });
    if (result.status === 0 && result.stdout.trim()) {
      return result.stdout.split(/\r?\n/).filter(Boolean);
    }
  }
  return [];
}

// ── Determine which file(s) to format ──────────────────────────────────────

const event = readHookEvent();
const eventFile = fileFromEvent(event);

const candidates = eventFile
  ? [eventFile]
  : gitChangedFiles();

const prettierFiles = candidates.filter((f) => {
  const abs = path.isAbsolute(f) ? f : path.join(ROOT, f);
  return (
    PRETTIER_EXTENSIONS.has(path.extname(f)) &&
    !isBlockedPath(f) &&
    existsSync(abs)
  );
});

const goFiles = candidates.filter((f) => {
  const abs = path.isAbsolute(f) ? f : path.join(ROOT, f);
  return f.endsWith(".go") && !isBlockedPath(f) && existsSync(abs);
});

if (prettierFiles.length === 0 && goFiles.length === 0) {
  console.log("[portier hook] No formattable files detected.");
  process.exit(0);
}

let exitCode = 0;

// ── Prettier ────────────────────────────────────────────────────────────────
if (prettierFiles.length > 0) {
  const bin = prettierBin();
  if (!bin) {
    console.log("[portier hook] Prettier not installed locally; skipping TS/JSON/CSS/MD formatting.");
  } else {
    console.log(`[portier hook] Prettier: formatting ${prettierFiles.length} file(s).`);
    const result = spawnSync(bin, ["--write", ...prettierFiles], {
      cwd: ROOT,
      stdio: "inherit",
    });
    if ((result.status ?? 1) !== 0) exitCode = result.status ?? 1;
  }
}

// ── gofmt ───────────────────────────────────────────────────────────────────
if (goFiles.length > 0) {
  if (!gofmtAvailable()) {
    console.log("[portier hook] gofmt not found; skipping Go formatting.");
  } else {
    console.log(`[portier hook] gofmt: formatting ${goFiles.length} file(s).`);
    const absPaths = goFiles.map((f) => (path.isAbsolute(f) ? f : path.join(ROOT, f)));
    const result = spawnSync("gofmt", ["-w", ...absPaths], {
      cwd: ROOT,
      stdio: "inherit",
    });
    if ((result.status ?? 1) !== 0) exitCode = result.status ?? 1;
  }
}

process.exit(exitCode);
