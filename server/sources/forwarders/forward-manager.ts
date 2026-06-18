import crypto from "node:crypto";
import type { ActivityEventType, ActivitySeverity, ExportedConfig, ForwardRule, ForwardRuleInput, ForwardStatus, GroupActionResult, ImportMode, ImportResult, TcpConnectionInfo, UdpSessionInfo } from "@portier/shared";
import { deriveRuleHealth, listenKey, validateForwardRule, validateForwardRulePatch } from "@portier/shared";
import { buildExportedConfig, configExportedActivityEvent } from "../config/config-export.js";
import type { Forwarder, ForwarderStatus } from "./types.js";
import { TcpForwarder } from "./tcp-forwarder.js";
import { UdpForwarder } from "./udp-forwarder.js";
import type { ActivityStore } from "../activity/activity-store.js";
import { TcpConnectionRegistry } from "../connections/tcp-connection-registry.js";
import { UdpSessionRegistry } from "../connections/udp-session-registry.js";
import type { RecoveryLoadResult, RecoveryState } from "../recovery/config-recovery.js";

export interface RuleStore {
  load(): Promise<ForwardRule[]>;
  save(rules: ForwardRule[]): Promise<void>;
  /**
   * Optional startup load that recovers from load failures (config-load
   * recovery). The real ConfigStore implements it; in-memory test stores omit it
   * and the manager falls back to load(). When present, its recovery state is
   * carried so writes can be blocked while recovery is active.
   */
  loadWithRecovery?(): Promise<RecoveryLoadResult>;
}

/** One enabled rule that did not autostart, with a concise operator-safe reason. */
export interface RuleStartOutcome {
  ruleId: string;
  ruleName: string;
  error: string;
}

/**
 * Summary of a boot-time autostart pass. Informational (for logging); the pass
 * is non-fatal, so there is no thrown error. Mirrors the Go StartEnabledResult.
 */
export interface LoadAndStartResult {
  attempted: number;
  started: number;
  failed: RuleStartOutcome[];
  skipped: RuleStartOutcome[];
}

/** Returns a human message for a thrown value (an `Error`'s message, else its string form). */
export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export class ForwardManager {
  private rules = new Map<string, ForwardRule>();
  private forwarders = new Map<string, Forwarder>();
  private readonly tcpRegistry = new TcpConnectionRegistry();
  private readonly udpRegistry = new UdpSessionRegistry();
  // Last start failure per rule, for rules with no live forwarder (failed
  // autostart or a skipped duplicate-binding conflict). Mirrors the Go manager's
  // runtimeState.lastError so getStatus can report lastError + error health even
  // when no forwarder is retained.
  private readonly lastErrors = new Map<string, string>();
  private recovery?: RecoveryState;

  constructor(
    private readonly store: RuleStore,
    private readonly activity?: ActivityStore
  ) {}

  /**
   * The active startup recovery state, or undefined when the config loaded
   * normally. Internal accessor for later API/UI/CLI surfacing (Slice 5) and tests.
   */
  recoveryState(): RecoveryState | undefined {
    return this.recovery;
  }

  getLiveTcpConnections(): TcpConnectionInfo[] {
    return this.tcpRegistry.snapshot();
  }

  getLiveUdpSessions(): UdpSessionInfo[] {
    return this.udpRegistry.snapshot();
  }

  /**
   * Load persisted rules and autostart enabled ones. Non-fatal (v1.17 R-1): a
   * config-load failure, a persisted duplicate binding, or a per-rule bind
   * failure never throws to the bootstrap — the management API still comes up.
   *
   * - Config-load recovery (Slice 2/4): when the store supports it, load via
   *   loadWithRecovery; a recovered load yields empty rules + a recovery state
   *   (writes then blocked). Persisted duplicate bindings are NOT rejected at
   *   load (create/update/import still reject NEW duplicates).
   * - Autostart recovery (Slice 3/4): enabled rules that share a listen binding
   *   with another enabled rule are skipped (no arbitrary winner); a rule whose
   *   forwarder fails to bind is left enabled-but-stopped. Both are marked with a
   *   lastError so getStatus reports error health. Every other enabled rule is
   *   still attempted.
   *
   * Returns a summary for logging; rule status carries the authoritative truth.
   */
  async loadAndStartEnabled(): Promise<LoadAndStartResult> {
    const loaded: RecoveryLoadResult = this.store.loadWithRecovery
      ? await this.store.loadWithRecovery()
      : { rules: await this.store.load() };
    this.recovery = loaded.recovery;

    for (const rule of loaded.rules) {
      this.rules.set(rule.id, rule);
    }

    const result: LoadAndStartResult = { attempted: 0, started: 0, failed: [], skipped: [] };
    const conflicts = this.conflictingEnabledBindings();

    for (const rule of this.rules.values()) {
      if (!rule.enabled) {
        continue;
      }
      result.attempted += 1;

      const conflict = conflicts.get(rule.id);
      if (conflict) {
        this.markRuleStartFailure(rule, conflict);
        result.skipped.push({ ruleId: rule.id, ruleName: rule.name, error: conflict });
        continue;
      }

      try {
        await this.startRule(rule.id);
        result.started += 1;
      } catch (error) {
        result.failed.push({ ruleId: rule.id, ruleName: rule.name, error: errorMessage(error) });
      }
    }

    return result;
  }

  /**
   * For each enabled rule that shares a listen binding (protocol + listenHost +
   * listenPort) with another enabled rule, a deterministic operator-safe message
   * naming the conflict. Disabled rules never contribute (they will not bind), so
   * an enabled rule that only shares its binding with disabled rules autostarts
   * normally. Names are listed in stable rule order. Mirrors the Go
   * conflictingEnabledBindings.
   */
  private conflictingEnabledBindings(): Map<string, string> {
    const groups = new Map<string, ForwardRule[]>();
    for (const rule of this.rules.values()) {
      if (!rule.enabled) {
        continue;
      }
      const key = listenKey(rule);
      const group = groups.get(key) ?? [];
      group.push(rule);
      groups.set(key, group);
    }

    const messages = new Map<string, string>();
    for (const group of groups.values()) {
      if (group.length < 2) {
        continue;
      }
      const names = group.map((rule) => `"${rule.name}"`).join(", ");
      const first = group[0];
      const message =
        `Listen binding ${first.protocol.toUpperCase()} ${first.listenHost}:${first.listenPort} ` +
        `is claimed by ${group.length} enabled rules (${names}); not autostarted to avoid a conflict.`;
      for (const rule of group) {
        messages.set(rule.id, message);
      }
    }
    return messages;
  }

  /**
   * Record an enabled rule as stopped/error with lastError WITHOUT attempting to
   * bind, emitting the existing rule.error activity event. Used for autostart
   * conflict-skips; bind failures take the startRule path (which already sets
   * lastError and emits rule.error).
   */
  private markRuleStartFailure(rule: ForwardRule, message: string): void {
    this.lastErrors.set(rule.id, message);
    this.emitRuleEvent("rule.error", "error", rule, message);
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
    const rule = this.rules.get(ruleId);
    const forwarder = this.forwarders.get(ruleId);
    // The forwarders track runtime counters but not `enabled`; the manager is the
    // single place that derives `health` (it owns the rule definition).
    const isUdp = rule?.protocol === "udp";
    const base: ForwarderStatus = forwarder
      ? forwarder.getStatus()
      : {
          ruleId,
          running: false,
          activeConnections: rule?.protocol === "tcp" ? 0 : undefined,
          bytesIn: 0,
          bytesOut: 0,
          packetsIn: isUdp ? 0 : undefined,
          packetsOut: isUdp ? 0 : undefined,
          activeUdpSessions: rule?.udpMode === "bidirectional-multi-client" ? 0 : undefined
        };
    if (!forwarder) {
      // No live forwarder: surface the last start failure (failed autostart or a
      // skipped duplicate-binding conflict) so health derives to "error" — parity
      // with the Go manager, which keeps lastError in runtime state.
      const recorded = this.lastErrors.get(ruleId);
      if (recorded) {
        base.lastError = recorded;
      }
    }
    return {
      ...base,
      health: deriveRuleHealth({
        enabled: rule?.enabled ?? false,
        running: base.running,
        lastError: base.lastError
      })
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
    try {
      await this.persist();
    } catch (error) {
      // Roll back the appended rule so a failed persist leaves no partial
      // in-memory state (parity with Go manager.CreateRule).
      this.rules.delete(rule.id);
      throw error;
    }

    this.emitRuleEvent("rule.created", "success", rule, `Rule "${rule.name}" created.`);

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
    /* v8 ignore next 3 -- unreachable: a patch that passes patch validation, merged onto an already-valid rule, always re-validates as valid */
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
    try {
      await this.persist();
    } catch (error) {
      // Restore the original rule and, if we stopped a running forwarder for a
      // forwarding-field change, restart it — so a failed persist leaves no
      // partial in-memory or running-state mutation (parity with Go
      // manager.UpdateRule). A restart failure must not mask the persist error.
      this.rules.set(existing.id, existing);
      if (needsRestart) {
        await this.startRule(existing.id).catch(() => {});
      }
      throw error;
    }

    this.emitRuleEvent("rule.updated", "info", next, `Rule "${next.name}" updated.`);

    if (needsRestart) {
      // If start fails the rule stays stopped with the new config; error propagates to caller.
      await this.startRule(existing.id);
    }

    return next;
  }

  async deleteRule(ruleId: string): Promise<void> {
    const rule = this.requireRule(ruleId);
    const wasRunning = this.forwarders.has(ruleId);
    await this.stopRule(ruleId);
    this.rules.delete(ruleId);
    try {
      await this.persist();
    } catch (error) {
      // Restore the deleted rule and, if it was running, restart it — so a
      // failed persist leaves no partial mutation (parity with Go
      // manager.DeleteRule). A restart failure must not mask the persist error.
      this.rules.set(ruleId, rule);
      if (wasRunning) {
        await this.startRule(ruleId).catch(() => {});
      }
      throw error;
    }

    // Drop any retained start failure so a deleted id leaves no stale lastError
    // (parity with Go manager.DeleteRule, which removes the runtime entry).
    this.lastErrors.delete(ruleId);
    this.emitRuleEvent("rule.deleted", "warning", rule, `Rule "${rule.name}" deleted.`);
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
      this.lastErrors.delete(ruleId);
      this.emitRuleEvent("rule.started", "success", rule, `Rule "${rule.name}" started.`);
      return this.getStatus(ruleId);
    } catch (error) {
      this.forwarders.delete(ruleId);
      const message = errorMessage(error);
      // Retain the failure so getStatus reports lastError + error health even
      // though no forwarder is kept (parity with the Go manager).
      this.lastErrors.set(ruleId, message);
      this.emitRuleEvent("rule.error", "error", rule, `Rule "${rule.name}" failed to start: ${message}`);
      throw error;
    }
  }

  async stopRule(ruleId: string): Promise<ForwardStatus> {
    const rule = this.requireRule(ruleId);
    const forwarder = this.forwarders.get(ruleId);
    if (forwarder) {
      await forwarder.stop();
      this.forwarders.delete(ruleId);
      this.lastErrors.delete(ruleId);
      this.emitRuleEvent("rule.stopped", "info", rule, `Rule "${rule.name}" stopped.`);
    }
    return this.getStatus(ruleId);
  }

  async stopAll(): Promise<void> {
    await Promise.all([...this.forwarders.keys()].map((ruleId) => this.stopRule(ruleId)));
  }

  // Returns the rules whose normalized group equals `group`, in rule order.
  // `group` is expected already-normalized (trimmed); stored rule groups are
  // normalized on save, so an exact match is correct.
  private rulesInGroup(group: string): ForwardRule[] {
    return this.listRules().filter((rule) => rule.group === group);
  }

  // Starts every rule in the group, in rule order. Behaviour mirrors the
  // single-rule start (POST /api/forwards/:id/start): an already-running rule is
  // skipped (idempotent), and `enabled`/autostart is NOT a precondition. Returns
  // one result per matched rule; an empty array means no rule matched the group.
  // Does not mutate rule definitions, order, or metadata.
  async startGroup(group: string): Promise<GroupActionResult[]> {
    const results: GroupActionResult[] = [];
    for (const rule of this.rulesInGroup(group)) {
      if (this.forwarders.has(rule.id)) {
        results.push({ ruleId: rule.id, ruleName: rule.name, status: "skipped", reason: "already_running" });
        continue;
      }
      try {
        await this.startRule(rule.id);
        results.push({ ruleId: rule.id, ruleName: rule.name, status: "started" });
      } catch (error) {
        results.push({
          ruleId: rule.id,
          ruleName: rule.name,
          status: "failed",
          reason: errorMessage(error),
        });
      }
    }
    return results;
  }

  // Stops every running rule in the group, in rule order. A rule that is not
  // running is skipped. Mirrors single-rule stop semantics.
  async stopGroup(group: string): Promise<GroupActionResult[]> {
    const results: GroupActionResult[] = [];
    for (const rule of this.rulesInGroup(group)) {
      if (!this.forwarders.has(rule.id)) {
        results.push({ ruleId: rule.id, ruleName: rule.name, status: "skipped", reason: "not_running" });
        continue;
      }
      try {
        await this.stopRule(rule.id);
        results.push({ ruleId: rule.id, ruleName: rule.name, status: "stopped" });
        /* v8 ignore start -- stopRule does not throw for a running forwarder in practice; this mirrors startGroup's tested failure path */
      } catch (error) {
        results.push({
          ruleId: rule.id,
          ruleName: rule.name,
          status: "failed",
          reason: errorMessage(error),
        });
      }
      /* v8 ignore stop */
    }
    return results;
  }

  async flush(): Promise<void> {
    await this.persist();
  }

  exportConfig(now: Date = new Date()): ExportedConfig {
    const config = buildExportedConfig({ rules: this.listRules(), now });
    this.activity?.add(configExportedActivityEvent(config.rules.length));
    return config;
  }

  async importConfig(config: ExportedConfig, mode: ImportMode): Promise<ImportResult> {
    const rules = config.rules;

    // Validate all rules before applying any changes.
    const validated: ForwardRule[] = [];
    const errors: string[] = [];
    for (const raw of rules) {
      const result = validateForwardRule(raw);
      if (!result.valid) {
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

    // Reject duplicate listen bindings within the imported set before any
    // mutation, for both replace and merge modes (parity with Go
    // manager.ImportConfig → ensureNoDuplicateBindings). A listen binding is
    // protocol + listenHost + listenPort; two imported rules with distinct ids
    // but the same binding cannot both run, so the import is rejected with no
    // in-memory mutation, no persist, and no forwarder start/stop.
    const bindingError = ensureNoDuplicateBindings(validated);
    if (bindingError) {
      this.activity?.add({
        type: "config.import.failed",
        severity: "error",
        message: `Config import rejected: ${bindingError}`
      });
      return { imported: 0, skipped: 0, errors: [bindingError] };
    }

    let imported = 0;
    let skipped = 0;

    // Snapshot the current rules so a failed persist can roll back to the prior
    // config with no partial mutation (parity with Go manager.ImportConfig).
    const previousRules = new Map(this.rules);

    if (mode === "replace") {
      // Stop all running rules, replace everything.
      await this.stopAll();
      this.rules.clear();

      for (const rule of validated) {
        this.rules.set(rule.id, rule);
        imported += 1;
      }

      try {
        await this.persist();
      } catch (error) {
        this.rules = previousRules;
        throw error;
      }

      // Fresh config: drop retained start failures (parity with Go's runtime
      // reset on replace). The restart loop sets new ones for any that fail.
      this.lastErrors.clear();

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

      try {
        await this.persist();
      } catch (error) {
        // Stop any forwarders started during this merge and restore the prior
        // rules, so a failed persist leaves no partial mutation.
        for (const rule of toAdd) {
          if (this.forwarders.has(rule.id)) {
            await this.stopRule(rule.id).catch(() => {});
          }
        }
        this.rules = previousRules;
        throw error;
      }
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
      /* v8 ignore next -- every id was validated to exist above; the falsy branch is unreachable */
      if (rule) newRules.set(id, rule);
    }
    for (const [id, rule] of this.rules) {
      if (!newRules.has(id)) newRules.set(id, rule);
    }

    const previousRules = this.rules;
    this.rules = newRules;
    try {
      await this.persist();
    } catch (error) {
      // Restore the original order so a failed persist leaves no partial
      // mutation (parity with Go manager.ReorderRules).
      this.rules = previousRules;
      throw error;
    }
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

  // Records a rule-scoped activity event, populating ruleId/ruleName/protocol
  // from the given rule. Single emission path for rule-scoped events
  // (created/updated/deleted/started/stopped/error) so their payload shape
  // cannot drift between call sites — mirrors the Go manager's emitRuleEvent
  // (Arch-C2). Config-level events (export/import) are not rule-scoped and keep
  // calling this.activity?.add directly with their own details.
  private emitRuleEvent(
    type: ActivityEventType,
    severity: ActivitySeverity,
    rule: ForwardRule,
    message: string
  ): void {
    this.activity?.add({
      type,
      severity,
      ruleId: rule.id,
      ruleName: rule.name,
      protocol: rule.protocol,
      message
    });
  }

  private async persist(): Promise<void> {
    if (this.recovery?.writesBlocked) {
      throw new RecoveryError(
        "Configuration is in recovery mode; rule changes are blocked until the configuration is repaired."
      );
    }
    await this.store.save(this.listRules());
  }
}

/**
 * Returns an error message if two rules in the set share the same listen
 * binding (protocol + listenHost + listenPort), or undefined if all bindings
 * are unique. Mirrors the Go service `manager.ensureNoDuplicateBindings` —
 * including message wording — so config-import rejects the same intra-set
 * duplicate bindings in both runtimes.
 */
function ensureNoDuplicateBindings(rules: ForwardRule[]): string | undefined {
  const seen = new Map<string, ForwardRule>();
  for (const rule of rules) {
    const key = listenKey(rule);
    const existing = seen.get(key);
    if (existing) {
      return `a ${rule.protocol} rule is already listening on ${rule.listenHost}:${rule.listenPort} (rules "${existing.name}" and "${rule.name}")`;
    }
    seen.set(key, rule);
  }
  return undefined;
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

/**
 * Thrown when a mutating operation is refused because the runtime is in
 * config-load recovery mode (writes blocked). It carries no API schema change:
 * mapManagerError re-throws it so the error-envelope filter maps it to a generic
 * 500 (parity with the Go manager.RecoveryError). Slice 5 will decide on a
 * dedicated status / surfacing.
 */
export class RecoveryError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "RecoveryError";
  }
}
