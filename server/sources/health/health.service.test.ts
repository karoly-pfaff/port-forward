import { describe, expect, it } from "vitest";
import { HealthService } from "./health.service.js";

describe("HealthService", () => {
  it("reports a healthy node server without any runtime manager", () => {
    const service = new HealthService();

    expect(service.getHealth()).toEqual({ ok: true, server: "node", name: "Portier" });
  });

  it("returns a fresh object each call (no shared mutable state)", () => {
    const service = new HealthService();

    const first = service.getHealth();
    const second = service.getHealth();

    expect(first).not.toBe(second);
    expect(first).toEqual(second);
  });
});
