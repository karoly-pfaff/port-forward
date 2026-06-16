import type { ForwardRule } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { NotFoundError } from "../../forwarders/forward-manager.js";
import { ApiNotFoundException } from "../common/api-errors.js";
import { ReorderForwardRulesService } from "./reorder-forward-rules.service.js";
import type { ForwardRulesReorderer } from "./forwards.writer.js";

function rule(id: string): ForwardRule {
  return {
    id,
    name: id,
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort: 48010,
    targetHost: "127.0.0.1",
    targetPort: 8080,
    enabled: false,
  };
}

function service(reorderer: ForwardRulesReorderer): ReorderForwardRulesService {
  return new ReorderForwardRulesService(reorderer);
}

describe("ReorderForwardRulesService.reorder", () => {
  it("reorders then returns the full reordered list decorated with advisories", async () => {
    // A fake reorderer that records the requested order and returns it from listRules.
    let order: string[] = ["a", "b"];
    let receivedIds: string[] | undefined;
    const result = await service({
      reorderRules: async (ids) => {
        receivedIds = ids;
        order = ids;
      },
      listRules: () => order.map(rule),
    }).reorder(["b", "a"]);

    expect(receivedIds).toEqual(["b", "a"]);
    expect(result.map((r) => r.id)).toEqual(["b", "a"]);
    // Decorated with advisories (the same toForwardRuleResponse the list uses).
    expect(result[0]).toHaveProperty("advisories");
  });

  it("translates a manager NotFoundError (unknown id) to a 404 ApiNotFoundException", async () => {
    const reorderer: ForwardRulesReorderer = {
      reorderRules: async () => {
        throw new NotFoundError("Rule ghost was not found.");
      },
      listRules: () => [],
    };
    await expect(service(reorderer).reorder(["ghost"])).rejects.toBeInstanceOf(ApiNotFoundException);
  });

  it("re-throws an unexpected error (e.g. persist failure) unchanged → generic 500", async () => {
    const persistError = new Error("disk full");
    const reorderer: ForwardRulesReorderer = {
      reorderRules: async () => {
        throw persistError;
      },
      listRules: () => [],
    };
    await expect(service(reorderer).reorder(["a"])).rejects.toBe(persistError);
  });
});
