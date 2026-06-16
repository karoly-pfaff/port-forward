import { describe, expect, it } from "vitest";
import { defaultClockReader } from "./clock.reader.js";

describe("defaultClockReader", () => {
  it("returns the current wall-clock time", () => {
    const before = Date.now();
    const now = defaultClockReader.now();
    const after = Date.now();
    expect(now).toBeInstanceOf(Date);
    expect(now.getTime()).toBeGreaterThanOrEqual(before);
    expect(now.getTime()).toBeLessThanOrEqual(after);
  });
});
