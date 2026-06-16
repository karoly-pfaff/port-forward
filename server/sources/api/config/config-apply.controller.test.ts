import type { ConfigApplyResponse } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ApiBadRequestException } from "../common/api-errors.js";
import { ConfigApplyController } from "./config-apply.controller.js";
import type { ConfigApplyService } from "./config-apply.service.js";

function response(): ConfigApplyResponse {
  return {
    ok: true,
    dryRun: false,
    appliedAt: "2026-06-15T08:30:00.000Z",
    plan: {
      generatedAt: "2026-06-15T08:30:00.000Z",
      mode: "plan",
      summary: { add: 0, update: 0, remove: 0, unchanged: 0, destructive: 0, hasDrift: false, hasErrors: false },
      operations: [],
      errors: [],
      warnings: [],
    },
    applied: { add: 0, update: 0, remove: 0, unchanged: 0 },
  };
}

function controller(impl: (body: unknown) => Promise<ConfigApplyResponse>): ConfigApplyController {
  return new ConfigApplyController({ apply: impl } as unknown as ConfigApplyService);
}

describe("ConfigApplyController.apply", () => {
  it("delegates to the service and maps the result through the response DTO (fresh copy)", async () => {
    const body = response();
    let received: unknown;
    const result = await controller(async (b) => {
      received = b;
      return body;
    }).apply({ desired: [] });

    expect(received).toEqual({ desired: [] });
    expect(result).toEqual(body);
    expect(result).not.toBe(body); // mapped (fresh) copy
  });

  it("propagates a thrown ApiBadRequestException (the 400 path)", async () => {
    await expect(
      controller(async () => {
        throw new ApiBadRequestException(["desired is required."]);
      }).apply({})
    ).rejects.toBeInstanceOf(ApiBadRequestException);
  });
});
