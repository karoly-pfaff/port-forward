import { describe, expect, it } from "vitest";
import { emptyForwardsReader } from "./forwards.reader.js";

describe("emptyForwardsReader", () => {
  it("returns an empty rule list (default — no runtime wired)", () => {
    expect(emptyForwardsReader.listRules()).toEqual([]);
  });
});
