import type { ActivityEvent, ActivityEventInput } from "@portier/shared";
import { describe, expect, it } from "vitest";
import {
  createConfigExportRecorder,
  noopConfigExportRecorder,
} from "./config-export.recorder.js";
import type { ActivityStore } from "../../activity/activity-store.js";

describe("noopConfigExportRecorder", () => {
  it("records nothing (the static/OpenAPI/test default)", () => {
    // Must not throw and has no observable effect.
    expect(() => noopConfigExportRecorder.recordExport(3)).not.toThrow();
  });
});

describe("createConfigExportRecorder", () => {
  it("emits the canonical config.exported event into the activity store", () => {
    const added: ActivityEventInput[] = [];
    const sink: Pick<ActivityStore, "add"> = {
      add: (input) => {
        added.push(input);
        return input as ActivityEvent;
      },
    };

    createConfigExportRecorder(sink).recordExport(2);

    expect(added).toHaveLength(1);
    expect(added[0]).toEqual({
      type: "config.exported",
      severity: "info",
      message: "Config exported: 2 rule(s).",
      details: { ruleCount: 2 },
    });
  });
});
