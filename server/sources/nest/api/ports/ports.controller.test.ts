import { describe, expect, it } from "vitest";
import { ApiBadRequestException } from "../../common/api-errors.js";
import { PortsController } from "./ports.controller.js";
import type { AdvisoryResult, PortsService } from "./ports.service.js";

function controllerWith(result: AdvisoryResult): PortsController {
  const stub = { resolveAdvisories: () => result } as unknown as PortsService;
  return new PortsController(stub);
}

describe("PortsController.getAdvisory", () => {
  it("returns the advisories when the service resolves them", () => {
    const advisories = [{ code: "LAN_EXPOSURE", severity: "warning", message: "x" }] as const;
    const controller = controllerWith({ ok: true, advisories: [...advisories] });

    expect(controller.getAdvisory("48001", "forward", "0.0.0.0")).toEqual([...advisories]);
  });

  it("throws an ApiBadRequestException carrying the service errors (shared layer owns the envelope)", () => {
    const controller = controllerWith({ ok: false, errors: ["purpose must be management or forward."] });

    try {
      controller.getAdvisory("48001", "bogus");
      expect.unreachable("expected an ApiBadRequestException");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiBadRequestException);
      expect((error as ApiBadRequestException).getResponse()).toEqual({
        errors: ["purpose must be management or forward."],
      });
    }
  });
});
