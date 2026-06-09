import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createConsoleLogger, errorFields } from "./logger.js";

describe("createConsoleLogger", () => {
  beforeEach(() => {
    vi.spyOn(console, "log").mockImplementation(() => {});
    vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("info logs to console.log as JSON with correct fields", () => {
    const logger = createConsoleLogger();
    logger.info("test.event", "Hello");
    expect(console.log).toHaveBeenCalledOnce();
    const arg = (vi.mocked(console.log).mock.calls[0][0] as string);
    const entry = JSON.parse(arg) as Record<string, unknown>;
    expect(entry.level).toBe("info");
    expect(entry.event).toBe("test.event");
    expect(entry.message).toBe("Hello");
    expect(typeof entry.timestamp).toBe("string");
  });

  it("warn logs to console.log as JSON", () => {
    const logger = createConsoleLogger();
    logger.warn("test.warn", "Warning message");
    expect(console.log).toHaveBeenCalledOnce();
    const entry = JSON.parse(vi.mocked(console.log).mock.calls[0][0] as string) as Record<string, unknown>;
    expect(entry.level).toBe("warn");
    expect(entry.message).toBe("Warning message");
  });

  it("error logs to console.error as JSON", () => {
    const logger = createConsoleLogger();
    logger.error("test.error", "Error message");
    expect(console.error).toHaveBeenCalledOnce();
    expect(console.log).not.toHaveBeenCalled();
    const entry = JSON.parse(vi.mocked(console.error).mock.calls[0][0] as string) as Record<string, unknown>;
    expect(entry.level).toBe("error");
    expect(entry.message).toBe("Error message");
  });

  it("includes additional fields in the log entry", () => {
    const logger = createConsoleLogger();
    logger.info("test.fields", "With fields", { ruleId: "r1", count: 3 });
    const entry = JSON.parse(vi.mocked(console.log).mock.calls[0][0] as string) as Record<string, unknown>;
    expect(entry.ruleId).toBe("r1");
    expect(entry.count).toBe(3);
  });
});

describe("errorFields", () => {
  it("returns errorName, errorMessage, stack for Error instances", () => {
    const err = new Error("something went wrong");
    const fields = errorFields(err);
    expect(fields.errorName).toBe("Error");
    expect(fields.errorMessage).toBe("something went wrong");
    expect(typeof fields.stack).toBe("string");
  });

  it("returns errorMessage as String for non-Error values", () => {
    expect(errorFields("string error").errorMessage).toBe("string error");
    expect(errorFields(42).errorMessage).toBe("42");
    expect(errorFields(null).errorMessage).toBe("null");
  });
});
