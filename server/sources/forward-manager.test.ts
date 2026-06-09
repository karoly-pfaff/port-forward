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
