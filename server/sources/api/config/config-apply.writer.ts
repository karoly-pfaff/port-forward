import type { ExportedConfig, ForwardRule, ImportMode, ImportResult } from "@portier/shared";
import { ForwardManager } from "../../forwarders/forward-manager.js";
import { InMemoryRuleStore } from "../forwards/forwards.writer.js";

/**
 * Narrow write capability for `POST /api/config/apply`. It reads the current rules
 * (to build the plan's `currentRules`) and, when the plan has drift and is not a
 * dry-run, performs a `replace` import. The real domain `ForwardManager` satisfies
 * it via `listRules` + `importConfig` (the SAME `importConfig` the import route uses,
 * so apply inherits its replace-mutation, duplicate-binding rejection, persist
 * **rollback**, enabled-rule start, and `config.imported`/`config.import.failed`
 * activity emission). It is intentionally separate from `CONFIG_IMPORTER` so each
 * config endpoint's provider is independently overridable in tests; the shape is
 * the same. Tests bind a seeded manager; parity fixtures use `enabled:false` rules
 * so the replace import starts no forwarder.
 */
export interface ConfigApplier {
  listRules(): ForwardRule[];
  importConfig(config: ExportedConfig, mode: ImportMode): Promise<ImportResult>;
}

/** Injection token for the config applier. */
export const CONFIG_APPLIER = "CONFIG_APPLIER";

/**
 * Default: a fresh, isolated in-memory `ForwardManager` (no disk, no
 * shared state). When the NestJS server is the active runtime this token is
 * bound to the shared `ForwardManager`; tests override it with a seeded manager.
 */
export function createDefaultConfigApplier(): ConfigApplier {
  return new ForwardManager(new InMemoryRuleStore());
}
