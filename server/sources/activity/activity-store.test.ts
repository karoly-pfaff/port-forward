import { describe, expect, it } from "vitest";
import { ActivityStore } from "./activity-store.js";

const baseEvent = {
  type: "rule.started" as const,
  severity: "success" as const,
  ruleId: "r1",
  ruleName: "Test Rule",
  protocol: "tcp" as const,
  message: "Rule started."
};

describe("ActivityStore", () => {
  it("stores and retrieves events", () => {
    const store = new ActivityStore();
    store.add(baseEvent);
    const events = store.list();
    expect(events).toHaveLength(1);
    expect(events[0].type).toBe("rule.started");
    expect(events[0].severity).toBe("success");
    expect(events[0].message).toBe("Rule started.");
  });

  it("assigns a unique id and timestamp to each event", () => {
    const store = new ActivityStore();
    store.add(baseEvent);
    store.add({ ...baseEvent, message: "Second." });
    const events = store.list();
    expect(events[0].id).toBeTruthy();
    expect(events[1].id).toBeTruthy();
    expect(events[0].id).not.toBe(events[1].id);
    expect(events[0].timestamp).toMatch(/^\d{4}-\d{2}-\d{2}T/);
  });

  it("returns events newest first", () => {
    const store = new ActivityStore();
    store.add({ ...baseEvent, message: "First." });
    store.add({ ...baseEvent, message: "Second." });
    store.add({ ...baseEvent, message: "Third." });
    const events = store.list();
    expect(events[0].message).toBe("Third.");
    expect(events[1].message).toBe("Second.");
    expect(events[2].message).toBe("First.");
  });

  it("respects the limit parameter", () => {
    const store = new ActivityStore();
    for (let i = 0; i < 10; i++) {
      store.add({ ...baseEvent, message: `Event ${i}` });
    }
    const events = store.list({ limit: 3 });
    expect(events).toHaveLength(3);
    expect(events[0].message).toBe("Event 9");
  });

  it("filters by ruleId", () => {
    const store = new ActivityStore();
    store.add({ ...baseEvent, ruleId: "r1", message: "Rule 1 event." });
    store.add({ ...baseEvent, ruleId: "r2", message: "Rule 2 event." });
    store.add({ ...baseEvent, ruleId: "r1", message: "Rule 1 again." });
    const events = store.list({ ruleId: "r1" });
    expect(events).toHaveLength(2);
    expect(events.every((e) => e.ruleId === "r1")).toBe(true);
  });

  it("filters by type", () => {
    const store = new ActivityStore();
    store.add({ ...baseEvent, type: "rule.started" });
    store.add({ ...baseEvent, type: "rule.stopped" });
    store.add({ ...baseEvent, type: "rule.started" });
    const events = store.list({ type: "rule.started" });
    expect(events).toHaveLength(2);
    expect(events.every((e) => e.type === "rule.started")).toBe(true);
  });

  it("filters by severity", () => {
    const store = new ActivityStore();
    store.add({ ...baseEvent, severity: "success" });
    store.add({ ...baseEvent, severity: "error" });
    store.add({ ...baseEvent, severity: "success" });
    const events = store.list({ severity: "error" });
    expect(events).toHaveLength(1);
    expect(events[0].severity).toBe("error");
  });

  it("combines multiple filters", () => {
    const store = new ActivityStore();
    store.add({ ...baseEvent, ruleId: "r1", severity: "error", message: "r1 error" });
    store.add({ ...baseEvent, ruleId: "r2", severity: "error", message: "r2 error" });
    store.add({ ...baseEvent, ruleId: "r1", severity: "success", message: "r1 success" });
    const events = store.list({ ruleId: "r1", severity: "error" });
    expect(events).toHaveLength(1);
    expect(events[0].message).toBe("r1 error");
  });

  it("is bounded to 500 events", () => {
    const store = new ActivityStore();
    for (let i = 0; i < 550; i++) {
      store.add({ ...baseEvent, message: `Event ${i}` });
    }
    const events = store.list({ limit: 500 });
    expect(events).toHaveLength(500);
    // Should keep the newest 500 (events 50–549)
    expect(events[0].message).toBe("Event 549");
  });

  it("clear empties the store", () => {
    const store = new ActivityStore();
    store.add(baseEvent);
    store.clear();
    expect(store.list()).toHaveLength(0);
  });

  it("applies default limit of 100", () => {
    const store = new ActivityStore();
    for (let i = 0; i < 150; i++) {
      store.add({ ...baseEvent, message: `Event ${i}` });
    }
    const events = store.list();
    expect(events).toHaveLength(100);
  });
});
