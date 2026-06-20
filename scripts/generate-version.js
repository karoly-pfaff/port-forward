#!/usr/bin/env node
/* global console, process */
/**
 * Single source of truth for the Portier version.
 *
 * The version lives in the root package.json. Every other place that must carry
 * the version is a *surface* that this script keeps in lockstep — so a release
 * bump is one command, and drift is caught by CI instead of shipping (the client
 * sidebar once showed v1.14.1 across four releases because one surface was
 * hand-maintained and unguarded).
 *
 * Usage:
 *   node scripts/generate-version.js check          # verify every surface matches root (default)
 *   node scripts/generate-version.js list           # show each surface's current value
 *   node scripts/generate-version.js set <x.y.z>    # write the version to every surface
 *   node scripts/generate-version.js bump <major|minor|patch>
 *
 * Scopes:
 *   full        the exact version, e.g. 1.18.0
 *   majorminor  the OpenAPI doc version, e.g. 1.18 (derived from the full version)
 *
 * `set`/`bump` only rewrite the version token in place (JSON re-serialized with
 * the repo's 2-space + trailing-newline convention, verified byte-stable), so
 * diffs stay to the version line. They do not regenerate the OpenAPI document
 * from the running app — but docs/openapi.json's info.version is patched
 * directly to stay consistent, and `check` enforces it.
 */
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = join(dirname(fileURLToPath(import.meta.url)), "..");

// Every version surface in the repo. Adding a new one here is the ONLY change
// needed to bring it under the bump/guard. Keep this list explicit (not globbed)
// so it is reviewed and never accidentally matches an unrelated file.
const SURFACES = [
  { id: "root package.json", file: "package.json", kind: "json", path: ["version"], scope: "full" },
  { id: "client/package.json", file: "client/package.json", kind: "json", path: ["version"], scope: "full" },
  { id: "server/package.json", file: "server/package.json", kind: "json", path: ["version"], scope: "full" },
  { id: "shared/package.json", file: "shared/package.json", kind: "json", path: ["version"], scope: "full" },
  { id: "PORTIER_APP_VERSION", file: "shared/sources/index.ts", kind: "const", token: "PORTIER_APP_VERSION", scope: "full" },
  { id: "service version.go", file: "service/sources/version/version.go", kind: "govar", scope: "full" },
  { id: "cli version.go", file: "tools/cli/sources/version/version.go", kind: "govar", scope: "full" },
  { id: "replay version.go", file: "tools/replay/sources/version/version.go", kind: "govar", scope: "full" },
  { id: "OPENAPI_DOC_VERSION", file: "server/sources/openapi/openapi.ts", kind: "const", token: "OPENAPI_DOC_VERSION", scope: "majorminor" },
  { id: "docs/openapi.json info.version", file: "docs/openapi.json", kind: "json", path: ["info", "version"], scope: "majorminor" },
];

const SEMVER = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?$/;

function abs(file) {
  return join(repoRoot, file);
}

function majorMinor(version) {
  const parts = version.split(".");
  return parts.length >= 2 ? `${parts[0]}.${parts[1]}` : version;
}

function expectedFor(surface, version) {
  return surface.scope === "majorminor" ? majorMinor(version) : version;
}

// Regex capturing (prefix)(value)(suffix) for the assignment surfaces. Non-global
// so String.replace touches only the first occurrence (the declaration).
function assignPattern(surface) {
  if (surface.kind === "govar") return /(\bvar\s+Version\s*=\s*")([^"]*)(")/;
  return new RegExp(`(\\b${surface.token}\\s*=\\s*")([^"]*)(")`);
}

function jsonAt(obj, path) {
  return path.reduce((node, key) => (node == null ? undefined : node[key]), obj);
}

function setJsonAt(obj, path, value) {
  let node = obj;
  for (let i = 0; i < path.length - 1; i += 1) node = node[path[i]];
  node[path[path.length - 1]] = value;
}

function currentValue(surface) {
  const text = readFileSync(abs(surface.file), "utf8");
  if (surface.kind === "json") {
    const value = jsonAt(JSON.parse(text), surface.path);
    return value == null ? null : String(value);
  }
  const match = text.match(assignPattern(surface));
  return match ? match[2] : null;
}

function writeValue(surface, value) {
  const text = readFileSync(abs(surface.file), "utf8");
  if (surface.kind === "json") {
    const obj = JSON.parse(text);
    setJsonAt(obj, surface.path, value);
    writeFileSync(abs(surface.file), `${JSON.stringify(obj, null, 2)}\n`);
    return;
  }
  const pattern = assignPattern(surface);
  if (!pattern.test(text)) {
    throw new Error(`could not locate version assignment in ${surface.file}`);
  }
  writeFileSync(abs(surface.file), text.replace(pattern, `$1${value}$3`));
}

function rootVersion() {
  const value = currentValue(SURFACES[0]);
  if (!value) {
    console.error("[version] root package.json has no version field.");
    process.exit(1);
  }
  return value;
}

function pad(text, width) {
  return text.padEnd(width);
}

function report(version) {
  const idWidth = Math.max(...SURFACES.map((s) => s.id.length));
  const rows = SURFACES.map((surface) => {
    const expected = expectedFor(surface, version);
    const actual = currentValue(surface);
    return { surface, expected, actual, ok: actual === expected };
  });
  for (const row of rows) {
    const status = row.ok ? "ok  " : "DRIFT";
    const detail = row.ok ? row.actual : `${row.actual} (expected ${row.expected})`;
    console.log(`  ${status}  ${pad(row.surface.id, idWidth)}  ${detail}`);
  }
  return rows;
}

function runCheck() {
  const version = rootVersion();
  console.log(`[version] source of truth: root package.json = ${version} (OpenAPI ${majorMinor(version)})`);
  const rows = report(version);
  const drift = rows.filter((r) => !r.ok);
  if (drift.length > 0) {
    console.error(`\n[version] ${drift.length} surface(s) out of sync. Fix with: npm run version:set ${version}`);
    process.exit(1);
  }
  console.log(`\n[version] all ${rows.length} surfaces consistent.`);
}

function runList() {
  const version = rootVersion();
  console.log(`[version] root package.json = ${version} (OpenAPI ${majorMinor(version)})`);
  report(version);
}

function runSet(version) {
  if (!version || !SEMVER.test(version)) {
    console.error(`[version] invalid version "${version ?? ""}". Expected semver like 1.19.0 or 1.19.0-rc.1.`);
    process.exit(1);
  }
  let changed = 0;
  for (const surface of SURFACES) {
    const expected = expectedFor(surface, version);
    const before = currentValue(surface);
    if (before === expected) continue;
    writeValue(surface, expected);
    console.log(`  set   ${surface.id}: ${before} -> ${expected}`);
    changed += 1;
  }
  if (changed === 0) {
    console.log(`[version] already at ${version}; nothing to change.`);
  } else {
    console.log(`\n[version] updated ${changed} surface(s) to ${version} (OpenAPI ${majorMinor(version)}).`);
    console.log("[version] note: docs/openapi.json info.version was patched in place; regenerate with");
    console.log("          `npm run apidoc:generate` if the API itself also changed.");
  }
  const drift = report(version).filter((r) => !r.ok);
  if (drift.length > 0) {
    console.error("\n[version] post-write verification failed — some surfaces did not update.");
    process.exit(1);
  }
}

function runBump(level) {
  const base = rootVersion().split("-")[0];
  const [major, minor, patch] = base.split(".").map((n) => Number(n));
  let next;
  if (level === "major") next = `${major + 1}.0.0`;
  else if (level === "minor") next = `${major}.${minor + 1}.0`;
  else if (level === "patch") next = `${major}.${minor}.${patch + 1}`;
  else {
    console.error(`[version] unknown bump level "${level ?? ""}". Use major | minor | patch.`);
    process.exit(1);
  }
  console.log(`[version] bumping ${level}: ${rootVersion()} -> ${next}`);
  runSet(next);
}

const [command, arg] = process.argv.slice(2);
switch (command ?? "check") {
  case "check":
    runCheck();
    break;
  case "list":
    runList();
    break;
  case "set":
    runSet(arg);
    break;
  case "bump":
    runBump(arg);
    break;
  default:
    console.error(`[version] unknown command "${command}". Use: check | list | set <x.y.z> | bump <level>`);
    process.exit(1);
}
