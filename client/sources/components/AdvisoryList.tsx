import type { ReactElement } from "react";
import type { PortAdvisory } from "@portier/shared";

export function AdvisoryList({
  advisories,
  compact = false
}: {
  advisories: PortAdvisory[];
  compact?: boolean;
}): ReactElement | null {
  if (advisories.length === 0) {
    return null;
  }

  return (
    <>
      {advisories.map((advisory) => (
        <p
          key={`${advisory.code}-${advisory.message}`}
          className={`advisory ${advisory.severity} ${compact ? "compact" : ""}`}
        >
          {advisory.message}
        </p>
      ))}
    </>
  );
}
