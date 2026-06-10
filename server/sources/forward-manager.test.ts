import net from "node:net";
import { afterEach, describe, expect, it } from "vitest";
import type { ExportedConfig, ForwardRule } from "@portier/shared";
import { ConflictError, ForwardManager, NotFoundError, ValidationError, type RuleStore } from "./forward-manager.js";
import { ActivityStore } from "./activity/activity-store.js";
import { getFreeTcpPort } from "./test-helpers.js";

class MemoryStore implements RuleStore {
  constructor(private rules: ForwardRule[] = []) {}

  async load(): Promise<ForwardRule[]> {
    return this.rules;
  }

  async save(rules: ForwardRule[]): Promise<void> {
    this.rules = rules;
  }
}

// ControllableStore is a RuleStore fake whose save() can be made to fail on
// demand (Test-D persist-failure rollback tests). It also records every
// successfully-persisted snapshot so tests can assert nothing was written on a
// failed operation. Production config-store.ts is unchanged.
class ControllableStore implements RuleStore {
  saveCallCount = 0;
  readonly savedSnapshots: ForwardRule[][] = [];
  shouldFail = false;
  saveError: Error = new Error("simulated persist failure");

  constructor(private rules: ForwardRule[] = []) {}

  async load(): Promise<ForwardRule[]> {
    return this.rules.map((rule) => ({ ...rule }));
  }

  async save(rules: ForwardRule[]): Promise<void> {
    this.saveCallCount += 1;
    if (this.shouldFail) {
      throw this.saveError;
    }
    this.rules = rules.map((rule) => ({ ...rule }));
    this.savedSnapshots.push(this.rules.map((rule) => ({ ...rule })));
  }

  /** The last config the store actually persisted, or null if it never persisted. */
  lastPersisted(): ForwardRule[] | null {
    return this.savedSnapshots.length > 0 ? this.savedSnapshots[this.savedSnapshots.length - 1] : null;
  }
}

// Minimal TCP target server for update tests that need a running forwarder.
function startTcpTarget(port: number): Promise<net.Server> {
  const server = net.createServer();
  return new Promise((resolve) => server.listen(port, "127.0.0.1", () => resolve(server)));
}

function closeTcpServer(server: net.Server): Promise<void> {
  return new Promise((resolve) => server.close(() => resolve()));
}

describe("ForwardManager updateRule behavior", () => {
  const cleanup: Array<() => Promise<void>> = [];

  afterEach(async () => {
    await Promise.all(cleanup.splice(0).map((fn) => fn()));
  });

  async function startedRule(manager: ForwardManager, listenPort: number, targetPort: number) {
    const rule = await manager.addRule({
      id: "r1",
      name: "Test",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort,
      enabled: false
    });
    await manager.startRule(rule.id);
    cleanup.push(() => manager.stopAll());
    return rule;
  }

  it("does not restart a running rule when only enabled changes to false", async () => {
    const [listenPort, targetPort] = await Promise.all([getFreeTcpPort(), getFreeTcpPort()]);
    const target = await startTcpTarget(targetPort);
    cleanup.push(() => closeTcpServer(target));

    const manager = new ForwardManager(new MemoryStore());
    const rule = await startedRule(manager, listenPort, targetPort);
    expect(manager.getStatus(rule.id).running).toBe(true);

    const updated = await manager.updateRule(rule.id, { enabled: false });
    expect(updated.enabled).toBe(false);
    expect(manager.getStatus(rule.id).running).toBe(true);
  });

  it("does not restart a running rule when only name changes", async () => {
    const [listenPort, targetPort] = await Promise.all([getFreeTcpPort(), getFreeTcpPort()]);
    const target = await startTcpTarget(targetPort);
    cleanup.push(() => closeTcpServer(target));

    const manager = new ForwardManager(new MemoryStore());
    const rule = await startedRule(manager, listenPort, targetPort);
    expect(manager.getStatus(rule.id).running).toBe(true);

    const updated = await manager.updateRule(rule.id, { name: "Renamed" });
    expect(updated.name).toBe("Renamed");
    expect(manager.getStatus(rule.id).running).toBe(true);
  });

  it("restarts a running rule when a forwarding field changes", async () => {
    const [listenPort, targetPortA, targetPortB] = await Promise.all([
      getFreeTcpPort(), getFreeTcpPort(), getFreeTcpPort()
    ]);
    const targetA = await startTcpTarget(targetPortA);
    const targetB = await startTcpTarget(targetPortB);
    cleanup.push(() => closeTcpServer(targetA));
    cleanup.push(() => closeTcpServer(targetB));

    const manager = new ForwardManager(new MemoryStore());
    const rule = await startedRule(manager, listenPort, targetPortA);
    expect(manager.getStatus(rule.id).running).toBe(true);

    const updated = await manager.updateRule(rule.id, { targetPort: targetPortB });
    expect(updated.targetPort).toBe(targetPortB);
    expect(manager.getStatus(rule.id).running).toBe(true);
  });

  it("does not start a stopped rule when enabled changes to true", async () => {
    const manager = new ForwardManager(new MemoryStore());
    const rule = await manager.addRule({
      id: "r1",
      name: "Test",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 49910,
      targetHost: "127.0.0.1",
      targetPort: 49911,
      enabled: false
    });
    expect(manager.getStatus(rule.id).running).toBe(false);

    const updated = await manager.updateRule(rule.id, { enabled: true });
    expect(updated.enabled).toBe(true);
    expect(manager.getStatus(rule.id).running).toBe(false);
  });
});

describe("ForwardManager reorderRules behavior", () => {
  const cleanup: Array<() => Promise<void>> = [];

  afterEach(async () => {
    await Promise.all(cleanup.splice(0).map((fn) => fn()));
  });

  it("reorders rules without restarting running rules", async () => {
    const [listenPort1, listenPort2, targetPort] = await Promise.all([
      getFreeTcpPort(), getFreeTcpPort(), getFreeTcpPort()
    ]);
    const target = await startTcpTarget(targetPort);
    cleanup.push(() => closeTcpServer(target));

    const manager = new ForwardManager(new MemoryStore());

    const r1 = await manager.addRule({
      id: "r1", name: "Rule A", protocol: "tcp",
      listenHost: "127.0.0.1", listenPort: listenPort1,
      targetHost: "127.0.0.1", targetPort, enabled: false
    });
    const r2 = await manager.addRule({
      id: "r2", name: "Rule B", protocol: "tcp",
      listenHost: "127.0.0.1", listenPort: listenPort2,
      targetHost: "127.0.0.1", targetPort, enabled: false
    });
    cleanup.push(() => manager.stopAll());

    await manager.startRule(r1.id);
    expect(manager.getStatus(r1.id).running).toBe(true);

    await manager.reorderRules([r2.id, r1.id]);

    expect(manager.getStatus(r1.id).running).toBe(true);

    const rules = manager.listRules();
    expect(rules[0].id).toBe("r2");
    expect(rules[1].id).toBe("r1");
  });
});

describe("ForwardManager importConfig merge mode", () => {
  const cleanup: Array<() => Promise<void>> = [];

  afterEach(async () => {
    await Promise.all(cleanup.splice(0).map((fn) => fn()));
  });

  it("starts enabled rules imported in merge mode", async () => {
    const listenPort = await getFreeTcpPort();
    const manager = new ForwardManager(new MemoryStore());
    cleanup.push(() => manager.stopAll());

    const config: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [
        {
          id: "r-merge",
          name: "Merge Test",
          protocol: "tcp",
          listenHost: "127.0.0.1",
          listenPort,
          targetHost: "127.0.0.1",
          targetPort: 49999,
          enabled: true
        }
      ]
    };

    const result = await manager.importConfig(config, "merge");
    expect(result.imported).toBe(1);
    expect(result.errors).toHaveLength(0);
    expect(manager.getStatus("r-merge").running).toBe(true);
  });

  it("rejects merge when imported rule conflicts with existing listen binding", async () => {
    const listenPort = await getFreeTcpPort();
    const manager = new ForwardManager(new MemoryStore());

    await manager.addRule({
      id: "existing",
      name: "Existing",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: 49999,
      enabled: false
    });

    const config: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [
        {
          id: "incoming",
          name: "Conflicting",
          protocol: "tcp",
          listenHost: "127.0.0.1",
          listenPort,
          targetHost: "127.0.0.1",
          targetPort: 50000,
          enabled: false
        }
      ]
    };

    const result = await manager.importConfig(config, "merge");
    expect(result.imported).toBe(0);
    expect(result.errors.length).toBeGreaterThan(0);
    expect(result.errors[0]).toContain("conflicts with existing rule");
  });
});

describe("ForwardManager with ActivityStore", () => {
  const cleanup: Array<() => Promise<void>> = [];

  afterEach(async () => {
    await Promise.all(cleanup.splice(0).map((fn) => fn()));
  });

  it("logs rule.started when a rule starts with activity store present", async () => {
    const listenPort = await getFreeTcpPort();
    const activity = new ActivityStore();
    const manager = new ForwardManager(new MemoryStore(), activity);
    cleanup.push(() => manager.stopAll());

    await manager.addRule({
      id: "act-r1",
      name: "Activity Test",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: 49999,
      enabled: false
    });
    await manager.startRule("act-r1");

    const events = activity.list({});
    expect(events.some((e) => e.type === "rule.started")).toBe(true);
  });

  it("logs rule.error and re-throws when startRule fails", async () => {
    const occupiedPort = await getFreeTcpPort();
    const occupier = net.createServer();
    await new Promise<void>((resolve) => occupier.listen(occupiedPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => occupier.close(() => resolve())));

    const activity = new ActivityStore();
    const manager = new ForwardManager(new MemoryStore(), activity);

    await manager.addRule({
      id: "fail-r1",
      name: "Fail Test",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: occupiedPort,
      targetHost: "127.0.0.1",
      targetPort: 49999,
      enabled: false
    });

    await expect(manager.startRule("fail-r1")).rejects.toThrow();
    const events = activity.list({});
    expect(events.some((e) => e.type === "rule.error")).toBe(true);
  });
});

describe("ForwardManager addRule with enabled:true", () => {
  const cleanup: Array<() => Promise<void>> = [];

  afterEach(async () => {
    await Promise.all(cleanup.splice(0).map((fn) => fn()));
  });

  it("starts the rule immediately when enabled is true", async () => {
    const listenPort = await getFreeTcpPort();
    const manager = new ForwardManager(new MemoryStore());
    cleanup.push(() => manager.stopAll());

    const rule = await manager.addRule({
      id: "enabled-add",
      name: "Enabled Add",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: 49999,
      enabled: true
    });

    expect(manager.getStatus(rule.id).running).toBe(true);
  });
});

describe("ForwardManager updateRule validation", () => {
  it("throws ValidationError for an invalid patch input", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule({
      id: "r1",
      name: "Test",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 49600,
      targetHost: "127.0.0.1",
      targetPort: 49601,
      enabled: false
    });

    await expect(manager.updateRule("r1", { protocol: "ftp" })).rejects.toBeInstanceOf(ValidationError);
  });
});

describe("ForwardManager flush", () => {
  it("persists current rules without error", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule({
      id: "r1",
      name: "Test",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 49700,
      targetHost: "127.0.0.1",
      targetPort: 49701,
      enabled: false
    });
    await expect(manager.flush()).resolves.toBeUndefined();
  });
});

describe("ForwardManager importConfig replace mode", () => {
  const cleanup: Array<() => Promise<void>> = [];

  afterEach(async () => {
    await Promise.all(cleanup.splice(0).map((fn) => fn()));
  });

  it("starts enabled rules after replace", async () => {
    const listenPort = await getFreeTcpPort();
    const manager = new ForwardManager(new MemoryStore());
    cleanup.push(() => manager.stopAll());

    const config: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [
        {
          id: "r-replace",
          name: "Replace Test",
          protocol: "tcp",
          listenHost: "127.0.0.1",
          listenPort,
          targetHost: "127.0.0.1",
          targetPort: 49999,
          enabled: true
        }
      ]
    };

    const result = await manager.importConfig(config, "replace");
    expect(result.imported).toBe(1);
    expect(result.errors).toHaveLength(0);
    expect(manager.getStatus("r-replace").running).toBe(true);
  });
});

describe("ForwardManager reorderRules error handling", () => {
  it("throws NotFoundError when ID does not exist", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule({
      id: "r1",
      name: "Test",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 49800,
      targetHost: "127.0.0.1",
      targetPort: 49801,
      enabled: false
    });

    await expect(manager.reorderRules(["r1", "unknown-id"])).rejects.toBeInstanceOf(NotFoundError);
  });
});

describe("ForwardManager duplicate listen binding detection", () => {
  it("rejects rules with the same protocol, host, and port", async () => {
    const manager = new ForwardManager(new MemoryStore());

    await manager.addRule({
      id: "one",
      name: "One",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 10000,
      targetHost: "127.0.0.1",
      targetPort: 10001,
      enabled: false
    });

    await expect(
      manager.addRule({
        id: "two",
        name: "Two",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort: 10000,
        targetHost: "127.0.0.1",
        targetPort: 10002,
        enabled: false
      })
    ).rejects.toBeInstanceOf(ConflictError);
  });

  it("allows TCP and UDP to share a port number", async () => {
    const manager = new ForwardManager(new MemoryStore());

    await manager.addRule({
      id: "tcp",
      name: "TCP",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 10000,
      targetHost: "127.0.0.1",
      targetPort: 10001,
      enabled: false
    });

    await expect(
      manager.addRule({
        id: "udp",
        name: "UDP",
        protocol: "udp",
        listenHost: "127.0.0.1",
        listenPort: 10000,
        targetHost: "127.0.0.1",
        targetPort: 10001,
        enabled: false
      })
    ).resolves.toMatchObject({ id: "udp" });
  });
});

// ── Persist-failure rollback (Test-D — runtime parity with the Go manager) ────
//
// The Go manager rolls back every persist path so that a failed store.Save()
// leaves NO partial in-memory or running-state mutation
// (service/sources/manager/manager_test.go: TestCreateRulePersistFailureRollsBack,
// TestUpdateRulePersistFailureRollsBack,
// TestUpdateRulePersistFailureWithRunningRuleRestartsOriginal,
// TestDeleteRulePersistFailureRollsBack, TestReorderRulesPersistFailureRollsBack).
// These tests prove the TypeScript ForwardManager has the same rollback semantics.
// Why it matters: persist-failure paths are correctness paths, not optional edge
// cases — a half-applied mutation that is not on disk diverges the two runtimes
// and can leave the in-memory rule set inconsistent with rules.json. See the
// durable testing rule in CLAUDE.md/AGENTS.md.
describe("ForwardManager persist-failure rollback (Test-D, Go parity)", () => {
  const cleanup: Array<() => Promise<void>> = [];

  afterEach(async () => {
    await Promise.all(cleanup.splice(0).map((fn) => fn()));
  });

  function ruleInput(
    overrides: Partial<ForwardRule> & { id: string; listenPort: number }
  ): ForwardRule {
    return {
      name: "Test",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      targetHost: "127.0.0.1",
      targetPort: 49999,
      enabled: false,
      ...overrides
    } as ForwardRule;
  }

  // A. Mirrors Go TestCreateRulePersistFailureRollsBack.
  it("addRule removes the appended rule and persists nothing when save fails", async () => {
    const store = new ControllableStore();
    const activity = new ActivityStore();
    const manager = new ForwardManager(store, activity);

    store.shouldFail = true;
    await expect(manager.addRule(ruleInput({ id: "r1", listenPort: 49000 }))).rejects.toThrow();

    expect(manager.listRules()).toHaveLength(0);
    expect(store.lastPersisted()).toBeNull();
    // A failed create must not emit a rule.created activity event.
    expect(activity.list({}).some((e) => e.type === "rule.created")).toBe(false);
  });

  // B. Mirrors Go TestUpdateRulePersistFailureRollsBack (non-forwarding field).
  it("updateRule restores the original rule when a non-forwarding update fails to persist", async () => {
    const store = new ControllableStore();
    const manager = new ForwardManager(store);
    await manager.addRule(ruleInput({ id: "r1", name: "Original", listenPort: 49010 }));

    store.shouldFail = true;
    await expect(manager.updateRule("r1", { name: "Renamed" })).rejects.toThrow();

    const rules = manager.listRules();
    expect(rules).toHaveLength(1);
    expect(rules[0].name).toBe("Original");
    expect(manager.getStatus("r1").running).toBe(false);
  });

  // C. Mirrors Go TestUpdateRulePersistFailureWithRunningRuleRestartsOriginal.
  //    Uses a real running forwarder; ports come from the Test-A free-port helper.
  it("updateRule restores and restarts a running rule when a forwarding update fails to persist", async () => {
    const [listenPort, targetPortA, targetPortB] = await Promise.all([
      getFreeTcpPort(),
      getFreeTcpPort(),
      getFreeTcpPort()
    ]);
    const targetA = await startTcpTarget(targetPortA);
    cleanup.push(() => closeTcpServer(targetA));

    const store = new ControllableStore();
    const manager = new ForwardManager(store);
    cleanup.push(() => manager.stopAll());

    await manager.addRule(ruleInput({ id: "r1", listenPort, targetPort: targetPortA }));
    await manager.startRule("r1");
    expect(manager.getStatus("r1").running).toBe(true);

    store.shouldFail = true;
    await expect(manager.updateRule("r1", { targetPort: targetPortB })).rejects.toThrow();

    const rules = manager.listRules();
    expect(rules).toHaveLength(1);
    expect(rules[0].targetPort).toBe(targetPortA); // original forwarding config restored
    expect(manager.getStatus("r1").running).toBe(true); // pre-update running state restored
  });

  // D. Mirrors Go TestDeleteRulePersistFailureRollsBack.
  it("deleteRule keeps the rule when save fails", async () => {
    const store = new ControllableStore();
    const manager = new ForwardManager(store);
    await manager.addRule(ruleInput({ id: "r1", name: "Keep", listenPort: 49020 }));

    store.shouldFail = true;
    await expect(manager.deleteRule("r1")).rejects.toThrow();

    const rules = manager.listRules();
    expect(rules).toHaveLength(1);
    expect(rules[0].id).toBe("r1");
    expect(rules[0].name).toBe("Keep");
  });

  // D2. Delete of a *running* rule: rollback must restore both the rule and its
  //     running state (covers the wasRunning restart branch; Go restores the
  //     prior runtime state in manager.DeleteRule).
  it("deleteRule restores and restarts a running rule when save fails", async () => {
    const [listenPort, targetPort] = await Promise.all([getFreeTcpPort(), getFreeTcpPort()]);
    const target = await startTcpTarget(targetPort);
    cleanup.push(() => closeTcpServer(target));

    const store = new ControllableStore();
    const manager = new ForwardManager(store);
    cleanup.push(() => manager.stopAll());

    await manager.addRule(ruleInput({ id: "r1", listenPort, targetPort }));
    await manager.startRule("r1");
    expect(manager.getStatus("r1").running).toBe(true);

    store.shouldFail = true;
    await expect(manager.deleteRule("r1")).rejects.toThrow();

    const rules = manager.listRules();
    expect(rules).toHaveLength(1);
    expect(rules[0].id).toBe("r1");
    expect(manager.getStatus("r1").running).toBe(true); // running state restored
  });

  // E. Mirrors Go TestReorderRulesPersistFailureRollsBack.
  it("reorderRules preserves the original order when save fails", async () => {
    const store = new ControllableStore();
    const manager = new ForwardManager(store);
    await manager.addRule(ruleInput({ id: "r1", listenPort: 49030 }));
    await manager.addRule(ruleInput({ id: "r2", name: "Other", listenPort: 49031 }));

    store.shouldFail = true;
    await expect(manager.reorderRules(["r2", "r1"])).rejects.toThrow();

    expect(manager.listRules().map((rule) => rule.id)).toEqual(["r1", "r2"]);
  });

  // F. TypeScript-side extension of the parity set: the Go manager has the
  //    ImportConfig rollback branch (manager.go) but no dedicated Go test for it.
  //    Proves the existing config survives a failed replace-import persist.
  it("importConfig replace keeps the existing config when save fails", async () => {
    const store = new ControllableStore();
    const manager = new ForwardManager(store);
    await manager.addRule(ruleInput({ id: "r1", name: "Original", listenPort: 49040 }));

    store.shouldFail = true;
    const config: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [ruleInput({ id: "r2", name: "Imported", listenPort: 49041 })]
    };
    await expect(manager.importConfig(config, "replace")).rejects.toThrow();

    const rules = manager.listRules();
    expect(rules).toHaveLength(1);
    expect(rules[0].id).toBe("r1");
    expect(rules[0].name).toBe("Original");
  });

  // F2. Merge import: a failed persist must stop any forwarder started during the
  //     merge and restore the prior rules (covers the merge rollback branch).
  it("importConfig merge stops started forwarders and restores config when save fails", async () => {
    const [existingPort, incomingPort] = await Promise.all([getFreeTcpPort(), getFreeTcpPort()]);
    const store = new ControllableStore();
    const manager = new ForwardManager(store);
    cleanup.push(() => manager.stopAll());

    await manager.addRule(ruleInput({ id: "r1", name: "Original", listenPort: existingPort }));

    store.shouldFail = true;
    const config: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [ruleInput({ id: "r2", name: "Incoming", listenPort: incomingPort, enabled: true })]
    };
    await expect(manager.importConfig(config, "merge")).rejects.toThrow();

    const rules = manager.listRules();
    expect(rules).toHaveLength(1);
    expect(rules[0].id).toBe("r1");
    // The merged rule's forwarder must not be left running after rollback.
    expect(manager.getStatus("r2").running).toBe(false);
  });
});
