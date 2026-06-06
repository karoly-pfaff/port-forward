import net from "node:net";
import { afterEach, describe, expect, it } from "vitest";
import type { ForwardRule } from "@portier/shared";
import { ForwardManager, ConflictError, type RuleStore } from "./forward-manager.js";
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
