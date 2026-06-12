import type {
  ConfigPlanOperation,
  ConfigPlanOperationType,
  ConfigPlanResponse
} from "@portier/shared";

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

// Display-only mirror of the server's FORWARDING_FIELDS set
// (server/sources/config-plan.ts). Changing one of these restarts the
// forwarder, which is what makes an update destructive; every other material
// field (name, enabled/autostart, group) is metadata only. This copy exists
// purely to *label* a change in the preview — the server stays authoritative
// for the `destructive` flag we gate the Apply button on.
const FORWARDING_FIELDS = new Set<string>([
  "protocol", "listenHost", "listenPort", "targetHost", "targetPort", "udpMode"
]);

const FIELD_LABELS: Record<string, string> = {
  name: "Name",
  protocol: "Protocol",
  listenHost: "Listen host",
  listenPort: "Listen port",
  targetHost: "Target host",
  targetPort: "Target port",
  enabled: "Autostart",
  udpMode: "UDP mode",
  group: "Group"
};

export type ChangeImpact = "forwarding" | "metadata";

// Friendly, human-readable name for a changed snapshot field. Falls back to
// the raw field key for anything not in the table (forward-compatible).
export function formatFieldLabel(field: string): string {
  return FIELD_LABELS[field] ?? field;
}

// Whether changing this field affects the live forwarder (and so triggers a
// restart) or is metadata only. Group is metadata — a group-only change never
// restarts the forwarder.
export function changeImpact(field: string): ChangeImpact {
  return FORWARDING_FIELDS.has(field) ? "forwarding" : "metadata";
}

// True when an update changes only metadata fields (e.g. group, name,
// autostart) and no forwarding field — so applying it does not restart the
// forwarder. Used to reassure operators that a group-only edit is safe.
export function isMetadataOnlyUpdate(op: ConfigPlanOperation): boolean {
  if (op.type !== "update") return false;
  const changes = op.changes ?? [];
  return changes.length > 0 && changes.every((c) => changeImpact(c.field) === "metadata");
}

// Short, plain-language note describing what applying this operation does. For
// updates we inspect the actual changed fields (via isMetadataOnlyUpdate) so we
// never imply a socket restart unless the plan really changes a forwarding
// field — a group/name/autostart-only edit is reported as metadata-only.
export function describeOperationImpact(op: ConfigPlanOperation): string {
  switch (op.type) {
    case "add":
      return "Creates a new rule";
    case "remove":
      return "Removes this existing rule";
    case "unchanged":
      return "No changes";
    case "update":
      return isMetadataOnlyUpdate(op)
        ? "Metadata only — the forwarder is not restarted"
        : "Changes forwarding — the forwarder will restart";
    default:
      return "";
  }
}

export function isDestructivePlan(plan: ConfigPlanResponse): boolean {
  return plan.summary.destructive > 0;
}

export function hasPlanErrors(plan: ConfigPlanResponse): boolean {
  return plan.summary.hasErrors;
}
