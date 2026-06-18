import crypto from "node:crypto";
import { mkdir, open, readFile, rename, rm } from "node:fs/promises";
import { dirname, join } from "node:path";
import type { ForwardRule } from "@portier/shared";
import { parseConfig } from "./config-parse.js";
import { loadConfigWithRecovery, type RecoveryLoadResult } from "../recovery/config-recovery.js";

/**
 * Minimal file-operation seam used by {@link ConfigStore.save}. It exposes only
 * the operations the atomic-write path needs, so tests can inject a fake that
 * fails at a precise step (write / sync / close / rename) without a real
 * disk-full event. This is intentionally NOT a general virtual filesystem —
 * `load()` still uses `node:fs/promises` directly. The default implementation
 * (`defaultFileOps`) is the real `fs/promises` backing.
 */
export interface ConfigFileHandle {
  write(data: string): Promise<void>;
  sync(): Promise<void>;
  close(): Promise<void>;
}

export interface ConfigStoreFileOps {
  mkdir(dir: string): Promise<void>;
  open(path: string): Promise<ConfigFileHandle>;
  rename(from: string, to: string): Promise<void>;
  remove(path: string): Promise<void>;
}

export const defaultFileOps: ConfigStoreFileOps = {
  async mkdir(dir) {
    await mkdir(dir, { recursive: true });
  },
  async open(path) {
    const handle = await open(path, "w");
    return {
      async write(data) {
        await handle.write(data);
      },
      async sync() {
        await handle.sync();
      },
      async close() {
        await handle.close();
      }
    };
  },
  async rename(from, to) {
    await rename(from, to);
  },
  async remove(path) {
    await rm(path, { force: true });
  }
};

export class ConfigStore {
  constructor(
    private readonly filePath: string,
    private readonly fileOps: ConfigStoreFileOps = defaultFileOps
  ) {}

  async load(): Promise<ForwardRule[]> {
    let raw: string;
    try {
      raw = await readFile(this.filePath, "utf8");
    } catch (error) {
      if (isNodeError(error) && error.code === "ENOENT") {
        return [];
      }
      throw error;
    }
    return parseConfig(raw);
  }

  /**
   * Startup load that recovers from load failures instead of throwing (v1.17
   * Slice 4, R-1): a malformed/schema-invalid/unreadable config returns empty
   * rules plus a recovery state (and quarantines the bad file where safe) so the
   * management API can still start. Delegates to the recovery module so the
   * classification/quarantine policy stays in one place, parity-aligned with Go.
   */
  async loadWithRecovery(now: Date = new Date()): Promise<RecoveryLoadResult> {
    return loadConfigWithRecovery(this.filePath, now);
  }

  /**
   * Persist rules atomically, mirroring the Go service `config.Store.Save`
   * (`service/sources/config/config.go`): write to a unique temp file in the
   * SAME directory as the target, fsync the contents, close, then atomically
   * rename the temp file over the target. A crash, disk-full, or interrupted
   * write therefore cannot truncate or corrupt the previous `rules.json` — the
   * old file stays intact until the rename succeeds. The temp file is removed
   * on any failure before the rename. The caller (ForwardManager) receives the
   * original persistence error so its in-memory rollback can run.
   *
   * Unlike Go, no remove-and-retry recovery branch is needed after `rename`:
   * Node's `fs.rename` replaces an existing destination atomically on the same
   * filesystem on both POSIX (`rename(2)`) and Windows (libuv `MoveFileExW` with
   * `MOVEFILE_REPLACE_EXISTING`). A best-effort directory fsync is intentionally
   * omitted (not portable on Windows); the metadata-durability gap that leaves
   * is the same one the Go runtime accepts.
   */
  async save(rules: ForwardRule[]): Promise<void> {
    const dir = dirname(this.filePath);
    await this.fileOps.mkdir(dir);

    const data = `${JSON.stringify(rules, null, 2)}\n`;
    const tempPath = join(
      dir,
      `.portier-forwards-${process.pid}-${Date.now()}-${crypto.randomBytes(6).toString("hex")}.tmp`
    );

    let renamed = false;
    try {
      const handle = await this.fileOps.open(tempPath);
      try {
        await handle.write(data);
        await handle.sync();
      } catch (error) {
        // Best-effort close so a failed write/sync does not leak the handle;
        // the original error must still propagate (parity with Go, which
        // ignores the close error on the write/sync failure path).
        await handle.close().catch(() => {});
        throw error;
      }
      // A close error here IS surfaced (matches Go returning the Close error).
      await handle.close();

      await this.fileOps.rename(tempPath, this.filePath);
      renamed = true;
    } finally {
      if (!renamed) {
        // Clean up the temp file on any pre-rename failure. Best-effort so a
        // cleanup error cannot mask the original persistence error.
        await this.fileOps.remove(tempPath).catch(() => {});
      }
    }
  }
}

function isNodeError(error: unknown): error is NodeJS.ErrnoException {
  return error instanceof Error && "code" in error;
}
