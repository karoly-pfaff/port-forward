import type { ForwardRule, ForwardStatus } from "@portier/shared";
import { ForwardManager } from "../../../forward-manager.js";
import { InMemoryRuleStore } from "./forwards.writer.js";

/**
 * Narrow read capability for `POST /api/forwards/:id/diagnose`. Diagnose is
 * READ-ONLY (it inspects the rule + its running state, then runs the shared
 * `diagnoseRule` probes; it never mutates the rule store). The real domain
 * `ForwardManager` satisfies it via `getRule` + `getStatus`; tests bind a seeded
 * manager shared with Express for parity. `getRule` returns `undefined` for an
 * unknown id (the route maps that to a `404`), and `getStatus` returns the rule's
 * current status (its `running` flag gates the listen-bind probe).
 */
export interface DiagnosticReader {
  getRule(ruleId: string): ForwardRule | undefined;
  getStatus(ruleId: string): ForwardStatus;
}

/** Injection token for the diagnostic reader. */
export const DIAGNOSTIC_READER = "DIAGNOSTIC_READER";

/** Scaffold default: a fresh, isolated in-memory `ForwardManager` (an unknown id → `404`). */
export function createDefaultDiagnosticReader(): DiagnosticReader {
  return new ForwardManager(new InMemoryRuleStore());
}
