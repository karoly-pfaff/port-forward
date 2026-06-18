import { mkdir, mkdtemp, readFile, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { loadConfigWithRecovery } from "./config-recovery.js";

const VALID_CONFIG = JSON.stringify([
  {
    id: "rule-1",
    name: "Local app",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort: 48001,
    targetHost: "127.0.0.1",
    targetPort: 3000,
    enabled: true,
  },
]);

const FIXED_NOW = new Date("2026-06-18T14:25:30.000Z");

describe("loadConfigWithRecovery", () => {
  const dirs: string[] = [];

  afterEach(() => {
    dirs.splice(0);
  });

  async function tempConfig(content?: string): Promise<string> {
    const dir = await mkdtemp(join(tmpdir(), "portier-recovery-"));
    dirs.push(dir);
    const path = join(dir, "rules.json");
    if (content !== undefined) {
      await writeFile(path, content, "utf8");
    }
    return path;
  }

  async function exists(path: string): Promise<boolean> {
    try {
      await stat(path);
      return true;
    } catch {
      return false;
    }
  }

  it("missing config is normal (empty rules, no recovery)", async () => {
    const path = join(await mkdtemp(join(tmpdir(), "portier-recovery-")), "missing.json");
    const result = await loadConfigWithRecovery(path, FIXED_NOW);
    expect(result.recovery).toBeUndefined();
    expect(result.rules).toEqual([]);
  });

  it("valid config loads normally (no recovery)", async () => {
    const path = await tempConfig(VALID_CONFIG);
    const result = await loadConfigWithRecovery(path, FIXED_NOW);
    expect(result.recovery).toBeUndefined();
    expect(result.rules).toHaveLength(1);
    expect(result.rules[0].id).toBe("rule-1");
  });

  it("malformed JSON quarantines the original and recovers with empty rules", async () => {
    const path = await tempConfig("this is not json");
    const result = await loadConfigWithRecovery(path, FIXED_NOW);

    expect(result.rules).toEqual([]);
    expect(result.recovery?.reason).toBe("malformed");
    expect(result.recovery?.writesBlocked).toBe(true);

    // Original moved away; bad bytes preserved in the quarantine file.
    expect(await exists(path)).toBe(false);
    expect(result.recovery?.quarantinePath).toBeDefined();
    expect(await readFile(result.recovery!.quarantinePath!, "utf8")).toBe("this is not json");
  });

  it("wrong top-level type is classified malformed and quarantined", async () => {
    const path = await tempConfig(JSON.stringify({ version: "1" }));
    const result = await loadConfigWithRecovery(path, FIXED_NOW);
    expect(result.recovery?.reason).toBe("malformed");
    expect(result.recovery?.quarantinePath).toBeDefined();
    expect(result.rules).toEqual([]);
  });

  it("schema-invalid config recovers with no partial salvage", async () => {
    // One valid rule + one invalid rule: the whole file is rejected.
    const path = await tempConfig(
      JSON.stringify([
        {
          id: "good",
          name: "Good",
          protocol: "tcp",
          listenHost: "127.0.0.1",
          listenPort: 48001,
          targetHost: "127.0.0.1",
          targetPort: 3000,
          enabled: true,
        },
        { name: "", protocol: "invalid" },
      ])
    );
    const result = await loadConfigWithRecovery(path, FIXED_NOW);
    expect(result.recovery?.reason).toBe("schema-invalid");
    expect(result.recovery?.writesBlocked).toBe(true);
    expect(result.recovery?.quarantinePath).toBeDefined();
    expect(result.rules).toEqual([]);
  });

  it("unreadable config (a directory) is preserved in place, not quarantined", async () => {
    // A directory at the config path makes readFile fail with a non-ENOENT error
    // on all platforms — a portable stand-in for an unreadable file.
    const dir = await mkdtemp(join(tmpdir(), "portier-recovery-"));
    const path = join(dir, "rules-as-dir.json");
    await mkdir(path);

    const result = await loadConfigWithRecovery(path, FIXED_NOW);
    expect(result.recovery?.reason).toBe("unreadable");
    expect(result.recovery?.quarantinePath).toBeUndefined();
    expect(result.recovery?.writesBlocked).toBe(true);
    expect(result.rules).toEqual([]);
    expect(await exists(path)).toBe(true);
  });

  it("keeps recovery active and preserves the original when quarantine fails", async () => {
    const path = await tempConfig("not json");
    const result = await loadConfigWithRecovery(path, FIXED_NOW, {
      rename: async () => {
        throw new Error("simulated rename failure");
      },
    });

    expect(result.recovery?.reason).toBe("malformed");
    expect(result.recovery?.quarantinePath).toBeUndefined();
    expect(result.recovery?.message).toContain("left in place");
    expect(result.rules).toEqual([]);
    // Original must survive a failed quarantine (never deleted/overwritten).
    expect(await exists(path)).toBe(true);
    expect(await readFile(path, "utf8")).toBe("not json");
  });

  it("never overwrites an existing quarantine file", async () => {
    const path = await tempConfig("not json");
    // Pre-create the name the first quarantine would pick.
    const taken = `${path}.corrupt-2026-06-18T142530Z`;
    await writeFile(taken, "PRIOR", "utf8");

    const result = await loadConfigWithRecovery(path, FIXED_NOW);
    expect(result.recovery?.quarantinePath).toBeDefined();
    expect(result.recovery?.quarantinePath).not.toBe(taken);
    // Prior quarantine untouched.
    expect(await readFile(taken, "utf8")).toBe("PRIOR");
  });
});
