import type { ForwardRuleResponse } from "@portier/shared";
import type { Response } from "express";
import { describe, expect, it } from "vitest";
import { ApiBadRequestException } from "../../common/api-errors.js";
import { ConfigImportController } from "./config-import.controller.js";
import type { ConfigImportOutcome, ConfigImportService } from "./config-import.service.js";

const RULE: ForwardRuleResponse = {
  id: "r1", name: "Web", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48010,
  targetHost: "127.0.0.1", targetPort: 8080, enabled: false, advisories: [],
};

/** A minimal `res` that records the status set via `res.status(...)`. */
function fakeRes(): Response & { statusCode: number } {
  const res = {
    statusCode: 0,
    status(code: number) {
      res.statusCode = code;
      return res;
    },
  };
  return res as unknown as Response & { statusCode: number };
}

function controller(impl: (body: unknown) => Promise<ConfigImportOutcome>): ConfigImportController {
  return new ConfigImportController({ import: impl } as unknown as ConfigImportService);
}

describe("ConfigImportController.import", () => {
  it("sets status 200 and maps the success body for a 200 outcome", async () => {
    const res = fakeRes();
    let received: unknown;
    const body = { result: { imported: 1, skipped: 0, errors: [] }, rules: [RULE] };
    const result = await controller(async (b) => {
      received = b;
      return { status: 200, body };
    }).import({ mode: "replace", config: { version: "1", rules: [] } }, res);

    expect(received).toEqual({ mode: "replace", config: { version: "1", rules: [] } });
    expect(res.statusCode).toBe(200);
    expect(result).toEqual(body);
    expect(result).not.toBe(body); // mapped (fresh) copy
  });

  it("sets status 422 and maps the error body for a 422 outcome", async () => {
    const res = fakeRes();
    const body = { errors: ["bad"], result: { imported: 0, skipped: 0, errors: ["bad"] } };
    const result = await controller(async () => ({ status: 422, body })).import({}, res);

    expect(res.statusCode).toBe(422);
    expect(result).toEqual(body);
    expect(result).not.toBe(body);
  });

  it("propagates a thrown ApiBadRequestException (the 400 path) without setting a status", async () => {
    const res = fakeRes();
    await expect(
      controller(async () => {
        throw new ApiBadRequestException(["mode must be replace or merge."]);
      }).import({}, res)
    ).rejects.toBeInstanceOf(ApiBadRequestException);
    expect(res.statusCode).toBe(0); // never reached res.status
  });
});
