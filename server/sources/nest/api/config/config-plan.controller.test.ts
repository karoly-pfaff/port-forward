import type { ConfigPlanResponse } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ConfigPlanController } from "./config-plan.controller.js";
import type { ConfigPlanService } from "./config-plan.service.js";

const PLAN: ConfigPlanResponse = {
  generatedAt: "2026-06-15T00:00:00.000Z",
  mode: "plan",
  summary: { add: 0, update: 0, remove: 0, unchanged: 0, destructive: 0, hasDrift: false, hasErrors: false },
  operations: [],
  errors: [],
  warnings: [],
};

function controller(plan: (body: unknown) => ConfigPlanResponse): ConfigPlanController {
  return new ConfigPlanController({ plan } as unknown as ConfigPlanService);
}

describe("ConfigPlanController.plan", () => {
  it("delegates the body to the service and maps the plan to the response DTO", () => {
    const body = { desired: { rules: [] } };
    let received: unknown;
    const result = controller((b) => {
      received = b;
      return PLAN;
    }).plan(body);

    expect(received).toBe(body); // raw body passed through unchanged
    expect(result).toEqual(PLAN); // byte-for-byte
    expect(result).not.toBe(PLAN); // mapped copy (fresh object)
  });
});
