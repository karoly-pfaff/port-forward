import { describe, expect, it } from "vitest";
import type { AppRuntime } from "./runtime-context.js";
import { DEFAULT_SERVER_LOG_LEVELS, resolveLoggerOption } from "./app.factory.js";

// resolveLoggerOption only checks the truthiness of `runtime`, so a minimal cast suffices.
const liveRuntime = {} as AppRuntime;

describe("resolveLoggerOption", () => {
  it("defaults to silent (false) when there is no live runtime", () => {
    expect(resolveLoggerOption(undefined, {})).toBe(false);
  });

  it("defaults to the server log levels when a live runtime is present", () => {
    expect(resolveLoggerOption(liveRuntime, {})).toEqual(DEFAULT_SERVER_LOG_LEVELS);
  });

  it("honours an explicit `false` override even with a live runtime", () => {
    expect(resolveLoggerOption(liveRuntime, { logger: false })).toBe(false);
  });

  it("honours an explicit log-level override", () => {
    expect(resolveLoggerOption(undefined, { logger: ["error"] })).toEqual(["error"]);
  });
});
