import type { PortAdvisory } from "@portier/shared";
import { describe, expect, it, vi } from "vitest";
import { PortsController } from "./ports.controller.js";
import { PortsAdvisoryQueryDto } from "./ports-advisory.query.dto.js";
import type { PortsService } from "./ports.service.js";

describe("PortsController.getAdvisory", () => {
  it("delegates the validated query to the service and maps the result to the response DTO", () => {
    const advisories: PortAdvisory[] = [{ code: "LAN_EXPOSURE", severity: "warning", message: "x" }];
    const getAdvisories = vi.fn(() => advisories);
    const controller = new PortsController({ getAdvisories } as unknown as PortsService);

    const query = Object.assign(new PortsAdvisoryQueryDto(), { port: 48001, purpose: "forward" as const });
    const result = controller.getAdvisory(query);

    expect(getAdvisories).toHaveBeenCalledWith(query);
    expect(result).toEqual(advisories); // byte-for-byte
    expect(result).not.toBe(advisories); // mapped copy, not the raw service result
  });
});
