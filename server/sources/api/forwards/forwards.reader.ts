import type { ForwardRule } from "@portier/shared";

/**
 * Narrow read-only view of the forward manager that `GET /api/forwards` needs.
 * The real domain `ForwardManager` satisfies it; tests inject a seeded manager.
 * 
 */
export interface ForwardsReader {
  listRules(): ForwardRule[];
}

/** Injection token for the forwards reader. */
export const FORWARDS_READER = "FORWARDS_READER";

/**
 * Default: no forwarding runtime is wired into the static AppModule, so
 * the rule list is empty. When the NestJS server is the active runtime this
 * token is bound to the shared `ForwardManager`; tests override it with a seeded
 * manager.
 */
export const emptyForwardsReader: ForwardsReader = {
  listRules: () => [],
};
