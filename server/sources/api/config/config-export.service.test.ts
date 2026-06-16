import type { ForwardRule } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ConfigExportService } from "./config-export.service.js";
import type { ConfigExportReader } from "./config-export.reader.js";
import {
  noopConfigExportRecorder,
  type ConfigExportRecorder,
} from "./config-export.recorder.js";
import type { ClockReader } from "../common/clock.reader.js";

const NOW = new Date("2026-06-14T12:00:00.000Z");

const RULE: ForwardRule = {
  id: "r1",
  name: "Web",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48010,
  targetHost: "127.0.0.1",
  targetPort: 8080,
  enabled: false,
};

function service(
  rules: ForwardRule[],
  recorder: ConfigExportRecorder = noopConfigExportRecorder,
  now: Date = NOW
): ConfigExportService {
  const reader: ConfigExportReader = { listRules: () => rules };
  const clock: ClockReader = { now: () => now };
  return new ConfigExportService(reader, clock, recorder);
}

describe("ConfigExportService.export", () => {
  it("snapshots the reader's rules and stamps exportedAt from the clock", () => {
    expect(service([RULE]).export()).toEqual({
      version: "1",
      exportedAt: "2026-06-14T12:00:00.000Z",
      rules: [RULE],
    });
  });

  it("exports an empty config when there are no rules", () => {
    expect(service([]).export()).toEqual({
      version: "1",
      exportedAt: "2026-06-14T12:00:00.000Z",
      rules: [],
    });
  });

  it("records exactly one export with the rule count, without altering the response", () => {
    const counts: number[] = [];
    const recorder: ConfigExportRecorder = { recordExport: (n) => counts.push(n) };

    const config = service([RULE], recorder).export();

    expect(counts).toEqual([1]); // exactly one call, ruleCount 1
    // The recorder side-effect must not change the response body.
    expect(config).toEqual({
      version: "1",
      exportedAt: "2026-06-14T12:00:00.000Z",
      rules: [RULE],
    });
  });
});
