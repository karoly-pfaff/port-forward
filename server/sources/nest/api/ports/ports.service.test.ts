import { describe, expect, it } from "vitest";
import { PortsService } from "./ports.service.js";

describe("PortsService.resolveAdvisories", () => {
  const service = new PortsService();

  it("returns LAN exposure advisory for a 0.0.0.0 forward (listenHost present)", () => {
    const result = service.resolveAdvisories("48001", "forward", "0.0.0.0");

    expect(result).toEqual({
      ok: true,
      advisories: [
        {
          code: "LAN_EXPOSURE",
          severity: "warning",
          message:
            "Listening on 0.0.0.0 exposes this forwarded port on all interfaces. Other LAN devices may be able to connect if firewall settings allow it.",
        },
      ],
    });
  });

  it("resolves a management request with no listenHost (listenHost absent branch)", () => {
    const result = service.resolveAdvisories("47831", "management", undefined);
    expect(result.ok).toBe(true);
  });

  it("treats a non-string listenHost as absent", () => {
    const result = service.resolveAdvisories("48001", "forward", 123);
    expect(result.ok).toBe(true);
  });

  it.each(["0", "-1", "70000", "1.5", "abc", undefined])(
    "rejects the invalid port %j",
    (rawPort) => {
      expect(service.resolveAdvisories(rawPort, "forward", undefined)).toEqual({
        ok: false,
        errors: ["port must be an integer from 1 to 65535."],
      });
    }
  );

  it.each(["bogus", undefined, ""])("rejects the invalid purpose %j", (rawPurpose) => {
    expect(service.resolveAdvisories("48001", rawPurpose, undefined)).toEqual({
      ok: false,
      errors: ["purpose must be management or forward."],
    });
  });
});
