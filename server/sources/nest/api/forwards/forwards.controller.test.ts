import type { ForwardRuleResponse } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ForwardsController } from "./forwards.controller.js";
import type { ForwardsService } from "./forwards.service.js";

describe("ForwardsController.list", () => {
  it("delegates to the service and returns its rule responses", () => {
    const responses: ForwardRuleResponse[] = [
      {
        id: "r1",
        name: "Web",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort: 48010,
        targetHost: "127.0.0.1",
        targetPort: 8080,
        enabled: false,
        advisories: [],
      },
    ];
    const controller = new ForwardsController({ list: () => responses } as unknown as ForwardsService);

    expect(controller.list()).toBe(responses);
  });
});
