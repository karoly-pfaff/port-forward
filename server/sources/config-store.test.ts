import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { afterEach, describe, expect, it } from "vitest";
import { ConfigStore } from "./config-store.js";
import type { ForwardRule } from "@portier/shared";

const tempDirs: string[] = [];

afterEach(async () => {
  await Promise.all(tempDirs.splice(0).map((dir) => rm(dir, { recursive: true, force: true })));
});

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
});
