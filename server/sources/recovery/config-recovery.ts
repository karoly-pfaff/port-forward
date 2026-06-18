import { readFile, rename, stat } from "node:fs/promises";
import type { ForwardRule } from "@portier/shared";
import { parseConfig, SchemaInvalidConfigError } from "../persistence/config-parse.js";

/**
 * TypeScript/NestJS startup config-load recovery (v1.17 Slice 4), the parity
 * mirror of the Go `service/sources/recovery` package. It classifies how a
 * persisted config failed to load, preserves/quarantines a bad config where
 * safe, and produces an internal `RecoveryState` so startup can continue with no
 * active rules instead of throwing to `main().catch` before Nest listens.
 *
 * Scope: config-load recovery only. The duplicate-binding / per-rule autostart
 * recovery (Slice 3 parity) lives in the ForwardManager; surfacing the
 * `RecoveryState` on the public API is Slice 5. See docs/recovery.md.
 */

/** Machine-stable classification of a startup recovery condition. */
export type RecoveryReason =
  | "unreadable"
  | "malformed"
  | "schema-invalid"
  | "unsupported-version"
  | "duplicate-binding";

/**
 * An active startup recovery condition. Internal to the runtime (carried by the
 * ForwardManager so it can block writes); not exposed on the public API yet.
 */
export interface RecoveryState {
  reason: RecoveryReason;
  /** Operator-safe summary. Never contains config file contents. */
  message: string;
  configPath: string;
  /** Where the bad config was moved, or undefined when nothing was quarantined. */
  quarantinePath?: string;
  /** Rule persistence must be refused while true (no silent overwrite). */
  writesBlocked: boolean;
  detectedAt: Date;
}

export interface RecoveryLoadResult {
  rules: ForwardRule[];
  /** Undefined when the config loaded normally or was simply missing. */
  recovery?: RecoveryState;
}

/**
 * Minimal file-operation seam for the quarantine step, mirroring the
 * `ConfigStoreFileOps` philosophy in config-store.ts: it exposes only `rename`
 * so a test can force a quarantine failure without a real permission/IO event.
 * The default is the real `fs.rename`.
 */
export interface RecoveryFileOps {
  rename(from: string, to: string): Promise<void>;
}

export const defaultRecoveryFileOps: RecoveryFileOps = { rename };

/**
 * Load the persisted config, recovering from load failures instead of throwing.
 * Returns the rules to run with (empty when recovery is active) and an optional
 * `RecoveryState`. It never throws for a classified load failure — that is what
 * keeps the management API reachable.
 */
export async function loadConfigWithRecovery(
  configPath: string,
  now: Date = new Date(),
  fileOps: RecoveryFileOps = defaultRecoveryFileOps
): Promise<RecoveryLoadResult> {
  let raw: string;
  try {
    raw = await readFile(configPath, "utf8");
  } catch (error) {
    if (isNodeError(error) && error.code === "ENOENT") {
      // Missing config is the normal first-run state.
      return { rules: [] };
    }
    // Unreadable: bytes may be fine — do not quarantine. Preserve in place and
    // block writes so a later save cannot clobber a file we never read.
    return {
      rules: [],
      recovery: {
        reason: "unreadable",
        message:
          "Configuration file could not be read; started with no active rules. The original file was left untouched.",
        configPath,
        writesBlocked: true,
        detectedAt: now,
      },
    };
  }

  let rules: ForwardRule[];
  try {
    rules = parseConfig(raw);
  } catch (error) {
    const { reason, summary } = classifyParseError(error);
    const state: RecoveryState = {
      reason,
      message: summary,
      configPath,
      writesBlocked: true,
      detectedAt: now,
    };

    // Readable but bad: quarantine so the user's data is preserved and a fresh
    // empty config cannot silently replace it.
    const quarantinePath = await quarantine(configPath, now, fileOps);
    if (quarantinePath) {
      state.quarantinePath = quarantinePath;
      state.message = `${summary} The original file was quarantined.`;
    } else {
      state.message = `${summary} The original file could not be quarantined and was left in place.`;
    }
    return { rules: [], recovery: state };
  }

  return { rules };
}

function classifyParseError(error: unknown): { reason: RecoveryReason; summary: string } {
  if (error instanceof SchemaInvalidConfigError) {
    return {
      reason: "schema-invalid",
      summary: "Configuration file contains an invalid rule; started with no active rules.",
    };
  }
  // MalformedConfigError, or any unexpected parse failure, is treated as malformed.
  return {
    reason: "malformed",
    summary: "Configuration file could not be parsed; started with no active rules.",
  };
}

/**
 * Rename the bad config to a unique, timestamped name in the same directory
 * (same filesystem → atomic rename; preserves bytes). Never overwrites an
 * existing quarantine. Returns the quarantine path, or undefined if the rename
 * failed (the original is then left untouched). Mirrors the Go quarantine helper.
 */
async function quarantine(
  configPath: string,
  now: Date,
  fileOps: RecoveryFileOps
): Promise<string | undefined> {
  const base = `${configPath}.corrupt-${quarantineStamp(now)}`;
  let candidate = base;
  for (let attempt = 1; await pathExists(candidate); attempt += 1) {
    candidate = `${base}-${attempt}`;
  }

  try {
    await fileOps.rename(configPath, candidate);
    return candidate;
  } catch {
    return undefined;
  }
}

/** UTC timestamp in a filename-safe form, e.g. 2026-06-18T142530Z (no colons). */
function quarantineStamp(now: Date): string {
  const iso = now.toISOString();
  return `${iso.slice(0, 10)}T${iso.slice(11, 19).replace(/:/g, "")}Z`;
}

async function pathExists(path: string): Promise<boolean> {
  try {
    await stat(path);
    return true;
  } catch {
    return false;
  }
}

function isNodeError(error: unknown): error is NodeJS.ErrnoException {
  return error instanceof Error && "code" in error;
}
