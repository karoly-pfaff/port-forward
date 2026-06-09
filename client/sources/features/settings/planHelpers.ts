import type { ConfigPlanOperationType, ConfigPlanResponse } from "@portier/shared";

export function formatOperationType(type: ConfigPlanOperationType): string {
  if (type === "add") return "Add";
  if (type === "update") return "Update";
  if (type === "remove") return "Remove";
  return "Unchanged";
}

export function formatChangeValue(v: unknown): string {
  if (v === null || v === undefined) return "(none)";
  if (typeof v === "boolean") return v ? "yes" : "no";
  return String(v);
}

export function isDestructivePlan(plan: ConfigPlanResponse): boolean {
  return plan.summary.destructive > 0;
}

export function hasPlanErrors(plan: ConfigPlanResponse): boolean {
  return plan.summary.hasErrors;
}
