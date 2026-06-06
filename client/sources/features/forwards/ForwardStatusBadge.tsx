import type { ReactElement } from "react";
import type { ForwardStatus } from "@portier/shared";

interface ForwardStatusBadgeProps {
  status: ForwardStatus | undefined;
  onErrorClick?: () => void;
}

export function ForwardStatusBadge({ status, onErrorClick }: ForwardStatusBadgeProps): ReactElement {
  const running = status?.running ?? false;
  const hasError = !!status?.lastError;

  const badgeClass = hasError && !running ? "error" : running ? "running" : "stopped";
  const badgeText = hasError && !running ? "Error" : running ? "Running" : "Stopped";

  return (
    <div className="status-cell">
      <span className={`state ${badgeClass}`}>{badgeText}</span>
      {hasError && (
        onErrorClick ? (
          <button
            type="button"
            className="status-error-icon"
            title={status!.lastError}
            aria-label={`Error: ${status!.lastError}`}
            onClick={onErrorClick}
          >
            ⚠
          </button>
        ) : (
          <span
            className="status-error-icon"
            title={status!.lastError}
            aria-label={`Error: ${status!.lastError}`}
          >
            ⚠
          </span>
        )
      )}
    </div>
  );
}
