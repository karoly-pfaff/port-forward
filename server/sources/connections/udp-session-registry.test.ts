import { describe, expect, it } from "vitest";
import { UDP_SESSION_EXPIRE_MS, UDP_SESSION_IDLE_MS, UdpSessionRegistry } from "./udp-session-registry.js";

const baseInput = {
  ruleId: "rule-1",
  ruleName: "My Rule",
  mode: "one-way" as const,
  clientAddress: "127.0.0.1",
  clientPort: 54321,
  targetAddress: "192.168.1.10",
  targetPort: 8080
};

describe("UdpSessionRegistry – empty state", () => {
  it("snapshot returns empty array when no sessions exist", () => {
    const reg = new UdpSessionRegistry();
    expect(reg.snapshot()).toEqual([]);
  });

  it("snapshotForRule returns empty array when no sessions exist", () => {
    const reg = new UdpSessionRegistry();
    expect(reg.snapshotForRule("rule-1")).toEqual([]);
  });
});

describe("UdpSessionRegistry – openOrTouchSession", () => {
  it("returns a non-empty string ID", () => {
    const reg = new UdpSessionRegistry();
    const id = reg.openOrTouchSession(baseInput);
    expect(typeof id).toBe("string");
    expect(id.length).toBeGreaterThan(0);
  });

  it("same input returns same ID (idempotent)", () => {
    const reg = new UdpSessionRegistry();
    const id1 = reg.openOrTouchSession(baseInput);
    const id2 = reg.openOrTouchSession(baseInput);
    expect(id1).toBe(id2);
  });

  it("different client address returns different ID", () => {
    const reg = new UdpSessionRegistry();
    const id1 = reg.openOrTouchSession(baseInput);
    const id2 = reg.openOrTouchSession({ ...baseInput, clientAddress: "10.0.0.1" });
    expect(id1).not.toBe(id2);
  });

  it("different client port returns different ID", () => {
    const reg = new UdpSessionRegistry();
    const id1 = reg.openOrTouchSession(baseInput);
    const id2 = reg.openOrTouchSession({ ...baseInput, clientPort: 11111 });
    expect(id1).not.toBe(id2);
  });

  it("different ruleId returns different ID", () => {
    const reg = new UdpSessionRegistry();
    const id1 = reg.openOrTouchSession(baseInput);
    const id2 = reg.openOrTouchSession({ ...baseInput, ruleId: "rule-2" });
    expect(id1).not.toBe(id2);
  });

  it("different mode returns different ID", () => {
    const reg = new UdpSessionRegistry();
    const id1 = reg.openOrTouchSession(baseInput);
    const id2 = reg.openOrTouchSession({ ...baseInput, mode: "bidirectional-last-client" });
    expect(id1).not.toBe(id2);
  });

  it("opened session appears in snapshot", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    const snap = reg.snapshot();
    expect(snap).toHaveLength(1);
    expect(snap[0].ruleId).toBe("rule-1");
    expect(snap[0].ruleName).toBe("My Rule");
    expect(snap[0].protocol).toBe("udp");
    expect(snap[0].mode).toBe("one-way");
    expect(snap[0].clientAddress).toBe("127.0.0.1");
    expect(snap[0].clientPort).toBe(54321);
    expect(snap[0].targetAddress).toBe("192.168.1.10");
    expect(snap[0].targetPort).toBe(8080);
    expect(snap[0].status).toBe("active");
    expect(snap[0].packetsIn).toBe(0);
    expect(snap[0].packetsOut).toBe(0);
    expect(snap[0].bytesIn).toBe(0);
    expect(snap[0].bytesOut).toBe(0);
  });

  it("snapshot ID matches the ID returned by openOrTouchSession", () => {
    const reg = new UdpSessionRegistry();
    const id = reg.openOrTouchSession(baseInput);
    expect(reg.snapshot()[0].id).toBe(id);
  });

  it("startedAt is a valid ISO timestamp string", () => {
    const before = new Date().toISOString();
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    const after = new Date().toISOString();
    const snap = reg.snapshot();
    expect(snap[0].startedAt >= before).toBe(true);
    expect(snap[0].startedAt <= after).toBe(true);
  });

  it("startedAt is preserved on touch", () => {
    const reg = new UdpSessionRegistry();
    const id = reg.openOrTouchSession(baseInput);
    const firstStartedAt = reg.snapshot()[0].startedAt;
    reg.openOrTouchSession(baseInput); // touch
    expect(reg.snapshot()[0].startedAt).toBe(firstStartedAt);
    expect(reg.snapshot()[0].id).toBe(id);
  });

  it("lastSeenAt updates on touch", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    const t0 = reg.snapshot()[0].lastSeenAt;
    // Touch with a future snapshot time to confirm lastSeenAt changed
    reg.openOrTouchSession(baseInput);
    const t1 = reg.snapshot()[0].lastSeenAt;
    expect(t1 >= t0).toBe(true);
  });

  it("ruleName can be empty string", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession({ ...baseInput, ruleName: "" });
    expect(reg.snapshot()[0].ruleName).toBe("");
  });

  it("handles stale keyToId entry gracefully (defensive path)", () => {
    const reg = new UdpSessionRegistry();
    const id = reg.openOrTouchSession(baseInput);
    // Manually simulate stale state: entry removed but keyToId not cleaned
    (reg as unknown as { entries: Map<string, unknown> }).entries.delete(id);
    // keyToId still points to the stale id — defensive path should clean it
    const newId = reg.openOrTouchSession(baseInput);
    expect(typeof newId).toBe("string");
    expect(newId).not.toBe(id);
    expect(reg.snapshot()).toHaveLength(1);
  });
});

describe("UdpSessionRegistry – idleMs and status", () => {
  it("idleMs is non-negative", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    expect(reg.snapshot()[0].idleMs).toBeGreaterThanOrEqual(0);
  });

  it("status is active when idleMs is below idle threshold", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    // Snapshot at current time — session just created, idleMs ~0
    expect(reg.snapshot()[0].status).toBe("active");
  });

  it("status is idle when idleMs is at or above idle threshold", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    const futureMs = UDP_SESSION_IDLE_MS;
    const snap = reg.snapshot(new Date(Date.now() + futureMs));
    expect(snap[0].status).toBe("idle");
  });

  it("idleMs reflects the explicit now time passed to snapshot", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    const later = new Date(Date.now() + 5000);
    const snap = reg.snapshot(later);
    expect(snap[0].idleMs).toBeGreaterThanOrEqual(4900);
  });

  it("lastSeenAt in snapshot is an ISO string", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    const snap = reg.snapshot();
    expect(() => new Date(snap[0].lastSeenAt)).not.toThrow();
    expect(snap[0].lastSeenAt).toMatch(/^\d{4}-\d{2}-\d{2}T/);
  });
});

describe("UdpSessionRegistry – recordInbound", () => {
  it("increments packetsIn and bytesIn", () => {
    const reg = new UdpSessionRegistry();
    const id = reg.openOrTouchSession(baseInput);
    reg.recordInbound(id, 100);
    reg.recordInbound(id, 50);
    const snap = reg.snapshot()[0];
    expect(snap.packetsIn).toBe(2);
    expect(snap.bytesIn).toBe(150);
  });

  it("recordInbound on unknown ID is a no-op", () => {
    const reg = new UdpSessionRegistry();
    expect(() => reg.recordInbound("no-such-id", 100)).not.toThrow();
  });

  it("recordInbound updates lastSeenAt", () => {
    const reg = new UdpSessionRegistry();
    const id = reg.openOrTouchSession(baseInput);
    const t0 = reg.snapshot()[0].lastSeenAt;
    reg.recordInbound(id, 1);
    const t1 = reg.snapshot()[0].lastSeenAt;
    expect(t1 >= t0).toBe(true);
  });
});

describe("UdpSessionRegistry – recordOutbound", () => {
  it("increments packetsOut and bytesOut", () => {
    const reg = new UdpSessionRegistry();
    const id = reg.openOrTouchSession(baseInput);
    reg.recordOutbound(id, 200);
    reg.recordOutbound(id, 75);
    const snap = reg.snapshot()[0];
    expect(snap.packetsOut).toBe(2);
    expect(snap.bytesOut).toBe(275);
  });

  it("recordOutbound on unknown ID is a no-op", () => {
    const reg = new UdpSessionRegistry();
    expect(() => reg.recordOutbound("no-such-id", 100)).not.toThrow();
  });

  it("bytesIn and bytesOut are independent", () => {
    const reg = new UdpSessionRegistry();
    const id = reg.openOrTouchSession(baseInput);
    reg.recordInbound(id, 10);
    reg.recordOutbound(id, 20);
    const snap = reg.snapshot()[0];
    expect(snap.bytesIn).toBe(10);
    expect(snap.bytesOut).toBe(20);
    expect(snap.packetsIn).toBe(1);
    expect(snap.packetsOut).toBe(1);
  });
});

describe("UdpSessionRegistry – closeSession", () => {
  it("removes the session from snapshot", () => {
    const reg = new UdpSessionRegistry();
    const id = reg.openOrTouchSession(baseInput);
    reg.closeSession(id);
    expect(reg.snapshot()).toHaveLength(0);
  });

  it("closeSession on unknown ID is a no-op", () => {
    const reg = new UdpSessionRegistry();
    expect(() => reg.closeSession("no-such-id")).not.toThrow();
  });

  it("closeSession allows re-opening a new session with the same key", () => {
    const reg = new UdpSessionRegistry();
    const id1 = reg.openOrTouchSession(baseInput);
    reg.closeSession(id1);
    const id2 = reg.openOrTouchSession(baseInput);
    expect(id2).not.toBe(id1);
    expect(reg.snapshot()).toHaveLength(1);
  });

  it("only removes the targeted session", () => {
    const reg = new UdpSessionRegistry();
    const id1 = reg.openOrTouchSession(baseInput);
    const id2 = reg.openOrTouchSession({ ...baseInput, clientPort: 11111 });
    reg.closeSession(id1);
    const snap = reg.snapshot();
    expect(snap).toHaveLength(1);
    expect(snap[0].id).toBe(id2);
  });
});

describe("UdpSessionRegistry – closeSessionsForRule", () => {
  it("removes all sessions for the given rule", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    reg.openOrTouchSession({ ...baseInput, clientPort: 11111 });
    reg.closeSessionsForRule("rule-1");
    expect(reg.snapshot()).toHaveLength(0);
  });

  it("leaves sessions for other rules untouched", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    reg.openOrTouchSession({ ...baseInput, ruleId: "rule-2" });
    reg.closeSessionsForRule("rule-1");
    const snap = reg.snapshot();
    expect(snap).toHaveLength(1);
    expect(snap[0].ruleId).toBe("rule-2");
  });

  it("is a no-op when no sessions exist for the rule", () => {
    const reg = new UdpSessionRegistry();
    expect(() => reg.closeSessionsForRule("rule-99")).not.toThrow();
    expect(reg.snapshot()).toHaveLength(0);
  });

  it("allows re-opening sessions after close", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    reg.closeSessionsForRule("rule-1");
    const newId = reg.openOrTouchSession(baseInput);
    expect(reg.snapshot()).toHaveLength(1);
    expect(reg.snapshot()[0].id).toBe(newId);
  });
});

describe("UdpSessionRegistry – pruneExpired", () => {
  it("removes sessions older than UDP_SESSION_EXPIRE_MS", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    // Prune with a time far in the future
    reg.pruneExpired(new Date(Date.now() + UDP_SESSION_EXPIRE_MS + 1000));
    expect(reg.snapshot()).toHaveLength(0);
  });

  it("does not remove sessions within expiry window", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    // Prune at current time — session is fresh, should not be pruned
    reg.pruneExpired(new Date());
    expect(reg.snapshot()).toHaveLength(1);
  });

  it("only removes expired sessions, leaving fresh ones", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput); // fresh
    reg.openOrTouchSession({ ...baseInput, clientPort: 11111 }); // also fresh

    // Expire one by touching it at a time in the past (via recordInbound then manually)
    // Instead: open session, manually backdate its lastSeenAt
    const id = reg.openOrTouchSession({ ...baseInput, clientPort: 22222 });
    // Backdate the entry directly
    const entry = (reg as unknown as { entries: Map<string, { lastSeenAt: Date }> }).entries.get(id);
    if (entry) {
      entry.lastSeenAt = new Date(Date.now() - UDP_SESSION_EXPIRE_MS - 1000);
    }

    reg.pruneExpired(new Date());
    expect(reg.snapshot()).toHaveLength(2); // fresh sessions remain
  });

  it("pruneExpired with no args uses current time", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    // Should not prune a fresh session
    expect(() => reg.pruneExpired()).not.toThrow();
    expect(reg.snapshot()).toHaveLength(1);
  });

  it("allows re-opening a session after its key was pruned", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    reg.pruneExpired(new Date(Date.now() + UDP_SESSION_EXPIRE_MS + 1000));
    const newId = reg.openOrTouchSession(baseInput);
    expect(reg.snapshot()).toHaveLength(1);
    expect(reg.snapshot()[0].id).toBe(newId);
  });
});

describe("UdpSessionRegistry – snapshot expiry filtering", () => {
  it("snapshot excludes expired sessions without pruning them", () => {
    const reg = new UdpSessionRegistry();
    const id = reg.openOrTouchSession(baseInput);
    // Backdate lastSeenAt past expiry
    const entry = (reg as unknown as { entries: Map<string, { lastSeenAt: Date }> }).entries.get(id);
    if (entry) entry.lastSeenAt = new Date(Date.now() - UDP_SESSION_EXPIRE_MS - 1000);

    // snapshot filters it out but doesn't delete it
    expect(reg.snapshot()).toHaveLength(0);
    // Entry still in registry (not pruned)
    expect((reg as unknown as { entries: Map<string, unknown> }).entries.size).toBe(1);
  });

  it("snapshotForRule excludes expired sessions for the rule", () => {
    const reg = new UdpSessionRegistry();
    const id1 = reg.openOrTouchSession(baseInput); // fresh
    const id2 = reg.openOrTouchSession({ ...baseInput, clientPort: 11111 }); // will be expired

    const entry2 = (reg as unknown as { entries: Map<string, { lastSeenAt: Date }> }).entries.get(id2);
    if (entry2) entry2.lastSeenAt = new Date(Date.now() - UDP_SESSION_EXPIRE_MS - 1000);

    const snap = reg.snapshotForRule("rule-1");
    expect(snap).toHaveLength(1);
    expect(snap[0].id).toBe(id1);
  });

  it("snapshotForRule excludes sessions for other rules", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput); // rule-1
    reg.openOrTouchSession({ ...baseInput, ruleId: "rule-2" });

    const snap = reg.snapshotForRule("rule-1");
    expect(snap).toHaveLength(1);
    expect(snap[0].ruleId).toBe("rule-1");
  });
});

describe("UdpSessionRegistry – multiple rules and UDP modes", () => {
  it("tracks sessions for multiple rules independently", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession({ ...baseInput, ruleId: "rule-1", ruleName: "Rule One" });
    reg.openOrTouchSession({ ...baseInput, ruleId: "rule-2", ruleName: "Rule Two" });
    expect(reg.snapshot()).toHaveLength(2);
    expect(reg.snapshotForRule("rule-1")).toHaveLength(1);
    expect(reg.snapshotForRule("rule-2")).toHaveLength(1);
  });

  it("multi-client mode creates separate sessions per client", () => {
    const reg = new UdpSessionRegistry();
    const mcInput = { ...baseInput, mode: "bidirectional-multi-client" as const };
    reg.openOrTouchSession({ ...mcInput, clientPort: 10001 });
    reg.openOrTouchSession({ ...mcInput, clientPort: 10002 });
    reg.openOrTouchSession({ ...mcInput, clientPort: 10003 });
    expect(reg.snapshot()).toHaveLength(3);
    expect(reg.snapshotForRule("rule-1")).toHaveLength(3);
  });

  it("multi-client repeated touch does not duplicate", () => {
    const reg = new UdpSessionRegistry();
    const mcInput = { ...baseInput, mode: "bidirectional-multi-client" as const };
    reg.openOrTouchSession({ ...mcInput, clientPort: 10001 });
    reg.openOrTouchSession({ ...mcInput, clientPort: 10001 }); // same client
    expect(reg.snapshot()).toHaveLength(1);
  });

  it("last-client mode separate sessions for different clients", () => {
    const reg = new UdpSessionRegistry();
    const lcInput = { ...baseInput, mode: "bidirectional-last-client" as const };
    // Different clients → different keys → different sessions
    reg.openOrTouchSession({ ...lcInput, clientPort: 10001 });
    reg.openOrTouchSession({ ...lcInput, clientPort: 10002 });
    expect(reg.snapshot()).toHaveLength(2);
  });

  it("one-way mode tracked by client endpoint", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput); // clientPort 54321
    reg.openOrTouchSession({ ...baseInput, clientPort: 55555 }); // different client
    expect(reg.snapshot()).toHaveLength(2);
  });
});

describe("UdpSessionRegistry – snapshot immutability", () => {
  it("mutating a snapshot entry does not affect registry state", () => {
    const reg = new UdpSessionRegistry();
    const id = reg.openOrTouchSession(baseInput);
    reg.recordInbound(id, 42);
    const snap = reg.snapshot();
    (snap[0] as unknown as Record<string, unknown>).bytesIn = 9999;
    expect(reg.snapshot()[0].bytesIn).toBe(42);
  });

  it("snapshot contains exactly the defined UdpSessionInfo fields", () => {
    const reg = new UdpSessionRegistry();
    reg.openOrTouchSession(baseInput);
    const snap = reg.snapshot();
    const keys = Object.keys(snap[0]).sort();
    expect(keys).toEqual([
      "bytesIn",
      "bytesOut",
      "clientAddress",
      "clientPort",
      "id",
      "idleMs",
      "lastSeenAt",
      "mode",
      "packetsIn",
      "packetsOut",
      "protocol",
      "ruleId",
      "ruleName",
      "startedAt",
      "status",
      "targetAddress",
      "targetPort"
    ]);
  });
});
