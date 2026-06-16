import { describe, expect, it } from "vitest";
import { emptyConfigExportReader } from "./config-export.reader.js";

describe("emptyConfigExportReader", () => {
  it("returns an empty rule list (default — no runtime wired)", () => {
    expect(emptyConfigExportReader.listRules()).toEqual([]);
  });
});
