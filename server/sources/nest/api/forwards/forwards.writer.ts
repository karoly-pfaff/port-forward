import type { ForwardRule } from "@portier/shared";
import { ForwardManager, type RuleStore } from "../../../forward-manager.js";

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
 * Minimal in-memory rule store (no disk) for the scaffold default creator. The
 * NestJS app is shadow-only and has no real runtime wired; this keeps creates
 * isolated and disk-free until the manager is bound for real.
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
 * Scaffold default: a fresh, isolated in-memory `ForwardManager` (no disk, no
 * shared state). When the NestJS server becomes the active runtime this token is
 * bound to the shared `ForwardManager`; tests override it with a seeded manager.
 */
export function createDefaultForwardRuleCreator(): ForwardRuleCreator {
  return new ForwardManager(new InMemoryRuleStore());
}
