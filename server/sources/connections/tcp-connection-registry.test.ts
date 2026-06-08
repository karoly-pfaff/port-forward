import { describe, expect, it } from "vitest";
import { TcpConnectionRegistry } from "./tcp-connection-registry.js";

const baseInput = {
  ruleId: "rule-1",
  ruleName: "My Rule",
  clientAddress: "127.0.0.1",
  clientPort: 54321,
  targetAddress: "192.168.1.10",
  targetPort: 8080
};

describe("TcpConnectionRegistry – empty state", () => {
  it("snapshot returns empty array when no connections are open", () => {
    const reg = new TcpConnectionRegistry();
    expect(reg.snapshot()).toEqual([]);
  });

  it("snapshotForRule returns empty array when no connections are open", () => {
    const reg = new TcpConnectionRegistry();
    expect(reg.snapshotForRule("rule-1")).toEqual([]);
  });
});

describe("TcpConnectionRegistry – openConnection", () => {
  it("returns a non-empty string ID", () => {
    const reg = new TcpConnectionRegistry();
    const id = reg.openConnection(baseInput);
    expect(typeof id).toBe("string");
    expect(id.length).toBeGreaterThan(0);
  });

  it("each call returns a unique ID", () => {
    const reg = new TcpConnectionRegistry();
    const id1 = reg.openConnection(baseInput);
    const id2 = reg.openConnection(baseInput);
    expect(id1).not.toBe(id2);
  });

  it("opened connection appears in snapshot", () => {
    const reg = new TcpConnectionRegistry();
    reg.openConnection(baseInput);
    const snap = reg.snapshot();
    expect(snap).toHaveLength(1);
    expect(snap[0].ruleId).toBe("rule-1");
    expect(snap[0].ruleName).toBe("My Rule");
    expect(snap[0].protocol).toBe("tcp");
    expect(snap[0].clientAddress).toBe("127.0.0.1");
    expect(snap[0].clientPort).toBe(54321);
    expect(snap[0].targetAddress).toBe("192.168.1.10");
    expect(snap[0].targetPort).toBe(8080);
    expect(snap[0].status).toBe("active");
    expect(snap[0].bytesIn).toBe(0);
    expect(snap[0].bytesOut).toBe(0);
  });

  it("snapshot ID matches the ID returned by openConnection", () => {
    const reg = new TcpConnectionRegistry();
    const id = reg.openConnection(baseInput);
    const snap = reg.snapshot();
    expect(snap[0].id).toBe(id);
  });

  it("startedAt is a valid ISO timestamp string", () => {
    const before = new Date().toISOString();
    const reg = new TcpConnectionRegistry();
    reg.openConnection(baseInput);
    const after = new Date().toISOString();
    const snap = reg.snapshot();
    expect(snap[0].startedAt >= before).toBe(true);
    expect(snap[0].startedAt <= after).toBe(true);
  });

  it("ruleName can be empty string", () => {
    const reg = new TcpConnectionRegistry();
    reg.openConnection({ ...baseInput, ruleName: "" });
    const snap = reg.snapshot();
    expect(snap[0].ruleName).toBe("");
  });

  it("clientAddress and clientPort reflect fallback values (unknown/0)", () => {
    const reg = new TcpConnectionRegistry();
    reg.openConnection({ ...baseInput, clientAddress: "unknown", clientPort: 0 });
    const snap = reg.snapshot();
    expect(snap[0].clientAddress).toBe("unknown");
    expect(snap[0].clientPort).toBe(0);
  });
});

describe("TcpConnectionRegistry – durationMs", () => {
  it("durationMs is non-negative", () => {
    const reg = new TcpConnectionRegistry();
    reg.openConnection(baseInput);
    const snap = reg.snapshot();
    expect(snap[0].durationMs).toBeGreaterThanOrEqual(0);
  });

  it("durationMs increases with a later snapshot time", () => {
    const reg = new TcpConnectionRegistry();
    reg.openConnection(baseInput);
    const t0 = new Date();
    const snap1 = reg.snapshot(new Date(t0.getTime() + 100));
    const snap2 = reg.snapshot(new Date(t0.getTime() + 200));
    expect(snap2[0].durationMs).toBeGreaterThan(snap1[0].durationMs);
  });

  it("durationMs reflects the explicit now time passed to snapshot", () => {
    const reg = new TcpConnectionRegistry();
    reg.openConnection(baseInput);
    // Use a snapshot time well in the future
    const later = new Date(Date.now() + 5000);
    const snap = reg.snapshot(later);
    expect(snap[0].durationMs).toBeGreaterThanOrEqual(4900);
  });
});

describe("TcpConnectionRegistry – bytes", () => {
  it("addBytesIn accumulates bytesIn", () => {
    const reg = new TcpConnectionRegistry();
    const id = reg.openConnection(baseInput);
    reg.addBytesIn(id, 100);
    reg.addBytesIn(id, 50);
    expect(reg.snapshot()[0].bytesIn).toBe(150);
  });

  it("addBytesOut accumulates bytesOut", () => {
    const reg = new TcpConnectionRegistry();
    const id = reg.openConnection(baseInput);
    reg.addBytesOut(id, 200);
    reg.addBytesOut(id, 75);
    expect(reg.snapshot()[0].bytesOut).toBe(275);
  });

  it("addBytesIn on unknown ID is a no-op", () => {
    const reg = new TcpConnectionRegistry();
    expect(() => reg.addBytesIn("no-such-id", 100)).not.toThrow();
  });

  it("addBytesOut on unknown ID is a no-op", () => {
    const reg = new TcpConnectionRegistry();
    expect(() => reg.addBytesOut("no-such-id", 100)).not.toThrow();
  });

  it("bytesIn and bytesOut are independent per connection", () => {
    const reg = new TcpConnectionRegistry();
    const id = reg.openConnection(baseInput);
    reg.addBytesIn(id, 10);
    reg.addBytesOut(id, 20);
    const snap = reg.snapshot()[0];
    expect(snap.bytesIn).toBe(10);
    expect(snap.bytesOut).toBe(20);
  });
});

describe("TcpConnectionRegistry – closeConnection", () => {
  it("removesambers the entry from snapshot", () => {
    const reg = new TcpConnectionRegistry();
    const id = reg.openConnection(baseInput);
    reg.closeConnection(id);
    expect(reg.snapshot()).toHaveLength(0);
  });

  it("closeConnection on unknown ID is a no-op", () => {
    const reg = new TcpConnectionRegistry();
    expect(() => reg.closeConnection("no-such-id")).not.toThrow();
  });

  it("only closes the targeted connection, leaving others open", () => {
    const reg = new TcpConnectionRegistry();
    const id1 = reg.openConnection(baseInput);
    const id2 = reg.openConnection(baseInput);
    reg.closeConnection(id1);
    const snap = reg.snapshot();
    expect(snap).toHaveLength(1);
    expect(snap[0].id).toBe(id2);
  });
});

describe("TcpConnectionRegistry – closeConnectionsForRule", () => {
  it("removes all connections for the given rule", () => {
    const reg = new TcpConnectionRegistry();
    reg.openConnection(baseInput);
    reg.openConnection(baseInput);
    reg.closeConnectionsForRule("rule-1");
    expect(reg.snapshot()).toHaveLength(0);
  });

  it("leaves connections for other rules untouched", () => {
    const reg = new TcpConnectionRegistry();
    reg.openConnection(baseInput);
    reg.openConnection({ ...baseInput, ruleId: "rule-2" });
    reg.closeConnectionsForRule("rule-1");
    const snap = reg.snapshot();
    expect(snap).toHaveLength(1);
    expect(snap[0].ruleId).toBe("rule-2");
  });

  it("is a no-op when no connections exist for the rule", () => {
    const reg = new TcpConnectionRegistry();
    expect(() => reg.closeConnectionsForRule("rule-99")).not.toThrow();
    expect(reg.snapshot()).toHaveLength(0);
  });
});

describe("TcpConnectionRegistry – multiple rules and connections", () => {
  it("tracks connections for multiple rules independently", () => {
    const reg = new TcpConnectionRegistry();
    reg.openConnection({ ...baseInput, ruleId: "rule-1", ruleName: "Rule One" });
    reg.openConnection({ ...baseInput, ruleId: "rule-2", ruleName: "Rule Two" });
    expect(reg.snapshot()).toHaveLength(2);
    expect(reg.snapshotForRule("rule-1")).toHaveLength(1);
    expect(reg.snapshotForRule("rule-2")).toHaveLength(1);
  });

  it("tracks multiple connections under the same rule", () => {
    const reg = new TcpConnectionRegistry();
    reg.openConnection(baseInput);
    reg.openConnection(baseInput);
    reg.openConnection(baseInput);
    expect(reg.snapshot()).toHaveLength(3);
    expect(reg.snapshotForRule("rule-1")).toHaveLength(3);
  });

  it("snapshotForRule excludes other rules", () => {
    const reg = new TcpConnectionRegistry();
    reg.openConnection({ ...baseInput, ruleId: "rule-1" });
    reg.openConnection({ ...baseInput, ruleId: "rule-2" });
    const snap = reg.snapshotForRule("rule-1");
    expect(snap).toHaveLength(1);
    expect(snap[0].ruleId).toBe("rule-1");
  });
});

describe("TcpConnectionRegistry – snapshot immutability", () => {
  it("mutating a snapshot entry does not affect registry state", () => {
    const reg = new TcpConnectionRegistry();
    const id = reg.openConnection(baseInput);
    reg.addBytesIn(id, 42);
    const snap = reg.snapshot();
    (snap[0] as unknown as Record<string, unknown>).bytesIn = 9999;
    expect(reg.snapshot()[0].bytesIn).toBe(42);
  });

  it("snapshot includes no payload fields beyond the defined TcpConnectionInfo shape", () => {
    const reg = new TcpConnectionRegistry();
    reg.openConnection(baseInput);
    const snap = reg.snapshot();
    const keys = Object.keys(snap[0]).sort();
    const expected = [
      "bytesIn",
      "bytesOut",
      "clientAddress",
      "clientPort",
      "durationMs",
      "id",
      "protocol",
      "ruleId",
      "ruleName",
      "startedAt",
      "status",
      "targetAddress",
      "targetPort"
    ];
    expect(keys).toEqual(expected);
  });
});
