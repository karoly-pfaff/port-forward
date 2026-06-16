import type { ActivityEvent } from "@portier/shared";
import { describe, expect, it, vi } from "vitest";
import { ActivityController } from "./activity.controller.js";
import type { ActivityService } from "./activity.service.js";

describe("ActivityController.list", () => {
  it("delegates the query params to the service and maps events to the response DTO", () => {
    const events: ActivityEvent[] = [
      { id: "1", timestamp: "2026-06-14T00:00:00.000Z", type: "rule.error", severity: "error", message: "boom" },
    ];
    const list = vi.fn(() => events);
    const controller = new ActivityController({ list } as unknown as ActivityService);

    const result = controller.list("5", "rule-1", "rule.error", "error");

    expect(list).toHaveBeenCalledWith("5", "rule-1", "rule.error", "error");
    expect(result).toEqual({ events }); // byte-for-byte
    expect(result.events).not.toBe(events); // mapped copy
  });

  it("passes undefined through when no query params are supplied", () => {
    const list = vi.fn((): ActivityEvent[] => []);
    const controller = new ActivityController({ list } as unknown as ActivityService);

    expect(controller.list()).toEqual({ events: [] });
    expect(list).toHaveBeenCalledWith(undefined, undefined, undefined, undefined);
  });
});

describe("ActivityController.clear", () => {
  it("delegates to the service and returns nothing (204 no body)", () => {
    const clear = vi.fn();
    const controller = new ActivityController({ clear } as unknown as ActivityService);

    expect(controller.clear()).toBeUndefined();
    expect(clear).toHaveBeenCalledOnce();
  });
});
