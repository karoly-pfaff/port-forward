import { getPortAdvisories } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { PortsAdvisoryQueryDto } from "./ports-advisory.query.dto.js";
import { PortsService } from "./ports.service.js";

function query(partial: Partial<PortsAdvisoryQueryDto>): PortsAdvisoryQueryDto {
  return Object.assign(new PortsAdvisoryQueryDto(), partial);
}

describe("PortsService.getAdvisories", () => {
  const service = new PortsService();

  it("computes advisories from the validated query (LAN exposure for 0.0.0.0 forward)", () => {
    expect(service.getAdvisories(query({ port: 48001, purpose: "forward", listenHost: "0.0.0.0" }))).toEqual(
      getPortAdvisories({ port: 48001, listenHost: "0.0.0.0", purpose: "forward" })
    );
  });

  it("passes through an absent listenHost", () => {
    expect(service.getAdvisories(query({ port: 47831, purpose: "management" }))).toEqual(
      getPortAdvisories({ port: 47831, listenHost: undefined, purpose: "management" })
    );
  });
});
