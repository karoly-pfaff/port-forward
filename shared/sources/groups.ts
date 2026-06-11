// Group operations (v1.8 Slice 4) — start/stop all rules sharing a `group`
// label. These are behaviour over existing rule metadata, not config mutation:
// they do not change rule definitions, order, enabled/group fields, or
// duplicate-binding behaviour. The result list follows the manager's rule order.

export type GroupActionType = "start" | "stop";

/**
 * Per-rule outcome of a group action.
 * - start: `started` (succeeded), `skipped` (reason `already_running`), `failed`
 * - stop:  `stopped` (succeeded), `skipped` (reason `not_running`), `failed`
 * `reason` carries the skip token, or — for `failed` — the error message.
 */
export type GroupActionResultStatus = "started" | "stopped" | "skipped" | "failed";

export interface GroupActionResult {
  ruleId: string;
  ruleName: string;
  status: GroupActionResultStatus;
  reason?: string;
}

export interface GroupActionResponse {
  group: string;
  action: GroupActionType;
  total: number;
  succeeded: number;
  skipped: number;
  failed: number;
  results: GroupActionResult[];
}

/**
 * Build the summary response from an ordered per-rule result list. Counts:
 * `succeeded` = started/stopped, `skipped`, `failed`; `total` = results length.
 * The Go service mirrors this counting; validate:contract guards parity.
 */
export function summarizeGroupAction(
  group: string,
  action: GroupActionType,
  results: GroupActionResult[]
): GroupActionResponse {
  let succeeded = 0;
  let skipped = 0;
  let failed = 0;
  for (const result of results) {
    if (result.status === "started" || result.status === "stopped") succeeded += 1;
    else if (result.status === "skipped") skipped += 1;
    else if (result.status === "failed") failed += 1;
  }
  return { group, action, total: results.length, succeeded, skipped, failed, results };
}
