import type { ForwardRule } from "@portier/shared";
import { describe, expect, it } from "vitest";
import {
  createDefaultForwardGroupStopper,
  createDefaultForwardRuleCreator,
  createDefaultForwardRuleDeleter,
  createDefaultForwardRulesReorderer,
  createDefaultForwardRuleStarter,
  createDefaultForwardRuleStopper,
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

describe("createDefaultForwardRuleStarter", () => {
  it("creates an isolated in-memory starter that returns 404-able NotFound for an unknown id (no runtime wired)", async () => {
    // The scaffold default is an empty ForwardManager — starting an unknown id
    // rejects with NotFoundError (no rule, no socket opened).
    const starter = createDefaultForwardRuleStarter();
    await expect(starter.startRule("missing")).rejects.toThrow(/not found/i);
  });
});

describe("createDefaultForwardRuleStopper", () => {
  it("creates an isolated in-memory stopper that returns 404-able NotFound for an unknown id (no runtime wired)", async () => {
    // The scaffold default is an empty ForwardManager — stopping an unknown id
    // rejects with NotFoundError (no rule, no socket touched).
    const stopper = createDefaultForwardRuleStopper();
    await expect(stopper.stopRule("missing")).rejects.toThrow(/not found/i);
  });
});

describe("createDefaultForwardGroupStopper", () => {
  it("returns an empty result array for a group with no rules (the route maps that to 404)", async () => {
    const stopper = createDefaultForwardGroupStopper();
    expect(await stopper.stopGroup("nope")).toEqual([]);
  });

  it("stops a seeded group's stopped rules as not_running skips (no sockets)", async () => {
    const stopper = createDefaultForwardGroupStopper() as unknown as {
      addRule(input: unknown): Promise<ForwardRule>;
      stopGroup(group: string): Promise<Array<{ ruleId: string; status: string; reason?: string }>>;
    };
    await stopper.addRule({ ...RULE, id: "g1", group: "web" });
    const results = await stopper.stopGroup("web");
    expect(results).toEqual([{ ruleId: "g1", ruleName: "Web", status: "skipped", reason: "not_running" }]);
  });
});

describe("createDefaultForwardRulesReorderer", () => {
  it("creates an isolated in-memory reorderer that no-ops on an empty list and lists rules", async () => {
    const reorderer = createDefaultForwardRulesReorderer();
    await reorderer.reorderRules([]); // empty list is a no-op (no rules yet)
    expect(reorderer.listRules()).toEqual([]);
  });

  it("rejects an unknown id with NotFound (no rule wired)", async () => {
    const reorderer = createDefaultForwardRulesReorderer();
    await expect(reorderer.reorderRules(["missing"])).rejects.toThrow(/not found/i);
  });
});
