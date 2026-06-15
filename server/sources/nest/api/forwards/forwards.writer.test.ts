import type { ForwardRule } from "@portier/shared";
import { describe, expect, it } from "vitest";
import {
  createDefaultForwardRuleCreator,
  createDefaultForwardRuleDeleter,
  createDefaultForwardRuleUpdater,
  InMemoryRuleStore,
} from "./forwards.writer.js";

const RULE: ForwardRule = {
  id: "r1",
  name: "Web",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48010,
  targetHost: "127.0.0.1",
  targetPort: 8080,
  enabled: false,
};

describe("InMemoryRuleStore", () => {
  it("round-trips rules through save/load (no disk)", async () => {
    const store = new InMemoryRuleStore();
    expect(await store.load()).toEqual([]);
    await store.save([RULE]);
    expect(await store.load()).toEqual([RULE]);
  });
});

describe("createDefaultForwardRuleCreator", () => {
  it("creates an isolated in-memory creator that can add a (stopped) rule", async () => {
    const creator = createDefaultForwardRuleCreator();
    const created = await creator.addRule({ ...RULE, id: undefined });
    expect(created.name).toBe("Web");
    expect(created.id).toBeTypeOf("string");
    expect(created.enabled).toBe(false);
  });

  it("returns a fresh, independent creator each call", async () => {
    const a = createDefaultForwardRuleCreator();
    const b = createDefaultForwardRuleCreator();
    await a.addRule({ ...RULE, id: "a-rule" });
    // b has not seen a's rule — adding the same binding to b does not conflict.
    const created = await b.addRule({ ...RULE, id: "b-rule" });
    expect(created.id).toBe("b-rule");
  });
});

describe("createDefaultForwardRuleUpdater", () => {
  it("creates an isolated in-memory updater that can update an existing rule", async () => {
    // The default uses a ForwardManager, which also creates; add then update.
    const updater = createDefaultForwardRuleUpdater() as unknown as {
      addRule(input: unknown): Promise<ForwardRule>;
      updateRule(id: string, patch: unknown): Promise<ForwardRule>;
    };
    await updater.addRule({ ...RULE, id: "u1" });
    const updated = await updater.updateRule("u1", { name: "Renamed" });
    expect(updated.id).toBe("u1");
    expect(updated.name).toBe("Renamed");
    // Unspecified fields are preserved (no undefined overwrite).
    expect(updated.listenPort).toBe(RULE.listenPort);
  });
});

describe("createDefaultForwardRuleDeleter", () => {
  it("creates an isolated in-memory deleter that can delete an existing rule", async () => {
    // The default uses a ForwardManager, which also creates; add then delete.
    const deleter = createDefaultForwardRuleDeleter() as unknown as {
      addRule(input: unknown): Promise<ForwardRule>;
      deleteRule(id: string): Promise<void>;
      listRules(): ForwardRule[];
    };
    await deleter.addRule({ ...RULE, id: "d1" });
    await deleter.deleteRule("d1");
    expect(deleter.listRules()).toEqual([]);
  });
});
