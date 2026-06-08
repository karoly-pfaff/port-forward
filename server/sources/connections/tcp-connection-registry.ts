import crypto from "node:crypto";
import type { TcpConnectionInfo } from "@portier/shared";

export interface TcpConnectionInput {
  ruleId: string;
  ruleName: string;
  clientAddress: string;
  clientPort: number;
  targetAddress: string;
  targetPort: number;
}

interface TcpConnectionEntry {
  id: string;
  ruleId: string;
  ruleName: string;
  clientAddress: string;
  clientPort: number;
  targetAddress: string;
  targetPort: number;
  startedAt: Date;
  bytesIn: number;
  bytesOut: number;
}

export class TcpConnectionRegistry {
  private readonly entries = new Map<string, TcpConnectionEntry>();

  openConnection(input: TcpConnectionInput): string {
    const id = crypto.randomUUID();
    this.entries.set(id, {
      id,
      ruleId: input.ruleId,
      ruleName: input.ruleName,
      clientAddress: input.clientAddress,
      clientPort: input.clientPort,
      targetAddress: input.targetAddress,
      targetPort: input.targetPort,
      startedAt: new Date(),
      bytesIn: 0,
      bytesOut: 0
    });
    return id;
  }

  addBytesIn(id: string, byteCount: number): void {
    const entry = this.entries.get(id);
    if (entry) entry.bytesIn += byteCount;
  }

  addBytesOut(id: string, byteCount: number): void {
    const entry = this.entries.get(id);
    if (entry) entry.bytesOut += byteCount;
  }

  closeConnection(id: string): void {
    this.entries.delete(id);
  }

  closeConnectionsForRule(ruleId: string): void {
    const toDelete: string[] = [];
    for (const [id, entry] of this.entries) {
      if (entry.ruleId === ruleId) toDelete.push(id);
    }
    for (const id of toDelete) {
      this.entries.delete(id);
    }
  }

  snapshot(now: Date = new Date()): TcpConnectionInfo[] {
    const nowMs = now.getTime();
    return [...this.entries.values()].map((entry) => this.toInfo(entry, nowMs));
  }

  snapshotForRule(ruleId: string, now: Date = new Date()): TcpConnectionInfo[] {
    const nowMs = now.getTime();
    return [...this.entries.values()]
      .filter((entry) => entry.ruleId === ruleId)
      .map((entry) => this.toInfo(entry, nowMs));
  }

  private toInfo(entry: TcpConnectionEntry, nowMs: number): TcpConnectionInfo {
    return {
      id: entry.id,
      ruleId: entry.ruleId,
      ruleName: entry.ruleName,
      protocol: "tcp",
      clientAddress: entry.clientAddress,
      clientPort: entry.clientPort,
      targetAddress: entry.targetAddress,
      targetPort: entry.targetPort,
      startedAt: entry.startedAt.toISOString(),
      durationMs: nowMs - entry.startedAt.getTime(),
      bytesIn: entry.bytesIn,
      bytesOut: entry.bytesOut,
      status: "active"
    };
  }
}
