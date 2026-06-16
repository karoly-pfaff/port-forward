import type { ForwardRule, ForwardStatus } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { getFreeUdpPort } from "../../testing/test-helpers.js";
import { ApiNotFoundException } from "../common/api-errors.js";
import type { ClockReader } from "../common/clock.reader.js";
import { DiagnoseForwardRuleService } from "./diagnose-forward-rule.service.js";
import type { DiagnosticReader } from "./forwards-diagnostics.reader.js";

const FIXED = new Date("2026-06-15T00:00:00.000Z");
const fixedClock: ClockReader = { now: () => FIXED };

function status(running: boolean): ForwardStatus {
  return { ruleId: "d1", running, health: running ? "healthy" : "warning", bytesIn: 0, bytesOut: 0 };
}

function service(reader: DiagnosticReader, clock: ClockReader = fixedClock): DiagnoseForwardRuleService {
  return new DiagnoseForwardRuleService(reader, clock);
}

describe("DiagnoseForwardRuleService.diagnose", () => {
  it("throws a 404 ApiNotFoundException for an unknown id (no diagnose probes run)", async () => {
    let getStatusCalled = false;
    const reader: DiagnosticReader = {
      getRule: () => undefined,
      getStatus: () => {
        getStatusCalled = true;
        return status(false);
      },
    };
    await expect(service(reader).diagnose("nope")).rejects.toBeInstanceOf(ApiNotFoundException);
    expect(getStatusCalled).toBe(false); // short-circuits before reading status
  });

  it("diagnoses a (stopped) UDP rule via the shared diagnoseRule with the pinned clock", async () => {
    // UDP → target-connect is always "skip" (no TCP connect probe); 127.0.0.1
    // target resolves instantly; a free UDP listen port binds+releases once.
    const rule: ForwardRule = {
      id: "d1",
      name: "Diag UDP",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: await getFreeUdpPort(),
      targetHost: "127.0.0.1",
      targetPort: await getFreeUdpPort(),
      enabled: false,
      udpMode: "one-way",
    };
    let receivedStatusId: string | undefined;
    const reader: DiagnosticReader = {
      getRule: (id) => (id === "d1" ? rule : undefined),
      getStatus: (id) => {
        receivedStatusId = id;
        return status(false);
      },
    };

    const result = await service(reader).diagnose("d1");

    expect(receivedStatusId).toBe("d1");
    expect(result.ruleId).toBe("d1");
    expect(result.protocol).toBe("udp");
    expect(result.diagnosedAt).toBe(FIXED.toISOString()); // clock pinned the volatile field
    expect(result.checks.find((c) => c.id === "target-connect")?.status).toBe("skip");
    expect(result.checks.find((c) => c.id === "udp-mode")?.status).toBe("pass");
  });
});
