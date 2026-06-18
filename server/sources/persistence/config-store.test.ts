import { mkdir, mkdtemp, readdir, readFile, rm, stat, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { afterEach, describe, expect, it } from "vitest";
import {
  ConfigStore,
  defaultFileOps,
  type ConfigFileHandle,
  type ConfigStoreFileOps
} from "./config-store.js";
import type { ForwardRule } from "@portier/shared";

const tempDirs: string[] = [];

afterEach(async () => {
  await Promise.all(tempDirs.splice(0).map((dir) => rm(dir, { recursive: true, force: true })));
});

async function pathExists(path: string): Promise<boolean> {
  try {
    await stat(path);
    return true;
  } catch {
    return false;
  }
}

async function tempFilesIn(dir: string): Promise<string[]> {
  const entries = await readdir(dir);
  return entries.filter((name) => name.endsWith(".tmp"));
}

const sampleRules: ForwardRule[] = [
  {
    id: "rule-1",
    name: "TCP local",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort: 8080,
    targetHost: "127.0.0.1",
    targetPort: 8081,
    enabled: true
  }
];

// InjectableFileOps wraps the real fs-backed defaultFileOps but can fail at a
// precise step of the atomic-write path (write / sync / rename), so durability
// behavior can be tested without a real disk-full event. The temp file is still
// genuinely created on disk by the real delegate, so cleanup is observable.
class InjectableFileOps implements ConfigStoreFileOps {
  lastTempPath: string | undefined;
  removed: string[] = [];
  failWrite = false;
  failSync = false;
  failRename = false;

  constructor(private readonly delegate: ConfigStoreFileOps = defaultFileOps) {}

  async mkdir(dir: string): Promise<void> {
    await this.delegate.mkdir(dir);
  }

  async open(path: string): Promise<ConfigFileHandle> {
    this.lastTempPath = path;
    const handle = await this.delegate.open(path);
    return {
      write: async (data) => {
        if (this.failWrite) throw new Error("simulated write failure");
        await handle.write(data);
      },
      sync: async () => {
        if (this.failSync) throw new Error("simulated sync failure");
        await handle.sync();
      },
      close: async () => {
        await handle.close();
      }
    };
  }

  async rename(from: string, to: string): Promise<void> {
    if (this.failRename) throw new Error("simulated rename failure");
    await this.delegate.rename(from, to);
  }

  async remove(path: string): Promise<void> {
    this.removed.push(path);
    await this.delegate.remove(path);
  }
}

describe("ConfigStore", () => {
  it("returns an empty list when the config file does not exist", async () => {
    const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
    tempDirs.push(dir);
    const store = new ConfigStore(join(dir, "forwards.json"));

    await expect(store.load()).resolves.toEqual([]);
  });

  it("throws when the config file contains invalid JSON", async () => {
    const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
    tempDirs.push(dir);
    const filePath = join(dir, "forwards.json");
    await writeFile(filePath, "this is not json", "utf8");
    const store = new ConfigStore(filePath);
    await expect(store.load()).rejects.toThrow();
  });

  it("throws when the config file contains a non-array JSON value", async () => {
    const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
    tempDirs.push(dir);
    const filePath = join(dir, "forwards.json");
    await writeFile(filePath, JSON.stringify({ rules: [] }), "utf8");
    const store = new ConfigStore(filePath);
    await expect(store.load()).rejects.toThrow("Config file must contain an array of forward rules.");
  });

  it("throws when the config file contains an invalid rule", async () => {
    const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
    tempDirs.push(dir);
    const filePath = join(dir, "forwards.json");
    await writeFile(filePath, JSON.stringify([{ name: "", protocol: "invalid" }]), "utf8");
    const store = new ConfigStore(filePath);
    await expect(store.load()).rejects.toThrow("Invalid rule at index 0");
  });

  it("rethrows a non-ENOENT read error from load (e.g. the path is a directory)", async () => {
    const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
    tempDirs.push(dir);
    const asDir = join(dir, "rules-as-dir.json");
    await mkdir(asDir);
    // Reading a directory fails with EISDIR (not ENOENT) on all platforms.
    await expect(new ConfigStore(asDir).load()).rejects.toThrow();
  });

  it("loadWithRecovery recovers a malformed config (empty rules + recovery state + quarantine)", async () => {
    const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
    tempDirs.push(dir);
    const filePath = join(dir, "forwards.json");
    await writeFile(filePath, "this is not json", "utf8");

    const result = await new ConfigStore(filePath).loadWithRecovery();
    expect(result.rules).toEqual([]);
    expect(result.recovery?.reason).toBe("malformed");
    expect(result.recovery?.writesBlocked).toBe(true);
    expect(result.recovery?.quarantinePath).toBeDefined();
    // Original moved to the quarantine path.
    expect(await pathExists(filePath)).toBe(false);
  });

  it("loadWithRecovery returns no recovery state for a valid config", async () => {
    const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
    tempDirs.push(dir);
    const filePath = join(dir, "forwards.json");
    await writeFile(filePath, JSON.stringify(sampleRules), "utf8");

    const result = await new ConfigStore(filePath).loadWithRecovery();
    expect(result.recovery).toBeUndefined();
    expect(result.rules).toEqual(sampleRules);
  });

  it("saves and loads rules", async () => {
    const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
    tempDirs.push(dir);
    const store = new ConfigStore(join(dir, "nested", "forwards.json"));
    const rules: ForwardRule[] = [
      {
        id: "rule-1",
        name: "TCP local",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort: 8080,
        targetHost: "127.0.0.1",
        targetPort: 8081,
        enabled: true
      }
    ];

    await store.save(rules);

    await expect(store.load()).resolves.toEqual(rules);
  });

  describe("atomic save (Go parity)", () => {
    it("writes pretty-printed JSON with a trailing newline and leaves no temp file", async () => {
      const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
      tempDirs.push(dir);
      const filePath = join(dir, "forwards.json");
      const store = new ConfigStore(filePath);

      await store.save(sampleRules);

      const raw = await readFile(filePath, "utf8");
      expect(raw).toBe(`${JSON.stringify(sampleRules, null, 2)}\n`);
      expect(raw.endsWith("\n")).toBe(true);
      // No temp file is left behind on success.
      expect(await tempFilesIn(dir)).toEqual([]);
      await expect(store.load()).resolves.toEqual(sampleRules);
    });

    it("preserves the existing rules.json when the write fails before rename", async () => {
      const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
      tempDirs.push(dir);
      const filePath = join(dir, "forwards.json");

      // Establish a known-good existing config.
      const original = new ConfigStore(filePath);
      await original.save(sampleRules);
      const before = await readFile(filePath, "utf8");

      const fileOps = new InjectableFileOps();
      fileOps.failWrite = true;
      const store = new ConfigStore(filePath, fileOps);

      const next: ForwardRule[] = [{ ...sampleRules[0], id: "rule-2", name: "Changed", listenPort: 9090 }];
      await expect(store.save(next)).rejects.toThrow("simulated write failure");

      // The original file is byte-for-byte unchanged and still loads.
      expect(await readFile(filePath, "utf8")).toBe(before);
      await expect(new ConfigStore(filePath).load()).resolves.toEqual(sampleRules);
      // The temp file was cleaned up.
      expect(fileOps.removed).toContain(fileOps.lastTempPath);
      expect(await pathExists(fileOps.lastTempPath as string)).toBe(false);
      expect(await tempFilesIn(dir)).toEqual([]);
    });

    it("propagates a sync failure and cleans up the temp file", async () => {
      const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
      tempDirs.push(dir);
      const filePath = join(dir, "forwards.json");

      const fileOps = new InjectableFileOps();
      fileOps.failSync = true;
      const store = new ConfigStore(filePath, fileOps);

      await expect(store.save(sampleRules)).rejects.toThrow("simulated sync failure");
      expect(await pathExists(filePath)).toBe(false);
      expect(fileOps.removed).toContain(fileOps.lastTempPath);
      expect(await tempFilesIn(dir)).toEqual([]);
    });

    it("removes the temp file and keeps the target unchanged when rename fails", async () => {
      const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
      tempDirs.push(dir);
      const filePath = join(dir, "forwards.json");

      const original = new ConfigStore(filePath);
      await original.save(sampleRules);
      const before = await readFile(filePath, "utf8");

      const fileOps = new InjectableFileOps();
      fileOps.failRename = true;
      const store = new ConfigStore(filePath, fileOps);

      const next: ForwardRule[] = [{ ...sampleRules[0], id: "rule-2", name: "Changed" }];
      await expect(store.save(next)).rejects.toThrow("simulated rename failure");

      // Target unchanged; temp (which was fully written) is removed.
      expect(await readFile(filePath, "utf8")).toBe(before);
      expect(fileOps.removed).toContain(fileOps.lastTempPath);
      expect(await pathExists(fileOps.lastTempPath as string)).toBe(false);
      expect(await tempFilesIn(dir)).toEqual([]);
    });

    it("creates no target file when the first save fails before rename", async () => {
      const dir = await mkdtemp(join(tmpdir(), "portier-config-"));
      tempDirs.push(dir);
      const filePath = join(dir, "nested", "forwards.json");

      const fileOps = new InjectableFileOps();
      fileOps.failWrite = true;
      const store = new ConfigStore(filePath, fileOps);

      await expect(store.save(sampleRules)).rejects.toThrow("simulated write failure");
      expect(await pathExists(filePath)).toBe(false);
      expect(fileOps.removed).toContain(fileOps.lastTempPath);
      expect(await pathExists(fileOps.lastTempPath as string)).toBe(false);
    });
  });
});
