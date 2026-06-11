import crypto from "node:crypto";
import type { ForwardRule, ForwardRuleInput } from "@portier/shared";
import { listenKey, validateForwardRule } from "@portier/shared";
import type {
  ConfigAppliedCounts,
  ConfigPlanChange,
  ConfigPlanError,
  ConfigPlanOperation,
  ConfigPlanResponse,
  ConfigPlanRuleSnapshot,
  ConfigPlanSummary,
  ConfigPlanWarning,
} from "@portier/shared";

type SnapshotKey = keyof ConfigPlanRuleSnapshot;

const MATERIAL_FIELDS: SnapshotKey[] = [
  "name", "protocol", "listenHost", "listenPort",
  "targetHost", "targetPort", "enabled", "udpMode", "group",
];

// Forwarding fields drive the active socket; changing one of them makes an
// update destructive (it stops/restarts the forwarder). `group` is metadata
// only, so it is deliberately NOT here — a group-only change is non-destructive.
const FORWARDING_FIELDS = new Set<string>([
  "protocol", "listenHost", "listenPort", "targetHost", "targetPort", "udpMode",
]);

export interface BuildConfigPlanInput {
  currentRules: ForwardRule[];
  desiredRaw: unknown;
  now?: Date;
}

export function buildConfigPlan(input: BuildConfigPlanInput): ConfigPlanResponse {
  const generatedAt = (input.now ?? new Date()).toISOString();
  const errors: ConfigPlanError[] = [];
  const warnings: ConfigPlanWarning[] = [];
  const operations: ConfigPlanOperation[] = [];

  const rawRules = extractRulesArray(input.desiredRaw);
  if (rawRules === null) {
    errors.push({
      code: "INVALID_DESIRED_CONFIG",
      message: "desired must be an array of rules or an object with a rules array.",
    });
    return makeResponse(generatedAt, operations, errors, warnings);
  }

  const validDesired: ConfigPlanRuleSnapshot[] = [];
  for (let i = 0; i < rawRules.length; i++) {
    const raw = rawRules[i];
    const result = validateForwardRule(raw);
    if (!result.valid || !result.value) {
      errors.push({
        code: "INVALID_DESIRED_RULE",
        message: `Rule at index ${i}: ${result.errors.join(" ")}`,
        field: `rules[${i}]`,
      });
    } else {
      validDesired.push(validatedToSnapshot(result.value));
    }
  }

  if (errors.length > 0) {
    return makeResponse(generatedAt, operations, errors, warnings);
  }

  detectDuplicateIds(validDesired, errors);
  detectDuplicateKeys(validDesired, errors);

  if (errors.length > 0) {
    return makeResponse(generatedAt, operations, errors, warnings);
  }

  const currentById = new Map<string, ForwardRule>();
  const currentByKey = new Map<string, ForwardRule[]>();
  for (const rule of input.currentRules) {
    currentById.set(rule.id, rule);
    const key = listenKey(rule);
    const bucket = currentByKey.get(key);
    if (bucket) {
      bucket.push(rule);
    } else {
      currentByKey.set(key, [rule]);
    }
  }

  const matchedCurrentIds = new Set<string>();

  for (const desired of validDesired) {
    let matched: ForwardRule | undefined;

    if (desired.id !== undefined) {
      matched = currentById.get(desired.id);
    } else {
      const key = listenKey(desired);
      const candidates = currentByKey.get(key) ?? [];
      if (candidates.length === 1) {
        matched = candidates[0];
      } else if (candidates.length > 1) {
        errors.push({
          code: "AMBIGUOUS_CURRENT_MATCH",
          message: `Multiple current rules match identity key "${key}" for desired rule "${desired.name}". Use an explicit rule id to disambiguate.`,
          field: "id",
        });
        continue;
      }
    }

    if (matched !== undefined) {
      matchedCurrentIds.add(matched.id);
      const currentSnap = ruleToSnapshot(matched);
      const changes = diffMaterialFields(currentSnap, desired);

      if (changes.length === 0) {
        operations.push({
          type: "unchanged",
          ruleId: matched.id,
          ruleName: matched.name,
          protocol: matched.protocol,
          current: currentSnap,
          desired,
          destructive: false,
        });
      } else {
        operations.push({
          type: "update",
          ruleId: matched.id,
          ruleName: desired.name,
          protocol: desired.protocol,
          current: currentSnap,
          desired,
          changes,
          destructive: isDestructiveUpdate(changes),
        });
      }
    } else {
      operations.push({
        type: "add",
        ruleName: desired.name,
        protocol: desired.protocol,
        desired,
        destructive: false,
      });
    }

    if (desired.listenHost === "0.0.0.0") {
      warnings.push({
        code: "LAN_EXPOSURE",
        message: `Rule "${desired.name}" listens on 0.0.0.0 and will expose the port on all interfaces.`,
      });
    }
  }

  if (errors.length > 0) {
    return makeResponse(generatedAt, operations, errors, warnings);
  }

  for (const rule of input.currentRules) {
    if (!matchedCurrentIds.has(rule.id)) {
      operations.push({
        type: "remove",
        ruleId: rule.id,
        ruleName: rule.name,
        protocol: rule.protocol,
        current: ruleToSnapshot(rule),
        destructive: true,
      });
      warnings.push({
        code: "REMOVE_EXISTING",
        message: `Rule "${rule.name}" (${rule.id}) will be removed.`,
      });
    }
  }

  return makeResponse(generatedAt, operations, errors, warnings);
}

export function extractRulesArray(input: unknown): unknown[] | null {
  if (Array.isArray(input)) return input;
  if (input !== null && typeof input === "object") {
    const obj = input as Record<string, unknown>;
    if (Array.isArray(obj.rules)) return obj.rules;
  }
  return null;
}

function validatedToSnapshot(value: ForwardRuleInput): ConfigPlanRuleSnapshot {
  return {
    id: value.id,
    name: value.name!,
    protocol: value.protocol!,
    listenHost: value.listenHost!,
    listenPort: value.listenPort!,
    targetHost: value.targetHost!,
    targetPort: value.targetPort!,
    enabled: value.enabled!,
    udpMode: value.udpMode,
    group: value.group,
  };
}

function ruleToSnapshot(rule: ForwardRule): ConfigPlanRuleSnapshot {
  return {
    id: rule.id,
    name: rule.name,
    protocol: rule.protocol,
    listenHost: rule.listenHost,
    listenPort: rule.listenPort,
    targetHost: rule.targetHost,
    targetPort: rule.targetPort,
    enabled: rule.enabled,
    udpMode: rule.udpMode,
    group: rule.group,
  };
}

function diffMaterialFields(a: ConfigPlanRuleSnapshot, b: ConfigPlanRuleSnapshot): ConfigPlanChange[] {
  const changes: ConfigPlanChange[] = [];
  for (const field of MATERIAL_FIELDS) {
    const before = a[field];
    const after = b[field];
    if (before !== after) {
      changes.push({ field, before, after });
    }
  }
  return changes;
}

function isDestructiveUpdate(changes: ConfigPlanChange[]): boolean {
  return changes.some((c) => FORWARDING_FIELDS.has(c.field));
}

function detectDuplicateIds(rules: ConfigPlanRuleSnapshot[], errors: ConfigPlanError[]): void {
  const seen = new Map<string, number>();
  for (const rule of rules) {
    if (rule.id !== undefined) {
      seen.set(rule.id, (seen.get(rule.id) ?? 0) + 1);
    }
  }
  for (const [id, count] of seen) {
    if (count > 1) {
      errors.push({
        code: "DUPLICATE_DESIRED_ID",
        message: `Desired config has ${count} rules with the same id "${id}".`,
        field: "id",
      });
    }
  }
}

function detectDuplicateKeys(rules: ConfigPlanRuleSnapshot[], errors: ConfigPlanError[]): void {
  const seen = new Map<string, number>();
  for (const rule of rules) {
    const key = listenKey(rule);
    seen.set(key, (seen.get(key) ?? 0) + 1);
  }
  for (const [key, count] of seen) {
    if (count > 1) {
      errors.push({
        code: "DUPLICATE_DESIRED_IDENTITY_KEY",
        message: `Desired config has ${count} rules with identity key "${key}".`,
        field: "listenPort",
      });
    }
  }
}

function makeResponse(
  generatedAt: string,
  operations: ConfigPlanOperation[],
  errors: ConfigPlanError[],
  warnings: ConfigPlanWarning[],
): ConfigPlanResponse {
  const add = operations.filter((o) => o.type === "add").length;
  const update = operations.filter((o) => o.type === "update").length;
  const remove = operations.filter((o) => o.type === "remove").length;
  const unchanged = operations.filter((o) => o.type === "unchanged").length;
  const destructive = operations.filter((o) => o.destructive).length;

  const summary: ConfigPlanSummary = {
    add,
    update,
    remove,
    unchanged,
    destructive,
    hasDrift: add + update + remove > 0,
    hasErrors: errors.length > 0,
  };

  return { generatedAt, mode: "plan", summary, operations, errors, warnings };
}

// ── Apply orchestration ─────────────────────────────────────────────────────
// Apply transformation logic lives here, beside the plan engine — NOT in the
// HTTP handler. The handler owns request/response concerns (missing desired,
// yes/dryRun gating, status codes, calling the manager import). This helper
// owns the pure business transformation: deriving the desired-state replace
// rule list from a completed plan, injecting/preserving rule IDs, and computing
// applied counts. It does not mutate the plan, call the manager, write files,
// or start/stop rules. The Go service mirrors this in
// service/sources/configplan/plan.go (BuildApplyImportFromPlan); validate:contract
// is the parity guard for the externally observable result.

export interface ApplyImportResult {
  /** Desired-state rule list to pass to a "replace" import. Removes are omitted. */
  rules: ForwardRule[];
  /** Counts derived from the plan summary, returned to the caller verbatim. */
  applied: ConfigAppliedCounts;
}

/**
 * Derive the replace-import rule list and applied counts from a completed plan.
 *
 * ID rules (matching the prior inline handler behavior):
 * - `remove` operations are omitted from the result.
 * - When the desired snapshot carries an explicit id, it is preserved.
 * - For `unchanged`/`update` matches without an explicit desired id, the matched
 *   current rule's id (`op.ruleId`) is preserved so identity is stable.
 * - Otherwise (a genuine add) a fresh id is generated via `newId`.
 *
 * `newId` defaults to `crypto.randomUUID`; tests inject a deterministic generator.
 * The caller is responsible for only invoking the replace import when the plan
 * actually has drift — this helper always returns the full desired list.
 */
export function buildApplyImportFromPlan(
  plan: ConfigPlanResponse,
  newId: () => string = () => crypto.randomUUID(),
): ApplyImportResult {
  const rules: ForwardRule[] = [];
  for (const op of plan.operations) {
    if (op.type === "remove") {
      continue;
    }
    const desired = op.desired;
    if (!desired) {
      continue;
    }

    let id: string;
    if (desired.id !== undefined) {
      id = desired.id;
    } else if ((op.type === "unchanged" || op.type === "update") && op.ruleId !== undefined) {
      id = op.ruleId;
    } else {
      id = newId();
    }

    rules.push({
      id,
      name: desired.name,
      protocol: desired.protocol,
      listenHost: desired.listenHost,
      listenPort: desired.listenPort,
      targetHost: desired.targetHost,
      targetPort: desired.targetPort,
      enabled: desired.enabled,
      ...(desired.udpMode !== undefined ? { udpMode: desired.udpMode } : {}),
      ...(desired.group !== undefined ? { group: desired.group } : {}),
    });
  }

  return {
    rules,
    applied: {
      add: plan.summary.add,
      update: plan.summary.update,
      remove: plan.summary.remove,
      unchanged: plan.summary.unchanged,
    },
  };
}
