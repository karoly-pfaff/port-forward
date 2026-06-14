import { describe, expect, it } from "vitest";
import { emptyStatusReader } from "./status.reader.js";

describe("emptyStatusReader", () => {
  it("returns an empty status list (scaffold default — no runtime wired)", () => {
    expect(emptyStatusReader.listStatus()).toEqual([]);
  });
});
