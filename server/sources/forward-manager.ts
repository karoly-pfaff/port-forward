import crypto from "node:crypto";
import type { ExportedConfig, ForwardRule, ForwardRuleInput, ForwardStatus, ImportMode, ImportResult, TcpConnectionInfo, UdpSessionInfo } from "@portier/shared";
import { listenKey, validateForwardRule, validateForwardRulePatch } from "@portier/shared";
import type { Forwarder } from "./forwarders/types.js";
import { TcpForwarder } from "./forwarders/tcp-forwarder.js";
import { UdpForwarder } from "./forwarders/udp-forwarder.js";
import type { ActivityStore } from "./activity/activity-store.js";
import { TcpConnectionRegistry } from "./connections/tcp-connection-registry.js";
import { UdpSessionRegistry } from "./connections/udp-session-registry.js";

export interface RuleStore {
  load(): Promise<ForwardRule[]>;
  save(rules: ForwardRule[]): Promise<void>;
}

export class ForwardManager {
  private rules = new Map<string, ForwardRule>();
  private forwarders = new Map<string, Forwarder>();
  private readonly tcpRegistry = new TcpConnectionRegistry();
  private readonly udpRegistry = new UdpSessionRegistry();

  constructor(
    private readonly store: RuleStore,
    private readonly activity?: ActivityStore
  ) {}

  getLiveTcpConnections(): TcpConnectionInfo[] {
    return this.tcpRegistry.snapshot();
  }

  getLiveUdpSessions(): UdpSessionInfo[] {
    return this.udpRegistry.snapshot();
  }

  async loadAndStartEnabled(): Promise<number> {
    const rules = await this.store.load();
    for (const rule of rules) {
      this.ensureNoDuplicate(rule);
      this.rules.set(rule.id, rule);
    }

    let startedCount = 0;
    for (const rule of this.rules.values()) {
      if (rule.enabled) {
        await this.startRule(rule.id);
        startedCount += 1;
      }
    }

    return startedCount;
  }

  listRules(): ForwardRule[] {
    return [...this.rules.values()];
  }

  getRule(ruleId: string): ForwardRule | undefined {
    return this.rules.get(ruleId);
  }

  listStatus(): ForwardStatus[] {
    return this.listRules().map((rule) => this.getStatus(rule.id));
  }

  getStatus(ruleId: string): ForwardStatus {
    const forwarder = this.forwarders.get(ruleId);
    if (forwarder) {
      return forwarder.getStatus();
    }
    const rule = this.rules.get(ruleId);
    const isUdp = rule?.protocol === "udp";
    return {
      ruleId,
      running: false,
      activeConnections: rule?.protocol === "tcp" ? 0 : undefined,
      bytesIn: 0,
      bytesOut: 0,
      packetsIn: isUdp ? 0 : undefined,
      packetsOut: isUdp ? 0 : undefined,
      activeUdpSessions: rule?.udpMode === "bidirectional-multi-client" ? 0 : undefined
    };
  }

  async addRule(input: unknown): Promise<ForwardRule> {
    const result = validateForwardRule(input);
    if (!result.valid || !result.value) {
      throw new ValidationError(result.errors);
    }

    const rule: ForwardRule = {
      ...result.value,
      id: result.value.id ?? crypto.randomUUID()
    };
    this.ensureNoDuplicate(rule);
    this.rules.set(rule.id, rule);
    await this.persist();

    this.activity?.add({
      type: "rule.created",
      severity: "success",
      ruleId: rule.id,
      ruleName: rule.name,
      protocol: rule.protocol,
      message: `Rule "${rule.name}" created.`
    });

    if (rule.enabled) {
      await this.startRule(rule.id);
    }

    return rule;
  }

  async updateRule(ruleId: string, input: unknown): Promise<ForwardRule> {
    const existing = this.requireRule(ruleId);
    const result = validateForwardRulePatch(input);
    if (!result.valid || !result.value) {
      throw new ValidationError(result.errors);
    }

    const merged = {
      ...existing,
      ...result.value,
      id: existing.id
    } satisfies ForwardRuleInput as ForwardRule;

    const fullValidation = validateForwardRule(merged);
    if (!fullValidation.valid || !fullValidation.value) {
      throw new ValidationError(fullValidation.errors);
    }

    const next: ForwardRule = { ...fullValidation.value, id: existing.id };
    this.ensureNoDuplicate(next, existing.id);

    // Only restart if currently running AND a forwarding-affecting field changed.
    // enabled/name changes do not affect the active socket — no restart needed.
    const wasRunning = this.forwarders.has(existing.id);
    const needsRestart = wasRunning && this.forwardingFieldsChanged(existing, next);

    if (needsRestart) {
      await this.stopRule(existing.id);
    }

    this.rules.set(existing.id, next);
    await this.persist();

    this.activity?.add({
      type: "rule.updated",
      severity: "info",
      ruleId: next.id,
      ruleName: next.name,
      protocol: next.protocol,
      message: `Rule "${next.name}" updated.`
    });

    if (needsRestart) {
      // If start fails the rule stays stopped with the new config; error propagates to caller.
      await this.startRule(existing.id);
    }

    return next;
  }

  async deleteRule(ruleId: string): Promise<void> {
    const rule = this.requireRule(ruleId);
    await this.stopRule(ruleId);
    this.rules.delete(ruleId);
    await this.persist();

    this.activity?.add({
      type: "rule.deleted",
      severity: "warning",
      ruleId: rule.id,
      ruleName: rule.name,
      protocol: rule.protocol,
      message: `Rule "${rule.name}" deleted.`
    });
  }

  async startRule(ruleId: string): Promise<ForwardStatus> {
    const rule = this.requireRule(ruleId);
    if (this.forwarders.has(ruleId)) {
      return this.getStatus(ruleId);
    }

    const onEvent = this.activity
      ? this.activity.add.bind(this.activity)
      : undefined;

    const forwarder =
      rule.protocol === "tcp"
        ? new TcpForwarder(rule, onEvent, this.tcpRegistry)
        : new UdpForwarder(rule, onEvent, undefined, this.udpRegistry);
    this.forwarders.set(ruleId, forwarder);

    try {
      await forwarder.start();
      this.activity?.add({
        type: "rule.started",
        severity: "success",
        ruleId: rule.id,
        ruleName: rule.name,
        protocol: rule.protocol,
        message: `Rule "${rule.name}" started.`
      });
      return forwarder.getStatus();
    } catch (error) {
      this.forwarders.delete(ruleId);
      const message = error instanceof Error ? error.message : String(error);
      this.activity?.add({
        type: "rule.error",
        severity: "error",
        ruleId: rule.id,
        ruleName: rule.name,
        protocol: rule.protocol,
        message: `Rule "${rule.name}" failed to start: ${message}`
      });
      throw error;
    }
  }

  async stopRule(ruleId: string): Promise<ForwardStatus> {
    const rule = this.requireRule(ruleId);
    const forwarder = this.forwarders.get(ruleId);
    if (forwarder) {
      await forwarder.stop();
      this.forwarders.delete(ruleId);
      this.activity?.add({
        type: "rule.stopped",
        severity: "info",
        ruleId: rule.id,
        ruleName: rule.name,
        protocol: rule.protocol,
        message: `Rule "${rule.name}" stopped.`
      });
    }
    return this.getStatus(ruleId);
  }

  async stopAll(): Promise<void> {
    await Promise.all([...this.forwarders.keys()].map((ruleId) => this.stopRule(ruleId)));
  }

  async flush(): Promise<void> {
    await this.persist();
  }

  exportConfig(): ExportedConfig {
    const config: ExportedConfig = {
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: this.listRules()
    };
    this.activity?.add({
      type: "config.exported",
      severity: "info",
      message: `Config exported: ${config.rules.length} rule(s).`,
      details: { ruleCount: config.rules.length }
    });
    return config;
  }

  async importConfig(config: ExportedConfig, mode: ImportMode): Promise<ImportResult> {
    const rules = config.rules;

    // Validate all rules before applying any changes.
    const validated: ForwardRule[] = [];
    const errors: string[] = [];
    for (const raw of rules) {
      const result = validateForwardRule(raw);
      if (!result.valid || !result.value) {
        errors.push(`Rule "${String((raw as unknown as Record<string, unknown>).name ?? "?")}": ${result.errors.join(" ")}`);
      } else {
        validated.push({ ...result.value, id: (raw as ForwardRule).id ?? crypto.randomUUID() } as ForwardRule);
      }
    }

    if (errors.length > 0) {
      this.activity?.add({
        type: "config.import.failed",
        severity: "error",
        message: `Config import rejected: ${errors.length} invalid rule(s).`,
        details: { errors: errors.join("; ") }
      });
      return { imported: 0, skipped: 0, errors };
    }

    let imported = 0;
    let skipped = 0;

    if (mode === "replace") {
      // Stop all running rules, replace everything.
      await this.stopAll();
      this.rules.clear();

      for (const rule of validated) {
        this.rules.set(rule.id, rule);
        imported += 1;
      }

      await this.persist();

      // Restart enabled rules.
      for (const rule of this.rules.values()) {
        if (rule.enabled) {
          await this.startRule(rule.id).catch(() => {});
        }
      }
    } else {
      // Merge: add rules that don't conflict with existing IDs or listen bindings.
      const mergeErrors: string[] = [];
      const toAdd: ForwardRule[] = [];

      for (const rule of validated) {
        // Check ID conflict — regenerate if clashing
        const id = this.rules.has(rule.id) ? crypto.randomUUID() : rule.id;
        const candidate: ForwardRule = { ...rule, id };

        // Check listen binding conflict
        const key = listenKey(candidate);
        const conflict = [...this.rules.values()].find((r) => listenKey(r) === key);
        if (conflict) {
          mergeErrors.push(
            `Rule "${candidate.name}" conflicts with existing rule "${conflict.name}" on ${candidate.protocol.toUpperCase()} ${candidate.listenHost}:${candidate.listenPort}.`
          );
          skipped += 1;
          continue;
        }

        toAdd.push(candidate);
      }

      if (mergeErrors.length > 0) {
        this.activity?.add({
          type: "config.import.failed",
          severity: "warning",
          message: `Config merge had ${mergeErrors.length} conflict(s); import rejected.`,
          details: { errors: mergeErrors.join("; ") }
        });
        return { imported: 0, skipped, errors: mergeErrors };
      }

      for (const rule of toAdd) {
        this.rules.set(rule.id, rule);
        imported += 1;
        if (rule.enabled) {
          await this.startRule(rule.id).catch(() => {});
        }
      }

      await this.persist();
    }

    this.activity?.add({
      type: "config.imported",
      severity: "success",
      message: `Config imported (${mode}): ${imported} rule(s) added, ${skipped} skipped.`,
      details: { mode, imported, skipped }
    });

    return { imported, skipped, errors: [] };
  }

  async reorderRules(ids: string[]): Promise<void> {
    // Validate all IDs exist
    for (const id of ids) {
      if (!this.rules.has(id)) {
        throw new NotFoundError(`Rule ${id} was not found.`);
      }
    }

    // Rebuild Map in the specified order, then append any rules not in the list
    const newRules = new Map<string, ForwardRule>();
    for (const id of ids) {
      const rule = this.rules.get(id);
      if (rule) newRules.set(id, rule);
    }
    for (const [id, rule] of this.rules) {
      if (!newRules.has(id)) newRules.set(id, rule);
    }

    this.rules = newRules;
    await this.persist();
  }

  private requireRule(ruleId: string): ForwardRule {
    const rule = this.rules.get(ruleId);
    if (!rule) {
      throw new NotFoundError(`Forward rule ${ruleId} was not found.`);
    }
    return rule;
  }

  private ensureNoDuplicate(nextRule: ForwardRule, ignoreRuleId?: string): void {
    const nextKey = listenKey(nextRule);
    const duplicate = [...this.rules.values()].find(
      (rule) => rule.id !== ignoreRuleId && listenKey(rule) === nextKey
    );

    if (duplicate) {
      throw new ConflictError(
        `A ${nextRule.protocol.toUpperCase()} rule is already listening on ${nextRule.listenHost}:${nextRule.listenPort}.`
      );
    }
  }

  private forwardingFieldsChanged(a: ForwardRule, b: ForwardRule): boolean {
    return (
      a.protocol !== b.protocol ||
      a.listenHost !== b.listenHost ||
      a.listenPort !== b.listenPort ||
      a.targetHost !== b.targetHost ||
      a.targetPort !== b.targetPort ||
      a.udpMode !== b.udpMode
    );
  }

  private async persist(): Promise<void> {
    await this.store.save(this.listRules());
  }
}

export class ValidationError extends Error {
  constructor(public readonly errors: string[]) {
    super(errors.join(" "));
    this.name = "ValidationError";
  }
}

export class NotFoundError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "NotFoundError";
  }
}

export class ConflictError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ConflictError";
  }
}
