import type { ReactElement } from "react";
import type { ForwardStatus } from "@portier/shared";

interface ForwardStatusBadgeProps {
  status: ForwardStatus | undefined;
}

// The Status column reports lifecycle only — Running vs Stopped. A rule's error
// condition is shown in the Health column (RuleHealthBadge), so the status badge
// deliberately does not surface lastError.
export function ForwardStatusBadge({ status }: ForwardStatusBadgeProps): ReactElement {
  const running = status?.running ?? false;
  return (
    <span className={`state ${running ? "running" : "stopped"}`}>
      {running ? "Running" : "Stopped"}
    </span>
  );
}
