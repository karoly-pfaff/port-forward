import type { ActivityEvent } from "@portier/shared";
import { describe, expect, it, vi } from "vitest";
import { ActivityController } from "./activity.controller.js";
import type { ActivityService } from "./activity.service.js";

describe("ActivityController.list", () => {
  it("delegates the query params to the service and returns its result", () => {
    const events: ActivityEvent[] = [
      { id: "1", timestamp: "2026-06-14T00:00:00.000Z", type: "rule.error", severity: "error", message: "boom" },
    ];
    const list = vi.fn(() => ({ events }));
    const controller = new ActivityController({ list } as unknown as ActivityService);

    expect(controller.list("5", "rule-1", "rule.error", "error")).toEqual({ events });
    expect(list).toHaveBeenCalledWith("5", "rule-1", "rule.error", "error");
  });

  it("passes undefined through when no query params are supplied", () => {
    const list = vi.fn(() => ({ events: [] }));
    const controller = new ActivityController({ list } as unknown as ActivityService);

    expect(controller.list()).toEqual({ events: [] });
    expect(list).toHaveBeenCalledWith(undefined, undefined, undefined, undefined);
  });
});
