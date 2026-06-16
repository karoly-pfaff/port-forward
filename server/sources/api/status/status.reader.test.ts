import { describe, expect, it } from "vitest";
import { emptyStatusReader } from "./status.reader.js";

describe("emptyStatusReader", () => {
  it("returns an empty status list (default — no runtime wired)", () => {
    expect(emptyStatusReader.listStatus()).toEqual([]);
  });
});
