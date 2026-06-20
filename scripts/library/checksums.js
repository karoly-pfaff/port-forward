/**
 * Shared SHA-256 checksum helpers for Portier release artifacts.
 *
 * Pure functions (no console/process side effects) so they can be unit tested
 * and reused by both scripts/build-release.js (generation) and
 * scripts/validate-release.js (verification).
 *
 * Checksums live in a single `checksums.sha256` file next to the release artifacts,
 * in GNU coreutils text format ("<sha256>  <filename>"), one line per artifact,
 * lowercase hex, LF line endings. Entries are ordered the native installer/package
 * first, then the portable archive, alphabetical within each group — deterministic and
 * human-readable (package-first). Verifiable with `sha256sum -c checksums.sha256`.
 */

import { createHash } from "node:crypto";
import { readFileSync, writeFileSync, readdirSync, statSync, existsSync } from "node:fs";
import { join } from "node:path";

export const CHECKSUMS_NAME = "checksums.sha256";

// Release artifact extensions. .msi/.pkg/.deb/.rpm are listed so future installer
// artifacts are checksummed automatically once those slices land — this slice does
// not build them.
const ARTIFACT_EXTENSIONS = [".zip", ".exe", ".tar.gz", ".msi", ".pkg", ".deb", ".rpm"];

// Native installer/package extensions, in user-facing priority order. They sort before
// portable archives so the manifest/listings read package-first; among Linux packages this
// puts .deb before .rpm regardless of filename punctuation.
const INSTALLER_EXTENSIONS = [".msi", ".pkg", ".deb", ".rpm", ".exe"];

// compareReleaseArtifactNames orders release artifacts deterministically: native
// installers/packages first (by the extension priority above — e.g. .deb before .rpm), then
// portable archives, alphabetical within each group (so amd64 sorts before arm64).
export function compareReleaseArtifactNames(a, b) {
  // [group, extIndex]: group 0 = installer (ordered by extension priority), 1 = portable.
  const key = (name) => {
    const lower = name.toLowerCase();
    const idx = INSTALLER_EXTENSIONS.findIndex((ext) => lower.endsWith(ext));
    return idx >= 0 ? [0, idx] : [1, 0];
  };
  const [ga, ia] = key(a);
  const [gb, ib] = key(b);
  if (ga !== gb) return ga - gb;
  if (ia !== ib) return ia - ib;
  return a < b ? -1 : a > b ? 1 : 0;
}

// isReleaseArtifactName reports whether a filename is a release artifact (and not
// the checksums.sha256 file itself or a per-file .sha256 sidecar).
export function isReleaseArtifactName(name) {
  const lower = name.toLowerCase();
  if (lower === CHECKSUMS_NAME.toLowerCase()) return false;
  if (lower.endsWith(".sha256")) return false;
  return ARTIFACT_EXTENSIONS.some((ext) => lower.endsWith(ext));
}

// findReleaseArtifacts returns the release artifact filenames in `dir` that belong
// to `version` (the version string appears in the artifact name), ordered
// package-first then portable (compareReleaseArtifactNames). Stale artifacts from
// other versions are ignored.
export function findReleaseArtifacts(dir, version) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir)
    .filter((name) => {
      if (!statSync(join(dir, name)).isFile()) return false;
      if (!isReleaseArtifactName(name)) return false;
      return version ? name.includes(version) : true;
    })
    .sort(compareReleaseArtifactNames);
}

// sha256File returns the lowercase hex SHA-256 of a file's contents.
export function sha256File(filePath) {
  return createHash("sha256").update(readFileSync(filePath)).digest("hex");
}

// sha256Buffer returns the lowercase hex SHA-256 of a buffer/string (test helper).
export function sha256Buffer(data) {
  return createHash("sha256").update(data).digest("hex");
}

// formatSha256Sums renders entries ({ hash, name }) ordered package-first then portable
// (compareReleaseArtifactNames), GNU format "<sha256>  <name>", with a trailing newline.
export function formatSha256Sums(entries) {
  return (
    [...entries]
      .sort((a, b) => compareReleaseArtifactNames(a.name, b.name))
      .map((e) => `${e.hash.toLowerCase()}  ${e.name}`)
      .join("\n") + "\n"
  );
}

// generateChecksums computes SHA-256 for every version-scoped release artifact in
// `dir` and writes checksums.sha256. Returns the written entries.
export function generateChecksums(dir, version) {
  const entries = findReleaseArtifacts(dir, version).map((name) => ({
    name,
    hash: sha256File(join(dir, name)),
  }));
  writeFileSync(join(dir, CHECKSUMS_NAME), formatSha256Sums(entries));
  return entries;
}

// parseSha256Sums parses checksums.sha256 text into [{ hash, name }]. Throws on malformed
// lines, path separators in filenames, or duplicate filenames. Blank lines (e.g. a
// trailing newline) are ignored.
export function parseSha256Sums(text) {
  const entries = [];
  const seen = new Set();
  const lines = text.split(/\r?\n/);
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (line.trim() === "") continue;
    const m = /^([0-9a-f]{64}) {2}(\S.*)$/.exec(line);
    if (!m) {
      throw new Error(`malformed line ${i + 1}: ${JSON.stringify(line)}`);
    }
    const [, hash, name] = m;
    if (name.includes("/") || name.includes("\\")) {
      throw new Error(`line ${i + 1} filename contains a path separator: ${name}`);
    }
    if (seen.has(name)) {
      throw new Error(`duplicate entry for: ${name}`);
    }
    seen.add(name);
    entries.push({ hash, name });
  }
  return entries;
}
