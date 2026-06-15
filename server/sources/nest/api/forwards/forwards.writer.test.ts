import type { ForwardRule } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { createDefaultForwardRuleCreator, InMemoryRuleStore } from "./forwards.writer.js";

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
