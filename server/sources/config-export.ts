import type { ExportedConfig, ForwardRule } from "@portier/shared";

/**
 * Inputs for `buildExportedConfig`. The volatile timestamp source (`now`) is
 * passed in so the builder stays pure and deterministically parity-testable —
 * the caller owns the clock (Express passes `new Date()`; the NestJS service
 * passes its injected `ClockReader`).
 */
export interface BuildExportedConfigInput {
  rules: ForwardRule[];
  now: Date;
}

/**
 * Builds the `GET /api/config/export` snapshot. Pure: identical inputs always
 * produce identical output (including field order, which JSON serialization
 * preserves). The shared builder is the single source of truth for the export
 * shape, so the Express route and the NestJS service cannot drift. It does NOT
 * copy `rules` (the historical Express behavior passed the manager's array
 * straight through) and has **no side effects** — the `config.exported` activity
 * emission stays with the caller (the Express manager), not the snapshot builder.
 */
export function buildExportedConfig(input: BuildExportedConfigInput): ExportedConfig {
  return {
    version: "1",
    exportedAt: input.now.toISOString(),
    rules: input.rules,
  };
}
