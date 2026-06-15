import type { ForwardRule } from "@portier/shared";

/**
 * Narrow read-only view of the forward manager that `GET /api/config/export`
 * needs — just the current rule list to snapshot. The real domain
 * `ForwardManager` satisfies it; tests inject a seeded manager. Mirrors the
 * `StatusReader`/`ForwardsReader` seams. It reads only; the
 * `config.exported` activity emission stays with the Express manager (a write
 * side-effect that stays with the Express manager — the Nest read is pure).
 */
export interface ConfigExportReader {
  listRules(): ForwardRule[];
}

/** Injection token for the config-export reader. */
export const CONFIG_EXPORT_READER = "CONFIG_EXPORT_READER";

/**
 * Default: no forwarding runtime is wired into the NestJS app yet, so
 * the exported rule list is empty. When the NestJS server becomes the active
 * runtime this token is bound to the shared `ForwardManager`; tests override it
 * with a seeded manager.
 */
export const emptyConfigExportReader: ConfigExportReader = {
  listRules: () => [],
};
