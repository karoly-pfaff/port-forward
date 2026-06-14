import { describe, expect, it } from "vitest";
import {
  createDefaultRuntimeInfoReader,
  defaultClockReader,
  defaultProcessReader,
} from "./runtime.reader.js";

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

describe("defaultProcessReader", () => {
  it("reports the real process pid/platform/arch", () => {
    expect(defaultProcessReader.pid()).toBe(process.pid);
    expect(defaultProcessReader.platform()).toBe(process.platform);
    expect(defaultProcessReader.arch()).toBe(process.arch);
  });
});

describe("createDefaultRuntimeInfoReader", () => {
  it("has no options and a stable start time captured once at construction", () => {
    const reader = createDefaultRuntimeInfoReader();
    expect(reader.options()).toBeUndefined();
    const first = reader.startedAt();
    expect(first).toBeInstanceOf(Date);
    // Resolved once (mirrors Express's app-creation-time runtimeStartedAt).
    expect(reader.startedAt()).toBe(first);
  });

  it("creates an independent start time per reader", () => {
    const a = createDefaultRuntimeInfoReader();
    const b = createDefaultRuntimeInfoReader();
    expect(a.startedAt()).not.toBe(b.startedAt());
  });
});
