import { getPortAdvisories, type ForwardRule } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ForwardsService } from "./forwards.service.js";
import type { ForwardsReader } from "./forwards.reader.js";

function service(rules: ForwardRule[]): ForwardsService {
  const reader: ForwardsReader = { listRules: () => rules };
  return new ForwardsService(reader);
}

const lanRule: ForwardRule = {
  id: "r1",
  name: "Web",
  protocol: "tcp",
  listenHost: "0.0.0.0",
  listenPort: 48010,
  targetHost: "127.0.0.1",
  targetPort: 8080,
  enabled: false,
};

describe("ForwardsService.list", () => {
  it("decorates each rule with its forward-purpose advisories", () => {
    expect(service([lanRule]).list()).toEqual([
      {
        ...lanRule,
        advisories: getPortAdvisories({ port: 48010, listenHost: "0.0.0.0", purpose: "forward" }),
      },
    ]);
  });

  it("surfaces a LAN exposure advisory for a 0.0.0.0 listen host", () => {
    const [response] = service([lanRule]).list();
    expect(response.advisories.some((a) => a.code === "LAN_EXPOSURE")).toBe(true);
  });

  it("returns an empty list when there are no rules", () => {
    expect(service([]).list()).toEqual([]);
  });
});
