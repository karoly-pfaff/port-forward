import { describe, expect, it } from "vitest";
import type { ForwardRule } from "@portier/shared";
import { buildApplyImportFromPlan, buildConfigPlan, extractRulesArray } from "./config-plan.js";

// ── Fixtures ──────────────────────────────────────────────────────────────────

const NOW = new Date("2026-01-01T00:00:00.000Z");

const tcpRule: ForwardRule = {
  id: "r1",
  name: "TCP Rule",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 9001,
  targetHost: "127.0.0.1",
  targetPort: 9002,
  enabled: true,
};

const udpRule: ForwardRule = {
  id: "r2",
  name: "UDP Rule",
  protocol: "udp",
  listenHost: "127.0.0.1",
  listenPort: 9003,
  targetHost: "127.0.0.1",
  targetPort: 9004,
  enabled: false,
  udpMode: "one-way",
};

function makeTcpDesired(overrides: Partial<typeof tcpRule> = {}) {
  const { id: _id, ...rest } = tcpRule;
  return { ...rest, ...overrides };
}

// ── extractRulesArray ─────────────────────────────────────────────────────────

describe("extractRulesArray", () => {
  it("returns an array directly", () => {
    const arr = [{ name: "A" }];
    expect(extractRulesArray(arr)).toBe(arr);
  });

  it("returns rules from a wrapper object", () => {
    const rules = [{ name: "A" }];
    expect(extractRulesArray({ rules })).toBe(rules);
  });

  it("returns rules from an exported config shape", () => {
    const rules = [{ name: "A" }];
    expect(extractRulesArray({ version: "1", exportedAt: "2026-01-01T00:00:00.000Z", rules })).toBe(rules);
  });

  it("returns null for a plain string", () => {
    expect(extractRulesArray("not a config")).toBeNull();
  });

  it("returns null for a number", () => {
    expect(extractRulesArray(42)).toBeNull();
  });

  it("returns null for null", () => {
    expect(extractRulesArray(null)).toBeNull();
  });

  it("returns null for object without a rules array", () => {
    expect(extractRulesArray({ rules: "not-array" })).toBeNull();
  });

  it("returns null for an object with no rules field", () => {
    expect(extractRulesArray({ name: "foo" })).toBeNull();
  });
});

// ── buildConfigPlan — shape and generatedAt ────────────────────────────────────

describe("buildConfigPlan — response shape", () => {
  it("injects generatedAt from the now parameter", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [], now: NOW });
    expect(plan.generatedAt).toBe("2026-01-01T00:00:00.000Z");
  });

  it("sets mode to plan", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [], now: NOW });
    expect(plan.mode).toBe("plan");
  });

  it("always includes all summary fields", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [], now: NOW });
    expect(typeof plan.summary.add).toBe("number");
    expect(typeof plan.summary.update).toBe("number");
    expect(typeof plan.summary.remove).toBe("number");
    expect(typeof plan.summary.unchanged).toBe("number");
    expect(typeof plan.summary.destructive).toBe("number");
    expect(typeof plan.summary.hasDrift).toBe("boolean");
    expect(typeof plan.summary.hasErrors).toBe("boolean");
  });

  it("always returns operations, errors, warnings arrays", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [], now: NOW });
    expect(Array.isArray(plan.operations)).toBe(true);
    expect(Array.isArray(plan.errors)).toBe(true);
    expect(Array.isArray(plan.warnings)).toBe(true);
  });
});

// ── buildConfigPlan — empty / no drift ────────────────────────────────────────

describe("buildConfigPlan — empty current + empty desired", () => {
  it("produces no operations, no drift, no errors", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [], now: NOW });
    expect(plan.operations).toHaveLength(0);
    expect(plan.summary.hasDrift).toBe(false);
    expect(plan.summary.hasErrors).toBe(false);
  });
});

describe("buildConfigPlan — unchanged (no drift)", () => {
  it("produces unchanged operation when desired matches current exactly", () => {
    const desired = makeTcpDesired();
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    expect(plan.summary.unchanged).toBe(1);
    expect(plan.summary.hasDrift).toBe(false);
    expect(plan.operations[0].type).toBe("unchanged");
    expect(plan.operations[0].ruleId).toBe("r1");
  });

  it("includes current and desired snapshots in unchanged op", () => {
    const desired = makeTcpDesired();
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    const op = plan.operations[0];
    expect(op.current).toBeDefined();
    expect(op.desired).toBeDefined();
    expect(op.changes).toBeUndefined();
    expect(op.destructive).toBe(false);
  });
});

// ── buildConfigPlan — add operation ───────────────────────────────────────────

describe("buildConfigPlan — add operation", () => {
  it("produces add when desired has a rule not in current", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [makeTcpDesired()], now: NOW });
    expect(plan.summary.add).toBe(1);
    expect(plan.summary.hasDrift).toBe(true);
    expect(plan.operations[0].type).toBe("add");
  });

  it("add operation has desired snapshot and no current or changes", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [makeTcpDesired()], now: NOW });
    const op = plan.operations[0];
    expect(op.desired).toBeDefined();
    expect(op.current).toBeUndefined();
    expect(op.changes).toBeUndefined();
    expect(op.destructive).toBe(false);
    expect(op.ruleName).toBe("TCP Rule");
    expect(op.protocol).toBe("tcp");
  });

  it("desired rule with explicit id not in current is an add", () => {
    const desired = { ...makeTcpDesired(), id: "new-id" };
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    const addOp = plan.operations.find((o) => o.type === "add");
    expect(addOp).toBeDefined();
  });
});

// ── buildConfigPlan — remove operation ────────────────────────────────────────

describe("buildConfigPlan — remove operation", () => {
  it("produces remove when current has a rule not in desired", () => {
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [], now: NOW });
    expect(plan.summary.remove).toBe(1);
    expect(plan.summary.hasDrift).toBe(true);
    expect(plan.operations[0].type).toBe("remove");
  });

  it("remove operation is always destructive", () => {
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [], now: NOW });
    expect(plan.operations[0].destructive).toBe(true);
  });

  it("remove operation has current snapshot and no desired", () => {
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [], now: NOW });
    const op = plan.operations[0];
    expect(op.current).toBeDefined();
    expect(op.desired).toBeUndefined();
    expect(op.ruleId).toBe("r1");
  });

  it("remove operation emits REMOVE_EXISTING warning", () => {
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [], now: NOW });
    const warning = plan.warnings.find((w) => w.code === "REMOVE_EXISTING");
    expect(warning).toBeDefined();
    expect(warning?.message).toContain("TCP Rule");
    expect(warning?.message).toContain("r1");
  });
});

// ── buildConfigPlan — update operation ────────────────────────────────────────

describe("buildConfigPlan — update (non-destructive: name change)", () => {
  it("produces update when name differs", () => {
    const desired = makeTcpDesired({ name: "Renamed Rule" });
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    expect(plan.summary.update).toBe(1);
    expect(plan.summary.hasDrift).toBe(true);
    const op = plan.operations[0];
    expect(op.type).toBe("update");
    expect(op.destructive).toBe(false);
  });

  it("name change produces a ConfigPlanChange entry", () => {
    const desired = makeTcpDesired({ name: "Renamed Rule" });
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    const op = plan.operations[0];
    const nameChange = op.changes?.find((c) => c.field === "name");
    expect(nameChange).toBeDefined();
    expect(nameChange?.before).toBe("TCP Rule");
    expect(nameChange?.after).toBe("Renamed Rule");
  });

  it("enabled toggle is non-destructive", () => {
    const desired = makeTcpDesired({ enabled: false });
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    const op = plan.operations[0];
    expect(op.type).toBe("update");
    expect(op.destructive).toBe(false);
  });
});

describe("buildConfigPlan — update (destructive: forwarding field change)", () => {
  it("targetHost change is destructive", () => {
    const desired = makeTcpDesired({ targetHost: "192.168.1.1" });
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    const op = plan.operations[0];
    expect(op.type).toBe("update");
    expect(op.destructive).toBe(true);
  });

  it("targetPort change is destructive", () => {
    const desired = makeTcpDesired({ targetPort: 9999 });
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    expect(plan.operations[0].destructive).toBe(true);
  });

  it("listenPort change is destructive", () => {
    const desired = { ...makeTcpDesired({ listenPort: 9010 }), id: "r1" };
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    expect(plan.operations[0].destructive).toBe(true);
  });

  it("listenHost change is destructive", () => {
    const desired = { ...makeTcpDesired({ listenHost: "0.0.0.0" }), id: "r1" };
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    expect(plan.operations[0].destructive).toBe(true);
  });

  it("udpMode change is destructive", () => {
    const udpDesired = { id: undefined, name: "UDP Rule", protocol: "udp" as const, listenHost: "127.0.0.1", listenPort: 9003, targetHost: "127.0.0.1", targetPort: 9004, enabled: false, udpMode: "bidirectional-last-client" as const };
    const plan = buildConfigPlan({ currentRules: [udpRule], desiredRaw: [udpDesired], now: NOW });
    const op = plan.operations[0];
    expect(op.type).toBe("update");
    expect(op.destructive).toBe(true);
    const udpChange = op.changes?.find((c) => c.field === "udpMode");
    expect(udpChange?.before).toBe("one-way");
    expect(udpChange?.after).toBe("bidirectional-last-client");
  });

  it("update has both current and desired snapshots", () => {
    const desired = makeTcpDesired({ targetPort: 9999 });
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    const op = plan.operations[0];
    expect(op.current?.targetPort).toBe(9002);
    expect(op.desired?.targetPort).toBe(9999);
  });
});

// ── buildConfigPlan — matching semantics ──────────────────────────────────────

describe("buildConfigPlan — match by id", () => {
  it("matches by id when desired has explicit id", () => {
    const desired = { ...makeTcpDesired(), id: "r1" };
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    expect(plan.summary.unchanged).toBe(1);
    expect(plan.operations[0].ruleId).toBe("r1");
  });

  it("id match wins over identity key for an update", () => {
    // r1 has protocol:tcp listenHost:127.0.0.1 listenPort:9001
    // desired has id:r1 but different listen host — id match wins, produces update
    const desired = { ...makeTcpDesired(), id: "r1", listenHost: "0.0.0.0" };
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    expect(plan.summary.update).toBe(1);
    expect(plan.operations[0].ruleId).toBe("r1");
  });
});

describe("buildConfigPlan — match by identity key", () => {
  it("matches by protocol+listenHost+listenPort when desired has no id", () => {
    const desired = makeTcpDesired(); // no id, same protocol+listenHost+listenPort as r1
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    expect(plan.summary.unchanged).toBe(1);
    expect(plan.operations[0].ruleId).toBe("r1");
  });

  it("name match alone does NOT match rules (no fuzzy name matching)", () => {
    // different listenPort → different identity key → no match by name
    const desired = makeTcpDesired({ listenPort: 9099 });
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    expect(plan.summary.add).toBe(1);   // desired is an add
    expect(plan.summary.remove).toBe(1); // current r1 is a remove
  });
});

// ── buildConfigPlan — error cases ─────────────────────────────────────────────

describe("buildConfigPlan — INVALID_DESIRED_CONFIG", () => {
  it("returns error for a non-array non-object desired", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: "invalid", now: NOW });
    expect(plan.summary.hasErrors).toBe(true);
    const err = plan.errors.find((e) => e.code === "INVALID_DESIRED_CONFIG");
    expect(err).toBeDefined();
  });

  it("returns error for object with no rules field", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: { name: "bad" }, now: NOW });
    expect(plan.summary.hasErrors).toBe(true);
    expect(plan.errors[0].code).toBe("INVALID_DESIRED_CONFIG");
  });

  it("returns error for null", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: null, now: NOW });
    expect(plan.summary.hasErrors).toBe(true);
  });
});

describe("buildConfigPlan — INVALID_DESIRED_RULE", () => {
  it("returns error for a rule with an empty name", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [{ ...makeTcpDesired(), name: "" }], now: NOW });
    expect(plan.summary.hasErrors).toBe(true);
    const err = plan.errors.find((e: { code: string }) => e.code === "INVALID_DESIRED_RULE");
    expect(err).toBeDefined();
    expect(err?.field).toBe("rules[0]");
  });

  it("returns error for invalid protocol", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [{ ...makeTcpDesired(), protocol: "quic" }], now: NOW });
    expect(plan.errors.some((e: { code: string }) => e.code === "INVALID_DESIRED_RULE")).toBe(true);
  });

  it("returns error for invalid port (out of range)", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [{ ...makeTcpDesired(), listenPort: 0 }], now: NOW });
    expect(plan.errors.some((e: { code: string }) => e.code === "INVALID_DESIRED_RULE")).toBe(true);
  });

  it("returns error for invalid UDP mode", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [{ ...makeTcpDesired(), protocol: "udp", udpMode: "bad-mode" }], now: NOW });
    expect(plan.errors.some((e: { code: string }) => e.code === "INVALID_DESIRED_RULE")).toBe(true);
  });

  it("returns error for udpMode on a TCP rule", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [{ ...makeTcpDesired(), udpMode: "one-way" }], now: NOW });
    expect(plan.errors.some((e: { code: string }) => e.code === "INVALID_DESIRED_RULE")).toBe(true);
  });

  it("returns errors for multiple invalid rules, with index in field", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [
      { ...makeTcpDesired(), name: "" },
      { ...makeTcpDesired(), listenPort: 99999 }
    ], now: NOW });
    expect(plan.errors.some((e: { field?: string }) => e.field === "rules[0]")).toBe(true);
    expect(plan.errors.some((e: { field?: string }) => e.field === "rules[1]")).toBe(true);
  });
});

describe("buildConfigPlan — DUPLICATE_DESIRED_ID", () => {
  it("returns error when two desired rules share the same id", () => {
    const d1 = { ...makeTcpDesired(), id: "r1" };
    const d2 = { ...makeTcpDesired(), listenPort: 9099, id: "r1" };
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [d1, d2], now: NOW });
    expect(plan.summary.hasErrors).toBe(true);
    const err = plan.errors.find((e) => e.code === "DUPLICATE_DESIRED_ID");
    expect(err).toBeDefined();
    expect(err?.message).toContain('"r1"');
  });
});

describe("buildConfigPlan — DUPLICATE_DESIRED_IDENTITY_KEY", () => {
  it("returns error when two desired rules share the same identity key", () => {
    const d1 = makeTcpDesired();
    const d2 = makeTcpDesired({ name: "Duplicate Binding" });
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [d1, d2], now: NOW });
    expect(plan.summary.hasErrors).toBe(true);
    const err = plan.errors.find((e) => e.code === "DUPLICATE_DESIRED_IDENTITY_KEY");
    expect(err).toBeDefined();
  });
});

describe("buildConfigPlan — AMBIGUOUS_CURRENT_MATCH", () => {
  it("returns error when multiple current rules share the same identity key", () => {
    // Two current rules with the same identity key (should not normally happen, but tests the guard)
    const r1a: ForwardRule = { ...tcpRule, id: "r1a" };
    const r1b: ForwardRule = { ...tcpRule, id: "r1b" };
    const desired = makeTcpDesired(); // no id, matches by key
    const plan = buildConfigPlan({ currentRules: [r1a, r1b], desiredRaw: [desired], now: NOW });
    expect(plan.summary.hasErrors).toBe(true);
    const err = plan.errors.find((e) => e.code === "AMBIGUOUS_CURRENT_MATCH");
    expect(err).toBeDefined();
    expect(err?.message).toContain('"tcp:127.0.0.1:9001"');
  });
});

// ── buildConfigPlan — summary counts ─────────────────────────────────────────

describe("buildConfigPlan — summary with multiple operation types", () => {
  it("correctly counts all operation types", () => {
    const r2: ForwardRule = { ...udpRule };
    const r3: ForwardRule = { id: "r3", name: "Extra", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 9050, targetHost: "127.0.0.1", targetPort: 9051, enabled: false };

    // current: r1 (tcp/9001), r2 (udp/9003), r3 (tcp/9050)
    // desired: r1 unchanged, r2 update (name), new add (tcp/9090), r3 not present → remove
    const desiredR1 = makeTcpDesired(); // unchanged
    const desiredR2Updated = { name: "UDP Renamed", protocol: "udp" as const, listenHost: "127.0.0.1", listenPort: 9003, targetHost: "127.0.0.1", targetPort: 9004, enabled: false, udpMode: "one-way" as const };
    const newRule = { name: "New Rule", protocol: "tcp" as const, listenHost: "127.0.0.1", listenPort: 9090, targetHost: "127.0.0.1", targetPort: 9091, enabled: false };

    const plan = buildConfigPlan({ currentRules: [tcpRule, r2, r3], desiredRaw: [desiredR1, desiredR2Updated, newRule], now: NOW });
    expect(plan.summary.unchanged).toBe(1);
    expect(plan.summary.update).toBe(1);
    expect(plan.summary.add).toBe(1);
    expect(plan.summary.remove).toBe(1);
    expect(plan.summary.hasDrift).toBe(true);
  });

  it("destructive count reflects only destructive ops", () => {
    const desiredWithPortChange = makeTcpDesired({ targetPort: 9999 }); // destructive update
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desiredWithPortChange], now: NOW });
    expect(plan.summary.destructive).toBe(1);
  });

  it("destructive count is 0 for name-only update", () => {
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [makeTcpDesired({ name: "New Name" })], now: NOW });
    expect(plan.summary.destructive).toBe(0);
  });
});

// ── buildConfigPlan — warnings ────────────────────────────────────────────────

describe("buildConfigPlan — LAN_EXPOSURE warning", () => {
  it("emits LAN_EXPOSURE for add with 0.0.0.0 listenHost", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [makeTcpDesired({ listenHost: "0.0.0.0" })], now: NOW });
    const warn = plan.warnings.find((w) => w.code === "LAN_EXPOSURE");
    expect(warn).toBeDefined();
    expect(warn?.message).toContain("TCP Rule");
  });

  it("emits LAN_EXPOSURE for unchanged rule with 0.0.0.0", () => {
    const lanRule: ForwardRule = { ...tcpRule, listenHost: "0.0.0.0" };
    const desired = { ...makeTcpDesired(), listenHost: "0.0.0.0" };
    const plan = buildConfigPlan({ currentRules: [lanRule], desiredRaw: [desired], now: NOW });
    const warn = plan.warnings.find((w) => w.code === "LAN_EXPOSURE");
    expect(warn).toBeDefined();
  });

  it("does not emit LAN_EXPOSURE for normal 127.0.0.1 rules", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [makeTcpDesired()], now: NOW });
    expect(plan.warnings.some((w: { code: string }) => w.code === "LAN_EXPOSURE")).toBe(false);
  });
});

describe("buildConfigPlan — hasDrift and hasErrors", () => {
  it("hasDrift is false when all rules are unchanged", () => {
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [makeTcpDesired()], now: NOW });
    expect(plan.summary.hasDrift).toBe(false);
  });

  it("hasDrift is true when there is an add", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [makeTcpDesired()], now: NOW });
    expect(plan.summary.hasDrift).toBe(true);
  });

  it("hasDrift is true when there is a remove", () => {
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [], now: NOW });
    expect(plan.summary.hasDrift).toBe(true);
  });

  it("hasDrift is true when there is an update", () => {
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [makeTcpDesired({ name: "Changed" })], now: NOW });
    expect(plan.summary.hasDrift).toBe(true);
  });

  it("hasErrors is true when validation fails", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [{ name: "" }], now: NOW });
    expect(plan.summary.hasErrors).toBe(true);
  });

  it("hasErrors is false for warnings only", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [makeTcpDesired({ listenHost: "0.0.0.0" })], now: NOW });
    expect(plan.summary.hasErrors).toBe(false);
    expect(plan.warnings.length).toBeGreaterThan(0);
  });
});

// ── buildConfigPlan — input normalization ─────────────────────────────────────

describe("buildConfigPlan — accepts various desiredRaw shapes", () => {
  it("accepts a raw array", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [makeTcpDesired()], now: NOW });
    expect(plan.summary.add).toBe(1);
  });

  it("accepts a wrapper object { rules: [...] }", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: { rules: [makeTcpDesired()] }, now: NOW });
    expect(plan.summary.add).toBe(1);
  });

  it("accepts an exported config shape { version, exportedAt, rules }", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: { version: "1", exportedAt: "2026-01-01T00:00:00.000Z", rules: [makeTcpDesired()] }, now: NOW });
    expect(plan.summary.add).toBe(1);
  });
});

// ── buildConfigPlan — material vs transient fields ────────────────────────────

describe("buildConfigPlan — material field comparison", () => {
  it("detects changes for all 8 material fields", () => {
    const udpCurrent: ForwardRule = { id: "ux", name: "A", protocol: "udp", listenHost: "127.0.0.1", listenPort: 8000, targetHost: "10.0.0.1", targetPort: 8001, enabled: false, udpMode: "one-way" };
    // Match by id so listenHost/listenPort changes are tracked as updates, not new adds
    const udpDesired = { id: "ux", name: "B", protocol: "udp" as const, listenHost: "0.0.0.0", listenPort: 8002, targetHost: "10.0.0.2", targetPort: 8003, enabled: true, udpMode: "bidirectional-multi-client" as const };
    const plan = buildConfigPlan({ currentRules: [udpCurrent], desiredRaw: [udpDesired], now: NOW });
    const op = plan.operations[0];
    expect(op.type).toBe("update");
    const fields = op.changes?.map((c) => c.field) ?? [];
    expect(fields).toContain("name");
    expect(fields).toContain("listenHost");
    expect(fields).toContain("listenPort");
    expect(fields).toContain("targetHost");
    expect(fields).toContain("targetPort");
    expect(fields).toContain("enabled");
    expect(fields).toContain("udpMode");
  });

  it("id field is NOT included in material field changes", () => {
    // Desired has a different id than current — the id is an identity key, not a material field
    const desired = { ...makeTcpDesired() }; // no id
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    // matched by key, current has id:r1, desired has id:undefined
    // id is not in MATERIAL_FIELDS, so no change detected
    expect(plan.operations[0].type).toBe("unchanged");
  });
});

// ── buildConfigPlan — UDP defaults ────────────────────────────────────────────

describe("buildConfigPlan — UDP defaults", () => {
  it("desired UDP with no udpMode defaults to one-way → unchanged if current is one-way", () => {
    const desired = { name: "UDP Rule", protocol: "udp" as const, listenHost: "127.0.0.1", listenPort: 9003, targetHost: "127.0.0.1", targetPort: 9004, enabled: false };
    const plan = buildConfigPlan({ currentRules: [udpRule], desiredRaw: [desired], now: NOW });
    expect(plan.summary.unchanged).toBe(1);
  });
});

// ── buildApplyImportFromPlan — apply orchestration ─────────────────────────────

describe("buildApplyImportFromPlan", () => {
  // Deterministic id generator for assertions — production defaults to crypto.randomUUID.
  let counter = 0;
  const seqId = () => `gen-${++counter}`;
  const resetSeq = () => { counter = 0; };

  it("no-drift plan → desired config preserves the unchanged rule with its current id", () => {
    const desired = { ...makeTcpDesired() }; // matched by key, no explicit id
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    expect(plan.summary.hasDrift).toBe(false);

    resetSeq();
    const { rules, applied } = buildApplyImportFromPlan(plan, seqId);
    expect(rules).toHaveLength(1);
    expect(rules[0].id).toBe("r1"); // current id preserved, not regenerated
    expect(counter).toBe(0); // newId never called for an unchanged match
    expect(applied).toEqual({ add: 0, update: 0, remove: 0, unchanged: 1 });
  });

  it("add operation → injects a freshly generated id", () => {
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [makeTcpDesired()], now: NOW });
    expect(plan.operations[0].type).toBe("add");

    resetSeq();
    const { rules, applied } = buildApplyImportFromPlan(plan, seqId);
    expect(rules).toHaveLength(1);
    expect(rules[0].id).toBe("gen-1");
    expect(applied).toEqual({ add: 1, update: 0, remove: 0, unchanged: 0 });
  });

  it("add operation with an explicit desired id → preserves that id (no generation)", () => {
    const desired = { ...makeTcpDesired(), id: "explicit-id", listenPort: 9999 };
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [desired], now: NOW });
    expect(plan.operations[0].type).toBe("add");

    resetSeq();
    const { rules } = buildApplyImportFromPlan(plan, seqId);
    expect(rules[0].id).toBe("explicit-id");
    expect(counter).toBe(0);
  });

  it("update operation → preserves the matched current rule id", () => {
    const desired = { ...makeTcpDesired(), targetPort: 9999 }; // forwarding field changed
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    expect(plan.operations[0].type).toBe("update");

    const { rules, applied } = buildApplyImportFromPlan(plan, seqId);
    expect(rules[0].id).toBe("r1");
    expect(rules[0].targetPort).toBe(9999);
    expect(applied).toEqual({ add: 0, update: 1, remove: 0, unchanged: 0 });
  });

  it("remove operation → omitted from the desired config", () => {
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [], now: NOW });
    expect(plan.operations[0].type).toBe("remove");

    const { rules, applied } = buildApplyImportFromPlan(plan, seqId);
    expect(rules).toHaveLength(0);
    expect(applied).toEqual({ add: 0, update: 0, remove: 1, unchanged: 0 });
  });

  it("mixed plan → deterministic order: add, update, unchanged kept; remove dropped", () => {
    // current: tcpRule (r1, port 9001), udpRule (r2, port 9003)
    // desired: tcpRule updated (targetPort change), udpRule unchanged, a new rule added.
    // tcpRule's remove counterpart: none. udpRule kept. r1 updated. one add. nothing removed.
    const updatedTcp = { ...makeTcpDesired(), targetPort: 9999 };
    const unchangedUdp = { name: "UDP Rule", protocol: "udp" as const, listenHost: "127.0.0.1", listenPort: 9003, targetHost: "127.0.0.1", targetPort: 9004, enabled: false };
    const added = { name: "New", protocol: "tcp" as const, listenHost: "127.0.0.1", listenPort: 9100, targetHost: "127.0.0.1", targetPort: 9200, enabled: true };
    const plan = buildConfigPlan({ currentRules: [tcpRule, udpRule], desiredRaw: [updatedTcp, unchangedUdp, added], now: NOW });

    resetSeq();
    const { rules, applied } = buildApplyImportFromPlan(plan, seqId);
    // Order follows plan.operations order (desired order, then removes — none here).
    expect(rules.map((r) => r.id)).toEqual(["r1", "r2", "gen-1"]);
    expect(applied).toEqual({ add: 1, update: 1, remove: 0, unchanged: 1 });
  });

  it("does not mutate the input plan", () => {
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [makeTcpDesired({ targetPort: 9999 })], now: NOW });
    const before = JSON.stringify(plan);
    buildApplyImportFromPlan(plan, seqId);
    expect(JSON.stringify(plan)).toBe(before);
  });

  it("preserves udpMode on the desired rule", () => {
    const desired = { name: "UDP Rule", protocol: "udp" as const, listenHost: "127.0.0.1", listenPort: 9003, targetHost: "127.0.0.1", targetPort: 9004, enabled: false, udpMode: "bidirectional-multi-client" as const };
    const plan = buildConfigPlan({ currentRules: [udpRule], desiredRaw: [desired], now: NOW });
    const { rules } = buildApplyImportFromPlan(plan, seqId);
    expect(rules[0].udpMode).toBe("bidirectional-multi-client");
  });

  it("preserves group on the desired rule and omits it when absent", () => {
    const grouped = buildConfigPlan({ currentRules: [], desiredRaw: [makeTcpDesired({ group: "web-team" })], now: NOW });
    expect(buildApplyImportFromPlan(grouped, seqId).rules[0].group).toBe("web-team");

    const ungrouped = buildConfigPlan({ currentRules: [], desiredRaw: [makeTcpDesired()], now: NOW });
    expect("group" in buildApplyImportFromPlan(ungrouped, seqId).rules[0]).toBe(false);
  });
});

describe("buildConfigPlan — group metadata drift", () => {
  it("treats a group-only change as a non-destructive update", () => {
    const current: ForwardRule = { ...tcpRule, group: "before-team" };
    const desired = makeTcpDesired({ id: "r1", group: "after-team" });
    const plan = buildConfigPlan({ currentRules: [current], desiredRaw: [desired], now: NOW });

    expect(plan.summary.update).toBe(1);
    expect(plan.summary.destructive).toBe(0);
    const op = plan.operations.find((o) => o.type === "update");
    expect(op?.destructive).toBe(false);
    const change = op?.changes?.find((c) => c.field === "group");
    expect(change).toEqual({ field: "group", before: "before-team", after: "after-team" });
  });

  it("produces no drift when neither current nor desired has a group (legacy parity)", () => {
    const desired = makeTcpDesired({ id: "r1" });
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    expect(plan.summary.hasDrift).toBe(false);
    expect(plan.operations[0].type).toBe("unchanged");
  });

  it("detects adding a group to a previously ungrouped rule", () => {
    const desired = makeTcpDesired({ id: "r1", group: "ops" });
    const plan = buildConfigPlan({ currentRules: [tcpRule], desiredRaw: [desired], now: NOW });
    const op = plan.operations.find((o) => o.type === "update");
    expect(op?.changes?.find((c) => c.field === "group")).toEqual({ field: "group", before: undefined, after: "ops" });
    expect(op?.destructive).toBe(false);
  });

  it("default id generator produces unique ids for adds (no injected generator)", () => {
    const a = { name: "A", protocol: "tcp" as const, listenHost: "127.0.0.1", listenPort: 9100, targetHost: "127.0.0.1", targetPort: 9200, enabled: true };
    const b = { name: "B", protocol: "tcp" as const, listenHost: "127.0.0.1", listenPort: 9101, targetHost: "127.0.0.1", targetPort: 9201, enabled: true };
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [a, b], now: NOW });
    const { rules } = buildApplyImportFromPlan(plan);
    expect(rules).toHaveLength(2);
    expect(rules[0].id).toBeTruthy();
    expect(rules[1].id).toBeTruthy();
    expect(rules[0].id).not.toBe(rules[1].id);
  });
});

describe("buildConfigPlan clock default (Slice 30)", () => {
  it("uses the current time when no clock is provided", () => {
    const before = Date.now();
    const plan = buildConfigPlan({ currentRules: [], desiredRaw: [] });
    const after = Date.now();
    const generated = Date.parse(plan.generatedAt);
    expect(Number.isNaN(generated)).toBe(false);
    expect(generated).toBeGreaterThanOrEqual(before);
    expect(generated).toBeLessThanOrEqual(after);
  });
});
