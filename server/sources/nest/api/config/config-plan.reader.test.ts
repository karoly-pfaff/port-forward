import { describe, expect, it } from "vitest";
import { emptyConfigPlanReader } from "./config-plan.reader.js";

describe("emptyConfigPlanReader", () => {
  it("returns an empty current rule list (no runtime wired)", () => {
    expect(emptyConfigPlanReader.listRules()).toEqual([]);
  });
});
