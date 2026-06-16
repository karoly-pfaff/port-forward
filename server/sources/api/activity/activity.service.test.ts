import type { ActivityEvent } from "@portier/shared";
import { describe, expect, it } from "vitest";
import type { ActivityListParams } from "../../activity/activity-store.js";
import { ActivityService, parseActivityLimit } from "./activity.service.js";
import type { ActivityClearer, ActivityReader } from "./activity.reader.js";

/** Records the params/clears it was asked for and returns a fixed event list. */
class RecordingStore implements ActivityReader, ActivityClearer {
  lastParams?: ActivityListParams;
  cleared = false;
  readonly events: ActivityEvent[] = [
    { id: "1", timestamp: "2026-06-14T00:00:00.000Z", type: "rule.started", severity: "success", message: "m" },
  ];

  list(params: ActivityListParams): ActivityEvent[] {
    this.lastParams = params;
    return this.events;
  }

  clear(): void {
    this.cleared = true;
  }
}

describe("parseActivityLimit", () => {
  it.each([
    [undefined, 100],
    ["", 100],
    ["abc", 100],
    ["0", 100],
    ["-3", 100],
    ["5.5", 100],
    ["5", 5],
    ["500", 500],
    ["600", 500],
  ])("coerces %j to %d", (raw, expected) => {
    expect(parseActivityLimit(raw)).toBe(expected);
  });
});

describe("ActivityService.list", () => {
  it("defaults all params and returns the store events", () => {
    const store = new RecordingStore();
    const service = new ActivityService(store);

    const result = service.list();

    expect(result).toBe(store.events);
    expect(store.lastParams).toEqual({ limit: 100, ruleId: undefined, type: undefined, severity: undefined });
  });

  it("passes through the parsed filters and clamps the limit", () => {
    const store = new RecordingStore();
    const service = new ActivityService(store);

    service.list("600", "rule-1", "rule.error", "error");

    expect(store.lastParams).toEqual({
      limit: 500,
      ruleId: "rule-1",
      type: "rule.error",
      severity: "error",
    });
  });
});

describe("ActivityService.clear", () => {
  it("clears the underlying store", () => {
    const store = new RecordingStore();
    const service = new ActivityService(store);

    service.clear();

    expect(store.cleared).toBe(true);
  });
});
