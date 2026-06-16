import net from "node:net";
import { afterEach, describe, expect, it } from "vitest";
import type { ExportedConfig, ForwardRule } from "@portier/shared";
import { ConflictError, errorMessage, ForwardManager, NotFoundError, ValidationError, type RuleStore } from "./forward-manager.js";
import { ActivityStore } from "./activity/activity-store.js";
import { getFreeTcpPort, startRuleStable, startTcpServerOnFreePort } from "./test-helpers.js";

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

// Closes a TCP target server opened via startTcpServerOnFreePort (which binds a
// live listener on a free port, avoiding the allocate-close-rebind race).
function closeTcpServer(server: net.Server): Promise<void> {
  return new Promise((resolve) => server.close(() => resolve()));
}

describe("ForwardManager updateRule behavior", () => {
  const cleanup: Array<() => Promise<void>> = [];

  afterEach(async () => {
    await Promise.all(cleanup.splice(0).map((fn) => fn()));
  });

  // Creates a stopped rule on a fresh listen port and starts it via the Test-A
  // bind-retry helper (startRuleStable rebinds on EADDRINUSE only), so the
  // listen bind cannot lose a TOCTOU race with a parallel test.
  async function startedRule(manager: ForwardManager, targetPort: number) {
    const listenPort = await getFreeTcpPort();
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
    await startRuleStable(manager, rule.id, getFreeTcpPort);
    cleanup.push(() => manager.stopAll());
    return rule;
  }

  it("does not restart a running rule when only enabled changes to false", async () => {
    const { server: target, port: targetPort } = await startTcpServerOnFreePort();
    cleanup.push(() => closeTcpServer(target));

    const manager = new ForwardManager(new MemoryStore());
    const rule = await startedRule(manager, targetPort);
    expect(manager.getStatus(rule.id).running).toBe(true);

    const updated = await manager.updateRule(rule.id, { enabled: false });
    expect(updated.enabled).toBe(false);
    expect(manager.getStatus(rule.id).running).toBe(true);
  });

  it("does not restart a running rule when only name changes", async () => {
    const { server: target, port: targetPort } = await startTcpServerOnFreePort();
    cleanup.push(() => closeTcpServer(target));

    const manager = new ForwardManager(new MemoryStore());
    const rule = await startedRule(manager, targetPort);
    expect(manager.getStatus(rule.id).running).toBe(true);

    const updated = await manager.updateRule(rule.id, { name: "Renamed" });
    expect(updated.name).toBe("Renamed");
    expect(manager.getStatus(rule.id).running).toBe(true);
  });

  it("restarts a running rule when a forwarding field changes", async () => {
    const { server: targetA, port: targetPortA } = await startTcpServerOnFreePort();
    const { server: targetB, port: targetPortB } = await startTcpServerOnFreePort();
    cleanup.push(() => closeTcpServer(targetA));
    cleanup.push(() => closeTcpServer(targetB));

    const manager = new ForwardManager(new MemoryStore());
    const rule = await startedRule(manager, targetPortA);
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
    const [listenPort1, listenPort2] = await Promise.all([getFreeTcpPort(), getFreeTcpPort()]);
    const { server: target, port: targetPort } = await startTcpServerOnFreePort();
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

    await startRuleStable(manager, r1.id, getFreeTcpPort);
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
    await startRuleStable(manager, "act-r1", getFreeTcpPort);

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

describe("ForwardManager rule-scoped activity payloads (emitRuleEvent, Go parity)", () => {
  // Locks the exact payload the private emitRuleEvent helper produces for every
  // rule-scoped event, so the helper refactor cannot drift ruleId/ruleName/
  // protocol/severity/message from the prior inline blocks. Mirrors the Go
  // manager's Test*ActivityPayload tests. Generated id/timestamp are not asserted.
  const cleanup: Array<() => Promise<void>> = [];

  afterEach(async () => {
    await Promise.all(cleanup.splice(0).map((fn) => fn()));
  });

  function lastOfType(activity: ActivityStore, type: string) {
    return activity.list({}).filter((e) => e.type === type).at(-1);
  }

  it("emits exact payloads for create / update / delete (no sockets)", async () => {
    const activity = new ActivityStore();
    const manager = new ForwardManager(new MemoryStore(), activity);

    await manager.addRule({
      id: "p1",
      name: "Payload Rule",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 49600,
      targetHost: "127.0.0.1",
      targetPort: 49601,
      enabled: false
    });
    expect(lastOfType(activity, "rule.created")).toMatchObject({
      type: "rule.created",
      severity: "success",
      ruleId: "p1",
      ruleName: "Payload Rule",
      protocol: "tcp",
      message: 'Rule "Payload Rule" created.'
    });

    await manager.updateRule("p1", { name: "Renamed Rule" });
    expect(lastOfType(activity, "rule.updated")).toMatchObject({
      type: "rule.updated",
      severity: "info",
      ruleId: "p1",
      ruleName: "Renamed Rule",
      protocol: "tcp",
      message: 'Rule "Renamed Rule" updated.'
    });

    await manager.deleteRule("p1");
    expect(lastOfType(activity, "rule.deleted")).toMatchObject({
      type: "rule.deleted",
      severity: "warning",
      ruleId: "p1",
      ruleName: "Renamed Rule",
      protocol: "tcp",
      message: 'Rule "Renamed Rule" deleted.'
    });
  });

  it("emits exact rule.started and rule.stopped payloads for a real forwarder", async () => {
    const activity = new ActivityStore();
    const manager = new ForwardManager(new MemoryStore(), activity);
    cleanup.push(() => manager.stopAll());

    const listenPort = await getFreeTcpPort();
    await manager.addRule({
      id: "p2",
      name: "Live Rule",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: 49999,
      enabled: false
    });
    await startRuleStable(manager, "p2", getFreeTcpPort);
    expect(lastOfType(activity, "rule.started")).toMatchObject({
      type: "rule.started",
      severity: "success",
      ruleId: "p2",
      ruleName: "Live Rule",
      protocol: "tcp",
      message: 'Rule "Live Rule" started.'
    });

    await manager.stopRule("p2");
    expect(lastOfType(activity, "rule.stopped")).toMatchObject({
      type: "rule.stopped",
      severity: "info",
      ruleId: "p2",
      ruleName: "Live Rule",
      protocol: "tcp",
      message: 'Rule "Live Rule" stopped.'
    });
  });

  it("emits an exact rule.error payload (severity error) when a start fails", async () => {
    const occupiedPort = await getFreeTcpPort();
    const occupier = net.createServer();
    await new Promise<void>((resolve) => occupier.listen(occupiedPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => occupier.close(() => resolve())));

    const activity = new ActivityStore();
    const manager = new ForwardManager(new MemoryStore(), activity);
    await manager.addRule({
      id: "p3",
      name: "Doomed Rule",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: occupiedPort,
      targetHost: "127.0.0.1",
      targetPort: 49999,
      enabled: false
    });

    await expect(manager.startRule("p3")).rejects.toThrow();
    const errorEvent = lastOfType(activity, "rule.error");
    expect(errorEvent).toMatchObject({
      type: "rule.error",
      severity: "error",
      ruleId: "p3",
      ruleName: "Doomed Rule",
      protocol: "tcp"
    });
    // Message is "Rule "<name>" failed to start: <os error>" — assert the stable prefix.
    expect(errorEvent?.message).toMatch(/^Rule "Doomed Rule" failed to start: /);
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

describe("ForwardManager importConfig duplicate listen binding parity (Go parity)", () => {
  // The Go service rejects duplicate listen bindings within the imported set
  // for BOTH replace and merge modes (manager.ImportConfig →
  // ensureNoDuplicateBindings). These tests pin the TypeScript runtime to the
  // same behavior so the two runtimes cannot drift. A listen binding is
  // protocol + listenHost + listenPort; targetHost/targetPort/name/enabled do
  // not matter.

  function duplicateBindingConfig(): ExportedConfig {
    // Two rules with distinct ids and names, different targets, but the SAME
    // protocol/listenHost/listenPort — an invalid set neither runtime accepts.
    return {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [
        {
          id: "alpha",
          name: "Alpha",
          protocol: "tcp",
          listenHost: "127.0.0.1",
          listenPort: 48020,
          targetHost: "127.0.0.1",
          targetPort: 49001,
          enabled: false
        },
        {
          id: "beta",
          name: "Beta",
          protocol: "tcp",
          listenHost: "127.0.0.1",
          listenPort: 48020,
          targetHost: "10.0.0.5",
          targetPort: 49002,
          enabled: false
        }
      ]
    };
  }

  it("rejects replace import with duplicate listen bindings without mutating or persisting", async () => {
    const store = new ControllableStore();
    const activity = new ActivityStore();
    const manager = new ForwardManager(store, activity);

    // A known-good existing config that must survive the rejected import.
    await manager.addRule({
      id: "existing",
      name: "Existing",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48010,
      targetHost: "127.0.0.1",
      targetPort: 49000,
      enabled: false
    });
    const savesAfterSetup = store.saveCallCount;

    const result = await manager.importConfig(duplicateBindingConfig(), "replace");

    // Rejected with a binding error naming the colliding endpoint and both rules.
    expect(result.imported).toBe(0);
    expect(result.errors).toHaveLength(1);
    expect(result.errors[0]).toBe(
      'a tcp rule is already listening on 127.0.0.1:48020 (rules "Alpha" and "Beta")'
    );

    // No mutation: the original config is intact, the imported ids are absent.
    expect(manager.listRules().map((r) => r.id)).toEqual(["existing"]);
    expect(manager.getRule("alpha")).toBeUndefined();
    expect(manager.getRule("beta")).toBeUndefined();

    // No persist and no forwarders started/stopped.
    expect(store.saveCallCount).toBe(savesAfterSetup);

    // The failure is surfaced as config.import.failed (error), not a misleading
    // config.imported success — with the exact intended payload (no details).
    const events = activity.list({});
    expect(events.some((e) => e.type === "config.imported")).toBe(false);
    const failures = events.filter((e) => e.type === "config.import.failed");
    expect(failures).toHaveLength(1);
    expect(failures[0]).toMatchObject({
      type: "config.import.failed",
      severity: "error",
      message:
        'Config import rejected: a tcp rule is already listening on 127.0.0.1:48020 (rules "Alpha" and "Beta")'
    });
    expect(failures[0].details).toBeUndefined();
  });

  it("rejects merge import with duplicate listen bindings within the imported set", async () => {
    const store = new ControllableStore();
    const manager = new ForwardManager(store);
    const savesBefore = store.saveCallCount;

    const result = await manager.importConfig(duplicateBindingConfig(), "merge");

    expect(result.imported).toBe(0);
    expect(result.errors).toHaveLength(1);
    expect(result.errors[0]).toBe(
      'a tcp rule is already listening on 127.0.0.1:48020 (rules "Alpha" and "Beta")'
    );
    // No rules added, no persist.
    expect(manager.listRules()).toHaveLength(0);
    expect(store.saveCallCount).toBe(savesBefore);
  });

  it("accepts replace import with distinct listen bindings", async () => {
    const store = new ControllableStore();
    const manager = new ForwardManager(store);

    const config: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [
        {
          id: "alpha",
          name: "Alpha",
          protocol: "tcp",
          listenHost: "127.0.0.1",
          listenPort: 48030,
          targetHost: "127.0.0.1",
          targetPort: 49001,
          enabled: false
        },
        {
          id: "beta",
          name: "Beta",
          protocol: "tcp",
          listenHost: "127.0.0.1",
          listenPort: 48031,
          targetHost: "127.0.0.1",
          targetPort: 49002,
          enabled: false
        }
      ]
    };

    const result = await manager.importConfig(config, "replace");
    expect(result.errors).toHaveLength(0);
    expect(result.imported).toBe(2);
    expect(manager.listRules().map((r) => r.id)).toEqual(["alpha", "beta"]);
    expect(store.lastPersisted()?.map((r) => r.id)).toEqual(["alpha", "beta"]);
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
    const [listenPort, targetPortB] = await Promise.all([getFreeTcpPort(), getFreeTcpPort()]);
    const { server: targetA, port: targetPortA } = await startTcpServerOnFreePort();
    cleanup.push(() => closeTcpServer(targetA));

    const store = new ControllableStore();
    const manager = new ForwardManager(store);
    cleanup.push(() => manager.stopAll());

    await manager.addRule(ruleInput({ id: "r1", listenPort, targetPort: targetPortA }));
    await startRuleStable(manager, "r1", getFreeTcpPort);
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
    const listenPort = await getFreeTcpPort();
    const { server: target, port: targetPort } = await startTcpServerOnFreePort();
    cleanup.push(() => closeTcpServer(target));

    const store = new ControllableStore();
    const manager = new ForwardManager(store);
    cleanup.push(() => manager.stopAll());

    await manager.addRule(ruleInput({ id: "r1", listenPort, targetPort }));
    await startRuleStable(manager, "r1", getFreeTcpPort);
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

describe("ForwardManager rule group metadata (v1.8 Slice 1)", () => {
  function groupRuleInput(overrides: Partial<ForwardRule> & { id: string }): ForwardRule {
    return {
      name: "Grouped",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48201,
      targetHost: "127.0.0.1",
      targetPort: 49999,
      enabled: false,
      ...overrides
    } as ForwardRule;
  }

  it("stores a group on create and returns it", async () => {
    const manager = new ForwardManager(new MemoryStore());
    const rule = await manager.addRule(groupRuleInput({ id: "g1", group: "  web-team  " }));
    expect(rule.group).toBe("web-team");
    expect(manager.getRule("g1")?.group).toBe("web-team");
  });

  it("omits group when none is provided", async () => {
    const manager = new ForwardManager(new MemoryStore());
    const rule = await manager.addRule(groupRuleInput({ id: "g2" }));
    expect("group" in rule).toBe(false);
  });

  it("updates a group via patch without restarting (metadata only)", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule(groupRuleInput({ id: "g3", group: "web-team" }));
    const updated = await manager.updateRule("g3", { group: "api-team" });
    expect(updated.group).toBe("api-team");
  });

  it("clears a group when patched with an empty string", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule(groupRuleInput({ id: "g4", group: "web-team" }));
    const updated = await manager.updateRule("g4", { group: "" });
    expect("group" in updated).toBe(false);
  });

  it("leaves the group untouched when the patch omits it", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule(groupRuleInput({ id: "g5", group: "web-team" }));
    const updated = await manager.updateRule("g5", { name: "Renamed" });
    expect(updated.group).toBe("web-team");
    expect(updated.name).toBe("Renamed");
  });

  it("rejects an invalid group on create", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await expect(manager.addRule(groupRuleInput({ id: "g6", group: "x".repeat(65) }))).rejects.toThrow(ValidationError);
  });

  it("round-trips a group through export and import", async () => {
    const manager = new ForwardManager(new MemoryStore());
    await manager.addRule(groupRuleInput({ id: "g7", group: "payments" }));
    const exported = manager.exportConfig();
    expect(exported.rules[0].group).toBe("payments");

    const fresh = new ForwardManager(new MemoryStore());
    await fresh.importConfig(exported, "replace");
    expect(fresh.getRule("g7")?.group).toBe("payments");
  });
});

describe("ForwardManager group operations (v1.8 Slice 4)", () => {
  const cleanup: Array<() => Promise<void>> = [];

  afterEach(async () => {
    await Promise.all(cleanup.splice(0).map((fn) => fn()));
  });

  async function addRule(
    manager: ForwardManager,
    overrides: Partial<ForwardRule> & { id: string; group?: string }
  ): Promise<ForwardRule> {
    const listenPort = await getFreeTcpPort();
    return manager.addRule({
      name: overrides.id,
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: 49999,
      enabled: false,
      ...overrides
    } as ForwardRule);
  }

  it("starts all stopped rules in a group, in rule order, leaving other groups untouched", async () => {
    const manager = new ForwardManager(new MemoryStore());
    cleanup.push(() => manager.stopAll());
    await addRule(manager, { id: "w1", group: "web" });
    await addRule(manager, { id: "a1", group: "api" });
    await addRule(manager, { id: "w2", group: "web" });
    await addRule(manager, { id: "u1" }); // ungrouped

    const results = await manager.startGroup("web");

    expect(results.map((r) => r.ruleId)).toEqual(["w1", "w2"]); // rule order, only "web"
    expect(results.every((r) => r.status === "started")).toBe(true);
    expect(manager.getStatus("w1").running).toBe(true);
    expect(manager.getStatus("w2").running).toBe(true);
    expect(manager.getStatus("a1").running).toBe(false); // other group untouched
    expect(manager.getStatus("u1").running).toBe(false); // ungrouped untouched
  });

  it("skips already-running rules on start", async () => {
    const manager = new ForwardManager(new MemoryStore());
    cleanup.push(() => manager.stopAll());
    await addRule(manager, { id: "w1", group: "web" });
    await startRuleStable(manager, "w1", getFreeTcpPort);

    const results = await manager.startGroup("web");
    expect(results).toEqual([
      { ruleId: "w1", ruleName: "w1", status: "skipped", reason: "already_running" }
    ]);
  });

  it("stops running rules and skips stopped ones", async () => {
    const manager = new ForwardManager(new MemoryStore());
    cleanup.push(() => manager.stopAll());
    await addRule(manager, { id: "w1", group: "web" });
    await addRule(manager, { id: "w2", group: "web" });
    await startRuleStable(manager, "w1", getFreeTcpPort); // only w1 running

    const results = await manager.stopGroup("web");
    expect(results).toEqual([
      { ruleId: "w1", ruleName: "w1", status: "stopped" },
      { ruleId: "w2", ruleName: "w2", status: "skipped", reason: "not_running" }
    ]);
    expect(manager.getStatus("w1").running).toBe(false);
  });

  it("returns an empty result list when no rule matches the group", async () => {
    const manager = new ForwardManager(new MemoryStore());
    cleanup.push(() => manager.stopAll());
    await addRule(manager, { id: "w1", group: "web" });

    expect(await manager.startGroup("ghost")).toEqual([]);
    expect(await manager.stopGroup("ghost")).toEqual([]);
  });

  it("reports a failed start while still starting the rest of the group (partial)", async () => {
    const manager = new ForwardManager(new MemoryStore());
    cleanup.push(() => manager.stopAll());

    // Occupy a port so a rule bound to it fails to start.
    const { server: blocker, port: occupied } = await startTcpServerOnFreePort();
    cleanup.push(() => closeTcpServer(blocker));

    await manager.addRule({
      id: "bad", name: "bad", protocol: "tcp", listenHost: "127.0.0.1",
      listenPort: occupied, targetHost: "127.0.0.1", targetPort: 49999, enabled: false, group: "web"
    } as ForwardRule);
    await addRule(manager, { id: "good", group: "web" });

    const results = await manager.startGroup("web");
    expect(results[0].ruleId).toBe("bad");
    expect(results[0].status).toBe("failed");
    expect(results[0].reason).toBeTruthy();
    expect(results[1]).toEqual({ ruleId: "good", ruleName: "good", status: "started" });
    expect(manager.getStatus("good").running).toBe(true);
  });

  it("does not mutate rule definitions, order, or metadata", async () => {
    const manager = new ForwardManager(new MemoryStore());
    cleanup.push(() => manager.stopAll());
    await addRule(manager, { id: "w1", group: "web" });
    await addRule(manager, { id: "a1", group: "api" });
    const before = JSON.stringify(manager.listRules());

    await manager.startGroup("web");
    await manager.stopGroup("web");

    expect(JSON.stringify(manager.listRules())).toBe(before);
  });
});

describe("ForwardManager rule health (v1.8 Slice 7)", () => {
  const cleanup: Array<() => Promise<void>> = [];

  afterEach(async () => {
    await Promise.all(cleanup.splice(0).map((fn) => fn()));
  });

  async function addRule(
    manager: ForwardManager,
    overrides: Partial<ForwardRule> & { id: string }
  ): Promise<ForwardRule> {
    const listenPort = await getFreeTcpPort();
    return manager.addRule({
      name: overrides.id,
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: 49999,
      enabled: false,
      ...overrides
    } as ForwardRule);
  }

  it("reports healthy for an intentionally stopped (disabled) rule", async () => {
    const manager = new ForwardManager(new MemoryStore());
    cleanup.push(() => manager.stopAll());
    await addRule(manager, { id: "h1", enabled: false });
    expect(manager.getStatus("h1").health).toBe("healthy");
  });

  it("reports healthy for a running rule", async () => {
    const manager = new ForwardManager(new MemoryStore());
    cleanup.push(() => manager.stopAll());
    await addRule(manager, { id: "h2", enabled: false });
    await startRuleStable(manager, "h2", getFreeTcpPort);
    expect(manager.getStatus("h2").running).toBe(true);
    expect(manager.getStatus("h2").health).toBe("healthy");
  });

  it("reports warning for an enabled rule that is not running", async () => {
    const manager = new ForwardManager(new MemoryStore());
    cleanup.push(() => manager.stopAll());
    // Created enabled → auto-started; stop it so it is enabled but not running.
    const rule = await addRule(manager, { id: "h3", enabled: true });
    expect(manager.getStatus(rule.id).running).toBe(true);
    await manager.stopRule(rule.id);
    expect(manager.getStatus(rule.id).health).toBe("warning");
  });
});

describe("ForwardManager coverage hardening (Slice 30)", () => {
  const managers: ForwardManager[] = [];

  afterEach(async () => {
    await Promise.all(managers.splice(0).map((m) => m.stopAll()));
  });

  function track(m: ForwardManager): ForwardManager {
    managers.push(m);
    return m;
  }

  it("errorMessage returns an Error's message and the string form of anything else", () => {
    expect(errorMessage(new Error("boom"))).toBe("boom");
    expect(errorMessage("plain")).toBe("plain");
    expect(errorMessage(42)).toBe("42");
  });

  it("getStatus reports zeroed counters for stopped TCP/UDP rules and sessions for multi-client", async () => {
    const m = track(new ForwardManager(new MemoryStore()));
    const tcp = await m.addRule({
      name: "t", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48710,
      targetHost: "127.0.0.1", targetPort: 9100, enabled: false
    });
    const oneWay = await m.addRule({
      name: "u1", protocol: "udp", listenHost: "127.0.0.1", listenPort: 48711,
      targetHost: "127.0.0.1", targetPort: 9101, enabled: false, udpMode: "one-way"
    });
    const multi = await m.addRule({
      name: "u2", protocol: "udp", listenHost: "127.0.0.1", listenPort: 48712,
      targetHost: "127.0.0.1", targetPort: 9102, enabled: false, udpMode: "bidirectional-multi-client"
    });

    const tcpStatus = m.getStatus(tcp.id);
    expect(tcpStatus.running).toBe(false);
    expect(tcpStatus.activeConnections).toBe(0);
    expect(tcpStatus.packetsIn).toBeUndefined();

    const oneWayStatus = m.getStatus(oneWay.id);
    expect(oneWayStatus.packetsIn).toBe(0);
    expect(oneWayStatus.packetsOut).toBe(0);
    expect(oneWayStatus.activeUdpSessions).toBeUndefined();

    expect(m.getStatus(multi.id).activeUdpSessions).toBe(0);
  });

  it("loadAndStartEnabled starts an enabled rule loaded from the store", async () => {
    const listenPort = await getFreeTcpPort();
    const rule: ForwardRule = {
      id: "load-enabled", name: "Load Enabled", protocol: "tcp", listenHost: "127.0.0.1",
      listenPort, targetHost: "127.0.0.1", targetPort: 49998, enabled: true
    };
    const m = track(new ForwardManager(new MemoryStore([rule])));
    const started = await m.loadAndStartEnabled();
    expect(started).toBe(1);
    expect(m.getStatus("load-enabled").running).toBe(true);
  });

  it("startRule is idempotent: a second start returns the running status without restarting", async () => {
    const listenPort = await getFreeTcpPort();
    const m = track(new ForwardManager(new MemoryStore()));
    const rule = await m.addRule({
      name: "idem", protocol: "tcp", listenHost: "127.0.0.1", listenPort,
      targetHost: "127.0.0.1", targetPort: 49997, enabled: false
    });
    await startRuleStable(m, rule.id, getFreeTcpPort);
    const first = m.getStatus(rule.id);
    const again = await m.startRule(rule.id);
    expect(again.running).toBe(true);
    expect(again.startedAt).toBe(first.startedAt);
  });

  it("operates without an ActivityStore across the full rule lifecycle", async () => {
    const listenPort = await getFreeTcpPort();
    const m = track(new ForwardManager(new MemoryStore())); // no ActivityStore
    const rule = await m.addRule({
      name: "no-activity", protocol: "tcp", listenHost: "127.0.0.1", listenPort,
      targetHost: "127.0.0.1", targetPort: 49996, enabled: false
    });
    await startRuleStable(m, rule.id, getFreeTcpPort);
    await m.updateRule(rule.id, { name: "renamed" });
    await m.stopRule(rule.id);

    const exported = m.exportConfig();
    expect(exported.rules.length).toBe(1);
    const importResult = await m.importConfig(exported, "replace");
    expect(importResult.errors).toEqual([]);

    await m.deleteRule(m.listRules()[0].id);
    expect(m.listRules()).toEqual([]);
  });

  it("importConfig replace generates an id for a valid rule that has none", async () => {
    const m = track(new ForwardManager(new MemoryStore()));
    const config: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [{
        name: "no-id", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48720,
        targetHost: "127.0.0.1", targetPort: 9300, enabled: false
      }] as unknown as ForwardRule[]
    };
    const result = await m.importConfig(config, "replace");
    expect(result.imported).toBe(1);
    const [imported] = m.listRules();
    expect(typeof imported.id).toBe("string");
    expect(imported.id.length).toBeGreaterThan(0);
  });

  it("importConfig rejects an invalid rule and reports the error without mutating", async () => {
    const m = track(new ForwardManager(new MemoryStore()));
    const config: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [{
        name: "bad-port", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 70000,
        targetHost: "127.0.0.1", targetPort: 9301, enabled: false
      }] as unknown as ForwardRule[]
    };
    const result = await m.importConfig(config, "replace");
    expect(result.imported).toBe(0);
    expect(result.errors.length).toBeGreaterThan(0);
    expect(m.listRules()).toEqual([]);
  });

  it("importConfig merge regenerates the id when an imported rule clashes with an existing id", async () => {
    const m = track(new ForwardManager(new MemoryStore()));
    const existing = await m.addRule({
      id: "shared-id", name: "existing", protocol: "tcp", listenHost: "127.0.0.1",
      listenPort: 48730, targetHost: "127.0.0.1", targetPort: 9400, enabled: false
    });
    const config: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [{
        id: "shared-id", name: "incoming", protocol: "tcp", listenHost: "127.0.0.1",
        listenPort: 48731, targetHost: "127.0.0.1", targetPort: 9401, enabled: false
      }] as unknown as ForwardRule[]
    };
    const result = await m.importConfig(config, "merge");
    expect(result.imported).toBe(1);
    const ids = m.listRules().map((r) => r.id);
    expect(ids).toContain(existing.id);
    expect(ids.length).toBe(2);
    // The incoming rule's clashing id was regenerated to a fresh one.
    expect(ids.filter((id) => id === "shared-id").length).toBe(1);
  });
});

describe("ForwardManager startRule for UDP (Slice 30)", () => {
  const managers: ForwardManager[] = [];
  afterEach(async () => {
    await Promise.all(managers.splice(0).map((m) => m.stopAll()));
  });

  it("starts a UDP rule through the manager", async () => {
    const listenPort = await getFreeTcpPort();
    const manager = new ForwardManager(new MemoryStore());
    managers.push(manager);
    const rule = await manager.addRule({
      name: "udp-start", protocol: "udp", listenHost: "127.0.0.1", listenPort,
      targetHost: "127.0.0.1", targetPort: 49995, enabled: false, udpMode: "one-way"
    });
    await startRuleStable(manager, rule.id, getFreeTcpPort);
    expect(manager.getStatus(rule.id).running).toBe(true);
  });
});

describe("ForwardManager config-level activity emission (Slice 30)", () => {
  const managers: ForwardManager[] = [];
  afterEach(async () => {
    await Promise.all(managers.splice(0).map((m) => m.stopAll()));
  });

  // Exercises every config-level activity call site: config.exported,
  // config.import.failed (validation), config.import.failed (duplicate binding),
  // config.imported (success), and config.import.failed (merge conflict).
  async function exerciseConfigPaths(manager: ForwardManager): Promise<void> {
    manager.exportConfig();

    const invalid: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [{ name: "bad", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 70000, targetHost: "127.0.0.1", targetPort: 9500, enabled: false }] as unknown as ForwardRule[]
    };
    expect((await manager.importConfig(invalid, "replace")).errors.length).toBeGreaterThan(0);

    const dup: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [
        { id: "d1", name: "d1", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48740, targetHost: "127.0.0.1", targetPort: 9501, enabled: false },
        { id: "d2", name: "d2", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48740, targetHost: "127.0.0.1", targetPort: 9502, enabled: false }
      ] as unknown as ForwardRule[]
    };
    expect((await manager.importConfig(dup, "replace")).errors.length).toBeGreaterThan(0);

    const valid: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [{ id: "v1", name: "v1", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48741, targetHost: "127.0.0.1", targetPort: 9503, enabled: false }] as unknown as ForwardRule[]
    };
    expect((await manager.importConfig(valid, "replace")).imported).toBe(1);

    // Merge a rule whose listen binding clashes with the just-imported v1.
    const conflict: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [{ id: "c1", name: "c1", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48741, targetHost: "127.0.0.1", targetPort: 9504, enabled: false }] as unknown as ForwardRule[]
    };
    expect((await manager.importConfig(conflict, "merge")).errors.length).toBeGreaterThan(0);
  }

  it("emits config-level activity events when an ActivityStore is present", async () => {
    const activity = new ActivityStore();
    const manager = new ForwardManager(new MemoryStore(), activity);
    managers.push(manager);
    await exerciseConfigPaths(manager);
    const types = activity.list({}).map((e) => e.type);
    expect(types).toContain("config.exported");
    expect(types).toContain("config.import.failed");
    expect(types).toContain("config.imported");
  });

  it("operates without emitting when no ActivityStore is present", async () => {
    const manager = new ForwardManager(new MemoryStore());
    managers.push(manager);
    await exerciseConfigPaths(manager);
    expect(manager.listRules().length).toBe(1);
  });
});

describe("ForwardManager additional branch coverage (Slice 30)", () => {
  const managers: ForwardManager[] = [];
  afterEach(async () => {
    await Promise.all(managers.splice(0).map((m) => m.stopAll()));
  });

  it("importConfig labels a nameless invalid rule with a placeholder", async () => {
    const manager = new ForwardManager(new MemoryStore());
    managers.push(manager);
    const config: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: [{ protocol: "tcp", listenHost: "127.0.0.1", listenPort: 70000, targetHost: "127.0.0.1", targetPort: 9700, enabled: false }] as unknown as ForwardRule[]
    };
    const result = await manager.importConfig(config, "replace");
    expect(result.imported).toBe(0);
    expect(result.errors[0]).toContain('Rule "?"');
  });

  it("reorderRules appends rules omitted from the id list, preserving their prior order", async () => {
    const manager = new ForwardManager(new MemoryStore());
    managers.push(manager);
    await manager.addRule({ id: "a", name: "a", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48750, targetHost: "127.0.0.1", targetPort: 9600, enabled: false });
    await manager.addRule({ id: "b", name: "b", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48751, targetHost: "127.0.0.1", targetPort: 9601, enabled: false });
    await manager.addRule({ id: "c", name: "c", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48752, targetHost: "127.0.0.1", targetPort: 9602, enabled: false });

    await manager.reorderRules(["c"]); // only c is listed; a and b are appended in prior order

    expect(manager.listRules().map((r) => r.id)).toEqual(["c", "a", "b"]);
  });
});
