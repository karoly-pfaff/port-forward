#!/usr/bin/env node
/* global console, process */
import { existsSync, rmSync } from "node:fs";
import { dirname, resolve, join } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");

const targets = [
  // Top-level packaged-runtime / release / test-binary outputs.
  "build/portier",
  "build/windows",
  "build/macos",
  "build/linux",
  "build/releases",
  "build/tests",
  // Per-workspace dev build outputs (repo-internal, not distributed).
  "client/build",
  "server/build",
  "shared/build",
  "service/build",
  "tools/cli/build",
  "tools/replay/build",
];

for (const rel of targets) {
  const full = join(repoRoot, rel);
  if (existsSync(full)) {
    rmSync(full, { recursive: true, force: true });
    console.log(`[build:clean] Removed: ${rel}`);
  } else {
    console.log(`[build:clean] Already absent: ${rel}`);
  }
}

console.log("[build:clean] Done.");
process.exit(0);
