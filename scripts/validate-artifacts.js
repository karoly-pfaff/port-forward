#!/usr/bin/env node
/* global console, process */
/**
 * Release-artifact transparency + privacy guard (CI upload gate).
 *
 * Lists every file under build/releases/ (what the GitHub Actions release matrix
 * uploads as workflow artifacts) and FAILS if anything that looks like private
 * planning material (a `private` path segment, e.g. docs/private/**) is present.
 *
 * This is belt-and-suspenders: the release build never stages docs/private into
 * build/releases/, and validate-portable.js already asserts docs/private is absent
 * from each portable archive. This guard makes the upload contents explicit in the
 * CI log and hard-fails before upload if that invariant is ever violated.
 *
 * Runs on any host (pure Node, no shell quoting). Usage:
 *   node scripts/validate-artifacts.js
 */

import { existsSync, readdirSync, statSync } from "node:fs";
import { join, resolve, dirname, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const releasesDir = join(repoRoot, "build", "releases");

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) walk(full, out);
    else out.push(full);
  }
  return out;
}

if (!existsSync(releasesDir)) {
  console.error("[validate-artifacts] build/releases/ not found — nothing to upload. Run a build:release:* target first.");
  process.exit(1);
}

const files = walk(releasesDir)
  .map((f) => relative(repoRoot, f).split(sep).join("/"))
  .sort();

console.log(`[validate-artifacts] ${files.length} file(s) under build/releases/ (these are uploaded):`);
for (const f of files) console.log(`  ${f}`);

// Privacy guard: refuse to upload anything with a `private` path segment.
const offenders = files.filter((f) => f.split("/").includes("private"));
if (offenders.length > 0) {
  console.error("\n[validate-artifacts] PRIVACY GUARD FAILED — private material must not be uploaded:");
  for (const f of offenders) console.error(`  ${f}`);
  process.exit(1);
}

if (files.length === 0) {
  console.error("[validate-artifacts] No artifacts produced — failing so the empty upload is caught.");
  process.exit(1);
}

console.log("\n[validate-artifacts] OK — no private material; artifacts ready to upload.");
