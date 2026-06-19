#!/usr/bin/env node
/* global console, process */
/**
 * Structural payload validation for the Portier Linux .rpm.
 *
 * Lists the package payload (`rpm -qlp`) and asserts the expected file-install layout under
 * /opt/portier plus the systemd unit, that the package metadata version matches, and that no
 * user config (rules.json) or private/dev material is shipped. Does NOT install anything.
 *
 * Linux-only and needs the `rpm` CLI (the `rpm` package on Debian/Ubuntu, native on
 * Fedora/RHEL). On other platforms it exits 0 with a skip notice so cross-platform release
 * validation is unaffected.
 *
 * Usage:
 *   node scripts/validate-rpm-payload.js [--rpm <path>]
 */

import { existsSync, readFileSync, statSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const isLinux = process.platform === "linux";

const rawArgs = process.argv.slice(2);
const flagValue = (f) => {
  const i = rawArgs.indexOf(f);
  return i >= 0 && i + 1 < rawArgs.length ? rawArgs[i + 1] : null;
};

let passed = 0;
let failed = 0;
const pass = (m) => { console.log(`  ✓ ${m}`); passed++; };
const fail = (m) => { console.error(`  ✗ ${m}`); failed++; };

const version = JSON.parse(readFileSync(join(repoRoot, "package.json"), "utf8")).version;

function main() {
  if (!isLinux) {
    console.log(`[rpm-payload] Linux-only; skipping on ${process.platform}.`);
    return;
  }
  if ((spawnSync("rpm", ["--version"], { encoding: "utf8" }).status ?? 1) !== 0) {
    fail("rpm CLI not available — install the 'rpm' package (Debian/Ubuntu) or use Fedora/RHEL");
    process.exit(1);
  }

  const rpmPath = flagValue("--rpm")
    ? resolve(flagValue("--rpm"))
    : join(repoRoot, "build", "releases", "linux", `portier-${version}-1.x86_64.rpm`);

  console.log(`[rpm-payload] RPM : ${rpmPath}`);
  if (!existsSync(rpmPath)) {
    fail(`.rpm not found: ${rpmPath} — build with: npm run build:release:current (Linux, needs rpmbuild)`);
    process.exit(1);
  }
  pass(`.rpm present (${(statSync(rpmPath).size / 1024 / 1024).toFixed(1)} MB)`);

  // Package metadata version.
  const v = spawnSync("rpm", ["-qp", "--queryformat", "%{VERSION}", rpmPath], { encoding: "utf8" });
  const metaVersion = (v.stdout || "").trim();
  if (metaVersion === version) pass(`rpm metadata version = ${metaVersion}`);
  else fail(`rpm metadata version = "${metaVersion}", expected ${version}`);

  // Payload file list.
  const ql = spawnSync("rpm", ["-qlp", rpmPath], { encoding: "utf8" });
  if ((ql.status ?? 1) !== 0) {
    fail(`rpm -qlp failed: ${(ql.stderr || "").trim()}`);
    process.exit(1);
  }
  const files = (ql.stdout || "").split(/\r?\n/).map((s) => s.trim()).filter(Boolean);

  console.log("Payload layout:");
  for (const req of [
    "/opt/portier/portier",
    "/opt/portier/service",
    "/opt/portier/server.js",
    "/opt/portier/api/openapi.json",
    "/opt/portier/web/index.html",
    "/opt/portier/readme.txt",
    "/lib/systemd/system/portier.service",
  ]) {
    if (files.includes(req)) pass(req);
    else fail(`missing from payload: ${req}`);
  }

  console.log("Forbidden content:");
  const forbidden = [
    [/rules\.json$/, "rules.json (user config must not be shipped)"],
    [/(^|\/)\.env$/, ".env"],
    [/docs\/private/, "docs/private"],
    [/(^|\/)sources(\/|$)/, "sources/"],
  ];
  for (const [re, label] of forbidden) {
    if (files.some((f) => re.test(f))) fail(`must not contain: ${label}`);
    else pass(`absent: ${label}`);
  }

  console.log(`\n[rpm-payload] ${passed} passed, ${failed} failed.`);
  if (failed > 0) {
    console.error("[rpm-payload] RPM payload validation FAILED.\n");
    process.exit(1);
  }
  console.log("[rpm-payload] RPM payload structurally validated.\n");
}

main();
