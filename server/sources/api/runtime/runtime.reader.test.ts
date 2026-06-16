import { describe, expect, it } from "vitest";
import { createDefaultRuntimeInfoReader, defaultProcessReader } from "./runtime.reader.js";

// defaultClockReader now lives in (and is tested under) common/clock.reader.ts.

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
