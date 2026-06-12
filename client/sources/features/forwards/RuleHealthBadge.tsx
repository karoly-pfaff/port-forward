import { type ReactElement } from "react";
import type { RuleHealth } from "@portier/shared";

// Operator-facing health (v1.8 Slice 7). Distinct from the lifecycle status
// badge: status = running/stopped/error; health = a derived interpretation
// (e.g. "warning" = enabled but not running). Rendered as a compact dot with an
// accessible label/tooltip so it adds signal without cluttering the row.
const HEALTH_LABEL: Record<RuleHealth, string> = {
  healthy: "Healthy",
  warning: "Warning — enabled but not running",
  error: "Error — see last error"
};

// Short, one-word label for the dedicated Health column (paired with the dot).
const HEALTH_SHORT_LABEL: Record<RuleHealth, string> = {
  healthy: "Healthy",
  warning: "Warning",
  error: "Error"
};

export function healthShortLabel(health: RuleHealth): string {
  return HEALTH_SHORT_LABEL[health];
}

export function RuleHealthBadge({ health }: { health: RuleHealth }): ReactElement {
  return (
    <span
      className={`health-badge health-badge--${health}`}
      role="img"
      aria-label={`Health: ${HEALTH_LABEL[health]}`}
      title={HEALTH_LABEL[health]}
    />
  );
}
