import type { ForwardRule, ForwardRuleResponse, GroupActionResponse, RuleDiagnosticsResult } from "@portier/shared";
import { describe, expect, it } from "vitest";
import type { ForwardStatus } from "@portier/shared";
import { ForwardsController } from "./forwards.controller.js";
import type { CreateForwardRuleService } from "./create-forward-rule.service.js";
import type { DeleteForwardRuleService } from "./delete-forward-rule.service.js";
import type { DiagnoseForwardRuleService } from "./diagnose-forward-rule.service.js";
import type { ReorderForwardRulesService } from "./reorder-forward-rules.service.js";
import type { StartForwardGroupService } from "./start-forward-group.service.js";
import type { StartForwardRuleService } from "./start-forward-rule.service.js";
import type { StopForwardGroupService } from "./stop-forward-group.service.js";
import type { StopForwardRuleService } from "./stop-forward-rule.service.js";
import type { UpdateForwardRuleService } from "./update-forward-rule.service.js";
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

const STATUS: ForwardStatus = {
  ruleId: "r1",
  running: true,
  health: "healthy",
  bytesIn: 0,
  bytesOut: 0,
  startedAt: "2026-06-15T00:00:00.000Z",
};

const DIAGNOSTICS: RuleDiagnosticsResult = {
  ruleId: "r1",
  ruleName: "Web",
  protocol: "tcp",
  summary: { status: "pass", message: "All checks passed." },
  checks: [{ id: "listen-host", label: "Listen address", status: "pass", message: "ok" }],
  diagnosedAt: "2026-06-15T00:00:00.000Z",
};

const GROUP_ACTION: GroupActionResponse = {
  group: "web",
  action: "stop",
  total: 1,
  succeeded: 0,
  skipped: 1,
  failed: 0,
  results: [{ ruleId: "r1", ruleName: "Web", status: "skipped", reason: "not_running" }],
};

function controller(overrides: {
  list?: () => ForwardRuleResponse[];
  create?: (body: unknown) => Promise<ForwardRuleResponse>;
  update?: (id: string, body: unknown) => Promise<ForwardRuleResponse>;
  remove?: (id: string) => Promise<void>;
  start?: (id: string) => Promise<ForwardStatus>;
  stop?: (id: string) => Promise<ForwardStatus>;
  reorder?: (ids: string[]) => Promise<ForwardRuleResponse[]>;
  diagnose?: (id: string) => Promise<RuleDiagnosticsResult>;
  stopGroup?: (group: string) => Promise<GroupActionResponse>;
  startGroup?: (group: string) => Promise<GroupActionResponse>;
}): ForwardsController {
  return new ForwardsController(
    { list: overrides.list ?? (() => []) } as unknown as ForwardsService,
    { create: overrides.create ?? (async () => RESPONSE) } as unknown as CreateForwardRuleService,
    { update: overrides.update ?? (async () => RESPONSE) } as unknown as UpdateForwardRuleService,
    { delete: overrides.remove ?? (async () => undefined) } as unknown as DeleteForwardRuleService,
    { start: overrides.start ?? (async () => STATUS) } as unknown as StartForwardRuleService,
    { stop: overrides.stop ?? (async () => STATUS) } as unknown as StopForwardRuleService,
    { reorder: overrides.reorder ?? (async () => [RESPONSE]) } as unknown as ReorderForwardRulesService,
    { diagnose: overrides.diagnose ?? (async () => DIAGNOSTICS) } as unknown as DiagnoseForwardRuleService,
    { stop: overrides.stopGroup ?? (async () => GROUP_ACTION) } as unknown as StopForwardGroupService,
    { start: overrides.startGroup ?? (async () => GROUP_ACTION) } as unknown as StartForwardGroupService
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

describe("ForwardsController.update", () => {
  it("delegates to the update service with the id + body and maps the result to the response DTO", async () => {
    const body = { name: "Renamed" };
    let receivedId: string | undefined;
    let receivedBody: unknown;
    const result = await controller({
      update: async (id, input) => {
        receivedId = id;
        receivedBody = input;
        return RESPONSE;
      },
    }).update("r1", body);

    expect(receivedId).toBe("r1"); // path id passed through
    expect(receivedBody).toBe(body); // body passed through unchanged
    expect(result).toEqual(RESPONSE); // byte-for-byte
    expect(result).not.toBe(RESPONSE); // mapped copy
  });
});

describe("ForwardsController.remove", () => {
  it("delegates to the delete service with the id and returns no body (204)", async () => {
    let receivedId: string | undefined;
    const result = await controller({
      remove: async (id) => {
        receivedId = id;
      },
    }).remove("r1");

    expect(receivedId).toBe("r1"); // path id passed through
    expect(result).toBeUndefined(); // no response body
  });
});

describe("ForwardsController.start", () => {
  it("delegates to the start service with the id and maps the status to the response DTO", async () => {
    let receivedId: string | undefined;
    const result = await controller({
      start: async (id) => {
        receivedId = id;
        return STATUS;
      },
    }).start("r1");

    expect(receivedId).toBe("r1"); // path id passed through
    expect(result).toEqual(STATUS); // byte-for-byte
    expect(result).not.toBe(STATUS); // mapped copy
  });
});

describe("ForwardsController.stop", () => {
  it("delegates to the stop service with the id and maps the status to the response DTO", async () => {
    let receivedId: string | undefined;
    const result = await controller({
      stop: async (id) => {
        receivedId = id;
        return STATUS;
      },
    }).stop("r1");

    expect(receivedId).toBe("r1"); // path id passed through
    expect(result).toEqual(STATUS); // byte-for-byte
    expect(result).not.toBe(STATUS); // mapped copy
  });
});

describe("ForwardsController.reorder", () => {
  it("delegates the ids to the reorder service and maps the list to the response DTO", async () => {
    const list = [RESPONSE, { ...RESPONSE, id: "r2" }];
    let receivedIds: string[] | undefined;
    const result = await controller({
      reorder: async (ids) => {
        receivedIds = ids;
        return list;
      },
    }).reorder({ ids: ["r2", "r1"] });

    expect(receivedIds).toEqual(["r2", "r1"]); // body.ids passed through
    expect(result).toEqual(list); // byte-for-byte
    expect(result).not.toBe(list); // mapped copy (fresh array)
  });
});

describe("ForwardsController.diagnose", () => {
  it("delegates the id to the diagnose service and maps the result to the response DTO", async () => {
    let receivedId: string | undefined;
    const result = await controller({
      diagnose: async (id) => {
        receivedId = id;
        return DIAGNOSTICS;
      },
    }).diagnose("r1");

    expect(receivedId).toBe("r1"); // path id passed through
    expect(result).toEqual(DIAGNOSTICS); // byte-for-byte
    expect(result).not.toBe(DIAGNOSTICS); // mapped copy (fresh object)
  });
});

describe("ForwardsController.stopGroup", () => {
  it("delegates the group to the stop-group service and maps the summary to the response DTO", async () => {
    let receivedGroup: string | undefined;
    const result = await controller({
      stopGroup: async (group) => {
        receivedGroup = group;
        return GROUP_ACTION;
      },
    }).stopGroup("web");

    expect(receivedGroup).toBe("web"); // path group passed through
    expect(result).toEqual(GROUP_ACTION); // byte-for-byte
    expect(result).not.toBe(GROUP_ACTION); // mapped copy (fresh object)
  });
});

describe("ForwardsController.startGroup", () => {
  it("delegates the group to the start-group service and maps the summary to the response DTO", async () => {
    let receivedGroup: string | undefined;
    const started: GroupActionResponse = { ...GROUP_ACTION, action: "start" };
    const result = await controller({
      startGroup: async (group) => {
        receivedGroup = group;
        return started;
      },
    }).startGroup("web");

    expect(receivedGroup).toBe("web"); // path group passed through
    expect(result).toEqual(started); // byte-for-byte
    expect(result).not.toBe(started); // mapped copy (fresh object)
  });
});
