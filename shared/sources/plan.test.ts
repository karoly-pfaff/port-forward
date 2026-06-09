import { describe, expect, it } from "vitest";
import type {
  ConfigAppliedCounts,
  ConfigApplyRequest,
  ConfigApplyResponse,
  ConfigPlanChange,
  ConfigPlanError,
  ConfigPlanOperation,
  ConfigPlanOperationType,
  ConfigPlanRequest,
  ConfigPlanResponse,
  ConfigPlanRuleSnapshot,
  ConfigPlanSummary,
  ConfigPlanWarning,
  DesiredConfig
} from "./plan.js";

describe("ConfigPlanOperationType", () => {
  it("accepts all four operation types", () => {
    const add: ConfigPlanOperationType = "add";
    const update: ConfigPlanOperationType = "update";
    const remove: ConfigPlanOperationType = "remove";
    const unchanged: ConfigPlanOperationType = "unchanged";
    expect(add).toBe("add");
    expect(update).toBe("update");
    expect(remove).toBe("remove");
    expect(unchanged).toBe("unchanged");
  });
});

describe("ConfigPlanChange shape", () => {
  it("accepts a field change with before and after values", () => {
    const change: ConfigPlanChange = {
      field: "listenPort",
      before: 48000,
      after: 48001
    };
    expect(change.field).toBe("listenPort");
    expect(change.before).toBe(48000);
    expect(change.after).toBe(48001);
  });

  it("accepts string field changes", () => {
    const change: ConfigPlanChange = {
      field: "name",
      before: "Old name",
      after: "New name"
    };
    expect(change.before).toBe("Old name");
    expect(change.after).toBe("New name");
  });

  it("accepts boolean field changes", () => {
    const change: ConfigPlanChange = {
      field: "enabled",
      before: false,
      after: true
    };
    expect(change.before).toBe(false);
    expect(change.after).toBe(true);
  });
});

describe("ConfigPlanRuleSnapshot shape", () => {
  it("accepts a minimal TCP rule snapshot", () => {
    const snapshot: ConfigPlanRuleSnapshot = {
      name: "Postgres forward",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48001,
      targetHost: "127.0.0.1",
      targetPort: 5432,
      enabled: true
    };
    expect(snapshot.protocol).toBe("tcp");
    expect(snapshot.listenPort).toBe(48001);
    expect(snapshot.enabled).toBe(true);
    expect(snapshot.id).toBeUndefined();
    expect(snapshot.udpMode).toBeUndefined();
  });

  it("accepts a UDP rule snapshot with all fields", () => {
    const snapshot: ConfigPlanRuleSnapshot = {
      id: "rule-dns-001",
      name: "DNS forward",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: 48053,
      targetHost: "1.1.1.1",
      targetPort: 53,
      enabled: true,
      udpMode: "one-way"
    };
    expect(snapshot.protocol).toBe("udp");
    expect(snapshot.udpMode).toBe("one-way");
    expect(snapshot.id).toBe("rule-dns-001");
  });

  it("accepts all three UDP modes", () => {
    const oneWay: ConfigPlanRuleSnapshot = {
      name: "One-way",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: 48001,
      targetHost: "10.0.0.1",
      targetPort: 53,
      enabled: true,
      udpMode: "one-way"
    };
    const lastClient: ConfigPlanRuleSnapshot = {
      name: "Last client",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: 48002,
      targetHost: "10.0.0.1",
      targetPort: 53,
      enabled: true,
      udpMode: "bidirectional-last-client"
    };
    const multiClient: ConfigPlanRuleSnapshot = {
      name: "Multi client",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: 48003,
      targetHost: "10.0.0.1",
      targetPort: 53,
      enabled: true,
      udpMode: "bidirectional-multi-client"
    };
    expect(oneWay.udpMode).toBe("one-way");
    expect(lastClient.udpMode).toBe("bidirectional-last-client");
    expect(multiClient.udpMode).toBe("bidirectional-multi-client");
  });
});

describe("ConfigPlanOperation shape", () => {
  it("accepts an add operation for a new rule", () => {
    const desired: ConfigPlanRuleSnapshot = {
      name: "New rule",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48010,
      targetHost: "127.0.0.1",
      targetPort: 3000,
      enabled: true
    };
    const op: ConfigPlanOperation = {
      type: "add",
      ruleName: "New rule",
      protocol: "tcp",
      desired,
      destructive: false
    };
    expect(op.type).toBe("add");
    expect(op.destructive).toBe(false);
    expect(op.current).toBeUndefined();
    expect(op.ruleId).toBeUndefined();
  });

  it("accepts a remove operation as destructive", () => {
    const current: ConfigPlanRuleSnapshot = {
      id: "rule-old-001",
      name: "Old rule",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48020,
      targetHost: "127.0.0.1",
      targetPort: 8080,
      enabled: true
    };
    const op: ConfigPlanOperation = {
      type: "remove",
      ruleId: "rule-old-001",
      ruleName: "Old rule",
      protocol: "tcp",
      current,
      destructive: true
    };
    expect(op.type).toBe("remove");
    expect(op.destructive).toBe(true);
    expect(op.desired).toBeUndefined();
  });

  it("accepts an update operation with changes", () => {
    const current: ConfigPlanRuleSnapshot = {
      id: "rule-db-001",
      name: "DB forward",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48030,
      targetHost: "127.0.0.1",
      targetPort: 5432,
      enabled: true
    };
    const desired: ConfigPlanRuleSnapshot = {
      id: "rule-db-001",
      name: "DB forward",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48031,
      targetHost: "127.0.0.1",
      targetPort: 5432,
      enabled: true
    };
    const op: ConfigPlanOperation = {
      type: "update",
      ruleId: "rule-db-001",
      ruleName: "DB forward",
      protocol: "tcp",
      current,
      desired,
      changes: [{ field: "listenPort", before: 48030, after: 48031 }],
      destructive: true
    };
    expect(op.type).toBe("update");
    expect(op.changes).toHaveLength(1);
    expect(op.changes![0].field).toBe("listenPort");
    expect(op.destructive).toBe(true);
  });

  it("accepts an unchanged operation", () => {
    const snapshot: ConfigPlanRuleSnapshot = {
      id: "rule-ok-001",
      name: "Unchanged rule",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48040,
      targetHost: "10.0.0.1",
      targetPort: 22,
      enabled: false
    };
    const op: ConfigPlanOperation = {
      type: "unchanged",
      ruleId: "rule-ok-001",
      ruleName: "Unchanged rule",
      protocol: "tcp",
      current: snapshot,
      desired: snapshot,
      changes: [],
      destructive: false
    };
    expect(op.type).toBe("unchanged");
    expect(op.changes).toHaveLength(0);
    expect(op.destructive).toBe(false);
  });
});

describe("ConfigPlanSummary shape", () => {
  it("accepts a summary with no drift", () => {
    const summary: ConfigPlanSummary = {
      add: 0,
      update: 0,
      remove: 0,
      unchanged: 3,
      destructive: 0,
      hasDrift: false,
      hasErrors: false
    };
    expect(summary.unchanged).toBe(3);
    expect(summary.hasDrift).toBe(false);
    expect(summary.hasErrors).toBe(false);
  });

  it("accepts a summary with drift", () => {
    const summary: ConfigPlanSummary = {
      add: 2,
      update: 1,
      remove: 1,
      unchanged: 1,
      destructive: 2,
      hasDrift: true,
      hasErrors: false
    };
    expect(summary.add).toBe(2);
    expect(summary.update).toBe(1);
    expect(summary.remove).toBe(1);
    expect(summary.destructive).toBe(2);
    expect(summary.hasDrift).toBe(true);
  });

  it("accepts a summary with errors", () => {
    const summary: ConfigPlanSummary = {
      add: 0,
      update: 0,
      remove: 0,
      unchanged: 0,
      destructive: 0,
      hasDrift: false,
      hasErrors: true
    };
    expect(summary.hasErrors).toBe(true);
  });
});

describe("ConfigPlanError shape", () => {
  it("accepts an error with required fields", () => {
    const err: ConfigPlanError = {
      code: "INVALID_CONFIG",
      message: "The desired config is not a valid Portier v1 config."
    };
    expect(err.code).toBe("INVALID_CONFIG");
    expect(err.message).toBeDefined();
    expect(err.field).toBeUndefined();
  });

  it("accepts an error with an optional field reference", () => {
    const err: ConfigPlanError = {
      code: "DUPLICATE_BINDING",
      message: "Two desired rules share the same protocol:listenHost:listenPort.",
      field: "listenPort"
    };
    expect(err.field).toBe("listenPort");
  });
});

describe("ConfigPlanWarning shape", () => {
  it("accepts a warning with code and message", () => {
    const warn: ConfigPlanWarning = {
      code: "LAN_EXPOSURE",
      message: "One or more desired rules listen on 0.0.0.0."
    };
    expect(warn.code).toBe("LAN_EXPOSURE");
    expect(warn.message).toBeDefined();
  });
});

describe("ConfigPlanResponse shape", () => {
  it("accepts a minimal empty plan response", () => {
    const response: ConfigPlanResponse = {
      generatedAt: "2026-06-09T12:00:00.000Z",
      mode: "plan",
      summary: {
        add: 0,
        update: 0,
        remove: 0,
        unchanged: 0,
        destructive: 0,
        hasDrift: false,
        hasErrors: false
      },
      operations: [],
      errors: [],
      warnings: []
    };
    expect(response.mode).toBe("plan");
    expect(response.operations).toHaveLength(0);
    expect(response.errors).toHaveLength(0);
  });

  it("accepts a fully populated plan response", () => {
    const response: ConfigPlanResponse = {
      generatedAt: "2026-06-09T12:00:00.000Z",
      mode: "plan",
      summary: {
        add: 1,
        update: 1,
        remove: 1,
        unchanged: 1,
        destructive: 2,
        hasDrift: true,
        hasErrors: false
      },
      operations: [
        {
          type: "add",
          ruleName: "New rule",
          protocol: "tcp",
          desired: {
            name: "New rule",
            protocol: "tcp",
            listenHost: "127.0.0.1",
            listenPort: 48050,
            targetHost: "10.0.0.1",
            targetPort: 8080,
            enabled: true
          },
          destructive: false
        },
        {
          type: "remove",
          ruleId: "rule-old-001",
          ruleName: "Old rule",
          protocol: "udp",
          current: {
            id: "rule-old-001",
            name: "Old rule",
            protocol: "udp",
            listenHost: "127.0.0.1",
            listenPort: 48060,
            targetHost: "10.0.0.1",
            targetPort: 53,
            enabled: true,
            udpMode: "one-way"
          },
          destructive: true
        }
      ],
      errors: [],
      warnings: [
        { code: "REMOVE_RUNNING_RULE", message: "Applying this plan would remove a running rule." }
      ]
    };
    expect(response.summary.hasDrift).toBe(true);
    expect(response.operations).toHaveLength(2);
    expect(response.operations[0].type).toBe("add");
    expect(response.operations[1].type).toBe("remove");
    expect(response.operations[1].destructive).toBe(true);
    expect(response.warnings).toHaveLength(1);
  });
});

describe("DesiredConfig shape", () => {
  it("accepts an empty desired config", () => {
    const config: DesiredConfig = { rules: [] };
    expect(config.rules).toHaveLength(0);
  });

  it("accepts a desired config with rules", () => {
    const config: DesiredConfig = {
      rules: [
        {
          name: "DB tunnel",
          protocol: "tcp",
          listenHost: "127.0.0.1",
          listenPort: 48001,
          targetHost: "127.0.0.1",
          targetPort: 5432,
          enabled: true
        }
      ]
    };
    expect(config.rules).toHaveLength(1);
    expect(config.rules[0].name).toBe("DB tunnel");
  });
});

describe("ConfigPlanRequest shape", () => {
  it("wraps a desired config", () => {
    const req: ConfigPlanRequest = {
      desired: { rules: [] }
    };
    expect(req.desired.rules).toHaveLength(0);
  });
});

describe("ConfigAppliedCounts shape", () => {
  it("accepts applied counts with all four fields", () => {
    const counts: ConfigAppliedCounts = { add: 1, update: 2, remove: 1, unchanged: 5 };
    expect(counts.add).toBe(1);
    expect(counts.update).toBe(2);
    expect(counts.remove).toBe(1);
    expect(counts.unchanged).toBe(5);
  });

  it("accepts zero counts for a no-drift apply", () => {
    const counts: ConfigAppliedCounts = { add: 0, update: 0, remove: 0, unchanged: 3 };
    expect(counts.add).toBe(0);
    expect(counts.unchanged).toBe(3);
  });
});

describe("ConfigApplyRequest shape", () => {
  it("requires desired and yes flag", () => {
    const req: ConfigApplyRequest = {
      desired: { rules: [] },
      yes: true
    };
    expect(req.yes).toBe(true);
    expect(req.dryRun).toBeUndefined();
  });

  it("accepts optional dryRun flag", () => {
    const req: ConfigApplyRequest = {
      desired: { rules: [] },
      yes: false,
      dryRun: true
    };
    expect(req.dryRun).toBe(true);
  });
});

describe("ConfigApplyResponse shape", () => {
  const minimalPlan: ConfigPlanResponse = {
    generatedAt: "2026-06-09T12:00:00.000Z",
    mode: "plan",
    summary: { add: 0, update: 0, remove: 0, unchanged: 0, destructive: 0, hasDrift: false, hasErrors: false },
    operations: [],
    errors: [],
    warnings: []
  };

  it("accepts a successful apply response", () => {
    const res: ConfigApplyResponse = {
      ok: true,
      dryRun: false,
      appliedAt: "2026-06-09T12:00:00.000Z",
      plan: minimalPlan,
      applied: { add: 1, update: 0, remove: 0, unchanged: 2 }
    };
    expect(res.ok).toBe(true);
    expect(res.dryRun).toBe(false);
    expect(res.applied.add).toBe(1);
    expect(res.applied.unchanged).toBe(2);
  });

  it("accepts a dry-run response with ok:true", () => {
    const res: ConfigApplyResponse = {
      ok: true,
      dryRun: true,
      appliedAt: "2026-06-09T12:00:00.000Z",
      plan: minimalPlan,
      applied: { add: 0, update: 0, remove: 0, unchanged: 0 }
    };
    expect(res.dryRun).toBe(true);
    expect(res.ok).toBe(true);
  });

  it("accepts a plan-error response with ok:false", () => {
    const errorPlan: ConfigPlanResponse = {
      ...minimalPlan,
      summary: { ...minimalPlan.summary, hasErrors: true },
      errors: [{ code: "INVALID_DESIRED_RULE", message: "name is required." }]
    };
    const res: ConfigApplyResponse = {
      ok: false,
      dryRun: false,
      appliedAt: "2026-06-09T12:00:00.000Z",
      plan: errorPlan,
      applied: { add: 0, update: 0, remove: 0, unchanged: 0 }
    };
    expect(res.ok).toBe(false);
    expect(res.plan.summary.hasErrors).toBe(true);
  });
});
