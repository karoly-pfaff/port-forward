import type { ExportedConfig, ForwardRule, ImportMode, ImportResult } from "@portier/shared";
import { ForwardManager } from "../../../forward-manager.js";
import { InMemoryRuleStore } from "../forwards/forwards.writer.js";

/**
 * Narrow write capability for `POST /api/config/import`. The real domain
 * `ForwardManager` satisfies it via `importConfig` + `listRules`; tests bind a
 * seeded manager. `importConfig` validates every rule, rejects duplicate listen
 * bindings (and, in merge mode, id/binding conflicts) BEFORE any mutation —
 * returning an `ImportResult` with `errors` and no mutation in those cases — and
 * otherwise replaces (replace mode) or merges (merge mode) the rules, persists
 * with **rollback** on a persist failure (restores the prior rules and stops any
 * forwarders started during the import), starts `enabled` rules, and emits the
 * `config.imported`/`config.import.failed` activity events. The route then returns
 * the full rule list, so the importer also exposes `listRules` (the SAME method the
 * list/export reads use). Tests import `enabled:false` rules so no forwarder starts.
 */
export interface ConfigImporter {
  importConfig(config: ExportedConfig, mode: ImportMode): Promise<ImportResult>;
  listRules(): ForwardRule[];
}

/** Injection token for the config importer. */
export const CONFIG_IMPORTER = "CONFIG_IMPORTER";

/**
 * Default: a fresh, isolated in-memory `ForwardManager` (no disk, no
 * shared state). When the NestJS server becomes the active runtime this token is
 * bound to the shared `ForwardManager`; tests override it with a seeded manager.
 */
export function createDefaultConfigImporter(): ConfigImporter {
  return new ForwardManager(new InMemoryRuleStore());
}
