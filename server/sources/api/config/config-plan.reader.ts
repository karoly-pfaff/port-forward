import type { ForwardRule } from "@portier/shared";

/**
 * Narrow read-only view of the forward manager that `POST /api/config/plan` needs
 * — just the current rule list, which the plan engine diffs against the desired
 * config. The real domain `ForwardManager` satisfies it; tests inject a seeded
 * manager. Mirrors the `ConfigExportReader`/`ForwardsReader` seams. Plan is purely
 * a read + a pure computation — it never mutates rules, opens sockets, or emits
 * activity.
 */
export interface ConfigPlanReader {
  listRules(): ForwardRule[];
}

/** Injection token for the config-plan reader. */
export const CONFIG_PLAN_READER = "CONFIG_PLAN_READER";

/**
 * Default: no forwarding runtime is wired into the static AppModule, so the
 * current rule list is empty (a plan against an empty current config). When the
 * NestJS server is the active runtime this token is bound to the shared
 * `ForwardManager`; tests override it with a seeded manager.
 */
export const emptyConfigPlanReader: ConfigPlanReader = {
  listRules: () => [],
};
