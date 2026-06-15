import type { ForwardRule, ForwardRuleResponse } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ForwardsController } from "./forwards.controller.js";
import type { CreateForwardRuleService } from "./create-forward-rule.service.js";
import type { ForwardsService } from "./forwards.service.js";

const RESPONSE: ForwardRuleResponse = {
  id: "r1",
  name: "Web",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48010,
  targetHost: "127.0.0.1",
  targetPort: 8080,
  enabled: false,
  advisories: [],
};

function controller(overrides: {
  list?: () => ForwardRuleResponse[];
  create?: (body: unknown) => Promise<ForwardRuleResponse>;
}): ForwardsController {
  return new ForwardsController(
    { list: overrides.list ?? (() => []) } as unknown as ForwardsService,
    { create: overrides.create ?? (async () => RESPONSE) } as unknown as CreateForwardRuleService
  );
}

describe("ForwardsController.list", () => {
  it("delegates to the service and maps the result to the response DTO", () => {
    const responses = [RESPONSE];
    const result = controller({ list: () => responses }).list();

    expect(result).toEqual(responses); // byte-for-byte
    expect(result).not.toBe(responses); // mapped copy
  });
});

describe("ForwardsController.create", () => {
  it("delegates to the create service and maps the result to the response DTO", async () => {
    const body: ForwardRule = { ...RESPONSE };
    let received: unknown;
    const result = await controller({
      create: async (input) => {
        received = input;
        return RESPONSE;
      },
    }).create(body);

    expect(received).toBe(body); // body passed through unchanged
    expect(result).toEqual(RESPONSE); // byte-for-byte
    expect(result).not.toBe(RESPONSE); // mapped copy
  });
});
