import crypto from "node:crypto";
import type { UdpMode, UdpSessionInfo } from "@portier/shared";

export const UDP_SESSION_IDLE_MS = 30_000;
export const UDP_SESSION_EXPIRE_MS = 300_000;

export interface UdpSessionInput {
  ruleId: string;
  ruleName: string;
  mode: UdpMode;
  clientAddress: string;
  clientPort: number;
  targetAddress: string;
  targetPort: number;
}

interface UdpSessionEntry {
  id: string;
  key: string;
  ruleId: string;
  ruleName: string;
  mode: UdpMode;
  clientAddress: string;
  clientPort: number;
  targetAddress: string;
  targetPort: number;
  startedAt: Date;
  lastSeenAt: Date;
  packetsIn: number;
  packetsOut: number;
  bytesIn: number;
  bytesOut: number;
}

export class UdpSessionRegistry {
  private readonly entries = new Map<string, UdpSessionEntry>();
  private readonly keyToId = new Map<string, string>();

  openOrTouchSession(input: UdpSessionInput): string {
    const key = `${input.ruleId}:${input.mode}:${input.clientAddress}:${input.clientPort}`;
    const existingId = this.keyToId.get(key);

    if (existingId !== undefined) {
      const entry = this.entries.get(existingId);
      if (entry) {
        entry.lastSeenAt = new Date();
        return existingId;
      }
      // Defensive: stale key — entry was removed without cleaning keyToId
      this.keyToId.delete(key);
    }

    const id = crypto.randomUUID();
    const now = new Date();
    this.entries.set(id, {
      id,
      key,
      ruleId: input.ruleId,
      ruleName: input.ruleName,
      mode: input.mode,
      clientAddress: input.clientAddress,
      clientPort: input.clientPort,
      targetAddress: input.targetAddress,
      targetPort: input.targetPort,
      startedAt: now,
      lastSeenAt: now,
      packetsIn: 0,
      packetsOut: 0,
      bytesIn: 0,
      bytesOut: 0
    });
    this.keyToId.set(key, id);
    return id;
  }

  recordInbound(id: string, byteCount: number): void {
    const entry = this.entries.get(id);
    if (entry) {
      entry.packetsIn += 1;
      entry.bytesIn += byteCount;
      entry.lastSeenAt = new Date();
    }
  }

  recordOutbound(id: string, byteCount: number): void {
    const entry = this.entries.get(id);
    if (entry) {
      entry.packetsOut += 1;
      entry.bytesOut += byteCount;
    }
  }

  closeSession(id: string): void {
    const entry = this.entries.get(id);
    if (entry) {
      this.keyToId.delete(entry.key);
      this.entries.delete(id);
    }
  }

  closeSessionsForRule(ruleId: string): void {
    const toDelete: string[] = [];
    for (const [id, entry] of this.entries) {
      if (entry.ruleId === ruleId) toDelete.push(id);
    }
    for (const id of toDelete) {
      const entry = this.entries.get(id);
      if (entry) this.keyToId.delete(entry.key);
      this.entries.delete(id);
    }
  }

  pruneExpired(now: Date = new Date()): void {
    const cutoff = now.getTime() - UDP_SESSION_EXPIRE_MS;
    const toDelete: string[] = [];
    for (const [id, entry] of this.entries) {
      if (entry.lastSeenAt.getTime() < cutoff) toDelete.push(id);
    }
    for (const id of toDelete) {
      const entry = this.entries.get(id);
      if (entry) this.keyToId.delete(entry.key);
      this.entries.delete(id);
    }
  }

  snapshot(now: Date = new Date()): UdpSessionInfo[] {
    const nowMs = now.getTime();
    const expiryCutoff = nowMs - UDP_SESSION_EXPIRE_MS;
    return [...this.entries.values()]
      .filter((entry) => entry.lastSeenAt.getTime() >= expiryCutoff)
      .map((entry) => this.toInfo(entry, nowMs));
  }

  snapshotForRule(ruleId: string, now: Date = new Date()): UdpSessionInfo[] {
    const nowMs = now.getTime();
    const expiryCutoff = nowMs - UDP_SESSION_EXPIRE_MS;
    return [...this.entries.values()]
      .filter((entry) => entry.ruleId === ruleId && entry.lastSeenAt.getTime() >= expiryCutoff)
      .map((entry) => this.toInfo(entry, nowMs));
  }

  private toInfo(entry: UdpSessionEntry, nowMs: number): UdpSessionInfo {
    const idleMs = nowMs - entry.lastSeenAt.getTime();
    return {
      id: entry.id,
      ruleId: entry.ruleId,
      ruleName: entry.ruleName,
      protocol: "udp",
      mode: entry.mode,
      clientAddress: entry.clientAddress,
      clientPort: entry.clientPort,
      targetAddress: entry.targetAddress,
      targetPort: entry.targetPort,
      startedAt: entry.startedAt.toISOString(),
      lastSeenAt: entry.lastSeenAt.toISOString(),
      idleMs,
      packetsIn: entry.packetsIn,
      packetsOut: entry.packetsOut,
      bytesIn: entry.bytesIn,
      bytesOut: entry.bytesOut,
      status: idleMs >= UDP_SESSION_IDLE_MS ? "idle" : "active"
    };
  }
}
