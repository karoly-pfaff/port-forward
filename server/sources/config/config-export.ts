import type { ActivityEventInput, ExportedConfig, ForwardRule } from "@portier/shared";

/**
 * Inputs for `buildExportedConfig`. The volatile timestamp source (`now`) is
 * passed in so the builder stays pure and deterministically parity-testable —
 * the caller owns the clock (e.g. `new Date()`; the NestJS service
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
 * shape, so the service computes the export shape in one place. It does NOT
 * copy `rules` (historically the manager's array was passed
 * straight through) and has **no side effects** — the `config.exported` activity
 * emission is owned by the caller (the live config-export recorder, and the
 * `ForwardManager`), not the snapshot builder.
 */
export function buildExportedConfig(input: BuildExportedConfigInput): ExportedConfig {
  return {
    version: "1",
    exportedAt: input.now.toISOString(),
    rules: input.rules,
  };
}

/**
 * The canonical `config.exported` activity event payload, shared by every caller
 * that records a successful config export (the live config-export recorder behind
 * `GET /api/config/export`, and the `ForwardManager`) so the event `type`/
 * `severity`/`message`/`details` cannot drift between call sites. Pure: it builds
 * the payload object and emits nothing (the caller passes it to `ActivityStore.add`).
 */
export function configExportedActivityEvent(ruleCount: number): ActivityEventInput {
  return {
    type: "config.exported",
    severity: "info",
    message: `Config exported: ${ruleCount} rule(s).`,
    details: { ruleCount },
  };
}
