import type { ForwardRule, ForwardStatus, GroupActionResult } from "@portier/shared";
import { ForwardManager, type RuleStore } from "../../forwarders/forward-manager.js";

/**
 * Narrow write capability for `POST /api/forwards` (rule creation). The real
 * domain `ForwardManager` satisfies it via `addRule`; tests bind a seeded manager
 * shared with Express for parity. `addRule` validates (shared `validateForwardRule`),
 * rejects duplicate listen bindings, persists, and — only for an `enabled` rule —
 * starts the forwarder (identical to Express; tests create `enabled: false` rules
 * so no sockets/listeners start).
 */
export interface ForwardRuleCreator {
  addRule(input: unknown): Promise<ForwardRule>;
}

/** Injection token for the rule creator. */
export const FORWARD_RULE_CREATOR = "FORWARD_RULE_CREATOR";

/**
 * Minimal in-memory rule store (no disk) for the default creator. The
 * NestJS app is the static AppModule has no real runtime wired; this keeps creates
 * isolated and disk-free where the live server binds the real manager.
 */
export class InMemoryRuleStore implements RuleStore {
  private rules: ForwardRule[] = [];
  async load(): Promise<ForwardRule[]> {
    return this.rules;
  }
  async save(rules: ForwardRule[]): Promise<void> {
    this.rules = rules;
  }
}

/**
 * Default: a fresh, isolated in-memory `ForwardManager` (no disk, no
 * shared state). When the NestJS server is the active runtime this token is
 * bound to the shared `ForwardManager`; tests override it with a seeded manager.
 */
export function createDefaultForwardRuleCreator(): ForwardRuleCreator {
  return new ForwardManager(new InMemoryRuleStore());
}

/**
 * Narrow write capability for `PATCH /api/forwards/:id` (rule update). The real
 * domain `ForwardManager` satisfies it via `updateRule`; tests bind a seeded
 * manager shared with Express for parity. `updateRule` validates the partial
 * patch (shared `validateForwardRulePatch`, preserving absent fields), merges
 * over the existing rule, rejects duplicate bindings, throws `NotFoundError` for
 * an unknown id, persists (with rollback), and restarts the forwarder ONLY when
 * the rule is running AND a forwarding field changed (tests use `enabled:false`
 * stopped rules so no restart/socket occurs).
 */
export interface ForwardRuleUpdater {
  updateRule(ruleId: string, input: unknown): Promise<ForwardRule>;
}

/** Injection token for the rule updater. */
export const FORWARD_RULE_UPDATER = "FORWARD_RULE_UPDATER";

/** Default: a fresh, isolated in-memory `ForwardManager` (mirrors the creator default). */
export function createDefaultForwardRuleUpdater(): ForwardRuleUpdater {
  return new ForwardManager(new InMemoryRuleStore());
}

/**
 * Narrow write capability for `DELETE /api/forwards/:id` (rule delete). The real
 * domain `ForwardManager` satisfies it via `deleteRule`; tests bind a seeded
 * manager shared with Express for parity. `deleteRule` throws `NotFoundError` for
 * an unknown id, stops a running forwarder (runtime cleanup), removes the rule,
 * persists (with rollback — a failed persist restores the rule and restarts it if
 * it was running), and emits `rule.deleted`. It resolves with no value; the route
 * returns `204` with an empty body. Tests delete `enabled:false` stopped rules so
 * no socket/forwarder is involved.
 */
export interface ForwardRuleDeleter {
  deleteRule(ruleId: string): Promise<void>;
}

/** Injection token for the rule deleter. */
export const FORWARD_RULE_DELETER = "FORWARD_RULE_DELETER";

/** Default: a fresh, isolated in-memory `ForwardManager` (mirrors the creator/updater defaults). */
export function createDefaultForwardRuleDeleter(): ForwardRuleDeleter {
  return new ForwardManager(new InMemoryRuleStore());
}

/**
 * Narrow lifecycle capability for `POST /api/forwards/:id/start`. The real domain
 * `ForwardManager` satisfies it via `startRule`; tests bind a seeded manager
 * shared with Express for parity. `startRule` throws `NotFoundError` for an
 * unknown id, is **idempotent** (an already-running rule returns its current
 * status without restarting the socket), otherwise opens the forwarder's
 * listener, emits `rule.started`, and resolves with the rule's `ForwardStatus`
 * (a started rule's status carries a wall-clock `startedAt`). A start failure
 * (e.g. the port is in use) rejects with the underlying error. `enabled`/
 * autostart is NOT a precondition (parity with Express). Socket-opening tests use
 * free ephemeral ports and stop the rule afterwards; the deterministic byte-for-
 * byte parity test pre-starts the rule once on a shared manager so both runtimes
 * hit the idempotent path and return the SAME pinned status (no volatile drift).
 */
export interface ForwardRuleStarter {
  startRule(ruleId: string): Promise<ForwardStatus>;
}

/** Injection token for the rule starter. */
export const FORWARD_RULE_STARTER = "FORWARD_RULE_STARTER";

/** Default: a fresh, isolated in-memory `ForwardManager` (mirrors the other write defaults). */
export function createDefaultForwardRuleStarter(): ForwardRuleStarter {
  return new ForwardManager(new InMemoryRuleStore());
}

/**
 * Narrow lifecycle capability for `POST /api/forwards/:id/stop`. The real domain
 * `ForwardManager` satisfies it via `stopRule`; tests bind a seeded manager. The
 * natural pair of `startRule`: `stopRule` throws `NotFoundError` for an unknown id,
 * is **idempotent** (a not-running rule returns its current status without touching a
 * socket), otherwise closes the forwarder's listener, emits `rule.stopped`, and
 * resolves with the rule's `ForwardStatus`. A stopped status is fully
 * deterministic (`running: false`, zeroed counters, **no `startedAt`**), so —
 * unlike start — byte-for-byte parity needs no shared manager and the
 * already-stopped path needs no socket at all.
 */
export interface ForwardRuleStopper {
  stopRule(ruleId: string): Promise<ForwardStatus>;
}

/** Injection token for the rule stopper. */
export const FORWARD_RULE_STOPPER = "FORWARD_RULE_STOPPER";

/** Default: a fresh, isolated in-memory `ForwardManager` (mirrors the other write defaults). */
export function createDefaultForwardRuleStopper(): ForwardRuleStopper {
  return new ForwardManager(new InMemoryRuleStore());
}

/**
 * Narrow lifecycle capability for `POST /api/forwards/groups/:group/stop`. The real
 * domain `ForwardManager` satisfies it via `stopGroup`; tests bind a seeded manager.
 * `stopGroup` iterates the rules sharing the group (in rule order) and stops each —
 * a not-running rule is a `skipped`/`not_running` no-op (NO socket), a running rule
 * is stopped (`stopped`), a stop error is `failed`; it returns one result per rule
 * (an empty array means no rule matched the group → the route returns `404`). It
 * never mutates rule definitions/order/`enabled`/`group`. Because stopping a group
 * of already-stopped rules touches no socket, byte-for-byte parity needs none.
 */
export interface ForwardGroupStopper {
  stopGroup(group: string): Promise<GroupActionResult[]>;
}

/** Injection token for the group stopper. */
export const FORWARD_GROUP_STOPPER = "FORWARD_GROUP_STOPPER";

/** Default: a fresh, isolated in-memory `ForwardManager` (an empty group → `404`). */
export function createDefaultForwardGroupStopper(): ForwardGroupStopper {
  return new ForwardManager(new InMemoryRuleStore());
}

/**
 * Narrow lifecycle capability for `POST /api/forwards/groups/:group/start`. The real
 * domain `ForwardManager` satisfies it via `startGroup`; tests bind a seeded manager.
 * `startGroup` iterates the rules sharing the group (in rule order) and starts each —
 * an already-running rule is a `skipped`/`already_running` no-op (NO new socket),
 * a stopped rule is started (`started`, opens its forwarder), a start error is
 * `failed`; `enabled`/autostart is NOT a precondition (parity with single-rule
 * start). It returns one result per rule (an empty array means no rule matched the
 * group → the route returns `404`) and never mutates rule definitions/order/
 * `enabled`/`group`. The `GroupActionResponse` carries NO volatile field, so parity
 * is byte-for-byte even across separate managers on different ports.
 */
export interface ForwardGroupStarter {
  startGroup(group: string): Promise<GroupActionResult[]>;
}

/** Injection token for the group starter. */
export const FORWARD_GROUP_STARTER = "FORWARD_GROUP_STARTER";

/** Default: a fresh, isolated in-memory `ForwardManager` (an empty group → `404`). */
export function createDefaultForwardGroupStarter(): ForwardGroupStarter {
  return new ForwardManager(new InMemoryRuleStore());
}

/**
 * Narrow write capability for `POST /api/forwards/reorder`. The real domain
 * `ForwardManager` satisfies it via `reorderRules` + `listRules`; tests bind a
 * seeded manager shared with Express for parity. `reorderRules` validates that
 * every id exists (an unknown id throws `NotFoundError` → 404), rebuilds the rule
 * order with the listed ids first and any unlisted rules appended in their prior
 * order (so a partial set is allowed and a duplicate id is tolerated), persists
 * (with rollback to the previous order on a persist failure), and touches no
 * socket (reorder is metadata only — running rules keep running). An empty `ids`
 * is a no-op. The route then returns the full reordered rule list, so the
 * reorderer also exposes `listRules` (the SAME method the list read uses).
 */
export interface ForwardRulesReorderer {
  reorderRules(ids: string[]): Promise<void>;
  listRules(): ForwardRule[];
}

/** Injection token for the rule reorderer. */
export const FORWARD_RULES_REORDERER = "FORWARD_RULES_REORDERER";

/** Default: a fresh, isolated in-memory `ForwardManager` (mirrors the other write defaults). */
export function createDefaultForwardRulesReorderer(): ForwardRulesReorderer {
  return new ForwardManager(new InMemoryRuleStore());
}
