import { describe, expect, it } from "vitest";
import { HealthController } from "./health.controller.js";
import { HealthService } from "./health.service.js";

describe("HealthController", () => {
  it("delegates to the health service", () => {
    const service = new HealthService();
    const controller = new HealthController(service);

    expect(controller.getHealth()).toEqual({ ok: true, server: "node", name: "Portier" });
  });

  it("returns exactly what the service produces", () => {
    const sentinel = { ok: true, server: "node", name: "Portier" } as const;
    const stub = { getHealth: () => sentinel } as HealthService;
    const controller = new HealthController(stub);

    expect(controller.getHealth()).toBe(sentinel);
  });
});
