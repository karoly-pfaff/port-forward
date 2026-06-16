import type { RuleDiagnosticsResult } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toRuleDiagnosticsResponseDto } from "./rule-diagnostics.response.dto.js";

const RESULT: RuleDiagnosticsResult = {
  ruleId: "d1",
  ruleName: "Diag",
  protocol: "tcp",
  summary: { status: "warn", message: "1 check(s) need attention: Listen address." },
  checks: [
    { id: "listen-host", label: "Listen address", status: "warn", message: "x", details: { listenHost: "0.0.0.0" } },
    { id: "target-connect", label: "Target connection", status: "skip", message: "y" }, // no details
  ],
  diagnosedAt: "2026-06-15T00:00:00.000Z",
};

describe("toRuleDiagnosticsResponseDto", () => {
  it("preserves the diagnose result shape byte-for-byte without mutating the source", () => {
    const snapshot = structuredClone(RESULT);

    const dto = toRuleDiagnosticsResponseDto(RESULT);

    expect(dto).toEqual(RESULT);
    expect(RESULT).toEqual(snapshot); // source untouched
  });

  it("returns fresh nested objects (summary, checks, details) — not the source references", () => {
    const dto = toRuleDiagnosticsResponseDto(RESULT);

    expect(dto).not.toBe(RESULT);
    expect(dto.summary).not.toBe(RESULT.summary);
    expect(dto.checks).not.toBe(RESULT.checks);
    expect(dto.checks[0]).not.toBe(RESULT.checks[0]);
    expect(dto.checks[0].details).not.toBe(RESULT.checks[0].details);
    // The check without details stays without a details key.
    expect(dto.checks[1]).not.toHaveProperty("details");
  });
});
