#!/usr/bin/env node
/* global console, process */
import { existsSync, rmSync } from "node:fs";
import { dirname, resolve, join } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");

const targets = [
  "build/portier",
  "build/windows",
  "build/macos",
  "build/linux",
  "build/releases",
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
