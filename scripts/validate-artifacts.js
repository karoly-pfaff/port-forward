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
import { CHECKSUMS_NAME } from "./release-checksums.js";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const releasesDir = join(repoRoot, "build", "releases");

const INSTALLER_EXTS = [".msi", ".pkg", ".deb", ".rpm", ".exe"];

// Display order within a platform directory: native installer/package first, portable
// archive second, the checksums.sha256 manifest last.
function displayRank(path) {
  const base = path.split("/").pop().toLowerCase();
  if (base === CHECKSUMS_NAME) return 2;
  if (INSTALLER_EXTS.some((ext) => base.endsWith(ext))) return 0;
  return 1;
}

function dirOf(path) {
  return path.split("/").slice(0, -1).join("/");
}

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

// Group by platform directory, then package-first / portable / checksums-last within.
const files = walk(releasesDir)
  .map((f) => relative(repoRoot, f).split(sep).join("/"))
  .sort((a, b) => {
    const da = dirOf(a);
    const db = dirOf(b);
    if (da !== db) return da < db ? -1 : 1;
    const ra = displayRank(a);
    const rb = displayRank(b);
    if (ra !== rb) return ra - rb;
    return a < b ? -1 : a > b ? 1 : 0;
  });

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
