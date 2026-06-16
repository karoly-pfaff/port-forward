import type { ForwardRule } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { buildExportedConfig } from "./config-export.js";

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

describe("buildExportedConfig", () => {
  it("builds an empty export with version 1 and the clock's ISO timestamp", () => {
    expect(buildExportedConfig({ rules: [], now: NOW })).toEqual({
      version: "1",
      exportedAt: "2026-06-14T12:00:00.000Z",
      rules: [],
    });
  });

  it("includes the provided rules", () => {
    const config = buildExportedConfig({ rules: [RULE], now: NOW });
    expect(config).toEqual({
      version: "1",
      exportedAt: "2026-06-14T12:00:00.000Z",
      rules: [RULE],
    });
  });

  it("passes the rules array through without copying (historical pass-through behavior)", () => {
    const rules = [RULE];
    expect(buildExportedConfig({ rules, now: NOW }).rules).toBe(rules);
  });

  it("preserves the contract field order in JSON serialization", () => {
    const json = JSON.stringify(buildExportedConfig({ rules: [RULE], now: NOW }));
    expect(Object.keys(JSON.parse(json) as Record<string, unknown>)).toEqual([
      "version",
      "exportedAt",
      "rules",
    ]);
  });
});
