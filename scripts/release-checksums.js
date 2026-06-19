/**
 * Shared SHA-256 checksum helpers for Portier release artifacts.
 *
 * Pure functions (no console/process side effects) so they can be unit tested
 * and reused by both scripts/build-release.js (generation) and
 * scripts/validate-release.js (verification).
 *
 * Checksums live in a single `SHA256SUMS` file next to the release artifacts,
 * in GNU coreutils text format ("<sha256>  <filename>"), one line per artifact,
 * lowercase hex, sorted by filename, LF line endings.
 */

import { createHash } from "node:crypto";
import { readFileSync, writeFileSync, readdirSync, statSync, existsSync } from "node:fs";
import { join } from "node:path";

export const SHA256SUMS_NAME = "SHA256SUMS";

// Release artifact extensions. .msi/.pkg/.deb/.rpm are listed so future installer
// artifacts are checksummed automatically once those slices land — this slice does
// not build them.
const ARTIFACT_EXTENSIONS = [".zip", ".exe", ".tar.gz", ".msi", ".pkg", ".deb", ".rpm"];

// isReleaseArtifactName reports whether a filename is a release artifact (and not
// the SHA256SUMS file itself or a per-file .sha256 sidecar).
export function isReleaseArtifactName(name) {
  const lower = name.toLowerCase();
  if (lower === SHA256SUMS_NAME.toLowerCase()) return false;
  if (lower.endsWith(".sha256")) return false;
  return ARTIFACT_EXTENSIONS.some((ext) => lower.endsWith(ext));
}

// findReleaseArtifacts returns the release artifact filenames in `dir` that belong
// to `version` (the version string appears in the artifact name), sorted
// deterministically. Stale artifacts from other versions are ignored.
export function findReleaseArtifacts(dir, version) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir)
    .filter((name) => {
      if (!statSync(join(dir, name)).isFile()) return false;
      if (!isReleaseArtifactName(name)) return false;
      return version ? name.includes(version) : true;
    })
    .sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
}

// sha256File returns the lowercase hex SHA-256 of a file's contents.
export function sha256File(filePath) {
  return createHash("sha256").update(readFileSync(filePath)).digest("hex");
}

// sha256Buffer returns the lowercase hex SHA-256 of a buffer/string (test helper).
export function sha256Buffer(data) {
  return createHash("sha256").update(data).digest("hex");
}

// formatSha256Sums renders entries ({ hash, name }) sorted by name, with a
// trailing newline.
export function formatSha256Sums(entries) {
  return (
    [...entries]
      .sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))
      .map((e) => `${e.hash.toLowerCase()}  ${e.name}`)
      .join("\n") + "\n"
  );
}

// generateChecksums computes SHA-256 for every version-scoped release artifact in
// `dir` and writes SHA256SUMS. Returns the written entries.
export function generateChecksums(dir, version) {
  const entries = findReleaseArtifacts(dir, version).map((name) => ({
    name,
    hash: sha256File(join(dir, name)),
  }));
  writeFileSync(join(dir, SHA256SUMS_NAME), formatSha256Sums(entries));
  return entries;
}

// parseSha256Sums parses SHA256SUMS text into [{ hash, name }]. Throws on malformed
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
